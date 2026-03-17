package localproxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"flowpilot/internal/models"
)

type endpointEntry struct {
	upstream      models.ProxyConfig
	listener      net.Listener
	addr          string
	localUsername string
	localPassword string
	active        int
	lastUsed      time.Time
	stopping      bool
}

type Manager struct {
	mu                sync.Mutex
	endpoints         map[string]*endpointEntry
	idleTimeout       time.Duration
	stopCh            chan struct{}
	wg                sync.WaitGroup
	endpointCreations int64
	endpointReuses    int64
	authFailures      int64
	upstreamFailures  int64
	lastError         string
}

func NewManager(idleTimeout time.Duration) *Manager {
	if idleTimeout <= 0 {
		idleTimeout = 5 * time.Minute
	}
	m := &Manager{
		endpoints:   make(map[string]*endpointEntry),
		idleTimeout: idleTimeout,
		stopCh:      make(chan struct{}),
	}
	m.wg.Add(1)
	go m.reaper()
	return m
}

func upstreamKey(cfg models.ProxyConfig) string {
	return strings.Join([]string{string(cfg.Protocol), cfg.Server, cfg.Username, cfg.Password}, "|")
}

func (m *Manager) Endpoint(cfg models.ProxyConfig) (models.ProxyConfig, error) {
	if strings.TrimSpace(cfg.Server) == "" {
		return cfg, nil
	}
	key := upstreamKey(cfg)

	m.mu.Lock()
	if entry, ok := m.endpoints[key]; ok && !entry.stopping {
		entry.lastUsed = time.Now()
		m.endpointReuses++
		local := models.ProxyConfig{Server: entry.addr, Protocol: models.ProxySOCKS5, Username: entry.localUsername, Password: entry.localPassword}
		m.mu.Unlock()
		return local, nil
	}
	m.mu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return models.ProxyConfig{}, fmt.Errorf("listen local proxy: %w", err)
	}
	localUser, err := randomCredential("fp")
	if err != nil {
		_ = ln.Close()
		return models.ProxyConfig{}, fmt.Errorf("generate local proxy username: %w", err)
	}
	localPass, err := randomCredential("tok")
	if err != nil {
		_ = ln.Close()
		return models.ProxyConfig{}, fmt.Errorf("generate local proxy password: %w", err)
	}
	entry := &endpointEntry{upstream: cfg, listener: ln, addr: ln.Addr().String(), localUsername: localUser, localPassword: localPass, lastUsed: time.Now()}

	m.mu.Lock()
	if existing, ok := m.endpoints[key]; ok && !existing.stopping {
		m.mu.Unlock()
		_ = ln.Close()
		return models.ProxyConfig{Server: existing.addr, Protocol: models.ProxySOCKS5, Username: existing.localUsername, Password: existing.localPassword}, nil
	}
	m.endpoints[key] = entry
	m.endpointCreations++
	m.wg.Add(1)
	m.mu.Unlock()

	go m.serve(key, entry)
	return models.ProxyConfig{Server: entry.addr, Protocol: models.ProxySOCKS5, Username: entry.localUsername, Password: entry.localPassword}, nil
}

func randomCredential(prefix string) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(buf), nil
}

func (m *Manager) serve(key string, entry *endpointEntry) {
	defer m.wg.Done()
	for {
		conn, err := entry.listener.Accept()
		if err != nil {
			m.mu.Lock()
			stopping := entry.stopping
			m.mu.Unlock()
			if stopping {
				return
			}
			return
		}
		m.mu.Lock()
		entry.active++
		entry.lastUsed = time.Now()
		m.mu.Unlock()
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			defer conn.Close()
			if err := handleSOCKS5Client(conn, entry.upstream, entry.localUsername, entry.localPassword); err != nil {
				if strings.Contains(err.Error(), "invalid socks5 credentials") || strings.Contains(err.Error(), "no acceptable socks5 auth method") {
					m.RecordAuthFailure(err)
				} else if strings.Contains(err.Error(), "upstream") || strings.Contains(err.Error(), "dial") || strings.Contains(err.Error(), "connect") {
					m.RecordUpstreamFailure(err)
				}
			}
			m.mu.Lock()
			if entry.active > 0 {
				entry.active--
			}
			entry.lastUsed = time.Now()
			m.mu.Unlock()
		}()
	}
}

func (m *Manager) reaper() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.pruneIdle()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) pruneIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for key, entry := range m.endpoints {
		if entry.stopping || entry.active > 0 {
			continue
		}
		if now.Sub(entry.lastUsed) < m.idleTimeout {
			continue
		}
		entry.stopping = true
		_ = entry.listener.Close()
		delete(m.endpoints, key)
	}
}

func (m *Manager) Stop() {
	close(m.stopCh)
	m.mu.Lock()
	for key, entry := range m.endpoints {
		entry.stopping = true
		_ = entry.listener.Close()
		delete(m.endpoints, key)
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) EndpointStatsByProxy(proxies []models.Proxy) map[string]int {
	stats := make(map[string]int, len(proxies))
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range proxies {
		if entry, ok := m.endpoints[upstreamKey(p.ToProxyConfig())]; ok && !entry.stopping {
			stats[p.ID] = entry.active
		}
	}
	return stats
}

func (m *Manager) RecordAuthFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authFailures++
	if err != nil {
		m.lastError = err.Error()
	}
}

func (m *Manager) RecordUpstreamFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upstreamFailures++
	if err != nil {
		m.lastError = err.Error()
	}
}

func (m *Manager) Stats() models.LocalProxyGatewayStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return models.LocalProxyGatewayStats{
		ActiveEndpoints:   len(m.endpoints),
		EndpointCreations: m.endpointCreations,
		EndpointReuses:    m.endpointReuses,
		AuthFailures:      m.authFailures,
		UpstreamFailures:  m.upstreamFailures,
		LastError:         m.lastError,
	}
}

func (m *Manager) EndpointAddr(cfg models.ProxyConfig) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.endpoints[upstreamKey(cfg)]; ok && !entry.stopping {
		return entry.addr
	}
	return ""
}
