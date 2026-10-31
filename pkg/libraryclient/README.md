# libraryclient

`libraryclient` is a high-level, production-ready unified Go client library for communicating with both RFID reader/writer daemons (`rfid-headless`) and serial barcode scanner daemons (`barcode-headless`) over **WSBroker** using `wsbrokerclient`.

---

## Installation

```go
import "github.com/je4/Library-RFID-Apps/pkg/libraryclient"
```

---

## Quickstart

### 1. Connect and Listen for RFID & Barcode Events

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/wsbrokerclient"
)

func main() {
	cfg := wsbrokerclient.DefaultConfig()
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.ClientName = "desk-app"

	client, err := libraryclient.NewFromConfig(cfg,
		libraryclient.WithChannels("all", "rfid"),
		libraryclient.WithDefaultTarget("rfid-headless-01"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize library client: %v", err)
	}

	// Listen for RFID tag scans
	client.OnTagRead(func(evt *libraryclient.TagReadEvent) {
		fmt.Printf("RFID Tag Scanned: ItemID=%s, UID=%s, AFI=%s\n",
			evt.Data.ItemID, evt.Data.UID, evt.Data.AFI)
	})

	// Listen for Barcode scans
	client.OnBarcodeRead(func(evt *libraryclient.BarcodeReadEvent) {
		fmt.Printf("Barcode Scanned: %s (Type: %s, Reader: %s)\n",
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

### 2. RFID Tag & AFI Security Programming

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Write ISO 28560 metadata and security byte to transponder
req := libraryclient.NewWriteTagRequest("E004010000000001", "31145000123456", "CH-000001-1", libraryclient.AFISecured)
resp, err := client.WriteTag(ctx, "rfid-headless-01", req)
if err != nil {
	log.Printf("Tag write failed: %v", err)
} else if resp.Data.IsSuccess() {
	log.Printf("Tag programmed successfully!")
}

// Set AFI security status (Check-out: Unsecure)
afiResp, err := client.SetAFIUnsecured(ctx, "rfid-headless-01", "E004010000000001")
if err != nil {
	log.Printf("AFI update failed: %v", err)
} else {
	log.Printf("AFI set to: %s (%s)", afiResp.Data.AFI, afiResp.Data.SecurityStatus())
}
```

---

## Event Architecture

| Event Constant | Event Wire Name | Payload Data Structure |
|---|---|---|
| `EventTagRead` | `rfid/tag_read` | `TagReadData` |
| `EventTagsCleared` | `rfid/tags_cleared` | `TagsClearedData` |
| `EventGetDeviceMetadata` | `rfid/get_device_metadata` | `DeviceMetadataRequest` |
| `EventDeviceMetadataResponse` | `rfid/device_metadata_response` | `DeviceMetadataResponseData` |
| `EventWriteTag` | `rfid/write_tag` | `WriteTagRequest` |
| `EventWriteTagResponse` | `rfid/write_tag_response` | `WriteTagResponseData` |
| `EventWriteAFI` | `rfid/write_afi` | `WriteAFIRequest` |
| `EventWriteAFIResponse` | `rfid/write_afi_response` | `WriteAFIResponseData` |
| `EventBarcodeRead` | `barcode/read` | `BarcodeReadData` |
| `EventBarcodeGetDeviceMetadata` | `barcode/get_device_metadata` | `BarcodeDeviceMetadataRequest` |
| `EventBarcodeDeviceMetadataResponse` | `barcode/device_metadata_response` | `BarcodeDeviceMetadataResponseData` |
