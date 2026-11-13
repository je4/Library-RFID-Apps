package desk

import (
	"strconv"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTagItem_ReadTimeFromTagReadDataTime(t *testing.T) {
	customTime := time.Date(2026, 11, 13, 14, 30, 45, 0, time.UTC)
	eventTime := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	tagEvent := &libraryclient.TagReadEvent{
		Timestamp: eventTime,
		Data: libraryclient.TagReadData{
			UID:    "E0040150",
			ItemID: "BOOK-12345",
			Time:   &customTime,
		},
	}

	item := NewTagItem(tagEvent)
	require.NotNil(t, item)
	assert.Equal(t, "14:30:45", item.TimeFormatted)
	assert.Equal(t, "13.11.2026", item.DateFormatted)
	assert.Equal(t, customTime.UnixMilli(), item.Timestamp)
	assert.Equal(t, "E0040150-"+strconv.FormatInt(eventTime.UnixMilli(), 10), item.ID)
	assert.Equal(t, eventTime, item.ReadTime)
}

func TestNewTagItem_FallbackToEventTimestamp(t *testing.T) {
	eventTime := time.Date(2026, 11, 13, 10, 15, 20, 0, time.UTC)

	tagEvent := &libraryclient.TagReadEvent{
		Timestamp: eventTime,
		Data: libraryclient.TagReadData{
			UID:    "E0040151",
			ItemID: "BOOK-67890",
		},
	}

	item := NewTagItem(tagEvent)
	require.NotNil(t, item)
	assert.Equal(t, "10:15:20", item.TimeFormatted)
	assert.Equal(t, "13.11.2026", item.DateFormatted)
	assert.Equal(t, eventTime.UnixMilli(), item.Timestamp)
	assert.Equal(t, eventTime, item.ReadTime)
}

func TestViewModels(t *testing.T) {
	deskData := DeskData{}
	if deskData.IsLoan() {
		t.Errorf("DeskData.IsLoan() = true, want false")
	}
	if deskData.IsEncode() {
		t.Errorf("DeskData.IsEncode() = true, want false")
	}

	loanData := LoanData{}
	if !loanData.IsLoan() {
		t.Errorf("LoanData.IsLoan() = false, want true")
	}
	if loanData.IsEncode() {
		t.Errorf("LoanData.IsEncode() = true, want false")
	}

	encodeData := EncodeData{
		DefaultISIL:  "CH-000001",
		DefaultAFI:   "07",
		DefaultParts: 1,
	}
	if encodeData.IsLoan() {
		t.Errorf("EncodeData.IsLoan() = true, want false")
	}
	if !encodeData.IsEncode() {
		t.Errorf("EncodeData.IsEncode() = false, want true")
	}
	if encodeData.DefaultISIL != "CH-000001" {
		t.Errorf("EncodeData.DefaultISIL = %q, want %q", encodeData.DefaultISIL, "CH-000001")
	}
	if encodeData.DefaultAFI != "07" {
		t.Errorf("EncodeData.DefaultAFI = %q, want %q", encodeData.DefaultAFI, "07")
	}
}
