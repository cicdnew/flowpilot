package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

func setupTestManager(t *testing.T, strategy models.RotationStrategy) (*Manager, *database.DB) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatalf("init crypto key: %v", err)
	}
	t.Cleanup(func() { crypto.ResetForTest() })

	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	config := models.ProxyPoolConfig{
		Strategy:            strategy,
		HealthCheckInterval: 300,
		MaxFailures:         3,
		HealthCheckURL:      "https://httpbin.org/ip",
	}

	m := NewManager(db, config)
	t.Cleanup(func() { m.Stop() })
	return m, db
}

func getProxy(t *testing.T, db *database.DB, id string) models.Proxy {
	t.Helper()
	proxies, err := db.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	for _, p := range proxies {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("proxy %s not found", id)
	return models.Proxy{}
}

func addHealthyProxy(t *testing.T, db *database.DB, id, server, geo string, latency, totalUsed int) {
	t.Helper()
	p := models.Proxy{
		ID:        id,
		Server:    server,
		Protocol:  models.ProxyHTTP,
		Geo:       geo,
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy %s: %v", id, err)
	}
	if err := db.UpdateProxyHealth(context.Background(), id, models.ProxyStatusHealthy, latency); err != nil {
		t.Fatalf("UpdateProxyHealth %s: %v", id, err)
	}
	// Simulate usage by incrementing the counter
	for i := 0; i < totalUsed; i++ {
		if err := db.IncrementProxyUsage(context.Background(), id, true); err != nil {
			t.Fatalf("IncrementProxyUsage %s: %v", id, err)
		}
	}
}

// --- NewManager Tests ---

func TestNewManagerDefaults(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	m := NewManager(db, models.ProxyPoolConfig{})
	defer m.Stop()

	if m.config.HealthCheckURL != "https://httpbin.org/ip" {
		t.Errorf("HealthCheckURL: got %q, want default", m.config.HealthCheckURL)
	}
	if m.config.HealthCheckInterval != 300 {
		t.Errorf("HealthCheckInterval: got %d, want 300", m.config.HealthCheckInterval)
	}
	if m.config.MaxFailures != 3 {
		t.Errorf("MaxFailures: got %d, want 3", m.config.MaxFailures)
	}
}

// --- SelectProxy Round-Robin Tests ---

func TestSelectProxyRoundRobin(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "rr-1", "proxy1.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "rr-2", "proxy2.example.com:8080", "US", 200, 0)
	addHealthyProxy(t, db, "rr-3", "proxy3.example.com:8080", "US", 150, 0)

	// Round-robin should cycle through proxies
	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		p, err := m.SelectProxy("")
		if err != nil {
			t.Fatalf("SelectProxy %d: %v", i, err)
		}
		seen[p.ID]++
	}

	// Each proxy should be selected 3 times over 9 iterations
	for id, count := range seen {
		if count != 3 {
			t.Errorf("proxy %s selected %d times, want 3", id, count)
		}
	}
}

func TestSelectProxyRoundRobinWrapsAround(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "wrap-1", "p1.example.com:8080", "", 100, 0)
	addHealthyProxy(t, db, "wrap-2", "p2.example.com:8080", "", 200, 0)

	// Select more times than there are proxies
	for i := 0; i < 10; i++ {
		_, err := m.SelectProxy("")
		if err != nil {
			t.Fatalf("SelectProxy %d: %v", i, err)
		}
	}
}

// --- SelectProxy Random Tests ---

func TestSelectProxyRandom(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRandom)

	addHealthyProxy(t, db, "rand-1", "r1.example.com:8080", "", 100, 0)
	addHealthyProxy(t, db, "rand-2", "r2.example.com:8080", "", 200, 0)
	addHealthyProxy(t, db, "rand-3", "r3.example.com:8080", "", 150, 0)

	// Run many selections to ensure no panics and reasonable distribution
	seen := make(map[string]int)
	for i := 0; i < 100; i++ {
		p, err := m.SelectProxy("")
		if err != nil {
			t.Fatalf("SelectProxy %d: %v", i, err)
		}
		seen[p.ID]++
	}

	// All proxies should be selected at least once in 100 tries
	for _, id := range []string{"rand-1", "rand-2", "rand-3"} {
		if seen[id] == 0 {
			t.Errorf("proxy %s never selected in 100 random selections", id)
		}
	}
}

// --- SelectProxy Least-Used Tests ---

func TestSelectProxyLeastUsed(t *testing.T) {
	m, db := setupTestManager(t, models.RotationLeastUsed)

	addHealthyProxy(t, db, "lu-1", "lu1.example.com:8080", "", 100, 10)
	addHealthyProxy(t, db, "lu-2", "lu2.example.com:8080", "", 100, 2)
	addHealthyProxy(t, db, "lu-3", "lu3.example.com:8080", "", 100, 5)

	p, err := m.SelectProxy("")
	if err != nil {
		t.Fatalf("SelectProxy: %v", err)
	}

	// The proxy with TotalUsed=2 should be selected,
	// but note: db.ListHealthyProxies orders by success_rate DESC, latency ASC.
	// In the slice, we're looking for the one with lowest TotalUsed.
	// Since all have the same latency after UpdateProxyHealth, order might vary.
	// The least-used algorithm should find the one with TotalUsed=2 regardless of slice order.
	if p.ID != "lu-2" {
		t.Errorf("expected least-used proxy lu-2, got %s (totalUsed=%d)", p.ID, p.TotalUsed)
	}
}

// --- SelectProxy Lowest-Latency Tests ---

func TestSelectProxyLowestLatency(t *testing.T) {
	m, db := setupTestManager(t, models.RotationLowestLatency)

	addHealthyProxy(t, db, "lat-1", "lat1.example.com:8080", "", 200, 0)
	addHealthyProxy(t, db, "lat-2", "lat2.example.com:8080", "", 50, 0)
	addHealthyProxy(t, db, "lat-3", "lat3.example.com:8080", "", 150, 0)

	p, err := m.SelectProxy("")
	if err != nil {
		t.Fatalf("SelectProxy: %v", err)
	}

	if p.ID != "lat-2" {
		t.Errorf("expected lowest-latency proxy lat-2, got %s (latency=%d)", p.ID, p.Latency)
	}
}

func TestSelectProxyLowestLatencyIgnoresZero(t *testing.T) {
	m, db := setupTestManager(t, models.RotationLowestLatency)

	addHealthyProxy(t, db, "lz-1", "lz1.example.com:8080", "", 100, 0)
	// lz-2 gets latency set to 0 (unchecked) - add with latency 0 but mark healthy
	p := models.Proxy{
		ID:        "lz-2",
		Server:    "lz2.example.com:8080",
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusHealthy,
		Latency:   0,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}
	if err := db.UpdateProxyHealth(context.Background(), "lz-2", models.ProxyStatusHealthy, 0); err != nil {
		t.Fatalf("UpdateProxyHealth: %v", err)
	}

	result, err := m.SelectProxy("")
	if err != nil {
		t.Fatalf("SelectProxy: %v", err)
	}

	// Should prefer lz-1 with latency 100 over lz-2 with latency 0
	if result.ID != "lz-1" {
		t.Errorf("expected lz-1 (latency=100), got %s (latency=%d)", result.ID, result.Latency)
	}
}

// --- Geo Filtering Tests ---

func TestSelectProxyWithGeoFilter(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "geo-us-1", "us1.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "geo-uk-1", "uk1.example.com:8080", "UK", 100, 0)
	addHealthyProxy(t, db, "geo-us-2", "us2.example.com:8080", "US", 100, 0)

	p, err := m.SelectProxy("UK")
	if err != nil {
		t.Fatalf("SelectProxy(UK): %v", err)
	}
	if p.Geo != "UK" {
		t.Errorf("expected UK proxy, got geo=%s", p.Geo)
	}
	if p.ID != "geo-uk-1" {
		t.Errorf("expected geo-uk-1, got %s", p.ID)
	}
}

func TestSelectProxyWithGeoRandomizesWithinCountryPool(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "geo-rand-us-1", "us1.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "geo-rand-us-2", "us2.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "geo-rand-uk-1", "uk1.example.com:8080", "UK", 100, 0)

	seen := map[string]int{}
	for i := 0; i < 100; i++ {
		p, err := m.SelectProxy("us")
		if err != nil {
			t.Fatalf("SelectProxy(us) %d: %v", i, err)
		}
		if p.Geo != "US" {
			t.Fatalf("expected US proxy, got %s", p.Geo)
		}
		seen[p.ID]++
	}
	if seen["geo-rand-us-1"] == 0 || seen["geo-rand-us-2"] == 0 {
		t.Fatalf("expected both US proxies to be selected randomly, got %+v", seen)
	}
	if seen["geo-rand-uk-1"] != 0 {
		t.Fatalf("expected UK proxy to never be selected for US geo, got %+v", seen)
	}
}

func TestSelectProxyNoMatchingGeo(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "geo-only-us", "us.example.com:8080", "US", 100, 0)

	_, err := m.SelectProxy("JP")
	if err == nil {
		t.Fatal("expected error when no proxies match geo filter")
	}
}

func TestSelectProxyNoHealthyProxies(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	// Add an unhealthy proxy
	p := models.Proxy{
		ID:        "unhealthy-1",
		Server:    "bad.example.com:8080",
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusUnhealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	_, err := m.SelectProxy("")
	if err == nil {
		t.Fatal("expected error when no healthy proxies available")
	}
}

func TestSelectProxyEmptyPool(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)

	_, err := m.SelectProxy("")
	if err == nil {
		t.Fatal("expected error for empty proxy pool")
	}
}

// --- Default Strategy Tests ---

func TestSelectProxyDefaultStrategy(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	config := models.ProxyPoolConfig{
		Strategy: "unknown_strategy",
	}
	m := NewManager(db, config)
	defer m.Stop()

	addHealthyProxy(t, db, "def-1", "def.example.com:8080", "", 100, 0)

	p, err := m.SelectProxy("")
	if err != nil {
		t.Fatalf("SelectProxy with default strategy: %v", err)
	}
	if p == nil {
		t.Fatal("expected a proxy, got nil")
	}
}

// --- RecordUsage Tests ---

func TestRecordUsage(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "usage-1", "usage.example.com:8080", "", 100, 0)

	if err := m.RecordUsage("usage-1", true); err != nil {
		t.Fatalf("RecordUsage(success): %v", err)
	}
	if err := m.RecordUsage("usage-1", false); err != nil {
		t.Fatalf("RecordUsage(failure): %v", err)
	}
}

func TestReserveProxyTracksAndReleasesReservations(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "reserve-1", "reserve.example.com:8080", "", 100, 0)

	lease, err := m.ReserveProxy("")
	if err != nil {
		t.Fatalf("ReserveProxy: %v", err)
	}
	if got := m.ActiveReservations("reserve-1"); got != 1 {
		t.Fatalf("ActiveReservations: got %d, want 1", got)
	}
	if err := lease.Complete(true); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if got := m.ActiveReservations("reserve-1"); got != 0 {
		t.Fatalf("ActiveReservations after complete: got %d, want 0", got)
	}

	proxyState := getProxy(t, db, "reserve-1")
	if proxyState.TotalUsed != 1 {
		t.Fatalf("TotalUsed after completion: got %d, want 1", proxyState.TotalUsed)
	}
}

func TestReserveProxyPrefersLowerReservationPressure(t *testing.T) {
	m, db := setupTestManager(t, models.RotationLeastUsed)

	addHealthyProxy(t, db, "pressure-1", "p1.example.com:8080", "", 100, 0)
	addHealthyProxy(t, db, "pressure-2", "p2.example.com:8080", "", 100, 0)

	first, err := m.ReserveProxy("")
	if err != nil {
		t.Fatalf("first ReserveProxy: %v", err)
	}
	second, err := m.ReserveProxy("")
	if err != nil {
		t.Fatalf("second ReserveProxy: %v", err)
	}
	if first.ProxyID() == second.ProxyID() {
		t.Fatalf("expected second reservation to prefer lower-pressure proxy, got %s twice", first.ProxyID())
	}
	_ = first.Release()
	_ = second.Release()
}

func TestReserveProxyWithGeoRandomizesWithinCountryPool(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)

	addHealthyProxy(t, db, "reserve-us-1", "us1.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "reserve-us-2", "us2.example.com:8080", "US", 100, 0)
	addHealthyProxy(t, db, "reserve-fr-1", "fr1.example.com:8080", "FR", 100, 0)

	seen := map[string]int{}
	for i := 0; i < 50; i++ {
		lease, err := m.ReserveProxy("US")
		if err != nil {
			t.Fatalf("ReserveProxy(US) %d: %v", i, err)
		}
		if lease.Proxy().Geo != "US" {
			t.Fatalf("expected reserved proxy geo US, got %s", lease.Proxy().Geo)
		}
		seen[lease.ProxyID()]++
		if err := lease.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}
	}
	if seen["reserve-us-1"] == 0 || seen["reserve-us-2"] == 0 {
		t.Fatalf("expected both US proxies to be reserved over time, got %+v", seen)
	}
	if seen["reserve-fr-1"] != 0 {
		t.Fatalf("expected FR proxy never to be reserved for US, got %+v", seen)
	}
}

// --- Stop Tests ---

func TestStopIdempotent(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)

	// Should not panic
	m.Stop()
	m.Stop()
	m.Stop()
}

// --- StartHealthChecks Tests ---

func TestStartHealthChecksRespondsToStop(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	config := models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: 1,                    // 1 second for fast test
		HealthCheckURL:      "http://localhost:1", // Will fail fast
	}
	m := NewManager(db, config)

	done := make(chan struct{})
	go func() {
		m.StartHealthChecks(context.Background())
		close(done)
	}()

	// Stop should cause StartHealthChecks to return
	time.Sleep(100 * time.Millisecond)
	m.Stop()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("StartHealthChecks did not stop within timeout")
	}
}

func TestStartHealthChecksRespondsToContextCancel(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	config := models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: 3600, // Long interval so only ctx cancel triggers exit
		HealthCheckURL:      "http://localhost:1",
	}
	m := NewManager(db, config)
	defer m.Stop()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.StartHealthChecks(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("StartHealthChecks did not respond to context cancellation")
	}
}

// --- Selection algorithm correctness ---

func TestSelectRoundRobinDirect(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)

	proxies := []models.Proxy{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}

	results := make([]string, 6)
	for i := range results {
		p := m.selectRoundRobin(proxies)
		results[i] = p.ID
	}

	expected := []string{"a", "b", "c", "a", "b", "c"}
	for i, want := range expected {
		if results[i] != want {
			t.Errorf("round-robin[%d]: got %s, want %s", i, results[i], want)
		}
	}
}

func TestSelectRandomDirect(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRandom)

	proxies := []models.Proxy{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}

	// Just verify no panic and returns valid proxy
	for i := 0; i < 50; i++ {
		p := m.selectRandom(proxies)
		if p.ID != "a" && p.ID != "b" && p.ID != "c" {
			t.Errorf("unexpected proxy ID: %s", p.ID)
		}
	}
}

func TestSelectLeastUsedDirect(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationLeastUsed)

	proxies := []models.Proxy{
		{ID: "a", TotalUsed: 10},
		{ID: "b", TotalUsed: 2},
		{ID: "c", TotalUsed: 5},
	}

	p := m.selectLeastUsed(proxies)
	if p.ID != "b" {
		t.Errorf("least-used: got %s (used=%d), want b (used=2)", p.ID, p.TotalUsed)
	}
}

func TestSelectLowestLatencyDirect(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationLowestLatency)

	proxies := []models.Proxy{
		{ID: "a", Latency: 200},
		{ID: "b", Latency: 50},
		{ID: "c", Latency: 150},
	}

	p := m.selectLowestLatency(proxies)
	if p.ID != "b" {
		t.Errorf("lowest-latency: got %s (latency=%d), want b (latency=50)", p.ID, p.Latency)
	}
}

func TestSelectLowestLatencyAllZero(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationLowestLatency)

	proxies := []models.Proxy{
		{ID: "a", Latency: 0},
		{ID: "b", Latency: 0},
		{ID: "c", Latency: 0},
	}

	// When all latencies are 0, should return first proxy (no measured latency)
	p := m.selectLowestLatency(proxies)
	if p.ID != "a" {
		t.Errorf("lowest-latency all-zero: got %s, want a (first)", p.ID)
	}
}

func TestCheckProxyHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"origin":"1.2.3.4"}`))
	}))
	defer srv.Close()

	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = srv.URL

	p := models.Proxy{
		ID:        "hc-healthy-1",
		Server:    srv.Listener.Addr().String(),
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusUnhealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	m.checkProxy(context.Background(), p)

	got := getProxy(t, db, "hc-healthy-1")
	if got.Status != models.ProxyStatusHealthy {
		t.Errorf("status after healthy check: got %q, want %q", got.Status, models.ProxyStatusHealthy)
	}
	if got.Latency < 0 {
		t.Errorf("latency should be >= 0 after health check, got %d", got.Latency)
	}
}

func TestCheckProxyUnhealthyBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = srv.URL

	p := models.Proxy{
		ID:        "hc-bad-status-1",
		Server:    srv.Listener.Addr().String(),
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	m.checkProxy(context.Background(), p)

	got := getProxy(t, db, "hc-bad-status-1")
	if got.Status != models.ProxyStatusUnhealthy {
		t.Errorf("status after bad status check: got %q, want %q", got.Status, models.ProxyStatusUnhealthy)
	}
}

func TestCheckProxyUnhealthyConnectionRefused(t *testing.T) {
	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = "http://127.0.0.1:1"

	p := models.Proxy{
		ID:        "hc-connrefused-1",
		Server:    "127.0.0.1:1",
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	m.checkProxy(context.Background(), p)

	got := getProxy(t, db, "hc-connrefused-1")
	if got.Status != models.ProxyStatusUnhealthy {
		t.Errorf("status after connection refused: got %q, want %q", got.Status, models.ProxyStatusUnhealthy)
	}
}

func TestCheckProxyWithAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"origin":"1.2.3.4"}`))
	}))
	defer srv.Close()

	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = srv.URL

	p := models.Proxy{
		ID:        "hc-auth-1",
		Server:    srv.Listener.Addr().String(),
		Protocol:  models.ProxyHTTP,
		Username:  "testuser",
		Password:  "testpass",
		Status:    models.ProxyStatusUnhealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	m.checkProxy(context.Background(), p)

	got := getProxy(t, db, "hc-auth-1")
	if got.Latency < 0 {
		t.Errorf("latency should be >= 0, got %d", got.Latency)
	}
}

func TestCheckProxyCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = srv.URL

	p := models.Proxy{
		ID:        "hc-ctx-cancel-1",
		Server:    srv.Listener.Addr().String(),
		Protocol:  models.ProxyHTTP,
		Status:    models.ProxyStatusHealthy,
		CreatedAt: time.Now(),
	}
	if err := db.CreateProxy(context.Background(), p); err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.checkProxy(ctx, p)

	got := getProxy(t, db, "hc-ctx-cancel-1")
	if got.Status != models.ProxyStatusUnhealthy {
		t.Errorf("status after cancelled context: got %q, want %q", got.Status, models.ProxyStatusUnhealthy)
	}
}

func TestCheckAllWithProxies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"origin":"1.2.3.4"}`))
	}))
	defer srv.Close()

	m, db := setupTestManager(t, models.RotationRoundRobin)
	m.config.HealthCheckURL = srv.URL

	for i := 0; i < 3; i++ {
		p := models.Proxy{
			ID:        fmt.Sprintf("checkall-%d", i),
			Server:    srv.Listener.Addr().String(),
			Protocol:  models.ProxyHTTP,
			Status:    models.ProxyStatusUnhealthy,
			CreatedAt: time.Now(),
		}
		if err := db.CreateProxy(context.Background(), p); err != nil {
			t.Fatalf("CreateProxy %d: %v", i, err)
		}
	}

	m.checkAll(context.Background())

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("checkall-%d", i)
		got := getProxy(t, db, id)
		if got.Status != models.ProxyStatusHealthy {
			t.Errorf("proxy %s status: got %q, want %q", id, got.Status, models.ProxyStatusHealthy)
		}
	}
}

func TestCheckAllEmptyPool(t *testing.T) {
	m, _ := setupTestManager(t, models.RotationRoundRobin)
	m.checkAll(context.Background())
}

func TestNewManagerCustomConfig(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	config := models.ProxyPoolConfig{
		HealthCheckURL:      "http://custom.example.com/health",
		HealthCheckInterval: 600,
		MaxFailures:         10,
		Strategy:            models.RotationLeastUsed,
	}
	m := NewManager(db, config)
	defer m.Stop()

	if m.config.HealthCheckURL != "http://custom.example.com/health" {
		t.Errorf("HealthCheckURL: got %q, want custom", m.config.HealthCheckURL)
	}
	if m.config.HealthCheckInterval != 600 {
		t.Errorf("HealthCheckInterval: got %d, want 600", m.config.HealthCheckInterval)
	}
	if m.config.MaxFailures != 10 {
		t.Errorf("MaxFailures: got %d, want 10", m.config.MaxFailures)
	}
}
