package sip2

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, server func(net.Conn)) *Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			server(conn)
		}
	}()
	client, err := Dial(context.Background(), listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestClientLoginAndExchange(t *testing.T) {
	requests := make(chan string, 2)
	client := testClient(t, func(conn net.Conn) {
		reader := bufio.NewReader(conn)
		for _, response := range []string{"941\r", "24response\r"} {
			request, err := reader.ReadString('\r')
			if err != nil {
				return
			}
			requests <- request
			conn.Write([]byte(response))
		}
	})
	ok, err := client.Login(context.Background(), "user", "secret", "branch")
	if err != nil || !ok {
		t.Fatalf("login: %v, %v", ok, err)
	}
	if got := <-requests; got != "9300CNuser|COsecret|CPbranch|\r" {
		t.Fatalf("login request: %q", got)
	}
	response, err := client.Exchange(context.Background(), "23request", "24")
	if err != nil || response != "24response" {
		t.Fatalf("exchange: %q, %v", response, err)
	}
	if got := <-requests; got != "23request\r" {
		t.Fatalf("request: %q", got)
	}
}

func TestClientLoginRejectedAndInvalid(t *testing.T) {
	client := testClient(t, func(conn net.Conn) {
		bufio.NewReader(conn).ReadString('\r')
		conn.Write([]byte("940\r"))
	})
	if _, err := client.Login(context.Background(), "bad|user", "secret", ""); err == nil {
		t.Fatal("expected invalid field error")
	}
	ok, err := client.Login(context.Background(), "user", "secret", "")
	if err != nil || ok {
		t.Fatalf("rejected login: %v, %v", ok, err)
	}
}

func TestClientInvalidResponseClosesConnection(t *testing.T) {
	for _, response := range []string{"950\r", "94\x00\r", strings.Repeat("x", maxMessageSize)} {
		t.Run(response[:2], func(t *testing.T) {
			client := testClient(t, func(conn net.Conn) {
				bufio.NewReader(conn).ReadString('\r')
				conn.Write([]byte(response))
			})
			if _, err := client.Exchange(context.Background(), "93", "94"); err == nil {
				t.Fatal("expected response error")
			}
			if _, err := client.Exchange(context.Background(), "93", "94"); err == nil {
				t.Fatal("expected closed connection")
			}
		})
	}
}

func TestClientCancellation(t *testing.T) {
	received := make(chan struct{})
	client := testClient(t, func(conn net.Conn) {
		bufio.NewReader(conn).ReadString('\r')
		close(received)
		bufio.NewReader(conn).ReadString('\r')
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Exchange(ctx, "93", "94")
		done <- err
	}()
	<-received
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("exchange did not stop")
	}
}
