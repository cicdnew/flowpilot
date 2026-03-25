package main

import "testing"

func TestAddProxyWithRateLimitRejectsNegativeLimit(t *testing.T) {
	app := setupTestApp(t)
	if _, err := app.AddProxyWithRateLimit("127.0.0.1:8080", "http", "", "", "US", -1); err == nil {
		t.Fatal("expected error for negative rate limit")
	}
}
