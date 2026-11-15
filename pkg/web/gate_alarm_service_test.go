package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateAlarmService_Detection(t *testing.T) {
	gateChannels := []string{"gates", "gate-exit"}
	service := NewGateAlarmService(gateChannels, 50, nil)

	assert.True(t, service.IsGateChannel("gates"))
	assert.True(t, service.IsGateChannel("GATES"))
	assert.True(t, service.IsGateChannel("gate-exit"))
	assert.False(t, service.IsGateChannel("mediathek01"))
	assert.False(t, service.IsGateChannel("desk"))

	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.UTC)

	// 1. Tag on desk channel (not gate) with AFI 07 -> No gate alarm
	tagDesk := &libraryclient.TagReadEvent{
		Channel: "mediathek01",
		Data: libraryclient.TagReadData{
			UID:      "E004010001",
			ItemID:   "BARCODE-001",
			AFI:      "07",
			ReaderID: "desk-reader",
			Time:     &now,
		},
	}
	item, payload, isAlarm := service.ProcessTagRead(tagDesk)
	assert.False(t, isAlarm)
	assert.Nil(t, item)
	assert.Nil(t, payload)
	assert.Empty(t, service.GetAlarms())

	// 2. Tag on gate channel with unsecured AFI C2 (on loan) -> No gate alarm
	tagGateUnsecured := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010002",
			ItemID:   "BARCODE-002",
			AFI:      "C2",
			ReaderID: "gate-antenna-01",
			Time:     &now,
		},
	}
	item, payload, isAlarm = service.ProcessTagRead(tagGateUnsecured)
	assert.False(t, isAlarm)
	assert.Nil(t, item)
	assert.Nil(t, payload)
	assert.Empty(t, service.GetAlarms())

	// 3. Tag on gate channel with empty AFI -> No gate alarm
	tagGateEmptyAFI := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010003",
			ItemID:   "BARCODE-003",
			AFI:      "",
			ReaderID: "gate-antenna-01",
			Time:     &now,
		},
	}
	item, payload, isAlarm = service.ProcessTagRead(tagGateEmptyAFI)
	assert.False(t, isAlarm)
	assert.Nil(t, item)
	assert.Nil(t, payload)
	assert.Empty(t, service.GetAlarms())

	// 4. Tag on gate channel with secured AFI 07 (closed) -> GATE ALARM!
	tagGateSecured := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010004",
			ItemID:   "3133742",
			AFI:      "07",
			ReaderID: "gate-hauptausgang",
			ISIL:     "CH-001596-0",
			Time:     &now,
		},
	}
	item, payload, isAlarm = service.ProcessTagRead(tagGateSecured)
	require.True(t, isAlarm)
	require.NotNil(t, item)
	require.NotNil(t, payload)

	assert.Equal(t, "3133742", item.Barcode)
	assert.Equal(t, "E004010004", item.UID)
	assert.Equal(t, "gate-hauptausgang", item.Gate)
	assert.Equal(t, "gates", item.Channel)
	assert.Equal(t, "07", item.AFI)
	assert.Equal(t, "14:30:00", item.TimeString)
	assert.Equal(t, "CH-001596-0", item.ISIL)
	assert.Equal(t, 1, payload.TotalCount)
	assert.Len(t, payload.Items, 1)

	// 5. Multiple books triggering alarm at the gate
	now2 := now.Add(2 * time.Second)
	tagBook2 := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010005",
			ItemID:   "98765432",
			AFI:      "07",
			ReaderID: "gate-hauptausgang",
			Time:     &now2,
		},
	}
	item2, payload2, isAlarm2 := service.ProcessTagRead(tagBook2)
	require.True(t, isAlarm2)
	require.NotNil(t, item2)
	require.NotNil(t, payload2)
	assert.Equal(t, "98765432", item2.Barcode)
	assert.Equal(t, 2, payload2.TotalCount)
	assert.Len(t, payload2.Items, 2)
	assert.Equal(t, "98765432", payload2.Items[0].Barcode) // newest first
	assert.Equal(t, "3133742", payload2.Items[1].Barcode)

	now3 := now.Add(3 * time.Second)
	tagBook3 := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010006",
			ItemID:   "55443322",
			AFI:      "7", // single-digit hex "7" normalized to "07"
			ReaderID: "gate-hauptausgang",
			Time:     &now3,
		},
	}
	item3, payload3, isAlarm3 := service.ProcessTagRead(tagBook3)
	require.True(t, isAlarm3)
	require.NotNil(t, item3)
	require.NotNil(t, payload3)
	assert.Equal(t, "55443322", item3.Barcode)
	assert.Equal(t, "07", item3.AFI)
	assert.Equal(t, 3, payload3.TotalCount)
	assert.Len(t, payload3.Items, 3)

	// Verify all alarms list
	allAlarms := service.GetAlarms()
	assert.Len(t, allAlarms, 3)

	// Acknowledge one alarm
	ok := service.AcknowledgeAlarm(item2.ID)
	assert.True(t, ok)
	assert.Len(t, service.GetAlarms(), 2)

	// Clear all alarms
	service.ClearAlarms()
	assert.Empty(t, service.GetAlarms())
}

func TestController_GateAlarmAPIAndSSE(t *testing.T) {
	hub := desk.NewHub(100)
	ctrl, err := NewController(Config{
		Hub:          hub,
		Channels:     []string{"mediathek01"},
		GateChannels: []string{"gates"},
		ClientName:   "test-desk-client",
		DefaultISIL:  "CH-001596-0",
		DefaultAFI:   "07",
	})
	require.NoError(t, err)

	// 1. Initial alarms empty
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/gate-alarms", nil)
	ctrl.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string          `json:"status"`
		Alarms []GateAlarmItem `json:"alarms"`
		Count  int             `json:"count"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, 0, resp.Count)

	// 2. Subscribe an SSE client to all channels
	sseChan := hub.Subscribe("")
	defer hub.Unsubscribe(sseChan)

	// 3. Trigger a gate alarm tag scan
	scanTime := time.Date(2026, 11, 15, 14, 45, 10, 0, time.UTC)
	alarmTag := &libraryclient.TagReadEvent{
		Channel: "gates",
		Data: libraryclient.TagReadData{
			UID:      "E004010099",
			ItemID:   "BOOK-9999",
			AFI:      "07",
			ReaderID: "gate-north",
			Time:     &scanTime,
		},
	}
	ctrl.HandleTagRead(alarmTag)

	// 4. Verify SSE broadcast event received
	select {
	case msg := <-sseChan:
		require.NotNil(t, msg)
		if msg.Event == "tag_read" {
			// Might receive tag_read first, look for gate_alarm next
			select {
			case msg2 := <-sseChan:
				assert.Equal(t, "gate_alarm", msg2.Event)
				var payload GateAlarmEventPayload
				err = json.Unmarshal([]byte(msg2.Data), &payload)
				require.NoError(t, err)
				assert.Equal(t, "BOOK-9999", payload.Item.Barcode)
				assert.Equal(t, "gate-north", payload.Gate)
				assert.Equal(t, "14:45:10", payload.TimeString)
			case <-time.After(1 * time.Second):
				t.Fatal("timed out waiting for gate_alarm SSE event")
			}
		} else {
			assert.Equal(t, "gate_alarm", msg.Event)
			var payload GateAlarmEventPayload
			err = json.Unmarshal([]byte(msg.Data), &payload)
			require.NoError(t, err)
			assert.Equal(t, "BOOK-9999", payload.Item.Barcode)
			assert.Equal(t, "gate-north", payload.Gate)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE message")
	}

	// 5. Query /api/v1/gate-alarms
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/gate-alarms", nil)
	ctrl.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, "BOOK-9999", resp.Alarms[0].Barcode)

	// 6. Clear alarms via POST /api/v1/gate-alarms/clear
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/api/v1/gate-alarms/clear", strings.NewReader(""))
	ctrl.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 7. Verify cleared
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/v1/gate-alarms", nil)
	ctrl.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)
}
