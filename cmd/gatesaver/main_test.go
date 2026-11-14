package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestHandleTagRead(t *testing.T) {
	var buf bytes.Buffer
	loggerZero := zerolog.New(&buf).With().Timestamp().Logger()
	var logger zLogger.ZLogger = &loggerZero

	now := time.Now().UTC()
	evt := &libraryclient.TagReadEvent{
		Channel: "gates",
		Source:  "reader-01",
		Data: libraryclient.TagReadData{
			ReaderID: "reader-01",
			UID:      "E004010000000001",
			ItemID:   "31111000000001",
			ISIL:     "CH-001596-0",
			AFI:      "07",
			CRCOk:    true,
			Time:     &now,
		},
	}

	handleTagRead(logger, evt)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"channel":"gates"`)
	assert.Contains(t, logOutput, `"reader_id":"reader-01"`)
	assert.Contains(t, logOutput, `"uid":"E004010000000001"`)
	assert.Contains(t, logOutput, `"item_id":"31111000000001"`)
	assert.Contains(t, logOutput, `"isil":"CH-001596-0"`)
	assert.Contains(t, logOutput, `"afi":"07"`)
	assert.Contains(t, logOutput, `"crc_ok":true`)
}
