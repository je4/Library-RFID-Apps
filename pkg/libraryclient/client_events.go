package libraryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"emperror.dev/errors"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// HandlerRegistry manages a slice of handler functions in a thread-safe manner.
type HandlerRegistry[H any] struct {
	mu       sync.RWMutex
	handlers []H
}

// Register adds a handler to the registry and returns an unsubscribe function.
func (r *HandlerRegistry[H]) Register(handler H) wsbrokerclient.UnsubscribeFunc {
	if any(handler) == nil {
		return func() {}
	}
	r.mu.Lock()
	r.handlers = append(r.handlers, handler)
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.handlers = removeHandler(r.handlers, handler)
	}
}

// Snapshot returns a shallow copy of the current handler list for safe iteration without holding locks.
func (r *HandlerRegistry[H]) Snapshot() []H {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]H(nil), r.handlers...)
}

// Len returns the count of currently registered handlers.
func (r *HandlerRegistry[H]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

func removeHandler[T any](slice []T, target T) []T {
	targetPtr := fmt.Sprintf("%p", any(target))
	for i, h := range slice {
		if fmt.Sprintf("%p", any(h)) == targetPtr {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// ReaderIdentifiable describes payload types that provide a reader identifier.
type ReaderIdentifiable interface {
	GetReaderID() string
}

func extractReaderID(evt any) string {
	if evt == nil {
		return ""
	}
	type readerGetter interface {
		GetReaderID() string
	}
	if rg, ok := evt.(readerGetter); ok {
		return rg.GetReaderID()
	}
	val := reflect.ValueOf(evt)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		field := val.FieldByName("ReaderID")
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}

// populateReaderID sets ReaderID on target payload if currently empty and source is non-empty.
func populateReaderID(data any, source string) {
	if source == "" || data == nil {
		return
	}
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		field := val.FieldByName("ReaderID")
		if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
			if field.String() == "" {
				field.SetString(source)
			}
		}
	}
}

// filterReader wraps a typed event handler with case-insensitive reader ID filtering.
func filterReader[T any, H ~func(*wsbrokerclient.Event[T])](readerID string, handler H) H {
	if any(handler) == nil {
		return func(*wsbrokerclient.Event[T]) {}
	}
	trimmedID := strings.TrimSpace(readerID)
	if trimmedID == "" {
		return handler
	}
	return func(evt *wsbrokerclient.Event[T]) {
		if strings.EqualFold(extractReaderID(&evt.Data), trimmedID) {
			handler(evt)
		}
	}
}

// filterReaderAny wraps a raw/any event handler with case-insensitive reader ID filtering.
func filterReaderAny[H ~func(any)](readerID string, handler H) H {
	if any(handler) == nil {
		return func(any) {}
	}
	trimmedID := strings.TrimSpace(readerID)
	if trimmedID == "" {
		return handler
	}
	return func(evt any) {
		if rID := extractReaderID(evt); rID == "" || strings.EqualFold(rID, trimmedID) {
			handler(evt)
		}
	}
}

// subscribeStream creates a buffered channel and lifecycle goroutine for streaming typed events.
func subscribeStream[T any, H ~func(*wsbrokerclient.Event[T])](
	ctx context.Context,
	channel string,
	register func(handler H) wsbrokerclient.UnsubscribeFunc,
	logger zLogger.ZLogger,
	streamName string,
) (<-chan *wsbrokerclient.Event[T], wsbrokerclient.UnsubscribeFunc, error) {
	evtCh := make(chan *wsbrokerclient.Event[T], 64)
	unsub := register(func(evt *wsbrokerclient.Event[T]) {
		if channel == "" || strings.EqualFold(evt.Channel, channel) {
			select {
			case evtCh <- evt:
			case <-ctx.Done():
			default:
				if logger != nil {
					logger.Warn().Msgf("%s stream channel buffer full, dropping event", streamName)
				}
			}
		}
	})

	go func() {
		<-ctx.Done()
		unsub()
		close(evtCh)
	}()

	return evtCh, unsub, nil
}

// Subscribe is a generic helper to subscribe to any event stream matching type T given a registration function.
func Subscribe[T any, H ~func(*wsbrokerclient.Event[T])](
	ctx context.Context,
	channel string,
	register func(handler H) wsbrokerclient.UnsubscribeFunc,
	logger ...zLogger.ZLogger,
) (<-chan *wsbrokerclient.Event[T], wsbrokerclient.UnsubscribeFunc, error) {
	var l zLogger.ZLogger
	if len(logger) > 0 {
		l = logger[0]
	}
	return subscribeStream(ctx, channel, register, l, fmt.Sprintf("%T", *new(T)))
}

// dispatchTyped unmarshals a raw event to a typed event, populates reader ID fallback, and dispatches to registered handlers.
func dispatchTyped[T any, H ~func(*wsbrokerclient.Event[T])](
	evt *wsbrokerclient.RawEvent,
	registry *HandlerRegistry[H],
	typeName string,
) (*wsbrokerclient.Event[T], error) {
	typed, err := wsbrokerclient.ToTyped[T](evt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %s", typeName)
	}
	populateReaderID(&typed.Data, evt.Source)

	for _, h := range registry.Snapshot() {
		h(typed)
	}
	return typed, nil
}

// RFID Event Handlers

// OnTagRead registers a listener for incoming tag read events.
func (c *LibraryClient) OnTagRead(handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	return c.tagReadHandlers.Register(handler)
}

// OnTagReadForReader registers a listener for incoming tag read events from a specific reader ID.
func (c *LibraryClient) OnTagReadForReader(readerID string, handler TagReadHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnTagRead(filterReader(readerID, handler))
}

// OnTagsCleared registers a listener for reader field cleared events.
func (c *LibraryClient) OnTagsCleared(handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	return c.clearedHandlers.Register(handler)
}

// OnTagsClearedForReader registers a listener for reader field cleared events from a specific reader ID.
func (c *LibraryClient) OnTagsClearedForReader(readerID string, handler TagsClearedHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnTagsCleared(filterReader(readerID, handler))
}

// OnDeviceMetadata registers a listener for RFID device metadata response events.
func (c *LibraryClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return c.rfidMetaHandlers.Register(handler)
}

// OnDeviceMetadataForReader registers a listener for RFID device metadata response events from a specific reader ID.
func (c *LibraryClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnDeviceMetadata(filterReader(readerID, handler))
}

// OnWriteTagResponse registers a listener for write tag response events.
func (c *LibraryClient) OnWriteTagResponse(handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return c.writeTagHandlers.Register(handler)
}

// OnWriteTagResponseForReader registers a listener for write tag response events from a specific reader ID.
func (c *LibraryClient) OnWriteTagResponseForReader(readerID string, handler WriteTagResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnWriteTagResponse(filterReader(readerID, handler))
}

// OnWriteAFIResponse registers a listener for write AFI response events.
func (c *LibraryClient) OnWriteAFIResponse(handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return c.writeAFIHandlers.Register(handler)
}

// OnWriteAFIResponseForReader registers a listener for write AFI response events from a specific reader ID.
func (c *LibraryClient) OnWriteAFIResponseForReader(readerID string, handler WriteAFIResponseHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnWriteAFIResponse(filterReader(readerID, handler))
}

// OnGateStatus registers a listener for gate status and people counter events.
func (c *LibraryClient) OnGateStatus(handler GateStatusHandler) wsbrokerclient.UnsubscribeFunc {
	return c.gateStatusHandlers.Register(handler)
}

// OnGateStatusForReader registers a listener for gate status events from a specific reader ID.
func (c *LibraryClient) OnGateStatusForReader(readerID string, handler GateStatusHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnGateStatus(filterReader(readerID, handler))
}

// OnAnyRFIDEvent registers a global listener for all RFID-related events.
func (c *LibraryClient) OnAnyRFIDEvent(handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	return c.anyRFIDHandlers.Register(handler)
}

// OnAnyRFIDEventForReader registers a listener for all RFID-related events originating from a specific reader ID.
func (c *LibraryClient) OnAnyRFIDEventForReader(readerID string, handler AnyRFIDEventHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnAnyRFIDEvent(filterReaderAny(readerID, handler))
}

// Barcode Event Handlers

// OnBarcodeRead registers a listener for incoming barcode read events.
func (c *LibraryClient) OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return c.barcodeReadHandlers.Register(handler)
}

// OnBarcodeReadForReader registers a listener for incoming barcode read events from a specific reader ID.
func (c *LibraryClient) OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnBarcodeRead(filterReader(readerID, handler))
}

// OnBarcodeDeviceMetadata registers a listener for barcode device metadata response events.
func (c *LibraryClient) OnBarcodeDeviceMetadata(handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return c.barcodeMetaHandlers.Register(handler)
}

// OnBarcodeDeviceMetadataForReader registers a listener for barcode device metadata response events from a specific reader ID.
func (c *LibraryClient) OnBarcodeDeviceMetadataForReader(readerID string, handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnBarcodeDeviceMetadata(filterReader(readerID, handler))
}

// OnAnyBarcodeEvent registers a global listener for all barcode-related events.
func (c *LibraryClient) OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return c.anyBarcodeHandlers.Register(handler)
}

// OnAnyBarcodeEventForReader registers a listener for all barcode-related events originating from a specific reader ID.
func (c *LibraryClient) OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return c.OnAnyBarcodeEvent(filterReaderAny(readerID, handler))
}

// Global Event Handler

// OnAnyEvent registers a global listener for all RFID and Barcode events.
func (c *LibraryClient) OnAnyEvent(handler AnyEventHandler) wsbrokerclient.UnsubscribeFunc {
	return c.anyHandlers.Register(handler)
}

// Streaming Subscriptions

// SubscribeTags creates a channel delivering stream-based TagReadEvent objects from a given channel.
func (c *LibraryClient) SubscribeTags(ctx context.Context, channel string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, c.OnTagRead, c.logger, "tag")
}

// SubscribeTagsForReader creates a channel delivering stream-based TagReadEvent objects filtered for a specific reader ID.
func (c *LibraryClient) SubscribeTagsForReader(ctx context.Context, channel string, readerID string) (<-chan *TagReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, func(h TagReadHandler) wsbrokerclient.UnsubscribeFunc {
		return c.OnTagReadForReader(readerID, h)
	}, c.logger, "tag")
}

// SubscribeGateStatus creates a channel delivering stream-based GateStatusEvent objects from a given channel.
func (c *LibraryClient) SubscribeGateStatus(ctx context.Context, channel string) (<-chan *GateStatusEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, c.OnGateStatus, c.logger, "gate status")
}

// SubscribeGateStatusForReader creates a channel delivering stream-based GateStatusEvent objects filtered for a specific reader ID.
func (c *LibraryClient) SubscribeGateStatusForReader(ctx context.Context, channel string, readerID string) (<-chan *GateStatusEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, func(h GateStatusHandler) wsbrokerclient.UnsubscribeFunc {
		return c.OnGateStatusForReader(readerID, h)
	}, c.logger, "gate status")
}

// SubscribeBarcodes creates a channel delivering stream-based BarcodeReadEvent objects from a given channel.
func (c *LibraryClient) SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, c.OnBarcodeRead, c.logger, "barcode")
}

// SubscribeBarcodesForReader creates a channel delivering stream-based BarcodeReadEvent objects filtered for a specific reader ID.
func (c *LibraryClient) SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return subscribeStream(ctx, channel, func(h BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
		return c.OnBarcodeReadForReader(readerID, h)
	}, c.logger, "barcode")
}

// HandleEvent parses and routes a raw wsbroker Event through registered handlers.
func (c *LibraryClient) HandleEvent(evt *wsbrokerclient.RawEvent) (any, error) {
	if evt == nil {
		return nil, errors.New("cannot handle nil event")
	}

	var dispatchData any
	var isRFID, isBarcode bool

	switch evt.Type {
	case event.EventType(EventTagRead):
		typed, err := dispatchTyped(evt, &c.tagReadHandlers, "TagReadEvent")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventTagsCleared):
		typed, err := dispatchTyped(evt, &c.clearedHandlers, "TagsClearedEvent")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventDeviceMetadataResponse):
		typed, err := dispatchTyped(evt, &c.rfidMetaHandlers, "DeviceMetadataResponse")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventWriteTagResponse):
		typed, err := dispatchTyped(evt, &c.writeTagHandlers, "WriteTagResponse")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventWriteAFIResponse):
		typed, err := dispatchTyped(evt, &c.writeAFIHandlers, "WriteAFIResponse")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventGateStatus), "rfid/gate_status", "rfid/people_counter":
		typed, err := dispatchTyped(evt, &c.gateStatusHandlers, "GateStatusEvent")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isRFID = true

	case event.EventType(EventBarcodeRead):
		typed, err := dispatchTyped(evt, &c.barcodeReadHandlers, "BarcodeReadEvent")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isBarcode = true

	case event.EventType(EventBarcodeDeviceMetadataResponse):
		typed, err := dispatchTyped(evt, &c.barcodeMetaHandlers, "BarcodeDeviceMetadataResponse")
		if err != nil {
			return nil, err
		}
		dispatchData = typed
		isBarcode = true

	default:
		// Attempt dynamic unmarshaling based on payload properties
		var rawMap map[string]any
		if err := json.Unmarshal(evt.Data, &rawMap); err == nil {
			if _, hasUID := rawMap["uid"]; hasUID {
				if typed, err := dispatchTyped(evt, &c.tagReadHandlers, "TagReadEvent"); err == nil {
					dispatchData = typed
					isRFID = true
				}
			} else if _, hasBarcode := rawMap["barcode"]; hasBarcode {
				if typed, err := dispatchTyped(evt, &c.barcodeReadHandlers, "BarcodeReadEvent"); err == nil {
					dispatchData = typed
					isBarcode = true
				}
			} else if _, hasCounters := rawMap["counters"]; hasCounters {
				if typed, err := dispatchTyped(evt, &c.gateStatusHandlers, "GateStatusEvent"); err == nil {
					dispatchData = typed
					isRFID = true
				}
			} else if _, hasDiag := rawMap["diagnostics"]; hasDiag {
				if typed, err := dispatchTyped(evt, &c.gateStatusHandlers, "GateStatusEvent"); err == nil {
					dispatchData = typed
					isRFID = true
				}
			}
		}
	}

	if dispatchData == nil {
		dispatchData = evt
	}

	if isRFID {
		for _, h := range c.anyRFIDHandlers.Snapshot() {
			h(dispatchData)
		}
	} else if isBarcode {
		for _, h := range c.anyBarcodeHandlers.Snapshot() {
			h(dispatchData)
		}
	}

	for _, h := range c.anyHandlers.Snapshot() {
		h(dispatchData)
	}

	return dispatchData, nil
}
