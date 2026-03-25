package browser

import (
	"context"
	"testing"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/chromedp"
)

func TestBrowserPoolConfigAndOptionsCopies(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])
	p := NewBrowserPool(PoolConfig{Size: 3, MaxTabs: 7, IdleTimeout: time.Minute, AcquireTimeout: 5 * time.Second}, opts)
	defer p.Stop()

	cfg := p.Config()
	if cfg.Size != 3 || cfg.MaxTabs != 7 || cfg.IdleTimeout != time.Minute || cfg.AcquireTimeout != 5*time.Second {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	gotOpts := p.Options()
	if len(gotOpts) != len(opts) {
		t.Fatalf("options len = %d, want %d", len(gotOpts), len(opts))
	}
}

func TestProxyPoolKey(t *testing.T) {
	key := proxyPoolKey(models.ProxyConfig{Protocol: models.ProxySOCKS5, Server: "127.0.0.1:9000"})
	if key != "socks5://127.0.0.1:9000" {
		t.Fatalf("key = %q", key)
	}
}

func TestProxyPoolMetricsAggregatesAllProxyPools(t *testing.T) {
	r, err := NewRunner(t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	defer r.StopProxyPools()
	base := NewBrowserPool(PoolConfig{Size: 2, MaxTabs: 2, IdleTimeout: time.Minute, AcquireTimeout: time.Second}, nil)
	defer base.Stop()
	r.SetPool(base)

	if _, err := r.getPoolForProxy(context.Background(), models.ProxyConfig{Protocol: models.ProxyHTTP, Server: "127.0.0.1:8001"}); err != nil {
		t.Fatalf("getPoolForProxy 1: %v", err)
	}
	if _, err := r.getPoolForProxy(context.Background(), models.ProxyConfig{Protocol: models.ProxyHTTP, Server: "127.0.0.1:8002"}); err != nil {
		t.Fatalf("getPoolForProxy 2: %v", err)
	}

	metrics := r.ProxyPoolMetrics()
	if metrics.Pools != 2 {
		t.Fatalf("Pools = %d, want 2", metrics.Pools)
	}
	if metrics.MaxBrowsers != 4 {
		t.Fatalf("MaxBrowsers = %d, want 4", metrics.MaxBrowsers)
	}
}
