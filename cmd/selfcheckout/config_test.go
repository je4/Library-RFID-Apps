package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/sip2"
)

func TestLoadConfig(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	conf, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if conf.Name != "mediathek:selfcheckout:01" || conf.WSBroker != "websocket-broker.k8s-tls.ub.unibas.ch" || !reflect.DeepEqual(conf.Channels, []string{"mediathek01"}) || conf.ClientTLS.Type != "minivault" {
		t.Fatalf("unexpected embedded configuration: %+v", conf)
	}
	want := sip2.TunnelConfig{RemoteAddr: "slsp-ubs.alma.exlibrisgroup.com:6443", CertificateFile: "certs/slsp/client.crt", KeyFile: "certs/slsp/client.key", CAFile: "certs/slsp/ca.pem", ConnectTimeout: 200 * time.Second}
	if conf.Tunnel != want {
		t.Fatalf("unexpected embedded tunnel configuration: %+v", conf.Tunnel)
	}
}

func TestLoadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := `name = "checkout-01"
wsbroker = "wss://broker.example.com"
channels = ["one", "two"]
read_limit = 2048
[tunnel]
remote_addr = "alma.example.com:6443"
certificate_file = "client.pem"
key_file = "key.pem"
ca_file = "ca.pem"
connect_timeout = 3000000000
[clienttls]
type = "dev"
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	conf, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if conf.Name != "checkout-01" || conf.WSBroker != "wss://broker.example.com" || !reflect.DeepEqual(conf.Channels, []string{"one", "two"}) || conf.ReadLimit != 2048 || conf.ClientTLS.Type != "dev" {
		t.Fatalf("unexpected file configuration: %+v", conf)
	}
	want := sip2.TunnelConfig{RemoteAddr: "alma.example.com:6443", CertificateFile: "client.pem", KeyFile: "key.pem", CAFile: "ca.pem", ConnectTimeout: 3 * time.Second}
	if conf.Tunnel != want {
		t.Fatalf("unexpected file tunnel configuration: %+v", conf.Tunnel)
	}
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Fatal("expected error for missing configuration")
	}
}
