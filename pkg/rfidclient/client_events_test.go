package rfidclient

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

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
	rfid.OnTagRead(func(tag *TagReadEvent) {
		receivedTag = tag
		wg.Done()
	})

	// 2. TagsCleared handler
	var receivedCleared *TagsClearedEvent
	wg.Add(1)
	rfid.OnTagsCleared(func(cleared *TagsClearedEvent) {
		receivedCleared = cleared
		wg.Done()
	})

	// 3. AnyRFIDEvent handler
	var receivedAnyCount int
	var anyMu sync.Mutex
	rfid.OnAnyRFIDEvent(func(rfidEvt any) {
		anyMu.Lock()
		receivedAnyCount++
		anyMu.Unlock()
	})

	// Trigger incoming tag scan
	fake.simulateIncomingEvent(EventTagRead, TagReadData{
		UID:    "E0040111223344",
		ItemID: "ITEM-777",
	}, "all", "rfid-reader-station-01")

	// Trigger incoming tags cleared
	fake.simulateIncomingEvent(EventTagsCleared, TagsClearedData{
		Status: "empty",
	}, "all", "rfid-reader-station-01")

	wg.Wait()

	if receivedTag == nil || receivedTag.Data.ItemID != "ITEM-777" {
		t.Errorf("expected receivedTag with ItemID 'ITEM-777', got %+v", receivedTag)
	}
	if receivedCleared == nil || receivedCleared.Data.Status != "empty" {
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
	fake.simulateIncomingEvent(EventTagRead, TagReadData{
		UID:    "E004019999",
		ItemID: "BOOK-101",
	}, "intern", "rfid-station-01")

	select {
	case tag := <-stream:
		if tag.Data.ItemID != "BOOK-101" {
			t.Errorf("ItemID = %q, want 'BOOK-101'", tag.Data.ItemID)
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
	rfid.OnAnyRFIDEvent(func(rfidEvt any) {
		handlerCalled = true
	})
	rfid.OnTagRead(func(tag *TagReadEvent) {
		handlerCalled = true
	})

	// 1. Direct HandleEvent call with pong event
	pongEvt, err := event.NewRawEvent(event.TypePong, "wsbroker", "rfid:desk:01", "pong")
	if err != nil {
		t.Fatalf("NewRawEvent pong failed: %v", err)
	}

	res, err := rfid.HandleEvent(pongEvt)
	if err != nil {
		t.Fatalf("HandleEvent should not return error for non-message event, got: %v", err)
	}
	if res != nil {
		t.Errorf("HandleEvent should return nil result for non-message event, got: %+v", res)
	}

	// 2. Direct HandleEvent call with ping event
	pingEvt, err := event.NewRawEvent(event.TypePing, "rfid:desk:01", "", nil)
	if err != nil {
		t.Fatalf("NewRawEvent ping failed: %v", err)
	}

	res, err = rfid.HandleEvent(pingEvt)
	if err != nil {
		t.Fatalf("HandleEvent should not return error for ping event, got: %v", err)
	}
	if res != nil {
		t.Errorf("HandleEvent should return nil result for ping event, got: %+v", res)
	}

	// 3. Attach event
	attachEvt, err := event.NewRawEvent(event.TypeAttach, "rfid:desk:01", "all", nil)
	if err != nil {
		t.Fatalf("NewRawEvent attach failed: %v", err)
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
	rfid.OnAnyRFIDEvent(func(rfidEvt any) {
		anyMu.Lock()
		anyReceived = append(anyReceived, rfidEvt)
		anyMu.Unlock()
	})

	var afiRespReceived *WriteAFIResponse
	rfid.OnWriteAFIResponse(func(resp *WriteAFIResponse) {
		afiRespReceived = resp
	})

	var tagRespReceived *WriteTagResponse
	rfid.OnWriteTagResponse(func(resp *WriteTagResponse) {
		tagRespReceived = resp
	})

	// 1. Simulate self-echoed WriteAFIRequest
	rawReq := []byte(`{"uid":"E004015099887766","afi":"07"}`)
	evt1 := &wsbrokerclient.RawEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Type:   EventWriteAFI,
			Source: "rfid:desk:01",
			Target: "rfid",
		},
		Data: rawReq,
	}
	res1, err := rfid.HandleEvent(evt1)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteAFIRequest: %v", err)
	}
	if _, ok := res1.(*WriteAFIRequest); !ok {
		t.Fatalf("expected *WriteAFIRequest, got %T", res1)
	}

	// 2. Simulate WriteAFIResponse
	rawAFIResp := []byte(`{"status":"ok","uid":"E004015099887766","afi":"07"}`)
	evt2 := &wsbrokerclient.RawEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Type:   EventWriteAFIResponse,
			Source: "rfid:reader:01",
			Target: "rfid:desk:01",
		},
		Data: rawAFIResp,
	}
	res2, err := rfid.HandleEvent(evt2)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteAFIResponse: %v", err)
	}
	if _, ok := res2.(*WriteAFIResponse); !ok {
		t.Fatalf("expected *WriteAFIResponse, got %T", res2)
	}

	// 3. Simulate WriteTagResponse
	rawTagResp := []byte(`{"status":"ok","uid":"E004015099887766","itemId":"12345"}`)
	evt3 := &wsbrokerclient.RawEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Type:   EventWriteTagResponse,
			Source: "rfid:reader:01",
			Target: "rfid:desk:01",
		},
		Data: rawTagResp,
	}
	res3, err := rfid.HandleEvent(evt3)
	if err != nil {
		t.Fatalf("HandleEvent failed for WriteTagResponse: %v", err)
	}
	if _, ok := res3.(*WriteTagResponse); !ok {
		t.Fatalf("expected *WriteTagResponse, got %T", res3)
	}

	time.Sleep(50 * time.Millisecond)
	if afiRespReceived == nil || afiRespReceived.Data.AFI != "07" {
		t.Errorf("expected afiRespReceived with AFI '07'")
	}
	if tagRespReceived == nil || tagRespReceived.Data.ItemID != "12345" {
		t.Errorf("expected tagRespReceived with ItemID '12345'")
	}

	anyMu.Lock()
	count := len(anyReceived)
	anyMu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 any events, got %d", count)
	}
}
