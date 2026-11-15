package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Embedded(t *testing.T) {
	// Change working directory to project root so relative paths resolve correctly
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../..")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, "mediathek:mockreader:01", cfg.Name)
	assert.Equal(t, "https://localhost:8765", cfg.WSBroker)
	assert.Equal(t, []string{"mediathek01", "gates"}, cfg.Channels)
	assert.Equal(t, "mock-reader-01", cfg.ReaderID)
	assert.Equal(t, "2s", cfg.Interval)
	assert.Equal(t, 2*time.Second, cfg.IntervalDuration)
	assert.Equal(t, "random", cfg.AFIMode)
	assert.Equal(t, 0.5, cfg.AFISecuredProbability)
	assert.True(t, cfg.SendClearEvent)
	assert.Equal(t, time.Second, cfg.TagDurationValue)
	assert.NotEmpty(t, cfg.Barcodes)
	assert.Equal(t, "minivault", cfg.ClientTLS.Type)
}

func TestLoadConfig_File(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "mockreader.toml")

	content := `
name = "custom:mockreader:02"
wsbroker = "wss://broker.example.com:8765"
channels = ["desk-1", "desk-2"]
read_limit = 2097152
reader_id = "custom-reader-01"
interval = "500ms"
random_interval = true
min_interval = "200ms"
max_interval = "800ms"
afi_mode = "closed"
send_clear_event = false
tag_duration = "300ms"
isil = "CH-000001-0"
country = "CH"
crc_ok = true
barcodes = ["BC-100", "BC-200"]

[clienttls]
type = "dev"
`
	err := os.WriteFile(confPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "custom:mockreader:02", cfg.Name)
	assert.Equal(t, "wss://broker.example.com:8765", cfg.WSBroker)
	assert.Equal(t, []string{"desk-1", "desk-2"}, cfg.Channels)
	assert.Equal(t, int64(2097152), cfg.ReadLimit)
	assert.Equal(t, "custom-reader-01", cfg.ReaderID)
	assert.Equal(t, 500*time.Millisecond, cfg.IntervalDuration)
	assert.True(t, cfg.RandomInterval)
	assert.Equal(t, 200*time.Millisecond, cfg.MinIntervalDuration)
	assert.Equal(t, 800*time.Millisecond, cfg.MaxIntervalDuration)
	assert.Equal(t, "closed", cfg.AFIMode)
	assert.False(t, cfg.SendClearEvent)
	assert.Equal(t, 300*time.Millisecond, cfg.TagDurationValue)
	assert.Equal(t, "CH-000001-0", cfg.ISIL)
	assert.Equal(t, []string{"BC-100", "BC-200"}, cfg.Barcodes)
	assert.Equal(t, "dev", cfg.ClientTLS.Type)
}

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "empty.toml")

	err := os.WriteFile(confPath, []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(confPath)
	require.NoError(t, err)
	assert.Equal(t, "mediathek:mockreader:01", cfg.Name)
	assert.Equal(t, "https://localhost:8765", cfg.WSBroker)
	assert.Equal(t, []string{"mediathek01", "gates"}, cfg.Channels)
	assert.Equal(t, "mock-reader-01", cfg.ReaderID)
	assert.Equal(t, "random", cfg.AFIMode)
	assert.Equal(t, 2*time.Second, cfg.IntervalDuration)
	assert.NotEmpty(t, cfg.Barcodes)
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "invalid.toml")

	err := os.WriteFile(confPath, []byte("invalid toml [["), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(confPath)
	assert.Error(t, err)
}

func TestLoadConfig_InvalidInterval(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "invalid_interval.toml")

	err := os.WriteFile(confPath, []byte(`interval = "invalid"`), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(confPath)
	assert.Error(t, err)
}

func TestDetermineAFI(t *testing.T) {
	cfg := &MockReaderConfig{
		AFIMode:               "open",
		AFISecuredProbability: 0.5,
	}
	assert.Equal(t, libraryclient.AFIUnsecured, cfg.DetermineAFI(0.1))
	assert.Equal(t, libraryclient.AFIUnsecured, cfg.DetermineAFI(0.9))

	cfg.AFIMode = "closed"
	assert.Equal(t, libraryclient.AFISecured, cfg.DetermineAFI(0.1))
	assert.Equal(t, libraryclient.AFISecured, cfg.DetermineAFI(0.9))

	cfg.AFIMode = "random"
	cfg.AFISecuredProbability = 0.5
	assert.Equal(t, libraryclient.AFISecured, cfg.DetermineAFI(0.2))   // < 0.5 -> secured
	assert.Equal(t, libraryclient.AFIUnsecured, cfg.DetermineAFI(0.7)) // >= 0.5 -> unsecured
}
