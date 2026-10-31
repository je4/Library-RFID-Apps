package barcodescanner_test

import (
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/barcodescanner"
	"github.com/stretchr/testify/assert"
)

func TestParseSymbology(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		stripAIM      bool
		trimWS        bool
		wantBarcode   string
		wantSymbology string
	}{
		{
			name:          "AIM Code 128 prefix",
			raw:           "]C11234567890\r\n",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "1234567890",
			wantSymbology: "Code 128 / GS1-128",
		},
		{
			name:          "AIM Code 128 without stripping",
			raw:           "]C11234567890\r\n",
			stripAIM:      false,
			trimWS:        true,
			wantBarcode:   "]C11234567890",
			wantSymbology: "Code 128 / GS1-128",
		},
		{
			name:          "AIM Code 39 prefix",
			raw:           "]A0ABC-123\r",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "ABC-123",
			wantSymbology: "Code 39",
		},
		{
			name:          "AIM Data Matrix prefix",
			raw:           "]d2DM-DATA-99",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "DM-DATA-99",
			wantSymbology: "Data Matrix",
		},
		{
			name:          "AIM QR Code prefix",
			raw:           "]Q1HTTPS://EXAMPLE.COM",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "HTTPS://EXAMPLE.COM",
			wantSymbology: "QR Code",
		},
		{
			name:          "Plain numeric EAN-13 without AIM prefix",
			raw:           "7612345678901\r\n",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "7612345678901",
			wantSymbology: "EAN-13",
		},
		{
			name:          "Plain numeric EAN-8 without AIM prefix",
			raw:           "12345670\r",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "12345670",
			wantSymbology: "EAN-8",
		},
		{
			name:          "Plain alphanumeric without AIM prefix",
			raw:           "BOOK-ITEM-9988\n",
			stripAIM:      true,
			trimWS:        true,
			wantBarcode:   "BOOK-ITEM-9988",
			wantSymbology: "Code 128 / Code 39",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBarcode, gotSym := barcodescanner.ParseSymbology(tt.raw, tt.stripAIM, tt.trimWS)
			assert.Equal(t, tt.wantBarcode, gotBarcode)
			assert.Equal(t, tt.wantSymbology, gotSym)
		})
	}
}
