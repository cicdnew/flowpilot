package localproxy

import (
	"strings"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestEndpointReturnsAuthenticatedLocalProxyConfig(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:8080", Protocol: models.ProxyHTTP}
	local, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("Endpoint: %v", err)
	}
	if !strings.HasPrefix(local.Server, "127.0.0.1:") {
		t.Fatalf("expected localhost endpoint, got %s", local.Server)
	}
	if local.Protocol != models.ProxySOCKS5 {
		t.Fatalf("expected socks5 local endpoint, got %s", local.Protocol)
	}
	if local.Username == "" || local.Password == "" {
		t.Fatal("expected generated local credentials")
	}
}

func TestEndpointReusesCredentialsWhileEndpointAlive(t *testing.T) {
	m := NewManager(time.Minute)
	defer m.Stop()

	cfg := models.ProxyConfig{Server: "proxy.example:8080", Protocol: models.ProxyHTTP}
	first, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("first Endpoint: %v", err)
	}
	second, err := m.Endpoint(cfg)
	if err != nil {
		t.Fatalf("second Endpoint: %v", err)
	}
	if first.Server != second.Server {
		t.Fatalf("expected same local endpoint, got %s and %s", first.Server, second.Server)
	}
	if first.Username != second.Username || first.Password != second.Password {
		t.Fatal("expected local credentials to be reused for cached endpoint")
	}
}
