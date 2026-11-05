package config

import _ "embed"

//go:embed library-desk.toml
var LibraryDeskConfigTOML string

//go:embed barcode-headless.toml
var BarcodeHeadlessConfigTOML string

//go:embed almaproxy.toml
var AlmaProxyConfigTOML string

//go:embed selfcheckout.toml
var SelfcheckoutConfigTOML string
