package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Embedded(t *testing.T) {
	// Change working directory to project root so relative certificate paths resolve correctly
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../..")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Name)
	assert.NotEmpty(t, cfg.WSBroker)
	assert.NotEmpty(t, cfg.Channels)
	assert.NotEmpty(t, cfg.Gates)

	gate := cfg.Gates[0]
	assert.Equal(t, "gate-01", gate.ID)
	assert.Equal(t, "feig-gate-01", gate.DeviceID)
	assert.NotEmpty(t, gate.Port)
	assert.Equal(t, 38400, gate.BaudRate)
}

func TestLoadConfig_File(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.toml")

	content := `
name = "custom-gates-daemon"
wsbroker = "https://broker.example.com:8765"
channels = ["gate01", "security"]

[[gates]]
id = "gate-custom"
port = "COM5"
baud_rate = 115200
channels = ["gate01"]
`
	err := os.WriteFile(confPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-gates-daemon", cfg.Name)
	assert.Equal(t, "https://broker.example.com:8765", cfg.WSBroker)
	assert.Equal(t, []string{"gate01", "security"}, cfg.Channels)
	require.Len(t, cfg.Gates, 1)

	g := cfg.Gates[0]
	assert.Equal(t, "gate-custom", g.ID)
	assert.Equal(t, "COM5", g.Port)
	assert.Equal(t, 115200, g.BaudRate)
	assert.Equal(t, "feig-gate", g.DeviceType) // Default applied
	assert.Equal(t, 3, g.ReconnectSec)         // Default applied
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "empty.toml")

	err := os.WriteFile(confPath, []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "mediathek:gates:01", cfg.Name)
	assert.Equal(t, "https://localhost:8765", cfg.WSBroker)
	assert.Equal(t, []string{"gates"}, cfg.Channels)
}
