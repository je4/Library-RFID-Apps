package libraryclient

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRFID_AFIHelpers(t *testing.T) {
	assert.Equal(t, "07", NormalizeAFI("7"))
	assert.Equal(t, "07", NormalizeAFI("07"))
	assert.Equal(t, "C2", NormalizeAFI("c2"))
	assert.Equal(t, "C2", NormalizeAFI(" C2 "))

	assert.True(t, IsAFISecured("07"))
	assert.True(t, IsAFISecured("7"))
	assert.False(t, IsAFISecured("C2"))

	assert.True(t, IsAFIUnsecured("C2"))
	assert.True(t, IsAFIUnsecured("c2"))
	assert.False(t, IsAFIUnsecured("07"))

	assert.Equal(t, "Gesichert", AFISecurityStatus("07"))
	assert.Equal(t, "Entsichert", AFISecurityStatus("C2"))
	assert.Equal(t, "", AFISecurityStatus("00"))
}

func TestRFID_TagReadTimeJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{"with time", `{"uid":"E001","time":"2026-11-12T09:59:00.123Z"}`, "2026-11-12T09:59:00.123Z"},
		{"without time", `{"uid":"E001"}`, ""},
		{"null time", `{"uid":"E001","time":null}`, ""},
		{"empty time", `{"uid":"E001","time":""}`, ""},
		{"invalid time", `{"uid":"E001","time":"invalid"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data TagReadData
			if !assert.NoError(t, json.Unmarshal([]byte(tc.payload), &data)) {
				return
			}
			if tc.want == "" {
				assert.Nil(t, data.Time)
			} else if assert.NotNil(t, data.Time) {
				want, err := time.Parse(time.RFC3339Nano, tc.want)
				assert.NoError(t, err)
				assert.True(t, data.Time.Equal(want))
			}
			encoded, err := json.Marshal(data)
			if assert.NoError(t, err) {
				if tc.want == "" {
					assert.NotContains(t, string(encoded), `"time"`)
				} else {
					assert.Contains(t, string(encoded), `"time":"`+tc.want+`"`)
				}
			}
		})
	}
}

func TestRFID_TagReadData(t *testing.T) {
	parts := 2
	partNo := 1
	usage := 1

	data := TagReadData{
		ReaderID:  "pad-01",
		UID:       "E004010000000001",
		ItemID:    "BOOK-999",
		ISIL:      "CH-000001-1",
		Country:   "CH",
		CRCOk:     true,
		Parts:     &parts,
		PartNo:    &partNo,
		UsageType: &usage,
		AFI:       "07",
	}

	assert.Equal(t, "pad-01", data.GetReaderID())
	assert.True(t, data.IsCRCValidStatus())
	assert.Equal(t, 2, data.GetParts())
	assert.Equal(t, 1, data.GetPartNo())
	assert.Equal(t, 1, data.GetUsageType())
	assert.True(t, data.IsMultiPart())
	assert.True(t, data.IsSecured())
	assert.False(t, data.IsUnsecured())
	assert.Equal(t, "Gesichert", data.SecurityStatus())
	assert.Contains(t, data.Summary(), "BOOK-999")
	assert.Contains(t, data.Summary(), "E004010000000001")
}

func TestRFID_WriteTagRequest(t *testing.T) {
	reqSingle := NewWriteTagRequest("ITEM1", "E001")
	assert.NoError(t, reqSingle.Validate())
	assert.Equal(t, "ITEM1", reqSingle.ItemID)
	assert.Equal(t, "E001", reqSingle.UID)

	reqBatch := NewWriteTagBatchRequestFromMap(map[string]string{
		"E001": "ITEM1",
		"E002": "ITEM2",
	}, "CH-001", "C2")
	assert.NoError(t, reqBatch.Validate())
	tags := reqBatch.GetAllTags()
	assert.Len(t, tags, 2)

	invalidReq := WriteTagRequest{}
	assert.Error(t, invalidReq.Validate())
}

func TestRFID_WriteAFIRequest(t *testing.T) {
	req := NewWriteAFIRequest("07", "E001")
	assert.NoError(t, req.Validate())
	assert.Equal(t, "07", req.AFI)
	assert.Equal(t, "E001", req.UID)

	batchReq := NewWriteAFIBatchRequest("c2", []string{"E001", "E002"})
	assert.NoError(t, batchReq.Validate())
	assert.Equal(t, "C2", batchReq.AFI)
	assert.Len(t, batchReq.UIDs, 2)

	invalid := WriteAFIRequest{}
	assert.Error(t, invalid.Validate())
}

func TestRFID_WriteResponses(t *testing.T) {
	tagResp := WriteTagResponseData{
		Status:   "ok",
		UID:      "E001",
		ItemID:   "ITEM1",
		ReaderID: "pad-01",
	}
	assert.True(t, tagResp.IsSuccess())
	assert.Contains(t, tagResp.Summary(), "[TAG WRITE OK]")

	afiResp := WriteAFIResponseData{
		Status:   "ok",
		AFI:      "07",
		UID:      "E001",
		ReaderID: "pad-01",
	}
	assert.True(t, afiResp.IsSuccess())
	assert.True(t, afiResp.IsSecured())
	assert.Equal(t, "Gesichert", afiResp.SecurityStatus())
	assert.Contains(t, afiResp.Summary(), "[AFI WRITE OK]")
}

func TestBarcode_Models(t *testing.T) {
	bcData := BarcodeReadData{
		ReaderID:  "scanner-01",
		Barcode:   "1234567890",
		Symbology: "Code128",
	}

	assert.Equal(t, "scanner-01", bcData.GetReaderID())
	assert.Equal(t, "1234567890", bcData.GetBarcode())
	assert.Equal(t, "1234567890", bcData.GetItemID())
	assert.Equal(t, "Code128", bcData.GetSymbology())
	assert.Contains(t, bcData.Summary(), "1234567890")

	meta := BarcodeDeviceMetadataResponseData{
		Status:     "ok",
		DeviceID:   "honeywell-01",
		DeviceType: "honeywell-1300g",
		Port:       "COM3",
		BaudRate:   9600,
	}

	assert.True(t, meta.IsSuccess())
	assert.Contains(t, meta.Summary(), "honeywell-01")
	assert.Contains(t, meta.GetDescription(), "honeywell-1300g on COM3")
}

func TestRFID_GateStatusData(t *testing.T) {
	rawJSON := `{
		"counters": [
			{
				"address": {
					"portType": 1,
					"busAddress": 1
				},
				"detector1Counter1": 10,
				"detector1Counter2": 15,
				"detector2Counter1": 0,
				"detector2Counter2": 0
			},
			{
				"address": {
					"portType": 1,
					"busAddress": 2
				},
				"detector1Counter1": 20,
				"detector1Counter2": 25,
				"detector2Counter1": 5,
				"detector2Counter2": 2
			}
		],
		"diagnostics": [
			{
				"address": {
					"portType": 1,
					"busAddress": 1
				},
				"internalError": 0,
				"rfDetect": 1
			}
		],
		"readerId": "gate-reader-01",
		"status": "ok",
		"time": "2026-11-15T12:00:00Z"
	}`

	var data GateStatusData
	err := json.Unmarshal([]byte(rawJSON), &data)
	assert.NoError(t, err)

	assert.Equal(t, "gate-reader-01", data.GetReaderID())
	assert.True(t, data.IsSuccess())
	assert.Len(t, data.Counters, 2)
	assert.Equal(t, 1, data.Counters[0].Address.PortType)
	assert.Equal(t, 1, data.Counters[0].Address.BusAddress)
	assert.Equal(t, 10, data.Counters[0].Detector1Counter1)
	assert.Equal(t, 15, data.Counters[0].Detector1Counter2)
	assert.Equal(t, 20, data.Counters[1].Detector1Counter1)
	assert.Equal(t, 25, data.Counters[1].Detector1Counter2)
	assert.Equal(t, 5, data.Counters[1].Detector2Counter1)
	assert.Equal(t, 2, data.Counters[1].Detector2Counter2)

	assert.Len(t, data.Diagnostics, 1)
	assert.Equal(t, 1, data.Diagnostics[0].Address.BusAddress)
	assert.Equal(t, 0, data.Diagnostics[0].InternalError)
	assert.Equal(t, 1, data.Diagnostics[0].RFDetect)

	assert.NotNil(t, data.Time)
	assert.Equal(t, 2026, data.Time.Year())

	summary := data.Summary()
	assert.Contains(t, summary, "[GATE STATUS]")
	assert.Contains(t, summary, "gate-reader-01")
	assert.Contains(t, summary, "Counters: 2")
	assert.Contains(t, summary, "c1=10, c2=15")
	assert.Contains(t, summary, "Diagnostics: 1")

	// WithReaderID check
	data.WithReaderID("gate-02")
	assert.Equal(t, "gate-02", data.GetReaderID())
}
