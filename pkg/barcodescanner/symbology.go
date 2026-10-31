package barcodescanner

import (
	"strings"
	"unicode"
)

// Standard AIM Symbology Identifiers (ISO/IEC 15424)
var aimSymbologyMap = map[string]string{
	"A": "Code 39",
	"B": "Telepen",
	"C": "Code 128 / GS1-128",
	"d": "Data Matrix",
	"E": "EAN / UPC",
	"e": "GS1 Composite",
	"F": "Codabar",
	"G": "GS1 DataBar",
	"H": "Code 11",
	"I": "Interleaved 2 of 5 (ITF)",
	"L": "PDF417",
	"M": "MSI",
	"N": "Anker Code",
	"P": "Plessey Code",
	"Q": "QR Code",
	"R": "MicroQR Code",
	"s": "Codablock",
	"T": "Code 49",
	"U": "MaxiCode",
	"V": "Vericode",
	"X": "Generic Barcode",
	"z": "Aztec Code",
}

// ParseSymbology extracts AIM symbology prefix (e.g. "]C1", "]E0", "]d2") if present,
// returns the clean barcode and identified symbology name.
func ParseSymbology(raw string, stripAIM bool, trimWhitespace bool) (cleanBarcode string, symbology string) {
	trimmed := raw
	if trimWhitespace {
		trimmed = strings.TrimFunc(trimmed, func(r rune) bool {
			return unicode.IsSpace(r) || r == '\x00' || r == '\x02' || r == '\x03' || r == '\x04'
		})
	}

	symbology = "Unknown"
	cleanBarcode = trimmed

	// Check for AIM prefix standard: Starts with ']' followed by flag char and modifier digit (e.g. "]C1")
	if len(trimmed) >= 3 && trimmed[0] == ']' {
		codeChar := string(trimmed[1])
		if name, ok := aimSymbologyMap[codeChar]; ok {
			symbology = name
			if stripAIM {
				cleanBarcode = trimmed[3:]
				if trimWhitespace {
					cleanBarcode = strings.TrimSpace(cleanBarcode)
				}
			}
			return cleanBarcode, symbology
		}
	}

	// Heuristic detection if no AIM prefix is present
	symbology = detectSymbologyHeuristic(cleanBarcode)
	return cleanBarcode, symbology
}

func detectSymbologyHeuristic(code string) string {
	if code == "" {
		return "Unknown"
	}

	allDigits := true
	for _, r := range code {
		if !unicode.IsDigit(r) {
			allDigits = false
			break
		}
	}

	if allDigits {
		switch len(code) {
		case 8:
			return "EAN-8"
		case 12:
			return "UPC-A"
		case 13:
			return "EAN-13"
		case 14:
			return "ITF-14 / GTIN-14"
		default:
			return "Numeric Barcode"
		}
	}

	return "Code 128 / Code 39"
}
