package libraryclient

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

func extractReaderID(evt any) string {
	type readerGetter interface {
		GetReaderID() string
	}
	if rg, ok := evt.(readerGetter); ok {
		return rg.GetReaderID()
	}
	return ""
}

// OnTagRead registers a listener for incoming tag read events.
func (c *LibraryClient) OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnTagReadForReader registers a listener for incoming tag read events from a specific reader ID.
func (c *LibraryClient) OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnTagRead(func(evt *TagReadEvent) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnTagsCleared registers a listener for reader field cleared events.
func (c *LibraryClient) OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnTagsClearedForReader registers a listener for reader field cleared events from a specific reader ID.
func (c *LibraryClient) OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnTagsCleared(func(evt *TagsClearedEvent) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnDeviceMetadata registers a listener for RFID device metadata response events.
func (c *LibraryClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.rfidMetaHandlers = append(c.rfidMetaHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.rfidMetaHandlers = removeHandler(c.rfidMetaHandlers, handler)
	}
}

// OnDeviceMetadataForReader registers a listener for RFID device metadata response events from a specific reader ID.
func (c *LibraryClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnWriteTagResponse registers a listener for write tag response events.
func (c *LibraryClient) OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnWriteTagResponseForReader registers a listener for write tag response events from a specific reader ID.
func (c *LibraryClient) OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnWriteTagResponse(func(evt *WriteTagResponse) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnWriteAFIResponse registers a listener for write AFI response events.
func (c *LibraryClient) OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnWriteAFIResponseForReader registers a listener for write AFI response events from a specific reader ID.
func (c *LibraryClient) OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnWriteAFIResponse(func(evt *WriteAFIResponse) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnAnyRFIDEvent registers a global listener for all RFID-related events.
func (c *LibraryClient) OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.anyRFIDHandlers = append(c.anyRFIDHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.anyRFIDHandlers = removeHandler(c.anyRFIDHandlers, handler)
	}
}

// OnAnyRFIDEventForReader registers a listener for all RFID-related events originating from a specific reader ID.
func (c *LibraryClient) OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnAnyRFIDEvent(func(rfidEvt any) {
		if trimmedID == "" {
			handler(rfidEvt)
			return
		}
		if rID := extractReaderID(rfidEvt); rID == "" || strings.EqualFold(rID, trimmedID) {
			handler(rfidEvt)
		}
	})
}

// OnBarcodeRead registers a listener for incoming barcode read events.
func (c *LibraryClient) OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.barcodeReadHandlers = append(c.barcodeReadHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.barcodeReadHandlers = removeHandler(c.barcodeReadHandlers, handler)
	}
}

// OnBarcodeReadForReader registers a listener for incoming barcode read events from a specific reader ID.
func (c *LibraryClient) OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnBarcodeDeviceMetadata registers a listener for barcode device metadata response events.
func (c *LibraryClient) OnBarcodeDeviceMetadata(handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.barcodeMetaHandlers = append(c.barcodeMetaHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.barcodeMetaHandlers = removeHandler(c.barcodeMetaHandlers, handler)
	}
}

// OnBarcodeDeviceMetadataForReader registers a listener for barcode device metadata response events from a specific reader ID.
func (c *LibraryClient) OnBarcodeDeviceMetadataForReader(readerID string, handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(readerID)
	return c.OnBarcodeDeviceMetadata(func(evt *BarcodeDeviceMetadataResponse) {
		if trimmedID == "" || strings.EqualFold(evt.Data.GetReaderID(), trimmedID) {
			handler(evt)
		}
	})
}

// OnAnyBarcodeEvent registers a global listener for all barcode-related events.
func (c *LibraryClient) OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	c.mu.Lock()
	c.anyBarcodeHandlers = append(c.anyBarcodeHandlers, handler)
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.anyBarcodeHandlers = removeHandler(c.anyBarcodeHandlers, handler)
	}
}

// OnAnyBarcodeEventForReader registers a listener for all barcode-related events originating from a specific reader ID.
func (c *LibraryClient) OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
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

// OnAnyEvent registers a global listener for all RFID and Barcode events.
func (c *LibraryClient) OnAnyEvent(handler AnyEventHandler) wsbrokerclient.UnsubscribeFunc {
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
func (c *LibraryClient) SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	tagCh := make(chan *TagReadEvent, 64)
	unsub := c.OnTagRead(func(evt *TagReadEvent) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case tagCh <- evt:
			case <-ctx.Done():
			default:
				if c.logger != nil {
					c.logger.Warn().Msg("tag stream channel buffer full, dropping event")
				}
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(tagCh)
	}()

	return tagCh, unsub, nil
}

// SubscribeTagsForReader creates a channel delivering stream-based TagReadEvent objects filtered for a specific reader ID.
func (c *LibraryClient) SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	tagCh := make(chan *TagReadEvent, 64)
	trimmedID := strings.TrimSpace(readerID)
	unsub := c.OnTagReadForReader(trimmedID, func(evt *TagReadEvent) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case tagCh <- evt:
			case <-ctx.Done():
			default:
				if c.logger != nil {
					c.logger.Warn().Msg("tag stream channel buffer full, dropping event")
				}
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(tagCh)
	}()

	return tagCh, unsub, nil
}

// SubscribeBarcodes creates a channel delivering stream-based BarcodeReadEvent objects from a given channel.
func (c *LibraryClient) SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	bcCh := make(chan *BarcodeReadEvent, 64)
	unsub := c.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case bcCh <- evt:
			case <-ctx.Done():
			default:
				if c.logger != nil {
					c.logger.Warn().Msg("barcode stream channel buffer full, dropping event")
				}
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(bcCh)
	}()

	return bcCh, unsub, nil
}

// SubscribeBarcodesForReader creates a channel delivering stream-based BarcodeReadEvent objects filtered for a specific reader ID.
func (c *LibraryClient) SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	bcCh := make(chan *BarcodeReadEvent, 64)
	trimmedID := strings.TrimSpace(readerID)
	unsub := c.OnBarcodeReadForReader(trimmedID, func(evt *BarcodeReadEvent) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case bcCh <- evt:
			case <-ctx.Done():
			default:
				if c.logger != nil {
					c.logger.Warn().Msg("barcode stream channel buffer full, dropping event")
				}
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(bcCh)
	}()

	return bcCh, unsub, nil
}

// HandleEvent parses and routes a raw wsbroker Event through registered handlers.
func (c *LibraryClient) HandleEvent(evt *wsbrokerclient.RawEvent) (any, error) {
	if evt == nil {
		return nil, errors.New("cannot handle nil event")
	}

	c.mu.RLock()
	tagHandlers := append([]TagReadHandler(nil), c.tagReadHandlers...)
	clearedHandlers := append([]TagsClearedHandler(nil), c.clearedHandlers...)
	rfidMetaHandlers := append([]DeviceMetadataHandler(nil), c.rfidMetaHandlers...)
	writeTagHandlers := append([]WriteTagResponseHandler(nil), c.writeTagHandlers...)
	writeAFIHandlers := append([]WriteAFIResponseHandler(nil), c.writeAFIHandlers...)
	anyRFIDHandlers := append([]AnyRFIDEventHandler(nil), c.anyRFIDHandlers...)

	bcReadHandlers := append([]BarcodeReadHandler(nil), c.barcodeReadHandlers...)
	bcMetaHandlers := append([]BarcodeDeviceMetadataHandler(nil), c.barcodeMetaHandlers...)
	anyBCHandlers := append([]AnyBarcodeEventHandler(nil), c.anyBarcodeHandlers...)

	anyHandlers := append([]AnyEventHandler(nil), c.anyHandlers...)
	c.mu.RUnlock()

	var dispatchData any
	var isRFID, isBarcode bool

	switch evt.Type {
	case event.EventType(EventTagRead):
		typed, err := wsbrokerclient.ToTyped[TagReadData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagReadEvent")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range tagHandlers {
			h(typed)
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventTagsCleared):
		typed, err := wsbrokerclient.ToTyped[TagsClearedData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagsClearedEvent")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range clearedHandlers {
			h(typed)
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventDeviceMetadataResponse):
		typed, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataResponse")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range rfidMetaHandlers {
			h(typed)
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventWriteTagResponse):
		typed, err := wsbrokerclient.ToTyped[WriteTagResponseData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponse")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range writeTagHandlers {
			h(typed)
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventWriteAFIResponse):
		typed, err := wsbrokerclient.ToTyped[WriteAFIResponseData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponse")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range writeAFIHandlers {
			h(typed)
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventBarcodeRead):
		typed, err := wsbrokerclient.ToTyped[BarcodeReadData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal BarcodeReadEvent")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range bcReadHandlers {
			h(typed)
		}
		dispatchData = typed
		isBarcode = true

	case event.EventType(EventBarcodeDeviceMetadataResponse):
		typed, err := wsbrokerclient.ToTyped[BarcodeDeviceMetadataResponseData](evt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal BarcodeDeviceMetadataResponse")
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		for _, h := range bcMetaHandlers {
			h(typed)
		}
		dispatchData = typed
		isBarcode = true

	default:
		// Attempt dynamic unmarshaling based on payload properties
		var rawMap map[string]any
		if err := json.Unmarshal(evt.Data, &rawMap); err == nil {
			if _, hasUID := rawMap["uid"]; hasUID {
				if typed, err := wsbrokerclient.ToTyped[TagReadData](evt); err == nil {
					if typed.Data.ReaderID == "" && evt.Source != "" {
						typed.Data.ReaderID = evt.Source
					}
					for _, h := range tagHandlers {
						h(typed)
					}
					dispatchData = typed
					isRFID = true
				}
			} else if _, hasBarcode := rawMap["barcode"]; hasBarcode {
				if typed, err := wsbrokerclient.ToTyped[BarcodeReadData](evt); err == nil {
					if typed.Data.ReaderID == "" && evt.Source != "" {
						typed.Data.ReaderID = evt.Source
					}
					for _, h := range bcReadHandlers {
						h(typed)
					}
					dispatchData = typed
					isBarcode = true
				}
			}
		}
	}

	if dispatchData != nil {
		if isRFID {
			for _, h := range anyRFIDHandlers {
				h(dispatchData)
			}
		}
		if isBarcode {
			for _, h := range anyBCHandlers {
				h(dispatchData)
			}
		}
		for _, h := range anyHandlers {
			h(dispatchData)
		}
	}

	return dispatchData, nil
}
