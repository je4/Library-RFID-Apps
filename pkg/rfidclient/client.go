package rfidclient

import (
	"context"
	"encoding/json"
	"fmt"
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

	// GetDeviceMetadata queries reader hardware details and diagnostic initialization report.
	GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error)

	// WriteTag sends a command to program ISO 28560-2 library metadata and optional AFI to a transponder.
	WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error)

	// WriteTagBatch sends a command to program a batch of RFID tags with individual metadata and optional AFI.
	WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error)

	// WriteTags is a variadic convenience method for batch programming multiple RFID tags.
	WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error)

	// SendWriteTagBatch sends an asynchronous batch write tag command to a specific reader target.
	SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error

	// BroadcastWriteTagBatch broadcasts a batch write tag command to a specific broker channel or configured channels.
	BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error

	// WriteAFI sends a command to set/toggle the AFI security byte on a transponder or batch of transponders.
	WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error)

	// WriteAFIValue is a convenience wrapper for writing an AFI value to a target transponder.
	WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error)

	// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
	WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error)

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

	// OnTagsCleared registers a listener for reader cleared events.
	OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc

	// OnDeviceMetadata registers a listener for device metadata response events.
	OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteTagResponse registers a listener for write tag response events.
	OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnWriteAFIResponse registers a listener for write AFI response events.
	OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc

	// OnAnyRFIDEvent registers a global listener for all RFID-related events.
	OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc

	// SubscribeTags creates a channel delivering stream-based TagReadEvent objects from a given channel.
	SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error)

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

	reqPayload := NewDeviceMetadataRequest()
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

	c.logger.Debug().Msgf("sending device metadata request to %s (id: %s, token: %s)", targetAddr, reqPayload.ID, token)
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
	if metaResp.ClientID == "" && respEvt.Source != "" {
		metaResp.ClientID = respEvt.Source
	}

	return &metaResp, nil
}

// WriteTag sends a command to write ISO 28560 metadata and optional AFI to a transponder.
func (c *RFIDClient) WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error) {
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
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", req.ID).
			Str("target", targetAddr)
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
	if writeResp.ClientID == "" && respEvt.Source != "" {
		writeResp.ClientID = respEvt.Source
	}

	if c.logger != nil {
		if writeResp.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr)
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
				Str("error", writeResp.GetError()).
				Msg("failed to write RFID tag")
		}
	}

	return &writeResp, nil
}

// WriteTagBatch sends a command to program a batch of RFID tags with individual metadata and optional AFI.
func (c *RFIDClient) WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error) {
	if len(tags) == 0 {
		return nil, errors.New("write tag batch: tags list must not be empty")
	}
	return c.WriteTag(ctx, target, NewWriteTagBatchRequest(tags))
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

	if err := c.broker.Send(ctx, reqEvt); err != nil {
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

		if err := c.broker.Send(ctx, reqEvt); err != nil {
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
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", req.ID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("target", targetAddr)
		if len(req.UIDs) > 0 {
			logEvt = logEvt.Strs("uids", req.UIDs)
		} else if req.UID != "" {
			logEvt = logEvt.Str("uid", req.UID)
		}
		logEvt.Msg("writing RFID AFI security byte")
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
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
	if afiResp.ClientID == "" && respEvt.Source != "" {
		afiResp.ClientID = respEvt.Source
	}

	if c.logger != nil {
		if afiResp.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiResp.AFI).
				Str("status", AFISecurityStatus(afiResp.AFI)).
				Str("target", targetAddr)
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
				Str("uid", afiResp.UID).
				Str("error", afiResp.GetError()).
				Msg("failed to update RFID AFI security byte")
		}
	}

	return &afiResp, nil
}

// WriteAFIValue is a convenience wrapper for writing an AFI value to a target reader/transponder.
func (c *RFIDClient) WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(afi, uid))
}

// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
func (c *RFIDClient) WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIBatchRequest(afi, uids))
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

	if err := c.broker.Send(ctx, reqEvt); err != nil {
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

		if err := c.broker.Send(ctx, reqEvt); err != nil {
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
