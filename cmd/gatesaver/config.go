package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// GatesaverConfig defines configuration parameters for the gatesaver daemon.
//
// Fields:
//   - Name: Unique client identifier (e.g. "mediathek:gatesaver:01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "https://localhost:8765").
//   - Channels: List of WSBroker channels to subscribe for gate events (e.g. ["gates"]).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
type GatesaverConfig struct {
	Name      string        `toml:"name"`
	WSBroker  string        `toml:"wsbroker"`
	Channels  []string      `toml:"channels"`
	ReadLimit int64         `toml:"read_limit"`
	ClientTLS loader.Config `toml:"clienttls"`
}

// Config is an alias for GatesaverConfig.
type Config = GatesaverConfig

func (c *GatesaverConfig) applyDefaults() {
	if c.Name == "" {
		c.Name = "mediathek:gatesaver:01"
	}
	if c.WSBroker == "" {
		c.WSBroker = "https://localhost:8765"
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{"gates"}
	}
}

// LoadConfig loads the GatesaverConfig from the specified TOML file path,
// or falls back to the embedded default configuration if path is empty.
func LoadConfig(path string) (*GatesaverConfig, error) {
	conf := &GatesaverConfig{}
	if path == "" {
		if _, err := toml.Decode(config.GatesaverConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default config")
		}
	} else if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
	}
	conf.applyDefaults()
	return conf, nil
}
