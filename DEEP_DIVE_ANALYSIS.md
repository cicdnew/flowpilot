# Deep Dive: Task Execution, Browser Automation & Monitoring

## 1. TASK EXECUTION PIPELINE

### State Progression Flow
```
pending 
    ↓ (App.StartTask / App.CreateTask with AutoStart)
queued 
    ↓ (Queue.Submit → priority heap)
    ↓ (Queue.worker dequeues)
running
    ↓ (Browser.RunTask completes)
completed (success=true) OR failed (error set)
    ↓ (Optional retry with backoff)
retrying
    ↓ (re-enter running)
```

### Entry Points & Call Stack

**Creation Entry** (`app_tasks.go:55-89`):
```go
App.CreateTask(params) 
  → validates params (app_tasks.go:35-52)
  → creates Task with UUID (line 63-76)
  → stores in DB with status='pending' (line 78)
  → optionally calls Queue.Submit if AutoStart=true (line 83)
```

**Execution Entry** (`app_tasks.go:128-140`):
```go
App.StartTask(id)
  → fetches task from DB
  → calls Queue.Submit(ctx, *task)
```

**Queue Submission** (`internal/queue/queue.go:206-228`):
```go
Queue.Submit(ctx, task)
  → validates task not already queued (line 208)
  → adds to priority heap with priority comparison (line 213)
  → enqueues TaskStateChange to pending DB write channel (line 216)
  → signals condition var to wake worker (line 226)
```

**Worker Dequeue** (`internal/queue/queue.go:620-638`):
```go
Queue.worker() // Fixed pool worker
  → blocks on sync.Cond.Wait() (line 632)
  → dequeues runnable task (line 626)
    → checks: not cancelled, not paused (line 646-651)
    → checks: proxy concurrency limit (line 654)
    → checks: proxy geo-availability (line 658)
    → marks as running (line 663)
  → calls Queue.executeTask() (line 629)
```

**Task Execution** (`internal/queue/queue.go:736-780`):
```go
Queue.executeTask(ctx, task, countsAgainstProxyLimit, autoProxy)
  → handles auto-proxy reservation (line 751)
  → updates task status to 'running' in DB (line 756)
  → calls Browser.RunTask(taskCtx, task) (line 762)
  → handles result: success/failure/retry logic (line 764-779)
```

**Browser Execution** (`internal/browser/browser.go:295-373`):
```go
Browser.RunTask(ctx, task)
  → resolves logging policy (line 297)
  → sets up panic recovery (line 305-318)
  → acquires browser context from pool or creates new allocator (line 337)
  → sets up network logging if enabled (line 343)
  → clears cookies (line 345)
  → sets up proxy auth if needed (line 350-356)
  → runs all steps with PC-based control flow (line 358)
  → collects network logs (line 360, 367)
  → returns result with success/error (line 372)
```

---

## 2. BROWSER LIFECYCLE MANAGEMENT

### Browser Pool & Context Acquisition

**Pool Configuration** (`internal/browser/pool.go:52-86`):
```go
NewBrowserPool(cfg PoolConfig, opts []chromedp.ExecAllocatorOption)
  → defaults: Size=5, MaxTabs=10, IdleTimeout=5min, AcquireTimeout=60s
  → initializes priority heap for browser reuse
  → spawns cleanup goroutine (line 83)
```

**Acquire Context** (`internal/browser/pool.go:88-139`):
```go
BrowserPool.Acquire(ctx)
  → tries to reuse existing browser (line 101)
    → selects least-used browser to balance load (line 181)
    → increments inUse counter (line 186)
  → if no reuse available, creates new browser (line 113-132)
    → blocks if at pool limit until slot available (line 135)
    → timeout: 60s per config
  → returns browserCtx + release func (line 132)
```

**Browser Creation** (`internal/browser/browser.go:375-414`):
```go
Runner.createAllocator(ctx, proxyConfig, headless)
  → copies default chromedp options (line 377-378)
  → sets headless mode (line 385-389)
    → respects forced headless (line 381-382)
  → disables optimizations for reliability:
    - GPU (line 391)
    - shared memory (line 392)
    - background networking/extensions/sync (line 394-398)
    - background timer throttling (line 400-402)
  → sets proxy if configured (line 405-411)
  → returns ExecAllocator context + cancel (line 413)
```

### Chrome Process Lifecycle

**Launch Flags** (lines 385-403):
```
headless / headless=false
--disable-gpu
--disable-dev-shm-usage           // Avoid /dev/shm OOM
--js-flags="--max-old-space-size=512"  // V8 memory limit
--disable-background-networking
--disable-default-apps
--disable-extensions
--disable-sync
--disable-translate
--no-first-run
--disable-background-timer-throttling
--disable-renderer-backgrounding
--disable-backgrounding-occluded-windows
```

**Memory Management**:
- Fixed pool size (default 5, max 200)
- Each browser can open max 10 tabs by default
- Idle timeout: 5 minutes
- Cleanup goroutine runs every 30s
- Staggered warm-up: 50ms per worker to avoid thundering herd

**Cleanup** (`internal/browser/pool.go` cleanup loop):
- Removes browsers idle > 5 minutes
- Releases browser contexts and cancels allocators
- Graceful shutdown: waits for all running tasks

---

## 3. HEADLESS BROWSER IMPLEMENTATION (chromedp)

### CDP Protocol Setup

**Network Logging** (`internal/browser/browser.go:268-292`):
```go
Runner.setupNetworkLogging(ctx, taskID, policy, result)
  → creates NetworkLogger if policy.captureNetworkLogs (line 270)
  → registers CDP event listener (line 275)
    → EventRequestWillBeSent: records request start time (line 277)
    → EventResponseReceived: stores response metadata (line 280)
    → EventLoadingFinished: builds network log entry (line 281)
    → EventLoadingFailed: cleanup (line 283)
  → enables network domain (line 288)
  → returns NetworkLogger for step-level association
```

**Network Log Entry Structure** (`internal/models/network_log.go`):
```go
type NetworkLog struct {
    TaskID          string
    StepIndex       int           // Which step made this request
    RequestURL      string
    Method          string
    StatusCode      int
    MimeType        string
    DurationMs      int64
    ResponseHeaders string        // JSON
    RequestHeaders  string        // JSON
    ResponseSize    int64         // EncodedDataLength from CDP
    Timestamp       time.Time
}
```

**Proxy Authentication** (`internal/browser/browser.go:677-691`):
```go
Runner.setupProxyAuth(ctx, proxyConfig)
  → registers Fetch domain listener (line 678)
    → EventAuthRequired: responds with username/password (line 680)
    → EventRequestPaused: continues request (line 682)
  → enables fetch interceptor (line 687)
```

### Multi-Tab Support

**Tab Context Creation** (`internal/browser/pool.go:199+`):
- `chromedp.NewContext(allocCtx)` creates new tab within existing browser
- Each tab is independent context tree
- Parent: allocCtx → browserCtx → tabCtx

**Tab Switching** (`internal/browser/steps.go` ActionTabSwitch):
- Records which CDP target is active
- Subsequent actions target current tab

**Event Listener for Tab Switching** (`app.go` references EventTargetInfoChanged):
- Monitors Chrome DevTools Protocol for tab creation/destruction
- Updates recorder's current tab pointer

---

## 4. BROWSER STEP EXECUTOR

### PC-Based Control Flow Engine

**Step Execution Loop** (`internal/browser/browser.go:418-632`):

The runner uses a **program counter (PC)** based approach instead of recursion:

```go
runSteps(steps []TaskStep)
  → builds label index for goto targets (line 430)
  → initializes loop stack and while stack (line 437-445)
  → pc := 0
  → for pc < len(steps):
       step := steps[pc]
       
       // Control flow handlers (no executeStep call)
       case ActionLoop:    push frame, pc++ 
       case ActionEndLoop: pop frame, conditional jump
       case ActionWhile:   evaluate condition, push frame or skip
       case ActionGoto:    unconditional jump
       case ActionIfElement/IfText/IfURL: conditional jump
       
       // Normal execution with timeout
       case all others:
         timeout := step.Timeout or 30s
         stepCtx, cancel := WithTimeout(browserCtx, timeout)
         err := executeStep(stepCtx, step, result)
         cancel()
         
         // Log step completion
         stepLogger.EndStep(...)
         
         if err != nil:
           result.Error = formatted error
           return err
         
         pc++
```

**Why PC-based?**
- Supports loops, conditionals, and gotos without recursion limits
- Allows max iterations on while loops (default 1000)
- Easy to jump/break/continue
- Simple serialization for debugging

### Step Action Dispatch

**Simple Actions** (`internal/browser/steps.go:23-79`):
- No result parameter needed: navigate, click, type, wait, scroll, select, eval, tab_switch, etc.
- Dispatch via switch statement

**Complex Actions** (`internal/browser/steps.go:85-132`):
- Need task result: extract, solve_captcha, get_title, get_attributes, click_ad, conditional logic, variable operations, etc.
- Dispatch via handler map (line 86-122)

### Timeout & Error Handling

**Per-Step Timeout** (line 567-570):
```go
timeout := defaultTimeout  // 30s
if step.Timeout > 0 {
    timeout = time.Duration(step.Timeout) * time.Millisecond
}
stepCtx, stepCancel := context.WithTimeout(browserCtx, timeout)
```

**Error Classification** (`internal/browser/browser.go:588` + `internal/models/errors.go`):
```go
code := models.ClassifyError(err)
// Returns: SELECTOR_NOT_FOUND, TIMEOUT, NETWORK_ERROR, EVAL_BLOCKED, etc.
```

**Panic Recovery** (`internal/browser/browser.go:304-318`):
```go
defer func() {
    if p := recover(); p != nil {
        // Catches chromedp upstream panics (e.g., "close of closed channel")
        result.Error = formatted panic message
        err = fmt.Errorf("browser panic: %v", p)
    }
}()
```

### All ~50 Step Actions

**Navigation**: navigate, navigate_back, navigate_forward, reload

**DOM Interaction**: click, type, double_click, context_click, hover, drag_drop, scroll, scroll_into_view, submit_form, file_upload

**Element Checks**: wait, wait_visible, wait_not_present, wait_enabled, wait_function

**Selection**: select, select_random

**Form**: emulate_device, focus, blur, clear

**JavaScript**: eval, get_session, set_session, load_session, save_session

**Cookies**: get_cookies, set_cookie, delete_cookies

**Storage**: get_storage, set_storage, delete_storage

**Extract & Inspection**: extract, get_title, get_attributes, screenshot, highlight

**Control Flow**: loop, end_loop, while_condition, end_while, break_loop, goto, if_element, if_text, if_url, if_not_exists, if_visible, if_enabled

**Variables**: variable_set, variable_math, variable_string

**Anti-Bot**: anti_bot, solve_captcha, click_ad, random_mouse, human_typing

**Download**: download

**Caching**: cache_get, cache_set, cache_clear

**Debug**: debug_pause, debug_resume, debug_step

---

## 5. CURRENT MONITORING & LOGGING

### What's Currently Tracked

**Task Logs** (TaskResult.Logs):
```go
type LogEntry struct {
    Timestamp time.Time
    Level     string    // "info", "warn", "error", "debug"
    Message   string
}
```
- Captured via `Runner.addLog()` (line 693-714)
- Task creation, browser pool usage, step completion, errors
- Limited to LogLimit (default 1000 entries per task)
- Output: slog (structured logging) + in-memory array

**Step Logs** (TaskResult.StepLogs):
```go
type StepLog struct {
    TaskID      string
    StepIndex   int
    Action      StepAction
    Selector    string
    Value       string
    SnapshotID  string
    ErrorCode   string
    DurationMs  int64
    StartedAt   time.Time
    ErrorMsg    string
}
```
- Captured via `StepLogger.EndStep()` (internal/logs/logger.go:37-54)
- Per-step duration, error classification
- Stored in DB: `step_logs` table
- Disabled if LoggingPolicy.CaptureStepLogs = false

**Network Logs** (TaskResult.NetworkLogs):
```go
type NetworkLog struct {
    TaskID          string
    StepIndex       int
    RequestURL      string
    Method          string
    StatusCode      int
    MimeType        string
    DurationMs      int64
    ResponseHeaders string  // JSON
    RequestHeaders  string  // JSON
    ResponseSize    int64
    Timestamp       time.Time
}
```
- Captured via `NetworkLogger` (internal/logs/network.go)
- Max 10,000 entries per task
- Limited pending requests: 5,000 concurrent
- Stored in DB: `network_logs` table
- Disabled if LoggingPolicy.CaptureNetworkLogs = false

**Screenshots** (TaskResult.Screenshots):
- File paths only
- Stored in disk directory (default: ~/.flowpilot/screenshots)
- Disabled if LoggingPolicy.CaptureScreenshots = false

**Queue Metrics** (Queue.metrics):
```go
type QueueMetrics struct {
    TotalSubmitted  int
    TotalCompleted  int
    TotalFailed     int
    TotalRetried    int
    // ... (only partially used)
}
```
- Incremented on submit/complete/fail/retry
- Not continuously exposed; stale metrics

**Task Events** (task_events table):
```sql
CREATE TABLE task_events (
    id TEXT PRIMARY KEY,
    task_id TEXT,
    from_state TEXT,
    to_state TEXT,
    event_data TEXT,
    created_at DATETIME
);
```
- Stores every status transition
- Audit trail: pending → queued → running → completed
- Limited query via `App.ListAuditTrail()`

### Where Logs Go

1. **In-Memory**: TaskResult.Logs (limited to LogLimit)
2. **Database**: 
   - `step_logs` table
   - `network_logs` table
   - `task_events` table
3. **Disk**: Screenshots in screenshotDir
4. **slog**: Structured logging to stdout/file (if configured)
5. **Wails IPC**: `task:event` emitted to frontend on status change

### Critical Gaps

1. **No Browser Metrics**
   - Heap size, memory usage of Chrome processes
   - Tab count per browser
   - Pool utilization (how many browsers idle vs. in-use)

2. **No Queue Metrics**
   - Tasks pending in priority queue (current vs. high watermark)
   - Worker utilization (how many workers busy vs. idle)
   - Backlog depth by priority
   - Proxy concurrency pressure (running vs. limit)

3. **No Step-Level Performance Metrics**
   - Each step's duration not extracted / queryable
   - No slowest-steps analysis
   - No selector resolution time

4. **No Network Analysis**
   - Total network time per task not aggregated
   - Failed requests not flagged distinctly
   - No bandwidth/payload size metrics
   - No redirect chains tracking

5. **No Task Retry Metrics**
   - Which proxies caused retries
   - Retry success rate by proxy/geo
   - Backoff timing not logged

6. **No Error Metrics**
   - Error distribution by type (selector, timeout, network, eval, etc.)
   - Error rates by action type
   - No error correlation analysis

7. **No Execution Context**
   - Which proxy was used for each step (if auto-proxy)
   - Task dependencies / batch execution metrics
   - Concurrency bottleneck identification

8. **No Alerting**
   - Queue depth > threshold
   - Task failure rate > threshold
   - Browser pool saturation

---

## 6. RECOMMENDED MONITORING & LOGGING IMPROVEMENTS

### 20+ High-Impact Improvements

#### Category 1: Browser & Resource Monitoring

1. **Browser Pool Telemetry**
   - Expose `BrowserPool.browsers` metrics: total, idle, in-use, max tabs used
   - Gauge: `flowpilot_browser_pool_size{state=idle|in_use|total}`
   - Track pool acquisition latency: `flowpilot_browser_acquire_duration_ms`
   - Alert if pool full for > 30s

2. **Chrome Process Memory**
   - Poll `/proc/{pid}/status` for each browser process
   - Gauge: `flowpilot_chrome_memory_mb{browser_id}`
   - Detect OOM risk (> 1GB per process)
   - Alert if memory trend rises steeply

3. **Tab Lifecycle Tracking**
   - Log on tab create/destroy: `{ event: "tab_created", browser_id, tab_count }`
   - Detect tab leaks (count > max_tabs)
   - Gauge: `flowpilot_browser_tabs{browser_id}`

4. **Allocator Context Leak Detection**
   - Track allocCtx/browserCtx creation and cancellation
   - Warn if cancel not called after 10s
   - Gauge: `flowpilot_browser_contexts_uncancelled`

#### Category 2: Queue & Concurrency Monitoring

5. **Queue Depth Metrics**
   - Expose pending queue size: `flowpilot_queue_pending{priority=high|normal|low}`
   - High watermark per priority
   - Gauge: `flowpilot_queue_backlog_seconds` (estimate wait time)

6. **Worker Pool Utilization**
   - Track workers busy vs. idle per second
   - Gauge: `flowpilot_workers_busy / flowpilot_workers_total`
   - Alert if utilization > 90% for > 2min

7. **Proxy Concurrency Pressure**
   - Expose current vs. limit: `flowpilot_proxy_concurrent{proxy_id}`
   - Gauge: `flowpilot_proxy_queue_deferred` (tasks waiting on proxy limit)
   - Alert if proxy queue > 100

8. **Task Wait Time**
   - Record time from submit to dequeue: `flowpilot_task_wait_duration_ms{priority}`
   - Percentile: p50, p95, p99
   - Detect queue saturation early

9. **Dequeue Cycle Time**
   - Measure time per dequeue decision: `flowpilot_dequeue_cycle_ms`
   - Detect algorithmic bottleneck (should be <1ms)
   - Alert if > 100ms (indicates lock contention)

#### Category 3: Task Execution Monitoring

10. **Task Lifecycle Events with Metadata**
    - Log structured events on all transitions:
      ```json
      {
        "event": "task_transition",
        "task_id": "uuid",
        "from_state": "pending",
        "to_state": "queued",
        "metadata": { "priority": 5, "batch_id": "...", "proxy": "..." }
      }
      ```
    - Store in `task_events` with task context

11. **Step-by-Step Performance Heatmap**
    - Aggregate step duration percentiles by action:
      ```json
      {
        "action": "click",
        "p50_ms": 145,
        "p95_ms": 820,
        "p99_ms": 2100,
        "samples": 15000
      }
      ```
    - Histogram: `flowpilot_step_duration_ms{action}`
    - Detect regressions

12. **Selector Resolution Timing**
    - Log selector evaluation time separately:
      ```go
      start := time.Now()
      err := chromedp.WaitVisible(selector)
      selectorTime := time.Since(start)
      ```
    - Histogram: `flowpilot_selector_resolution_ms`
    - Flag slow selectors (> 5s)

13. **Task Error Attribution**
    - Log error with full context:
      ```json
      {
        "error_code": "SELECTOR_NOT_FOUND",
        "step_index": 5,
        "action": "click",
        "selector": "#non-existent",
        "task_id": "uuid",
        "proxy": "proxy-1.com",
        "url": "https://...",
        "duration_ms": 250
      }
      ```
    - Counter: `flowpilot_step_errors_total{code, action}`

14. **Task Retry Tracking**
    - Log on retry with reason:
      ```json
      {
        "event": "task_retry",
        "task_id": "uuid",
        "retry_count": 1,
        "reason": "network timeout",
        "backoff_ms": 500,
        "proxy_changed": false
      }
      ```
    - Gauge: `flowpilot_task_retries{proxy, reason}`

#### Category 4: Network Performance

15. **Network Timing Aggregation**
    - Aggregate network logs per task:
      ```json
      {
        "task_id": "uuid",
        "total_requests": 48,
        "failed_requests": 2,
        "total_response_time_ms": 3200,
        "largest_response_mb": 2.5,
        "average_ttfb_ms": 150
      }
      ```
    - Histogram: `flowpilot_network_request_duration_ms{status_code}`

16. **Redirect Chain Tracking**
    - Detect redirect loops:
      ```go
      type RedirectChain struct {
          requests []NetworkLog
          loop_detected bool
      }
      ```
    - Alert if chain > 5 redirects

17. **Failed Request Analysis**
    - Count by status code: `flowpilot_network_failures_total{status_code}`
    - Log failed requests with context: `{ url, method, status, step_index }`

18. **Response Size Trend**
    - Track total response bytes per task
    - Gauge: `flowpilot_task_response_bytes`
    - Alert if > 500MB (data leak risk)

#### Category 5: Observability & Debugging

19. **Distributed Tracing Integration**
    - Add trace ID to task creation: `trace_id := uuid.New()`
    - Propagate through all logs (step, network, events)
    - Enable correlation: task → steps → network requests

20. **Execution Flamegraph Data**
    - Export pprof CPU/memory profiles per task batch
    - Endpoint: `GET /debug/pprof/profile?task_id=...&duration=30s`
    - Identify hot paths in runSteps

21. **Browser State Snapshots**
    - On error, log browser state:
      ```json
      {
        "error": "selector not found",
        "browser_state": {
          "url": "https://...",
          "title": "...",
          "cookies_count": 12,
          "local_storage_keys": ["session", "..."]
        }
      }
      ```

22. **Task Execution Timeline**
    - Export task duration breakdown:
      ```json
      {
        "task_id": "uuid",
        "total_ms": 5000,
        "phases": {
          "browser_acquire_ms": 200,
          "navigation_ms": 800,
          "steps_ms": 3500,
          "cleanup_ms": 500
        }
      }
      ```

---

## 7. IMPLEMENTATION ROADMAP

### Phase 1: Foundation (Days 1-2)
- [ ] Add TaskMetrics struct (execution time breakdown)
- [ ] Wire step duration capture in StepLogger
- [ ] Add error attribution to errors.go ClassifyError
- [ ] Implement BrowserPool telemetry export
- [ ] Add queue depth metrics to Queue struct

### Phase 2: Advanced Metrics (Days 3-4)
- [ ] Network log aggregation service
- [ ] Retry tracking in Queue
- [ ] Worker utilization gauge
- [ ] Proxy concurrency pressure metrics
- [ ] Error distribution counter

### Phase 3: Observability (Days 5-6)
- [ ] Structured event logging (JSON + slog)
- [ ] Trace ID propagation
- [ ] pprof integration
- [ ] Browser state snapshots on error
- [ ] Task timeline export

### Phase 4: Alerting & Dashboard (Days 7+)
- [ ] Prometheus exporter endpoint
- [ ] Alert rules (queue depth, error rate, pool saturation)
- [ ] Grafana dashboard
- [ ] Historical trend analysis

---

## 8. CODE LOCATIONS REFERENCE

| Component | File | Key Lines |
|-----------|------|-----------|
| Task Creation | `app_tasks.go` | 55-89 |
| Task Submission | `app_tasks.go` | 128-140 |
| Queue Submit | `internal/queue/queue.go` | 206-228 |
| Worker Loop | `internal/queue/queue.go` | 620-638 |
| Task Execution | `internal/queue/queue.go` | 736-780 |
| Browser RunTask | `internal/browser/browser.go` | 295-373 |
| Step Executor (PC-based) | `internal/browser/browser.go` | 418-632 |
| Step Dispatch | `internal/browser/steps.go` | 23-132 |
| Pool Acquisition | `internal/browser/pool.go` | 88-139 |
| Browser Creation | `internal/browser/browser.go` | 375-414 |
| Network Logging | `internal/browser/browser.go` | 268-292 |
| Network Logger | `internal/logs/network.go` | 1-128 |
| Step Logger | `internal/logs/logger.go` | 1-59 |
| Task Logging | `internal/browser/browser.go` | 693-714 |
| Error Classification | `internal/models/errors.go` | (check file) |
| Retry Logic | `internal/queue/queue.go` | 772-779 |
| Panic Recovery | `internal/browser/browser.go` | 304-318 |
| Proxy Auth | `internal/browser/browser.go` | 677-691 |

---

## 9. KEY DIAGRAMS

### Task Execution State Machine
```
         CreateTask
            ↓
       [PENDING] ←--------- (on AutoStart=false)
            ↓
       StartTask()
            ↓
    Queue.Submit()
            ↓
       [QUEUED] ←--------- (in priority heap)
            ↓
   Queue.worker dequeues
            ↓
    Browser.RunTask()
            ↓
       [RUNNING]
            ├→ success → [COMPLETED] (success=true)
            │
            ├→ error ──→ [FAILED] (error set)
            │              ↓
            │           should_retry?
            │              ├→ YES → [RETRYING]
            │              │          ↓ (wait backoff)
            │              │       re-queue → [QUEUED]
            │              │
            │              └→ NO → END
            │
            └→ cancelled → [CANCELLED] (by user)
```

### PC-Based Step Execution
```
pc=0: navigate("https://...")
     ↓ (not ctrl flow, timeout 30s, execute)
     ↓ (no error, continue)
pc=1: loop(count=5)
     ↓ (push loop frame, pc++)
pc=2: click(selector="button")
     ↓ (timeout 30s, execute, log)
pc=3: if_element(selector="next")
     ├→ true: jump_to="loop_body", pc = labelIndex["loop_body"]
     └→ false: pc++
pc=4: end_loop
     ├→ iteration < maxIter: pc = loopFrame.startPC+1 (jump back)
     └→ iteration == maxIter: pop loop, pc++
pc=5: screenshot()
     ↓ (execute, save)
pc=6: (end of steps)
     ↓
RETURN nil (success)
```

### Browser Pool Lifecycle
```
NewBrowserPool(size=5)
    ↓
for i=0..4: spawn pooledBrowser
    ↓
pooledBrowser {
    allocCtx      ← chromedp.NewExecAllocator(baseCtx, opts...)
    browserCtx    ← chromedp.NewContext(allocCtx)
    maxTabs = 10
    inUse = 0
    lastUsed = now
}
    ↓
Task.RunTask()
    ├→ Acquire() calls pool.acquireReusableBrowser()
    │  ├→ if browser.inUse < maxTabs: return browser, inUse++
    │  └→ else: create new or wait
    ├→ chromedp.NewContext(browser.browserCtx) ← new tab
    ├→ execute steps
    └→ release() → inUse--, lastUsed=now
    ↓
cleanup loop (every 30s)
    └→ if browser.lastUsed < 5min: cancel allocCtx, remove
```

---

## 10. Performance Considerations

### Bottlenecks
1. **Lock Contention**: Queue.mu held during dequeue (can block workers)
2. **Chromedp Overhead**: CDP protocol round-trips for each action
3. **Network Logging**: Mutex per NetworkLogger (can serialize CDP events)
4. **Database Writes**: Batched every 100ms but might still cause latency spikes
5. **Selector Resolution**: WaitVisible can timeout if selector not yet in DOM

### Optimization Opportunities
1. Use atomic.Value for metrics instead of lock-held maps
2. Shard NetworkLogger by request ID range
3. Batch step logs in memory before DB insert
4. Implement selector caching for repeated selectors
5. Parallelize network log writes via channels

---

## 11. Security Considerations

### Current Protections
- Proxy credentials encrypted at rest (internal/crypto)
- Eval scripts validated (size, patterns, function count limits)
- File upload sandbox (pathWithinBase checks)
- Input validation at API boundaries

### Logging Security
- Screenshots written with 0o600 permissions (owner-only)
- Network logs may contain sensitive headers → consider redaction
- Task logs may contain user-entered data (passwords in type steps) → sanitize
- Task event audit trail is unsanitized → control access

---

## Appendix: Models Reference

### TaskResult
```go
type TaskResult struct {
    TaskID         string
    Success        bool
    Error          string
    Duration       time.Duration
    Logs           []LogEntry        // Task logs
    StepLogs       []StepLog         // Step-level logs
    NetworkLogs    []NetworkLog      // HTTP requests
    Screenshots    []string          // File paths
    ExtractedData  map[string]string // Extracted values
    LogLimit       int               // Max log entries
}
```

### QueueMetrics
```go
type QueueMetrics struct {
    TotalSubmitted  int
    TotalCompleted  int
    TotalFailed     int
    TotalRetried    int
    // NOTE: Not fully utilized; many gaps
}
```

### Task State Transitions
- pending: Initial state, not yet queued
- queued: In priority heap, waiting for worker
- running: Worker executing Browser.RunTask
- completed: Browser.RunTask returned success=true
- failed: Browser.RunTask returned error
- retrying: Waiting to retry after backoff
- cancelled: Cancelled by user

---

## End of Deep Dive Analysis
