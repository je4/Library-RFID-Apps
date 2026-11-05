package sip2

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCertificate(t *testing.T) (string, string, tls.Certificate, *x509.CertPool) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certPath, keyPath := filepath.Join(t.TempDir(), "cert.pem"), filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return certPath, keyPath, cert, pool
}

func TestTunnelEphemeralLoopbackPorts(t *testing.T) {
	cert, key, _, _ := testCertificate(t)
	cfg := TunnelConfig{CertificateFile: cert, KeyFile: key}
	first, err := OpenTunnel(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenTunnel(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstAddr := first.Addr().(*net.TCPAddr)
	secondAddr := second.Addr().(*net.TCPAddr)
	if !firstAddr.IP.Equal(net.ParseIP("127.0.0.1")) || !secondAddr.IP.Equal(net.ParseIP("127.0.0.1")) || firstAddr.Port == 0 || secondAddr.Port == 0 || firstAddr.Port == secondAddr.Port {
		t.Fatalf("expected distinct loopback ports, got %v and %v", firstAddr, secondAddr)
	}
}

func TestTunnelMutualTLS(t *testing.T) {
	certPath, keyPath, cert, pool := testCertificate(t)
	server, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverErr := make(chan error, 1)
	go func() {
		conn, err := server.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		_, err = io.Copy(conn, conn)
		serverErr <- err
	}()
	tunnel, err := OpenTunnel(TunnelConfig{RemoteAddr: server.Addr().String(), CertificateFile: certPath, KeyFile: keyPath, CAFile: certPath})
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	conn, err := net.Dial("tcp", tunnel.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("SIP2\r")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "SIP2\r" {
		t.Fatalf("unexpected response: %q", buf)
	}
	conn.Close()
	tunnel.Close()
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestTunnelValidation(t *testing.T) {
	if _, err := OpenTunnel(TunnelConfig{}); err == nil {
		t.Fatal("expected missing certificate error")
	}
	cert, key, _, _ := testCertificate(t)
	if _, err := OpenTunnel(TunnelConfig{CertificateFile: cert, KeyFile: key, CAFile: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected missing CA error")
	}
}

func TestCloseCancelsHandshake(t *testing.T) {
	cert, key, _, _ := testCertificate(t)
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := server.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	tunnel, err := OpenTunnel(TunnelConfig{RemoteAddr: server.Addr().String(), CertificateFile: cert, KeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", tunnel.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case remote := <-accepted:
		defer remote.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("connection not accepted")
	}
	done := make(chan struct{})
	go func() { tunnel.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("close blocked during handshake")
	}
}
