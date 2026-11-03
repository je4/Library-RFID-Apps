package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// AlmaProxyConfig defines configuration parameters for the almaproxy application.
//
// Fields:
//   - Addr: Local listen address for the HTTP server (e.g. "localhost:5000").
//   - ExtAddr: External address for display and WSDL (e.g. "http://localhost:5000").
//   - Name: Unique client identifier (e.g. "mediathek:almaproxy:01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "https://localhost:8765").
//   - Channels: List of WSBroker channels to subscribe on connect (e.g. ["mediathek01"]).
//   - DefaultReader: Target reader identifier for RPC commands (e.g. "rfid-reader-station-01").
//   - DefaultISIL: Default ISIL identifier formatted as <Country>-<ISIL> (e.g. "CH-001596-0").
//   - DefaultAFI: Default AFI hex byte (e.g. "07" for secured, "C2" for unsecured).
//   - MaxTags: Maximum number of scanned tags to retain in memory (default: 100).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
//   - WebTLS: TLS configuration for the local web server.
type AlmaProxyConfig struct {
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

// Config is an alias for AlmaProxyConfig.
type Config = AlmaProxyConfig

// LoadConfig loads the AlmaProxyConfig from the specified TOML file path, or falls back
// to the embedded default configuration (config/almaproxy.toml) if path is empty.
func LoadConfig(path string) (*AlmaProxyConfig, error) {
	conf := &AlmaProxyConfig{}
	if path == "" {
		if _, err := toml.Decode(config.AlmaProxyConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default config")
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

func (c *AlmaProxyConfig) applyDefaults() {
	if c.Addr == "" {
		c.Addr = "localhost:5000"
	}
	if c.ExtAddr == "" {
		c.ExtAddr = "http://" + c.Addr
	}
	if c.Name == "" {
		c.Name = "mediathek:almaproxy:01"
	}
	if c.WSBroker == "" {
		c.WSBroker = "https://localhost:8765"
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{"mediathek01"}
	}
	if c.DefaultISIL == "" {
		c.DefaultISIL = "CH-001596-0"
	}
	if c.DefaultAFI == "" {
		c.DefaultAFI = "07"
	}
	if c.MaxTags <= 0 {
		c.MaxTags = 100
	}
}
