package sip2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const maxMessageSize = 64 * 1024

// Client exchanges SIP2 messages over a single connection. Requests are serialized.
// Close the client after use. A failed exchange closes the connection because its
// protocol state may be unknown; create a new client to reconnect.
type Client struct {
	conn      net.Conn
	reader    *bufio.Reader
	gate      chan struct{}
	checksums bool
	sequence  byte
	skipLF    bool
}

// ClientOptions configures optional SIP2 error detection for the connection.
type ClientOptions struct {
	Checksums bool
}

// Dial connects to a SIP2 server, for example to tunnel.Addr().String().
// A successful dial to a tunnel does not guarantee that its remote TLS connection
// has been established; the first exchange can still fail.
func Dial(ctx context.Context, addr string, options ...ClientOptions) (*Client, error) {
	if len(options) > 1 {
		return nil, errors.New("only one SIP2 client options value is allowed")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial SIP2: %w", err)
	}
	client := &Client{conn: conn, reader: bufio.NewReaderSize(conn, maxMessageSize), gate: make(chan struct{}, 1)}
	if len(options) == 1 {
		client.checksums = options[0].Checksums
	}
	return client, nil
}

// Close closes the connection and interrupts any exchange in progress.
func (c *Client) Close() error { return c.conn.Close() }

// Exchange sends one complete SIP2 message without its terminating CR and
// returns the response without its terminating CR. expectedCode must be the
// two-character response code. The caller constructs and parses message fields.
// When enabled, sequence and checksum fields are added and verified internally.
// The client does not automatically retry transactions.
func (c *Client) Exchange(ctx context.Context, message, expectedCode string) (string, error) {
	if len(message) < 2 || len(message)+10 >= maxMessageSize || strings.ContainsAny(message, "\r\n\x00") {
		return "", errors.New("invalid SIP2 request")
	}
	if len(expectedCode) != 2 {
		return "", errors.New("expected SIP2 response code must have two bytes")
	}
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			c.conn.Close()
			return "", fmt.Errorf("set SIP2 deadline: %w", err)
		}
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		c.conn.SetDeadline(time.Now())
		close(done)
	})
	defer func() {
		if !stop() {
			<-done
		}
		c.conn.SetDeadline(time.Time{})
	}()

	request := message
	if c.checksums {
		request += fmt.Sprintf("AY%dAZ", c.sequence)
		request += fmt.Sprintf("%04X", checksum(request))
	}
	request += "\r"
	for len(request) > 0 {
		n, err := c.conn.Write([]byte(request))
		if err != nil || n == 0 {
			c.conn.Close()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if err == nil {
				err = io.ErrShortWrite
			}
			return "", fmt.Errorf("write SIP2 request: %w", err)
		}
		request = request[n:]
	}
	response, err := c.readMessage()
	if err != nil {
		c.conn.Close()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("read SIP2 response: %w", err)
	}
	if c.checksums {
		if len(response) < 10 || response[len(response)-9:len(response)-7] != "AY" || response[len(response)-7] != '0'+c.sequence || response[len(response)-6:len(response)-4] != "AZ" {
			c.conn.Close()
			return "", errors.New("invalid SIP2 sequence or checksum fields")
		}
		value, err := strconv.ParseUint(response[len(response)-4:], 16, 16)
		if err != nil || strings.ContainsAny(response[len(response)-4:], "+-") || uint16(value) != checksum(response[:len(response)-4]) {
			c.conn.Close()
			return "", errors.New("invalid SIP2 checksum")
		}
		response = response[:len(response)-9]
		c.sequence = (c.sequence + 1) % 10
	}
	if len(response) < 2 || response[:2] != expectedCode || strings.ContainsRune(response, '\x00') {
		c.conn.Close()
		return "", fmt.Errorf("unexpected or malformed SIP2 response")
	}
	return response, nil
}

func checksum(message string) uint16 {
	var sum uint16
	for i := 0; i < len(message); i++ {
		sum += uint16(message[i])
	}
	return -sum
}

func (c *Client) readMessage() (string, error) {
	var message []byte
	for {
		b, err := c.reader.ReadByte()
		if err != nil {
			return "", err
		}
		if c.skipLF {
			c.skipLF = false
			if b == '\n' {
				continue
			}
		}
		if b == '\r' || b == '\n' {
			c.skipLF = b == '\r'
			return string(message), nil
		}
		message = append(message, b)
		if len(message) >= maxMessageSize {
			return "", errors.New("SIP2 response too large")
		}
	}
}

// Login sends the SIP2 login message using unencrypted credentials (algorithm 0).
// Use a protected connection such as Tunnel. An empty location omits CP.
// A false result means the server explicitly rejected the login.
func (c *Client) Login(ctx context.Context, username, password, location string) (bool, error) {
	for _, value := range []string{username, password, location} {
		if len(value) > 255 || strings.ContainsAny(value, "|\r\n\x00") {
			return false, errors.New("invalid SIP2 login field")
		}
	}
	message := "9300CN" + username + "|CO" + password + "|"
	if location != "" {
		message += "CP" + location + "|"
	}
	response, err := c.Exchange(ctx, message, "94")
	if err != nil {
		return false, err
	}
	if len(response) != 3 || (response[2] != '0' && response[2] != '1') {
		c.Close()
		return false, errors.New("invalid SIP2 login response")
	}
	return response[2] == '1', nil
}
