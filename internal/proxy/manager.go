package proxy

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

// Manager handles proxy pool selection, rotation, and health checks.
type Manager struct {
	db     *database.DB
	config models.ProxyPoolConfig

	mu       sync.Mutex
	rrIndex  int // round-robin index
	stopOnce sync.Once
	stopCh   chan struct{}
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
		db:     db,
		config: config,
		stopCh: make(chan struct{}),
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
	proxies, err := m.db.ListHealthyProxies(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list healthy proxies: %w", err)
	}

	if geo != "" {
		filtered := make([]models.Proxy, 0, len(proxies))
		for _, p := range proxies {
			if p.Geo == geo {
				filtered = append(filtered, p)
			}
		}
		proxies = filtered
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no healthy proxies available (geo=%s)", geo)
	}

	var selected models.Proxy
	switch m.config.Strategy {
	case models.RotationRoundRobin:
		selected = m.selectRoundRobin(proxies)
	case models.RotationRandom:
		selected = m.selectRandom(proxies)
	case models.RotationLeastUsed:
		selected = m.selectLeastUsed(proxies)
	case models.RotationLowestLatency:
		selected = m.selectLowestLatency(proxies)
	default:
		selected = m.selectRoundRobin(proxies)
	}
	return &selected, nil
}

// RecordUsage records whether a proxy was used successfully.
func (m *Manager) RecordUsage(proxyID string, success bool) error {
	return m.db.IncrementProxyUsage(context.Background(), proxyID, success)
}

func (m *Manager) selectRoundRobin(proxies []models.Proxy) models.Proxy {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.rrIndex % len(proxies)
	m.rrIndex++
	return proxies[idx]
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
