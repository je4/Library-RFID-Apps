package barcodeclient

import (
	"time"

	"github.com/je4/utils/v2/pkg/zLogger"
)

type clientConfig struct {
	logger          zLogger.ZLogger
	defaultTarget   string
	defaultReaderID string
	defaultTimeout  time.Duration
	clientName      string
	channels        []string
}

func defaultConfig() *clientConfig {
	return &clientConfig{
		defaultTimeout: 5 * time.Second,
	}
}

// Option configures barcode client settings.
type Option func(*clientConfig)

// WithLogger configures a structured zerolog logger.
func WithLogger(logger zLogger.ZLogger) Option {
	return func(c *clientConfig) {
		c.logger = logger
	}
}

// WithDefaultTarget sets the default target reader client identifier.
func WithDefaultTarget(target string) Option {
	return func(c *clientConfig) {
		c.defaultTarget = target
	}
}

// WithDefaultReaderID sets the default logical reader identifier.
func WithDefaultReaderID(readerID string) Option {
	return func(c *clientConfig) {
		c.defaultReaderID = readerID
	}
}

// WithDefaultTimeout sets the default timeout for synchronous request-response calls.
func WithDefaultTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.defaultTimeout = timeout
	}
}

// WithClientName sets the client identifier for WSBroker connection.
func WithClientName(name string) Option {
	return func(c *clientConfig) {
		c.clientName = name
	}
}

// WithChannels sets initial channels to subscribe.
func WithChannels(channels ...string) Option {
	return func(c *clientConfig) {
		c.channels = channels
	}
}
