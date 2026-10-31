package libraryclient

import (
	"context"
	"strings"
	"sync"

	"emperror.dev/errors"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/rs/zerolog"
)

// AnyEventHandler receives any parsed RFID or Barcode event payload.
type AnyEventHandler func(evt any)

// Client defines the high-level interface for library hardware communication (RFID and Barcode).
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

	// RFID Event Handlers
	OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc
	OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc
	OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc
	OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc
	OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc
	OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc
	OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc
	OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc
	OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc
	OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc
	OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc
	OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc

	// RFID Commands
	GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error)
	GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error)
	GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error)
	WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error)
	WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error)
	WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error)
	WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error)
	WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error)
	SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error
	BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error
	WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error)
	WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error)
	WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error)
	WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error)
	WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error)
	SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)
	SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)
	SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)
	UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)
	SendAFI(ctx context.Context, target string, req WriteAFIRequest) error
	BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error
	SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error)
	SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// Barcode Event Handlers
	OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc
	OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc
	OnBarcodeDeviceMetadata(handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc
	OnBarcodeDeviceMetadataForReader(readerID string, handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc
	OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc
	OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc

	// Barcode Commands
	GetBarcodeDeviceMetadata(ctx context.Context, target ...string) (*BarcodeDeviceMetadataResponse, error)
	GetBarcodeDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*BarcodeDeviceMetadataResponse, error)
	SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error)
	SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error)

	// Generic & Global Listeners
	OnAnyEvent(handler AnyEventHandler) wsbrokerclient.UnsubscribeFunc
	HandleEvent(evt *wsbrokerclient.RawEvent) (any, error)
}

var _ Client = (*LibraryClient)(nil)

// LibraryClient implements the Client interface for library RFID and barcode operations over wsbrokerclient.
type LibraryClient struct {
	broker wsbrokerclient.ClientInterface
	config *clientConfig
	logger zLogger.ZLogger

	mu sync.RWMutex
	// RFID handlers
	tagReadHandlers  []TagReadHandler
	clearedHandlers  []TagsClearedHandler
	rfidMetaHandlers []DeviceMetadataHandler
	writeTagHandlers []WriteTagResponseHandler
	writeAFIHandlers []WriteAFIResponseHandler
	anyRFIDHandlers  []AnyRFIDEventHandler

	// Barcode handlers
	barcodeReadHandlers []BarcodeReadHandler
	barcodeMetaHandlers []BarcodeDeviceMetadataHandler
	anyBarcodeHandlers  []AnyBarcodeEventHandler

	// Global handlers
	anyHandlers []AnyEventHandler

	brokerUnsub wsbrokerclient.UnsubscribeFunc
	closeOnce   sync.Once
}

// New creates a new LibraryClient on top of an existing wsbroker ClientInterface.
func New(brokerClient wsbrokerclient.ClientInterface, opts ...Option) (*LibraryClient, error) {
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

	c := &LibraryClient{
		broker: brokerClient,
		config: cfg,
		logger: logger,
	}

	// Register listener for incoming events from WSBroker
	unsub := brokerClient.OnEvent(func(rawEvt *wsbrokerclient.RawEvent) {
		_, _ = c.HandleEvent(rawEvt)
	})
	c.brokerUnsub = unsub

	return c, nil
}

// NewFromConfig initializes a new WSBroker client internally and wraps it with LibraryClient.
func NewFromConfig(brokerCfg wsbrokerclient.Config, opts ...Option) (*LibraryClient, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.clientName != "" && brokerCfg.ClientName == "" {
		brokerCfg.ClientName = cfg.clientName
	}

	if len(cfg.channels) > 0 {
		for _, ch := range cfg.channels {
			if !strings.EqualFold(ch, "all") && !containsString(brokerCfg.InitialChannels, ch) {
				brokerCfg.InitialChannels = append(brokerCfg.InitialChannels, ch)
			}
		}
	}

	brokerClient, err := wsbrokerclient.New(brokerCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create wsbroker client")
	}

	return New(brokerClient, opts...)
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, val) {
			return true
		}
	}
	return false
}

// BrokerClient returns the underlying WSBroker client interface.
func (c *LibraryClient) BrokerClient() wsbrokerclient.ClientInterface {
	return c.broker
}

// Connect initiates asynchronous connection to the WSBroker server.
func (c *LibraryClient) Connect(ctx context.Context) error {
	return c.broker.Connect(ctx)
}

// ConnectSync connects synchronously and blocks until connected or ctx expires.
func (c *LibraryClient) ConnectSync(ctx context.Context) error {
	return c.broker.ConnectSync(ctx)
}

// Close gracefully shuts down the client and releases connections.
func (c *LibraryClient) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.brokerUnsub != nil {
			c.brokerUnsub()
		}
		err = c.broker.Close()
	})
	return err
}

// ForReader returns a reader-scoped client view where commands and event handlers
// are automatically filtered and targeted to a specific logical reader ID.
func (c *LibraryClient) ForReader(readerID string) Client {
	return &readerScopedClient{
		parent:   c,
		readerID: strings.TrimSpace(readerID),
	}
}

// DefaultReaderID returns the configured default reader identifier.
func (c *LibraryClient) DefaultReaderID() string {
	return c.config.defaultReaderID
}

func (c *LibraryClient) resolveTarget(target ...string) (string, error) {
	if len(target) > 0 && strings.TrimSpace(target[0]) != "" {
		return strings.TrimSpace(target[0]), nil
	}
	if c.config.defaultTarget != "" {
		return c.config.defaultTarget, nil
	}
	return "", errors.New("target reader client name or channel is required (no default configured)")
}
