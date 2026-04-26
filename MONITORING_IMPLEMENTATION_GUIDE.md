# Monitoring & Logging Implementation Guide

## Quick Reference: High-Priority Wins

| Priority | Improvement | Effort | Impact | Files Affected |
|----------|-------------|--------|--------|-----------------|
| 🔴 P0 | Queue depth metrics | 1h | High | queue.go, models.go |
| 🔴 P0 | Step duration extraction | 1h | High | logger.go, browser.go |
| 🔴 P0 | Error attribution | 2h | High | browser.go, errors.go |
| 🟡 P1 | Browser pool telemetry | 3h | Medium | pool.go, browser.go |
| 🟡 P1 | Task retry tracking | 2h | Medium | queue.go |
| 🟡 P1 | Network aggregation | 3h | Medium | logs/network.go |
| 🟢 P2 | Trace ID propagation | 4h | High | app.go, browser.go, queue.go |
| 🟢 P2 | Structured event logging | 5h | Medium | app.go, queue.go |

---

## Implementation Code Examples

### 1. Queue Depth Metrics (P0 - 1 hour)

**File: `internal/queue/queue.go`**

Add to Queue struct:
```go
type Queue struct {
    // ... existing fields ...
    metrics               models.QueueMetrics
    // ADD:
    metricsLock          sync.RWMutex
    pendingByPriority    map[models.TaskPriority]int  // O(1) lookup
    maxPendingObserved   int                          // High watermark
    avgDequeueTimeMs     float64                      // Exponential moving average
    lastDequeueTime      time.Time
}
```

Track in Submit():
```go
func (q *Queue) Submit(ctx context.Context, task models.Task) error {
    q.mu.Lock()
    if err := q.validateSubmitTask(task.ID); err != nil {
        q.mu.Unlock()
        return err
    }

    _, cancel := q.addTaskToHeap(task, ctx)
    
    // NEW: Update metrics
    q.metricsLock.Lock()
    q.pendingByPriority[task.Priority]++
    total := q.pq.Len() + q.pausedPQ.Len()
    if total > q.maxPendingObserved {
        q.maxPendingObserved = total
    }
    q.metricsLock.Unlock()
    
    q.mu.Unlock()

    // ... rest of Submit
}
```

Export metrics:
```go
func (q *Queue) GetQueueMetrics() models.QueueMetrics {
    q.metricsLock.RLock()
    defer q.metricsLock.RUnlock()
    
    return models.QueueMetrics{
        TotalSubmitted:       q.metrics.TotalSubmitted,
        TotalCompleted:       q.metrics.TotalCompleted,
        TotalFailed:          q.metrics.TotalFailed,
        PendingCount:         q.pq.Len(),
        PendingByPriority:    copyMap(q.pendingByPriority),
        MaxPendingObserved:   q.maxPendingObserved,
        WorkersActive:        len(q.running),
        WorkerPoolSize:       q.workerCount,
        ProxyConccurrentUsed: q.runningProxied,
    }
}
```

**File: `internal/models/models.go`**

Update struct:
```go
type QueueMetrics struct {
    TotalSubmitted       int                         `json:"total_submitted"`
    TotalCompleted       int                         `json:"total_completed"`
    TotalFailed          int                         `json:"total_failed"`
    TotalRetried         int                         `json:"total_retried"`
    // NEW:
    PendingCount         int                         `json:"pending_count"`
    PendingByPriority    map[TaskPriority]int        `json:"pending_by_priority"`
    MaxPendingObserved   int                         `json:"max_pending_observed"`
    WorkersActive        int                         `json:"workers_active"`
    WorkerPoolSize       int                         `json:"worker_pool_size"`
    ProxyConcurrentUsed  int                         `json:"proxy_concurrent_used"`
    AvgWaitTimeMs        float64                     `json:"avg_wait_time_ms"`
}
```

Expose in app.go:
```go
func (a *App) GetQueueMetrics() models.QueueMetrics {
    return a.queue.GetQueueMetrics()
}
```

---

### 2. Step Duration Extraction (P0 - 1 hour)

**File: `internal/logs/logger.go`**

Update StepLogger:
```go
type StepLogger struct {
    taskID      string
    stepLogs    []models.StepLog
    // NEW: Track step timing
    stepTimings map[models.StepAction]StepTimingStats
    mu          sync.Mutex
}

type StepTimingStats struct {
    Count    int64         // Number of executions
    TotalMs  int64         // Total time across all
    MinMs    int64         // Minimum time
    MaxMs    int64         // Maximum time
    LastMs   int64         // Most recent
}

func (l *StepLogger) EndStep(p EndStepParams) {
    log := models.StepLog{
        // ... existing fields ...
        DurationMs: time.Since(p.Start).Milliseconds(),
        // ... rest of log ...
    }
    l.stepLogs = append(l.stepLogs, log)
    
    // NEW: Aggregate timing stats
    l.mu.Lock()
    defer l.mu.Unlock()
    
    durationMs := log.DurationMs
    stats := l.stepTimings[p.Action]
    stats.Count++
    stats.TotalMs += durationMs
    if stats.MinMs == 0 || durationMs < stats.MinMs {
        stats.MinMs = durationMs
    }
    if durationMs > stats.MaxMs {
        stats.MaxMs = durationMs
    }
    stats.LastMs = durationMs
    l.stepTimings[p.Action] = stats
}

func (l *StepLogger) GetTimingStats() map[models.StepAction]StepTimingStats {
    l.mu.Lock()
    defer l.mu.Unlock()
    result := make(map[models.StepAction]StepTimingStats)
    for k, v := range l.stepTimings {
        result[k] = v
    }
    return result
}
```

In TaskResult:
```go
type TaskResult struct {
    // ... existing fields ...
    StepTimings map[models.StepAction]logger.StepTimingStats `json:"step_timings"`
}
```

In browser.go runSteps:
```go
if stepLogger != nil {
    defer func() {
        result.StepTimings = stepLogger.GetTimingStats()
    }()
}
```

---

### 3. Error Attribution (P0 - 2 hours)

**File: `internal/models/errors.go`**

Add context:
```go
type ErrorContext struct {
    TaskID       string
    StepIndex    int
    Action       StepAction
    Selector     string
    ProxyServer  string
    URL          string
    DurationMs   int64
    Timestamp    time.Time
}

type ClassifiedError struct {
    Code    ErrorCode
    Message string
    Context ErrorContext
}
```

Update classifier:
```go
func ClassifyError(err error, ctx ...ErrorContext) ErrorCode {
    // Existing logic ...
    if strings.Contains(err.Error(), "selector not found") {
        return ErrorCodeSelectorNotFound
    }
    if strings.Contains(err.Error(), "context deadline") {
        return ErrorCodeTimeout
    }
    // ... etc
}

func ClassifyErrorWithContext(err error, ctx ErrorContext) ClassifiedError {
    code := ClassifyError(err)
    return ClassifiedError{
        Code:    code,
        Message: err.Error(),
        Context: ctx,
    }
}
```

In browser.go:
```go
if err != nil {
    // Capture error context
    errCtx := models.ErrorContext{
        TaskID:     result.TaskID,
        StepIndex:  pc,
        Action:     step.Action,
        Selector:   step.Selector,
        ProxyServer: task.Proxy.Server,
        URL:        currentURL,  // Capture from browser
        DurationMs: time.Since(startedAt).Milliseconds(),
        Timestamp:  time.Now(),
    }
    classified := models.ClassifyErrorWithContext(err, errCtx)
    
    r.addLog(result, "error", fmt.Sprintf(
        "step %d failed [%s]: %v (selector: %q)",
        pc+1, classified.Code, err, step.Selector,
    ))
    
    logs.Logger.Error("step_error",
        "task_id", result.TaskID,
        "error_code", string(classified.Code),
        "step_index", pc,
        "action", string(step.Action),
        "proxy", task.Proxy.Server,
        "duration_ms", errCtx.DurationMs,
    )
    
    result.Error = fmt.Sprintf("step %d (%s) failed: %v", pc+1, step.Action, err)
    return err
}
```

---

### 4. Browser Pool Telemetry (P1 - 3 hours)

**File: `internal/browser/pool.go`**

Add stats collection:
```go
type PoolStats struct {
    TotalCreated       int64
    TotalClosed        int64
    CurrentBrowsers    int
    IdleBrowsers       int
    MaxTabsObserved    int
    AcquireTimeP95Ms   int64
    LastCleanupTime    time.Time
}

type BrowserPool struct {
    // ... existing fields ...
    stats              PoolStats
    statsLock          sync.RWMutex
    acquireTimes       []int64  // Rolling window of last 1000 acquisitions
    acquireTimesIdx    int
}
```

Track acquisition time:
```go
func (p *BrowserPool) Acquire(ctx context.Context) (browserCtx context.Context, release func(), err error) {
    acquireStart := time.Now()
    
    // ... existing acquire logic ...
    
    // Record timing
    acqTime := time.Since(acquireStart).Milliseconds()
    p.statsLock.Lock()
    p.acquireTimes[p.acquireTimesIdx] = acqTime
    p.acquireTimesIdx = (p.acquireTimesIdx + 1) % len(p.acquireTimes)
    p.statsLock.Unlock()
    
    return browserCtx, release, err
}
```

Export stats:
```go
func (p *BrowserPool) Stats() PoolStats {
    p.mu.Lock()
    idleCount := 0
    for _, b := range p.browsers {
        if b.inUse == 0 {
            idleCount++
        }
    }
    p.mu.Unlock()
    
    p.statsLock.RLock()
    defer p.statsLock.RUnlock()
    
    p95 := percentile(p.acquireTimes, 95)
    
    return PoolStats{
        TotalCreated:     p.stats.TotalCreated,
        TotalClosed:      p.stats.TotalClosed,
        CurrentBrowsers:  len(p.browsers),
        IdleBrowsers:     idleCount,
        AcquireTimeP95Ms: p95,
    }
}
```

---

### 5. Task Retry Tracking (P1 - 2 hours)

**File: `internal/queue/queue.go`**

Add to metrics:
```go
type RetryMetrics struct {
    TotalRetries         int
    RetryByReason        map[string]int        // "timeout", "network", "selector"
    RetrySuccessCount    int                   // Retries that succeeded
    RetryFailureCount    int                   // Retries that failed again
    AvgBackoffMs         float64
}
```

Update retryInfo:
```go
func (q *Queue) handleFailure(ctx context.Context, task models.Task, err error, result *models.TaskResult) retryInfo {
    // ... existing logic ...
    
    reason := "unknown"
    if strings.Contains(err.Error(), "timeout") {
        reason = "timeout"
    } else if strings.Contains(err.Error(), "network") {
        reason = "network"
    } else if strings.Contains(err.Error(), "selector") {
        reason = "selector"
    }
    
    if shouldRetry {
        // NEW: Log retry event
        logs.Logger.Info("task_scheduled_retry",
            "task_id", task.ID,
            "retry_count", task.RetryCount,
            "reason", reason,
            "backoff_ms", backoff.Milliseconds(),
            "proxy", task.Proxy.Server,
        )
    }
    
    return retryInfo{shouldRetry: shouldRetry, task: task, backoff: backoff}
}
```

Track success:
```go
func (q *Queue) handleSuccess(ctx context.Context, task models.Task, result *models.TaskResult) {
    // NEW: Track if this was a retry that succeeded
    if task.RetryCount > 0 {
        logs.Logger.Info("retry_succeeded",
            "task_id", task.ID,
            "retry_count", task.RetryCount,
            "duration_ms", result.Duration.Milliseconds(),
        )
    }
    
    // ... rest of handleSuccess ...
}
```

---

### 6. Network Log Aggregation (P1 - 3 hours)

**File: `internal/logs/network.go`**

Add aggregation:
```go
type NetworkAggregation struct {
    TotalRequests      int
    FailedRequests     int
    TotalResponseTimeMs int64
    LargestResponseSizeBytes int64
    AverageResponseTimeMs float64
    FailuresByStatusCode map[int]int
}

func (n *NetworkLogger) Aggregate() NetworkAggregation {
    n.mu.Lock()
    defer n.mu.Unlock()
    
    agg := NetworkAggregation{
        TotalRequests:        len(n.logs),
        FailuresByStatusCode: make(map[int]int),
    }
    
    for _, log := range n.logs {
        agg.TotalResponseTimeMs += log.DurationMs
        if log.ResponseSize > agg.LargestResponseSizeBytes {
            agg.LargestResponseSizeBytes = log.ResponseSize
        }
        if log.StatusCode >= 400 {
            agg.FailedRequests++
            agg.FailuresByStatusCode[log.StatusCode]++
        }
    }
    
    if agg.TotalRequests > 0 {
        agg.AverageResponseTimeMs = float64(agg.TotalResponseTimeMs) / float64(agg.TotalRequests)
    }
    
    return agg
}
```

In TaskResult:
```go
type TaskResult struct {
    // ... existing fields ...
    NetworkAgg *NetworkAggregation `json:"network_aggregation,omitempty"`
}
```

In browser.go:
```go
if netLogger != nil {
    result.NetworkLogs = netLogger.Logs()
    result.NetworkAgg = &netLogger.Aggregate()  // NEW
}
```

---

### 7. Trace ID Propagation (P2 - 4 hours)

**File: `internal/models/models.go`**

Add to Task:
```go
type Task struct {
    // ... existing fields ...
    TraceID string  // NEW: UUID for distributed tracing
}
```

**File: `app_tasks.go`**

In CreateTask:
```go
func (a *App) CreateTask(p CreateTaskParams) (*models.Task, error) {
    // ... validation ...
    
    task := models.Task{
        ID:        uuid.New().String(),
        TraceID:   uuid.New().String(),  // NEW
        Name:      p.Name,
        // ... rest
    }
    
    // ... store and return
}
```

**File: `internal/logs/logger.go`**

Update StepLogger:
```go
type StepLogger struct {
    taskID      string
    traceID     string  // NEW
    stepLogs    []models.StepLog
}

func NewStepLogger(taskID, traceID string) *StepLogger {
    return &StepLogger{
        taskID:   taskID,
        traceID:  traceID,
        stepLogs: []models.StepLog{},
    }
}
```

Update StepLog struct in models:
```go
type StepLog struct {
    // ... existing ...
    TraceID  string `json:"trace_id"`  // NEW
}
```

In browser.go runSteps:
```go
if policy.captureStepLogs {
    stepLogger = logs.NewStepLogger(result.TaskID, task.TraceID)  // NEW
}
```

Update all log calls:
```go
logs.Logger.Error("step_error",
    "trace_id", task.TraceID,  // NEW
    "task_id", result.TaskID,
    "step_index", pc,
    // ... rest
)
```

---

### 8. Structured Event Logging (P2 - 5 hours)

**File: `internal/models/models.go`**

Add event type:
```go
type TaskEvent struct {
    ID        string    `json:"id"`
    TaskID    string    `json:"task_id"`
    TraceID   string    `json:"trace_id"`  // NEW
    FromState TaskStatus `json:"from_state"`
    ToState   TaskStatus `json:"to_state"`
    EventData interface{} `json:"event_data"`
    CreatedAt time.Time  `json:"created_at"`
}

type TaskEventData struct {
    Reason    string                 `json:"reason,omitempty"`
    Error     string                 `json:"error,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
```

**File: `internal/queue/queue.go`**

Update emitEvent:
```go
func (q *Queue) emitEvent(taskID string, newStatus models.TaskStatus, reason string) {
    task, err := q.db.GetTask(context.Background(), taskID)
    if err != nil {
        return
    }
    
    eventData := models.TaskEventData{
        Reason: reason,
        Metadata: map[string]interface{}{
            "timestamp": time.Now().Unix(),
            "priority":  task.Priority,
        },
    }
    
    event := models.TaskEvent{
        ID:        uuid.New().String(),
        TaskID:    taskID,
        TraceID:   task.TraceID,  // NEW
        FromState: task.Status,
        ToState:   newStatus,
        EventData: eventData,
        CreatedAt: time.Now(),
    }
    
    // NEW: Log structured event
    logs.Logger.Info("task_event",
        "trace_id", event.TraceID,
        "task_id", event.TaskID,
        "from_state", string(event.FromState),
        "to_state", string(event.ToState),
        "reason", reason,
    )
    
    if q.onEvent != nil {
        q.onEvent(event)
    }
}
```

---

## Implementation Checklist

### Week 1: Foundation
- [ ] Add QueueMetrics fields (1h)
- [ ] Export queue depth metrics via API (1h)
- [ ] Add StepLogger timing aggregation (1h)
- [ ] Update step logs with duration stats (1h)
- [ ] Test metrics collection (1h)

### Week 2: Advanced Metrics
- [ ] Add BrowserPool stats collection (2h)
- [ ] Implement pool acquisition timing (1h)
- [ ] Add error context to ClassifyError (2h)
- [ ] Update browser.go error logging (1h)
- [ ] Test error attribution (1h)

### Week 3: Network & Retry
- [ ] Implement NetworkAggregation (2h)
- [ ] Wire aggregation into TaskResult (1h)
- [ ] Add RetryMetrics to Queue (2h)
- [ ] Track retry success/failure (1h)
- [ ] Test retry tracking (1h)

### Week 4: Observability
- [ ] Add TraceID to Task model (1h)
- [ ] Propagate TraceID through all logs (3h)
- [ ] Implement structured event logging (2h)
- [ ] Test trace ID correlation (1h)
- [ ] Documentation (1h)

### Week 5: Validation & Dashboards
- [ ] Integration testing with real workloads (8h)
- [ ] Create Prometheus exporter (4h)
- [ ] Grafana dashboard (4h)
- [ ] Performance regression testing (4h)

---

## Validation Strategy

### Unit Tests
```bash
go test -tags=dev ./internal/logs -v
go test -tags=dev ./internal/queue -v -run TestMetrics
go test -tags=dev ./internal/browser -v -run TestPoolStats
```

### Integration Tests
```bash
# Simulate workload
for i in {1..100}; do
  curl -X POST http://localhost:3000/api/task/create \
    -d '{"name":"test-$i","url":"http://chhotu-bin.infy.uk"}'
done

# Observe metrics
curl http://localhost:3000/api/metrics/queue
curl http://localhost:3000/api/metrics/browser-pool
curl http://localhost:3000/api/task/TASK_ID  # Check result.StepTimings
```

### Performance Benchmarks
```bash
go test -bench=BenchmarkStepLogger -benchmem ./internal/logs
go test -bench=BenchmarkNetworkLogger -benchmem ./internal/logs
go test -bench=BenchmarkQueueMetrics -benchmem ./internal/queue
```

---

## Files to Modify

1. `internal/queue/queue.go` - Queue metrics
2. `internal/logs/logger.go` - Step timing
3. `internal/logs/network.go` - Network aggregation
4. `internal/browser/browser.go` - Error context, trace ID
5. `internal/browser/pool.go` - Pool stats
6. `internal/models/models.go` - New structs
7. `internal/models/errors.go` - Error classification
8. `app_tasks.go` - Trace ID in Task
9. `app.go` - Expose metrics API

---

## Estimated Timeline

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Foundation | 5 days | Queue & step metrics working |
| Advanced | 5 days | Error & pool telemetry |
| Observability | 5 days | Trace ID + structured events |
| Validation | 5 days | Tests + dashboards |
| **Total** | **~20 days** | Production-ready monitoring |

---

## Success Metrics

- [ ] Queue depth API returns accurate metrics
- [ ] Step timings available in task result
- [ ] Error classification improves debugging time by 50%
- [ ] Browser pool stats match actual Chrome processes
- [ ] Trace IDs correlate 100% of logs
- [ ] Monitoring overhead < 5% CPU
- [ ] All metrics queryable via API
- [ ] Dashboard shows queue health in real-time

---

End of Implementation Guide
