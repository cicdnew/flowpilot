package proxy

import (
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestRateLimitStatusFiltersLimitedProxies(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.requestTimes["limited"] = []time.Time{now.Add(-10 * time.Second)}

	available, wait := m.rateLimitStatus([]models.Proxy{
		{ID: "limited", MaxRequestsPerMinute: 1},
		{ID: "open", MaxRequestsPerMinute: 2},
	})
	if len(available) != 1 || available[0].ID != "open" {
		t.Fatalf("available proxies = %#v, want only open", available)
	}
	if wait <= 0 {
		t.Fatalf("wait = %s, want positive", wait)
	}
}

func TestRecordSelectionLockedTracksRequestTimes(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	m.mu.Lock()
	m.recordSelectionLocked(models.Proxy{ID: "p1", MaxRequestsPerMinute: 5})
	m.mu.Unlock()

	if got := len(m.requestTimes["p1"]); got != 1 {
		t.Fatalf("requestTimes count = %d, want 1", got)
	}
	if !m.requestTimes["p1"][0].Equal(now) {
		t.Fatalf("request time = %v, want %v", m.requestTimes["p1"][0], now)
	}
}
