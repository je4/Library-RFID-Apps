package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EmbeddedDefault(t *testing.T) {
	// Change working directory to project root so relative certificate paths in library-desk.toml resolve correctly
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
	if conf.Themes.Dark != "" && conf.Themes.Dark != "mediathek-dark" {
		t.Errorf("expected dark theme 'mediathek-dark' or empty, got '%s'", conf.Themes.Dark)
	}
	if conf.Themes.Light != "" && conf.Themes.Light != "mediathek-light" {
		t.Errorf("expected light theme 'mediathek-light' or empty, got '%s'", conf.Themes.Light)
	}
}

func TestLoadConfig_CustomFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom.toml")

	tomlData := `
addr = "localhost:9999"
extaddr = "http://localhost:9999"
name = "custom-desk"
wsbroker = "wss://broker.example.com:8765"
channels = ["custom-chan"]
max_tags = 250
`
	if err := os.WriteFile(configPath, []byte(tomlData), 0644); err != nil {
		t.Fatalf("failed to write custom config file: %v", err)
	}

	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load custom config file: %v", err)
	}

	if conf.Name != "custom-desk" {
		t.Errorf("expected 'custom-desk', got '%s'", conf.Name)
	}
	if conf.Addr != "localhost:9999" {
		t.Errorf("expected 'localhost:9999', got '%s'", conf.Addr)
	}
	if conf.MaxTags != 250 {
		t.Errorf("expected max_tags 250, got %d", conf.MaxTags)
	}
}

func TestLoadConfig_Inventory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "inventory.toml")

	tomlData := `
addr = "localhost:8762"
name = "station-a"
wsbroker = "wss://broker.example.com"
channels = ["inventory01"]

[inventory]
enabled = true

[inventory.webservice]
target_url = "https://inventory.sammlung.cc/inventory?clientid=station-a"
http_method = "POST"
jwt_key = "secret123"
debug_mode = true
timeout = "10s"
`
	if err := os.WriteFile(configPath, []byte(tomlData), 0644); err != nil {
		t.Fatalf("failed to write inventory config file: %v", err)
	}

	conf, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load inventory config file: %v", err)
	}

	if !conf.Inventory.Enabled {
		t.Error("expected inventory.enabled to be true")
	}
	if conf.Inventory.Webservice.TargetURL != "https://inventory.sammlung.cc/inventory?clientid=station-a" {
		t.Errorf("unexpected target_url: %s", conf.Inventory.Webservice.TargetURL)
	}
	if conf.Name != "station-a" {
		t.Errorf("unexpected application name: %s", conf.Name)
	}
	if conf.Inventory.Webservice.HTTPMethod != "POST" {
		t.Errorf("unexpected http_method: %s", conf.Inventory.Webservice.HTTPMethod)
	}
	if conf.Inventory.Webservice.JWTKey != "secret123" {
		t.Errorf("unexpected jwt_key: %s", conf.Inventory.Webservice.JWTKey)
	}
	if !conf.Inventory.Webservice.DebugMode {
		t.Error("expected debug_mode to be true")
	}
	if conf.Inventory.Webservice.Timeout.Seconds() != 10 {
		t.Errorf("expected timeout 10s, got %v", conf.Inventory.Webservice.Timeout)
	}
}
