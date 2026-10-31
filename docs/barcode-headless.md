# Barcode-Headless Daemon

`barcode-headless` is a lightweight, autonomous daemon designed to interface with serial barcode readers (such as the Honeywell 1300G-2 handheld scanner) and broadcast scanned barcodes over **WSBroker** using secure mTLS connections.

---

## Key Features

- **Continuous Event Streaming**: Reads serial barcode scans asynchronously and dispatches `barcode/read` events over WebSocket.
- **Unidirectional Scanning**: Pure event-driven design — scans are pushed automatically without polling or write overhead.
- **Hardware Agnostic & Serial Configuration**: Fully configurable baud rate, data bits, stop bits, parity, and line terminators (`\r`, `\n`).
- **AIM Symbology Parsing**: Automatically strips AIM identifiers (`]C1`, `]A0`, `]E0`, etc.) and determines symbologies (`Code128`, `EAN13`, etc.).
- **Debouncing & Duplicate Suppression**: Configurable time window to suppress rapid duplicate reads of the same barcode.
- **Auto-Reconnect**: Automatically recovers from disconnected USB-to-serial ports without dropping the service.
- **Multi-Reader Support**: Run and configure multiple serial barcode readers concurrently on independent ports.
- **Secure mTLS with MiniVault**: Integrated certificate management with automatic renewal via `certloader`.

---

## Configuration (`barcode-headless.toml`)

```toml
# General settings
client_name = "barcode-headless-01"
log_level = "info"

[broker]
endpoint = "wss://localhost:8080/ws"
channels = ["intern", "all"]
timeout = "10s"

[tls]
type = "minivault"
ca_cert = "certs/ca.crt"
cert = "certs/client.crt"
key = "certs/client.key"

[[readers]]
id = "honeywell-01"
device_id = "honeywell-1300g-01"
device_type = "honeywell-1300g"
port = "COM3"
baud_rate = 9600
data_bits = 8
stop_bits = 1
parity = "none"
delimiter = "\r"
trim_whitespace = true
strip_aim_id = true
debounce_ms = 300
channels = ["intern", "all"]
read_only = true
enabled = true
reconnect_sec = 3
```

---

## Running the Daemon

```bash
# Run with default or embedded config
barcode-headless

# Run with explicit config path
barcode-headless -config config/barcode-headless.toml

# Run with verbose debug logging
barcode-headless -config config/barcode-headless.toml -log-level debug
```

---

## Event Architecture

1. **Barcode Scan (`barcode/read`)**:
   When a barcode is scanned, the daemon packages the decoded value into `BarcodeReadData` and publishes it to all subscribed broker channels.
2. **Metadata Query (`barcode/get_device_metadata`)**:
   Clients can query diagnostic connection status and scan metrics, answered with `barcode/device_metadata_response`.
