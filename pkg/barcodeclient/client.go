package barcodeclient

import (
	"context"
	"strings"
	"sync"

	"emperror.dev/errors"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
)

// Handler function signatures for concrete Barcode events.
type (
	// BarcodeReadHandler receives barcode read scan events.
	BarcodeReadHandler func(evt *BarcodeReadEvent)

	// DeviceMetadataHandler receives reader hardware metadata responses.
	DeviceMetadataHandler func(evt *DeviceMetadataResponse)

	// AnyBarcodeEventHandler receives any parsed Barcode event payload.
	AnyBarcodeEventHandler func(barcodeEvt any)
)

// Client defines the high-level interface for interacting with barcode-headless daemons.
type Client interface {
	// BrokerClient returns the underlying WSBroker client interface.
	BrokerClient() wsbrokerclient.ClientInterface

	// Connect initiates asynchronous connection to the WSBroker server.
	Connect(ctx context.Context) error

	// ConnectSync connects synchronously and blocks until connected or ctx expires.
	ConnectSync(ctx context.Context) error

	// Close gracefully shuts down the client and releases connections.
	Close() error

	// ForReader returns a reader-scoped client view where commands and event handlers
	// are automatically filtered and targeted to a specific logical reader ID.
	ForReader(readerID string) Client

	// DefaultReaderID returns the configured default reader identifier.
	DefaultReaderID() string

	// GetDeviceMetadata queries reader hardware details and status report.
	GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error)

	// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
	GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error)

	// OnBarcodeRead registers a listener for incoming barcode read events.
	OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc

	// OnBarcodeReadForReader registers a listener for incoming barcode read events from a specific reader ID.
	OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc

	// OnDeviceMetadata registers a listener for device metadata response events.
	OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc

	// OnDeviceMetadataForReader registers a listener for device metadata response events from a specific reader ID.
	OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc

	// OnAnyBarcodeEvent registers a global listener for all barcode-related events.
	OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc

	// OnAnyBarcodeEventForReader registers a listener for all barcode-related events originating from a specific reader ID.
	OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc

	// SubscribeBarcodes creates a channel delivering stream-based BarcodeReadEvent objects from a given channel.
	SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// SubscribeBarcodesForReader creates a channel delivering stream-based BarcodeReadEvent objects filtered for a specific reader ID.
	SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// HandleEvent parses and routes a raw wsbroker Event through the registered Barcode handlers.
	HandleEvent(evt *wsbrokerclient.RawEvent) (any, error)
}

var _ Client = (*BarcodeClient)(nil)

// BarcodeClient implements the Client interface for barcode-headless communications over wsbrokerclient.
type BarcodeClient struct {
	broker wsbrokerclient.ClientInterface
	config *clientConfig
	logger zLogger.ZLogger

	mu           sync.RWMutex
	readHandlers []BarcodeReadHandler
	metaHandlers []DeviceMetadataHandler
	anyHandlers  []AnyBarcodeEventHandler

	brokerUnsub wsbrokerclient.UnsubscribeFunc
	closeOnce   sync.Once
}

// New creates a new BarcodeClient on top of an existing wsbroker ClientInterface.
func New(brokerClient wsbrokerclient.ClientInterface, opts ...Option) (*BarcodeClient, error) {
	if brokerClient == nil {
		return nil, errors.New("brokerClient cannot be nil")
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	logger := cfg.logger
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
		cfg.logger = logger
	}

	c := &BarcodeClient{
		broker: brokerClient,
		config: cfg,
		logger: logger,
	}

	// Register listener for incoming events from WSBroker
	c.brokerUnsub = c.broker.OnEvent(func(evt *event.RawEvent) {
		_, _ = c.HandleEvent(evt)
	})

	return c, nil
}

// NewFromConfig initializes a new wsbroker client from config and creates the BarcodeClient wrapper.
func NewFromConfig(cfg wsbrokerclient.Config, opts ...Option) (*BarcodeClient, error) {
	clientCfg := defaultConfig()
	if cfg.Logger != nil {
		clientCfg.logger = cfg.Logger
	}
	for _, opt := range opts {
		opt(clientCfg)
	}

	if clientCfg.clientName != "" && cfg.ClientName == "" {
		cfg.ClientName = clientCfg.clientName
	}
	if len(clientCfg.channels) > 0 && len(cfg.InitialChannels) == 0 {
		cfg.InitialChannels = clientCfg.channels
	}

	brokerClient, err := wsbrokerclient.New(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create wsbrokerclient")
	}

	allOpts := make([]Option, 0, len(opts)+1)
	if clientCfg.logger != nil {
		allOpts = append(allOpts, WithLogger(clientCfg.logger))
	}
	allOpts = append(allOpts, opts...)

	return New(brokerClient, allOpts...)
}

// BrokerClient returns the underlying wsbrokerclient.ClientInterface.
func (c *BarcodeClient) BrokerClient() wsbrokerclient.ClientInterface {
	return c.broker
}

// ForReader returns a reader-scoped client view where commands and event handlers
// are automatically filtered and targeted to a specific logical reader ID.
func (c *BarcodeClient) ForReader(readerID string) Client {
	return &readerScopedClient{
		parent:   c,
		readerID: strings.TrimSpace(readerID),
	}
}

// DefaultReaderID returns the configured default reader identifier.
func (c *BarcodeClient) DefaultReaderID() string {
	return c.config.defaultReaderID
}

// Connect initiates connection to the WSBroker server.
func (c *BarcodeClient) Connect(ctx context.Context) error {
	return c.broker.Connect(ctx)
}

// ConnectSync connects synchronously and blocks until connected or ctx expires.
func (c *BarcodeClient) ConnectSync(ctx context.Context) error {
	return c.broker.ConnectSync(ctx)
}

// Close gracefully closes the client and removes internal listeners.
func (c *BarcodeClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.brokerUnsub != nil {
			c.brokerUnsub()
		}
		err = c.broker.Close()
	})
	return err
}

func (c *BarcodeClient) resolveTarget(target ...string) (string, error) {
	if len(target) > 0 && strings.TrimSpace(target[0]) != "" {
		return strings.TrimSpace(target[0]), nil
	}
	if strings.TrimSpace(c.config.defaultTarget) != "" {
		return strings.TrimSpace(c.config.defaultTarget), nil
	}
	if len(c.config.channels) > 0 && strings.TrimSpace(c.config.channels[0]) != "" {
		return strings.TrimSpace(c.config.channels[0]), nil
	}
	return "", errors.New("target reader client identifier or channel is required")
}
