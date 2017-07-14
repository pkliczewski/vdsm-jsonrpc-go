package vdsm

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/go-stomp/stomp"
	"github.com/satori/go.uuid"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Congiuration struct {
	TlsEnabled        bool
	CaCert            string
	VdsmCert          string
	VdsmKey           string
	Hostname          string
	Port              string
	IncomingHeartbeat int
	OutgoingHeartbeat int
	TLSConfig         *tls.Config
}

type Client struct {
	connection    *stomp.Conn
	configuration *Congiuration
	subscriptions []*stomp.Subscription
}

type jsonRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Id      string      `json:"id"`
	Version string      `json:"jsonrpc"`
}

type jsonResponse struct {
	Id     string                 `json:"id"`
	Result map[string]interface{} `json:"result"`
	Error  *VdsmError             `json:"error"`
}

type VdsmError struct {
	Code    int
	Message string
}

func (e *VdsmError) Error() string {
	return fmt.Sprintf("Code %d, message %s", e.Code, e.Message)
}

func GetConfig(filename string, configuration interface{}) error {

	if len(filename) == 0 {
		return nil
	}
	path, _ := filepath.Abs(filename)
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		return err
	}

	return nil
}

func GetId() string {
	return fmt.Sprintf("%x", uuid.NewV4())
}

func loadCerts(config *Congiuration) {
	ca := x509.NewCertPool()
	ca_bytes, _ := ioutil.ReadFile(config.CaCert)
	ok := ca.AppendCertsFromPEM(ca_bytes)
	if !ok {
		log.Fatal("Failed to load CA certificate")
	}
	certificate, err := tls.LoadX509KeyPair(config.VdsmCert, config.VdsmKey)
	if err != nil {
		log.Fatal(err)
	}
	config.TLSConfig = &tls.Config{
		RootCAs:            ca,
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: false, // TODO: check what is needed to enable it
	}
}

func (client *Client) getSubscription(destination string) *stomp.Subscription {
	for _, sub := range client.subscriptions {
		if sub.Destination() == destination {
			return sub
		}
	}
	return nil
}

func (client *Client) Connect(config *Congiuration) error {
	var connection io.ReadWriteCloser
	var err error

	if config.TlsEnabled {
		loadCerts(config)
		connection, err = tls.Dial("tcp", config.Hostname+":"+config.Port, config.TLSConfig)
	} else {
		connection, err = net.Dial("tcp", config.Hostname+":"+config.Port)
	}
	if err != nil {
		log.Fatal("Failed to connect to ", config.Hostname, ":", config.Port)
	}
	conn, err := stomp.Connect(connection,
		stomp.ConnOpt.AcceptVersion(stomp.V12),
		stomp.ConnOpt.HeartBeat(time.Duration(
			config.OutgoingHeartbeat)*time.Second,
			time.Duration(config.IncomingHeartbeat)*time.Second),
	)
	if err != nil {
		return err
	}
	client.connection = conn
	client.configuration = config
	return nil
}

func (client *Client) Disconnect() {
	if client.connection != nil {
		// none of our clients is graceful so we can ignore any network issues
		client.connection.MustDisconnect()
		client.connection = nil
	}
}

func (client *Client) Subscribe(destination string) error {
	subscription, err := client.connection.Subscribe(
		destination, stomp.AckAuto,
		stomp.SubscribeOpt.Header("id", GetId()))
	if err != nil {
		return err
	}
	client.subscriptions = append(client.subscriptions, subscription)
	return nil
}

func (client *Client) Unsubscribe(destination string) {
	subs := client.subscriptions[:0]
	for _, sub := range client.subscriptions {
		if sub.Destination() == destination {
			sub.Unsubscribe()
		} else {
			subs = append(subs, sub)
		}
	}
	client.subscriptions = subs
}

func (client *Client) Send(destination string, method string, params interface{}) (map[string]interface{}, error) {
	req := new(jsonRequest)
	req.Method = method
	req.Id = GetId()
	req.Params = params
	req.Version = "2.0"

	content, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	err = client.connection.Send("jms.topic.vdsm_requests", "",
		content,
		stomp.SendOpt.Header("reply-to", destination))
	if err != nil {
		return nil, err
	}
	subscription := client.getSubscription(destination)
	resp, err := subscription.Read()
	var response jsonResponse
	err = json.Unmarshal(resp.Body, &response)
	if err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error
	}

	return response.Result, nil
}
