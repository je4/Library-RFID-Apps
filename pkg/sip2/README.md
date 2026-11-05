# SIP2 tunnel and client

`OpenTunnel` starts a local TCP listener and forwards each connection over TLS
with a client certificate, like the stunnel configuration in the SLSP guide.
The client provides SIP2 login, typed circulation operations and a generic
message exchange.

```go
tunnel, err := sip2.OpenTunnel(sip2.TunnelConfig{
    CertificateFile: "certs/slsp/client.crt",
    KeyFile:         "certs/slsp/client.key",
    // CAFile: "path/to/trusted-server-ca.pem",
})
if err != nil {
    return err
}
defer tunnel.Close()
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
client, err := sip2.Dial(ctx, tunnel.Addr().String())
if err != nil {
    return err
}
defer client.Close()
ok, err := client.Login(ctx, "username", "password", "")
if err != nil {
    return err
}
if !ok {
    return errors.New("SIP2 login rejected")
}
result, err := client.Checkout(ctx, sip2.CheckoutRequest{
    TransactionDate: time.Now().Format("20060102    150405"),
    DueDate:         time.Now().AddDate(0, 0, 14).Format("20060102    150405"),
    InstitutionID:   "library",
    PatronID:        "patron-barcode",
    ItemID:          "item-barcode",
})
if err != nil {
    return err
}
if !result.OK {
    // Inspect result.Fields, including any AF screen messages.
}
```

The listener binds to `127.0.0.1` on an automatically assigned port, available
through `tunnel.Addr()`. Other local processes can also connect to this port.
By default the remote endpoint is `slsp-ubs.alma.exlibrisgroup.com:6443` and
the connection timeout is 200 seconds; these can be overridden in `TunnelConfig`.
The remote connection is
opened only when a local client connects. Server certificates are verified
against the system trust store, or against `CAFile` if supplied; use the latter
only if it contains the CA that signs the Alma server certificate. Connection
failures close the local connection. Protect the private key and do not commit it.

Typed methods cover checkout, checkin, patron status and information, blocking
and enabling patrons, end session, item information and status updates, SC
status, fee payment, holds, renewals and renew-all. Their request dates are
18-byte SIP2 date/time strings; response `Fields` contains variable fields by
their two-character identifiers and preserves repeated values. Consult the SIP2
specification and server configuration for field meanings and required values.

`Exchange` sends and receives raw SIP2 messages without the terminating CR;
its caller is responsible for constructing and parsing their fields. The client
not implement sequence numbers, checksums, resend handling or automatic retries.
Use it only with servers configured without SIP2 error detection. The caller
must agree on a character encoding with the server (the SIP2 default is CP850).
Use contexts with deadlines; after an exchange error, reconnect before retrying,
and consider that the server might already have processed the request. Login
uses algorithm 0 (plaintext credentials), so use a protected connection.