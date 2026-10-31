# barcodeclient

`barcodeclient` is a high-level, production-ready Go client library for communicating with `barcode-headless` serial scanner daemons over **WSBroker** using `wsbrokerclient`.

---

## Installation

```go
import "github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
```

---

## Quickstart

### 1. Connect and Listen for Barcodes

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
	"github.com/je4/wsbrokerclient"
)

func main() {
	cfg := wsbrokerclient.DefaultConfig()
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.ClientName = "desk-app"

	client, err := barcodeclient.NewFromConfig(cfg,
		barcodeclient.WithChannels("intern", "all"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize barcode client: %v", err)
	}

	// Register typed barcode read event handler
	client.OnBarcodeRead(func(evt *barcodeclient.BarcodeReadEvent) {
		fmt.Printf("Scanned Barcode: %s (Type: %s, Reader: %s)\n",
			evt.Data.Barcode, evt.Data.Symbology, evt.Data.ReaderID)
	})

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.Close()

	select {}
}
```

### 2. Query Scanner Hardware Metadata

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

meta, err := client.GetDeviceMetadata(ctx, "honeywell-01")
if err != nil {
	log.Printf("Query error: %v", err)
} else {
	fmt.Printf("Scanner Status: %s on Port: %s (Total scans: %d)\n",
		meta.Data.Status, meta.Data.Port, meta.Data.TotalScans)
}
```

---

## Event Architecture

| Event Constant | Event Wire Name | Payload Data Structure |
|---|---|---|
| `EventBarcodeRead` | `barcode/read` | `BarcodeReadData` |
| `EventGetDeviceMetadata` | `barcode/get_device_metadata` | `DeviceMetadataRequest` |
| `EventDeviceMetadataResponse` | `barcode/device_metadata_response` | `DeviceMetadataResponseData` |
