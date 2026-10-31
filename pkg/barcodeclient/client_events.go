package barcodeclient

import (
	"context"
	"encoding/json"
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

// OnBarcodeRead registers a listener for incoming barcode read events.
func (c *BarcodeClient) OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.readHandlers = append(c.readHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.readHandlers = removeHandler(c.readHandlers, handler)
	}
}

// OnBarcodeReadForReader registers a listener for incoming barcode read events from a specific reader ID.
func (c *BarcodeClient) OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnDeviceMetadata registers a listener for device metadata response events.
func (c *BarcodeClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnDeviceMetadataForReader registers a listener for device metadata response events from a specific reader ID.
func (c *BarcodeClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnDeviceMetadata(func(evt *DeviceMetadataResponse) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnAnyBarcodeEvent registers a global listener for all barcode-related events.
func (c *BarcodeClient) OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnAnyBarcodeEventForReader registers a listener for all barcode-related events originating from a specific reader ID.
func (c *BarcodeClient) OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnAnyBarcodeEvent(func(barcodeEvt any) {
		if trimmedID == "" {
			handler(barcodeEvt)
			return
		}
		if rID := extractReaderID(barcodeEvt); rID == "" || strings.EqualFold(rID, trimmedID) {
			handler(barcodeEvt)
		}
	})
}

func extractReaderID(barcodeEvt any) string {
	switch v := barcodeEvt.(type) {
	case *BarcodeReadEvent:
		return v.Data.GetReaderID()
	case *DeviceMetadataResponse:
		return v.Data.GetReaderID()
	case *BarcodeReadData:
		return v.GetReaderID()
	case *DeviceMetadataResponseData:
		return v.GetReaderID()
	default:
		return ""
	}
}

// SubscribeBarcodes creates a channel delivering stream-based BarcodeReadEvent objects from a given channel.
func (c *BarcodeClient) SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	outCh := make(chan *BarcodeReadEvent, 100)
	unsub := c.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case outCh <- evt:
			default:
				c.logger.Warn().Str("channel", evt.Channel).Msg("barcode event buffer full, dropping event")
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(outCh)
	}()

	return outCh, unsub, nil
}

// SubscribeBarcodesForReader creates a channel delivering stream-based BarcodeReadEvent objects filtered for a specific reader ID.
func (c *BarcodeClient) SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	outCh := make(chan *BarcodeReadEvent, 100)
	trimmedID := strings.TrimSpace(readerID)
	unsub := c.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		if (channel == "" || strings.EqualFold(evt.Channel, channel)) &&
			(trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID)) {
			select {
			case outCh <- evt:
			default:
				c.logger.Warn().Str("channel", evt.Channel).Str("readerId", trimmedID).Msg("barcode event buffer full, dropping event")
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(outCh)
	}()

	return outCh, unsub, nil
}

// GetDeviceMetadata queries reader hardware details and diagnostic initialization report.
func (c *BarcodeClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target...)
	if err != nil {
		return nil, err
	}
	return c.GetDeviceMetadataForReader(ctx, targetAddr, c.config.defaultReaderID)
}

// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
func (c *BarcodeClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewDeviceMetadataRequest(readerID)
	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventGetDeviceMetadata, c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		return nil, errors.Wrapf(err, "device metadata request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for device metadata request")
	}

	metaResp, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](respEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode device metadata response")
	}

	if metaResp.Data.ReaderID == "" && respEvt.Source != "" {
		metaResp.Data.ReaderID = respEvt.Source
	}

	if metaResp.RefID == "" {
		metaResp.RefID = reqID
	}

	return metaResp, nil
}

// HandleEvent parses and routes a raw wsbroker Event through the registered Barcode handlers.
func (c *BarcodeClient) HandleEvent(raw *event.RawEvent) (any, error) {
	if raw == nil || len(raw.Data) == 0 {
		return nil, nil
	}

	switch raw.Type {
	case event.TypePing, event.TypePong, event.TypeAttach, event.TypeDetach:
		return nil, nil
	}

	eventName := extractEventName(raw)
	normalized := strings.TrimSpace(strings.ToLower(eventName))

	switch normalized {
	case EventBarcodeRead, "barcode:read":
		typedEvt, err := wsbrokerclient.ToTyped[BarcodeReadData](raw)
		if err != nil {
			var altData BarcodeReadData
			if errU := json.Unmarshal(raw.Data, &altData); errU == nil {
				typedEvt = &wsbrokerclient.Event[BarcodeReadData]{
					BaseEvent: raw.BaseEvent,
					Data:      altData,
				}
			} else {
				return nil, errors.Wrap(err, "failed to parse BarcodeReadEvent")
			}
		}

		c.mu.RLock()
		handlers := make([]BarcodeReadHandler, len(c.readHandlers))
		copy(handlers, c.readHandlers)
		anyList := make([]AnyBarcodeEventHandler, len(c.anyHandlers))
		copy(anyList, c.anyHandlers)
		c.mu.RUnlock()

		for _, h := range handlers {
			h(typedEvt)
		}
		for _, h := range anyList {
			h(typedEvt)
		}
		return typedEvt, nil

	case EventDeviceMetadataResponse, "barcode:device_metadata_response":
		typedEvt, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](raw)
		if err != nil {
			var altData DeviceMetadataResponseData
			if errU := json.Unmarshal(raw.Data, &altData); errU == nil {
				typedEvt = &wsbrokerclient.Event[DeviceMetadataResponseData]{
					BaseEvent: raw.BaseEvent,
					Data:      altData,
				}
			} else {
				return nil, errors.Wrap(err, "failed to parse DeviceMetadataResponse")
			}
		}

		c.mu.RLock()
		handlers := make([]DeviceMetadataHandler, len(c.metaHandlers))
		copy(handlers, c.metaHandlers)
		anyList := make([]AnyBarcodeEventHandler, len(c.anyHandlers))
		copy(anyList, c.anyHandlers)
		c.mu.RUnlock()

		for _, h := range handlers {
			h(typedEvt)
		}
		for _, h := range anyList {
			h(typedEvt)
		}
		return typedEvt, nil

	default:
		return nil, nil
	}
}

func extractEventName(raw *event.RawEvent) string {
	rawTypeStr := string(raw.Type)
	if rawTypeStr != "" && rawTypeStr != "message" && rawTypeStr != "action" {
		return rawTypeStr
	}

	var envelope struct {
		Event string `json:"event"`
	}
	if len(raw.Data) > 0 {
		_ = json.Unmarshal(raw.Data, &envelope)
		if envelope.Event != "" {
			return envelope.Event
		}
	}

	return rawTypeStr
}
