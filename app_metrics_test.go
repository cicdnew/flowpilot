package main

import (
	"strings"
	"testing"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/models"
)

func TestGetPrometheusMetricsIncludesProxyAndPoolMetrics(t *testing.T) {
	app := setupTestApp(t)
	runner, err := browser.NewRunner(t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	app.runner = runner
	app.pool = browser.NewBrowserPool(browser.PoolConfig{
		Size:           2,
		MaxTabs:        3,
		IdleTimeout:    time.Minute,
		AcquireTimeout: time.Second,
	}, nil)
	defer app.runner.StopProxyPools()
	defer app.pool.Stop()
	app.runner.SetPool(app.pool)

	proxyA := models.Proxy{
		ID:        "p1",
		Server:    "127.0.0.1:8001",
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	proxyB := models.Proxy{
		ID:        "p2",
		Server:    "127.0.0.1:8002",
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusUnhealthy,
		CreatedAt: time.Now(),
	}
	if err := app.db.CreateProxy(app.ctx, proxyA); err != nil {
		t.Fatalf("CreateProxy A: %v", err)
	}
	if err := app.db.CreateProxy(app.ctx, proxyB); err != nil {
		t.Fatalf("CreateProxy B: %v", err)
	}

	metrics := app.GetPrometheusMetrics()
	for _, want := range []string{
		"flowpilot_proxies_total 2",
		"flowpilot_proxies_healthy 1",
		"flowpilot_proxies_unhealthy 1",
		"flowpilot_browser_pool_browsers",
		"flowpilot_proxy_browser_pools",
	} {
		if !strings.Contains(metrics, want) {
			t.Fatalf("metrics missing %q:\n%s", want, metrics)
		}
	}
}
