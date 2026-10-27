package rfidclient

import (
	"context"

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
		return c.writeTagBatchStreamWithReader(ctx, target, tags, s.readerID, expectedResults)
	}
	return s.parent.WriteTagBatchStream(ctx, target, tags, expectedResults)
}

func (s *readerScopedClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return s.WriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	if c, ok := s.parent.(*RFIDClient); ok {
		return c.sendWriteTagBatchWithReader(ctx, target, tags, s.readerID)
	}
	return s.parent.SendWriteTagBatch(ctx, target, tags)
}

func (s *readerScopedClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	if c, ok := s.parent.(*RFIDClient); ok {
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
