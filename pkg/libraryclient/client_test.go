package libraryclient

import (
	"context"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLibraryClient_Initialization(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	client, err := New(fakeBroker,
		WithDefaultTarget("rfid-daemon"),
		WithDefaultReaderID("pad-01"),
		WithDefaultTimeout(10*time.Second),
		WithChannels("all", "rfid"),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	assert.Equal(t, fakeBroker, client.BrokerClient())
	assert.Equal(t, "pad-01", client.DefaultReaderID())

	err = client.Connect(context.Background())
	assert.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestLibraryClient_RFIDEventDispatch(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	client, err := New(fakeBroker)
	require.NoError(t, err)

	var tagReadReceived *TagReadEvent
	var tagsClearedReceived *TagsClearedEvent
	var rfidMetaReceived *DeviceMetadataResponse
	var writeTagReceived *WriteTagResponse
	var writeAFIReceived *WriteAFIResponse
	var anyRFIDReceived []any
	var anyEventReceived []any

	client.OnTagRead(func(evt *TagReadEvent) {
		tagReadReceived = evt
	})
	client.OnTagsCleared(func(evt *TagsClearedEvent) {
		tagsClearedReceived = evt
	})
	client.OnDeviceMetadata(func(evt *DeviceMetadataResponse) {
		rfidMetaReceived = evt
	})
	client.OnWriteTagResponse(func(evt *WriteTagResponse) {
		writeTagReceived = evt
	})
	client.OnWriteAFIResponse(func(evt *WriteAFIResponse) {
		writeAFIReceived = evt
	})
	client.OnAnyRFIDEvent(func(evt any) {
		anyRFIDReceived = append(anyRFIDReceived, evt)
	})
	client.OnAnyEvent(func(evt any) {
		anyEventReceived = append(anyEventReceived, evt)
	})

	// 1. Emit TagReadEvent
	rawTagEvt, err := wsbrokerclient.NewRawEvent(EventTagRead, "rfid-daemon", "test-client", TagReadData{
		UID:      "E004010000000001",
		ItemID:   "BOOK-12345",
		ReaderID: "pad-01",
		AFI:      "07",
		CRCOk:    true,
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawTagEvt)

	require.NotNil(t, tagReadReceived)
	assert.Equal(t, "BOOK-12345", tagReadReceived.Data.ItemID)
	assert.Equal(t, "E004010000000001", tagReadReceived.Data.UID)
	assert.True(t, tagReadReceived.Data.IsSecured())

	// 2. Emit TagsClearedEvent
	rawClearedEvt, err := wsbrokerclient.NewRawEvent(EventTagsCleared, "rfid-daemon", "test-client", TagsClearedData{
		ReaderID: "pad-01",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawClearedEvt)

	require.NotNil(t, tagsClearedReceived)
	assert.Equal(t, "pad-01", tagsClearedReceived.Data.ReaderID)

	// 3. Emit WriteTagResponse
	rawWriteTagEvt, err := wsbrokerclient.NewRawEvent(EventWriteTagResponse, "rfid-daemon", "test-client", WriteTagResponseData{
		Status: "ok",
		UID:    "E004010000000001",
		ItemID: "BOOK-12345",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawWriteTagEvt)

	require.NotNil(t, writeTagReceived)
	assert.True(t, writeTagReceived.Data.IsSuccess())

	// 4. Emit WriteAFIResponse
	rawWriteAFIEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFIResponse, "rfid-daemon", "test-client", WriteAFIResponseData{
		Status: "ok",
		AFI:    "C2",
		UID:    "E004010000000001",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawWriteAFIEvt)

	require.NotNil(t, writeAFIReceived)
	assert.True(t, writeAFIReceived.Data.IsUnsecured())

	// 5. Emit DeviceMetadataResponse
	rawMetaEvt, err := wsbrokerclient.NewRawEvent(EventDeviceMetadataResponse, "rfid-daemon", "test-client", DeviceMetadataResponseData{
		Status:   "ok",
		DeviceID: "feig-lr2500",
		ReaderID: "pad-01",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawMetaEvt)

	require.NotNil(t, rfidMetaReceived)
	assert.Equal(t, "feig-lr2500", rfidMetaReceived.Data.DeviceID)

	// Verify any listeners received all 5 RFID events
	assert.Len(t, anyRFIDReceived, 5)
	assert.Len(t, anyEventReceived, 5)
}

func TestLibraryClient_BarcodeEventDispatch(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	client, err := New(fakeBroker)
	require.NoError(t, err)

	var barcodeReadReceived *BarcodeReadEvent
	var barcodeMetaReceived *BarcodeDeviceMetadataResponse
	var anyBarcodeReceived []any
	var anyEventReceived []any

	client.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		barcodeReadReceived = evt
	})
	client.OnBarcodeDeviceMetadata(func(evt *BarcodeDeviceMetadataResponse) {
		barcodeMetaReceived = evt
	})
	client.OnAnyBarcodeEvent(func(evt any) {
		anyBarcodeReceived = append(anyBarcodeReceived, evt)
	})
	client.OnAnyEvent(func(evt any) {
		anyEventReceived = append(anyEventReceived, evt)
	})

	// 1. Emit BarcodeReadEvent
	rawBCEvt, err := wsbrokerclient.NewRawEvent(EventBarcodeRead, "barcode-daemon", "test-client", BarcodeReadData{
		Barcode:   "9783161484100",
		ItemID:    "9783161484100",
		ReaderID:  "scanner-01",
		Symbology: "EAN-13",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawBCEvt)

	require.NotNil(t, barcodeReadReceived)
	assert.Equal(t, "9783161484100", barcodeReadReceived.Data.Barcode)
	assert.Equal(t, "scanner-01", barcodeReadReceived.Data.ReaderID)

	// 2. Emit Barcode Device Metadata
	rawBCMetaEvt, err := wsbrokerclient.NewRawEvent(EventBarcodeDeviceMetadataResponse, "barcode-daemon", "test-client", BarcodeDeviceMetadataResponseData{
		Status:   "ok",
		DeviceID: "honeywell-1300g",
		Port:     "COM3",
	})
	require.NoError(t, err)
	fakeBroker.emitIncomingEvent(rawBCMetaEvt)

	require.NotNil(t, barcodeMetaReceived)
	assert.Equal(t, "honeywell-1300g", barcodeMetaReceived.Data.DeviceID)

	assert.Len(t, anyBarcodeReceived, 2)
	assert.Len(t, anyEventReceived, 2)
}

func TestLibraryClient_ScopedReader(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	client, err := New(fakeBroker)
	require.NoError(t, err)

	scoped := client.ForReader("pad-01")
	assert.Equal(t, "pad-01", scoped.DefaultReaderID())

	var scopedTagCount int
	var scopedBarcodeCount int

	scoped.OnTagRead(func(evt *TagReadEvent) {
		scopedTagCount++
	})
	scoped.OnBarcodeRead(func(evt *BarcodeReadEvent) {
		scopedBarcodeCount++
	})

	// Emit matching reader ID tag
	matchingTag, _ := wsbrokerclient.NewRawEvent(EventTagRead, "rfid-daemon", "test-client", TagReadData{
		ReaderID: "pad-01",
		UID:      "E001",
		ItemID:   "ITEM1",
	})
	fakeBroker.emitIncomingEvent(matchingTag)

	// Emit non-matching reader ID tag
	otherTag, _ := wsbrokerclient.NewRawEvent(EventTagRead, "rfid-daemon", "test-client", TagReadData{
		ReaderID: "pad-02",
		UID:      "E002",
		ItemID:   "ITEM2",
	})
	fakeBroker.emitIncomingEvent(otherTag)

	// Emit matching barcode
	matchingBC, _ := wsbrokerclient.NewRawEvent(EventBarcodeRead, "barcode-daemon", "test-client", BarcodeReadData{
		ReaderID: "pad-01",
		Barcode:  "111111",
	})
	fakeBroker.emitIncomingEvent(matchingBC)

	// Emit non-matching barcode
	otherBC, _ := wsbrokerclient.NewRawEvent(EventBarcodeRead, "barcode-daemon", "test-client", BarcodeReadData{
		ReaderID: "scanner-02",
		Barcode:  "222222",
	})
	fakeBroker.emitIncomingEvent(otherBC)

	assert.Equal(t, 1, scopedTagCount)
	assert.Equal(t, 1, scopedBarcodeCount)
}

func TestLibraryClient_Commands(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	fakeBroker.requestFunc = func(ctx context.Context, evt *wsbrokerclient.RawEvent) (*wsbrokerclient.RawEvent, error) {
		switch evt.Type {
		case EventWriteTag:
			return wsbrokerclient.NewRawEvent(EventWriteTagResponse, "rfid-daemon", "test-client", WriteTagResponseData{
				Status: "ok",
				UID:    "E001",
				ItemID: "ITEM-1",
			})
		case EventWriteAFI:
			return wsbrokerclient.NewRawEvent(EventWriteAFIResponse, "rfid-daemon", "test-client", WriteAFIResponseData{
				Status: "ok",
				AFI:    "07",
				UID:    "E001",
			})
		case EventGetDeviceMetadata:
			return wsbrokerclient.NewRawEvent(EventDeviceMetadataResponse, "rfid-daemon", "test-client", DeviceMetadataResponseData{
				Status:   "ok",
				DeviceID: "reader-rfid",
			})
		case EventBarcodeGetDeviceMetadata:
			return wsbrokerclient.NewRawEvent(EventBarcodeDeviceMetadataResponse, "barcode-daemon", "test-client", BarcodeDeviceMetadataResponseData{
				Status:   "ok",
				DeviceID: "scanner-barcode",
			})
		}
		return nil, nil
	}

	client, err := New(fakeBroker, WithDefaultTarget("rfid-daemon"))
	require.NoError(t, err)

	ctx := context.Background()

	// Write Tag
	writeResp, err := client.WriteTag(ctx, "rfid-daemon", NewWriteTagRequest("ITEM-1", "E001"))
	require.NoError(t, err)
	assert.True(t, writeResp.Data.IsSuccess())

	// Write AFI
	afiResp, err := client.SetAFISecured(ctx, "rfid-daemon", "E001")
	require.NoError(t, err)
	assert.True(t, afiResp.Data.IsSecured())

	// RFID Device Metadata
	rfidMeta, err := client.GetDeviceMetadata(ctx, "rfid-daemon")
	require.NoError(t, err)
	assert.Equal(t, "reader-rfid", rfidMeta.Data.DeviceID)

	// Barcode Device Metadata
	bcMeta, err := client.GetBarcodeDeviceMetadata(ctx, "barcode-daemon")
	require.NoError(t, err)
	assert.Equal(t, "scanner-barcode", bcMeta.Data.DeviceID)
}

func TestLibraryClient_Unsubscribe(t *testing.T) {
	fakeBroker := newFakeBroker("test-client")
	client, err := New(fakeBroker)
	require.NoError(t, err)

	count := 0
	unsub := client.OnTagRead(func(evt *TagReadEvent) {
		count++
	})

	tagEvt, _ := wsbrokerclient.NewRawEvent(EventTagRead, "rfid-daemon", "test-client", TagReadData{
		UID:    "E001",
		ItemID: "ITEM1",
	})
	fakeBroker.emitIncomingEvent(tagEvt)
	assert.Equal(t, 1, count)

	unsub()

	fakeBroker.emitIncomingEvent(tagEvt)
	assert.Equal(t, 1, count)
}
