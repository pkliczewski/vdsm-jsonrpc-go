package main

import (
	"flag"
	"github.com/pkliczewski/vdsm-jsonrpc-go"
	"log"
)

func main() {
	configPath := flag.String("config_file", "example/config.json", "Path to config file")
	flag.Parse()

	config := new(vdsm.Congiuration)
	err := vdsm.GetConfig(*configPath, &config)
	if err != nil {
		log.Fatal(err)
	}
	client := new(vdsm.Client)
	err = client.Connect(config)
	if err != nil {
		log.Fatal(err)
	}
	destination := vdsm.GetId()
	err = client.Subscribe(destination)
	if err != nil {
		log.Fatal(err)
		client.Disconnect()
	}
	response, err := client.Send(destination, "Host.getCapabilities", []string{})
	if err != nil {
		log.Fatal(err)
	} else {
		log.Print(response)
	}
	client.Unsubscribe(destination)
	client.Disconnect()
}
