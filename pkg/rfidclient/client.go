package rfidclient

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

// Handler function signatures for concrete RFID events.
type (
	// TagReadHandler receives tag read scan events.
	TagReadHandler func(evt *TagReadEvent)

	// TagsClearedHandler receives reader field cleared events.
	TagsClearedHandler func(evt *TagsClearedEvent)

	// DeviceMetadataHandler receives reader hardware metadata responses.
	DeviceMetadataHandler func(evt *DeviceMetadataResponse)

	// WriteTagResponseHandler receives tag write responses.
	WriteTagResponseHandler func(evt *WriteTagResponse)

	// WriteAFIResponseHandler receives AFI byte write responses.
	WriteAFIResponseHandler func(evt *WriteAFIResponse)

	// AnyRFIDEventHandler receives any parsed RFID event payload.
	AnyRFIDEventHandler func(rfidEvt any)
)

// Client defines the high-level interface for interacting with rfid-headless reader daemons.
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

	// GetDeviceMetadata queries reader hardware details and diagnostic initialization report.
	GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error)

	// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
	GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error)

	// GetDeviceMetadataMulti queries hardware metadata from multiple readers or broadcast channels concurrently.
	GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error)

	// WriteTag sends a command to program ISO 28560-2 library metadata and optional AFI to a transponder.
	WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error)

	// WriteTagForReader sends a write tag command directed to a specific logical reader ID.
	WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error)

	// WriteTagBatch sends a command to program a batch of RFID tags with individual metadata and optional AFI.
	WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error)

	// WriteTagBatchStream sends a batch write tag request and streams write tag responses via a channel using RequestN.
	WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error)

	// WriteTags is a variadic convenience method for batch programming multiple RFID tags.
	WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error)

	// SendWriteTagBatch sends an asynchronous batch write tag command to a specific reader target.
	SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error

	// BroadcastWriteTagBatch broadcasts a batch write tag command to a specific broker channel or configured channels.
	BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error

	// WriteAFI sends a command to set/toggle the AFI security byte on a transponder or batch of transponders.
	WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error)

	// WriteAFIForReader sends an AFI write command directed to a specific logical reader ID.
	WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error)

	// WriteAFIValue is a convenience wrapper for writing an AFI value to a target transponder.
	WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error)

	// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
	WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error)

	// WriteAFIBatchStream sends a batch AFI write request and streams responses via a channel using RequestN.
	WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error)

	// SetAFISecured sets the AFI byte of a tag (or batch of tags / all tags) to AFISecured ("07").
	SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)

	// SetAFIUnsecured sets the AFI byte of a tag (or batch of tags / all tags) to AFIUnsecured ("C2").
	SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)

	// SecureTag is a convenience method that writes AFISecured ("07") to a specific tag UID.
	SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)

	// UnsecureTag is a convenience method that writes AFIUnsecured ("C2") to a specific tag UID.
	UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)

	// SendAFI sends an asynchronous AFI write command to a specific reader target or default reader.
	SendAFI(ctx context.Context, target string, req WriteAFIRequest) error

	// BroadcastAFI broadcasts an AFI write command to a specific broker channel or configured channels.
	BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error

	// OnTagRead registers a listener for incoming tag read events.
	OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc

	// OnTagReadForReader registers a listener for incoming tag read events from a specific reader ID.
	OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc

	// OnTagsCleared registers a listener for reader cleared events.
	OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc

	// OnTagsClearedForReader registers a listener for reader cleared events from a specific reader ID.
	OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc

	// OnDeviceMetadata registers a listener for device metadata response events.
	OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc

	// OnDeviceMetadataForReader registers a listener for device metadata response events from a specific reader ID.
	OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteTagResponse registers a listener for write tag response events.
	OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteTagResponseForReader registers a listener for write tag response events from a specific reader ID.
	OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteAFIResponse registers a listener for write AFI response events.
	OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteAFIResponseForReader registers a listener for write AFI response events from a specific reader ID.
	OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnAnyRFIDEvent registers a global listener for all RFID-related events.
	OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc

	// OnAnyRFIDEventForReader registers a listener for all RFID-related events originating from a specific reader ID.
	OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc

	// SubscribeTags creates a channel delivering stream-based TagReadEvent objects from a given channel.
	SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// SubscribeTagsForReader creates a channel delivering stream-based TagReadEvent objects filtered for a specific reader ID.
	SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// HandleEvent parses and routes a raw wsbroker Event through the registered RFID handlers.
	HandleEvent(evt *wsbrokerclient.RawEvent) (any, error)
}

var _ Client = (*RFIDClient)(nil)

// RFIDClient implements the Client interface for rfid-headless communications over wsbrokerclient.
type RFIDClient struct {
	broker wsbrokerclient.ClientInterface
	config *clientConfig
	logger zLogger.ZLogger

	mu               sync.RWMutex
	tagReadHandlers  []TagReadHandler
	clearedHandlers  []TagsClearedHandler
	metaHandlers     []DeviceMetadataHandler
	writeTagHandlers []WriteTagResponseHandler
	writeAFIHandlers []WriteAFIResponseHandler
	anyHandlers      []AnyRFIDEventHandler

	brokerUnsub wsbrokerclient.UnsubscribeFunc
	closeOnce   sync.Once
}

// New creates a new RFIDClient on top of an existing wsbroker ClientInterface.
func New(brokerClient wsbrokerclient.ClientInterface, opts ...Option) (*RFIDClient, error) {
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

	c := &RFIDClient{
		broker: brokerClient,
		config: cfg,
		logger: logger,
	}

	// Register listener for incoming message events from WSBroker
	c.brokerUnsub = c.broker.OnMessage(func(evt *event.RawEvent) {
		_, _ = c.HandleEvent(evt)
	})

	return c, nil
}

// NewFromConfig initializes a new wsbroker client from config and creates the RFIDClient wrapper.
func NewFromConfig(cfg wsbrokerclient.Config, opts ...Option) (*RFIDClient, error) {
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
func (c *RFIDClient) BrokerClient() wsbrokerclient.ClientInterface {
	return c.broker
}

// ForReader returns a reader-scoped client view where commands and event handlers
// are automatically filtered and targeted to a specific logical reader ID.
func (c *RFIDClient) ForReader(readerID string) Client {
	return &readerScopedClient{
		parent:   c,
		readerID: strings.TrimSpace(readerID),
	}
}

// DefaultReaderID returns the configured default reader identifier.
func (c *RFIDClient) DefaultReaderID() string {
	return c.config.defaultReaderID
}

// Connect initiates connection to the WSBroker server.
func (c *RFIDClient) Connect(ctx context.Context) error {
	return c.broker.Connect(ctx)
}

// ConnectSync connects synchronously and blocks until connected or ctx expires.
func (c *RFIDClient) ConnectSync(ctx context.Context) error {
	return c.broker.ConnectSync(ctx)
}

// Close gracefully closes the client and removes internal listeners.
func (c *RFIDClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.brokerUnsub != nil {
			c.brokerUnsub()
		}
		err = c.broker.Close()
	})
	return err
}

func (c *RFIDClient) resolveTarget(target ...string) (string, error) {
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
