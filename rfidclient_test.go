package rfidclient

import (
	"context"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReExports_BackwardCompatibility(t *testing.T) {
	// Verify constants
	assert.Equal(t, libraryclient.EventTagRead, EventTagRead)
	assert.Equal(t, libraryclient.EventTagsCleared, EventTagsCleared)
	assert.Equal(t, libraryclient.AFISecured, AFISecured)
	assert.Equal(t, libraryclient.AFIUnsecured, AFIUnsecured)

	// Verify helpers
	assert.Equal(t, "07", NormalizeAFI("7"))
	assert.True(t, IsAFISecured("07"))
	assert.True(t, IsAFIUnsecured("C2"))
	assert.Equal(t, "Gesichert", AFISecurityStatus("07"))

	// Verify request builders
	req := NewWriteTagRequest("ITEM1", "E001")
	assert.Equal(t, "E001", req.UID)
	assert.Equal(t, "ITEM1", req.ItemID)

	afiReq := NewWriteAFIRequest("07", "E001")
	assert.Equal(t, "07", afiReq.AFI)
	assert.Equal(t, "E001", afiReq.UID)

	metaReq := NewDeviceMetadataRequest("pad-01")
	assert.Equal(t, "pad-01", metaReq.ReaderID)

	// Verify config
	cfg := DefaultWSBrokerConfig()
	assert.True(t, cfg.AutoReconnect)

	// Verify option creation
	_ = WithDefaultTarget("rfid-daemon")
	_ = WithDefaultReaderID("pad-01")
	_ = WithDefaultTimeout(5 * time.Second)
	_ = WithChannels("all")
	_ = WithClientName("desk")

	// Context check
	ctx := context.Background()
	assert.NotNil(t, ctx)
	require.NotNil(t, cfg)
}
