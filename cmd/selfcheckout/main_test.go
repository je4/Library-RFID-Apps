package main

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
)

func TestConnectSIP2(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		status   string
		wantErr  bool
	}{
		{"accepted", "941\r", "98YYYYYN03000320240101    1200002.00AOlib|BXYYYYYYYYYYYYYYYY|\r", false},
		{"rejected", "940\r", "98YYYYYN03000320240101    1200002.00AOlib|BXYYYYYYYYYYYYYYYY|\r", true},
		{"malformed", "94x\r", "98YYYYYN03000320240101    1200002.00AOlib|BXYYYYYYYYYYYYYYYY|\r", true},
		{"invalid status", "", "bad\r", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			requests := make(chan string, 2)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					requests <- ""
					return
				}
				defer conn.Close()
				reader := bufio.NewReader(conn)
				request, _ := reader.ReadString('\r')
				requests <- request
				_, _ = conn.Write([]byte(tc.status))
				if tc.response == "" {
					return
				}
				request, _ = reader.ReadString('\r')
				requests <- request
				_, _ = conn.Write([]byte(tc.response))
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, err := connectSIP2(ctx, listener.Addr().String(), SIP2Config{Username: "user", Password: "secret", Location: "library"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("connectSIP2 error: %v", err)
			}
			if client != nil {
				defer client.Close()
			}
			if got := <-requests; got != "9900802.00\r" {
				t.Fatalf("unexpected status request: %q", got)
			}
			if tc.response != "" {
				if got := <-requests; got != "9300CNuser|COsecret|CPlibrary|\r" {
					t.Fatalf("unexpected login request: %q", got)
				}
			}
		})
	}
}
