package main

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestGetPrometheusMetricsReportsProxyScrapeErrorsBestEffort(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	proxy := models.Proxy{
		ID:        "proxy-good",
		Server:    "http://proxy.example.com:8080",
		Protocol:  "http",
		Username:  "user",
		Password:  "pass",
		Geo:       "US",
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	if err := app.db.CreateProxy(ctx, proxy); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	badCiphertext := base64.StdEncoding.EncodeToString([]byte("this-is-not-a-valid-aes-gcm-ciphertext-padding-x"))
	_, err := app.db.Conn().ExecContext(ctx, `
		INSERT INTO proxies (id, server, protocol, username, password, geo, status, max_requests_per_minute, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "proxy-bad", "http://bad-proxy:8080", "http", badCiphertext, badCiphertext, "CA", "unknown", 0, time.Now())
	if err != nil {
		t.Fatalf("insert bad proxy row: %v", err)
	}

	metrics := app.GetPrometheusMetrics()
	if !strings.Contains(metrics, "flowpilot_proxies_total 1") {
		t.Fatalf("expected metrics to count one valid proxy, got:\n%s", metrics)
	}
	if !strings.Contains(metrics, "flowpilot_proxies_scrape_errors 1") {
		t.Fatalf("expected metrics to report one scrape error, got:\n%s", metrics)
	}
}
