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

func TestHub_ActiveTagsAndAFI(t *testing.T) {
	hub := NewHub(50)
	hub.SetConfiguredReader("rfid:reader:desk01")
	hub.SetActiveWindow(500 * time.Millisecond)

	// Tag 1 from configured reader
	tag1 := &rfidclient.TagReadEvent{
		UID:    "UID-001",
		ItemID: "ITEM-001",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "rfid:reader:desk01",
			Timestamp: time.Now().UnixMilli(),
		},
	}
	// Tag 2 from another reader
	tag2 := &rfidclient.TagReadEvent{
		UID:    "UID-002",
		ItemID: "ITEM-002",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "rfid:reader:other",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	hub.AddTag(tag1)
	hub.AddTag(tag2)

	active := hub.GetActiveTags()
	if len(active) != 1 {
		t.Fatalf("expected 1 active tag for configured reader, got %d", len(active))
	}
	if active[0].UID != "UID-001" {
		t.Errorf("expected UID-001, got %s", active[0].UID)
	}

	// Update AFI on active tag
	hub.UpdateActiveAFI("C2", "UID-001")
	active = hub.GetActiveTags()
	if len(active) != 1 || active[0].AFI != "C2" || !active[0].AFIUnsecured {
		t.Errorf("expected active tag AFI to be updated to C2 unsecured, got %+v", active[0])
	}

	// Test ClearActive
	hub.ClearActive("rfid:reader:desk01")
	if len(hub.GetActiveTags()) != 0 {
		t.Errorf("expected 0 active tags after ClearActive")
	}

	// Test Active Window Expiration
	oldTag := &rfidclient.TagReadEvent{
		UID:    "UID-OLD",
		ItemID: "ITEM-OLD",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "rfid:reader:desk01",
			Timestamp: time.Now().Add(-1 * time.Second).UnixMilli(),
		},
	}
	hub.AddTag(oldTag)
	// Tag is older than 500ms activeWindow, so GetActiveTags should prune it
	if len(hub.GetActiveTags()) != 0 {
		t.Errorf("expected expired tag to be pruned from active tags")
	}
}

func TestHub_SequentialTagsAccumulation(t *testing.T) {
	hub := NewHub(50)
	hub.SetConfiguredReader("reader-1")

	// Default activeWindow is 0 (no time-based expiration)
	if hub.GetActiveWindow() != 0 {
		t.Fatalf("expected default activeWindow 0, got %v", hub.GetActiveWindow())
	}

	// Add tag 1
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "UID-1",
		ItemID: "ITEM-1",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-1",
			Timestamp: time.Now().Add(-10 * time.Second).UnixMilli(),
		},
	})

	// Add tag 2 sequentially later
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "UID-2",
		ItemID: "ITEM-2",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-1",
			Timestamp: time.Now().Add(-5 * time.Second).UnixMilli(),
		},
	})

	// Add tag 3 sequentially later
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "UID-3",
		ItemID: "ITEM-3",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-1",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	active := hub.GetActiveTags()
	if len(active) != 3 {
		t.Fatalf("expected 3 accumulated active tags, got %d", len(active))
	}
	if active[0].ItemID != "ITEM-1" || active[1].ItemID != "ITEM-2" || active[2].ItemID != "ITEM-3" {
		t.Errorf("unexpected active tags ordering: %+v", active)
	}

	// Tags cleared / removed message clears the active list
	hub.ClearActive("reader-1")
	if len(hub.GetActiveTags()) != 0 {
		t.Errorf("expected 0 active tags after ClearActive")
	}
}

func TestHub_MatchReader(t *testing.T) {
	cases := []struct {
		tagReader    string
		targetReader string
		expected     bool
	}{
		{"", "", true},
		{"rfid:reader:01", "", true},
		{"rfid:reader:01", "*", true},
		{"rfid:reader:01", "all", true},
		{"rfid:reader:01", "rfid:reader:01", true},
		{"wsbroker:client:rfid:reader:01", "rfid:reader:01", true},
		{"rfid:reader:01", "wsbroker:client:rfid:reader:01", true},
		{"reader-01", "reader-01", true},
		{"reader-01", "reader-02", false},
	}

	for _, tc := range cases {
		got := MatchReader(tc.tagReader, tc.targetReader)
		if got != tc.expected {
			t.Errorf("MatchReader(%q, %q) = %v; want %v", tc.tagReader, tc.targetReader, got, tc.expected)
		}
	}
}

func TestHub_SetTagAFIResult(t *testing.T) {
	hub := NewHub(10)
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "TAG-001",
		ItemID: "BOOK-001",
		AFI:    "07",
	})
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "TAG-002",
		ItemID: "BOOK-002",
		AFI:    "07",
	})

	// Set success on TAG-001
	hub.SetTagAFIResult("TAG-001", "C2", true, "")
	// Set error on TAG-002
	hub.SetTagAFIResult("TAG-002", "C2", false, "Antenna communication failure")

	active := hub.GetActiveTags()
	if len(active) != 2 {
		t.Fatalf("expected 2 active tags, got %d", len(active))
	}

	var item1, item2 *TagItem
	for _, item := range active {
		if item.UID == "TAG-001" {
			item1 = item
		}
		if item.UID == "TAG-002" {
			item2 = item
		}
	}

	if item1 == nil || item2 == nil {
		t.Fatalf("could not find both active tags: item1=%v, item2=%v", item1, item2)
	}

	if item1.AFI != "C2" || item1.AFIWriteStatus != "success" || item1.AFIWriteError != "" {
		t.Errorf("unexpected status for TAG-001: AFI=%s, Status=%s, Error=%s", item1.AFI, item1.AFIWriteStatus, item1.AFIWriteError)
	}

	if item2.AFI != "07" || item2.AFIWriteStatus != "error" || item2.AFIWriteError != "Antenna communication failure" {
		t.Errorf("unexpected status for TAG-002: AFI=%s, Status=%s, Error=%s", item2.AFI, item2.AFIWriteStatus, item2.AFIWriteError)
	}
}

func TestHub_AFIWriteStatusLifecycle(t *testing.T) {
	hub := NewHub(10)
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "TAG-001",
		ItemID: "BOOK-001",
		AFI:    "07",
	})

	// Mark pending
	hub.SetTagAFIPending("TAG-001")
	tags := hub.GetActiveTags()
	if len(tags) != 1 || !tags[0].AFIWritePending || tags[0].AFIWriteStatus != "pending" {
		t.Fatalf("expected tag to be pending, got %+v", tags[0])
	}

	// Update with success
	hub.SetTagAFIResult("TAG-001", "C2", true, "")
	tags = hub.GetActiveTags()
	if tags[0].AFIWritePending || tags[0].AFIWriteStatus != "success" || tags[0].AFI != "C2" {
		t.Fatalf("expected tag to be success, got %+v", tags[0])
	}

	// Re-reading tag on reader pad preserves write status
	hub.AddTag(&rfidclient.TagReadEvent{
		UID:    "TAG-001",
		ItemID: "BOOK-001",
		AFI:    "C2",
	})
	tags = hub.GetActiveTags()
	if tags[0].AFIWriteStatus != "success" {
		t.Fatalf("expected write status to be preserved on re-scan, got %s", tags[0].AFIWriteStatus)
	}

	// Clear write status
	hub.ClearActiveAFIWriteStatus("TAG-001")
	tags = hub.GetActiveTags()
	if tags[0].AFIWriteStatus != "" || tags[0].AFIWriteError != "" {
		t.Fatalf("expected write status to be cleared, got %+v", tags[0])
	}
}

func TestHub_ChannelFiltering(t *testing.T) {
	hub := NewHub(10)

	tagA := &rfidclient.TagReadEvent{
		UID:    "UID-CH-A",
		ItemID: "ITEM-A",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Channel:   "channel-a",
			Timestamp: time.Now().UnixMilli(),
		},
	}
	tagB := &rfidclient.TagReadEvent{
		UID:    "UID-CH-B",
		ItemID: "ITEM-B",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Channel:   "channel-b",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	hub.AddTag(tagA)
	hub.AddTag(tagB)

	// Test GetTags with channel filtering
	allTags := hub.GetTags()
	if len(allTags) != 2 {
		t.Fatalf("expected 2 total tags, got %d", len(allTags))
	}

	tagsA := hub.GetTags("channel-a")
	if len(tagsA) != 1 || tagsA[0].UID != "UID-CH-A" {
		t.Fatalf("expected 1 tag for channel-a (UID-CH-A), got %v", tagsA)
	}

	tagsB := hub.GetTags("channel-b")
	if len(tagsB) != 1 || tagsB[0].UID != "UID-CH-B" {
		t.Fatalf("expected 1 tag for channel-b (UID-CH-B), got %v", tagsB)
	}

	// Test Count with channel filtering
	if hub.Count("channel-a") != 1 || hub.Count("channel-b") != 1 || hub.Count() != 2 {
		t.Errorf("unexpected counts: a=%d, b=%d, all=%d", hub.Count("channel-a"), hub.Count("channel-b"), hub.Count())
	}

	// Test GetActiveTagsForChannel
	activeA := hub.GetActiveTagsForChannel("channel-a")
	if len(activeA) != 1 || activeA[0].UID != "UID-CH-A" {
		t.Fatalf("expected 1 active tag for channel-a, got %v", activeA)
	}

	activeB := hub.GetActiveTagsForChannel("channel-b")
	if len(activeB) != 1 || activeB[0].UID != "UID-CH-B" {
		t.Fatalf("expected 1 active tag for channel-b, got %v", activeB)
	}

	// Test ClearForChannel
	hub.ClearForChannel("channel-a")
	if len(hub.GetTags("channel-a")) != 0 {
		t.Errorf("expected 0 tags for channel-a after ClearForChannel")
	}
	if len(hub.GetTags("channel-b")) != 1 {
		t.Errorf("expected 1 tag for channel-b to remain after ClearForChannel(channel-a)")
	}
}

func TestHub_SSEChannelRouting(t *testing.T) {
	hub := NewHub(10)

	subA := hub.Subscribe("channel-a")
	defer hub.Unsubscribe(subA)

	subB := hub.Subscribe("channel-b")
	defer hub.Unsubscribe(subB)

	subGlobal := hub.Subscribe("")
	defer hub.Unsubscribe(subGlobal)

	// Broadcast message targeting channel-a
	msgA := &SSEMessage{Event: "test_event", Data: "for_a"}
	hub.Broadcast(msgA, "channel-a")

	select {
	case received := <-subA:
		if received.Data != "for_a" {
			t.Errorf("unexpected data on subA: %s", received.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message on subA")
	}

	select {
	case received := <-subGlobal:
		if received.Data != "for_a" {
			t.Errorf("unexpected data on subGlobal: %s", received.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Errorf("timeout waiting for message on subGlobal")
	}

	select {
	case <-subB:
		t.Errorf("subB should NOT receive message broadcast to channel-a")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}
