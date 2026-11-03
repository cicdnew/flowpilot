package main

import (
	"strings"
	"testing"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/models"
	"flowpilot/internal/monitoring"
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

func TestRateAlertRuleUsesRatio(t *testing.T) {
	app := NewApp()
	app.monitor = monitoring.New()
	rule := app.buildAlertRule(models.AlertRule{ID: "high-error-rate", Metric: "totalErrors", Cond: "rate_gt", Threshold: 0.1})

	for i := 0; i < 100; i++ {
		app.monitor.RecordRequest()
	}
	app.monitor.RecordError()
	if rule.Condition(app.monitor) {
		t.Fatal("10% error-rate alert fired at a 1% error rate")
	}

	for i := 0; i < 20; i++ {
		app.monitor.RecordError()
	}
	if !rule.Condition(app.monitor) {
		t.Fatal("10% error-rate alert did not fire at a 21% error rate")
	}
}
