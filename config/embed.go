package config

import _ "embed"

//go:embed library-desk.toml
var LibraryDeskConfigTOML string

//go:embed barcode-headless.toml
var BarcodeHeadlessConfigTOML string
