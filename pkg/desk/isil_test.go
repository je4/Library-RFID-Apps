package desk

import (
	"testing"

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
