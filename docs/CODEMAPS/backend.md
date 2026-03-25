# FlowPilot — Backend Codemap

> **Freshness:** 2026-03-24  
> **Cross-refs:** [INDEX.md](INDEX.md) | [browser.md](browser.md) | [database.md](database.md) | [workers.md](workers.md)

## Overview

The backend is a **Go Wails application**. All public methods on the `App` struct are automatically exposed as JavaScript-callable bindings via the Wails IPC bridge. The code is split across `app*.go` files, each covering a feature domain.

---

## Entry Points

### `main.go`
```
main()
  └── wails.Run(options)
        ├── OnStartup  → app.startup(ctx)
        ├── OnShutdown → app.shutdown(ctx)
        └── Bind       → &App{}
```

- Builds `wails.Options` with the `App` struct bound
- Registers `OnBeforeClose` for graceful shutdown (signals `stopCh`)
- Passes `assets/` embedded FS as the frontend asset provider

### `main_dev.go` (build tag: `dev`)
- Replaces embedded assets with `os.DirFS("frontend/dist")` for hot-reload during `wails dev`

### `cmd/agent/main.go`
- Standalone CLI: `agent -db ./data.db -concurrency 4 -poll 10s`
- Flags: `-db`, `-concurrency`, `-poll`, `-captcha-key`, `-captcha-provider`
- Creates `database.DB`, `queue.Queue`, then calls `agent.Run(ctx, db, queue, cfg)`

---

## App Struct (`app.go`)

```go
type App struct {
    ctx        context.Context    // Wails runtime context
    db         *database.DB
    queue      *queue.Queue
    scheduler  *scheduler.Scheduler
    recorder   *recorder.Recorder // guarded by recorderMu
    recorderMu sync.Mutex
    stopCh     chan struct{}
}
```

### Lifecycle Methods

| Method | Description |
|--------|-------------|
| `startup(ctx)` | Opens DB, initialises queue + scheduler, starts retention goroutine |
| `shutdown(ctx)` | Stops queue, closes DB, signals scheduler stop |
| `PurgeOldRecords()` (goroutine) | Runs daily; calls `db.PurgeOldData(90 days)` |

### Startup Sequence
```
app.startup(ctx)
  ├── database.Open(path)          → WAL-mode SQLite, runs migrations
  ├── queue.New(db, concurrency)   → starts worker pool goroutines
  │     └── emits TaskLifecycleEvents via Wails EventsEmit
  ├── scheduler.New(db, queue)     → starts cron tick goroutine
  └── goroutine: PurgeOldRecords() → 90-day retention loop
```

---

## API Surface (App methods → JS bindings)

### `app_tasks.go` — Task Management

| Method | Description |
|--------|-------------|
| `CreateTask(input TaskInput)` | Validates, persists, returns Task |
| `ListTasksPaginated(filter, page, size)` | Returns `PaginatedResult[Task]` |
| `GetTask(id)` | Single task lookup |
| `UpdateTask(id, input)` | Patch task fields |
| `DeleteTask(id)` | Hard delete |
| `StartTask(id)` | Enqueues task in queue, sets status=queued |
| `CancelTask(id)` | Signals running task context cancellation |
| `RetryTask(id)` | Re-enqueues failed task |
| `GetQueueMetrics()` | Returns `QueueMetrics` (pending/running/done counts) |

**Validation:** All inputs go through `validation.ValidateTaskInput()` before DB write.

### `app_flows.go` — Recorded Flows

| Method | Description |
|--------|-------------|
| `ListFlows()` | Returns all `RecordedFlow` metadata |
| `GetFlow(id)` | Returns full flow with steps |
| `DeleteFlow(id)` | Hard delete |
| `PlayRecordedFlow(flowID, taskInput)` | Creates task from flow steps, enqueues |
| `GetDOMSnapshot(flowID, stepIndex)` | Returns saved DOM HTML for a step |

### `app_recorder.go` — Recording Session

| Method | Description |
|--------|-------------|
| `StartRecording(url)` | Launches headed Chrome, injects capture script |
| `StopRecording(name)` | Stops Chrome, saves RecordedFlow to DB |
| `GetRecordingStatus()` | Returns `{recording: bool, stepCount: int}` |

**Thread safety:** `recorderMu` guards the `recorder` field — only one session at a time.

### `app_batch.go` — Batch Operations

| Method | Description |
|--------|-------------|
| `CreateBatch(input BatchInput)` | Parses CSV/URLs, creates BatchGroup + N tasks |
| `CreateBatchFromFlow(flowID, input)` | Like CreateBatch but uses recorded flow steps |
| `ListBatches()` | Returns `[]BatchGroup` with progress counts |
| `GetBatchProgress(batchID)` | Returns `BatchProgress` |
| `PauseBatch(batchID)` | Sets all queued tasks in batch to paused |
| `ResumeBatch(batchID)` | Re-enqueues paused tasks |
| `RetryFailedInBatch(batchID)` | Re-enqueues all failed tasks |
| `CancelBatch(batchID)` | Cancels all active tasks in batch |

### `app_schedules.go` — Schedules

| Method | Description |
|--------|-------------|
| `CreateSchedule(input)` | Persists schedule with cron expression |
| `ListSchedules()` | Returns `[]Schedule` |
| `UpdateSchedule(id, input)` | Patch fields |
| `DeleteSchedule(id)` | Hard delete |
| `EnableSchedule(id)` | Sets enabled=true, triggers scheduler reload |
| `DisableSchedule(id)` | Sets enabled=false |

### `app_proxy.go` — Proxy Management

| Method | Description |
|--------|-------------|
| `AddProxy(input)` | Validates, encrypts credentials, saves |
| `ListProxies()` | Returns list with masked credentials |
| `UpdateProxy(id, input)` | Patch fields |
| `DeleteProxy(id)` | Hard delete |
| `TestProxy(id)` | Health-check via proxy.Manager |
| `AddProxyRoutingPreset(input)` | Create named routing rules |
| `ListProxyRoutingPresets()` | Return all presets |
| `DeleteProxyRoutingPreset(id)` | Hard delete |

**Security:** `maskCredential(s)` replaces all but last 4 chars with `*` before returning to frontend.

### `app_captcha.go` — CAPTCHA

| Method | Description |
|--------|-------------|
| `SaveCaptchaConfig(input)` | Encrypts API key, persists config |
| `GetCaptchaConfig()` | Returns config with masked key |
| `TestCaptchaSolver()` | Submits a balance-check request |

### `app_compliance.go` — Audit & Retention

| Method | Description |
|--------|-------------|
| `ListAuditTrail(filter)` | Returns `[]TaskEvent` with pagination |
| `PurgeOldData(days int)` | Manual purge trigger (also runs automatically) |

### `app_export.go` — Exports

| Method | Description |
|--------|-------------|
| `ExportResults(filter)` | Returns tasks as JSON or CSV string |
| `ExportTaskLogs(taskID)` | Returns step logs + network logs as ZIP |
| `ExportBatchLogs(batchID)` | Returns all logs for a batch as ZIP |
| `ExportWebSocketLogs(taskID)` | Returns WS frame logs as JSON |

### `app_flow_io.go` — Flow Import/Export

| Method | Description |
|--------|-------------|
| `ExportFlow(id)` | Returns flow as JSON string |
| `ImportFlow(json)` | Parses JSON, saves as new flow, returns ID |

### `app_vision.go` — Visual Regression

| Method | Description |
|--------|-------------|
| `CaptureBaseline(taskID, stepIndex)` | Saves screenshot as baseline |
| `CompareBaseline(taskID, stepIndex)` | Diffs current vs baseline, stores VisualDiff |
| `ListVisualDiffs(taskID)` | Returns `[]VisualDiff` |
| `GetVisualBaseline(id)` | Returns baseline image bytes |

---

## Event Emission

The queue emits `TaskLifecycleEvent` via Wails runtime:

```go
// internal/queue/queue.go
wails.EventsEmit(ctx, "task:lifecycle", event)
wails.EventsEmit(ctx, "queue:metrics", metrics)
```

Frontend subscribes:
```typescript
// App.svelte
EventsOn("task:lifecycle", handler)
EventsOn("queue:metrics", handler)
```

---

## Error Handling Pattern

All API methods follow this pattern:
```go
func (a *App) DoSomething(input SomeInput) (Result, error) {
    if err := validation.ValidateX(input); err != nil {
        return Result{}, fmt.Errorf("validate: %w", err)
    }
    result, err := a.db.DoSomething(input)
    if err != nil {
        return Result{}, fmt.Errorf("db: %w", err)
    }
    return result, nil
}
```

Errors are wrapped with `fmt.Errorf("context: %w", err)` at every layer.  
`models.ClassifyError(err)` maps errors to standardized `ErrorCode` strings for the frontend.

---

## Background Agent (`internal/agent/`)

The `agent` package provides a headless version of the app for server deployment:

```go
// internal/agent/agent.go
type Agent struct {
    db          *database.DB
    queue       *queue.Queue
    pollInterval time.Duration
}

func Run(ctx, db, queue, cfg) error
  └── ticker loop:
        db.ListPendingTasks() → queue.Submit(task) for each
```

No Wails runtime — uses the same `queue` and `browser` packages as the GUI app.

---

## See Also

- [browser.md](browser.md) — step execution, chromedp integration
- [workers.md](workers.md) — queue internals, scheduler, batch
- [database.md](database.md) — DB schema, migrations
