package barcodescanner

import (
	"io"
	"strings"
	"time"

	"emperror.dev/errors"
	"go.bug.st/serial"
)

// ReaderConfig defines serial port connection and reader parameters for a barcode scanner.
type ReaderConfig struct {
	ID             string   `toml:"id" json:"id"`
	DeviceID       string   `toml:"device_id" json:"deviceId"`
	DeviceType     string   `toml:"device_type" json:"deviceType"`
	Port           string   `toml:"port" json:"port"`
	BaudRate       int      `toml:"baud_rate" json:"baudRate"`
	DataBits       int      `toml:"data_bits" json:"dataBits"`
	StopBits       int      `toml:"stop_bits" json:"stopBits"`
	Parity         string   `toml:"parity" json:"parity"`
	Delimiter      string   `toml:"delimiter" json:"delimiter"`
	TrimWhitespace bool     `toml:"trim_whitespace" json:"trimWhitespace"`
	StripAIMID     bool     `toml:"strip_aim_id" json:"stripAimId"`
	DebounceMs     int      `toml:"debounce_ms" json:"debounceMs"`
	Channels       []string `toml:"channels" json:"channels"`
	ReadOnly       bool     `toml:"read_only" json:"readOnly"`
	Enabled        bool     `toml:"enabled" json:"enabled"`
	ReconnectSec   int      `toml:"reconnect_sec" json:"reconnectSec"`
}

// ApplyDefaults ensures sensible default parameters are configured.
func (c *ReaderConfig) ApplyDefaults() {
	if c.ID == "" {
		if c.DeviceID != "" {
			c.ID = c.DeviceID
		} else if c.Port != "" {
			c.ID = c.Port
		} else {
			c.ID = "barcode-reader-01"
		}
	}
	if c.DeviceID == "" {
		c.DeviceID = c.ID
	}
	if c.DeviceType == "" {
		c.DeviceType = "honeywell-1300g"
	}
	if c.BaudRate <= 0 {
		c.BaudRate = 9600
	}
	if c.DataBits <= 0 {
		c.DataBits = 8
	}
	if c.StopBits <= 0 {
		c.StopBits = 1
	}
	if c.Parity == "" {
		c.Parity = "none"
	}
	if c.Delimiter == "" {
		c.Delimiter = "\r"
	}
	if c.DebounceMs <= 0 {
		c.DebounceMs = 300
	}
	if c.ReconnectSec <= 0 {
		c.ReconnectSec = 3
	}
}

// SerialPort abstracts a physical or virtual serial communication port.
type SerialPort interface {
	io.ReadWriteCloser
	SetReadTimeout(t time.Duration) error
}

// PortOpener is a factory function for opening a SerialPort.
type PortOpener func(portName string, mode *serial.Mode) (SerialPort, error)

// DefaultPortOpener opens a real hardware serial port using go.bug.st/serial.
func DefaultPortOpener(portName string, mode *serial.Mode) (SerialPort, error) {
	return serial.Open(portName, mode)
}

// ParseSerialMode converts ReaderConfig settings into a serial.Mode configuration.
func (c *ReaderConfig) ParseSerialMode() (*serial.Mode, error) {
	mode := &serial.Mode{
		BaudRate: c.BaudRate,
		DataBits: c.DataBits,
	}

	switch strings.ToLower(strings.TrimSpace(c.Parity)) {
	case "", "none", "n":
		mode.Parity = serial.NoParity
	case "odd", "o":
		mode.Parity = serial.OddParity
	case "even", "e":
		mode.Parity = serial.EvenParity
	case "mark", "m":
		mode.Parity = serial.MarkParity
	case "space", "s":
		mode.Parity = serial.SpaceParity
	default:
		return nil, errors.Errorf("invalid parity '%s', supported: none, odd, even, mark, space", c.Parity)
	}

	switch c.StopBits {
	case 1:
		mode.StopBits = serial.OneStopBit
	case 2:
		mode.StopBits = serial.TwoStopBits
	default:
		return nil, errors.Errorf("invalid stop bits '%d', supported: 1, 2", c.StopBits)
	}

	return mode, nil
}
