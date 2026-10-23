package data

import "embed"

// WebFS contains all embedded web templates and static assets for the rfid-desk frontend.
//
//go:embed web/*
var WebFS embed.FS
