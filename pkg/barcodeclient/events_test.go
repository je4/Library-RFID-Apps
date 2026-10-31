package barcodeclient_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBarcodeReadData_Helpers(t *testing.T) {
	data := barcodeclient.BarcodeReadData{
		ReaderID:   "scanner-01",
		Barcode:    "1234567890",
		ItemID:     "ITEM-1234567890",
		Symbology:  "Code128",
		Raw:        "1234567890\r",
		Timestamp:  "2026-10-31T12:00:00Z",
		DeviceID:   "honeywell-1300g",
		DeviceType: "serial",
	}

	assert.Equal(t, "scanner-01", data.GetReaderID())
	assert.Equal(t, "1234567890", data.GetBarcode())
	assert.Equal(t, "ITEM-1234567890", data.GetItemID())
	assert.Equal(t, "Code128", data.GetSymbology())
	assert.Contains(t, data.Summary(), "1234567890")
	assert.Contains(t, data.Summary(), "scanner-01")

	// Fallback ItemID
	dataNoItem := barcodeclient.BarcodeReadData{
		Barcode: "9876543210",
	}
	assert.Equal(t, "9876543210", dataNoItem.GetItemID())
	assert.Equal(t, "Unknown", dataNoItem.GetSymbology())
}

func TestDeviceMetadataResponseData_Helpers(t *testing.T) {
	resp := barcodeclient.DeviceMetadataResponseData{
		ReaderID:     "scanner-01",
		Status:       "ok",
		DeviceID:     "honeywell-1300g-01",
		DeviceType:   "honeywell-1300g",
		Port:         "COM3",
		BaudRate:     9600,
		Description:  "Honeywell 1300G-2 on COM3 @ 9600",
		TotalScans:   42,
		LastScanTime: "2026-10-31T12:00:00Z",
	}

	assert.True(t, resp.IsSuccess())
	assert.Equal(t, "scanner-01", resp.GetReaderID())
	assert.Equal(t, "Honeywell 1300G-2 on COM3 @ 9600", resp.GetDescription())
	assert.Contains(t, resp.Summary(), "honeywell-1300g-01")
	assert.Contains(t, resp.Summary(), "COM3")
}

func TestBarcodeReadData_JSON(t *testing.T) {
	data := barcodeclient.BarcodeReadData{
		ReaderID:  "scanner-01",
		Barcode:   "3012345678",
		Symbology: "EAN13",
		Timestamp: barcodeclient.FormatTime(time.Now()),
	}

	bytes, err := json.Marshal(data)
	require.NoError(t, err)

	var decoded barcodeclient.BarcodeReadData
	err = json.Unmarshal(bytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, data.ReaderID, decoded.ReaderID)
	assert.Equal(t, data.Barcode, decoded.Barcode)
	assert.Equal(t, data.Symbology, decoded.Symbology)
}
