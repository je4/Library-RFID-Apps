package rfidclient

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"emperror.dev/errors"
	"github.com/beevik/ntp"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/client"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// fakeBrokerClient implements wsbrokerclient.ClientInterface for comprehensive testing.
type fakeBrokerClient struct {
	mu           sync.RWMutex
	clientName   string
	mode         client.ConnectionMode
	channels     map[string]struct{}
	state        client.ConnectionState
	connected    bool
	msgHandlers  []client.EventHandler
	lastSent     *event.Event
	requestFunc  func(ctx context.Context, evt *event.Event) (*event.Event, error)
	attachedChan []string
}

func newFakeBroker(clientName string) *fakeBrokerClient {
	return &fakeBrokerClient{
		clientName: clientName,
		mode:       client.ModeRW,
		channels:   make(map[string]struct{}),
		state:      client.StateConnected,
		connected:  true,
	}
}

func (f *fakeBrokerClient) ClientName() string               { return f.clientName }
func (f *fakeBrokerClient) Mode() client.ConnectionMode      { return f.mode }
func (f *fakeBrokerClient) IsConnected() bool                { return f.connected }
func (f *fakeBrokerClient) GetState() client.ConnectionState { return f.state }
func (f *fakeBrokerClient) Channels() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	res := make([]string, 0, len(f.channels))
	for ch := range f.channels {
		res = append(res, ch)
	}
	return res
}
func (f *fakeBrokerClient) Connect(ctx context.Context) error     { return nil }
func (f *fakeBrokerClient) ConnectSync(ctx context.Context) error { return nil }
func (f *fakeBrokerClient) Close() error                          { return nil }

func (f *fakeBrokerClient) Send(ctx context.Context, evt *event.Event) error {
	f.mu.Lock()
	f.lastSent = evt
	f.mu.Unlock()
	return nil
}
func (f *fakeBrokerClient) SendMessage(ctx context.Context, target string, data any) error {
	evt, err := event.NewMessageEvent(f.clientName, target, data)
	if err != nil {
		return err
	}
	return f.Send(ctx, evt)
}
func (f *fakeBrokerClient) Broadcast(ctx context.Context, channel string, data any) error {
	return f.SendMessage(ctx, channel, data)
}
func (f *fakeBrokerClient) SendDirect(ctx context.Context, targetClient string, data any) error {
	return f.SendMessage(ctx, targetClient, data)
}
func (f *fakeBrokerClient) Request(ctx context.Context, evt *event.Event) (*event.Event, error) {
	f.mu.Lock()
	f.lastSent = evt
	reqFunc := f.requestFunc
	f.mu.Unlock()

	if reqFunc != nil {
		return reqFunc(ctx, evt)
	}
	return nil, errors.New("no requestFunc configured on fake broker")
}
func (f *fakeBrokerClient) Attach(ctx context.Context, channels ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range channels {
		f.channels[ch] = struct{}{}
		f.attachedChan = append(f.attachedChan, ch)
	}
	return nil
}
func (f *fakeBrokerClient) Detach(ctx context.Context, channels ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range channels {
		delete(f.channels, ch)
	}
	return nil
}
func (f *fakeBrokerClient) Subscribe(ctx context.Context, channel string) (client.Subscription, error) {
	return nil, nil
}
func (f *fakeBrokerClient) Ping(ctx context.Context) error { return nil }
func (f *fakeBrokerClient) QueryNTP(ctx context.Context, data []byte) ([]byte, error) {
	return nil, nil
}
func (f *fakeBrokerClient) NTP(ctx context.Context, opts ...ntp.QueryOptions) (*ntp.Response, error) {
	return nil, nil
}
func (f *fakeBrokerClient) ClockOffset() time.Duration { return 0 }
func (f *fakeBrokerClient) Time() time.Time            { return time.Now() }
func (f *fakeBrokerClient) SyncNTP(ctx context.Context, opts ...ntp.QueryOptions) (time.Duration, error) {
	return 0, nil
}
func (f *fakeBrokerClient) NewNTPConn(ctx context.Context) net.Conn { return nil }
func (f *fakeBrokerClient) QueryChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (f *fakeBrokerClient) ChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (f *fakeBrokerClient) OnEvent(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnMessage(handler client.EventHandler) client.UnsubscribeFunc {
	f.mu.Lock()
	f.msgHandlers = append(f.msgHandlers, handler)
	f.mu.Unlock()
	return func() {}
}
func (f *fakeBrokerClient) OnChannel(channel string, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnDirect(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnType(t event.EventType, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnConnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnDisconnect(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnReconnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnError(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}

func (f *fakeBrokerClient) simulateIncomingMessage(data any, target, source string) {
	evt, err := event.NewMessageEvent(source, target, data)
	if err != nil {
		return
	}
	f.mu.RLock()
	handlers := append([]client.EventHandler(nil), f.msgHandlers...)
	f.mu.RUnlock()

	for _, h := range handlers {
		h(evt)
	}
}

func TestRFIDClient_GetDeviceMetadata(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.Event) (*event.Event, error) {
		if evt.Target != "rfid-reader-station-01" {
			t.Errorf("expected target 'rfid-reader-station-01', got %q", evt.Target)
		}
		respData := DeviceMetadataResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event:    EventDeviceMetadataResponse,
				ClientID: "rfid-reader-station-01",
			},
			Status:          "ok",
			DeviceID:        "0052E688",
			DeviceType:      "ISC.MR102-USB",
			InitDescription: "Reader operational. RF Field ON.",
		}
		raw, _ := json.Marshal(respData)
		return &event.Event{
			Type:   event.TypeMessage,
			Source: "rfid-reader-station-01",
			Target: "controller-app",
			Token:  evt.Token,
			Data:   raw,
		}, nil
	}

	rfid, err := New(fake, WithDefaultTarget("rfid-reader-station-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	meta, err := rfid.GetDeviceMetadata(ctx)
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}

	if !meta.IsSuccess() {
		t.Errorf("expected IsSuccess=true, got false")
	}
	if meta.DeviceID != "0052E688" {
		t.Errorf("DeviceID = %q, want '0052E688'", meta.DeviceID)
	}
	if meta.DeviceType != "ISC.MR102-USB" {
		t.Errorf("DeviceType = %q, want 'ISC.MR102-USB'", meta.DeviceType)
	}
	if meta.GetDescription() != "Reader operational. RF Field ON." {
		t.Errorf("GetDescription = %q", meta.GetDescription())
	}
}

func TestRFIDClient_WriteTag(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.Event) (*event.Event, error) {
		respData := WriteTagResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteTagResponse,
			},
			Status:    "ok",
			UID:       "E004015099887766",
			ItemID:    "3012345678",
			ISIL:      "CH-000001-1",
			Country:   "CH",
			Parts:     1,
			PartNo:    1,
			UsageType: 1,
			AFI:       "07",
			Message:   "Tag written successfully",
		}
		raw, _ := json.Marshal(respData)
		return &event.Event{
			Type:   event.TypeMessage,
			Source: evt.Target,
			Target: evt.Source,
			Token:  evt.Token,
			Data:   raw,
		}, nil
	}

	rfid, err := New(fake, WithDefaultTarget("rfid-reader-station-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	req := NewWriteTagRequest("3012345678", "E004015099887766")
	req.ISIL = "CH-000001-1"
	req.Country = "CH"
	req.AFI = "07"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := rfid.WriteTag(ctx, "", req)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("expected IsSuccess=true")
	}
	if resp.ItemID != "3012345678" {
		t.Errorf("ItemID = %q, want '3012345678'", resp.ItemID)
	}
	if resp.AFI != "07" {
		t.Errorf("AFI = %q, want '07'", resp.AFI)
	}
}

func TestRFIDClient_WriteAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.Event) (*event.Event, error) {
		respData := WriteAFIResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteAFIResponse,
			},
			Status:  "ok",
			UID:     "E004015099887766",
			AFI:     "C2",
			Message: "AFI written successfully",
		}
		raw, _ := json.Marshal(respData)
		return &event.Event{
			Type:   event.TypeMessage,
			Source: evt.Target,
			Target: evt.Source,
			Token:  evt.Token,
			Data:   raw,
		}, nil
	}

	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := rfid.WriteAFIValue(ctx, "rfid-reader-station-01", "E004015099887766", "C2")
	if err != nil {
		t.Fatalf("WriteAFIValue failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Errorf("expected IsSuccess=true")
	}
	if resp.AFI != "C2" {
		t.Errorf("AFI = %q, want 'C2'", resp.AFI)
	}
}

func TestRFIDClient_EventHandlers(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	var wg sync.WaitGroup

	// 1. TagRead handler
	var receivedTag *TagReadEvent
	wg.Add(1)
	rfid.OnTagRead(func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		receivedTag = tag
		wg.Done()
	})

	// 2. TagsCleared handler
	var receivedCleared *TagsClearedEvent
	wg.Add(1)
	rfid.OnTagsCleared(func(cleared *TagsClearedEvent, rawEvt *wsbrokerclient.Event) {
		receivedCleared = cleared
		wg.Done()
	})

	// 3. AnyRFIDEvent handler
	var receivedAnyCount int
	var anyMu sync.Mutex
	rfid.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {
		anyMu.Lock()
		receivedAnyCount++
		anyMu.Unlock()
	})

	// Trigger incoming tag scan
	fake.simulateIncomingMessage(TagReadEvent{
		BaseRFIDPayload: BaseRFIDPayload{
			Event: EventTagRead,
		},
		UID:    "E0040111223344",
		ItemID: "ITEM-777",
	}, "all", "rfid-reader-station-01")

	// Trigger incoming tags cleared
	fake.simulateIncomingMessage(TagsClearedEvent{
		BaseRFIDPayload: BaseRFIDPayload{
			Event: EventTagsCleared,
		},
		Status: "empty",
	}, "all", "rfid-reader-station-01")

	wg.Wait()

	if receivedTag == nil || receivedTag.ItemID != "ITEM-777" {
		t.Errorf("expected receivedTag with ItemID 'ITEM-777', got %+v", receivedTag)
	}
	if receivedCleared == nil || receivedCleared.Status != "empty" {
		t.Errorf("expected receivedCleared with status 'empty', got %+v", receivedCleared)
	}

	time.Sleep(50 * time.Millisecond)
	anyMu.Lock()
	count := receivedAnyCount
	anyMu.Unlock()
	if count < 2 {
		t.Errorf("expected at least 2 any event callbacks, got %d", count)
	}
}

func TestRFIDClient_SubscribeTags(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, unsub, err := rfid.SubscribeTags(ctx, "intern")
	if err != nil {
		t.Fatalf("SubscribeTags failed: %v", err)
	}
	defer unsub()

	// Simulate event for matching channel "intern"
	fake.simulateIncomingMessage(TagReadEvent{
		BaseRFIDPayload: BaseRFIDPayload{
			Event: EventTagRead,
		},
		UID:    "E004019999",
		ItemID: "BOOK-101",
	}, "intern", "rfid-station-01")

	select {
	case tag := <-stream:
		if tag.ItemID != "BOOK-101" {
			t.Errorf("ItemID = %q, want 'BOOK-101'", tag.ItemID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for tag stream event")
	}
}
