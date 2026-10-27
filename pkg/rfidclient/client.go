package rfidclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
)

// Handler function signatures for concrete RFID events.
type (
	// TagReadHandler receives tag read scan events.
	TagReadHandler func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event)

	// TagsClearedHandler receives reader field cleared events.
	TagsClearedHandler func(cleared *TagsClearedEvent, rawEvt *wsbrokerclient.Event)

	// DeviceMetadataHandler receives reader hardware metadata responses.
	DeviceMetadataHandler func(meta *DeviceMetadataResponse, rawEvt *wsbrokerclient.Event)

	// WriteTagResponseHandler receives tag write responses.
	WriteTagResponseHandler func(resp *WriteTagResponse, rawEvt *wsbrokerclient.Event)

	// WriteAFIResponseHandler receives AFI byte write responses.
	WriteAFIResponseHandler func(resp *WriteAFIResponse, rawEvt *wsbrokerclient.Event)

	// AnyRFIDEventHandler receives any parsed RFID event payload along with the raw envelope.
	AnyRFIDEventHandler func(rfidEvt any, rawEvt *wsbrokerclient.Event)
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
	HandleEvent(evt *wsbrokerclient.Event) (any, error)
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
	c.brokerUnsub = c.broker.OnMessage(func(evt *event.Event) {
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

// GetDeviceMetadata queries reader hardware details and diagnostic startup report.
func (c *RFIDClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target...)
	if err != nil {
		return nil, err
	}
	return c.GetDeviceMetadataForReader(ctx, targetAddr, c.config.defaultReaderID)
}

// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
func (c *RFIDClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewDeviceMetadataRequest(readerID)
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Token = token
	reqEvt.ID = reqPayload.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	c.logger.Debug().Msgf("sending device metadata request to %s (id: %s, reader: %s, token: %s)", targetAddr, reqPayload.ID, readerID, token)
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		return nil, errors.Wrapf(err, "device metadata request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for device metadata request")
	}

	var metaResp DeviceMetadataResponse
	if err := json.Unmarshal(respEvt.Data, &metaResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode device metadata response")
	}

	if metaResp.ID == "" && respEvt.ID != "" {
		metaResp.ID = respEvt.ID
	}
	if metaResp.RefID == "" {
		if respEvt.RefID != "" {
			metaResp.RefID = respEvt.RefID
		} else {
			metaResp.RefID = reqPayload.ID
		}
	}
	if metaResp.Source == "" && respEvt.Source != "" {
		metaResp.Source = respEvt.Source
	}
	if metaResp.ReaderID == "" && respEvt.Source != "" {
		metaResp.ReaderID = respEvt.Source
	}

	return &metaResp, nil
}

// GetDeviceMetadataMulti queries hardware metadata from multiple readers or broadcast channels concurrently using RequestN.
func (c *RFIDClient) GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error) {
	if expectedResponses <= 0 {
		expectedResponses = 1
	}
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewDeviceMetadataRequest(c.config.defaultReaderID)
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Token = token
	reqEvt.ID = reqPayload.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	rawCh := make(chan *event.Event, expectedResponses)
	if err := c.broker.RequestN(reqCtx, reqEvt, expectedResponses, rawCh); err != nil {
		return nil, errors.Wrapf(err, "device metadata multi request failed for target %s", targetAddr)
	}

	var results []*DeviceMetadataResponse
	for respEvt := range rawCh {
		if respEvt == nil || len(respEvt.Data) == 0 {
			continue
		}
		var metaResp DeviceMetadataResponse
		if err := json.Unmarshal(respEvt.Data, &metaResp); err != nil {
			if c.logger != nil {
				c.logger.Warn().Err(err).Msg("failed to decode device metadata response item")
			}
			continue
		}
		if metaResp.ID == "" && respEvt.ID != "" {
			metaResp.ID = respEvt.ID
		}
		if metaResp.RefID == "" {
			if respEvt.RefID != "" {
				metaResp.RefID = respEvt.RefID
			} else {
				metaResp.RefID = reqPayload.ID
			}
		}
		if metaResp.Source == "" && respEvt.Source != "" {
			metaResp.Source = respEvt.Source
		}
		if metaResp.ReaderID == "" && respEvt.Source != "" {
			metaResp.ReaderID = respEvt.Source
		}
		results = append(results, &metaResp)
	}

	return results, nil
}

// WriteTag sends a command to write ISO 28560 metadata and optional AFI to a transponder.
func (c *RFIDClient) WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteTag
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write tag request event")
	}
	reqEvt.Token = token
	reqEvt.ID = req.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		count := len(req.Tags)
		if len(req.Items) > count {
			count = len(req.Items)
		}
		if len(req.UIDs) > count {
			count = len(req.UIDs)
		}
		if count > 1 {
			batchTimeout := time.Duration(count)*2*time.Second + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", req.ID).
			Str("target", targetAddr)
		if req.ReaderID != "" {
			logEvt = logEvt.Str("readerId", req.ReaderID)
		}
		if len(req.Tags) > 0 {
			logEvt = logEvt.Int("tagsCount", len(req.Tags))
		} else if len(req.Items) > 0 {
			logEvt = logEvt.Int("itemsCount", len(req.Items))
		} else {
			logEvt = logEvt.
				Str("itemId", req.ItemID).
				Str("uid", req.UID).
				Str("afi", req.AFI)
		}
		logEvt.Msg("writing RFID tag")
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, errors.Wrapf(err, "write tag request timed out waiting for response from reader hardware (target: %s, readerId: %s)", targetAddr, req.ReaderID)
		}
		return nil, errors.Wrapf(err, "write tag request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for write tag request")
	}

	var writeResp WriteTagResponse
	if err := json.Unmarshal(respEvt.Data, &writeResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode write tag response")
	}

	if writeResp.ID == "" && respEvt.ID != "" {
		writeResp.ID = respEvt.ID
	}
	if writeResp.RefID == "" {
		if respEvt.RefID != "" {
			writeResp.RefID = respEvt.RefID
		} else {
			writeResp.RefID = req.ID
		}
	}
	if writeResp.Source == "" && respEvt.Source != "" {
		writeResp.Source = respEvt.Source
	}
	if writeResp.ReaderID == "" && respEvt.Source != "" {
		writeResp.ReaderID = respEvt.Source
	}

	if c.logger != nil {
		if writeResp.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr)
			if writeResp.ReaderID != "" {
				logEvt = logEvt.Str("readerId", writeResp.ReaderID)
			}
			if len(writeResp.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", writeResp.SuccessfulUIDs)
			} else if writeResp.UID != "" {
				logEvt = logEvt.Str("uid", writeResp.UID).
					Str("itemId", writeResp.ItemID).
					Str("afi", writeResp.AFI)
			}
			logEvt.Msg("RFID tag written successfully")
		} else if writeResp.IsPartial() {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr).
				Str("readerId", writeResp.ReaderID).
				Strs("successfulUids", writeResp.SuccessfulUIDs).
				Interface("failedUids", writeResp.FailedUIDs).
				Str("error", writeResp.GetError()).
				Msg("RFID tags partially written")
		} else {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("itemId", writeResp.ItemID).
				Str("uid", writeResp.UID).
				Str("target", targetAddr).
				Str("readerId", writeResp.ReaderID).
				Str("error", writeResp.GetError()).
				Msg("failed to write RFID tag")
		}
	}

	return &writeResp, nil
}

// WriteTagForReader sends a write tag command directed to a specific logical reader ID.
func (c *RFIDClient) WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error) {
	req.ReaderID = readerID
	return c.WriteTag(ctx, target, req)
}

// WriteTagBatch sends a command to program a batch of RFID tags with individual metadata and optional AFI.
func (c *RFIDClient) WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error) {
	if len(tags) == 0 {
		return nil, errors.New("write tag batch: tags list must not be empty")
	}
	return c.WriteTag(ctx, target, NewWriteTagBatchRequest(tags))
}

// WriteTagBatchStream sends a batch write tag request and streams write tag responses via a channel using RequestN.
func (c *RFIDClient) WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error) {
	if len(tags) == 0 {
		return nil, errors.New("write tag batch stream: tags list must not be empty")
	}
	if expectedResults <= 0 {
		expectedResults = len(tags)
	}
	req := NewWriteTagBatchRequest(tags)
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}
	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write tag request event")
	}
	reqEvt.Token = token
	reqEvt.ID = req.ID

	rawCh := make(chan *event.Event, expectedResults)
	outCh := make(chan *WriteTagResponse, expectedResults)

	go func() {
		defer close(outCh)
		for raw := range rawCh {
			if raw == nil || len(raw.Data) == 0 {
				continue
			}
			var resp WriteTagResponse
			if err := json.Unmarshal(raw.Data, &resp); err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write tag response item")
				}
				continue
			}
			if resp.ID == "" && raw.ID != "" {
				resp.ID = raw.ID
			}
			if resp.RefID == "" {
				if raw.RefID != "" {
					resp.RefID = raw.RefID
				} else {
					resp.RefID = req.ID
				}
			}
			if resp.Source == "" && raw.Source != "" {
				resp.Source = raw.Source
			}
			if resp.ReaderID == "" && raw.Source != "" {
				resp.ReaderID = raw.Source
			}
			outCh <- &resp
		}
	}()

	if err := c.broker.RequestN(ctx, reqEvt, expectedResults, rawCh); err != nil {
		return nil, errors.Wrapf(err, "failed to start streaming batch write tag request to %s", targetAddr)
	}

	return outCh, nil
}

// WriteTags is a variadic convenience method for batch programming multiple RFID tags.
func (c *RFIDClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return c.WriteTagBatch(ctx, target, tags)
}

// SendWriteTagBatch sends an asynchronous batch write tag command to a specific reader target.
func (c *RFIDClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	if len(tags) == 0 {
		return errors.New("send write tag batch: tags list must not be empty")
	}
	req := NewWriteTagBatchRequest(tags)
	if err := req.Validate(); err != nil {
		return err
	}
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return err
	}
	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteTag

	if c.logger != nil {
		c.logger.Info().
			Str("id", req.ID).
			Int("count", len(tags)).
			Str("target", targetAddr).
			Msg("sending batch RFID write tag command")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return errors.Wrap(err, "failed to create batch write tag request event")
	}
	reqEvt.ID = req.ID

	if err := c.broker.Request0(ctx, reqEvt); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Err(err).
				Str("id", req.ID).
				Str("target", targetAddr).
				Int("count", len(tags)).
				Msg("failed to send batch RFID write tag command")
		}
		return errors.Wrapf(err, "failed to send batch write tag command to target %s", targetAddr)
	}

	if c.logger != nil {
		c.logger.Info().
			Str("id", req.ID).
			Str("target", targetAddr).
			Int("count", len(tags)).
			Msg("batch RFID write tag command sent successfully")
	}

	return nil
}

// BroadcastWriteTagBatch broadcasts a batch write tag command to a specific broker channel or configured channels.
func (c *RFIDClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	if len(tags) == 0 {
		return errors.New("broadcast write tag batch: tags list must not be empty")
	}
	req := NewWriteTagBatchRequest(tags)
	if err := req.Validate(); err != nil {
		return err
	}
	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteTag

	channels := make([]string, 0)
	if strings.TrimSpace(channel) != "" {
		channels = append(channels, strings.TrimSpace(channel))
	} else if len(c.config.channels) > 0 {
		for _, ch := range c.config.channels {
			if strings.TrimSpace(ch) != "" {
				channels = append(channels, strings.TrimSpace(ch))
			}
		}
	} else {
		channels = append(channels, "all")
	}

	var firstErr error
	for _, ch := range channels {
		if c.logger != nil {
			c.logger.Info().
				Str("id", req.ID).
				Int("count", len(tags)).
				Str("channel", ch).
				Msg("broadcasting batch RFID write tag command")
		}

		reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), ch, req)
		if err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to create batch write tag broadcast event")
			}
			continue
		}
		reqEvt.Channel = ch
		reqEvt.ID = req.ID

		if err := c.broker.Request0(ctx, reqEvt); err != nil {
			if c.logger != nil {
				c.logger.Warn().
					Err(err).
					Str("id", req.ID).
					Str("channel", ch).
					Int("count", len(tags)).
					Msg("failed to broadcast batch RFID write tag command")
			}
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to broadcast batch write tag command to channel %s", ch)
			}
		} else if c.logger != nil {
			c.logger.Info().
				Str("id", req.ID).
				Str("channel", ch).
				Int("count", len(tags)).
				Msg("batch RFID write tag command broadcast successfully")
		}
	}

	return firstErr
}

// WriteAFI sends a command to set/toggle the AFI security byte on a transponder.
func (c *RFIDClient) WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteAFI
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write afi request event")
	}
	reqEvt.Token = token
	reqEvt.ID = req.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		// Dynamically scale timeout for batch operations if multiple UIDs are provided
		count := len(req.UIDs)
		if count > 1 {
			batchTimeout := time.Duration(count)*1500*time.Millisecond + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", req.ID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("target", targetAddr)
		if req.ReaderID != "" {
			logEvt = logEvt.Str("readerId", req.ReaderID)
		}
		if len(req.UIDs) > 0 {
			logEvt = logEvt.Strs("uids", req.UIDs)
		} else if req.UID != "" {
			logEvt = logEvt.Str("uid", req.UID)
		}
		logEvt.Msg("writing RFID AFI security byte")
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, errors.Wrapf(err, "write afi request timed out waiting for response from reader hardware (target: %s, readerId: %s)", targetAddr, req.ReaderID)
		}
		return nil, errors.Wrapf(err, "write afi request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for write afi request")
	}

	var afiResp WriteAFIResponse
	if err := json.Unmarshal(respEvt.Data, &afiResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode write afi response")
	}

	if afiResp.ID == "" && respEvt.ID != "" {
		afiResp.ID = respEvt.ID
	}
	if afiResp.RefID == "" {
		if respEvt.RefID != "" {
			afiResp.RefID = respEvt.RefID
		} else {
			afiResp.RefID = req.ID
		}
	}
	if afiResp.Source == "" && respEvt.Source != "" {
		afiResp.Source = respEvt.Source
	}
	if afiResp.ReaderID == "" && respEvt.Source != "" {
		afiResp.ReaderID = respEvt.Source
	}

	if c.logger != nil {
		if afiResp.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiResp.AFI).
				Str("status", AFISecurityStatus(afiResp.AFI)).
				Str("target", targetAddr)
			if afiResp.ReaderID != "" {
				logEvt = logEvt.Str("readerId", afiResp.ReaderID)
			}
			if len(afiResp.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", afiResp.SuccessfulUIDs)
			} else if afiResp.UID != "" {
				logEvt = logEvt.Str("uid", afiResp.UID)
			}
			logEvt.Msg("RFID AFI security byte updated successfully")
		} else if afiResp.IsPartial() {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiResp.AFI).
				Str("target", targetAddr).
				Str("readerId", afiResp.ReaderID).
				Strs("successfulUids", afiResp.SuccessfulUIDs).
				Interface("failedUids", afiResp.FailedUIDs).
				Str("error", afiResp.GetError()).
				Msg("RFID AFI security byte partially updated")
		} else {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiResp.AFI).
				Str("target", targetAddr).
				Str("readerId", afiResp.ReaderID).
				Str("uid", afiResp.UID).
				Str("error", afiResp.GetError()).
				Msg("failed to update RFID AFI security byte")
		}
	}

	return &afiResp, nil
}

// WriteAFIForReader sends an AFI write command directed to a specific logical reader ID.
func (c *RFIDClient) WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	req.ReaderID = readerID
	return c.WriteAFI(ctx, target, req)
}

// WriteAFIValue is a convenience wrapper for writing an AFI value to a target reader/transponder.
func (c *RFIDClient) WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(afi, uid))
}

// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
func (c *RFIDClient) WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIBatchRequest(afi, uids))
}

// WriteAFIBatchStream sends a batch AFI write request and streams responses via a channel using RequestN.
func (c *RFIDClient) WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if expectedResults <= 0 {
		if len(req.UIDs) > 0 {
			expectedResults = len(req.UIDs)
		} else {
			expectedResults = 1
		}
	}
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}
	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteAFI
	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write AFI request event")
	}
	reqEvt.Token = token
	reqEvt.ID = req.ID

	rawCh := make(chan *event.Event, expectedResults)
	outCh := make(chan *WriteAFIResponse, expectedResults)

	go func() {
		defer close(outCh)
		for raw := range rawCh {
			if raw == nil || len(raw.Data) == 0 {
				continue
			}
			var resp WriteAFIResponse
			if err := json.Unmarshal(raw.Data, &resp); err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write AFI response item")
				}
				continue
			}
			if resp.ID == "" && raw.ID != "" {
				resp.ID = raw.ID
			}
			if resp.RefID == "" {
				if raw.RefID != "" {
					resp.RefID = raw.RefID
				} else {
					resp.RefID = req.ID
				}
			}
			if resp.Source == "" && raw.Source != "" {
				resp.Source = raw.Source
			}
			if resp.ReaderID == "" && raw.Source != "" {
				resp.ReaderID = raw.Source
			}
			outCh <- &resp
		}
	}()

	if err := c.broker.RequestN(ctx, reqEvt, expectedResults, rawCh); err != nil {
		return nil, errors.Wrapf(err, "failed to start streaming batch AFI write request to %s", targetAddr)
	}

	return outCh, nil
}

// SetAFISecured sets the AFI byte of a tag (or batch of tags / all tags) to AFISecured ("07").
func (c *RFIDClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(AFISecured, uid...))
}

// SetAFIUnsecured sets the AFI byte of a tag (or batch of tags / all tags) to AFIUnsecured ("C2").
func (c *RFIDClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(AFIUnsecured, uid...))
}

// SecureTag writes AFISecured ("07") to the specified transponder UID.
func (c *RFIDClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFISecured)
}

// UnsecureTag writes AFIUnsecured ("C2") to the specified transponder UID.
func (c *RFIDClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFIUnsecured)
}

// SendAFI sends an asynchronous AFI write command to a specific reader target or default reader.
func (c *RFIDClient) SendAFI(ctx context.Context, target string, req WriteAFIRequest) error {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return err
	}

	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteAFI

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", req.ID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("target", targetAddr)
		if req.ReaderID != "" {
			logEvt = logEvt.Str("readerId", req.ReaderID)
		}
		if len(req.UIDs) > 0 {
			logEvt = logEvt.Strs("uids", req.UIDs)
		} else if req.UID != "" {
			logEvt = logEvt.Str("uid", req.UID)
		}
		logEvt.Msg("sending RFID AFI write command")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return errors.Wrap(err, "failed to create AFI write request event")
	}
	reqEvt.ID = req.ID

	if err := c.broker.Request0(ctx, reqEvt); err != nil {
		c.logger.Warn().
			Err(err).
			Str("id", req.ID).
			Str("afi", req.AFI).
			Str("target", targetAddr).
			Str("uid", req.UID).
			Msg("failed to send RFID AFI write command")
		return errors.Wrapf(err, "failed to send AFI write command to target %s", targetAddr)
	}

	c.logger.Info().
		Str("id", req.ID).
		Str("afi", req.AFI).
		Str("status", AFISecurityStatus(req.AFI)).
		Str("target", targetAddr).
		Str("uid", req.UID).
		Msg("RFID AFI write command sent successfully")

	return nil
}

// BroadcastAFI broadcasts an AFI write command to a specific broker channel or configured channels.
func (c *RFIDClient) BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	if req.ID == "" {
		req.ID = wsbrokerclient.GenerateEventID()
	}
	req.Event = EventWriteAFI

	channels := make([]string, 0)
	if strings.TrimSpace(channel) != "" {
		channels = append(channels, strings.TrimSpace(channel))
	} else if len(c.config.channels) > 0 {
		for _, ch := range c.config.channels {
			if strings.TrimSpace(ch) != "" {
				channels = append(channels, strings.TrimSpace(ch))
			}
		}
	} else {
		channels = append(channels, "all")
	}

	var firstErr error
	for _, ch := range channels {
		c.logger.Info().
			Str("id", req.ID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("channel", ch).
			Str("uid", req.UID).
			Msg("broadcasting RFID AFI write command")

		reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), ch, req)
		if err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to create AFI broadcast event")
			}
			continue
		}
		reqEvt.Channel = ch
		reqEvt.ID = req.ID

		if err := c.broker.Request0(ctx, reqEvt); err != nil {
			c.logger.Warn().
				Err(err).
				Str("id", req.ID).
				Str("afi", req.AFI).
				Str("channel", ch).
				Str("uid", req.UID).
				Msg("failed to broadcast RFID AFI write command")
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to broadcast AFI write command to channel %s", ch)
			}
		} else {
			c.logger.Info().
				Str("id", req.ID).
				Str("afi", req.AFI).
				Str("status", AFISecurityStatus(req.AFI)).
				Str("channel", ch).
				Str("uid", req.UID).
				Msg("RFID AFI write command broadcast successfully")
		}
	}

	return firstErr
}

// OnTagRead registers a listener for incoming tag read events.
func (c *RFIDClient) OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.tagReadHandlers = append(c.tagReadHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.tagReadHandlers {
			// Compare function pointers
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.tagReadHandlers = append(c.tagReadHandlers[:i], c.tagReadHandlers[i+1:]...)
				break
			}
		}
	}
}

// OnTagsCleared registers a listener for reader field cleared events.
func (c *RFIDClient) OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.clearedHandlers = append(c.clearedHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.clearedHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.clearedHandlers = append(c.clearedHandlers[:i], c.clearedHandlers[i+1:]...)
				break
			}
		}
	}
}

// OnDeviceMetadata registers a listener for device metadata response events.
func (c *RFIDClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.metaHandlers = append(c.metaHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.metaHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.metaHandlers = append(c.metaHandlers[:i], c.metaHandlers[i+1:]...)
				break
			}
		}
	}
}

// OnWriteTagResponse registers a listener for write tag response events.
func (c *RFIDClient) OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.writeTagHandlers = append(c.writeTagHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.writeTagHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.writeTagHandlers = append(c.writeTagHandlers[:i], c.writeTagHandlers[i+1:]...)
				break
			}
		}
	}
}

// OnWriteAFIResponse registers a listener for write AFI response events.
func (c *RFIDClient) OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.writeAFIHandlers = append(c.writeAFIHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.writeAFIHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.writeAFIHandlers = append(c.writeAFIHandlers[:i], c.writeAFIHandlers[i+1:]...)
				break
			}
		}
	}
}

// OnAnyRFIDEvent registers a global listener for all RFID-related events.
func (c *RFIDClient) OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.anyHandlers = append(c.anyHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, h := range c.anyHandlers {
			if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
				c.anyHandlers = append(c.anyHandlers[:i], c.anyHandlers[i+1:]...)
				break
			}
		}
	}
}

// SubscribeTags creates a channel delivering stream-based TagReadEvent objects from a given channel.
func (c *RFIDClient) SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	tagCh := make(chan *TagReadEvent, 64)

	unsub := c.OnTagRead(func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		if channel != "" && rawEvt != nil && rawEvt.Target != "" && !strings.EqualFold(rawEvt.Target, channel) && !strings.EqualFold(rawEvt.Target, "all") {
			return
		}
		select {
		case tagCh <- tag:
		case <-ctx.Done():
		default:
			c.logger.Warn().Msg("tag stream buffer full, dropping event")
		}
	})

	if channel != "" {
		if err := c.broker.Attach(ctx, channel); err != nil {
			unsub()
			close(tagCh)
			return nil, nil, errors.Wrapf(err, "failed to attach to channel %s", channel)
		}
	}

	cleanup := func() {
		unsub()
		close(tagCh)
	}

	return tagCh, cleanup, nil
}

// HandleEvent parses and routes a raw wsbroker Event through the registered RFID handlers.
func (c *RFIDClient) HandleEvent(evt *wsbrokerclient.Event) (any, error) {
	if evt == nil || len(evt.Data) == 0 {
		return nil, nil
	}

	// Only message events carry RFID payloads; ignore transport/heartbeat events (pong, ping, etc.)
	if evt.Type != "" && evt.Type != event.TypeMessage {
		return nil, nil
	}

	parsed, err := ParseEvent(evt)
	if err != nil {
		if c.logger != nil {
			c.logger.Debug().
				Err(err).
				Str("source", evt.Source).
				Str("target", evt.Target).
				Str("type", string(evt.Type)).
				Msg("event not parsed as RFID payload")
		}
		return nil, err
	}

	c.mu.RLock()
	tagHandlers := append([]TagReadHandler(nil), c.tagReadHandlers...)
	clearedHandlers := append([]TagsClearedHandler(nil), c.clearedHandlers...)
	metaHandlers := append([]DeviceMetadataHandler(nil), c.metaHandlers...)
	writeTagHandlers := append([]WriteTagResponseHandler(nil), c.writeTagHandlers...)
	writeAFIHandlers := append([]WriteAFIResponseHandler(nil), c.writeAFIHandlers...)
	anyHandlers := append([]AnyRFIDEventHandler(nil), c.anyHandlers...)
	c.mu.RUnlock()

	for _, h := range anyHandlers {
		go h(parsed, evt)
	}

	switch v := parsed.(type) {
	case *TagReadEvent:
		for _, h := range tagHandlers {
			go h(v, evt)
		}
	case *TagsClearedEvent:
		for _, h := range clearedHandlers {
			go h(v, evt)
		}
	case *DeviceMetadataResponse:
		for _, h := range metaHandlers {
			go h(v, evt)
		}
	case *WriteTagResponse:
		for _, h := range writeTagHandlers {
			go h(v, evt)
		}
	case *WriteAFIResponse:
		for _, h := range writeAFIHandlers {
			go h(v, evt)
		}
	}

	return parsed, nil
}

// OnTagReadForReader registers a listener for incoming tag read events from a specific reader ID.
func (c *RFIDClient) OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnTagRead(handler)
	}
	return c.OnTagRead(func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		if strings.EqualFold(tag.ReaderID, readerID) {
			handler(tag, rawEvt)
		}
	})
}

// OnTagsClearedForReader registers a listener for reader cleared events from a specific reader ID.
func (c *RFIDClient) OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnTagsCleared(handler)
	}
	return c.OnTagsCleared(func(cleared *TagsClearedEvent, rawEvt *wsbrokerclient.Event) {
		if strings.EqualFold(cleared.ReaderID, readerID) {
			handler(cleared, rawEvt)
		}
	})
}

// OnDeviceMetadataForReader registers a listener for device metadata response events from a specific reader ID.
func (c *RFIDClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnDeviceMetadata(handler)
	}
	return c.OnDeviceMetadata(func(meta *DeviceMetadataResponse, rawEvt *wsbrokerclient.Event) {
		if strings.EqualFold(meta.ReaderID, readerID) {
			handler(meta, rawEvt)
		}
	})
}

// OnWriteTagResponseForReader registers a listener for write tag response events from a specific reader ID.
func (c *RFIDClient) OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnWriteTagResponse(handler)
	}
	return c.OnWriteTagResponse(func(resp *WriteTagResponse, rawEvt *wsbrokerclient.Event) {
		if strings.EqualFold(resp.ReaderID, readerID) {
			handler(resp, rawEvt)
		}
	})
}

// OnWriteAFIResponseForReader registers a listener for write AFI response events from a specific reader ID.
func (c *RFIDClient) OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnWriteAFIResponse(handler)
	}
	return c.OnWriteAFIResponse(func(resp *WriteAFIResponse, rawEvt *wsbrokerclient.Event) {
		if strings.EqualFold(resp.ReaderID, readerID) {
			handler(resp, rawEvt)
		}
	})
}

// OnAnyRFIDEventForReader registers a listener for all RFID-related events originating from a specific reader ID.
func (c *RFIDClient) OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	if strings.TrimSpace(readerID) == "" {
		return c.OnAnyRFIDEvent(handler)
	}
	return c.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {
		if getter, ok := rfidEvt.(interface{ GetReaderID() string }); ok {
			if strings.EqualFold(getter.GetReaderID(), readerID) {
				handler(rfidEvt, rawEvt)
			}
		}
	})
}

// SubscribeTagsForReader creates a channel delivering stream-based TagReadEvent objects filtered for a specific reader ID.
func (c *RFIDClient) SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	tagCh := make(chan *TagReadEvent, 64)

	unsub := c.OnTagReadForReader(readerID, func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		if channel != "" && rawEvt != nil && rawEvt.Target != "" && !strings.EqualFold(rawEvt.Target, channel) && !strings.EqualFold(rawEvt.Target, "all") {
			return
		}
		select {
		case tagCh <- tag:
		case <-ctx.Done():
		default:
			c.logger.Warn().Msg("tag stream buffer full, dropping event")
		}
	})

	if channel != "" {
		if err := c.broker.Attach(ctx, channel); err != nil {
			unsub()
			close(tagCh)
			return nil, nil, errors.Wrapf(err, "failed to attach to channel %s", channel)
		}
	}

	cleanup := func() {
		unsub()
		close(tagCh)
	}

	return tagCh, cleanup, nil
}

// readerScopedClient provides a scoped Client implementation targeting a specific readerID.
type readerScopedClient struct {
	parent   Client
	readerID string
}

func (s *readerScopedClient) BrokerClient() wsbrokerclient.ClientInterface {
	return s.parent.BrokerClient()
}

func (s *readerScopedClient) Connect(ctx context.Context) error {
	return s.parent.Connect(ctx)
}

func (s *readerScopedClient) ConnectSync(ctx context.Context) error {
	return s.parent.ConnectSync(ctx)
}

func (s *readerScopedClient) Close() error {
	return s.parent.Close()
}

func (s *readerScopedClient) ForReader(readerID string) Client {
	return s.parent.ForReader(readerID)
}

func (s *readerScopedClient) DefaultReaderID() string {
	return s.readerID
}

func (s *readerScopedClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	var targetAddr string
	if len(target) > 0 {
		targetAddr = target[0]
	}
	return s.parent.GetDeviceMetadataForReader(ctx, targetAddr, s.readerID)
}

func (s *readerScopedClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	return s.parent.GetDeviceMetadataForReader(ctx, target, readerID)
}

func (s *readerScopedClient) GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error) {
	return s.parent.GetDeviceMetadataMulti(ctx, target, expectedResponses)
}

func (s *readerScopedClient) WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error) {
	if req.ReaderID == "" {
		req.ReaderID = s.readerID
	}
	return s.parent.WriteTag(ctx, target, req)
}

func (s *readerScopedClient) WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error) {
	return s.parent.WriteTagForReader(ctx, target, readerID, req)
}

func (s *readerScopedClient) WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error) {
	req := NewWriteTagBatchRequest(tags)
	req.ReaderID = s.readerID
	return s.WriteTag(ctx, target, req)
}

func (s *readerScopedClient) WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error) {
	if c, ok := s.parent.(*RFIDClient); ok {
		if len(tags) == 0 {
			return nil, errors.New("write tag batch stream: tags list must not be empty")
		}
		if expectedResults <= 0 {
			expectedResults = len(tags)
		}
		req := NewWriteTagBatchRequest(tags)
		req.ReaderID = s.readerID
		if err := req.Validate(); err != nil {
			return nil, err
		}
		targetAddr, err := c.resolveTarget(target)
		if err != nil {
			return nil, err
		}
		token, err := wsbrokerclient.GenerateToken()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate correlation token")
		}
		reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create write tag request event")
		}
		reqEvt.Token = token
		reqEvt.ID = req.ID

		rawCh := make(chan *event.Event, expectedResults)
		outCh := make(chan *WriteTagResponse, expectedResults)

		go func() {
			defer close(outCh)
			for raw := range rawCh {
				if raw == nil || len(raw.Data) == 0 {
					continue
				}
				var resp WriteTagResponse
				if err := json.Unmarshal(raw.Data, &resp); err != nil {
					if c.logger != nil {
						c.logger.Warn().Err(err).Msg("failed to decode streaming write tag response item")
					}
					continue
				}
				if resp.ID == "" && raw.ID != "" {
					resp.ID = raw.ID
				}
				if resp.RefID == "" {
					if raw.RefID != "" {
						resp.RefID = raw.RefID
					} else {
						resp.RefID = req.ID
					}
				}
				if resp.Source == "" && raw.Source != "" {
					resp.Source = raw.Source
				}
				if resp.ReaderID == "" && raw.Source != "" {
					resp.ReaderID = raw.Source
				}
				outCh <- &resp
			}
		}()

		if err := c.broker.RequestN(ctx, reqEvt, expectedResults, rawCh); err != nil {
			return nil, errors.Wrapf(err, "failed to start streaming batch write tag request to %s", targetAddr)
		}

		return outCh, nil
	}
	return s.parent.WriteTagBatchStream(ctx, target, tags, expectedResults)
}

func (s *readerScopedClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return s.WriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	req := NewWriteTagBatchRequest(tags)
	req.ReaderID = s.readerID
	// Use parent's underlying SendWriteTagBatch with the readerID request
	if c, ok := s.parent.(*RFIDClient); ok {
		if len(tags) == 0 {
			return errors.New("send write tag batch: tags list must not be empty")
		}
		if err := req.Validate(); err != nil {
			return err
		}
		targetAddr, err := c.resolveTarget(target)
		if err != nil {
			return err
		}
		if req.ID == "" {
			req.ID = wsbrokerclient.GenerateEventID()
		}
		req.Event = EventWriteTag
		reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, req)
		if err != nil {
			return errors.Wrap(err, "failed to create batch write tag request event")
		}
		reqEvt.ID = req.ID
		return c.broker.Request0(ctx, reqEvt)
	}
	return s.parent.SendWriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	req := NewWriteTagBatchRequest(tags)
	req.ReaderID = s.readerID
	if c, ok := s.parent.(*RFIDClient); ok {
		if len(tags) == 0 {
			return errors.New("broadcast write tag batch: tags list must not be empty")
		}
		if err := req.Validate(); err != nil {
			return err
		}
		if req.ID == "" {
			req.ID = wsbrokerclient.GenerateEventID()
		}
		req.Event = EventWriteTag
		channels := make([]string, 0)
		if strings.TrimSpace(channel) != "" {
			channels = append(channels, strings.TrimSpace(channel))
		} else if len(c.config.channels) > 0 {
			for _, ch := range c.config.channels {
				if strings.TrimSpace(ch) != "" {
					channels = append(channels, strings.TrimSpace(ch))
				}
			}
		} else {
			channels = append(channels, "all")
		}
		var firstErr error
		for _, ch := range channels {
			reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), ch, req)
			if err != nil {
				if firstErr == nil {
					firstErr = errors.Wrap(err, "failed to create batch write tag broadcast event")
				}
				continue
			}
			reqEvt.Channel = ch
			reqEvt.ID = req.ID
			if err := c.broker.Request0(ctx, reqEvt); err != nil && firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to broadcast batch write tag command to channel %s", ch)
			}
		}
		return firstErr
	}
	return s.parent.BroadcastWriteTagBatch(ctx, channel, tags)
}

func (s *readerScopedClient) WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	if req.ReaderID == "" {
		req.ReaderID = s.readerID
	}
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	return s.parent.WriteAFIForReader(ctx, target, readerID, req)
}

func (s *readerScopedClient) WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error) {
	req := NewWriteAFIRequest(afi, uid)
	req.ReaderID = s.readerID
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error) {
	req := NewWriteAFIBatchRequest(afi, uids)
	req.ReaderID = s.readerID
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error) {
	if req.ReaderID == "" {
		req.ReaderID = s.readerID
	}
	return s.parent.WriteAFIBatchStream(ctx, target, req, expectedResults)
}

func (s *readerScopedClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := NewWriteAFIRequest(AFISecured, uid...)
	req.ReaderID = s.readerID
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := NewWriteAFIRequest(AFIUnsecured, uid...)
	req.ReaderID = s.readerID
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return s.SetAFISecured(ctx, target, uid)
}

func (s *readerScopedClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return s.SetAFIUnsecured(ctx, target, uid)
}

func (s *readerScopedClient) SendAFI(ctx context.Context, target string, req WriteAFIRequest) error {
	if req.ReaderID == "" {
		req.ReaderID = s.readerID
	}
	return s.parent.SendAFI(ctx, target, req)
}

func (s *readerScopedClient) BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error {
	if req.ReaderID == "" {
		req.ReaderID = s.readerID
	}
	return s.parent.BroadcastAFI(ctx, channel, req)
}

func (s *readerScopedClient) OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnTagReadForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnTagReadForReader(readerID, handler)
}

func (s *readerScopedClient) OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnTagsClearedForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnTagsClearedForReader(readerID, handler)
}

func (s *readerScopedClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnDeviceMetadataForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnDeviceMetadataForReader(readerID, handler)
}

func (s *readerScopedClient) OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnWriteTagResponseForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnWriteTagResponseForReader(readerID, handler)
}

func (s *readerScopedClient) OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnWriteAFIResponseForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnWriteAFIResponseForReader(readerID, handler)
}

func (s *readerScopedClient) OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnAnyRFIDEventForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnAnyRFIDEventForReader(readerID, handler)
}

func (s *readerScopedClient) SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return s.parent.SubscribeTagsForReader(ctx, channel, s.readerID)
}

func (s *readerScopedClient) SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return s.parent.SubscribeTagsForReader(ctx, channel, readerID)
}

func (s *readerScopedClient) HandleEvent(evt *wsbrokerclient.Event) (any, error) {
	return s.parent.HandleEvent(evt)
}
