package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	DefaultPoolSize   = 5
	MaxPoolSize       = 200
	PoolIdleTimeout   = 5 * time.Minute
	PoolDialTimeout   = 30 * time.Second
	PoolCleanupPeriod = 30 * time.Second
)

type pooledBrowser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	lastUsed    time.Time
	inUse       int
	maxTabs     int
}

type BrowserPool struct {
	mu             sync.Mutex
	browsers       []*pooledBrowser
	poolSize       int
	maxTabs        int
	idleTimeout    time.Duration
	acquireTimeout time.Duration
	opts           []chromedp.ExecAllocatorOption
	stopped        bool
	creating       int
	stopCh         chan struct{}
	notifyCh       chan struct{}
	wg             sync.WaitGroup
}

type PoolConfig struct {
	Size           int
	MaxTabs        int
	IdleTimeout    time.Duration
	AcquireTimeout time.Duration
}

func NewBrowserPool(cfg PoolConfig, opts []chromedp.ExecAllocatorOption) *BrowserPool {
	if cfg.Size <= 0 {
		cfg.Size = DefaultPoolSize
	}
	if cfg.Size > MaxPoolSize {
		cfg.Size = MaxPoolSize
	}
	if cfg.MaxTabs <= 0 {
		cfg.MaxTabs = 10
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = PoolIdleTimeout
	}

	acquireTimeout := cfg.AcquireTimeout
	if acquireTimeout <= 0 {
		acquireTimeout = 60 * time.Second
	}

	p := &BrowserPool{
		browsers:       make([]*pooledBrowser, 0, cfg.Size),
		poolSize:       cfg.Size,
		maxTabs:        cfg.MaxTabs,
		idleTimeout:    cfg.IdleTimeout,
		acquireTimeout: acquireTimeout,
		opts:           opts,
		stopCh:         make(chan struct{}),
		notifyCh:       make(chan struct{}, 1),
	}

	p.wg.Add(1)
	go p.cleanupLoop()

	return p
}

func (p *BrowserPool) Acquire(ctx context.Context) (browserCtx context.Context, release func(), err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	deadline := time.Now().Add(p.acquireTimeout)
	for {
		p.mu.Lock()
		if p.stopped {
			p.mu.Unlock()
			return nil, nil, fmt.Errorf("browser pool is stopped")
		}

		if b := p.acquireReusableBrowserLocked(); b != nil {
			allocCtx := b.allocCtx
			p.mu.Unlock()
			return p.newTabContext(b, allocCtx)
		}

		canCreate := len(p.browsers)+p.creating < p.poolSize
		if canCreate {
			p.creating++
		}
		p.mu.Unlock()

		if canCreate {
			b, err := p.createBrowser(ctx)
			p.mu.Lock()
			p.creating--
			if err != nil {
				p.signalAvailabilityLocked()
				p.mu.Unlock()
				return nil, nil, fmt.Errorf("create pooled browser: %w", err)
			}
			if p.stopped {
				p.mu.Unlock()
				b.allocCancel()
				return nil, nil, fmt.Errorf("browser pool is stopped")
			}
			b.inUse++
			b.lastUsed = time.Now()
			p.browsers = append(p.browsers, b)
			allocCtx := b.allocCtx
			p.mu.Unlock()
			return p.newTabContext(b, allocCtx)
		}

		waitTimeout := time.Until(deadline)
		if waitTimeout <= 0 {
			return nil, nil, fmt.Errorf("browser pool acquire: timeout after %s", p.acquireTimeout)
		}
		if d, ok := ctx.Deadline(); ok {
			untilDeadline := time.Until(d)
			if untilDeadline <= 0 {
				return nil, nil, fmt.Errorf("browser pool acquire: context deadline exceeded")
			}
			if untilDeadline < waitTimeout {
				waitTimeout = untilDeadline
			}
		}

		timer := time.NewTimer(waitTimeout)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			if ctx.Err() == context.DeadlineExceeded {
				return nil, nil, fmt.Errorf("browser pool acquire: context deadline exceeded")
			}
			return nil, nil, fmt.Errorf("browser pool acquire: %w", ctx.Err())
		case <-p.stopCh:
			if !timer.Stop() {
				<-timer.C
			}
			return nil, nil, fmt.Errorf("browser pool is stopped")
		case <-p.notifyCh:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			return nil, nil, fmt.Errorf("browser pool acquire: timeout after %s", p.acquireTimeout)
		}
	}
}

func (p *BrowserPool) acquireReusableBrowserLocked() *pooledBrowser {
	var chosen *pooledBrowser
	for _, b := range p.browsers {
		if b.inUse >= b.maxTabs {
			continue
		}
		if chosen == nil || b.inUse < chosen.inUse || (b.inUse == chosen.inUse && b.lastUsed.Before(chosen.lastUsed)) {
			chosen = b
		}
	}
	if chosen != nil {
		chosen.inUse++
		chosen.lastUsed = time.Now()
	}
	return chosen
}

func (p *BrowserPool) signalAvailabilityLocked() {
	select {
	case p.notifyCh <- struct{}{}:
	default:
	}
}

func (p *BrowserPool) newTabContext(b *pooledBrowser, allocCtx context.Context) (context.Context, func(), error) {
	tabCtx, tabCancel := chromedp.NewContext(allocCtx,
		chromedp.WithNewBrowserContext())
	release := func() {
		tabCancel()
		p.mu.Lock()
		if b.inUse > 0 {
			b.inUse--
		}
		b.lastUsed = time.Now()
		p.signalAvailabilityLocked()
		p.mu.Unlock()
	}
	return tabCtx, release, nil
}

func (p *BrowserPool) createBrowser(ctx context.Context) (*pooledBrowser, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, p.opts...)

	browserCtx, _ := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(browserCtx); err != nil {
		allocCancel()
		return nil, fmt.Errorf("warm up pooled browser: %w", err)
	}

	return &pooledBrowser{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		lastUsed:    time.Now(),
		maxTabs:     p.maxTabs,
	}, nil
}

func (p *BrowserPool) cleanupLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(PoolCleanupPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.evictIdle()
		}
	}
}

func (p *BrowserPool) evictIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	active := make([]*pooledBrowser, 0, len(p.browsers))
	for _, b := range p.browsers {
		if b.inUse == 0 && now.Sub(b.lastUsed) > p.idleTimeout {
			// Use chromedp.Cancel for graceful browser shutdown, allowing
			// Chrome to save state and exit cleanly instead of force-killing.
			gracefulCtx, gracefulCancel := context.WithTimeout(context.Background(), 5*time.Second)
			chromedp.Cancel(gracefulCtx)
			gracefulCancel()
			b.allocCancel()
		} else {
			active = append(active, b)
		}
	}
	p.browsers = active
}

func (p *BrowserPool) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true

	for _, b := range p.browsers {
		b.allocCancel()
	}
	p.browsers = nil
	p.signalAvailabilityLocked()
	p.mu.Unlock()

	close(p.stopCh)
	p.wg.Wait()
}

func (p *BrowserPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{
		TotalBrowsers: len(p.browsers),
		MaxBrowsers:   p.poolSize,
	}
	for _, b := range p.browsers {
		stats.ActiveTabs += b.inUse
		if b.inUse == 0 {
			stats.IdleBrowsers++
		}
	}
	return stats
}

type PoolStats struct {
	TotalBrowsers int `json:"totalBrowsers"`
	MaxBrowsers   int `json:"maxBrowsers"`
	ActiveTabs    int `json:"activeTabs"`
	IdleBrowsers  int `json:"idleBrowsers"`
}
