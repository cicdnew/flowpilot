# TODO

## Bug 1: browser pool kills its own browsers (tasks wrongly "cancelled by user")

**Status:** open, not fixed
**Severity:** high. The shared pool (`app.go:288`) serves every task in production.
**Found by:** `TestRealWebPipelineThroughQueue` (`internal/queue/realweb_pipeline_test.go`, `realweb` build tag)

### Symptom

Four tasks submitted through the queue with concurrency 2: the first two complete, the next two
fail right away and are stored as `cancelled` with error `cancelled by user`. Nobody
cancelled them. The result was the same in 3 of 3 runs, and also on the code before
the cancellation-metrics changes (where these were counted as *failed* instead).

```
event pipe-internet-login -> completed
event pipe-quotes-scrape  -> completed
event pipe-books-product  -> cancelled cancelled by user
event pipe-httpbin-form   -> cancelled cancelled by user
queue metrics: submitted=4 completed=2 failed=0 cancelled=2
```

### Root cause

`BrowserPool.Acquire` (`internal/browser/pool.go:113`) starts a new pooled browser
with the **caller's** context:

```go
b, err := p.createBrowser(ctx)                                             // pool.go:113
allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, withAutomationFlags(p.opts)...) // pool.go:218
```

So the shared Chrome process lives only as long as the **first task** that used that
slot. When that task finishes, the queue cancels its context, Chrome exits, and the
`pooledBrowser` stays in `p.browsers` with `inUse` back at 0. The next `Acquire` hands
out the dead browser, the new tab fails right away with `context.Canceled`, and
`queue.handleTaskCancellation` stores it as a user cancellation.

Two factors make it worse:

- **Dead browsers are never removed while in use.** `evictIdle` (`pool.go:254`) only
  removes browsers idle longer than `idleTimeout`, and `acquireReusableBrowserLocked`
  (`pool.go:174`) prefers the least-used browser, so a busy pool keeps choosing the
  dead one. A Chrome crash or OOM kill leaves the slot broken in the same way.
- **The queue mislabels the error.** Any `context.Canceled` returned by the runner is
  treated as "cancelled by user" (`handleTaskCancellation`), even when the user did not
  cancel and the queue is not stopping. The task should fail (and be retried), not be
  marked cancelled.

Per-proxy pools (`Runner.getPoolForProxy`) use the same `createBrowser`, so proxied
tasks are affected too.

### Fix plan

- [ ] **Pool owns browser lifetime.** In `createBrowser`, make the allocator a child of a
      pool-owned root context (created in `NewBrowserPool`, cancelled in `Stop`) instead
      of the caller's `ctx`. Keep `Stop`/`evictIdle` calling `allocCancel` as today.
- [ ] **Startup still respects the caller.** While Chrome starts (`chromedp.Run(browserCtx)`),
      stop if the caller's `ctx` ends, e.g. `stop := context.AfterFunc(ctx, allocCancel)`
      around the warm-up, then `stop()` once it succeeds, so `Acquire` never waits past the
      caller's deadline.
- [ ] **Drop dead browsers.** In `acquireReusableBrowserLocked` (and `evictIdle`), skip and
      remove any `pooledBrowser` whose `allocCtx`/`browserCtx` is done, so a crashed or
      killed Chrome is replaced instead of being handed out again.
- [ ] **Queue: classify cancellations correctly.** A runner `context.Canceled` counts as a
      cancellation only if the task was cancelled (`q.cancelled[id]`) or the queue/parent
      context is stopping. Otherwise route it to `handleFailure`, so it is retried and
      counted as failed, not "cancelled by user".

### Tests to add

- [ ] `internal/browser` (real Chrome, `requireChrome`): acquire with context A, release,
      **cancel A**, acquire with context B, then run a navigation. It must succeed.
      This fails today.
- [ ] `internal/browser`: acquire, kill the pooled browser (`allocCancel`), acquire again.
      It must get a new working browser, not the dead one.
- [ ] `internal/browser`: caller context cancelled during warm-up. `Acquire` must return
      promptly with an error, and the pool must not keep the half-started browser.
- [ ] `internal/queue`: runner returns `context.Canceled` without a user cancel. The task
      must be retried or failed, never stored as `cancelled by user`.
- [ ] Re-run `TestRealWebPipelineThroughQueue` (`-tags=realweb`, headed and headless,
      3 times): expect 4/4 completed and metrics `completed=4 failed=0 cancelled=0`.

### Acceptance criteria

- [ ] Tasks after the first on a browser slot run normally. No spurious `cancelled by user`.
- [ ] A crashed or killed pooled Chrome is replaced on the next `Acquire`.
- [ ] `Acquire` still honours the caller's deadline while a browser starts.
- [ ] `go test -tags=dev -race ./...` passes. The new tests fail on the current code and pass
      after the fix.
- [ ] `TestRealWebPipelineThroughQueue` passes. Then drop it from the "known failures"
      in the PR description, and delete this TODO entry.
