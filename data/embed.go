package data

import "embed"

// WebFS contains all embedded web templates and static assets for the rfid-desk frontend.
//
//go:embed web/* isil-schweiz.csv.bz2
var WebFS embed.FS
