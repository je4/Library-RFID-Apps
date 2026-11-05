// Package sip2 provides connectivity to an Alma SIP2 endpoint.
package sip2

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// TunnelConfig configures a local TCP listener forwarding to Alma over mutual TLS.
type TunnelConfig struct {
	RemoteAddr      string        `toml:"remote_addr"`
	CertificateFile string        `toml:"certificate_file"`
	KeyFile         string        `toml:"key_file"`
	CAFile          string        `toml:"ca_file"`
	ConnectTimeout  time.Duration `toml:"connect_timeout"`
}

// Tunnel owns the listener and all connections accepted by it.
type Tunnel struct {
	listener net.Listener
	remote   string
	tls      *tls.Config
	timeout  time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	closed   bool
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
}

// OpenTunnel starts a local TCP-to-TLS tunnel. Close must be called to release it.
// An empty CAFile uses the system trust store. The certificate and key may be
// contained in the same PEM file.
func OpenTunnel(cfg TunnelConfig) (*Tunnel, error) {
	if cfg.RemoteAddr == "" {
		cfg.RemoteAddr = "slsp-ubs.alma.exlibrisgroup.com:6443"
	}
	if cfg.CertificateFile == "" || cfg.KeyFile == "" {
		return nil, fmt.Errorf("client certificate and key are required")
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 200 * time.Second
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertificateFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("CA file contains no certificates")
		}
		tlsConfig.RootCAs = pool
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for SIP2 connections: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tunnel{listener: listener, remote: cfg.RemoteAddr, tls: tlsConfig, timeout: cfg.ConnectTimeout, ctx: ctx, cancel: cancel, conns: make(map[net.Conn]struct{})}
	t.wg.Add(1)
	go t.serve()
	return t, nil
}

// Addr returns the local listening address, including an assigned ephemeral port.
func (t *Tunnel) Addr() net.Addr { return t.listener.Addr() }

func (t *Tunnel) serve() {
	defer t.wg.Done()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			conn.Close()
			return
		}
		t.conns[conn] = struct{}{}
		t.wg.Add(1)
		t.mu.Unlock()
		go t.forward(conn)
	}
}

func (t *Tunnel) forward(local net.Conn) {
	defer t.wg.Done()
	defer t.remove(local)
	ctx, cancel := context.WithTimeout(t.ctx, t.timeout)
	defer cancel()
	dialer := tls.Dialer{Config: t.tls}
	remote, err := dialer.DialContext(ctx, "tcp", t.remote)
	if err != nil {
		return
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		remote.Close()
		return
	}
	t.conns[remote] = struct{}{}
	t.mu.Unlock()
	defer t.remove(remote)
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(remote, local)
		remote.Close()
		close(done)
	}()
	_, _ = io.Copy(local, remote)
	local.Close()
	<-done
}

func (t *Tunnel) remove(conn net.Conn) {
	conn.Close()
	t.mu.Lock()
	delete(t.conns, conn)
	t.mu.Unlock()
}

// Close stops the listener and active connections. It is safe to call repeatedly.
func (t *Tunnel) Close() error {
	t.mu.Lock()
	if !t.closed {
		t.closed = true
		t.cancel()
		t.listener.Close()
		for conn := range t.conns {
			conn.Close()
		}
	}
	t.mu.Unlock()
	t.wg.Wait()
	return nil
}
