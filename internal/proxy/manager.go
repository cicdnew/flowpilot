package proxy

import (
	"context"
	"errors"
	"fmt"
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
		stopCh:               make(chan struct{}),
	}
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

// SelectProxy picks a proxy based on the configured rotation strategy.
// If geo is specified, only proxies in that geo are considered.
func (m *Manager) SelectProxy(geo string) (*models.Proxy, error) {
	selected, _, direct, err := m.SelectProxyWithFallback(geo, models.ProxyFallbackStrict)
	if direct || err != nil {
		return nil, err
	}
	return selected, nil
}

func (m *Manager) SelectProxyWithFallback(geo string, fallback models.ProxyRoutingFallback) (*models.Proxy, bool, bool, error) {
	proxies, err := m.db.ListHealthyProxies(context.Background())
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
	return m.db.IncrementProxyUsage(context.Background(), proxyID, success)
}

func (m *Manager) ReserveProxy(geo string) (*Reservation, error) {
	lease, _, direct, err := m.ReserveProxyWithFallback(geo, models.ProxyFallbackStrict)
	if direct || err != nil {
		return nil, err
	}
	return lease, nil
}

func (m *Manager) ReserveProxyWithFallback(geo string, fallback models.ProxyRoutingFallback) (*Reservation, bool, bool, error) {
	proxies, err := m.db.ListHealthyProxies(context.Background())
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
	if fallbackUsed {
		m.recordFallback(geo)
	}

	selected := m.selectWithReservations(filtered, strings.TrimSpace(geo) != "")
	m.mu.Lock()
	m.activeReservations[selected.ID]++
	m.mu.Unlock()

	return &Reservation{manager: m, proxy: selected}, fallbackUsed, false, nil
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
	proxyURL, err := url.Parse(fmt.Sprintf("%s://%s", proxy.Protocol, proxy.Server))
	if err != nil {
		if dbErr := m.db.UpdateProxyHealth(context.Background(), proxy.ID, models.ProxyStatusUnhealthy, 0); dbErr != nil {
			log.Printf("update proxy %s health: %v", proxy.ID, dbErr)
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
		if dbErr := m.db.UpdateProxyHealth(context.Background(), proxy.ID, models.ProxyStatusUnhealthy, 0); dbErr != nil {
			log.Printf("update proxy %s health: %v", proxy.ID, dbErr)
		}
		return
	}

	resp, err := client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		if dbErr := m.db.UpdateProxyHealth(context.Background(), proxy.ID, models.ProxyStatusUnhealthy, latency); dbErr != nil {
			log.Printf("update proxy %s health: %v", proxy.ID, dbErr)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if dbErr := m.db.UpdateProxyHealth(context.Background(), proxy.ID, models.ProxyStatusUnhealthy, latency); dbErr != nil {
			log.Printf("update proxy %s health: %v", proxy.ID, dbErr)
		}
		return
	}

	if dbErr := m.db.UpdateProxyHealth(context.Background(), proxy.ID, models.ProxyStatusHealthy, latency); dbErr != nil {
		log.Printf("update proxy %s health: %v", proxy.ID, dbErr)
	}
}
