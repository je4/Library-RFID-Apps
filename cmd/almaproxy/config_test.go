package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EmbeddedDefault(t *testing.T) {
	// Change working directory to project root so relative certificate paths in almaproxy.toml resolve correctly
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("failed to change directory to project root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	conf, err := LoadConfig("")
	if err != nil {
		t.Fatalf("failed to load embedded default config: %v", err)
	}

	if conf.Name == "" {
		t.Error("expected non-empty client name")
	}
	if conf.Addr == "" {
		t.Error("expected non-empty listen address")
	}
	if len(conf.Channels) == 0 {
		t.Error("expected non-empty channels list")
	}
	if conf.MaxTags != 100 {
		t.Errorf("expected default max_tags 100, got %d", conf.MaxTags)
	}
}

func TestLoadConfig_CustomFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.toml")

	tomlData := `
addr = "localhost:9999"
extaddr = "http://localhost:9999"
name = "custom-almaproxy"
wsbroker = "wss://broker.example.com:8765"
channels = ["custom-chan"]
default_reader = "reader-01"
default_isil = "CH-000001"
default_afi = "C2"
max_tags = 250
`
	if err := os.WriteFile(configPath, []byte(tomlData), 0644); err != nil {
		t.Fatalf("failed to write custom config file: %v", err)
	}

	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load custom config file: %v", err)
	}

	if conf.Name != "custom-almaproxy" {
		t.Errorf("expected 'custom-almaproxy', got '%s'", conf.Name)
	}
	if conf.Addr != "localhost:9999" {
		t.Errorf("expected 'localhost:9999', got '%s'", conf.Addr)
	}
	if conf.DefaultReader != "reader-01" {
		t.Errorf("expected default_reader 'reader-01', got '%s'", conf.DefaultReader)
	}
	if conf.DefaultISIL != "CH-000001" {
		t.Errorf("expected default_isil 'CH-000001', got '%s'", conf.DefaultISIL)
	}
	if conf.DefaultAFI != "C2" {
		t.Errorf("expected default_afi 'C2', got '%s'", conf.DefaultAFI)
	}
	if conf.MaxTags != 250 {
		t.Errorf("expected max_tags 250, got %d", conf.MaxTags)
	}
}
