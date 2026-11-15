package main

import (
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/Library-RFID-Apps/config"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"go.ub.unibas.ch/cloud/certloader/v2/pkg/loader"
)

// TagDefinition allows specifying predefined tags in the configuration.
type TagDefinition struct {
	UID       string `toml:"uid"`
	ItemID    string `toml:"item_id"`
	AFI       string `toml:"afi"`
	ISIL      string `toml:"isil"`
	Country   string `toml:"country"`
	Parts     int    `toml:"parts"`
	PartNo    int    `toml:"part_no"`
	UsageType int    `toml:"usage_type"`
}

// MockReaderConfig defines configuration parameters for the mock RFID tag reader emulator.
type MockReaderConfig struct {
	Name      string        `toml:"name"`
	WSBroker  string        `toml:"wsbroker"`
	Channels  []string      `toml:"channels"`
	ReadLimit int64         `toml:"read_limit"`
	ClientTLS loader.Config `toml:"clienttls"`

	// Reader hardware simulation parameters
	ReaderID   string `toml:"reader_id"`
	DeviceID   string `toml:"device_id"`
	DeviceType string `toml:"device_type"`

	// Tag generation and timing settings
	Interval              string `toml:"interval"`
	IntervalDuration      time.Duration
	RandomInterval        bool   `toml:"random_interval"`
	MinInterval           string `toml:"min_interval"`
	MinIntervalDuration   time.Duration
	MaxInterval           string `toml:"max_interval"`
	MaxIntervalDuration   time.Duration
	AFIMode               string  `toml:"afi_mode"`
	AFISecuredProbability float64 `toml:"afi_secured_probability"`
	SendClearEvent        bool    `toml:"send_clear_event"`
	TagDuration           string  `toml:"tag_duration"`
	TagDurationValue      time.Duration

	// Tag default properties
	ISIL     string          `toml:"isil"`
	Country  string          `toml:"country"`
	CRCOk    bool            `toml:"crc_ok"`
	Barcodes []string        `toml:"barcodes"`
	Tags     []TagDefinition `toml:"tags"`
}

// Config is an alias for MockReaderConfig.
type Config = MockReaderConfig

// LoadConfig loads the MockReaderConfig from the specified TOML file path, or falls back
// to the embedded default configuration (config/mockreader.toml) if path is empty.
func LoadConfig(path string) (*MockReaderConfig, error) {
	conf := &MockReaderConfig{}
	if path == "" {
		if _, err := toml.Decode(config.MockreaderConfigTOML, conf); err != nil {
			return nil, errors.Wrap(err, "cannot decode embedded default mockreader config")
		}
	} else {
		if _, err := toml.DecodeFile(path, conf); err != nil {
			return nil, errors.Wrapf(err, "cannot decode toml file '%s'", path)
		}
	}
	if err := conf.applyDefaults(); err != nil {
		return nil, err
	}
	return conf, nil
}

func (c *MockReaderConfig) applyDefaults() error {
	if c.Name == "" {
		c.Name = "mediathek:mockreader:01"
	}
	if c.WSBroker == "" {
		c.WSBroker = "https://localhost:8765"
	}
	if len(c.Channels) == 0 {
		c.Channels = []string{"mediathek01", "gates"}
	}
	if c.ReaderID == "" {
		c.ReaderID = "mock-reader-01"
	}
	if c.DeviceID == "" {
		c.DeviceID = "mock-feig-lrm2500"
	}
	if c.DeviceType == "" {
		c.DeviceType = "mock-rfid-reader"
	}

	// Timing defaults
	if c.Interval == "" {
		c.Interval = "2s"
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return errors.Wrapf(err, "invalid interval '%s'", c.Interval)
	}
	c.IntervalDuration = d

	if c.MinInterval == "" {
		c.MinInterval = "1s"
	}
	minD, err := time.ParseDuration(c.MinInterval)
	if err != nil {
		return errors.Wrapf(err, "invalid min_interval '%s'", c.MinInterval)
	}
	c.MinIntervalDuration = minD

	if c.MaxInterval == "" {
		c.MaxInterval = "4s"
	}
	maxD, err := time.ParseDuration(c.MaxInterval)
	if err != nil {
		return errors.Wrapf(err, "invalid max_interval '%s'", c.MaxInterval)
	}
	c.MaxIntervalDuration = maxD

	if c.TagDuration == "" {
		c.TagDuration = "1s"
	}
	tagD, err := time.ParseDuration(c.TagDuration)
	if err != nil {
		return errors.Wrapf(err, "invalid tag_duration '%s'", c.TagDuration)
	}
	c.TagDurationValue = tagD

	// AFI mode defaults
	if c.AFIMode == "" {
		c.AFIMode = "random"
	}
	if c.AFISecuredProbability <= 0 {
		c.AFISecuredProbability = 0.5
	}

	// Tag payload defaults
	if c.ISIL == "" {
		c.ISIL = "CH-001596-0"
	}
	if c.Country == "" {
		c.Country = "CH"
	}
	if len(c.Barcodes) == 0 && len(c.Tags) == 0 {
		c.Barcodes = []string{
			"31145000012345",
			"31145000012346",
			"31145000012347",
			"31145000098765",
			"E004010000000001",
			"UB-BS-2024-001",
			"UB-BS-2024-002",
			"BC-987654321",
			"A1029384756",
			"9783161484100",
		}
	}

	// Normalize AFI on predefined tags
	for i := range c.Tags {
		if c.Tags[i].AFI != "" {
			c.Tags[i].AFI = libraryclient.NormalizeAFI(c.Tags[i].AFI)
		}
		if c.Tags[i].ISIL == "" {
			c.Tags[i].ISIL = c.ISIL
		}
		if c.Tags[i].Country == "" {
			c.Tags[i].Country = c.Country
		}
		if c.Tags[i].Parts <= 0 {
			c.Tags[i].Parts = 1
		}
		if c.Tags[i].PartNo <= 0 {
			c.Tags[i].PartNo = 1
		}
		if c.Tags[i].UsageType <= 0 {
			c.Tags[i].UsageType = 1
		}
	}

	return nil
}

// DetermineAFI resolves the AFI byte based on config and random selection if configured.
func (c *MockReaderConfig) DetermineAFI(randVal float64) string {
	mode := strings.ToLower(strings.TrimSpace(c.AFIMode))
	switch mode {
	case "open", "unsecured", "c2", "offen", "entsichert":
		return libraryclient.AFIUnsecured
	case "closed", "secured", "07", "geschlossen", "gesichert":
		return libraryclient.AFISecured
	case "random", "mixed", "zufall":
		fallthrough
	default:
		if randVal < c.AFISecuredProbability {
			return libraryclient.AFISecured
		}
		return libraryclient.AFIUnsecured
	}
}
