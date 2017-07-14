package vdsm

import (
	"testing"
)

func Test_Load_Configuration(t *testing.T) {
	conf := Congiuration{}
	err := GetConfig("example/config.json", &conf)

	if err != nil {
		t.Error("Failed to load config", err)
	}
	if conf.Port != "54321" {
		t.Error("Port not loaded")
	}
	if conf.VdsmCert != "/etc/pki/vdsm/certs/vdsmcert.pem" {
		t.Error("Cert location not loaded")
	}
	if conf.IncomingHeartbeat != 10 {
		t.Error("Incoming heartbeat interval not loaded")
	}
}

func Test_Load_Configuration_Filename_Empty(t *testing.T) {
	conf := Congiuration{}
	err := GetConfig("", &conf)

	if err != nil {
		t.Error("getConfig should not panic", err)
	}
}
