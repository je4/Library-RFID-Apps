package libraryclient

import (
	"time"

	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
)

type clientConfig struct {
	defaultTarget   string
	defaultReaderID string
	defaultTimeout  time.Duration
	logger          zLogger.ZLogger
	channels        []string
	clientName      string
}

func defaultConfig() *clientConfig {
	nop := zerolog.Nop()
	return &clientConfig{
		defaultTimeout: 15 * time.Second,
		logger:         &nop,
		channels:       []string{"all"},
	}
}

// Option configures the LibraryClient.
type Option func(*clientConfig)

// WithDefaultTarget sets the default target reader client identifier or channel name.
func WithDefaultTarget(target string) Option {
	return func(c *clientConfig) {
		c.defaultTarget = target
	}
}

// WithDefaultReaderID sets the default reader identifier (readerId) for multi-reader daemons.
func WithDefaultReaderID(readerID string) Option {
	return func(c *clientConfig) {
		c.defaultReaderID = readerID
	}
}

// WithDefaultTimeout sets the default timeout for synchronous request-response commands.
func WithDefaultTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		if timeout > 0 {
			c.defaultTimeout = timeout
		}
	}
}

// WithLogger sets the structured logger for the client.
func WithLogger(logger zLogger.ZLogger) Option {
	return func(c *clientConfig) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithChannels sets initial channels to subscribe/attach for events.
func WithChannels(channels ...string) Option {
	return func(c *clientConfig) {
		if len(channels) > 0 {
			c.channels = channels
		}
	}
}

// WithClientName sets the client identifier name when constructing a wsbrokerclient instance.
func WithClientName(clientName string) Option {
	return func(c *clientConfig) {
		c.clientName = clientName
	}
}
