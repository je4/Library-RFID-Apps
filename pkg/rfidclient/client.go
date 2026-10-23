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

	// WriteAFI sends a command to set/toggle the AFI security byte on a transponder.
	WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error)

	// WriteAFIValue is a convenience wrapper for writing an AFI value to a target transponder.
	WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error)

	// SetAFISecured sets the AFI byte of a tag (or all tags) to AFISecured ("07").
	SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)

	// SetAFIUnsecured sets the AFI byte of a tag (or all tags) to AFIUnsecured ("C2").
	SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error)

	// SecureTag is a convenience method that writes AFISecured ("07") to a specific tag UID.
	SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)

	// UnsecureTag is a convenience method that writes AFIUnsecured ("C2") to a specific tag UID.
	UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error)

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

	c := &RFIDClient{
		broker: brokerClient,
		config: cfg,
		logger: cfg.logger,
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

	return New(brokerClient, opts...)
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

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	c.logger.Debug().Msgf("sending device metadata request to %s (token: %s)", targetAddr, token)
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

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	if c.logger != nil {
		c.logger.Info().
			Str("itemId", req.ItemID).
			Str("target", targetAddr).
			Str("uid", req.UID).
			Str("afi", req.AFI).
			Msg("writing RFID tag")
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

	if c.logger != nil {
		if writeResp.IsSuccess() {
			c.logger.Info().
				Str("itemId", writeResp.ItemID).
				Str("uid", writeResp.UID).
				Str("target", targetAddr).
				Str("afi", writeResp.AFI).
				Msg("RFID tag written successfully")
		} else {
			c.logger.Warn().
				Str("itemId", writeResp.ItemID).
				Str("uid", writeResp.UID).
				Str("target", targetAddr).
				Str("error", writeResp.GetError()).
				Msg("failed to write RFID tag")
		}
	}

	return &writeResp, nil
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

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	if c.logger != nil {
		c.logger.Info().
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("target", targetAddr).
			Str("uid", req.UID).
			Msg("writing RFID AFI security byte")
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

	if c.logger != nil {
		if afiResp.IsSuccess() {
			c.logger.Info().
				Str("afi", afiResp.AFI).
				Str("status", AFISecurityStatus(afiResp.AFI)).
				Str("target", targetAddr).
				Str("uid", afiResp.UID).
				Msg("RFID AFI security byte updated successfully")
		} else {
			c.logger.Warn().
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

// SetAFISecured sets the AFI byte of a tag (or all tags) to AFISecured ("07").
func (c *RFIDClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	var targetUID string
	if len(uid) > 0 {
		targetUID = uid[0]
	}
	return c.WriteAFIValue(ctx, target, targetUID, AFISecured)
}

// SetAFIUnsecured sets the AFI byte of a tag (or all tags) to AFIUnsecured ("C2").
func (c *RFIDClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	var targetUID string
	if len(uid) > 0 {
		targetUID = uid[0]
	}
	return c.WriteAFIValue(ctx, target, targetUID, AFIUnsecured)
}

// SecureTag writes AFISecured ("07") to the specified transponder UID.
func (c *RFIDClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFISecured)
}

// UnsecureTag writes AFIUnsecured ("C2") to the specified transponder UID.
func (c *RFIDClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFIUnsecured)
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
