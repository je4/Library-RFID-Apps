package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// DeskAppConfig defines configuration parameters for the rfid-desk application.
//
// Fields:
//   - Addr: Local listen address for the HTTP web server (e.g. "localhost:8762").
//   - ExtAddr: External address for links and display (e.g. "http://localhost:8762").
//   - Name: Unique client identifier (e.g. "rfid-desk-01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "https://localhost:8765").
//   - Channels: List of WSBroker channels to subscribe on connect (e.g. ["all", "intern", "rfid"]).
//   - DefaultReader: Target reader identifier for RPC commands (e.g. "rfid-reader-station-01").
//   - DefaultISIL: Default ISIL identifier formatted as <Country>-<ISIL> (e.g. "CH-000001").
//   - DefaultAFI: Default AFI hex byte (e.g. "07" for secured, "C2" for unsecured).
//   - MaxTags: Maximum number of scanned tags to retain in memory (default: 100).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
//   - WebTLS: TLS configuration for the local web server.
type DeskAppConfig struct {
	Addr          string        `toml:"addr"`
	ExtAddr       string        `toml:"extaddr"`
	Name          string        `toml:"name"`
	WSBroker      string        `toml:"wsbroker"`
	Channels      []string      `toml:"channels"`
	DefaultReader string        `toml:"default_reader"`
	DefaultISIL   string        `toml:"default_isil"`
	DefaultAFI    string        `toml:"default_afi"`
	MaxTags       int           `toml:"max_tags"`
	ReadLimit     int64         `toml:"read_limit"`
	ClientTLS     loader.Config `toml:"clienttls"`
	WebTLS        loader.Config `toml:"webtls"`
}

// Config is an alias for DeskAppConfig.
type Config = DeskAppConfig

// LoadConfig loads the DeskAppConfig from the specified TOML file path, or falls back
// to the embedded default configuration (config/rfid-desk.toml) if path is empty.
func LoadConfig(path string) (*DeskAppConfig, error) {
	conf := &DeskAppConfig{}
	if path == "" {
		if _, err := toml.Decode(config.RFIDDeskConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default config")
		}
		return conf, nil
	}
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
	}
	return conf, nil
}
