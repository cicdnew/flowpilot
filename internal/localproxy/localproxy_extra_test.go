package localproxy

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/models"
)

// TestManagerEmptyServerPassthrough checks that an empty server is returned as-is.
func TestManagerEmptyServerPassthrough(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "", Protocol: models.ProxyHTTP}
	result, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if result.Server != "" {
		t.Errorf("expected empty server passthrough, got %q", result.Server)
	}
}

// TestManagerWhitespaceServerPassthrough checks that a whitespace-only server is passed through.
func TestManagerWhitespaceServerPassthrough(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "   ", Protocol: models.ProxyHTTP}
	result, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if result.Server != "   " {
		t.Errorf("expected whitespace server passthrough, got %q", result.Server)
	}
}

// TestManagerDefaultIdleTimeout verifies zero idle timeout uses default.
func TestManagerDefaultIdleTimeout(t *testing.T) {
	m := NewManager(0) // should apply default 5 minutes
	defer m.Stop()
	if m.idleTimeout != 5*time.Minute {
		t.Errorf("expected 5m idle timeout, got %v", m.idleTimeout)
	}
}

// TestManagerStats checks stats counters update correctly.
func TestManagerStats(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:8080", Protocol: models.ProxyHTTP}
	_, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("first Endpoint: %v", err)
	}
	_, err = m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("second Endpoint: %v", err)
	}

	stats := m.Stats()
	if stats.EndpointCreations != 1 {
		t.Errorf("expected 1 creation, got %d", stats.EndpointCreations)
	}
	if stats.EndpointReuses != 1 {
		t.Errorf("expected 1 reuse, got %d", stats.EndpointReuses)
	}
	if stats.ActiveEndpoints != 1 {
		t.Errorf("expected 1 active endpoint, got %d", stats.ActiveEndpoints)
	}
}

// TestManagerRecordAuthFailure checks auth failure recording.
func TestManagerRecordAuthFailure(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	m.RecordAuthFailure(nil)
	m.RecordAuthFailure(nil)

	stats := m.Stats()
	if stats.AuthFailures != 2 {
		t.Errorf("expected 2 auth failures, got %d", stats.AuthFailures)
	}
}

// TestManagerRecordAuthFailureWithError checks last error is recorded.
func TestManagerRecordAuthFailureWithError(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	m.RecordAuthFailure(fmt.Errorf("bad creds"))

	stats := m.Stats()
	if stats.AuthFailures != 1 {
		t.Errorf("expected 1 auth failure, got %d", stats.AuthFailures)
	}
	if stats.LastError != "bad creds" {
		t.Errorf("expected last error 'bad creds', got %q", stats.LastError)
	}
}

// TestManagerRecordUpstreamFailure checks upstream failure recording.
func TestManagerRecordUpstreamFailure(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	m.RecordUpstreamFailure(fmt.Errorf("upstream down"))

	stats := m.Stats()
	if stats.UpstreamFailures != 1 {
		t.Errorf("expected 1 upstream failure, got %d", stats.UpstreamFailures)
	}
	if stats.LastError != "upstream down" {
		t.Errorf("expected last error 'upstream down', got %q", stats.LastError)
	}
}

// TestManagerEndpointAddr checks EndpointAddr returns addr for active endpoints.
func TestManagerEndpointAddr(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:9999", Protocol: models.ProxyHTTP}
	local, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}

	addr := m.EndpointAddr(cfg)
	if addr != local.Server {
		t.Errorf("expected addr %q, got %q", local.Server, addr)
	}
}

// TestManagerEndpointAddrMissing returns empty string for unknown proxy.
func TestManagerEndpointAddrMissing(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "missing.example:1234", Protocol: models.ProxyHTTP}
	addr := m.EndpointAddr(cfg)
	if addr != "" {
		t.Errorf("expected empty addr, got %q", addr)
	}
}

// TestManagerEndpointStatsByProxy checks active count mapping.
func TestManagerEndpointStatsByProxy(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:8080", Protocol: models.ProxyHTTP}
	_, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}

	proxy := models.Proxy{
		ID:       "p1",
		Server:   "proxy.example:8080",
		Protocol: models.ProxyHTTP,
	}
	stats := m.EndpointStatsByProxy([]models.Proxy{proxy})
	if _, ok := stats["p1"]; !ok {
		t.Error("expected stats entry for proxy p1")
	}
}

// TestManagerStopClearsEndpoints verifies Stop removes all active endpoints.
func TestManagerStopClearsEndpoints(t *testing.T) {
	m := NewManager(time.Minute)

	cfg := models.ProxyConfig{Server: "proxy.example:7777", Protocol: models.ProxyHTTP}
	_, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}

	m.Stop()

	stats := m.Stats()
	if stats.ActiveEndpoints != 0 {
		t.Errorf("expected 0 active endpoints after Stop, got %d", stats.ActiveEndpoints)
	}
}

// TestManagerPruneIdle checks pruneIdle removes endpoints past their idle timeout.
func TestManagerPruneIdle(t *testing.T) {
	m := NewManager(1 * time.Millisecond)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:6666", Protocol: models.ProxyHTTP}
	_, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}

	// Wait for idle timeout to expire
	time.Sleep(10 * time.Millisecond)
	m.pruneIdle()

	stats := m.Stats()
	if stats.ActiveEndpoints != 0 {
		t.Errorf("expected 0 active endpoints after pruneIdle, got %d", stats.ActiveEndpoints)
	}
}

// TestManagerDifferentUpstreamsGetDifferentEndpoints verifies distinct proxies get distinct endpoints.
func TestManagerDifferentUpstreamsGetDifferentEndpoints(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg1 := models.ProxyConfig{Server: "proxy1.example:8080", Protocol: models.ProxyHTTP}
	cfg2 := models.ProxyConfig{Server: "proxy2.example:8080", Protocol: models.ProxyHTTP}

	ep1, err := m.Endpoint(cfg1)
	if err != nil {
		t.Fatalf("Endpoint cfg1: %v", err)
	}
	ep2, err := m.Endpoint(cfg2)
	if err != nil {
		t.Fatalf("Endpoint cfg2: %v", err)
	}

	if ep1.Server == ep2.Server {
		t.Errorf("expected distinct endpoints, both got %q", ep1.Server)
	}

	stats := m.Stats()
	if stats.ActiveEndpoints != 2 {
		t.Errorf("expected 2 active endpoints, got %d", stats.ActiveEndpoints)
	}
}

// TestSOCKS5HandshakeNoAuth verifies handshake with no-auth method.
func TestSOCKS5HandshakeNoAuth(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- performSOCKS5Handshake(server, "", "")
	}()

	// Offer no-auth method (0x00)
	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := client.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}
	if resp[1] != 0x00 {
		t.Fatalf("expected no-auth method, got %d", resp[1])
	}
	if err := <-errCh; err != nil {
		t.Fatalf("performSOCKS5Handshake: %v", err)
	}
}

// TestSOCKS5HandshakeNoAcceptableMethod verifies error when no compatible method.
func TestSOCKS5HandshakeNoAcceptableMethod(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		// server expects auth (username != ""), client offers only no-auth
		errCh <- performSOCKS5Handshake(server, "user", "pass")
	}()

	// Offer only no-auth method (0x00), but server wants 0x02
	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := client.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}
	if resp[1] != 0xFF {
		t.Fatalf("expected no-acceptable-method (0xFF), got %d", resp[1])
	}
	// Server returns 0xFF then errors; close client so goroutine can finish
	client.Close()
	if err := <-errCh; err == nil {
		t.Fatal("expected error for no acceptable method")
	}
}

// TestSOCKS5HandshakeWrongVersion verifies error on wrong SOCKS version.
func TestSOCKS5HandshakeWrongVersion(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- performSOCKS5Handshake(server, "user", "pass")
	}()

	// Send SOCKS4 version (only 2-byte header needed; server reads 2 then errors)
	if _, err := client.Write([]byte{0x04, 0x01}); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Close client so the server goroutine can finish
	client.Close()

	if err := <-errCh; err == nil {
		t.Fatal("expected error for wrong SOCKS version")
	} else if !strings.Contains(err.Error(), "unsupported socks version") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSOCKS5AuthWrongVersion verifies error on wrong auth sub-version.
func TestSOCKS5AuthWrongVersion(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- authenticateSOCKS5UserPass(server, "user", "pass")
	}()

	// Send wrong auth version (0x02 instead of 0x01); server reads 2-byte header then errors
	if _, err := client.Write([]byte{0x02, 0x04}); err != nil {
		t.Fatalf("write: %v", err)
	}
	client.Close()

	if err := <-errCh; err == nil {
		t.Fatal("expected error for wrong auth version")
	} else if !strings.Contains(err.Error(), "unsupported socks5 auth version") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestReadSOCKS5ConnectRequestIPv4 verifies IPv4 address type parsing.
func TestReadSOCKS5ConnectRequestIPv4(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		target, err := readSOCKS5ConnectRequest(server)
		resultCh <- target
		errCh <- err
	}()

	// VER=5, CMD=CONNECT, RSV=0, ATYP=IPv4, addr=127.0.0.1, port=80
	if _, err := client.Write([]byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1, 0x00, 0x50}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("readSOCKS5ConnectRequest: %v", err)
	}
	target := <-resultCh
	if target != "127.0.0.1:80" {
		t.Errorf("expected 127.0.0.1:80, got %q", target)
	}
}

// TestReadSOCKS5ConnectRequestDomain verifies domain address type parsing.
func TestReadSOCKS5ConnectRequestDomain(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		target, err := readSOCKS5ConnectRequest(server)
		resultCh <- target
		errCh <- err
	}()

	// VER=5, CMD=CONNECT, RSV=0, ATYP=domain, len=11, "example.com", port=443
	domain := []byte("example.com")
	msg := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))}
	msg = append(msg, domain...)
	msg = append(msg, 0x01, 0xBB) // port 443
	if _, err := client.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("readSOCKS5ConnectRequest: %v", err)
	}
	target := <-resultCh
	if target != "example.com:443" {
		t.Errorf("expected example.com:443, got %q", target)
	}
}

// TestReadSOCKS5ConnectRequestIPv6 verifies IPv6 address type parsing.
func TestReadSOCKS5ConnectRequestIPv6(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		target, err := readSOCKS5ConnectRequest(server)
		resultCh <- target
		errCh <- err
	}()

	// VER=5, CMD=CONNECT, RSV=0, ATYP=IPv6, addr=::1, port=8080
	ipv6 := make([]byte, 16)
	ipv6[15] = 1 // ::1
	msg := []byte{0x05, 0x01, 0x00, 0x04}
	msg = append(msg, ipv6...)
	msg = append(msg, 0x1F, 0x90) // port 8080
	if _, err := client.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("readSOCKS5ConnectRequest: %v", err)
	}
	target := <-resultCh
	if !strings.Contains(target, "8080") {
		t.Errorf("expected port 8080 in target %q", target)
	}
}

// TestReadSOCKS5ConnectRequestUnsupportedCmd verifies error on non-CONNECT command.
func TestReadSOCKS5ConnectRequestUnsupportedCmd(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := readSOCKS5ConnectRequest(server)
		errCh <- err
	}()

	// CMD=2 (BIND, not CONNECT); server reads 4 bytes then errors
	if _, err := client.Write([]byte{0x05, 0x02, 0x00, 0x01}); err != nil {
		t.Fatalf("write: %v", err)
	}
	client.Close()

	if err := <-errCh; err == nil {
		t.Fatal("expected error for unsupported SOCKS5 command")
	}
}

// TestReadSOCKS5ConnectRequestUnsupportedAddrType verifies error on unknown address type.
func TestReadSOCKS5ConnectRequestUnsupportedAddrType(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := readSOCKS5ConnectRequest(server)
		errCh <- err
	}()

	// ATYP=0xFF (unknown)
	if _, err := client.Write([]byte{0x05, 0x01, 0x00, 0xFF}); err != nil {
		t.Fatalf("write: %v", err)
	}
	client.Close()

	if err := <-errCh; err == nil {
		t.Fatal("expected error for unsupported address type")
	}
}

// TestDialViaUpstreamUnsupportedProtocol verifies error for unknown protocol.
func TestDialViaUpstreamUnsupportedProtocol(t *testing.T) {
	upstream := models.ProxyConfig{
		Server:   "proxy.example:8080",
		Protocol: "ftp",
	}
	_, err := dialViaUpstream("", upstream, "target.example:80")
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
	if !strings.Contains(err.Error(), "unsupported upstream proxy protocol") {
		t.Errorf("unexpected error: %v", err)
	}
}
