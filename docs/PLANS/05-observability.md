# Implementation Plan: Observability — Logging, Metrics & Alerting

## Overview

FlowPilot has structured step/network logs and a basic audit trail but no unified observability stack. There is no metrics time-series, no alerting, no distributed trace correlation, and no single place to answer "why did this task fail at 2am?" This plan adds a layered observability system: structured log aggregation, an in-process Prometheus-compatible metrics registry, configurable alerting rules, and a dashboard panel — all while keeping the desktop-app constraint of zero mandatory external infrastructure.

## Requirements

- Structured JSON logs with consistent fields (task_id, batch_id, step_index, action, duration_ms, error)
- In-process metrics counters/histograms queryable from the frontend without an external server
- Alert rules evaluated in-process, firing via desktop notifications and/or webhook
- Log search/filter UI in the frontend (beyond the current export-only `LogViewer.svelte`)
- Execution timeline view per task (Gantt-style step durations)
- Exportable metrics snapshots (JSON/Prometheus text format)
- Zero required external dependencies (Prometheus server, Grafana, etc. optional)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      FlowPilot Process                          │
│                                                                 │
│  ┌──────────┐   structured   ┌─────────────┐                   │
│  │ Browser  │──── events ───►│  Log Router │                   │
│  │ Runner   │                │             │──► SQLite          │
│  └──────────┘                │  (fan-out)  │──► File (NDJSON)  │
│                              │             │──► Webhook (opt)  │
│  ┌──────────┐   metrics      └──────┬──────┘                   │
│  │  Queue   │──────────────────────►│                          │
│  └──────────┘                ┌──────▼──────┐                   │
│                              │  Metrics    │                   │
│  ┌──────────┐   events       │  Registry   │◄── Wails API      │
│  │Scheduler │──────────────► │  (in-proc)  │──► /metrics text  │
│  └──────────┘                └──────┬──────┘                   │
│                              ┌──────▼──────┐                   │
│                              │  Alert      │                   │
│                              │  Evaluator  │──► Desktop notif  │
│                              │  (30s tick) │──► Webhook POST   │
│                              └─────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Steps

### Phase 1 — Structured Logging Foundation (2–3 days, Low complexity)

#### 1.1 Unified Log Entry Model
**File:** `internal/models/logs.go` — add `LogEntry` struct

```go
// LogEntry is the canonical structured log record emitted by all subsystems.
type LogEntry struct {
    Timestamp   time.Time         `json:"ts"`
    Level       string            `json:"level"`   // "debug","info","warn","error"
    Source      string            `json:"source"`  // "queue","browser","scheduler","recorder"
    TaskID      string            `json:"task_id,omitempty"`
    BatchID     string            `json:"batch_id,omitempty"`
    ScheduleID  string            `json:"schedule_id,omitempty"`
    StepIndex   int               `json:"step_index,omitempty"`
    Action      string            `json:"action,omitempty"`
    DurationMs  int64             `json:"duration_ms,omitempty"`
    Message     string            `json:"msg"`
    Error       string            `json:"error,omitempty"`
    Fields      map[string]any    `json:"fields,omitempty"`
}
```

#### 1.2 Logger Interface
**New file:** `internal/observability/logger.go`

```go
package observability

import (
    "context"
    "io"
    "sync"
    "encoding/json"
    "time"

    "flowpilot/internal/models"
)

type Logger struct {
    mu       sync.Mutex
    handlers []LogHandler
}

type LogHandler interface {
    Handle(entry models.LogEntry) error
}

func (l *Logger) Emit(entry models.LogEntry) {
    if entry.Timestamp.IsZero() {
        entry.Timestamp = time.Now().UTC()
    }
    l.mu.Lock()
    handlers := l.handlers
    l.mu.Unlock()
    for _, h := range handlers {
        _ = h.Handle(entry)
    }
}

func (l *Logger) Info(source, msg string, fields map[string]any) {
    l.Emit(models.LogEntry{Level: "info", Source: source, Message: msg, Fields: fields})
}

func (l *Logger) Error(source, msg string, err error, fields map[string]any) {
    e := ""
    if err != nil { e = err.Error() }
    l.Emit(models.LogEntry{Level: "error", Source: source, Message: msg, Error: e, Fields: fields})
}

// NDJSONHandler writes newline-delimited JSON to any io.Writer.
type NDJSONHandler struct {
    mu  sync.Mutex
    enc *json.Encoder
}

func NewNDJSONHandler(w io.Writer) *NDJSONHandler {
    return &NDJSONHandler{enc: json.NewEncoder(w)}
}

func (h *NDJSONHandler) Handle(entry models.LogEntry) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    return h.enc.Encode(entry)
}
```

#### 1.3 Wire Logger into Browser Runner
**File:** `internal/browser/browser.go` — `Runner` struct

```go
type Runner struct {
    // ... existing fields ...
    logger *observability.Logger  // NEW
}

// In executeStep(), after each step completes:
r.logger.Emit(models.LogEntry{
    Level:      "info",
    Source:     "browser",
    TaskID:     taskID,
    StepIndex:  stepIdx,
    Action:     step.Action,
    DurationMs: elapsed.Milliseconds(),
    Message:    "step completed",
})
```

**File:** `app.go` — `startup()` — create logger and pass to runner, queue, scheduler.

#### 1.4 Rolling Log File
**New file:** `internal/observability/filehandler.go`

```go
// RollingFileHandler rotates log files when they exceed maxBytes.
type RollingFileHandler struct {
    mu       sync.Mutex
    path     string
    maxBytes int64
    current  *os.File
    written  int64
    enc      *json.Encoder
}

func (h *RollingFileHandler) Handle(entry models.LogEntry) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    if h.written > h.maxBytes {
        h.rotate()
    }
    n, err := /* encode entry */ 0, h.enc.Encode(entry)
    h.written += int64(n)
    return err
}

func (h *RollingFileHandler) rotate() {
    h.current.Close()
    archived := h.path + "." + time.Now().Format("20060102-150405")
    os.Rename(h.path, archived)
    h.current, _ = os.OpenFile(h.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
    h.enc = json.NewEncoder(h.current)
    h.written = 0
}
```

Default: rotate at 50 MB, keep last 5 files. Path: `~/.flowpilot/logs/flowpilot.ndjson`.

---

### Phase 2 — In-Process Metrics Registry (3–4 days, Medium complexity)

#### 2.1 Metrics Types
**New file:** `internal/observability/metrics.go`

```go
package observability

import (
    "sync"
    "sync/atomic"
    "math"
    "sort"
    "time"
)

// Registry holds all metrics. Thread-safe.
type Registry struct {
    mu         sync.RWMutex
    counters   map[string]*Counter
    gauges     map[string]*Gauge
    histograms map[string]*Histogram
}

// Counter is a monotonically increasing value.
type Counter struct{ val atomic.Int64 }
func (c *Counter) Inc()          { c.val.Add(1) }
func (c *Counter) Add(n int64)   { c.val.Add(n) }
func (c *Counter) Value() int64  { return c.val.Load() }

// Gauge is a value that can go up or down.
type Gauge struct {
    mu  sync.Mutex
    val float64
}
func (g *Gauge) Set(v float64)   { g.mu.Lock(); g.val = v; g.mu.Unlock() }
func (g *Gauge) Inc()            { g.mu.Lock(); g.val++; g.mu.Unlock() }
func (g *Gauge) Dec()            { g.mu.Lock(); g.val--; g.mu.Unlock() }
func (g *Gauge) Value() float64  { g.mu.Lock(); defer g.mu.Unlock(); return g.val }

// Histogram tracks a distribution of values (e.g., step durations).
type Histogram struct {
    mu      sync.Mutex
    buckets []float64 // upper bounds in ms
    counts  []int64
    sum     float64
    total   int64
}

func NewHistogram(buckets []float64) *Histogram {
    sort.Float64s(buckets)
    return &Histogram{buckets: buckets, counts: make([]int64, len(buckets)+1)}
}

func (h *Histogram) Observe(v float64) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.sum += v
    h.total++
    for i, b := range h.buckets {
        if v <= b { h.counts[i]++; return }
    }
    h.counts[len(h.buckets)]++ // overflow bucket
}

func (h *Histogram) Percentile(p float64) float64 {
    h.mu.Lock()
    defer h.mu.Unlock()
    target := int64(math.Ceil(p/100.0 * float64(h.total)))
    var cum int64
    for i, b := range h.buckets {
        cum += h.counts[i]
        if cum >= target { return b }
    }
    return math.Inf(1)
}
```

#### 2.2 Core Metrics Definitions
**New file:** `internal/observability/flowpilot_metrics.go`

```go
package observability

var (
    // Task lifecycle counters
    TasksSubmitted   = DefaultRegistry.Counter("tasks_submitted_total")
    TasksCompleted   = DefaultRegistry.Counter("tasks_completed_total")
    TasksFailed      = DefaultRegistry.Counter("tasks_failed_total")
    TasksCancelled   = DefaultRegistry.Counter("tasks_cancelled_total")
    TasksRetried     = DefaultRegistry.Counter("tasks_retried_total")

    // Queue state gauges
    QueueDepth       = DefaultRegistry.Gauge("queue_depth")
    RunningTasks     = DefaultRegistry.Gauge("running_tasks")
    ProxySlotsUsed   = DefaultRegistry.Gauge("proxy_slots_used")
    BrowserPoolUsed  = DefaultRegistry.Gauge("browser_pool_used")

    // Performance histograms (buckets in ms)
    StepDuration     = DefaultRegistry.Histogram("step_duration_ms",
        []float64{10, 50, 100, 500, 1000, 5000, 30000})
    TaskDuration     = DefaultRegistry.Histogram("task_duration_ms",
        []float64{100, 500, 1000, 5000, 30000, 60000, 300000})
    DBWriteLatency   = DefaultRegistry.Histogram("db_write_latency_ms",
        []float64{1, 5, 10, 50, 100, 500})

    // Error counters by type
    StepErrors       = DefaultRegistry.Counter("step_errors_total")
    ProxyErrors      = DefaultRegistry.Counter("proxy_errors_total")
    CaptchaErrors    = DefaultRegistry.Counter("captcha_errors_total")
    SchedulerFires   = DefaultRegistry.Counter("scheduler_fires_total")
)
```

#### 2.3 Wire Metrics into Queue
**File:** `internal/queue/queue.go` — instrument key paths:

```go
import "flowpilot/internal/observability"

// In Submit():
observability.TasksSubmitted.Inc()
observability.QueueDepth.Inc()

// In worker() on task completion:
observability.QueueDepth.Dec()
observability.RunningTasks.Dec()
switch finalStatus {
case models.StatusCompleted: observability.TasksCompleted.Inc()
case models.StatusFailed:    observability.TasksFailed.Inc()
case models.StatusCancelled: observability.TasksCancelled.Inc()
}
observability.TaskDuration.Observe(float64(elapsed.Milliseconds()))
```

#### 2.4 Metrics API Endpoint
**File:** `app.go` — new `GetMetricsSnapshot()` method:

```go
// GetMetricsSnapshot returns current metric values for the frontend dashboard.
func (a *App) GetMetricsSnapshot() models.MetricsSnapshot {
    return models.MetricsSnapshot{
        TasksSubmitted:  observability.TasksSubmitted.Value(),
        TasksCompleted:  observability.TasksCompleted.Value(),
        TasksFailed:     observability.TasksFailed.Value(),
        TasksCancelled:  observability.TasksCancelled.Value(),
        QueueDepth:      observability.QueueDepth.Value(),
        RunningTasks:    observability.RunningTasks.Value(),
        StepP50Ms:       observability.StepDuration.Percentile(50),
        StepP95Ms:       observability.StepDuration.Percentile(95),
        TaskP50Ms:       observability.TaskDuration.Percentile(50),
        TaskP95Ms:       observability.TaskDuration.Percentile(95),
        Timestamp:       time.Now().UTC(),
    }
}
```

Add `MetricsSnapshot` to `internal/models/event.go`.

#### 2.5 Prometheus Text Export
**File:** `app.go` — new `ExportMetricsPrometheus()` method:

```go
func (a *App) ExportMetricsPrometheus() (string, error) {
    return observability.DefaultRegistry.RenderPrometheusText(), nil
}
```

This allows `curl localhost:PORT/metrics` if a future HTTP server is added, or saves the snapshot to a file for scraping by an external Prometheus.

---

### Phase 3 — Alert Engine (3–4 days, Medium complexity)

#### 3.1 Alert Rule Model
**File:** `internal/models/` — new file `alert.go`

```go
package models

type AlertSeverity string
const (
    AlertSeverityInfo    AlertSeverity = "info"
    AlertSeverityWarning AlertSeverity = "warning"
    AlertSeverityCritical AlertSeverity = "critical"
)

type AlertRule struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Metric      string        `json:"metric"`       // e.g. "tasks_failed_total"
    Condition   string        `json:"condition"`    // "gt", "lt", "rate_gt"
    Threshold   float64       `json:"threshold"`
    Window      int           `json:"window_secs"`  // for rate conditions
    Severity    AlertSeverity `json:"severity"`
    Enabled     bool          `json:"enabled"`
    WebhookURL  string        `json:"webhook_url,omitempty"`
    CreatedAt   time.Time     `json:"created_at"`
}

type AlertFiring struct {
    ID         string        `json:"id"`
    RuleID     string        `json:"rule_id"`
    RuleName   string        `json:"rule_name"`
    Severity   AlertSeverity `json:"severity"`
    Value      float64       `json:"current_value"`
    Threshold  float64       `json:"threshold"`
    FiredAt    time.Time     `json:"fired_at"`
    ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
    Notified   bool          `json:"notified"`
}
```

#### 3.2 Alert Evaluator
**New file:** `internal/observability/alerter.go`

```go
package observability

import (
    "context"
    "time"
    "flowpilot/internal/models"
)

type AlertNotifier interface {
    Notify(alert models.AlertFiring) error
}

type Alerter struct {
    registry   *Registry
    db         AlertRuleStore
    notifiers  []AlertNotifier
    interval   time.Duration
    prevValues map[string]float64   // for rate calculations
    prevTime   map[string]time.Time
    firing     map[string]*models.AlertFiring // ruleID → active firing
}

func (a *Alerter) Run(ctx context.Context) {
    ticker := time.NewTicker(a.interval) // default 30s
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            a.evaluate(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (a *Alerter) evaluate(ctx context.Context) {
    rules, _ := a.db.ListAlertRules(ctx)
    for _, rule := range rules {
        if !rule.Enabled { continue }
        current := a.resolveMetric(rule.Metric)
        firing := a.checkCondition(rule, current)
        if firing && a.firing[rule.ID] == nil {
            alert := &models.AlertFiring{
                RuleID: rule.ID, RuleName: rule.Name,
                Severity: rule.Severity, Value: current,
                Threshold: rule.Threshold, FiredAt: time.Now(),
            }
            a.firing[rule.ID] = alert
            a.db.SaveAlertFiring(ctx, *alert)
            for _, n := range a.notifiers { n.Notify(*alert) }
        } else if !firing && a.firing[rule.ID] != nil {
            // Alert resolved
            now := time.Now()
            a.firing[rule.ID].ResolvedAt = &now
            a.db.UpdateAlertFiring(ctx, *a.firing[rule.ID])
            delete(a.firing, rule.ID)
        }
    }
}
```

#### 3.3 Notifiers
**New file:** `internal/observability/notifiers.go`

```go
// DesktopNotifier uses the Wails runtime notification API.
type DesktopNotifier struct {
    ctx context.Context // Wails context for runtime.EventsEmit
}

func (n *DesktopNotifier) Notify(alert models.AlertFiring) error {
    runtime.EventsEmit(n.ctx, "alert:fired", alert)
    return nil
}

// WebhookNotifier POSTs a JSON payload to a configured URL.
type WebhookNotifier struct {
    client *http.Client
}

func (n *WebhookNotifier) Notify(alert models.AlertFiring) error {
    body, _ := json.Marshal(alert)
    resp, err := n.client.Post(alert.WebhookURL, "application/json", bytes.NewReader(body))
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}
```

#### 3.4 Alert Database Schema
**File:** `internal/database/sqlite.go` — add to migration:

```sql
CREATE TABLE IF NOT EXISTS alert_rules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    metric      TEXT NOT NULL,
    condition   TEXT NOT NULL,
    threshold   REAL NOT NULL,
    window_secs INTEGER NOT NULL DEFAULT 0,
    severity    TEXT NOT NULL DEFAULT 'warning',
    enabled     INTEGER NOT NULL DEFAULT 1,
    webhook_url TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS alert_firings (
    id          TEXT PRIMARY KEY,
    rule_id     TEXT NOT NULL,
    rule_name   TEXT NOT NULL,
    severity    TEXT NOT NULL,
    value       REAL NOT NULL,
    threshold   REAL NOT NULL,
    fired_at    TEXT NOT NULL,
    resolved_at TEXT,
    notified    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_alert_firings_rule ON alert_firings(rule_id);
CREATE INDEX IF NOT EXISTS idx_alert_firings_fired ON alert_firings(fired_at);
```

#### 3.5 Alert App API
**New file:** `app_alerts.go`

```go
func (a *App) CreateAlertRule(name, metric, condition string, threshold float64,
    windowSecs int, severity, webhookURL string) (*models.AlertRule, error)

func (a *App) ListAlertRules() ([]models.AlertRule, error)

func (a *App) UpdateAlertRule(rule models.AlertRule) error

func (a *App) DeleteAlertRule(id string) error

func (a *App) ListAlertFirings(ruleID string, limit int) ([]models.AlertFiring, error)

func (a *App) GetActiveAlerts() ([]models.AlertFiring, error)
```

---

### Phase 4 — Log Search & Execution Timeline UI (3–4 days, Medium complexity)

#### 4.1 Log Search API
**File:** `internal/database/db_logs.go` — new `SearchLogs()` method

```go
type LogSearchQuery struct {
    TaskID     string
    BatchID    string
    Level      string
    Source     string
    Action     string
    TextSearch string
    FromTime   *time.Time
    ToTime     *time.Time
    Limit      int
    Offset     int
}

func (db *DB) SearchLogs(ctx context.Context, q LogSearchQuery) ([]models.StepLog, int, error) {
    // Build dynamic WHERE clause from non-zero fields
    // Uses idx_step_logs_task_id and idx_step_logs_task_step indexes
}
```

#### 4.2 Enhanced Log Viewer Component
**File:** `frontend/src/components/LogViewer.svelte` — replace export-only UI with inline viewer:

```
┌─────────────────────────────────────────────────────────────┐
│ Logs  [Search: ___________] [Level: ▼] [Source: ▼] [Clear] │
├─────────────────────────────────────────────────────────────┤
│ 04:12:01 [info ] browser  step 3: click #submit    12ms    │
│ 04:12:02 [warn ] browser  step 4: wait timed out   5000ms  │
│ 04:12:03 [error] browser  step 4: element not found        │
│ 04:12:05 [info ] queue    task retrying (attempt 2)        │
│ ...                                                         │
├─────────────────────────────────────────────────────────────┤
│ [Export JSON] [Export CSV] [Load More]                      │
└─────────────────────────────────────────────────────────────┘
```

Key features:
- Live tail mode: subscribe to `"log:entry"` Wails events, append to ring buffer (max 2000 entries)
- Retroactive search: calls `SearchLogs()` for historical records
- ANSI-style level coloring (green=info, yellow=warn, red=error)

#### 4.3 Execution Timeline Component
**New file:** `frontend/src/components/ExecutionTimeline.svelte`

Gantt-style visualization of step durations within a task:

```
Task: "Login and extract data"   Total: 4.2s
─────────────────────────────────────────────────────
Step 1  navigate  ████░░░░░░░░░░░░░░░░░░  890ms
Step 2  wait      ██░░░░░░░░░░░░░░░░░░░░  340ms
Step 3  click     █░░░░░░░░░░░░░░░░░░░░░  120ms
Step 4  type      ███░░░░░░░░░░░░░░░░░░░  580ms
Step 5  extract   █████████████░░░░░░░░░  2270ms ← slow
─────────────────────────────────────────────────────
```

Data source: `step_logs` table via new `GetTaskTimeline(taskID)` API call returning steps with `duration_ms`.

#### 4.4 Metrics Dashboard Panel
**New file:** `frontend/src/components/MetricsDashboard.svelte`

```
┌─── Live Metrics ──────────────────────────────────────────┐
│  Submitted  Completed  Failed   Queued   Running          │
│   12,450     11,890     312      45       200             │
├───────────────────────────────────────────────────────────┤
│  Step Duration (p50 / p95)    Task Duration (p50 / p95)  │
│       124ms / 2,340ms              4.2s / 38s            │
├───────────────────────────────────────────────────────────┤
│  [●] ACTIVE ALERTS                                        │
│  ⚠  tasks_failed_total > 50 — fired 2 min ago            │
│  ●  queue_depth > 500 — fired 8 min ago                  │
└───────────────────────────────────────────────────────────┘
```

Poll `GetMetricsSnapshot()` every 5 seconds (up from 2s header poll). Subscribe to `"alert:fired"` events for real-time alert badges.

---

### Phase 5 — Advanced Observability (Optional, 1–2 weeks, High complexity)

#### 5.1 Trace Correlation IDs
**File:** `internal/models/task.go` — add `TraceID string` to `Task`

Propagate a trace ID from task submission through each step log, network log, and alert firing. Enables correlating all events for a single task execution.

#### 5.2 OpenTelemetry Export (Optional)
**New file:** `internal/observability/otel.go`

Bridge the internal `Registry` to OTLP for users who run Grafana/Jaeger:

```go
// OTLPExporter periodically ships metrics to an OTLP endpoint.
type OTLPExporter struct {
    endpoint string      // e.g., "localhost:4317"
    registry *Registry
    interval time.Duration
}
```

Zero configuration by default; opt-in via `AppConfig.OTLPEndpoint`.

#### 5.3 Slow Step Detection
**File:** `internal/browser/browser.go` — `executeStep()` post-execution hook:

```go
const slowStepThreshold = 5 * time.Second
if elapsed > slowStepThreshold {
    logger.Emit(models.LogEntry{
        Level: "warn", Source: "browser",
        Message: "slow step detected",
        TaskID: taskID, StepIndex: stepIdx,
        Action: step.Action, DurationMs: elapsed.Milliseconds(),
        Fields: map[string]any{"threshold_ms": slowStepThreshold.Milliseconds()},
    })
    observability.StepErrors.Inc() // counted as degraded
}
```

---

## Testing Strategy

### Unit Tests
- **New file:** `internal/observability/metrics_test.go`
  - Counter: concurrent Inc() from 100 goroutines → verify exact total
  - Histogram: Observe 1000 values → verify p50/p95 within 1% of expected
  - Alert evaluator: mock registry values, verify firing/resolution state machine

- **New file:** `internal/observability/logger_test.go`
  - NDJSONHandler: emit 1000 entries → parse each line, verify JSON validity
  - RollingFileHandler: exceed maxBytes → verify new file created

### Integration Tests
- Alert fires within 2 evaluation ticks (≤ 60s) of threshold breach
- Log search returns correct results for all filter combinations
- Metrics survive 10,000 concurrent increments without data race (`-race` flag)

### Frontend Tests
- `LogViewer.svelte`: render 2000 log entries, verify ring buffer truncation
- `MetricsDashboard.svelte`: mock `GetMetricsSnapshot`, verify all fields rendered

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Logger fan-out blocks browser execution | Medium | High | Make `Handle()` non-blocking; drop on channel full with counter |
| Alert evaluator fires on transient spike | Medium | Low | Add `for` duration: alert must persist N evaluations before firing |
| Metrics registry memory grows unbounded | Low | Medium | Cap histogram bucket count; limit label cardinality |
| Rolling log file fills disk | Low | High | Cap total log directory size; delete oldest files first |
| NDJSON log file corrupted on crash | Low | Medium | Append-only writes; reader skips malformed lines |
| Webhook notifier blocks alert loop | Medium | Medium | POST in goroutine with 5s timeout |

---

## Success Criteria

- [ ] All step executions emit a structured `LogEntry` with task_id, step_index, action, duration_ms
- [ ] `GetMetricsSnapshot()` returns correct p50/p95 step durations within 1% accuracy
- [ ] Alert fires within 60 seconds of threshold breach
- [ ] Log search returns results in < 200ms for 1M log records (with indexes)
- [ ] Rolling log file rotates correctly at 50 MB
- [ ] Frontend `LogViewer` displays live tail with < 100ms event-to-display latency
- [ ] Execution timeline renders all step durations for a 100-step task
- [ ] Zero data races under `-race` flag across all observability code
- [ ] Metrics registry survives 10,000 concurrent updates without corruption
- [ ] `ExportMetricsPrometheus()` output is valid Prometheus text format (parseable by `promtool`)
