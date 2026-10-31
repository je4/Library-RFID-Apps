package barcodescanner_test

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/je4/Library-RFID-Apps/pkg/barcodescanner"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/wsbrokerclient/pkg/client"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testBrokerClient struct {
	mu          sync.RWMutex
	clientName  string
	sentEvents  []*event.RawEvent
	msgHandlers []client.EventHandler
}

func newTestBroker(clientName string) *testBrokerClient {
	return &testBrokerClient{
		clientName: clientName,
	}
}

func (t *testBrokerClient) ClientName() string                                   { return t.clientName }
func (t *testBrokerClient) Mode() client.ConnectionMode                          { return client.ModeRW }
func (t *testBrokerClient) IsConnected() bool                                    { return true }
func (t *testBrokerClient) GetState() client.ConnectionState                     { return client.StateConnected }
func (t *testBrokerClient) Channels() []string                                   { return []string{"barcode"} }
func (t *testBrokerClient) Connect(ctx context.Context) error                    { return nil }
func (t *testBrokerClient) ConnectSync(ctx context.Context) error                { return nil }
func (t *testBrokerClient) Close() error                                         { return nil }
func (t *testBrokerClient) Attach(ctx context.Context, channels ...string) error { return nil }
func (t *testBrokerClient) Detach(ctx context.Context, channels ...string) error { return nil }
func (t *testBrokerClient) Subscribe(ctx context.Context, channel string) (client.Subscription, error) {
	return nil, nil
}
func (t *testBrokerClient) Unsubscribe(ctx context.Context, channels ...string) error { return nil }
func (t *testBrokerClient) Ping(ctx context.Context) error                            { return nil }
func (t *testBrokerClient) GetRemoteAddr() net.Addr                                   { return nil }
func (t *testBrokerClient) GetLocalAddr() net.Addr                                    { return nil }
func (t *testBrokerClient) Send(ctx context.Context, evt *event.RawEvent) error {
	t.mu.Lock()
	t.sentEvents = append(t.sentEvents, evt)
	t.mu.Unlock()
	return nil
}
func (t *testBrokerClient) SendMessage(ctx context.Context, target string, data any) error {
	return nil
}
func (t *testBrokerClient) Broadcast(ctx context.Context, channel string, data any) error {
	return nil
}
func (t *testBrokerClient) SendDirect(ctx context.Context, targetClient string, data any) error {
	return nil
}
func (t *testBrokerClient) Reply(ctx context.Context, originalEvt *event.RawEvent, data any) error {
	return nil
}
func (t *testBrokerClient) ReplyType(ctx context.Context, originalEvt *event.RawEvent, replyType event.EventType, data any) error {
	return nil
}
func (t *testBrokerClient) CreateReply(original *event.RawEvent, replyType event.EventType, data any) (*event.RawEvent, error) {
	return nil, nil
}
func (t *testBrokerClient) Request(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	return nil, nil
}
func (t *testBrokerClient) Request0(ctx context.Context, evt *event.RawEvent) error { return nil }
func (t *testBrokerClient) Request1(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	return nil, nil
}
func (t *testBrokerClient) RequestN(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
	close(ch)
	return nil
}
func (t *testBrokerClient) QueryNTP(ctx context.Context, data []byte) ([]byte, error) {
	return nil, nil
}
func (t *testBrokerClient) NTP(ctx context.Context, opts ...ntp.QueryOptions) (*ntp.Response, error) {
	return nil, nil
}
func (t *testBrokerClient) ClockOffset() time.Duration { return 0 }
func (t *testBrokerClient) Time() time.Time            { return time.Now() }
func (t *testBrokerClient) SyncNTP(ctx context.Context, opts ...ntp.QueryOptions) (time.Duration, error) {
	return 0, nil
}
func (t *testBrokerClient) NewNTPConn(ctx context.Context) net.Conn { return nil }
func (t *testBrokerClient) QueryChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (t *testBrokerClient) ChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (t *testBrokerClient) OnEvent(handler client.EventHandler) client.UnsubscribeFunc {
	t.mu.Lock()
	t.msgHandlers = append(t.msgHandlers, handler)
	t.mu.Unlock()
	return func() {}
}
func (t *testBrokerClient) OnMessage(handler client.EventHandler) client.UnsubscribeFunc {
	return t.OnEvent(handler)
}
func (t *testBrokerClient) OnChannel(channel string, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (t *testBrokerClient) OnDirect(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (t *testBrokerClient) OnType(eventType event.EventType, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (t *testBrokerClient) OnConnect(callback func()) client.UnsubscribeFunc { return func() {} }
func (t *testBrokerClient) OnDisconnect(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}
func (t *testBrokerClient) OnReconnect(callback func()) client.UnsubscribeFunc  { return func() {} }
func (t *testBrokerClient) OnError(callback func(error)) client.UnsubscribeFunc { return func() {} }

func (t *testBrokerClient) emit(evt *event.RawEvent) {
	t.mu.RLock()
	handlers := append([]client.EventHandler(nil), t.msgHandlers...)
	t.mu.RUnlock()
	for _, h := range handlers {
		h(evt)
	}
}

func (t *testBrokerClient) getSent() []*event.RawEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]*event.RawEvent(nil), t.sentEvents...)
}

func TestBarcodeWsBrokerDispatcher_DispatchScan(t *testing.T) {
	reg := barcodescanner.NewBarcodeReaderRegistry()
	cfg1 := barcodescanner.ReaderConfig{
		ID:       "scanner-01",
		Channels: []string{"gate01", "barcode"},
	}
	cfg2 := barcodescanner.ReaderConfig{
		ID: "scanner-02",
		// no specific channels, falls back to global
	}
	reg.RegisterReader(barcodescanner.NewBarcodeReaderInstance(cfg1, nil))
	reg.RegisterReader(barcodescanner.NewBarcodeReaderInstance(cfg2, nil))

	mock := newTestBroker("barcode-daemon")
	dispatcher, err := barcodescanner.NewBarcodeWsBrokerDispatcher(
		"barcode-daemon",
		[]string{"global-barcode"},
		reg,
		mock,
		nil,
	)
	require.NoError(t, err)
	defer dispatcher.Close()

	// Dispatch scan for reader 1 (has dedicated channels)
	scan1 := &libraryclient.BarcodeReadData{
		ReaderID:  "scanner-01",
		Barcode:   "3012345678",
		Symbology: "Code 128 / GS1-128",
	}
	dispatcher.DispatchScan(context.Background(), scan1)

	sent := mock.getSent()
	require.Len(t, sent, 2)
	assert.Equal(t, "gate01", sent[0].Channel)
	assert.Equal(t, "barcode", sent[1].Channel)

	// Dispatch scan for reader 2 (falls back to global channels)
	scan2 := &libraryclient.BarcodeReadData{
		ReaderID:  "scanner-02",
		Barcode:   "9988776655",
		Symbology: "EAN-13",
	}
	dispatcher.DispatchScan(context.Background(), scan2)

	sent = mock.getSent()
	require.Len(t, sent, 3)
	assert.Equal(t, "global-barcode", sent[2].Channel)
}

func TestBarcodeWsBrokerDispatcher_MetadataQuery(t *testing.T) {
	reg := barcodescanner.NewBarcodeReaderRegistry()
	cfg := barcodescanner.ReaderConfig{
		ID:         "scanner-01",
		DeviceID:   "honeywell-1300g-01",
		DeviceType: "honeywell-1300g",
		Port:       "COM3",
		BaudRate:   9600,
	}
	reg.RegisterReader(barcodescanner.NewBarcodeReaderInstance(cfg, nil))

	mock := newTestBroker("barcode-daemon")
	dispatcher, err := barcodescanner.NewBarcodeWsBrokerDispatcher(
		"barcode-daemon",
		[]string{"barcode"},
		reg,
		mock,
		nil,
	)
	require.NoError(t, err)
	defer dispatcher.Close()

	reqPayload := libraryclient.BarcodeDeviceMetadataRequest{ReaderID: "scanner-01"}
	reqBytes, _ := json.Marshal(reqPayload)
	reqEvt := &event.RawEvent{
		BaseEvent: event.BaseEvent{
			ID:     "req-12345",
			Source: "ui-client",
			Target: "barcode-daemon",
			Type:   event.EventType(libraryclient.EventBarcodeGetDeviceMetadata),
		},
		Data: reqBytes,
	}

	mock.emit(reqEvt)

	sent := mock.getSent()
	require.Len(t, sent, 1)
	assert.Equal(t, "req-12345", sent[0].RefID)
	assert.Equal(t, "ui-client", sent[0].Target)

	var respData libraryclient.BarcodeDeviceMetadataResponseData
	err = json.Unmarshal(sent[0].Data, &respData)
	require.NoError(t, err)
	assert.Equal(t, "scanner-01", respData.ReaderID)
	assert.Equal(t, "honeywell-1300g-01", respData.DeviceID)
	assert.Equal(t, "COM3", respData.Port)
}
