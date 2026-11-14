package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Embedded(t *testing.T) {
	// Change working directory to project root so relative certificate paths in gatesaver.toml resolve correctly
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../..")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, "mediathek:gatesaver:01", cfg.Name)
	assert.Equal(t, "https://localhost:8765", cfg.WSBroker)
	assert.Equal(t, []string{"gates"}, cfg.Channels)
	assert.Equal(t, "minivault", cfg.ClientTLS.Type)
}

func TestLoadConfig_File(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "gatesaver.toml")

	content := `
name = "custom:gatesaver:02"
wsbroker = "wss://broker.example.com:8765"
channels = ["gates-east", "gates-west"]
read_limit = 1048576

[clienttls]
type = "dev"
`
	err := os.WriteFile(confPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "custom:gatesaver:02", cfg.Name)
	assert.Equal(t, "wss://broker.example.com:8765", cfg.WSBroker)
	assert.Equal(t, []string{"gates-east", "gates-west"}, cfg.Channels)
	assert.Equal(t, int64(1048576), cfg.ReadLimit)
	assert.Equal(t, "dev", cfg.ClientTLS.Type)
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "empty.toml")

	err := os.WriteFile(confPath, []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "mediathek:gatesaver:01", cfg.Name)
	assert.Equal(t, "https://localhost:8765", cfg.WSBroker)
	assert.Equal(t, []string{"gates"}, cfg.Channels)
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "invalid.toml")

	err := os.WriteFile(confPath, []byte("invalid toml syntax [["), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(confPath)
	assert.Error(t, err)
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := LoadConfig("non_existent_file.toml")
	assert.Error(t, err)
}
