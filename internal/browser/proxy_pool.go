package browser

import (
	"context"

	"flowpilot/internal/models"

	"github.com/chromedp/chromedp"
)

func (p *BrowserPool) Config() PoolConfig {
	return PoolConfig{
		Size:           p.poolSize,
		MaxTabs:        p.maxTabs,
		IdleTimeout:    p.idleTimeout,
		AcquireTimeout: p.acquireTimeout,
	}
}

func (p *BrowserPool) Options() []chromedp.ExecAllocatorOption {
	opts := make([]chromedp.ExecAllocatorOption, len(p.opts))
	copy(opts, p.opts)
	return opts
}

func proxyPoolKey(proxyConfig models.ProxyConfig) string {
	return string(proxyConfig.Protocol) + "://" + proxyConfig.Server
}

func (r *Runner) getPoolForProxy(ctx context.Context, proxyConfig models.ProxyConfig) (*BrowserPool, error) {
	r.mu.Lock()
	basePool := r.pool
	if proxyConfig.Server == "" || basePool == nil {
		r.mu.Unlock()
		return basePool, nil
	}
	if r.proxyPools == nil {
		r.proxyPools = make(map[string]*BrowserPool)
	}
	key := proxyPoolKey(proxyConfig)
	if pool := r.proxyPools[key]; pool != nil {
		r.mu.Unlock()
		return pool, nil
	}
	cfg := basePool.Config()
	opts := basePool.Options()
	proxyAddr := proxyConfig.Server
	if proxyConfig.Protocol != "" && proxyConfig.Protocol != models.ProxyHTTP {
		proxyAddr = string(proxyConfig.Protocol) + "://" + proxyConfig.Server
	}
	opts = append(opts, chromedp.ProxyServer(proxyAddr))
	pool := NewBrowserPool(cfg, opts)
	r.proxyPools[key] = pool
	r.mu.Unlock()
	return pool, nil
}

func (r *Runner) stopProxyPools() {
	r.mu.Lock()
	pools := make([]*BrowserPool, 0, len(r.proxyPools))
	for _, pool := range r.proxyPools {
		pools = append(pools, pool)
	}
	r.proxyPools = make(map[string]*BrowserPool)
	r.mu.Unlock()
	for _, pool := range pools {
		pool.Stop()
	}
}

type ProxyPoolMetrics struct {
	Pools         int `json:"pools"`
	TotalBrowsers int `json:"totalBrowsers"`
	MaxBrowsers   int `json:"maxBrowsers"`
	ActiveTabs    int `json:"activeTabs"`
	IdleBrowsers  int `json:"idleBrowsers"`
}

func (r *Runner) ProxyPoolMetrics() ProxyPoolMetrics {
	r.mu.Lock()
	pools := make([]*BrowserPool, 0, len(r.proxyPools))
	for _, pool := range r.proxyPools {
		pools = append(pools, pool)
	}
	r.mu.Unlock()

	metrics := ProxyPoolMetrics{Pools: len(pools)}
	for _, pool := range pools {
		stats := pool.Stats()
		metrics.TotalBrowsers += stats.TotalBrowsers
		metrics.MaxBrowsers += stats.MaxBrowsers
		metrics.ActiveTabs += stats.ActiveTabs
		metrics.IdleBrowsers += stats.IdleBrowsers
	}
	return metrics
}

func (r *Runner) StopProxyPools() {
	r.stopProxyPools()
}
