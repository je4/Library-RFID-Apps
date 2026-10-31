# rfidclient

`rfidclient` is a high-level, production-ready Go client library for communicating with `rfid-headless` RFID reader/writer daemons over **WSBroker** using `wsbrokerclient`.

---

## Installation

```go
import "github.com/je4/Library-RFID-Apps/pkg/rfidclient"
```

Or via the root module re-export:

```go
import rfidclient "github.com/je4/Library-RFID-Apps"
```

---

## Quickstart

### 1. Connect and Listen for RFID Tag Scans

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/wsbrokerclient"
)

func main() {
	cfg := wsbrokerclient.DefaultConfig()
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.ClientName = "desk-app"

	client, err := rfidclient.NewFromConfig(cfg,
		rfidclient.WithChannels("intern", "all"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize RFID client: %v", err)
	}

	// Register tag read handler
	client.OnTagRead(func(evt *rfidclient.TagReadEvent) {
		fmt.Printf("RFID Tag Read: ItemID=%s UID=%s (Secured=%v, Reader=%s)\n",
			evt.Data.ItemID, evt.Data.UID, evt.Data.IsSecured(), evt.Data.ReaderID)
	})

	// Register field cleared handler
	client.OnTagsCleared(func(evt *rfidclient.TagsClearedEvent) {
		fmt.Printf("Antenna field cleared on reader: %s\n", evt.Data.ReaderID)
	})

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.Close()

	select {}
}
```

### 2. Write an RFID Tag (ISO 28560-2)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

req := rfidclient.NewWriteTagRequest("3010998877", "CH-000001-1")
req.WithReaderID("feig-pad-01").WithAFI(rfidclient.AFISecured)

resp, err := client.WriteTag(ctx, req)
if err != nil {
	log.Fatalf("WriteTag failed: %v", err)
}
fmt.Printf("WriteTag Success: %v (UID: %s)\n", resp.Data.IsSuccess(), resp.Data.Results[0].UID)
```

### 3. Toggle AFI / EAS Security Byte

```go
// Check out / unsecure
resp, err := client.WriteAFI(ctx, rfidclient.NewWriteAFIRequest(rfidclient.AFIUnsecured))

// Check in / secure
resp, err := client.WriteAFI(ctx, rfidclient.NewWriteAFIRequest(rfidclient.AFISecured))
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
