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
	assert.NotEmpty(t, cfg.Readers)

	reader := cfg.Readers[0]
	assert.Equal(t, "honeywell-01", reader.ID)
	assert.Equal(t, "honeywell-1300g-01", reader.DeviceID)
	assert.NotEmpty(t, reader.Port)
	assert.Equal(t, 9600, reader.BaudRate)
}

func TestLoadConfig_File(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.toml")

	content := `
name = "custom-barcode-daemon"
wsbroker = "https://broker.example.com:8765"
channels = ["desk01", "scanner"]

[[readers]]
id = "honeywell-custom"
port = "COM4"
baud_rate = 115200
channels = ["desk01"]
`
	err := os.WriteFile(confPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-barcode-daemon", cfg.Name)
	assert.Equal(t, "https://broker.example.com:8765", cfg.WSBroker)
	assert.Equal(t, []string{"desk01", "scanner"}, cfg.Channels)
	require.Len(t, cfg.Readers, 1)

	r := cfg.Readers[0]
	assert.Equal(t, "honeywell-custom", r.ID)
	assert.Equal(t, "COM4", r.Port)
	assert.Equal(t, 115200, r.BaudRate)
	assert.Equal(t, 8, r.DataBits) // Default applied
	assert.Equal(t, "\r", r.Delimiter)
}
