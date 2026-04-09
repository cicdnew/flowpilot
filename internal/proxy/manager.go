package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

// Manager handles proxy pool selection, rotation, and health checks.
var ErrNoHealthyProxies = errors.New("no healthy proxies available")
var ErrProxyRateLimited = errors.New("all matching proxies are currently rate limited")

type Reservation struct {
	manager *Manager
	proxy   models.Proxy
	once    sync.Once
}

func (r *Reservation) Proxy() models.Proxy {
	return r.proxy
}

func (r *Reservation) ProxyID() string {
	return r.proxy.ID
}

func (r *Reservation) ProxyConfig() models.ProxyConfig {
	return r.proxy.ToProxyConfig()
}

func (r *Reservation) Complete(success bool) error {
	var err error
	r.once.Do(func() {
		err = r.manager.completeReservation(r.proxy.ID, success)
	})
	return err
}

func (r *Reservation) Release() error {
	return r.Complete(false)
}

// Manager handles proxy pool selection, rotation, and health checks.
type Manager struct {
	db     *database.DB
	config models.ProxyPoolConfig

	mu                   sync.Mutex
	rrIndex              int // round-robin index
	activeReservations   map[string]int
	countryFallbackCount map[string]int
	requestTimes         map[string][]time.Time
	now                  func() time.Time
	stopOnce             sync.Once
	stopCh               chan struct{}
}

// NewManager creates a new proxy manager.
func NewManager(db *database.DB, config models.ProxyPoolConfig) *Manager {
	if config.HealthCheckURL == "" {
		config.HealthCheckURL = "https://httpbin.org/ip"
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 300 // 5 minutes
	}
	if config.MaxFailures == 0 {
		config.MaxFailures = 3
	}

	return &Manager{
		db:                   db,
		config:               config,
		activeReservations:   make(map[string]int),
		countryFallbackCount: make(map[string]int),
		requestTimes:         make(map[string][]time.Time),
		now:                  time.Now,
		stopCh:               make(chan struct{}),
	}
}

func (m *Manager) dbWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	const dbWriteTimeout = 5 * time.Second
	if parent == nil || parent.Err() != nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, dbWriteTimeout)
}

func logUpdateProxyHealthError(proxyID string, err error) {
	const updateProxyHealthErrFmt = "update proxy %s health: %v"
	log.Printf(updateProxyHealthErrFmt, proxyID, err)
}

// StartHealthChecks begins periodic proxy health checks.
func (m *Manager) StartHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.HealthCheckInterval) * time.Second)
	defer ticker.Stop()

	// Run initial check.
	m.checkAll(ctx)

	for {
		select {
		case <-ticker.C:
			m.checkAll(ctx)
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop halts the health check loop. Safe to call multiple times.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// UpdateHealthCheckConfig updates the health check interval (in seconds) and URL
// for future health check cycles. Changes take effect on the next tick.
func (m *Manager) UpdateHealthCheckConfig(intervalSeconds int, url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if intervalSeconds > 0 {
		m.config.HealthCheckInterval = intervalSeconds
	}
	if url != "" {
		m.config.HealthCheckURL = url
	}
}

// SelectProxy picks a proxy based on the configured rotation strategy.
// If geo is specified, only proxies in that geo are considered.
func (m *Manager) SelectProxy(ctx context.Context, geo string) (*models.Proxy, error) {
	selected, _, direct, err := m.SelectProxyWithFallback(ctx, geo, models.ProxyFallbackStrict)
	if direct || err != nil {
		return nil, err
	}
	return selected, nil
}

func (m *Manager) SelectProxyWithFallback(ctx context.Context, geo string, fallback models.ProxyRoutingFallback) (*models.Proxy, bool, bool, error) {
	proxies, err := m.db.ListHealthyProxies(ctx)
	if err != nil {
		return nil, false, false, fmt.Errorf("list healthy proxies: %w", err)
	}

	filtered := filterProxiesByGeo(proxies, geo)
	fallbackUsed := false
	if len(filtered) == 0 {
		switch fallback {
		case models.ProxyFallbackAny:
			filtered = proxies
			fallbackUsed = strings.TrimSpace(geo) != ""
		case models.ProxyFallbackDirect:
			return nil, true, true, nil
		default:
			return nil, false, false, fmt.Errorf("no healthy proxies available (geo=%s)", geo)
		}
	}
	if len(filtered) == 0 {
		return nil, false, false, fmt.Errorf("no healthy proxies available (geo=%s)", geo)
	}
	if fallbackUsed {
		m.recordFallback(geo)
	}

	var selected models.Proxy
	if strings.TrimSpace(geo) != "" {
		selected = m.selectRandom(filtered)
	} else {
		switch m.config.Strategy {
		case models.RotationRoundRobin:
			selected = m.selectRoundRobin(filtered)
		case models.RotationRandom:
			selected = m.selectRandom(filtered)
		case models.RotationLeastUsed:
			selected = m.selectLeastUsed(filtered)
		case models.RotationLowestLatency:
			selected = m.selectLowestLatency(filtered)
		default:
			selected = m.selectRoundRobin(filtered)
		}
	}
	return &selected, fallbackUsed, false, nil
}

// RecordUsage records whether a proxy was used successfully.
func (m *Manager) RecordUsage(proxyID string, success bool) error {
	dbCtx, cancel := m.dbWriteContext(context.TODO())
	defer cancel()
	return m.db.IncrementProxyUsage(dbCtx, proxyID, success)
}

func (m *Manager) ReserveProxy(ctx context.Context, geo string) (*Reservation, error) {
	lease, _, direct, err := m.ReserveProxyWithFallback(ctx, geo, models.ProxyFallbackStrict)
	if direct || err != nil {
		return nil, err
	}
	return lease, nil
}

func (m *Manager) ReserveProxyWithFallback(ctx context.Context, geo string, fallback models.ProxyRoutingFallback) (*Reservation, bool, bool, error) {
	filtered, fallbackUsed, direct, err := m.availableProxies(ctx, geo, fallback)
	if err != nil || direct {
		return nil, fallbackUsed, direct, err
	}
	if fallbackUsed {
		m.recordFallback(geo)
	}

	selected := m.selectWithReservations(filtered, strings.TrimSpace(geo) != "")
	m.mu.Lock()
	m.activeReservations[selected.ID]++
	m.recordSelectionLocked(selected)
	m.mu.Unlock()

	return &Reservation{manager: m, proxy: selected}, fallbackUsed, false, nil
}

func (m *Manager) HasAvailableProxy(ctx context.Context, geo string, fallback models.ProxyRoutingFallback) (bool, time.Duration, error) {
	filtered, _, direct, err := m.availableProxies(ctx, geo, fallback)
	if direct {
		return true, 0, nil
	}
	if err != nil {
		if errors.Is(err, ErrProxyRateLimited) {
			_, wait := m.rateLimitStatus(filtered)
			return false, wait, nil
		}
		return false, 0, err
	}
	return true, 0, nil
}

func (m *Manager) availableProxies(ctx context.Context, geo string, fallback models.ProxyRoutingFallback) ([]models.Proxy, bool, bool, error) {
	proxies, err := m.db.ListHealthyProxies(ctx)
	if err != nil {
		return nil, false, false, fmt.Errorf("list healthy proxies: %w", err)
	}

	filtered := filterProxiesByGeo(proxies, geo)
	fallbackUsed := false
	if len(filtered) == 0 {
		switch fallback {
		case models.ProxyFallbackAny:
			filtered = proxies
			fallbackUsed = strings.TrimSpace(geo) != ""
		case models.ProxyFallbackDirect:
			return nil, true, true, nil
		default:
			return nil, false, false, fmt.Errorf("%w (geo=%s)", ErrNoHealthyProxies, geo)
		}
	}
	if len(filtered) == 0 {
		return nil, false, false, fmt.Errorf("%w (geo=%s)", ErrNoHealthyProxies, geo)
	}
	available, wait := m.rateLimitStatus(filtered)
	if len(available) == 0 {
		return filtered, fallbackUsed, false, fmt.Errorf("%w (geo=%s, retry_after=%s)", ErrProxyRateLimited, geo, wait)
	}
	return available, fallbackUsed, false, nil
}

func (m *Manager) rateLimitStatus(proxies []models.Proxy) ([]models.Proxy, time.Duration) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	available := make([]models.Proxy, 0, len(proxies))
	var minWait time.Duration
	for _, p := range proxies {
		remaining, wait := m.rateLimitStatusLocked(p, now)
		if remaining > 0 {
			available = append(available, p)
			continue
		}
		if minWait == 0 || (wait > 0 && wait < minWait) {
			minWait = wait
		}
	}
	return available, minWait
}

func (m *Manager) rateLimitStatusLocked(proxy models.Proxy, now time.Time) (int, time.Duration) {
	if proxy.MaxRequestsPerMinute <= 0 {
		return math.MaxInt, 0
	}
	windowStart := now.Add(-time.Minute)
	times := m.requestTimes[proxy.ID]
	kept := times[:0]
	for _, ts := range times {
		if ts.After(windowStart) {
			kept = append(kept, ts)
		}
	}
	m.requestTimes[proxy.ID] = kept
	remaining := proxy.MaxRequestsPerMinute - len(kept)
	if remaining > 0 {
		return remaining, 0
	}
	wait := time.Minute - now.Sub(kept[0])
	if wait < 0 {
		wait = 0
	}
	return 0, wait
}

func (m *Manager) recordSelectionLocked(proxy models.Proxy) {
	if proxy.MaxRequestsPerMinute <= 0 {
		return
	}
	m.requestTimes[proxy.ID] = append(m.requestTimes[proxy.ID], m.now())
}

func (m *Manager) completeReservation(proxyID string, success bool) error {
	m.mu.Lock()
	if m.activeReservations[proxyID] > 1 {
		m.activeReservations[proxyID]--
	} else {
		delete(m.activeReservations, proxyID)
	}
	m.mu.Unlock()
	return m.RecordUsage(proxyID, success)
}

func (m *Manager) ActiveReservations(proxyID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeReservations[proxyID]
}

func (m *Manager) selectRoundRobin(proxies []models.Proxy) models.Proxy {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.rrIndex % len(proxies)
	m.rrIndex++
	return proxies[idx]
}

func filterProxiesByGeo(proxies []models.Proxy, geo string) []models.Proxy {
	geo = strings.TrimSpace(geo)
	if geo == "" {
		return proxies
	}
	filtered := make([]models.Proxy, 0, len(proxies))
	for _, p := range proxies {
		if strings.EqualFold(strings.TrimSpace(p.Geo), geo) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func normalizeCountry(geo string) string {
	geo = strings.ToUpper(strings.TrimSpace(geo))
	if geo == "" {
		return "UNSPECIFIED"
	}
	return geo
}

func (m *Manager) recordFallback(geo string) {
	key := normalizeCountry(geo)
	m.mu.Lock()
	m.countryFallbackCount[key]++
	m.mu.Unlock()
}

func (m *Manager) CountryStats(proxies []models.Proxy, activeLocalEndpoints map[string]int) []models.ProxyCountryStats {
	m.mu.Lock()
	reservations := make(map[string]int, len(m.activeReservations))
	for id, count := range m.activeReservations {
		reservations[id] = count
	}
	fallbacks := make(map[string]int, len(m.countryFallbackCount))
	for country, count := range m.countryFallbackCount {
		fallbacks[country] = count
	}
	m.mu.Unlock()

	statsByCountry := make(map[string]*models.ProxyCountryStats)
	for _, p := range proxies {
		country := normalizeCountry(p.Geo)
		stat := statsByCountry[country]
		if stat == nil {
			stat = &models.ProxyCountryStats{Country: country}
			statsByCountry[country] = stat
		}
		stat.Total++
		if p.Status == models.ProxyStatusHealthy {
			stat.Healthy++
		}
		stat.TotalUsed += p.TotalUsed
		stat.ActiveReservations += reservations[p.ID]
		if activeLocalEndpoints != nil {
			stat.ActiveLocalEndpoints += activeLocalEndpoints[p.ID]
		}
	}
	for country, count := range fallbacks {
		stat := statsByCountry[country]
		if stat == nil {
			stat = &models.ProxyCountryStats{Country: country}
			statsByCountry[country] = stat
		}
		stat.FallbackAssignments = count
	}

	stats := make([]models.ProxyCountryStats, 0, len(statsByCountry))
	for _, stat := range statsByCountry {
		stats = append(stats, *stat)
	}
	return stats
}

func (m *Manager) selectWithReservations(proxies []models.Proxy, randomWithinPool bool) models.Proxy {
	m.mu.Lock()
	minReserved := math.MaxInt
	candidates := make([]models.Proxy, 0, len(proxies))
	for _, p := range proxies {
		reserved := m.activeReservations[p.ID]
		if reserved < minReserved {
			minReserved = reserved
			candidates = candidates[:0]
			candidates = append(candidates, p)
		} else if reserved == minReserved {
			candidates = append(candidates, p)
		}
	}
	m.mu.Unlock()

	if randomWithinPool {
		return m.selectRandom(candidates)
	}
	switch m.config.Strategy {
	case models.RotationRoundRobin:
		return m.selectRoundRobin(candidates)
	case models.RotationRandom:
		return m.selectRandom(candidates)
	case models.RotationLeastUsed:
		return m.selectLeastUsed(candidates)
	case models.RotationLowestLatency:
		return m.selectLowestLatency(candidates)
	default:
		return m.selectRoundRobin(candidates)
	}
}

func (m *Manager) selectRandom(proxies []models.Proxy) models.Proxy {
	idx := rand.Intn(len(proxies))
	return proxies[idx]
}

func (m *Manager) selectLeastUsed(proxies []models.Proxy) models.Proxy {
	best := proxies[0]
	for i := 1; i < len(proxies); i++ {
		if proxies[i].TotalUsed < best.TotalUsed {
			best = proxies[i]
		}
	}
	return best
}

func (m *Manager) selectLowestLatency(proxies []models.Proxy) models.Proxy {
	best := proxies[0]
	for i := 1; i < len(proxies); i++ {
		// Only consider proxies with measured latency (> 0).
		// Replace best if it has no latency data, or if this proxy is faster.
		if proxies[i].Latency > 0 && (best.Latency == 0 || proxies[i].Latency < best.Latency) {
			best = proxies[i]
		}
	}
	return best
}

func (m *Manager) checkAll(ctx context.Context) {
	proxies, err := m.db.ListProxies(ctx)
	if err != nil {
		log.Printf("health check: list proxies: %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, p := range proxies {
		wg.Add(1)
		go func(px models.Proxy) {
			defer wg.Done()
			m.checkProxy(ctx, px)
		}(p)
	}
	wg.Wait()
}

func (m *Manager) checkProxy(ctx context.Context, proxy models.Proxy) {
	if proxy.Protocol == "" {
		proxy.Protocol = "http"
	}
	proxyURL, err := url.Parse(fmt.Sprintf("%s://%s", proxy.Protocol, proxy.Server))
	if err != nil {
		dbCtx, cancel := m.dbWriteContext(ctx)
		dbErr := m.db.UpdateProxyHealth(dbCtx, proxy.ID, models.ProxyStatusUnhealthy, 0)
		cancel()
		if dbErr != nil {
			logUpdateProxyHealthError(proxy.ID, dbErr)
		}
		return
	}

	if proxy.Username != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	defer transport.CloseIdleConnections()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.config.HealthCheckURL, nil)
	if err != nil {
		dbCtx, cancel := m.dbWriteContext(ctx)
		dbErr := m.db.UpdateProxyHealth(dbCtx, proxy.ID, models.ProxyStatusUnhealthy, 0)
		cancel()
		if dbErr != nil {
			logUpdateProxyHealthError(proxy.ID, dbErr)
		}
		return
	}

	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		dbCtx, cancel := m.dbWriteContext(ctx)
		dbErr := m.db.UpdateProxyHealth(dbCtx, proxy.ID, models.ProxyStatusUnhealthy, latency)
		cancel()
		if dbErr != nil {
			logUpdateProxyHealthError(proxy.ID, dbErr)
		}
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // drain so connection can be reused
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		dbCtx, cancel := m.dbWriteContext(ctx)
		dbErr := m.db.UpdateProxyHealth(dbCtx, proxy.ID, models.ProxyStatusUnhealthy, latency)
		cancel()
		if dbErr != nil {
			logUpdateProxyHealthError(proxy.ID, dbErr)
		}
		return
	}

	dbCtx, cancel := m.dbWriteContext(ctx)
	dbErr := m.db.UpdateProxyHealth(dbCtx, proxy.ID, models.ProxyStatusHealthy, latency)
	cancel()
	if dbErr != nil {
		logUpdateProxyHealthError(proxy.ID, dbErr)
	}
}
