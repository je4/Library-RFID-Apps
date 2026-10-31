# Standard Event Protocol Specification

This document specifies the standardized WebSocket message events exchanged between headless reader daemons (`rfid-headless`, `barcode-headless`), client libraries (`pkg/rfidclient`, `pkg/barcodeclient`), and applications (`rfid-desk`, self-checkouts, return kiosks, inventory scanners) via **WSBroker**.

---

## Architecture Overview

All communication flows over `wsbrokerclient` using a generic envelope pattern:

```json
{
  "source": "barcode-headless-01",
  "target": "all",
  "channel": "intern",
  "token": "tok-12345",
  "type": "message",
  "event": "barcode/read",
  "id": "evt-abcde",
  "refId": "",
  "timestamp": "2026-10-31T15:44:00Z",
  "data": { ... }
}
```

- **Transport Metadata (`wsbrokerclient.BaseEvent`)**: Routing and correlation fields (`Source`, `Target`, `Channel`, `Token`, `Type`, `ID`, `RefID`, `Timestamp`).
- **Domain Payload (`Data`)**: Strongly-typed payload registered in the `wsbrokerclient` global registry for automatic serialization and deserialization.

---

## 1. Barcode Events (`pkg/barcodeclient`)

Serial barcode scanners (e.g. Honeywell 1300G-2) operate autonomously and stream read events. They also support diagnostic hardware status queries.

### 1.1 `barcode/read`
* **Type:** Asynchronous Broadcast / Unidirectional Stream
* **Direction:** `barcode-headless` &rarr; Applications / Clients
* **Payload Type:** `BarcodeReadData`

Emitted when a physical barcode is scanned and decoded from a serial port.

#### JSON Payload Schema
```json
{
  "readerId": "honeywell-01",
  "barcode": "301012345678",
  "itemId": "301012345678",
  "symbology": "Code128",
  "raw": "]C1301012345678\r",
  "timestamp": "2026-10-31T15:44:00Z",
  "deviceId": "honeywell-1300g-01",
  "deviceType": "honeywell-1300g"
}
```

#### Field Descriptions
| Field | Type | Description |
|---|---|---|
| `readerId` | string | Logical reader identifier configured in the daemon. |
| `barcode` | string | Extracted and cleaned barcode data (AIM symbology prefix and line delimiters stripped). |
| `itemId` | string | Normalized item identifier (defaults to `barcode`). |
| `symbology` | string | Decoded barcode standard (e.g., `Code128`, `EAN13`, `Code39`, `Unknown`). |
| `raw` | string | Raw byte string received from the serial port before parsing. |
| `timestamp` | string | ISO 8601 / RFC 3339 UTC timestamp when the scan was captured. |
| `deviceId` | string | Hardware or serial device identifier. |
| `deviceType` | string | Scanner hardware model / type. |

---

### 1.2 `barcode/get_device_metadata`
* **Type:** Synchronous RPC Query Request
* **Direction:** Application &rarr; `barcode-headless`
* **Payload Type:** `DeviceMetadataRequest`

Requests diagnostic hardware metadata and connection status from one or all configured barcode scanners.

#### JSON Payload Schema
```json
{
  "readerId": "honeywell-01"
}
```

#### Field Descriptions
| Field | Type | Description |
|---|---|---|
| `readerId` | string | Target reader ID to query. If omitted or `"all"`, metadata for all configured scanners is returned. |

---

### 1.3 `barcode/device_metadata_response`
* **Type:** Correlated RPC Response
* **Direction:** `barcode-headless` &rarr; Requesting Client
* **Payload Type:** `DeviceMetadataResponseData`

Returns hardware configuration, diagnostic status, scan counts, and serial port settings.

#### JSON Payload Schema
```json
{
  "readerId": "honeywell-01",
  "status": "connected",
  "deviceId": "honeywell-1300g-01",
  "deviceType": "honeywell-1300g",
  "port": "COM3",
  "baudRate": 9600,
  "description": "Honeywell 1300g Barcode Scanner on COM3 (9600 baud)",
  "totalScans": 142,
  "lastScanTime": "2026-10-31T15:44:00Z",
  "initDescription": "Serial Port COM3 opened at 9600 baud, 8N1"
}
```

#### Field Descriptions
| Field | Type | Description |
|---|---|---|
| `readerId` | string | Logical identifier of the reader. |
| `status` | string | Reader status (`"connected"`, `"ok"`, `"disconnected"`, `"error"`). |
| `deviceId` | string | Hardware device identifier. |
| `deviceType` | string | Scanner hardware model. |
| `port` | string | Serial COM port (e.g., `COM3`, `/dev/ttyUSB0`). |
| `baudRate` | int | Configured baud rate. |
| `description` | string | Human-readable reader summary description. |
| `totalScans` | int64 | Total successful scans processed since daemon startup. |
| `lastScanTime` | string | RFC 3339 timestamp of the last processed scan. |
| `initDescription` | string | Startup diagnostics and port parameters. |

---

## 2. RFID Events (`pkg/rfidclient`)

RFID readers (e.g. FEIG Electronic HF readers) stream transponder inventory events and execute synchronous ISO 28560 / AFI read and write commands.

### 2.1 `rfid/tag_read`
* **Type:** Asynchronous Broadcast / Unidirectional Stream
* **Direction:** `rfid-headless` &rarr; Applications / Clients
* **Payload Type:** `TagReadData`

Emitted when an ISO 15693 / ISO 28560-2 transponder is placed onto the antenna field and decoded.

#### JSON Payload Schema
```json
{
  "readerId": "feig-pad-01",
  "uid": "E004015099887766",
  "itemId": "3010998877",
  "isil": "CH-000001-1",
  "country": "CH",
  "crcOk": true,
  "isCrcValid": true,
  "crc": "OK",
  "tagName": "BookTransponder",
  "raw": "...",
  "parts": 1,
  "partNo": 1,
  "usageType": 1,
  "version": 1,
  "afi": "07"
}
```

#### Field Descriptions
| Field | Type | Description |
|---|---|---|
| `readerId` | string | Identifier of the antenna / reader device. |
| `uid` | string | Unique 64-bit ISO 15693 Transponder ID (Hex). |
| `itemId` | string | Library item identifier / barcode parsed from ISO 28560 data blocks. |
| `isil` | string | ISIL library identifier (e.g., `CH-000001-1`). |
| `country` | string | ISO country code. |
| `crcOk` | bool | True if ISO 28560-2 CRC-16 checksum matches. |
| `isCrcValid` | bool | Alternative boolean flag for CRC validity. |
| `crc` | string | CRC status text (`"OK"`, `"INVALID"`). |
| `parts` | int | Total number of parts in multi-part media set (default: 1). |
| `partNo` | int | 1-based part index of this item (default: 1). |
| `usageType` | int | ISO 28560-2 usage type (1 = circulating item). |
| `version` | int | ISO 28560 data model version. |
| `afi` | string | Application Family Identifier (Hex byte: `07` = Secured/Loaned in, `C2` = Unsecured/Loaned out). |

---

### 2.2 `rfid/tags_cleared`
* **Type:** Asynchronous Broadcast
* **Direction:** `rfid-headless` &rarr; Applications / Clients
* **Payload Type:** `TagsClearedData`

Emitted when all RFID transponders have been removed from the antenna field.

#### JSON Payload Schema
```json
{
  "readerId": "feig-pad-01",
  "status": "cleared",
  "totalTags": 0
}
```

---

### 2.3 `rfid/get_device_metadata`
* **Type:** Synchronous RPC Query Request
* **Direction:** Application &rarr; `rfid-headless`
* **Payload Type:** `DeviceMetadataRequest`

Queries reader firmware, RF power, antenna impedance, and operational diagnostics.

---

### 2.4 `rfid/device_metadata_response`
* **Type:** Correlated RPC Response
* **Direction:** `rfid-headless` &rarr; Requesting Client
* **Payload Type:** `DeviceMetadataResponseData`

```json
{
  "readerId": "feig-pad-01",
  "status": "ok",
  "deviceId": "OBID-CPR74",
  "deviceType": "FEIG-LRM2500",
  "initDescription": "Firmware 03.02, RF Power 2.0W, Antenna Auto-Tuned",
  "description": "FEIG CPR74 HF Reader on USB"
}
```

---

### 2.5 `rfid/write_tag`
* **Type:** Synchronous RPC Command
* **Direction:** Application &rarr; `rfid-headless`
* **Payload Type:** `WriteTagRequest`

Writes ISO 28560-2 encoded data blocks and optional AFI byte to a transponder.

#### JSON Payload Schema
```json
{
  "readerId": "feig-pad-01",
  "itemId": "3010998877",
  "uid": "E004015099887766",
  "isil": "CH-000001-1",
  "parts": 1,
  "partNo": 1,
  "usageType": 1,
  "afi": "07"
}
```

---

### 2.6 `rfid/write_tag_response`
* **Type:** Correlated RPC Response
* **Direction:** `rfid-headless` &rarr; Requesting Client
* **Payload Type:** `WriteTagResponseData`

```json
{
  "readerId": "feig-pad-01",
  "status": "ok",
  "totalSuccess": 1,
  "totalFailed": 0,
  "results": [
    {
      "uid": "E004015099887766",
      "itemId": "3010998877",
      "status": "ok",
      "afi": "07",
      "afiWritten": true
    }
  ]
}
```

---

### 2.7 `rfid/write_afi`
* **Type:** Synchronous RPC Command
* **Direction:** Application &rarr; `rfid-headless`
* **Payload Type:** `WriteAFIRequest`

Modifies or toggles the AFI (Application Family Identifier / EAS theft bit) on tags in the field.

#### JSON Payload Schema
```json
{
  "readerId": "feig-pad-01",
  "afi": "C2",
  "uid": "E004015099887766"
}
```

---

### 2.8 `rfid/write_afi_response`
* **Type:** Correlated RPC Response
* **Direction:** `rfid-headless` &rarr; Requesting Client
* **Payload Type:** `WriteAFIResponseData`

```json
{
  "readerId": "feig-pad-01",
  "status": "ok",
  "totalSuccess": 1,
  "totalFailed": 0,
  "results": [
    {
      "uid": "E004015099887766",
      "status": "ok",
      "afi": "C2"
    }
  ]
}
```

---

## 3. Summary of Standardized Event Names

| Subsystem | Standard Event Constant | Wire Event String | Payload Data Structure |
|---|---|---|---|
| **Barcode** | `EventBarcodeRead` | `barcode/read` | `BarcodeReadData` |
| **Barcode** | `EventGetDeviceMetadata` | `barcode/get_device_metadata` | `DeviceMetadataRequest` |
| **Barcode** | `EventDeviceMetadataResponse` | `barcode/device_metadata_response` | `DeviceMetadataResponseData` |
| **RFID** | `EventTagRead` | `rfid/tag_read` | `TagReadData` |
| **RFID** | `EventTagsCleared` | `rfid/tags_cleared` | `TagsClearedData` |
| **RFID** | `EventGetDeviceMetadata` | `rfid/get_device_metadata` | `DeviceMetadataRequest` |
| **RFID** | `EventDeviceMetadataResponse` | `rfid/device_metadata_response` | `DeviceMetadataResponseData` |
| **RFID** | `EventWriteTag` | `rfid/write_tag` | `WriteTagRequest` |
| **RFID** | `EventWriteTagResponse` | `rfid/write_tag_response` | `WriteTagResponseData` |
| **RFID** | `EventWriteAFI` | `rfid/write_afi` | `WriteAFIRequest` |
| **RFID** | `EventWriteAFIResponse` | `rfid/write_afi_response` | `WriteAFIResponseData` |
