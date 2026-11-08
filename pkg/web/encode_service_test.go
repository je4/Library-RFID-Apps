package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/wsbrokerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeService_ValidateAndNormalize(t *testing.T) {
	hub := desk.NewHub(50)
	svc := NewEncodeService(hub, nil, "CH-000001", "07", []string{"ch1"}, nil)

	t.Run("empty payload", func(t *testing.T) {
		err := svc.ValidateAndNormalize(nil)
		assert.Error(t, err)
	})

	t.Run("missing item id on write", func(t *testing.T) {
		payload := &WriteTagRequestPayload{
			UID:    "E0040150E2A12345",
			ItemID: "",
		}
		err := svc.ValidateAndNormalize(payload)
		assert.Error(t, err)
	})

	t.Run("valid multi-part write payload", func(t *testing.T) {
		payload := &WriteTagRequestPayload{
			UID:    "e0040150e2a12345 ",
			ItemID: " 31415926535 ",
			ISIL:   "CH-001596-0",
			Parts:  3,
			PartNo: 2,
			AFI:    "07",
		}
		err := svc.ValidateAndNormalize(payload)
		require.NoError(t, err)
		assert.Equal(t, "e0040150e2a12345", payload.UID)
		assert.Equal(t, "31415926535", payload.ItemID)
		assert.Equal(t, "CH", payload.Country)
		assert.Equal(t, "001596-0", payload.ISIL)
		assert.Equal(t, 3, payload.Parts)
		assert.Equal(t, 2, payload.PartNo)
		assert.Equal(t, "07", payload.AFI)
		assert.Equal(t, 1, payload.UsageType)
	})

	t.Run("normalize afi textual values", func(t *testing.T) {
		payload1 := &WriteTagRequestPayload{
			ItemID: "12345",
			AFI:    "gesichert",
		}
		require.NoError(t, svc.ValidateAndNormalize(payload1))
		assert.Equal(t, "07", payload1.AFI)

		payload2 := &WriteTagRequestPayload{
			ItemID: "12345",
			AFI:    "entsichert",
		}
		require.NoError(t, svc.ValidateAndNormalize(payload2))
		assert.Equal(t, "C2", payload2.AFI)
	})

	t.Run("clear payload resets metadata and sets unsecured AFI", func(t *testing.T) {
		payload := &WriteTagRequestPayload{
			UID:   "E0040150E2A12345",
			Clear: true,
		}
		err := svc.ValidateAndNormalize(payload)
		require.NoError(t, err)
		assert.Equal(t, "", payload.ItemID)
		assert.Equal(t, "", payload.ISIL)
		assert.Equal(t, "", payload.Country)
		assert.Equal(t, 1, payload.Parts)
		assert.Equal(t, 1, payload.PartNo)
		assert.Equal(t, "C2", payload.AFI)
	})
}

func TestEncodeService_ExecuteWriteTag_HubSync(t *testing.T) {
	hub := desk.NewHub(50)
	svc := NewEncodeService(hub, nil, "CH-000001", "07", []string{"ch1"}, nil)

	// Add tag to hub
	event := &libraryclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Source: "ch1", Channel: "ch1"},
		Data: libraryclient.TagReadData{
			UID:      "E0040150E2A12345",
			ReaderID: "reader1",
			ItemID:   "OLD_ITEM",
			AFI:      "C2",
		},
	}
	hub.AddTag(event)

	// Execute write
	ctx := context.Background()
	payload := WriteTagRequestPayload{
		UID:    "E0040150E2A12345",
		ItemID: "NEW_BARCODE_123",
		ISIL:   "CH-001596-0",
		Parts:  2,
		PartNo: 1,
		AFI:    "07",
	}

	result, statusCode := svc.ExecuteWriteTag(ctx, "ch1", payload)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "NEW_BARCODE_123", result.ItemID)

	// Verify hub metadata updated
	active := hub.GetActiveTagsForChannel("ch1", "reader1")
	require.Len(t, active, 1)
	assert.Equal(t, "NEW_BARCODE_123", active[0].ItemID)
	assert.Equal(t, "001596-0", active[0].ISIL)
	assert.Equal(t, "CH", active[0].Country)
	assert.Equal(t, 2, active[0].Parts)
	assert.Equal(t, 1, active[0].PartNo)
	assert.Equal(t, "07", active[0].AFI)

	// Clear tag
	clearPayload := WriteTagRequestPayload{
		UID:   "E0040150E2A12345",
		Clear: true,
	}
	clearResult, clearStatus := svc.ExecuteWriteTag(ctx, "ch1", clearPayload)
	assert.Equal(t, http.StatusOK, clearStatus)
	assert.Equal(t, "success", clearResult.Status)

	activeAfterClear := hub.GetActiveTagsForChannel("ch1", "reader1")
	require.Len(t, activeAfterClear, 1)
	assert.Equal(t, "", activeAfterClear[0].ItemID)
	assert.Equal(t, "C2", activeAfterClear[0].AFI)
}

func TestEncodeService_MultipleReadersInChannel(t *testing.T) {
	hub := desk.NewHub(50)
	hub.SetConfiguredReader("reader1")
	for _, reader := range []string{"reader1", "reader2"} {
		hub.AddTag(&libraryclient.TagReadEvent{
			BaseEvent: wsbrokerclient.BaseEvent{Source: "daemon-" + reader, Channel: "ch1"},
			Data:      libraryclient.TagReadData{UID: "uid-" + reader, ReaderID: reader},
		})
	}
	svc := NewEncodeService(hub, nil, "", "", []string{"ch1"}, nil)
	source, reader, uid := svc.ResolveTarget("ch1", "uid-reader2")
	assert.Equal(t, "daemon-reader2", source)
	assert.Equal(t, "reader2", reader)
	assert.Equal(t, "uid-reader2", uid)
	ctrl := &Controller{hub: hub}
	assert.Len(t, ctrl.getLoanData("ch1").Tags, 2)
}
