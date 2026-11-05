package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
	if conf.Name != "mediathek:selfcheckout:01" || conf.WSBroker != "https://localhost:8765" || !reflect.DeepEqual(conf.Channels, []string{"mediathek01"}) || conf.ClientTLS.Type != "minivault" {
		t.Fatalf("unexpected embedded configuration: %+v", conf)
	}
}

func TestLoadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := `name = "checkout-01"
wsbroker = "wss://broker.example.com"
channels = ["one", "two"]
read_limit = 2048
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
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Fatal("expected error for missing configuration")
	}
}
