package rfidclient

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"emperror.dev/errors"
	"github.com/beevik/ntp"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/client"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
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
func (f *fakeBrokerClient) Reply(ctx context.Context, originalEvt *event.Event, data any) error {
	replyEvt, err := event.NewReplyEvent(originalEvt, event.TypeMessage, data)
	if err != nil {
		return err
	}
	replyEvt.Source = f.clientName
	return f.Send(ctx, replyEvt)
}
func (f *fakeBrokerClient) ReplyType(ctx context.Context, originalEvt *event.Event, replyType event.EventType, data any) error {
	replyEvt, err := event.NewReplyEvent(originalEvt, replyType, data)
	if err != nil {
		return err
	}
	replyEvt.Source = f.clientName
	return f.Send(ctx, replyEvt)
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
	f.mu.Lock()
	f.msgHandlers = append(f.msgHandlers, handler)
	f.mu.Unlock()
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

func TestRFIDClient_AFIConvenienceMethods(t *testing.T) {
	fake := newFakeBroker("controller-app")
	var lastReq WriteAFIRequest
	var lastTarget string

	fake.requestFunc = func(ctx context.Context, evt *event.Event) (*event.Event, error) {
		lastTarget = evt.Target
		var req WriteAFIRequest
		if err := json.Unmarshal(evt.Data, &req); err != nil {
			return nil, err
		}
		lastReq = req

		respData := WriteAFIResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteAFIResponse,
			},
			Status: "ok",
			UID:    req.UID,
			AFI:    req.AFI,
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

	rfid, err := New(fake, WithDefaultTarget("default-station"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. SecureTag
	resp, err := rfid.SecureTag(ctx, "station-01", "E00401")
	if err != nil {
		t.Fatalf("SecureTag failed: %v", err)
	}
	if lastTarget != "station-01" || lastReq.UID != "E00401" || lastReq.AFI != AFISecured {
		t.Errorf("SecureTag sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}
	if !resp.IsSecured() || resp.SecurityStatus() != "Gesichert" {
		t.Errorf("expected resp.IsSecured()=true, got %+v", resp)
	}

	// 2. UnsecureTag
	resp, err = rfid.UnsecureTag(ctx, "station-02", "E00402")
	if err != nil {
		t.Fatalf("UnsecureTag failed: %v", err)
	}
	if lastTarget != "station-02" || lastReq.UID != "E00402" || lastReq.AFI != AFIUnsecured {
		t.Errorf("UnsecureTag sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}
	if !resp.IsUnsecured() || resp.SecurityStatus() != "Entsichert" {
		t.Errorf("expected resp.IsUnsecured()=true, got %+v", resp)
	}

	// 3. SetAFISecured with default target
	resp, err = rfid.SetAFISecured(ctx, "", "E00403")
	if err != nil {
		t.Fatalf("SetAFISecured failed: %v", err)
	}
	if lastTarget != "default-station" || lastReq.UID != "E00403" || lastReq.AFI != AFISecured {
		t.Errorf("SetAFISecured sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}

	// 4. SetAFIUnsecured without UID
	resp, err = rfid.SetAFIUnsecured(ctx, "station-03")
	if err != nil {
		t.Fatalf("SetAFIUnsecured failed: %v", err)
	}
	if lastTarget != "station-03" || lastReq.UID != "" || lastReq.AFI != AFIUnsecured {
		t.Errorf("SetAFIUnsecured sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
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

func TestRFIDClient_IgnoreNonMessageEvents(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	var handlerCalled bool
	rfid.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {
		handlerCalled = true
	})
	rfid.OnTagRead(func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		handlerCalled = true
	})

	// 1. Direct HandleEvent call with pong event
	pongEvt, err := wsbrokerclient.NewPongEvent("wsbroker", "rfid:desk:01", "pong")
	if err != nil {
		t.Fatalf("NewPongEvent failed: %v", err)
	}

	res, err := rfid.HandleEvent(pongEvt)
	if err != nil {
		t.Fatalf("HandleEvent should not return error for non-message event, got: %v", err)
	}
	if res != nil {
		t.Errorf("HandleEvent should return nil result for non-message event, got: %+v", res)
	}

	// 2. Direct HandleEvent call with ping event
	pingEvt, err := wsbrokerclient.NewPingEvent("rfid:desk:01")
	if err != nil {
		t.Fatalf("NewPingEvent failed: %v", err)
	}

	res, err = rfid.HandleEvent(pingEvt)
	if err != nil {
		t.Fatalf("HandleEvent should not return error for ping event, got: %v", err)
	}
	if res != nil {
		t.Errorf("HandleEvent should return nil result for ping event, got: %+v", res)
	}

	// 3. Attach event
	attachEvt, err := wsbrokerclient.NewAttachEvent("rfid:desk:01", "all")
	if err != nil {
		t.Fatalf("NewAttachEvent failed: %v", err)
	}

	res, err = rfid.HandleEvent(attachEvt)
	if err != nil {
		t.Fatalf("HandleEvent should not return error for attach event, got: %v", err)
	}
	if res != nil {
		t.Errorf("HandleEvent should return nil result for attach event, got: %+v", res)
	}

	time.Sleep(50 * time.Millisecond)
	if handlerCalled {
		t.Errorf("handler should not have been called for non-message events")
	}
}

func TestRFIDClient_HandleWriteAFIAndWriteTagEvents(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	var anyReceived []any
	var anyMu sync.Mutex
	rfid.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {
		anyMu.Lock()
		anyReceived = append(anyReceived, rfidEvt)
		anyMu.Unlock()
	})

	var afiRespReceived *WriteAFIResponse
	rfid.OnWriteAFIResponse(func(resp *WriteAFIResponse, rawEvt *wsbrokerclient.Event) {
		afiRespReceived = resp
	})

	var tagRespReceived *WriteTagResponse
	rfid.OnWriteTagResponse(func(resp *WriteTagResponse, rawEvt *wsbrokerclient.Event) {
		tagRespReceived = resp
	})

	// 1. Simulate self-echoed WriteAFIRequest
	rawReq := []byte(`{"event":"rfid:write_afi","uid":"E004015099887766","afi":"07"}`)
	evt1 := &wsbrokerclient.Event{
		Type:   event.TypeMessage,
		Source: "rfid:desk:01",
		Target: "rfid",
		Data:   rawReq,
	}
	res1, err := rfid.HandleEvent(evt1)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteAFIRequest: %v", err)
	}
	if _, ok := res1.(*WriteAFIRequest); !ok {
		t.Fatalf("expected *WriteAFIRequest, got %T", res1)
	}

	// 2. Simulate WriteAFIResponse
	rawAFIResp := []byte(`{"event":"rfid:write_afi_response","status":"ok","uid":"E004015099887766","afi":"07"}`)
	evt2 := &wsbrokerclient.Event{
		Type:   event.TypeMessage,
		Source: "rfid:reader:01",
		Target: "rfid:desk:01",
		Data:   rawAFIResp,
	}
	res2, err := rfid.HandleEvent(evt2)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteAFIResponse: %v", err)
	}
	if _, ok := res2.(*WriteAFIResponse); !ok {
		t.Fatalf("expected *WriteAFIResponse, got %T", res2)
	}

	// 3. Simulate WriteTagResponse
	rawTagResp := []byte(`{"event":"rfid:write_tag_response","status":"ok","uid":"E004015099887766","itemId":"12345"}`)
	evt3 := &wsbrokerclient.Event{
		Type:   event.TypeMessage,
		Source: "rfid:reader:01",
		Target: "rfid:desk:01",
		Data:   rawTagResp,
	}
	res3, err := rfid.HandleEvent(evt3)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteTagResponse: %v", err)
	}
	if _, ok := res3.(*WriteTagResponse); !ok {
		t.Fatalf("expected *WriteTagResponse, got %T", res3)
	}

	time.Sleep(50 * time.Millisecond)
	if afiRespReceived == nil || afiRespReceived.AFI != "07" {
		t.Errorf("expected afiRespReceived with AFI '07'")
	}
	if tagRespReceived == nil || tagRespReceived.ItemID != "12345" {
		t.Errorf("expected tagRespReceived with ItemID '12345'")
	}

	anyMu.Lock()
	count := len(anyReceived)
	anyMu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 any events, got %d", count)
	}
}

func TestRFIDClient_SendAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("default-reader"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. Send with explicit target
	req := NewWriteAFIRequest(AFISecured, "E004015099887766")
	if err := rfid.SendAFI(ctx, "rfid:reader:01", req); err != nil {
		t.Fatalf("SendAFI failed: %v", err)
	}

	fake.mu.Lock()
	lastSent := fake.lastSent
	fake.mu.Unlock()

	if lastSent == nil {
		t.Fatal("expected message to be sent")
	}
	if lastSent.Target != "rfid:reader:01" {
		t.Errorf("expected target 'rfid:reader:01', got %s", lastSent.Target)
	}

	var parsedReq WriteAFIRequest
	if err := json.Unmarshal(lastSent.Data, &parsedReq); err != nil {
		t.Fatalf("failed to unmarshal sent data: %v", err)
	}
	if parsedReq.AFI != AFISecured || parsedReq.UID != "E004015099887766" {
		t.Errorf("unexpected payload: %+v", parsedReq)
	}

	// 2. Send with default target fallback
	req2 := NewWriteAFIRequest(AFIUnsecured)
	if err := rfid.SendAFI(ctx, "", req2); err != nil {
		t.Fatalf("SendAFI with default target failed: %v", err)
	}

	fake.mu.Lock()
	lastSent2 := fake.lastSent
	fake.mu.Unlock()

	if lastSent2.Target != "default-reader" {
		t.Errorf("expected target 'default-reader', got %s", lastSent2.Target)
	}

	// 3. Validation error on empty AFI
	invalidReq := WriteAFIRequest{AFI: ""}
	if err := rfid.SendAFI(ctx, "rfid:reader:01", invalidReq); err == nil {
		t.Error("expected error for empty AFI")
	}
}

func TestRFIDClient_BroadcastAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithChannels("channel1", "channel2"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. Broadcast to specific channel
	req := NewWriteAFIRequest(AFISecured, "E004015099887766")
	if err := rfid.BroadcastAFI(ctx, "custom-channel", req); err != nil {
		t.Fatalf("BroadcastAFI failed: %v", err)
	}

	fake.mu.Lock()
	lastSent := fake.lastSent
	fake.mu.Unlock()

	if lastSent == nil {
		t.Fatal("expected message to be broadcast")
	}
	if lastSent.Target != "custom-channel" {
		t.Errorf("expected target 'custom-channel', got %s", lastSent.Target)
	}

	// 2. Broadcast without channel uses configured channels
	if err := rfid.BroadcastAFI(ctx, "", req); err != nil {
		t.Fatalf("BroadcastAFI to configured channels failed: %v", err)
	}

	// 3. Validation error on empty AFI
	invalidReq := WriteAFIRequest{AFI: ""}
	if err := rfid.BroadcastAFI(ctx, "custom-channel", invalidReq); err == nil {
		t.Error("expected error for empty AFI")
	}
}

func TestNewFromConfig_InheritsBrokerLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	cfg := wsbrokerclient.DefaultConfig()
	cfg.ClientName = "test-client"
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.Logger = &logger

	cli, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
	if cli.logger == nil {
		t.Fatal("expected non-nil logger")
	}
	cli.logger.Info().Msg("inherited test message")
	if !strings.Contains(buf.String(), "inherited test message") {
		t.Errorf("expected log buffer to contain message, got: %s", buf.String())
	}
}

func TestNewFromConfig_ExplicitWithLoggerOverrides(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	brokerLogger := zerolog.New(&buf1)
	overrideLogger := zerolog.New(&buf2)

	cfg := wsbrokerclient.DefaultConfig()
	cfg.ClientName = "test-client"
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.Logger = &brokerLogger

	cli, err := NewFromConfig(cfg, WithLogger(&overrideLogger))
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	cli.logger.Info().Msg("override message")
	if strings.Contains(buf1.String(), "override message") {
		t.Errorf("broker logger should NOT have received message")
	}
	if !strings.Contains(buf2.String(), "override message") {
		t.Errorf("expected override logger to contain message, got: %s", buf2.String())
	}
}

func TestRFIDClient_NilSafety(t *testing.T) {
	fake := newFakeBroker("controller-app")
	cli, err := New(fake, WithLogger(nil))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if cli.logger == nil {
		t.Fatal("expected logger to fallback to non-nil")
	}
	// Calling methods shouldn't panic
	_ = cli.BroadcastAFI(context.Background(), "all", NewWriteAFIRequest("07", "E00401"))
	_ = cli.SendAFI(context.Background(), "reader-01", NewWriteAFIRequest("07", "E00401"))
}

func TestRFIDClient_WriteTag_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		// Create correlated response with RefID = reqEvt.ID
		respData := WriteTagResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteTagResponse,
				RefID: reqEvt.ID,
			},
			Status: "ok",
			UID:    "E004010000001234",
			ItemID: "ITEM-WRITE-TEST",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.Event{
			ID:     "resp-evt-id-999",
			RefID:  reqEvt.ID,
			Source: "reader-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}
		return respEvt, nil
	}

	req := NewWriteTagRequest("ITEM-WRITE-TEST", "E004010000001234")
	origReqID := req.ID
	resp, err := rfid.WriteTag(context.Background(), "reader-01", req)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}
	if resp.RefID != origReqID {
		t.Errorf("resp.RefID = %q, want request ID %q", resp.RefID, origReqID)
	}
	if resp.ID != "resp-evt-id-999" {
		t.Errorf("resp.ID = %q, want resp-evt-id-999", resp.ID)
	}
}

func TestRFIDClient_WriteAFI_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		respData := WriteAFIResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteAFIResponse,
				RefID: reqEvt.ID,
			},
			Status: "ok",
			UID:    "E004010000001234",
			AFI:    "07",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.Event{
			ID:     "resp-afi-id-888",
			RefID:  reqEvt.ID,
			Source: "reader-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}
		return respEvt, nil
	}

	req := NewWriteAFIRequest("07", "E004010000001234")
	origReqID := req.ID
	resp, err := rfid.WriteAFI(context.Background(), "reader-01", req)
	if err != nil {
		t.Fatalf("WriteAFI failed: %v", err)
	}
	if resp.RefID != origReqID {
		t.Errorf("resp.RefID = %q, want request ID %q", resp.RefID, origReqID)
	}
	if resp.ID != "resp-afi-id-888" {
		t.Errorf("resp.ID = %q, want resp-afi-id-888", resp.ID)
	}
}

func TestRFIDClient_GetDeviceMetadata_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		respData := DeviceMetadataResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventDeviceMetadataResponse,
				RefID: reqEvt.ID,
			},
			Status:   "ok",
			DeviceID: "DEV-FEIG-01",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.Event{
			ID:     "meta-resp-id-777",
			RefID:  reqEvt.ID,
			Source: "reader-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}
		return respEvt, nil
	}

	resp, err := rfid.GetDeviceMetadata(context.Background(), "reader-01")
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}
	if resp.RefID == "" {
		t.Error("expected non-empty resp.RefID")
	}
	if resp.ID != "meta-resp-id-777" {
		t.Errorf("resp.ID = %q, want meta-resp-id-777", resp.ID)
	}
}

func TestRFIDClient_SendAFI_And_BroadcastAFI_IDPropagation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"), WithChannels("chan1"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	// 1. SendAFI
	reqSend := NewWriteAFIRequest("07", "E00401")
	reqSendID := reqSend.ID
	if err := rfid.SendAFI(context.Background(), "reader-01", reqSend); err != nil {
		t.Fatalf("SendAFI failed: %v", err)
	}
	fake.mu.Lock()
	sentEvt := fake.lastSent
	fake.mu.Unlock()
	if sentEvt.ID != reqSendID {
		t.Errorf("sentEvt.ID = %q, want reqSendID %q", sentEvt.ID, reqSendID)
	}

	// 2. BroadcastAFI
	reqBcast := NewWriteAFIRequest("C2", "E00402")
	reqBcastID := reqBcast.ID
	if err := rfid.BroadcastAFI(context.Background(), "chan1", reqBcast); err != nil {
		t.Fatalf("BroadcastAFI failed: %v", err)
	}
	fake.mu.Lock()
	bcastEvt := fake.lastSent
	fake.mu.Unlock()
	if bcastEvt.ID != reqBcastID {
		t.Errorf("bcastEvt.ID = %q, want reqBcastID %q", bcastEvt.ID, reqBcastID)
	}
}

func TestRFIDClient_WriteAFIBatch(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		var req WriteAFIRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(req.UIDs) != 2 || req.UIDs[0] != "TAG-1" || req.UIDs[1] != "TAG-2" {
			t.Errorf("unexpected request UIDs: %v", req.UIDs)
		}

		respData := WriteAFIResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteAFIResponse,
				RefID: reqEvt.ID,
			},
			Status:         "ok",
			UIDs:           req.UIDs,
			SuccessfulUIDs: req.UIDs,
			Results: []TagAFIResult{
				{UID: "TAG-1", Status: "ok"},
				{UID: "TAG-2", Status: "ok"},
			},
			AFI: req.AFI,
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.Event{
			ID:     "resp-batch-001",
			RefID:  reqEvt.ID,
			Source: "reader-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}
		return respEvt, nil
	}

	uids := []string{"TAG-1", "TAG-2"}
	resp, err := rfid.WriteAFIBatch(context.Background(), "reader-01", uids, "C2")
	if err != nil {
		t.Fatalf("WriteAFIBatch failed: %v", err)
	}
	if !resp.IsSuccess() {
		t.Errorf("expected success response, got %s", resp.Status)
	}
	if len(resp.SuccessfulUIDs) != 2 {
		t.Errorf("expected 2 successful UIDs, got %d", len(resp.SuccessfulUIDs))
	}
	if !resp.IsTagSuccessful("TAG-1") || !resp.IsTagSuccessful("TAG-2") {
		t.Errorf("expected TAG-1 and TAG-2 to be successful")
	}

	// Test SetAFISecured with multiple UIDs (batch)
	respSecured, err := rfid.SetAFISecured(context.Background(), "reader-01", "TAG-1", "TAG-2")
	if err != nil {
		t.Fatalf("SetAFISecured batch failed: %v", err)
	}
	if !respSecured.IsSuccess() {
		t.Errorf("expected success response from SetAFISecured batch, got %s", respSecured.Status)
	}
}

func TestRFIDClient_WriteTagBatch_And_WriteTags(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		var req WriteTagRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal write tag batch request: %v", err)
		}
		if len(req.Tags) != 2 {
			t.Errorf("expected 2 tags in request, got %d", len(req.Tags))
		}

		respData := WriteTagResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event: EventWriteTagResponse,
				RefID: reqEvt.ID,
			},
			Status:         "ok",
			SuccessfulUIDs: []string{"TAG-1", "TAG-2"},
			Results: []TagWriteResult{
				{UID: "TAG-1", ItemID: "ITEM-1", Status: "ok"},
				{UID: "TAG-2", ItemID: "ITEM-2", Status: "ok"},
			},
			Message: "2 tags written successfully",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.Event{
			ID:     "resp-tag-batch-001",
			RefID:  reqEvt.ID,
			Source: "reader-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}
		return respEvt, nil
	}

	items := []TagWriteItem{
		{UID: "TAG-1", ItemID: "ITEM-1", Parts: 2, PartNo: 1, AFI: "07"},
		{UID: "TAG-2", ItemID: "ITEM-2", Parts: 2, PartNo: 2, AFI: "07"},
	}

	// 1. WriteTagBatch
	resp, err := rfid.WriteTagBatch(context.Background(), "reader-01", items)
	if err != nil {
		t.Fatalf("WriteTagBatch failed: %v", err)
	}
	if !resp.IsSuccess() {
		t.Errorf("expected WriteTagBatch success, got %s", resp.Status)
	}
	if !resp.IsTagSuccessful("TAG-1") || !resp.IsTagSuccessful("TAG-2") {
		t.Errorf("expected TAG-1 and TAG-2 to be successful")
	}

	// 2. WriteTags (variadic)
	respVariadic, err := rfid.WriteTags(context.Background(), "reader-01", items[0], items[1])
	if err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}
	if !respVariadic.IsSuccess() {
		t.Errorf("expected WriteTags success, got %s", respVariadic.Status)
	}

	// 3. Empty batch error validation
	if _, err := rfid.WriteTagBatch(context.Background(), "reader-01", nil); err == nil {
		t.Error("expected error for empty tags list in WriteTagBatch")
	}
}

func TestRFIDClient_SendWriteTagBatch_And_BroadcastWriteTagBatch(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"), WithChannels("chan1"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	items := []TagWriteItem{
		{UID: "TAG-A", ItemID: "ITEM-A"},
		{UID: "TAG-B", ItemID: "ITEM-B"},
	}

	// 1. SendWriteTagBatch
	if err := rfid.SendWriteTagBatch(context.Background(), "reader-01", items); err != nil {
		t.Fatalf("SendWriteTagBatch failed: %v", err)
	}
	fake.mu.Lock()
	sentEvt := fake.lastSent
	fake.mu.Unlock()
	if sentEvt == nil || sentEvt.Target != "reader-01" {
		t.Errorf("expected sent event to target 'reader-01', got: %+v", sentEvt)
	}

	// 2. BroadcastWriteTagBatch
	if err := rfid.BroadcastWriteTagBatch(context.Background(), "chan1", items); err != nil {
		t.Fatalf("BroadcastWriteTagBatch failed: %v", err)
	}
	fake.mu.Lock()
	bcastEvt := fake.lastSent
	fake.mu.Unlock()
	if bcastEvt == nil || bcastEvt.Channel != "chan1" {
		t.Errorf("expected broadcast event to channel 'chan1', got: %+v", bcastEvt)
	}

	// 3. Empty batch error validation
	if err := rfid.SendWriteTagBatch(context.Background(), "reader-01", nil); err == nil {
		t.Error("expected error for empty SendWriteTagBatch")
	}
	if err := rfid.BroadcastWriteTagBatch(context.Background(), "chan1", nil); err == nil {
		t.Error("expected error for empty BroadcastWriteTagBatch")
	}
}

func TestRFIDClient_MultiReader_ForReader(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("daemon-01"), WithDefaultReaderID("default-reader"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	if rfid.DefaultReaderID() != "default-reader" {
		t.Errorf("DefaultReaderID() = %q, want 'default-reader'", rfid.DefaultReaderID())
	}

	gate1Client := rfid.ForReader("gate-01")
	if gate1Client.DefaultReaderID() != "gate-01" {
		t.Errorf("gate1Client.DefaultReaderID() = %q, want 'gate-01'", gate1Client.DefaultReaderID())
	}
	if gate1Client.BrokerClient() != fake {
		t.Error("expected BrokerClient() to match fake broker")
	}

	// 1. Test GetDeviceMetadata via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		var req DeviceMetadataRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}

		respData := DeviceMetadataResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event:    EventDeviceMetadataResponse,
				ReaderID: req.ReaderID,
				RefID:    reqEvt.ID,
			},
			Status:          "ok",
			DeviceID:        "FEIG-01",
			DeviceType:      "LRU1002",
			InitDescription: "Reader initialized for gate-01",
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.Event{
			ID:     "resp-meta-001",
			RefID:  reqEvt.ID,
			Source: "daemon-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}, nil
	}

	metaResp, err := gate1Client.GetDeviceMetadata(context.Background(), "daemon-01")
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}
	if metaResp.ReaderID != "gate-01" {
		t.Errorf("metaResp.ReaderID = %q, want 'gate-01'", metaResp.ReaderID)
	}

	// 2. Test WriteTag via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		var req WriteTagRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}
		if req.ItemID != "ITEM-456" {
			t.Errorf("expected itemId 'ITEM-456', got %q", req.ItemID)
		}

		respData := WriteTagResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event:    EventWriteTagResponse,
				ReaderID: req.ReaderID,
				RefID:    reqEvt.ID,
			},
			Status: "ok",
			UID:    req.UID,
			ItemID: req.ItemID,
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.Event{
			ID:     "resp-write-001",
			RefID:  reqEvt.ID,
			Source: "daemon-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}, nil
	}

	writeReq := NewWriteTagRequest("ITEM-456", "UID-456")
	writeResp, err := gate1Client.WriteTag(context.Background(), "daemon-01", writeReq)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}
	if writeResp.ReaderID != "gate-01" {
		t.Errorf("writeResp.ReaderID = %q, want 'gate-01'", writeResp.ReaderID)
	}

	// 3. Test WriteAFI via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.Event) (*event.Event, error) {
		var req WriteAFIRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}
		if req.AFI != "07" {
			t.Errorf("expected afi '07', got %q", req.AFI)
		}

		respData := WriteAFIResponse{
			BaseRFIDPayload: BaseRFIDPayload{
				Event:    EventWriteAFIResponse,
				ReaderID: req.ReaderID,
				RefID:    reqEvt.ID,
			},
			Status: "ok",
			UID:    req.UID,
			AFI:    req.AFI,
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.Event{
			ID:     "resp-afi-001",
			RefID:  reqEvt.ID,
			Source: "daemon-01",
			Target: "controller-app",
			Token:  reqEvt.Token,
			Type:   event.TypeMessage,
			Data:   dataBytes,
		}, nil
	}

	afiResp, err := gate1Client.SecureTag(context.Background(), "daemon-01", "UID-456")
	if err != nil {
		t.Fatalf("SecureTag failed: %v", err)
	}
	if afiResp.ReaderID != "gate-01" {
		t.Errorf("afiResp.ReaderID = %q, want 'gate-01'", afiResp.ReaderID)
	}
}

func TestRFIDClient_MultiReader_EventFiltering(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	gate1Tags := make([]*TagReadEvent, 0)
	gate2Tags := make([]*TagReadEvent, 0)
	var mu sync.Mutex

	unsub1 := rfid.OnTagReadForReader("gate-01", func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		mu.Lock()
		gate1Tags = append(gate1Tags, tag)
		mu.Unlock()
	})
	defer unsub1()

	unsub2 := rfid.OnTagReadForReader("gate-02", func(tag *TagReadEvent, rawEvt *wsbrokerclient.Event) {
		mu.Lock()
		gate2Tags = append(gate2Tags, tag)
		mu.Unlock()
	})
	defer unsub2()

	// Dispatch an event for gate-01
	evt1 := &event.Event{
		ID:     "evt-1",
		Source: "daemon-01",
		Target: "all",
		Type:   event.TypeMessage,
		Data: []byte(`{
			"event": "rfid:tag_read",
			"reader_id": "gate-01",
			"uid": "UID-G1",
			"itemId": "BOOK-G1"
		}`),
	}
	_, err = rfid.HandleEvent(evt1)
	if err != nil {
		t.Fatalf("HandleEvent evt1 failed: %v", err)
	}

	// Dispatch an event for gate-02
	evt2 := &event.Event{
		ID:     "evt-2",
		Source: "daemon-01",
		Target: "all",
		Type:   event.TypeMessage,
		Data: []byte(`{
			"event": "rfid:tag_read",
			"reader_id": "gate-02",
			"uid": "UID-G2",
			"itemId": "BOOK-G2"
		}`),
	}
	_, err = rfid.HandleEvent(evt2)
	if err != nil {
		t.Fatalf("HandleEvent evt2 failed: %v", err)
	}

	// Give goroutines time to run
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(gate1Tags) != 1 || gate1Tags[0].UID != "UID-G1" {
		t.Errorf("expected gate1 to receive 1 tag UID-G1, got: %+v", gate1Tags)
	}
	if len(gate2Tags) != 1 || gate2Tags[0].UID != "UID-G2" {
		t.Errorf("expected gate2 to receive 1 tag UID-G2, got: %+v", gate2Tags)
	}
}

func TestRFIDClient_MultiReader_SubscribeTagsForReader(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tagCh, unsub, err := rfid.SubscribeTagsForReader(ctx, "all", "gate-01")
	if err != nil {
		t.Fatalf("SubscribeTagsForReader failed: %v", err)
	}
	defer unsub()

	// Dispatch event for gate-02 (should be filtered out)
	_, _ = rfid.HandleEvent(&event.Event{
		ID:     "evt-g2",
		Source: "daemon-01",
		Target: "all",
		Type:   event.TypeMessage,
		Data: []byte(`{
			"event": "rfid:tag_read",
			"reader_id": "gate-02",
			"uid": "UID-G2",
			"itemId": "BOOK-G2"
		}`),
	})

	// Dispatch event for gate-01 (should be received)
	_, _ = rfid.HandleEvent(&event.Event{
		ID:     "evt-g1",
		Source: "daemon-01",
		Target: "all",
		Type:   event.TypeMessage,
		Data: []byte(`{
			"event": "rfid:tag_read",
			"reader_id": "gate-01",
			"uid": "UID-G1",
			"itemId": "BOOK-G1"
		}`),
	})

	select {
	case tag := <-tagCh:
		if tag.UID != "UID-G1" || tag.ReaderID != "gate-01" {
			t.Errorf("unexpected tag received: %+v", tag)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for gate-01 tag")
	}
}
