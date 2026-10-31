package config

import _ "embed"

//go:embed rfid-desk.toml
var RFIDDeskConfigTOML string

//go:embed barcode-headless.toml
var BarcodeHeadlessConfigTOML string
