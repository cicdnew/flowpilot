# Implementation Plan: Scalability — 1000+ Concurrent Tasks

## Overview

FlowPilot currently supports ~200 concurrent tasks (the `QueueConcurrency` default). Reaching 1000+ concurrent tasks requires fixing five compounding bottlenecks: O(n) heap cancellation, unbounded in-memory task loading, SQLite write saturation, Chrome pool exhaustion, and a frontend that renders every row without virtualization. This plan addresses each layer in priority order, with measurable benchmarks at each phase.

## Requirements

- Support ≥ 1000 simultaneously running tasks without queue saturation or OOM
- p95 task submission latency < 100ms at 1000 tasks
- Cancel any task in < 10ms regardless of queue depth
- Frontend renders 10,000 task rows without jank (> 30 fps)
- SQLite write throughput ≥ 500 status updates/second
- Chrome pool utilization > 80% under sustained load
- Memory footprint < 2 GB at 1000 concurrent tasks

## Current Bottlenecks

### B1 — O(n) Heap Cancellation (`internal/queue/queue.go`)
```
func (q *Queue) Cancel(id string) error {
    // Iterates entire pq slice to find and remove task — O(n)
    for i, item := range q.pq { ... }
}
```
Fix: maintain a `heapIndex map[string]int` (task ID → heap position) updated on every swap in the heap's `Swap()` method.

### B2 — Unbounded `ListTasks()` (`internal/database/db_tasks.go:228`)
```go
// Loads ALL tasks into memory — 1000 tasks × ~4 KB each = 4 MB minimum
func (db *DB) ListTasks(ctx context.Context) ([]models.Task, error) {
    rows, _ := db.readConn.QueryContext(ctx, `SELECT ... FROM tasks ORDER BY priority DESC, created_at DESC`)
```
Fix: deprecate `ListTasks()` in favour of `ListTasksPaginated()` which already exists at line 464. Remove `ListTasks()` call from `app_tasks.go:ListTasks()`.

### B3 — Full-Table Scan in `GetTaskStats()` (`internal/database/db_tasks.go:442`)
```go
func (db *DB) GetTaskStats(ctx context.Context) (map[string]int, error) {
    rows, _ := db.readConn.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
```
The composite index `idx_tasks_status_priority` covers the `status` column but the GROUP BY still requires a full scan. Fix: add a dedicated `stats` materialized cache refreshed on each status transition.

### B4 — Single SQLite Write Connection (`internal/database/sqlite.go:28`)
```go
db.conn.SetMaxOpenConns(1)   // hard SQLite constraint
db.conn.SetMaxIdleConns(1)
```
At 1000 concurrent tasks each writing status updates, the 50ms persistence batch window fills faster than it drains. Fix: increase batch size dynamically, tune WAL checkpoint, and add overflow backpressure.

### B5 — Chrome Pool Limited to 100 Processes (`internal/browser/pool.go`)
```go
const MaxPoolSize = 200   // hard cap
```
Each Chrome process uses ~150–200 MB RAM. 100 processes = ~15–20 GB RAM. Fix: tab-based multiplexing (already at `MaxTabs=20`), lazy creation, resource-aware eviction.

### B6 — Frontend Renders All Rows (no virtual scrolling)
`frontend/src/components/TaskTable.svelte` iterates `$filteredTasks` with a `{#each}` block rendering every row into the DOM. At 1000+ rows this causes frame drops.

### B7 — Serial Stale Task Recovery (`app.go:startup`)
`RecoverStaleTasks()` resets each stale task one-by-one. Fix: `BatchUpdateTaskStatus()` already exists at `db_tasks.go:642` — use it.

---

## Architecture Changes

```
Before:
  Queue Worker Pool (200)
       │
       ▼
  Priority Heap (O(n) cancel)  ──►  SQLite (1 writer, 50ms batch)
       │
       ▼
  BrowserPool (100 processes)
       │
       ▼
  Frontend: {#each tasks} (all rows)

After:
  Queue Worker Pool (dynamic, 200–1000)
       │
       ▼
  Priority Heap + heapIndex map (O(log n) cancel)
       │
       ▼
  SQLite WAL (1 writer, dynamic batch, checkpoint tuned)
  + Archive table for completed tasks
       │
       ▼
  BrowserPool (lazy, tab-multiplexed, memory-aware eviction)
       │
       ▼
  Frontend: Virtual scroll (windowed {#each}, only ~50 rows in DOM)
```

---

## Implementation Steps

### Phase 1 — Quick Wins (1–2 days, Low complexity)

#### 1.1 Fix O(n) Heap Cancellation
**File:** `internal/queue/priority_heap.go` and `internal/queue/queue.go`

Add a position index to the heap. Implement `Swap` to keep it updated:

```go
// In priority_heap.go — add index tracking
type taskHeap struct {
    items []*taskItem
    index map[string]int // taskID → position in items
}

func (h taskHeap) Swap(i, j int) {
    h.items[i], h.items[j] = h.items[j], h.items[i]
    h.index[h.items[i].id] = i
    h.index[h.items[j].id] = j
}

func (h *taskHeap) Push(x any) {
    item := x.(*taskItem)
    h.index[item.id] = len(h.items)
    h.items = append(h.items, item)
}

func (h *taskHeap) Pop() any {
    old := h.items
    n := len(old)
    item := old[n-1]
    h.items = old[:n-1]
    delete(h.index, item.id)
    return item
}
```

In `queue.go`, replace the O(n) cancel loop with:
```go
func (q *Queue) removeFromPQ(id string) bool {
    if i, ok := q.pq.index[id]; ok {
        heap.Remove(&q.pq, i)
        return true
    }
    return false
}
```
**Impact:** Cancel goes from O(n) to O(log n). At 10,000 queued tasks: ~13 comparisons vs 10,000.

#### 1.2 Replace `ListTasks()` with Paginated Variant
**File:** `app_tasks.go` — `ListTasks()` method

```go
// BEFORE (loads all into memory)
func (a *App) ListTasks() ([]models.Task, error) {
    return a.db.ListTasks(ctx)
}

// AFTER (always paginated, page 1, size 100 default)
func (a *App) ListTasks() ([]models.Task, error) {
    result, err := a.db.ListTasksPaginated(ctx, 1, 100, "", "")
    return result.Tasks, err
}
```

Frontend callers already use `ListTasksPaginated` via `App.d.ts`. Remove the unpaginated fallback entirely.

#### 1.3 Stale Task Recovery via Batch Update
**File:** `app.go` — `startup()` method

```go
// BEFORE: serial one-by-one recovery
for _, task := range staleTasks {
    db.UpdateTaskStatus(ctx, task.ID, models.StatusFailed, "recovered")
}

// AFTER: single batch write
ids := make([]string, len(staleTasks))
for i, t := range staleTasks { ids[i] = t.ID }
db.BatchUpdateTaskStatus(ctx, ids, models.StatusFailed, "recovered on restart")
```
**File:** `internal/database/db_tasks.go` — `BatchUpdateTaskStatus()` already exists at line 642. Wire it in `app.go`.

#### 1.4 Tune SQLite PRAGMAs for Higher Throughput
**File:** `internal/database/sqlite.go` — `New()` function

```go
// Add to write connection PRAGMA block:
_, _ = conn.Exec(`PRAGMA wal_autocheckpoint=1000`) // checkpoint every 1000 pages
_, _ = conn.Exec(`PRAGMA page_size=8192`)           // larger pages for log-heavy workload
_, _ = conn.Exec(`PRAGMA journal_size_limit=67108864`) // 64MB WAL cap
```

Also increase the persistence batch size in `queue.go:New()`:
```go
// BEFORE
persistenceBatchSize: max(16, maxConcurrency),
// AFTER
persistenceBatchSize: max(64, maxConcurrency*2),
```

---

### Phase 2 — Queue Optimization (3–5 days, Medium complexity)

#### 2.1 Dynamic Worker Pool Scaling
**File:** `internal/queue/queue.go`

Add a `scaler` goroutine that adjusts the active worker count based on queue depth:

```go
type Queue struct {
    // ... existing fields ...
    minWorkers  int
    maxWorkers  int
    activeWorkers atomic.Int32
    scaleCh     chan int // send +N or -N to adjust
}

func (q *Queue) scaler() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            depth := len(q.pq.items)
            active := int(q.activeWorkers.Load())
            if depth > active*2 && active < q.maxWorkers {
                // scale up by 10%
                add := min(int(float64(active)*0.1)+1, q.maxWorkers-active)
                q.spawnWorkers(add)
            } else if depth == 0 && active > q.minWorkers {
                // scale down: workers self-terminate when idle > 30s
            }
        case <-q.stopCh:
            return
        }
    }
}
```

#### 2.2 Overflow Backpressure on Persistence Channel
**File:** `internal/queue/queue.go` — `persistenceWorker()` and callers

```go
// BEFORE: non-blocking send (silently drops on overflow)
select {
case q.persistenceCh <- taskStateWrite{change}:
default: // dropped
}

// AFTER: metered send with backpressure counter
func (q *Queue) enqueueStateWrite(w taskStateWrite) {
    select {
    case q.persistenceCh <- w:
    case <-time.After(200 * time.Millisecond):
        // Channel full — flush synchronously to avoid data loss
        q.flushPersistenceSync(w)
        q.metrics.PersistenceDrops.Add(1)
    case <-q.stopCh:
    }
}
```

#### 2.3 Task Priority Aging (Starvation Prevention)
**File:** `internal/queue/priority_heap.go`

Add an age multiplier to prevent low-priority tasks from waiting indefinitely:

```go
// effectivePriority incorporates wait time to prevent starvation
func (item *taskItem) effectivePriority() int {
    ageBonus := int(time.Since(item.enqueuedAt).Minutes()) / 5 // +1 priority per 5 min wait
    if ageBonus > 2 { ageBonus = 2 }                           // cap at +2
    return item.priority + ageBonus
}
```

Update `Less()` to call `effectivePriority()` instead of reading `item.priority` directly.

#### 2.4 Proxy Reservation Timeout
**File:** `internal/queue/queue.go` — worker loop

Currently, proxied slots are held until task completion. Add a lease timeout:

```go
// Release proxied slot if task takes > maxProxyHoldDuration
const maxProxyHoldDuration = 10 * time.Minute
go func() {
    select {
    case <-time.After(maxProxyHoldDuration):
        q.mu.Lock()
        if q.runningProxied > 0 { q.runningProxied-- }
        q.mu.Unlock()
    case <-taskDone:
    }
}()
```

---

### Phase 3 — Database Scaling (3–5 days, Medium complexity)

#### 3.1 Schema Versioning
**File:** `internal/database/sqlite.go` — `migrate()` function

Replace the ad-hoc idempotent creates with a versioned migration table:

```go
func (db *DB) migrate() error {
    // Create version table
    _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
        version     INTEGER PRIMARY KEY,
        applied_at  TEXT NOT NULL DEFAULT (datetime('now'))
    )`)
    if err != nil { return err }

    var currentVersion int
    db.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&currentVersion)

    migrations := []struct {
        version int
        sql     string
    }{
        {1, schemaV1},   // original schema
        {2, schemaV2},   // add logging_policy column
        {3, schemaV3},   // add task_archive table
    }

    for _, m := range migrations {
        if m.version <= currentVersion { continue }
        if _, err := db.conn.Exec(m.sql); err != nil {
            return fmt.Errorf("migration v%d: %w", m.version, err)
        }
        db.conn.Exec(`INSERT INTO schema_version(version) VALUES(?)`, m.version)
    }
    return nil
}
```

#### 3.2 Task Archival Table
**File:** `internal/database/sqlite.go` — new migration `schemaV3`

```sql
CREATE TABLE IF NOT EXISTS task_archive (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    status          TEXT NOT NULL,
    error           TEXT,
    result          TEXT,
    batch_id        TEXT,
    flow_id         TEXT,
    priority        INTEGER NOT NULL DEFAULT 1,
    tags            TEXT NOT NULL DEFAULT '[]',
    created_at      TEXT NOT NULL,
    started_at      TEXT,
    completed_at    TEXT,
    archived_at     TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_archive_status     ON task_archive(status);
CREATE INDEX IF NOT EXISTS idx_archive_batch_id   ON task_archive(batch_id);
CREATE INDEX IF NOT EXISTS idx_archive_created_at ON task_archive(created_at);
```

**File:** `internal/database/db_tasks.go` — new `ArchiveCompletedTasks()` method:

```go
// ArchiveCompletedTasks moves tasks older than cutoff from tasks → task_archive.
func (db *DB) ArchiveCompletedTasks(ctx context.Context, cutoff time.Time) (int64, error) {
    tx, err := db.BeginTx(ctx)
    if err != nil { return 0, err }
    defer tx.Rollback()

    res, err := tx.ExecContext(ctx, `
        INSERT INTO task_archive SELECT id, name, url, status, error, result,
            batch_id, flow_id, priority, tags, created_at, started_at, completed_at, datetime('now')
        FROM tasks
        WHERE status IN ('completed','failed','cancelled')
          AND completed_at < ?`, cutoff)
    if err != nil { return 0, err }

    n, _ := res.RowsAffected()
    if n > 0 {
        _, err = tx.ExecContext(ctx, `
            DELETE FROM tasks
            WHERE status IN ('completed','failed','cancelled')
              AND completed_at < ?`, cutoff)
        if err != nil { return 0, err }
    }
    return n, tx.Commit()
}
```

Wire this into the daily retention goroutine in `app.go` alongside `PurgeOldRecords`.

#### 3.3 Materialized Stats Cache
**File:** `internal/database/db_tasks.go` — replace `GetTaskStats()` implementation

```go
type statsCache struct {
    mu      sync.RWMutex
    counts  map[string]int
    updated time.Time
    ttl     time.Duration
}

func (db *DB) GetTaskStats(ctx context.Context) (map[string]int, error) {
    db.statsCache.mu.RLock()
    if time.Since(db.statsCache.updated) < db.statsCache.ttl {
        out := maps.Clone(db.statsCache.counts)
        db.statsCache.mu.RUnlock()
        return out, nil
    }
    db.statsCache.mu.RUnlock()

    // Refresh
    rows, err := db.readConn.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
    // ... fill counts ...
    db.statsCache.mu.Lock()
    db.statsCache.counts = counts
    db.statsCache.updated = time.Now()
    db.statsCache.mu.Unlock()
    return counts, nil
}
```
Set TTL to 2 seconds — acceptable staleness for the header metrics poll.

---

### Phase 4 — Chrome Pool Scaling (2–3 days, Medium complexity)

#### 4.1 Memory-Aware Eviction
**File:** `internal/browser/pool.go` — `cleanup()` goroutine

```go
func (p *BrowserPool) cleanup() {
    // Existing idle timeout eviction ...

    // NEW: evict idle browsers if system memory pressure detected
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    heapMB := memStats.HeapInuse / 1024 / 1024
    if heapMB > 1500 { // > 1.5 GB Go heap
        p.evictIdlest(5) // evict 5 least-recently-used idle browsers
    }
}
```

#### 4.2 Pre-warming Browser Contexts
**File:** `internal/browser/pool.go` — `NewBrowserPool()` 

```go
// After creating pool, pre-warm minPrewarm browser processes:
func (p *BrowserPool) Prewarm(count int) {
    count = min(count, p.poolSize/4) // prewarm up to 25% of pool
    for i := 0; i < count; i++ {
        go func() {
            ctx, cancel := chromedp.NewExecAllocator(context.Background(), p.opts...)
            p.mu.Lock()
            if len(p.browsers) < p.poolSize {
                p.browsers = append(p.browsers, &pooledBrowser{
                    allocCtx: ctx, allocCancel: cancel, lastUsed: time.Now(),
                })
            } else {
                cancel()
            }
            p.mu.Unlock()
        }()
    }
}
```
Call `pool.Prewarm(10)` in `app.go:startup()` after pool creation.

#### 4.3 Health Check on Acquire
**File:** `internal/browser/pool.go` — `Acquire()` method

Before returning a pooled browser context, verify it is still responsive:

```go
func (p *BrowserPool) isHealthy(b *pooledBrowser) bool {
    ctx, cancel := context.WithTimeout(b.allocCtx, 2*time.Second)
    defer cancel()
    tabCtx, tabCancel := chromedp.NewContext(ctx)
    defer tabCancel()
    err := chromedp.Run(tabCtx) // no-op run to check liveness
    return err == nil
}
```
If `!isHealthy(b)`, evict the browser and create a replacement asynchronously.

---

### Phase 5 — Frontend Virtual Scrolling (2–3 days, Medium complexity)

#### 5.1 Windowed Task List
**File:** `frontend/src/components/TaskTable.svelte`

Replace the `{#each $filteredTasks as task}` block with a windowed renderer:

```svelte
<script>
  // Virtual scroll state
  let scrollTop = 0;
  const ROW_HEIGHT = 48; // px
  const OVERSCAN = 5;

  $: visibleStart = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  $: visibleEnd   = Math.min($filteredTasks.length, visibleStart + Math.ceil(containerHeight / ROW_HEIGHT) + OVERSCAN * 2);
  $: visibleTasks = $filteredTasks.slice(visibleStart, visibleEnd);
  $: totalHeight  = $filteredTasks.length * ROW_HEIGHT;
  $: offsetY      = visibleStart * ROW_HEIGHT;
</script>

<div class="scroll-container" bind:clientHeight={containerHeight}
     on:scroll={e => scrollTop = e.target.scrollTop}
     style="overflow-y: auto; height: 600px;">
  <div style="height: {totalHeight}px; position: relative;">
    <div style="transform: translateY({offsetY}px)">
      {#each visibleTasks as task (task.id)}
        <TaskRow {task} />
      {/each}
    </div>
  </div>
</div>
```

This keeps only ~50 rows in the DOM regardless of total count.

#### 5.2 Derived Store Memoization
**File:** `frontend/src/lib/store.ts`

Wrap expensive derived stores with equality checks:

```typescript
import { derived, get } from 'svelte/store';

// BEFORE: recomputes on every store change
export const filteredTasks = derived([tasks, filterStatus, filterTag], ([$tasks, $status, $tag]) =>
    $tasks.filter(t => (!$status || t.status === $status) && (!$tag || t.tags.includes($tag)))
);

// AFTER: only re-emits if result actually changed
export const filteredTasks = derived([tasks, filterStatus, filterTag], ([$tasks, $status, $tag], set) => {
    const next = $tasks.filter(t =>
        (!$status || t.status === $status) &&
        (!$tag || t.tags?.includes($tag))
    );
    const current = get(filteredTasks);
    if (!current || current.length !== next.length || current[0]?.id !== next[0]?.id) {
        set(next);
    }
});
```

---

## Testing Strategy

### Unit Tests
- `internal/queue/queue_test.go`: Add `TestCancelLargeQueue` — submit 10,000 tasks, cancel 5,000, verify O(log n) timing
- `internal/database/sqlite_test.go`: Add `TestArchiveCompletedTasks` — verify archive + delete transaction atomicity
- `internal/browser/pool_test.go`: Add `TestPoolHealthCheck` — inject a dead browser context, verify eviction

### Load Tests
**New file:** `internal/queue/load_test.go`
```go
func BenchmarkQueueSubmit1000(b *testing.B) {
    q := setupTestQueue(b, 200)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        for j := 0; j < 1000; j++ {
            q.Submit(makeTask(j))
        }
    }
}

func BenchmarkCancelFromLargeQueue(b *testing.B) {
    q := setupTestQueue(b, 50)
    ids := submitN(q, 10000)
    b.ResetTimer()
    for _, id := range ids {
        q.Cancel(id)
    }
}
```

### Integration Tests
- End-to-end: submit 500 tasks with mock runner, verify all complete within 60s
- Memory: measure heap growth from 0 → 1000 tasks using `runtime.ReadMemStats`
- SQLite throughput: sustain 500 `UpdateTaskStatus` calls/second for 30s

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| heapIndex diverges from heap positions | Medium | High — wrong task cancelled | Invariant check in tests: walk heap, verify all indices match |
| WAL grows unbounded under high write load | Low | Medium — disk full | `wal_autocheckpoint=1000` + `journal_size_limit=67108864` |
| Browser pre-warming consumes RAM at startup | Medium | Low — slow start | Cap at 25% of pool size; use lazy prewarm goroutine |
| Archive migration locks table under load | Medium | High — downtime | Run archive in off-peak window; use `BEGIN IMMEDIATE` transaction |
| Virtual scroll breaks keyboard navigation | Low | Medium — UX regression | Preserve focus on `tabindex`; test with keyboard-only usage |
| Persistence channel overflow loses state writes | Low | High — data loss | `flushPersistenceSync` fallback + `PersistenceDrops` metric alert |

---

## Success Criteria

- [ ] `TestCancelLargeQueue` completes in < 5ms for 10,000-task queue (proves O(log n))
- [ ] `BenchmarkQueueSubmit1000` shows < 100ms p95 submission latency
- [ ] `ListTasksPaginated` used everywhere; `ListTasks()` removed from API surface
- [ ] 1000 concurrent tasks running with Go heap < 2 GB
- [ ] SQLite sustains 500 writes/second for 60 seconds without error
- [ ] Frontend renders 10,000 rows with < 16ms frame time (60 fps)
- [ ] Task archive migrates 100,000 completed rows in < 5 seconds
- [ ] Schema version table present; all future migrations versioned

## Benchmark Targets

| Metric | Current | Target |
|--------|---------|--------|
| Max concurrent tasks | ~200 | 1,000+ |
| Cancel latency (10K queue) | O(n) ~5ms | O(log n) < 1ms |
| `ListTasks()` at 10K rows | ~40ms + 40MB | Paginated: < 5ms |
| SQLite writes/second | ~200 | 500+ |
| Frontend at 10K rows | Jank (> 100ms frames) | < 16ms frames |
| Startup stale recovery (100 tasks) | 100 × 5ms = 500ms | < 20ms (batch) |
| Memory at 1K tasks | ~800MB | < 2 GB |
