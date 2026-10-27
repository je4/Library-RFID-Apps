package rfidclient

import (
	"context"
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

func removeHandler[T any](slice []T, target T) []T {
	targetPtr := fmt.Sprintf("%p", any(target))
	for i, h := range slice {
		if fmt.Sprintf("%p", any(h)) == targetPtr {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
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
		c.tagReadHandlers = removeHandler(c.tagReadHandlers, handler)
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
		c.clearedHandlers = removeHandler(c.clearedHandlers, handler)
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
		c.metaHandlers = removeHandler(c.metaHandlers, handler)
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
		c.writeTagHandlers = removeHandler(c.writeTagHandlers, handler)
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
		c.writeAFIHandlers = removeHandler(c.writeAFIHandlers, handler)
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
		c.anyHandlers = removeHandler(c.anyHandlers, handler)
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
