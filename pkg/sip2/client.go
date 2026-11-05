package sip2

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const maxMessageSize = 64 * 1024

// Client exchanges SIP2 messages over a single connection. Requests are serialized.
// Close the client after use. A failed exchange closes the connection because its
// protocol state may be unknown; create a new client to reconnect.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	gate   chan struct{}
}

// Dial connects to a SIP2 server, for example to tunnel.Addr().String().
// A successful dial to a tunnel does not guarantee that its remote TLS connection
// has been established; the first exchange can still fail.
func Dial(ctx context.Context, addr string) (*Client, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial SIP2: %w", err)
	}
	return &Client{conn: conn, reader: bufio.NewReaderSize(conn, maxMessageSize), gate: make(chan struct{}, 1)}, nil
}

// Close closes the connection and interrupts any exchange in progress.
func (c *Client) Close() error { return c.conn.Close() }

// Exchange sends one complete SIP2 message without its terminating CR and
// returns the response without its terminating CR. expectedCode must be the
// two-character response code. The caller constructs and parses message fields.
// This client does not add or verify sequence numbers or checksums, and does
// not automatically retry transactions.
func (c *Client) Exchange(ctx context.Context, message, expectedCode string) (string, error) {
	if len(message) < 2 || len(message) >= maxMessageSize || strings.ContainsAny(message, "\r\x00") {
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

	request := message + "\r"
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
	response, err := c.reader.ReadSlice('\r')
	if err != nil {
		c.conn.Close()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("read SIP2 response: %w", err)
	}
	response = response[:len(response)-1]
	if len(response) < 2 || string(response[:2]) != expectedCode || strings.ContainsRune(string(response), '\x00') {
		c.conn.Close()
		return "", fmt.Errorf("unexpected or malformed SIP2 response")
	}
	return string(response), nil
}

// Login sends the SIP2 login message using unencrypted credentials (algorithm 0).
// Use a protected connection such as Tunnel. An empty location omits CP.
// A false result means the server explicitly rejected the login.
func (c *Client) Login(ctx context.Context, username, password, location string) (bool, error) {
	for _, value := range []string{username, password, location} {
		if len(value) > 255 || strings.ContainsAny(value, "|\r\x00") {
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
