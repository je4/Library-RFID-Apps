package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"github.com/je4/Library-RFID-Apps/pkg/barcodescanner"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// BarcodeHeadlessConfig defines configuration parameters for the barcode-headless daemon.
//
// Fields:
//   - Name: Unique client identifier (e.g. "barcode:headless:01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "https://localhost:8765").
//   - Channels: List of default WSBroker channels to broadcast scans (e.g. ["mediathek01", "barcode", "all"]).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
//   - Readers: List of configured serial barcode reader instances.
type BarcodeHeadlessConfig struct {
	Name      string                        `toml:"name"`
	WSBroker  string                        `toml:"wsbroker"`
	Channels  []string                      `toml:"channels"`
	ReadLimit int64                         `toml:"read_limit"`
	ClientTLS loader.Config                 `toml:"clienttls"`
	Readers   []barcodescanner.ReaderConfig `toml:"readers"`
}

// Config is an alias for BarcodeHeadlessConfig.
type Config = BarcodeHeadlessConfig

// LoadConfig loads the BarcodeHeadlessConfig from the specified TOML file path, or falls back
// to the embedded default configuration (config/barcode-headless.toml) if path is empty.
func LoadConfig(path string) (*BarcodeHeadlessConfig, error) {
	conf := &BarcodeHeadlessConfig{}
	if path == "" {
		if _, err := toml.Decode(config.BarcodeHeadlessConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default barcode-headless config")
		}
		conf.applyDefaults()
		return conf, nil
	}
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
	}
	conf.applyDefaults()
	return conf, nil
}

func (c *BarcodeHeadlessConfig) applyDefaults() {
	if c.Name == "" {
		c.Name = "barcode:headless:01"
	}
	if c.WSBroker == "" {
		c.WSBroker = "https://localhost:8765"
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{"barcode"}
	}
	for i := range c.Readers {
		c.Readers[i].ApplyDefaults()
	}
}
