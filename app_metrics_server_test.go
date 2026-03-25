package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMetricsServerServesPrometheusMetrics(t *testing.T) {
	app := NewApp()
	app.config = DefaultAppConfig()
	app.config.MetricsAddr = "127.0.0.1:0"
	app.startMetricsServer(t.Context())
	defer app.stopMetricsServer(t.Context())

	addr := app.MetricsAddress()
	if addr == "" {
		t.Fatal("expected metrics address")
	}
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	for _, want := range []string{"flowpilot_queue_running", "flowpilot_proxies_total", "flowpilot_browser_pool_browsers", "flowpilot_proxy_browser_pools"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("unexpected metrics body, missing %q: %s", want, body)
		}
	}
}
