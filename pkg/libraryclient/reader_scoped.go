package libraryclient

import (
	"context"
	"strings"

	"github.com/je4/wsbrokerclient"
)

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
	if s.readerID != "" {
		return s.readerID
	}
	return s.parent.DefaultReaderID()
}

// RFID Methods

func (s *readerScopedClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	var targetAddr string
	if len(target) > 0 {
		targetAddr = target[0]
	}
	return s.parent.GetDeviceMetadataForReader(ctx, targetAddr, s.readerID)
}

func (s *readerScopedClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	targetReader := s.readerID
	if readerID != "" {
		targetReader = readerID
	}
	return s.parent.GetDeviceMetadataForReader(ctx, target, targetReader)
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
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.writeTagBatchStreamWithReader(ctx, target, tags, s.readerID, expectedResults)
	}
	return s.parent.WriteTagBatchStream(ctx, target, tags, expectedResults)
}

func (s *readerScopedClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return s.WriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.sendWriteTagBatchWithReader(ctx, target, tags, s.readerID)
	}
	return s.parent.SendWriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.broadcastWriteTagBatchWithReader(ctx, channel, tags, s.readerID)
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
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.writeAFIBatchStreamWithReader(ctx, target, req, s.readerID, expectedResults)
	}
	return s.parent.WriteAFIBatchStream(ctx, target, req, expectedResults)
}

func (s *readerScopedClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := WriteAFIRequest{
		ReaderID: s.readerID,
		AFI:      AFISecured,
	}
	if len(uid) == 1 {
		req.UID = strings.TrimSpace(uid[0])
	} else if len(uid) > 1 {
		req.UIDs = uid
	}
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := WriteAFIRequest{
		ReaderID: s.readerID,
		AFI:      AFIUnsecured,
	}
	if len(uid) == 1 {
		req.UID = strings.TrimSpace(uid[0])
	} else if len(uid) > 1 {
		req.UIDs = uid
	}
	return s.parent.WriteAFI(ctx, target, req)
}

func (s *readerScopedClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return s.SetAFISecured(ctx, target, uid)
}

func (s *readerScopedClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return s.SetAFIUnsecured(ctx, target, uid)
}

func (s *readerScopedClient) SendAFI(ctx context.Context, target string, req WriteAFIRequest) error {
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.sendAFIWithReader(ctx, target, req, s.readerID)
	}
	return s.parent.SendAFI(ctx, target, req)
}

func (s *readerScopedClient) BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error {
	if c, ok := s.parent.(*LibraryClient); ok {
		return c.broadcastAFIWithReader(ctx, channel, req, s.readerID)
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
	targetReader := s.readerID
	if strings.TrimSpace(readerID) != "" {
		targetReader = readerID
	}
	return s.parent.SubscribeTagsForReader(ctx, channel, targetReader)
}

// Barcode Methods

func (s *readerScopedClient) GetBarcodeDeviceMetadata(ctx context.Context, target ...string) (*BarcodeDeviceMetadataResponse, error) {
	var tgt string
	if len(target) > 0 {
		tgt = target[0]
	}
	return s.parent.GetBarcodeDeviceMetadataForReader(ctx, tgt, s.readerID)
}

func (s *readerScopedClient) GetBarcodeDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*BarcodeDeviceMetadataResponse, error) {
	targetReader := s.readerID
	if readerID != "" {
		targetReader = readerID
	}
	return s.parent.GetBarcodeDeviceMetadataForReader(ctx, target, targetReader)
}

func (s *readerScopedClient) OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnBarcodeReadForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnBarcodeReadForReader(readerID, handler)
}

func (s *readerScopedClient) OnBarcodeDeviceMetadata(handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnBarcodeDeviceMetadataForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnBarcodeDeviceMetadataForReader(readerID string, handler BarcodeDeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnBarcodeDeviceMetadataForReader(readerID, handler)
}

func (s *readerScopedClient) OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnAnyBarcodeEventForReader(s.readerID, handler)
}

func (s *readerScopedClient) OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return s.parent.OnAnyBarcodeEventForReader(readerID, handler)
}

func (s *readerScopedClient) SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return s.parent.SubscribeBarcodesForReader(ctx, channel, s.readerID)
}

func (s *readerScopedClient) SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	targetReader := s.readerID
	if strings.TrimSpace(readerID) != "" {
		targetReader = readerID
	}
	return s.parent.SubscribeBarcodesForReader(ctx, channel, targetReader)
}

// Global Methods

func (s *readerScopedClient) OnAnyEvent(handler AnyEventHandler) wsbrokerclient.UnsubscribeFunc {
	if handler == nil {
		return func() {}
	}
	trimmedID := strings.TrimSpace(s.readerID)
	return s.parent.OnAnyEvent(func(evt any) {
		if trimmedID == "" {
			handler(evt)
			return
		}
		if rID := extractReaderID(evt); rID == "" || strings.EqualFold(rID, trimmedID) {
			handler(evt)
		}
	})
}

func (s *readerScopedClient) HandleEvent(evt *wsbrokerclient.RawEvent) (any, error) {
	return s.parent.HandleEvent(evt)
}
