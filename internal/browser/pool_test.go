package browser

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestNewBrowserPoolDefaults(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{}, opts)
	defer p.Stop()

	if p.poolSize != DefaultPoolSize {
		t.Errorf("pool size: got %d, want %d", p.poolSize, DefaultPoolSize)
	}
	if p.maxTabs != 10 {
		t.Errorf("max tabs: got %d, want 10", p.maxTabs)
	}
	if p.idleTimeout != PoolIdleTimeout {
		t.Errorf("idle timeout: got %v, want %v", p.idleTimeout, PoolIdleTimeout)
	}
}

func TestNewBrowserPoolCustom(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{
		Size:        3,
		MaxTabs:     5,
		IdleTimeout: 1 * time.Minute,
	}, opts)
	defer p.Stop()

	if p.poolSize != 3 {
		t.Errorf("pool size: got %d, want 3", p.poolSize)
	}
	if p.maxTabs != 5 {
		t.Errorf("max tabs: got %d, want 5", p.maxTabs)
	}
	if p.idleTimeout != 1*time.Minute {
		t.Errorf("idle timeout: got %v, want 1m", p.idleTimeout)
	}
}

func TestNewBrowserPoolCapsMax(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{Size: 999}, opts)
	defer p.Stop()

	if p.poolSize != MaxPoolSize {
		t.Errorf("pool size should be capped at %d, got %d", MaxPoolSize, p.poolSize)
	}
}

func TestBrowserPoolStopIdempotent(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{Size: 1}, opts)

	p.Stop()
	p.Stop()
}

func TestBrowserPoolStatsEmpty(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{Size: 3}, opts)
	defer p.Stop()

	stats := p.Stats()
	if stats.TotalBrowsers != 0 {
		t.Errorf("total browsers: got %d, want 0", stats.TotalBrowsers)
	}
	if stats.MaxBrowsers != 3 {
		t.Errorf("max browsers: got %d, want 3", stats.MaxBrowsers)
	}
	if stats.ActiveTabs != 0 {
		t.Errorf("active tabs: got %d, want 0", stats.ActiveTabs)
	}
}

func TestBrowserPoolAcquireAfterStop(t *testing.T) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	p := NewBrowserPool(PoolConfig{Size: 1}, opts)
	p.Stop()

	_, _, err := p.Acquire(nil)
	if err == nil {
		t.Fatal("expected error acquiring from stopped pool")
	}
}

func TestAcquireReusableBrowserLockedChoosesLeastUsed(t *testing.T) {
	p := &BrowserPool{}
	now := time.Now()
	busy := &pooledBrowser{allocCtx: context.Background(), inUse: 3, maxTabs: 5, lastUsed: now}
	best := &pooledBrowser{allocCtx: context.Background(), inUse: 1, maxTabs: 5, lastUsed: now.Add(-time.Minute)}
	alsoBestButNewer := &pooledBrowser{allocCtx: context.Background(), inUse: 1, maxTabs: 5, lastUsed: now}
	full := &pooledBrowser{allocCtx: context.Background(), inUse: 5, maxTabs: 5, lastUsed: now}
	p.browsers = []*pooledBrowser{busy, alsoBestButNewer, full, best}

	chosen := p.acquireReusableBrowserLocked()
	if chosen != best {
		t.Fatal("expected least-used oldest browser to be chosen")
	}
	if chosen.inUse != 2 {
		t.Fatalf("inUse after acquire: got %d, want 2", chosen.inUse)
	}
}

func TestSignalAvailabilityLockedDoesNotBlock(t *testing.T) {
	p := &BrowserPool{notifyCh: make(chan struct{}, 1)}
	p.signalAvailabilityLocked()
	p.signalAvailabilityLocked()
	select {
	case <-p.notifyCh:
	default:
		t.Fatal("expected notify signal")
	}
}
