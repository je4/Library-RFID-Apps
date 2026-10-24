package data

import "embed"

// WebFS contains all embedded web templates and static assets for the rfid-desk frontend.
//
//go:embed web/static/css/*
//go:embed web/static/scss/*.css
//go:embed web/static/scss/*.map
//go:embed web/static/img/*
//go:embed web/static/js/*
//go:embed web/templates/*
//go:embed isil-schweiz.csv.bz2
var WebFS embed.FS
