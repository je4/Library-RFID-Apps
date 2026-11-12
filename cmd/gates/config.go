package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// GateConfig defines hardware and channel configuration for an individual security gate reader.
type GateConfig struct {
	ID           string   `toml:"id"`
	DeviceID     string   `toml:"device_id"`
	DeviceType   string   `toml:"device_type"`
	Port         string   `toml:"port"`
	BaudRate     int      `toml:"baud_rate"`
	Channels     []string `toml:"channels"`
	Enabled      bool     `toml:"enabled"`
	ReconnectSec int      `toml:"reconnect_sec"`
}

// ApplyDefaults applies fallback values to a GateConfig instance.
func (g *GateConfig) ApplyDefaults() {
	if g.DeviceType == "" {
		g.DeviceType = "feig-gate"
	}
	if g.BaudRate <= 0 {
		g.BaudRate = 38400
	}
	if g.ReconnectSec <= 0 {
		g.ReconnectSec = 3
	}
}

// GatesConfig defines configuration parameters for the gates daemon.
//
// Fields:
//   - Name: Unique client identifier (e.g. "mediathek:gates:01") for WSBroker authorization.
//   - WSBroker: Target broker WebSocket endpoint URL (e.g. "https://localhost:8765").
//   - Channels: List of default WSBroker channels to listen/broadcast (e.g. ["mediathek01", "gates"]).
//   - ReadLimit: Maximum WebSocket message read limit in bytes (optional).
//   - ClientTLS: mTLS configuration via certloader (MiniVault, File, Env, Dev, or ACME).
//   - Gates: List of configured security gate reader instances.
type GatesConfig struct {
	Name      string        `toml:"name"`
	WSBroker  string        `toml:"wsbroker"`
	Channels  []string      `toml:"channels"`
	ReadLimit int64         `toml:"read_limit"`
	ClientTLS loader.Config `toml:"clienttls"`
	Gates     []GateConfig  `toml:"gates"`
}

// Config is an alias for GatesConfig.
type Config = GatesConfig

// LoadConfig loads the GatesConfig from the specified TOML file path, or falls back
// to the embedded default configuration (config/gates.toml) if path is empty.
func LoadConfig(path string) (*GatesConfig, error) {
	conf := &GatesConfig{}
	if path == "" {
		if _, err := toml.Decode(config.GatesConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default gates config")
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

func (c *GatesConfig) applyDefaults() {
	if c.Name == "" {
		c.Name = "mediathek:gates:01"
	}
	if c.WSBroker == "" {
		c.WSBroker = "https://localhost:8765"
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{"gates"}
	}
	for i := range c.Gates {
		c.Gates[i].ApplyDefaults()
	}
}
