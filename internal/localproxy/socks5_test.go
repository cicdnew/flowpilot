package localproxy

import (
	"net"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestPerformSOCKS5HandshakeWithValidCredentials(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- performSOCKS5Handshake(server, "user", "pass")
	}()

	if _, err := client.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := client.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}
	if resp[1] != 0x02 {
		t.Fatalf("expected user/pass auth method, got %d", resp[1])
	}
	if _, err := client.Write([]byte{0x01, 0x04, 'u', 's', 'e', 'r', 0x04, 'p', 'a', 's', 's'}); err != nil {
		t.Fatalf("write auth payload: %v", err)
	}
	authResp := make([]byte, 2)
	if _, err := client.Read(authResp); err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if authResp[1] != 0x00 {
		t.Fatalf("expected auth success, got %d", authResp[1])
	}
	if err := <-errCh; err != nil {
		t.Fatalf("performSOCKS5Handshake: %v", err)
	}
}

func TestPerformSOCKS5HandshakeRejectsInvalidCredentials(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- performSOCKS5Handshake(server, "user", "pass")
	}()

	if _, err := client.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := client.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}
	if _, err := client.Write([]byte{0x01, 0x04, 'b', 'a', 'd', '!', 0x04, 'n', 'o', 'p', 'e'}); err != nil {
		t.Fatalf("write auth payload: %v", err)
	}
	authResp := make([]byte, 2)
	if _, err := client.Read(authResp); err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if authResp[1] != 0x01 {
		t.Fatalf("expected auth failure, got %d", authResp[1])
	}
	if err := <-errCh; err == nil {
		t.Fatal("expected invalid credentials error")
	}
}

func TestPerformSOCKS5HandshakeNoAuthRequired(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- performSOCKS5Handshake(server, "", "")
	}()

	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := client.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}
	if resp[1] != 0x00 {
		t.Fatalf("expected no auth method (0x00), got %d", resp[1])
	}
	if err := <-errCh; err != nil {
		t.Fatalf("expected no error for no-auth: %v", err)
	}
}

func TestSendHTTPConnectRequest(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		upstream := models.ProxyConfig{Server: "proxy:8080", Protocol: "http"}
		errCh <- sendHTTPConnect(server, upstream, "target.chhotu-bin.infy.uk:443")
	}()

	buf := make([]byte, 1024)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read connect request: %v", err)
	}

	request := string(buf[:n])
	if !strings.Contains(request, "CONNECT target.chhotu-bin.infy.uk:443 HTTP/1.1") {
		t.Fatalf("expected CONNECT request, got: %s", request)
	}

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		t.Fatalf("write response: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("sendHTTPConnect: %v", err)
	}
}

func TestSendHTTPConnectWithAuth(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		upstream := models.ProxyConfig{
			Server:   "proxy:8080",
			Protocol: "http",
			Username: "user",
			Password: "pass",
		}
		errCh <- sendHTTPConnect(server, upstream, "target.chhotu-bin.infy.uk:443")
	}()

	buf := make([]byte, 1024)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read connect request: %v", err)
	}

	request := string(buf[:n])
	if !strings.Contains(request, "Proxy-Authorization: Basic") {
		t.Fatalf("expected Proxy-Authorization header, got: %s", request)
	}

	if _, err := client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		t.Fatalf("write response: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("sendHTTPConnect: %v", err)
	}
}

func TestDialViaHTTPConnectConnectionRefused(t *testing.T) {
	upstream := models.ProxyConfig{Server: "127.0.0.1:1", Protocol: "http"}
	_, err := dialViaHTTPConnect(upstream, "target:80", false)
	if err == nil {
		t.Fatal("expected error for unreachable proxy")
	}
}

func TestHandleSOCKS5ClientBasicFlow(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		upstream := models.ProxyConfig{Server: "127.0.0.1:9999", Protocol: "socks5"}
		errCh <- handleSOCKS5Client(client, upstream, "user", "pass")
	}()

	if _, err := server.Write([]byte{0x05, 0x01, 0x02}); err != nil {
		t.Fatalf("write methods: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := server.Read(resp); err != nil {
		t.Fatalf("read method response: %v", err)
	}

	if _, err := server.Write([]byte{0x01, 0x04, 'u', 's', 'e', 'r', 0x04, 'p', 'a', 's', 's'}); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	authResp := make([]byte, 2)
	if _, err := server.Read(authResp); err != nil {
		t.Fatalf("read auth response: %v", err)
	}

	connectReq := []byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1, 0x00, 0x50}
	if _, err := server.Write(connectReq); err != nil {
		t.Fatalf("write connect request: %v", err)
	}

	connectResp := make([]byte, 10)
	if _, err := server.Read(connectResp); err != nil {
		t.Fatalf("read connect response: %v", err)
	}

	<-errCh
}

func TestDialViaUpstreamSOCKS5(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	var conn net.Conn
	go func() {
		var err error
		conn, err = dialViaUpstream("127.0.0.1",
			models.ProxyConfig{Server: "127.0.0.1:1080", Protocol: "socks5"},
			"chhotu-bin.infy.uk:80")
		errCh <- err
	}()

	if err := <-errCh; err == nil {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestSendHTTPConnectBadResponse(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		upstream := models.ProxyConfig{Server: "proxy:8080", Protocol: "http"}
		errCh <- sendHTTPConnect(server, upstream, "target:80")
	}()

	buf := make([]byte, 1024)
	if _, err := client.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}

	if _, err := client.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")); err != nil {
		t.Fatalf("write response: %v", err)
	}

	if err := <-errCh; err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestDialViaHTTPConnectTLS(t *testing.T) {
	upstream := models.ProxyConfig{Server: "127.0.0.1:1", Protocol: "https"}
	_, err := dialViaHTTPConnect(upstream, "target:80", true)
	if err == nil {
		t.Fatal("expected error for unreachable TLS proxy")
	}
}

func TestDialViaUpstreamHTTP(t *testing.T) {
	upstream := models.ProxyConfig{Server: "127.0.0.1:1", Protocol: "http"}
	_, err := dialViaUpstream("127.0.0.1", upstream, "target:80")
	if err == nil {
		t.Fatal("expected error for unreachable HTTP proxy")
	}
}

func TestDialViaUpstreamHTTPS(t *testing.T) {
	upstream := models.ProxyConfig{Server: "127.0.0.1:1", Protocol: "https"}
	_, err := dialViaUpstream("127.0.0.1", upstream, "target:80")
	if err == nil {
		t.Fatal("expected error for unreachable HTTPS proxy")
	}
}

func waitErr(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for result")
		return nil
	}
}

func TestHandleSOCKS5ClientBadVersion(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- handleSOCKS5Client(server, models.ProxyConfig{}, "", "")
	}()
	_, _ = client.Write([]byte{0x04, 0x01, 0x00})
	_ = client.Close()
	err := waitErr(t, errCh)
	if err == nil {
		t.Fatal("expected error for bad SOCKS version")
	}
}

func TestHandleSOCKS5ClientNoAuth(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- handleSOCKS5Client(server, models.ProxyConfig{}, "", "")
	}()
	_, _ = client.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	_, _ = client.Read(resp)
	host := "localhost"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, []byte(host)...)
	req = append(req, []byte{0x00, 0x50}...)
	_, _ = client.Write(req)
	_ = client.Close()
	waitErr(t, errCh)
}

func TestHandleSOCKS5ClientIPv4(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- handleSOCKS5Client(server, models.ProxyConfig{}, "", "")
	}()
	_, _ = client.Write([]byte{0x05, 0x01, 0x00})
	resp := make([]byte, 2)
	_, _ = client.Read(resp)
	_, _ = client.Write([]byte{0x05, 0x01, 0x00, 0x01, 127, 0, 0, 1, 0x00, 0x50})
	_ = client.Close()
	waitErr(t, errCh)
}

func TestHandleSOCKS5ClientWrongPassword(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- handleSOCKS5Client(server, models.ProxyConfig{}, "user", "pass")
	}()
	_, _ = client.Write([]byte{0x05, 0x01, 0x02})
	resp := make([]byte, 2)
	_, _ = client.Read(resp)
	authMsg := []byte{0x01, 0x04, 'u', 's', 'e', 'r', 0x04, 'w', 'r', 'o', 'g'}
	_, _ = client.Write(authMsg)
	_ = client.Close()
	err := waitErr(t, errCh)
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestHandleSOCKS5ClientWithAuth(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- handleSOCKS5Client(server, models.ProxyConfig{}, "user", "pass")
	}()
	_, _ = client.Write([]byte{0x05, 0x01, 0x02})
	resp := make([]byte, 2)
	_, _ = client.Read(resp)
	authMsg := []byte{0x01, 0x04, 'u', 's', 'e', 'r', 0x04, 'p', 'a', 's', 's'}
	_, _ = client.Write(authMsg)
	authResp := make([]byte, 2)
	_, _ = client.Read(authResp)
	_ = client.Close()
	waitErr(t, errCh)
}

func TestDialViaHTTPConnectError(t *testing.T) {
	upstream := models.ProxyConfig{Server: "127.0.0.1:1", Protocol: models.ProxyHTTP}
	_, err := dialViaHTTPConnect(upstream, "target.chhotu-bin.infy.uk:80", false)
	if err == nil {
		t.Fatal("expected error for unreachable upstream")
	}
}

func TestSendHTTPConnect(t *testing.T) {
	server, client := net.Pipe()
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	errCh := make(chan error, 1)
	go func() {
		errCh <- sendHTTPConnect(server, models.ProxyConfig{Server: "proxy:8080"}, "target.com:443")
	}()
	buf := make([]byte, 512)
	n, _ := client.Read(buf)
	req := string(buf[:n])
	if !strings.Contains(req, "CONNECT target.com:443") {
		t.Errorf("unexpected CONNECT request: %q", req)
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	err := waitErr(t, errCh)
	if err != nil {
		t.Fatalf("sendHTTPConnect: %v", err)
	}
}
