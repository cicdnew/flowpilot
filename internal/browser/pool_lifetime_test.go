package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"flowpilot/internal/models"
)

const lifetimeTestPage = "data:text/html,<title>pool-ok</title>"

func newLifetimeTestPool(t *testing.T) *BrowserPool {
	t.Helper()
	p := NewBrowserPool(PoolConfig{Size: 1, MaxTabs: 2, AcquireTimeout: 30 * time.Second}, chromeOpts())
	t.Cleanup(p.Stop)
	return p
}

func navigateAndReadTitle(t *testing.T, p *BrowserPool, ctx context.Context) {
	t.Helper()
	tabCtx, release, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()
	runCtx, cancel := context.WithTimeout(tabCtx, 20*time.Second)
	defer cancel()
	var title string
	if err := chromedp.Run(runCtx, chromedp.Navigate(lifetimeTestPage), chromedp.Title(&title)); err != nil {
		t.Fatalf("navigate in pooled tab: %v", err)
	}
	if title != "pool-ok" {
		t.Fatalf("title: got %q, want %q", title, "pool-ok")
	}
}

func onlyPooledBrowser(t *testing.T, p *BrowserPool) *pooledBrowser {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.browsers) != 1 {
		t.Fatalf("pooled browsers: got %d, want 1", len(p.browsers))
	}
	return p.browsers[0]
}

func TestBrowserPoolBrowserOutlivesFirstCallerContext(t *testing.T) {
	requireChrome(t)
	p := newLifetimeTestPool(t)

	ctxA, cancelA := context.WithCancel(context.Background())
	navigateAndReadTitle(t, p, ctxA)
	cancelA()
	time.Sleep(200 * time.Millisecond)

	ctxB, cancelB := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelB()
	navigateAndReadTitle(t, p, ctxB)
}

func TestBrowserPoolReplacesKilledBrowser(t *testing.T) {
	requireChrome(t)
	p := newLifetimeTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	navigateAndReadTitle(t, p, ctx)
	dead := onlyPooledBrowser(t, p)
	dead.allocCancel()
	select {
	case <-dead.browserCtx.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("killed browser context was not closed")
	}

	navigateAndReadTitle(t, p, ctx)
	if replacement := onlyPooledBrowser(t, p); replacement == dead {
		t.Fatal("pool handed out the killed browser again")
	}
}

func TestBrowserPoolAcquireHonoursCallerCancelDuringWarmup(t *testing.T) {
	hangingChrome := filepath.Join(t.TempDir(), "hanging-chrome")
	if err := os.WriteFile(hangingChrome, []byte("#!/bin/sh\nexec sleep 60\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	p := NewBrowserPool(PoolConfig{Size: 1, AcquireTimeout: time.Minute}, []chromedp.ExecAllocatorOption{chromedp.ExecPath(hangingChrome)})
	defer p.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	_, _, err := p.Acquire(ctx)
	if err == nil {
		t.Fatal("expected Acquire to fail after caller cancellation")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Acquire returned after %s; want prompt return after caller cancellation", elapsed)
	}
	p.mu.Lock()
	browsers, creating := len(p.browsers), p.creating
	p.mu.Unlock()
	if browsers != 0 || creating != 0 {
		t.Fatalf("half-started browser kept: browsers=%d creating=%d", browsers, creating)
	}
}

func TestAcquireReusableBrowserLockedDropsDeadBrowsers(t *testing.T) {
	deadCtx, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	killed := false
	dead := &pooledBrowser{allocCtx: deadCtx, allocCancel: func() { killed = true }, browserCtx: deadCtx, maxTabs: 5}
	alive := &pooledBrowser{allocCtx: context.Background(), allocCancel: func() {}, browserCtx: context.Background(), inUse: 1, maxTabs: 5}
	p := &BrowserPool{browsers: []*pooledBrowser{dead, alive}}

	if chosen := p.acquireReusableBrowserLocked(); chosen != alive {
		t.Fatal("expected the live browser to be chosen over the dead idle one")
	}
	if len(p.browsers) != 1 || p.browsers[0] != alive {
		t.Fatalf("dead browser not removed: %d browsers left", len(p.browsers))
	}
	if !killed {
		t.Fatal("dead browser allocator was not cancelled")
	}
}

func TestPooledTaskCancellationInterruptsRunningStep(t *testing.T) {
	requireChrome(t)
	p := newLifetimeTestPool(t)
	r, err := NewRunner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r.SetPool(p)

	ctx, cancel := context.WithCancel(context.Background())
	task := models.Task{ID: "cancel-me", Headless: true, Steps: []models.TaskStep{
		{Action: models.ActionNavigate, Value: lifetimeTestPage},
		{Action: models.ActionWait, Value: "60000"},
	}}
	done := make(chan error, 1)
	go func() {
		_, err := r.RunTask(ctx, task)
		done <- err
	}()
	time.Sleep(1500 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled from a cancelled task, got %v", err)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Fatalf("cancellation took %s", elapsed)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancelling the task context did not interrupt the running pooled step")
	}

	ctxB, cancelB := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelB()
	navigateAndReadTitle(t, p, ctxB)
}
