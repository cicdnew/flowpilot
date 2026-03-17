package localproxy

import (
	"net"
	"testing"
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
