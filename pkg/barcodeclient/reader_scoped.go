package barcodeclient

import (
	"context"
	"strings"

	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

type readerScopedClient struct {
	parent   *BarcodeClient
	readerID string
}

func (r *readerScopedClient) BrokerClient() wsbrokerclient.ClientInterface {
	return r.parent.BrokerClient()
}

func (r *readerScopedClient) Connect(ctx context.Context) error {
	return r.parent.Connect(ctx)
}

func (r *readerScopedClient) ConnectSync(ctx context.Context) error {
	return r.parent.ConnectSync(ctx)
}

func (r *readerScopedClient) Close() error {
	return r.parent.Close()
}

func (r *readerScopedClient) ForReader(readerID string) Client {
	return r.parent.ForReader(readerID)
}

func (r *readerScopedClient) DefaultReaderID() string {
	if r.readerID != "" {
		return r.readerID
	}
	return r.parent.DefaultReaderID()
}

func (r *readerScopedClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	var tgt string
	if len(target) > 0 {
		tgt = target[0]
	}
	return r.parent.GetDeviceMetadataForReader(ctx, tgt, r.readerID)
}

func (r *readerScopedClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	targetReader := r.readerID
	if readerID != "" {
		targetReader = readerID
	}
	return r.parent.GetDeviceMetadataForReader(ctx, target, targetReader)
}

func (r *readerScopedClient) OnBarcodeRead(handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnBarcodeReadForReader(r.readerID, handler)
}

func (r *readerScopedClient) OnBarcodeReadForReader(readerID string, handler BarcodeReadHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnBarcodeReadForReader(readerID, handler)
}

func (r *readerScopedClient) OnDeviceMetadata(handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnDeviceMetadataForReader(r.readerID, handler)
}

func (r *readerScopedClient) OnDeviceMetadataForReader(readerID string, handler DeviceMetadataHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnDeviceMetadataForReader(readerID, handler)
}

func (r *readerScopedClient) OnAnyBarcodeEvent(handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnAnyBarcodeEventForReader(r.readerID, handler)
}

func (r *readerScopedClient) OnAnyBarcodeEventForReader(readerID string, handler AnyBarcodeEventHandler) wsbrokerclient.UnsubscribeFunc {
	return r.parent.OnAnyBarcodeEventForReader(readerID, handler)
}

func (r *readerScopedClient) SubscribeBarcodes(ctx context.Context, channel string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	return r.parent.SubscribeBarcodesForReader(ctx, channel, r.readerID)
}

func (r *readerScopedClient) SubscribeBarcodesForReader(ctx context.Context, channel string, readerID string) (<-chan *BarcodeReadEvent, wsbrokerclient.UnsubscribeFunc, error) {
	targetReader := r.readerID
	if strings.TrimSpace(readerID) != "" {
		targetReader = readerID
	}
	return r.parent.SubscribeBarcodesForReader(ctx, channel, targetReader)
}

func (r *readerScopedClient) HandleEvent(evt *event.RawEvent) (any, error) {
	return r.parent.HandleEvent(evt)
}
