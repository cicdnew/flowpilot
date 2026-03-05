package models

import "testing"

func TestProxyToProxyConfig(t *testing.T) {
	p := &Proxy{
		ID:       "test-id",
		Server:   "proxy.example.com:8080",
		Protocol: ProxyHTTP,
		Username: "user",
		Password: "pass",
		Geo:      "US",
	}

	config := p.ToProxyConfig()

	if config.Server != p.Server {
		t.Errorf("Server: got %q, want %q", config.Server, p.Server)
	}
	if config.Protocol != string(p.Protocol) {
		t.Errorf("Protocol: got %q, want %q", config.Protocol, p.Protocol)
	}
	if config.Username != p.Username {
		t.Errorf("Username: got %q, want %q", config.Username, p.Username)
	}
	if config.Password != p.Password {
		t.Errorf("Password: got %q, want %q", config.Password, p.Password)
	}
	if config.Geo != p.Geo {
		t.Errorf("Geo: got %q, want %q", config.Geo, p.Geo)
	}
}

func TestProxyToProxyConfigEmpty(t *testing.T) {
	p := &Proxy{}
	config := p.ToProxyConfig()

	if config.Server != "" {
		t.Errorf("Server: got %q, want empty", config.Server)
	}
	if config.Username != "" {
		t.Errorf("Username: got %q, want empty", config.Username)
	}
}
