package main

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"github.com/je4/Library-RFID-Apps/pkg/sip2"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

type Config struct {
	Name      string            `toml:"name"`
	WSBroker  string            `toml:"wsbroker"`
	Channels  []string          `toml:"channels"`
	ReadLimit int64             `toml:"read_limit"`
	ClientTLS loader.Config     `toml:"clienttls"`
	Tunnel    sip2.TunnelConfig `toml:"tunnel"`
	SIP2      SIP2Config        `toml:"sip2"`
}

type SIP2Config struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
	Location string `toml:"location"`
}

func LoadConfig(path string) (*Config, error) {
	conf := &Config{}
	if path == "" {
		if _, err := toml.Decode(config.SelfcheckoutConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default config")
		}
	} else if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
	}
	if conf.Name == "" {
		conf.Name = "mediathek:selfcheckout:01"
	}
	if conf.WSBroker == "" {
		conf.WSBroker = "https://localhost:8765"
	}
	if len(conf.Channels) == 0 {
		conf.Channels = []string{"mediathek01"}
	}
	return conf, nil
}
