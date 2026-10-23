package desk

import (
	"context"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

func TestHub_AddTagAndGetTags(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(3)
	hub.Start(ctx)
	defer hub.Close()

	if count := hub.Count(); count != 0 {
		t.Fatalf("expected initial count 0, got %d", count)
	}

	parts := 2
	partNo := 1
	tag1 := &rfidclient.TagReadEvent{
		UID:     "E004015011223344",
		ItemID:  "ITEM-001",
		ISIL:    "CH-000001-1",
		Country: "CH",
		CRCOk:   true,
		Parts:   &parts,
		PartNo:  &partNo,
		AFI:     "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "rfid:reader:01",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	item1 := hub.AddTag(tag1)
	if item1.UID != "E004015011223344" || item1.ItemID != "ITEM-001" {
		t.Errorf("unexpected item fields: %+v", item1)
	}
	if !item1.IsMultiPart || item1.Parts != 2 || item1.PartNo != 1 {
		t.Errorf("expected multi-part item: %+v", item1)
	}
	if item1.AFI != "07" {
		t.Errorf("expected AFI '07', got %s", item1.AFI)
	}
	if item1.ClientID != "rfid:reader:01" {
		t.Errorf("expected ClientID 'rfid:reader:01', got %s", item1.ClientID)
	}

	if count := hub.Count(); count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	tag2 := &rfidclient.TagReadEvent{UID: "UID-2", ItemID: "ITEM-2", Timestamp: time.Now().UnixMilli()}
	tag3 := &rfidclient.TagReadEvent{UID: "UID-3", ItemID: "ITEM-3", Timestamp: time.Now().UnixMilli()}
	tag4 := &rfidclient.TagReadEvent{UID: "UID-4", ItemID: "ITEM-4", Timestamp: time.Now().UnixMilli()}

	hub.AddTag(tag2)
	hub.AddTag(tag3)
	hub.AddTag(tag4)

	// Since maxTags is 3, count should stay 3 and tag1 should be evicted
	tags := hub.GetTags()
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags after eviction, got %d", len(tags))
	}
	if tags[0].UID != "UID-4" {
		t.Errorf("expected newest tag at front, got %s", tags[0].UID)
	}

	hub.Clear()
	if count := hub.Count(); count != 0 {
		t.Errorf("expected count 0 after clear, got %d", count)
	}
}

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(10)
	hub.Start(ctx)
	defer hub.Close()

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	msg := &SSEMessage{
		Event: "tag_read",
		Data:  "<tr><td>Test</td></tr>",
		ID:    "msg-1",
	}

	hub.Broadcast(msg)

	select {
	case received := <-ch:
		if received.Event != "tag_read" || received.Data != "<tr><td>Test</td></tr>" {
			t.Errorf("unexpected received message: %+v", received)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SSE message")
	}
}

func TestHub_DefaultMaxTags100(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Passing 0 or negative should default to 100
	hub := NewHub(0)
	hub.Start(ctx)
	defer hub.Close()

	for i := 0; i < 150; i++ {
		hub.AddTag(&rfidclient.TagReadEvent{
			UID:       "UID-" + string(rune('A'+(i%26))),
			ItemID:    "ITEM",
			Timestamp: time.Now().UnixMilli(),
		})
	}

	if count := hub.Count(); count != 100 {
		t.Fatalf("expected count 100, got %d", count)
	}

	tags := hub.GetTags()
	if len(tags) != 100 {
		t.Fatalf("expected exactly 100 retained tags, got %d", len(tags))
	}
}

func TestTagItem_AFISecurityStatus(t *testing.T) {
	tests := []struct {
		name           string
		afi            string
		raw            string
		wantSecured    bool
		wantUnsecured  bool
		wantStatusText string
	}{
		{
			name:           "Secured tag 07",
			afi:            "07",
			wantSecured:    true,
			wantUnsecured:  false,
			wantStatusText: "Gesichert",
		},
		{
			name:           "Secured tag single digit 7",
			afi:            "7",
			wantSecured:    true,
			wantUnsecured:  false,
			wantStatusText: "Gesichert",
		},
		{
			name:           "Unsecured tag C2",
			afi:            "C2",
			wantSecured:    false,
			wantUnsecured:  true,
			wantStatusText: "Entsichert",
		},
		{
			name:           "Unsecured tag lowercase c2",
			afi:            "c2",
			wantSecured:    false,
			wantUnsecured:  true,
			wantStatusText: "Entsichert",
		},
		{
			name:           "AFI extracted from Raw string",
			raw:            "AFI: 07 [OK]",
			wantSecured:    true,
			wantUnsecured:  false,
			wantStatusText: "Gesichert",
		},
		{
			name:           "Other arbitrary AFI",
			afi:            "01",
			wantSecured:    false,
			wantUnsecured:  false,
			wantStatusText: "",
		},
		{
			name:           "Empty AFI",
			afi:            "",
			wantSecured:    false,
			wantUnsecured:  false,
			wantStatusText: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := NewTagItem(&rfidclient.TagReadEvent{
				UID: "E00401",
				AFI: tc.afi,
				Raw: tc.raw,
			})
			if item.AFISecured != tc.wantSecured {
				t.Errorf("AFISecured = %v, want %v", item.AFISecured, tc.wantSecured)
			}
			if item.AFIUnsecured != tc.wantUnsecured {
				t.Errorf("AFIUnsecured = %v, want %v", item.AFIUnsecured, tc.wantUnsecured)
			}
			if item.AFISecurityStatus != tc.wantStatusText {
				t.Errorf("AFISecurityStatus = %q, want %q", item.AFISecurityStatus, tc.wantStatusText)
			}
		})
	}
}
