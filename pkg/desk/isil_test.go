package desk

import (
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadISILList(t *testing.T) {
	logger := zerolog.Nop()
	list, err := LoadISILList(&logger)
	require.NoError(t, err)
	assert.NotEmpty(t, list)
	assert.Greater(t, len(list), 2000)

	// Find a known ISIL entry to check fields
	var found *ISILEntry
	for _, entry := range list {
		if entry.ISIL == "CH-000050-X" {
			found = entry
			break
		}
	}

	require.NotNil(t, found, "ISIL CH-000050-X should be in the list")
	assert.Equal(t, "aktiv", found.Status)
	assert.Equal(t, "Aargauer Kantonsbibliothek", found.NameOriginal)
	assert.Equal(t, "Aarau", found.Ortschaft)
	assert.Equal(t, "Aargau", found.Kanton)
}

func TestFindISILEntry_And_TagItemEnrich(t *testing.T) {
	logger := zerolog.Nop()
	list, err := LoadISILList(&logger)
	require.NoError(t, err)
	require.NotEmpty(t, list)

	// Test FindISILEntry exact
	entry1 := FindISILEntry(list, "CH-000050-X", "")
	require.NotNil(t, entry1)
	assert.Equal(t, "Aargauer Kantonsbibliothek", entry1.NameOriginal)

	// Test FindISILEntry with ISIL without CH prefix and country CH
	entry2 := FindISILEntry(list, "000050-X", "CH")
	require.NotNil(t, entry2)
	assert.Equal(t, "Aargauer Kantonsbibliothek", entry2.NameOriginal)

	// Test FindISILEntry case-insensitive
	entry3 := FindISILEntry(list, "ch-000050-x", "")
	require.NotNil(t, entry3)
	assert.Equal(t, "Aargauer Kantonsbibliothek", entry3.NameOriginal)

	// Test TagItem.EnrichISIL
	tag := &TagItem{
		ISIL:    "CH-000050-X",
		Country: "CH",
	}
	tag.EnrichISIL(list)
	assert.Equal(t, "Aargauer Kantonsbibliothek", tag.ISILName)
	assert.NotNil(t, tag.ISILEntry)
	assert.Equal(t, "Aarau", tag.ISILEntry.Ortschaft)

	// Test Hub integration
	hub := NewHub(10, list)
	item := hub.AddTag(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:     "E004015011223344",
			ItemID:  "ITEM-001",
			ISIL:    "CH-000050-X",
			Country: "CH",
		},
	})
	assert.Equal(t, "Aargauer Kantonsbibliothek", item.ISILName)
	assert.NotNil(t, item.ISILEntry)
}

func TestParseISIL(t *testing.T) {
	tests := []struct {
		input       string
		wantCountry string
		wantISIL    string
	}{
		{"CH-000001", "CH", "000001"},
		{"ch-000050-x", "CH", "000050-x"},
		{"DE-12345", "DE", "12345"},
		{"000001", "", "000001"},
		{"", "", ""},
		{"  CH-000001  ", "CH", "000001"},
	}

	for _, tt := range tests {
		c, isil := ParseISIL(tt.input)
		assert.Equal(t, tt.wantCountry, c, "input: %s", tt.input)
		assert.Equal(t, tt.wantISIL, isil, "input: %s", tt.input)
	}
}
