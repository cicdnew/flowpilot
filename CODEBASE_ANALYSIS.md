# FlowPilot Codebase Analysis

## 1. internal/queue/queue.go

### Purpose
Manages task scheduling and execution using a fixed worker pool with a priority heap. Handles concurrency limiting, task lifecycle (queued → running → completed/failed), proxy management, and retry logic with exponential backoff.

### Public API Surface

**Key Types:**
- `Queue`: Main task scheduler
- `EventCallback`: Function type for status change notifications
- Constants: `ErrQueueFull`, `ErrBatchPaused`

**Core Methods:**
- `New(db, runner, maxConcurrency, onEvent)`: Creates queue with fixed worker pool
- `Submit(ctx, task)`: Enqueues single task, returns `ErrQueueFull` if at capacity
- `SubmitBatch(ctx, tasks)`: Bulk enqueue with single transaction
- `Cancel(taskID)`: Stops running/pending task, marks cancelled
- `PauseBatch(batchID)`: Moves batch tasks to paused heap
- `ResumeBatch(batchID)`: Restores paused tasks to main queue
- `Stop()`: Idempotent shutdown, cancels all tasks
- `RunningCount()`: Returns currently executing count
- `Metrics()`: Snapshot of queue stats (Running, Queued, Pending, TotalSubmitted, etc.)
- `RecoverStaleTasks(ctx)`: Finds crashed-task remnants, resets to pending, re-submits
- `SetProxyManager(pm)`, `SetProxyConcurrencyLimit(limit)`: Configure proxy pooling

### Internal Structure

**Concurrency Model:**
- Single `sync.Mutex` + `sync.Cond` for worker coordination
- Fixed worker goroutines (one per maxConcurrency) block on condition variable
- Program-counter pattern in `worker()` loop via goto
- Persistence worker (separate goroutine) batches DB writes with configurable flush interval

**Data Structures:**
- `pq` (taskHeap): Main priority queue, higher priority dequeued first, FIFO tiebreaker
- `pausedPQ`: Paused batch tasks (moved back on resume)
- `heapSet`, `pausedSet`: O(1) lookup maps for membership testing
- `running`: Map of taskID → cancelFunc (active tasks)
- `cancelled`, `paused`: Maps for state tracking

**Task Lifecycle:**
1. Submit → Queued (status change enqueued to DB)
2. Worker dequeues → Running (timeout-aware context created)
3. On success → Completed (via `FinalizeTaskSuccess`)
4. On failure:
   - If retries remain: Retrying → scheduled retry after exponential backoff (2^retryCount, capped at 60s)
   - If no retries: Failed (via `FinalizeTaskFailure`)
5. On cancellation → Cancelled

**Proxy & Concurrency Limiting:**
- `proxyConcurrencyLimit`: Max simultaneous proxy-using tasks
- `runningProxied`: Counter of proxy-limited tasks currently running
- Dequeue logic defers proxy tasks if limit reached, allows non-proxy tasks
- Auto-proxy selection via `ProxyManager.ReserveProxyWithFallback()`
- On proxy failure: falls back to direct or fails with error

**Persistence:**
- Async persistence channel with batching: `persistenceBatchSize` items or `persistenceInterval` (100ms)
- On queue stop: flushes remaining writes before shutdown
- DB write timeout: 5 seconds

### Notable Patterns

1. **Staggered worker startup**: Workers 1+ sleep 50ms * workerID (max 2s) before entering loop
2. **Heap management**: Main/paused heaps rebuilt via pop/push cycle (not in-place modification)
3. **Deferred proxy tasks**: Tasks that can't claim proxy slot pushed back to heap for later retry
4. **Context cascade**: Parent ctx → task ctx (with custom timeout) → step ctx (timeout from step)
5. **Cancellation handling**: Both graceful (context.Canceled) and explicit (Cancel() method)

### Limitations & TODOs

1. **Linear heap removal** in `removeFromHeap()`: O(n) scan instead of indexed removal (heap.Remove needs index)
   - Impact: Cancel operations on large queues slow; workaround needed for >10k pending tasks
2. **No task priority escalation**: Low-priority tasks may starve forever if high-priority always available
3. **No dead-letter queue**: Failed tasks after max retries discarded; no manual inspection/retry
4. **Simplistic auto-proxy fallback**: Doesn't distinguish geo-fallback reason (no healthy → bad credentials)
5. **No task dependency support**: Can't express "run B after A completes"
6. **Metrics are snapshots**: No historical tracking (cumulative counters only)
7. **Persistence batch loss on crash**: If process dies mid-flush, some state changes lost (trade-off for perf)

---

## 2. internal/queue/priority_heap.go

### Purpose
Implements Go's `container/heap` interface for priority-based task scheduling.

### Public API Surface

**Types:**
- `heapItem`: Wraps task with context, cancel function, and metadata
  - Fields: `task`, `ctx`, `cancel`, `addedAt`, `index`
- `taskHeap`: []*heapItem, implements heap.Interface

**Interface Methods:**
- `Len() int`: Heap size
- `Less(i, j int) bool`: Higher priority first; FIFO tiebreaker by `addedAt`
- `Swap(i, j int)`: Exchanges items, updates their indices
- `Push(x any)`: Appends heapItem, sets index
- `Pop() any`: Removes root, clears leak ref, resets index
- `peek() *heapItem`: Non-destructive view of top item (nil if empty)

### Notable Patterns

- **Dual sort key**: Priority DESC, then `addedAt` ASC
- **Index tracking**: Each item maintains its heap index for O(1) removal via `heap.Remove()`
- **Memory leak prevention**: `old[n-1] = nil` in Pop() to allow GC

### Limitations

1. **No re-prioritization**: Can't increase priority mid-execution
2. **No weight-based scheduling**: All high-priority tasks treated equally regardless of complexity

---

## 3. internal/queue/queue_test.go

### Purpose
Comprehensive test suite covering queue lifecycle, priority ordering, batch ops, pause/resume, proxy limiting, metrics, and failure/retry paths.

### Key Test Coverage

**Lifecycle:**
- Submit, SubmitBatch, Cancel (running/pending/paused)
- Stop (cancels all, clears heaps, idempotent)
- RecoverStaleTasks (finds crashed tasks, resets)

**Priority & Ordering:**
- High > Normal > Low (tests heapItem comparison)
- FIFO within same priority
- Pause/Resume batch without affecting others

**Proxy & Concurrency:**
- dequeueRunnableLocked: defers proxy tasks if limit reached
- Auto-proxy tasks (Geo + Fallback) treated as proxy-limited
- Allows non-proxy tasks while proxy slots full

**Failure & Retry:**
- handleFailure: exponential backoff, max retries cap
- scheduleRetry: timer-based resubmission, respects queue stop/context cancel
- Step logging, network logs persisted on retry

**Metrics:**
- Running/Queued/Pending counts correct
- TotalSubmitted incremented per Submit()
- TotalCompleted/TotalFailed updated on terminal state

### Test Patterns

- Helper `setupTestQueue()` with crypto reset, DB temp dir, event capture
- Mock runner (not exercised in unit tests; integration expected)
- Manual heap manipulation for state setup

---

## 4. internal/browser/browser.go

### Purpose
Main browser automation runner. Executes task steps (click, type, navigate, etc.) using chromedp, with logging, proxy auth, CAPTCHA solving, and policy-driven screenshot/log capture.

### Public API Surface

**Type: `Runner`**
- Fields: screenshotDir, allowEval (atomic), forceHeadless (atomic), pool, localProxyManager, defaultLoggingPolicy

**Methods:**
- `NewRunner(screenshotDir)`: Creates runner (eval blocked by default)
- `RunTask(ctx, task)`: Executes full task, returns TaskResult with logs/screenshots/extracted data
- `SetForceHeadless(bool)`, `SetAllowEval(bool)`, `SetCaptchaSolver(solver)`, `SetPool(pool)`, `SetLocalProxyManager(m)`, `SetDefaultLoggingPolicy(policy)`: Configuration setters
- `createAllocator(ctx, proxyConfig, headless)`: Builds chromedp ExecAllocator with safe option copying
- `runSteps(ctx, steps, result, netLogger, policy)`: Program-counter loop executing steps with support for conditions, loops, gotos
- `setupProxyAuth(ctx, proxyConfig)`: Intercepts fetch events for proxy auth challenges
- `evaluateCondition(ctx, step, vars)`: Conditional logic (if_element, if_text, if_url)
- `executeStep(ctx, step, result)`: Dispatches to action-specific handlers
- `addLog(result, level, message)`: Appends log entry, maintains limit

### Step Execution Model

**Program Counter (PC) Loop:**
- Steps indexed 0..n, loop counter-controlled
- Control flow via labels, gotos (ActionGoto), conditionals (ActionIfElement/Text/URL)
- Loop/EndLoop/BreakLoop support with depth tracking
- Extract variable substitution: `{{varName}}` interpolation in conditions

**Action Handlers (40+ actions supported):**
- Navigation: navigate, navigate_back, navigate_forward, reload
- Interaction: click, type, double_click, hover, drag_drop, context_click
- Selection: select, select_random
- File ops: file_upload, download
- Data extraction: extract, get_title, get_attributes, get_cookies, get_storage, get_session
- Waits: wait, wait_visible, wait_not_present, wait_enabled, wait_function
- Controls: scroll, scroll_into_view, submit_form, tab_switch
- Advanced: click_ad (auto-discovers Google ads), solve_captcha
- Variables: variable_set, variable_math, variable_string
- Eval (guarded): eval, wait_function (validates script size, blocks dangerous patterns)
- Bot evasion: anti_bot, random_mouse, human_typing
- Debug: debug_pause, debug_resume, debug_step

**Logging Policy:**
- Defaults: capture all (step logs, network logs, screenshots)
- Overridable at runner level (SetDefaultLoggingPolicy) or per-task
- Screenshot limit enforced by result.LogLimit
- Network logs linked to step index

### Proxy & Auth Flow

1. LocalProxyManager converts upstream proxy → local endpoint (if configured)
2. Direct proxy server passed to chromedp allocator
3. Auth credentials handled via fetch.EventAuthRequired listener
4. Fetch API enabled if proxy has username/password

### Security & Validation

**Eval Script Restrictions (allowEval=false by default):**
- Max size: 10,000 bytes
- Blocks: child_process, require(), process.exit/env, fs ops, __dirname, __filename
- Used by: eval action, wait_function condition

**Screenshot Path Validation:**
- Sanitizes filename (replaces /, \, ., null)
- Checks path doesn't escape screenshotDir via filepath.HasPrefix

### Notable Patterns

1. **Separate Executor interface**: Allows mocking chromedp for testing
2. **Atomic flags** (allowEval, forceHeadless): Lock-free read
3. **Pool optional**: If pool attached and no proxy → reuse browser, else create fresh
4. **Network capture via ListenTarget**: Subscribes to CDP events, links to step index
5. **Ad discovery script**: Injects JS to find Google ads, extracts coordinates, dispatches synthetic click

### Limitations & Bottlenecks

1. **No JavaScript sandbox**: eval/wait_function can't be easily isolated; relies on user trust
2. **No step retry within task**: If one step fails, entire task fails; no granular retry
3. **Cookie/storage ops use document.cookie**: Doesn't handle HttpOnly cookies; limited to accessible ones
4. **Tab switching by URL string**: No support for relative URLs or fuzzy matching
5. **Screenshot only full-page**: No cropped region screenshots
6. **No network request filtering**: All requests logged; no ability to filter by URL pattern
7. **Eval script size limit (10KB)**: Scripts larger than 10KB rejected; workaround is none (load from file or simplify)
8. **Hard-coded timeouts**: 30s default, no adaptive backoff
9. **No built-in retry for flaky steps**: User must implement via loop + conditions
10. **Screenshot compression fixed at 100%**: No quality control

---

## 5. internal/browser/executor.go

### Purpose
Thin abstraction over chromedp for testability.

### Public API Surface

**Interface: `Executor`**
- `Run(ctx, ...chromedp.Action) error`
- `RunResponse(ctx, ...chromedp.Action) (*network.Response, error)`
- `Targets(ctx) ([]*target.Info, error)`

**Implementation: `chromeExecutor`** (unexported)
- Delegates to chromedp.Run, chromedp.RunResponse, chromedp.Targets

### Design Pattern

Minimal wrapper; enables mock Executor in tests without starting real Chrome.

---

## 6. internal/browser/pool.go

### Purpose
Reuses Chrome process allocators and creates new browser tabs on demand, with idle timeout eviction and max concurrency constraints.

### Public API Surface

**Type: `BrowserPool`**
- Fields: browsers []*pooledBrowser, poolSize, maxTabs, idleTimeout, acquireTimeout, opts, stopped, creating, stopCh, notifyCh, wg

**Type: `PoolConfig`**
- Size, MaxTabs, IdleTimeout, AcquireTimeout

**Methods:**
- `NewBrowserPool(cfg, opts)`: Creates pool with defaults
- `Acquire(ctx)`: Acquires a tab context; blocks until available or timeout
- `Stop()`: Idempotent shutdown
- `stats()`: Returns poolStats (private method)

**pooledBrowser (internal):**
- allocCtx, allocCancel: ExecAllocator context & cancel
- lastUsed: Timestamp for eviction
- inUse: Tab count in use
- maxTabs: Limit per browser

### Acquire Logic

1. Reuse browser with available tab slots (load-balanced: fewest inUse, ties broken by oldest)
2. If all browsers maxed and pool not full, create new browser:
   - NewExecAllocator → warmup via chromedp.Run() → add to pool
3. If pool full, wait on notification channel with deadline
4. On availability signal, retry from step 1
5. Timeout → error after acquireTimeout

### Cleanup Loop

- Runs every 30s (PoolCleanupPeriod)
- Evicts idle browsers: inUse=0 AND (now - lastUsed) > idleTimeout
- Graceful shutdown via chromedp.Cancel(ctx with 5s timeout) before allocCancel()

### Notable Patterns

1. **Creating counter**: Prevents thundering herd; tracks in-flight browser creations
2. **Notification channel (non-blocking send)**: Signals waiter on availability
3. **Graceful browser shutdown**: Allows Chrome to save state before force-kill

### Limitations

1. **No max connection per browser to CDP**: Chrome's default ~256 connections per process
2. **No load-aware creation**: Always uses oldest browser first (doesn't account for step complexity)
3. **No prewarming**: Pool created empty; slow first request
4. **Fixed maxTabs**: Can't dynamically adjust based on memory usage
5. **No eviction on memory pressure**: Only evicts by timeout
6. **No pool stats API**: stats() is private; can't monitor pool health externally

---

## 7. internal/browser/steps.go

### Purpose
Implements 50+ step action handlers for browser automation (navigation, clicking, typing, extraction, advanced features like ad-clicking, CAPTCHA solving, session management, anti-bot evasion).

### Public API Surface

**Main Dispatcher: `executeStep(ctx, step, result) error`**
- Routes to action-specific handler based on step.Action

**Notable Handlers (selected):**

**Navigation & Interaction:**
- `execNavigate(ctx, step)`: Checks HTTP 400+ errors
- `execClick(ctx, step)`: WaitVisible + Click
- `execType(ctx, step)`: WaitVisible + Clear + SendKeys
- `execScrollIntoView(ctx, step)`, `execScroll(ctx, step)`
- `execTabSwitch(ctx, step)`: Activates tab by URL

**Data Extraction:**
- `execExtract(ctx, step, result)`: Extracts element text, stores in ExtractedData[key]
- `execGetTitle(ctx, step, result)`, `execGetAttributes(ctx, step, result)`
- `execGetCookies(ctx, step, result)`, `execGetStorage(ctx, step, result)`
- `execGetSession(ctx, step, result)`: Serializes cookies + localStorage + URL

**Advanced:**
- `execClickAd(ctx, step, result)`: Auto-discovers Google ads (multiple selectors), captures before/after screenshots, extracts metadata
- `execSolveCaptcha(ctx, step, result)`: Delegates to CaptchaSolver, injects token via JS
- `execEval(ctx, step)`: Validates script, executes if allowEval=true
- `execWaitFunction(ctx, step)`: Polls custom JS until true

**Variables:**
- `execVariableSet(ctx, step, result)`: Stores literal value
- `execVariableMath(ctx, step, result)`: +, -, *, / operations on vars
- `execVariableString(ctx, step, result)`: concat, upper, lower, trim, substring, replace, length

**Session & Storage:**
- `execSetCookie(ctx, step, result)`, `execDeleteCookies(ctx, step, result)`
- `execSetStorage(ctx, step, result)`, `execDeleteStorage(ctx, step, result)` (localStorage/sessionStorage)
- `execLoadSession(ctx, step, result)`, `execSaveSession(ctx, step, result)`: Persist/restore session

**Bot Evasion:**
- `execAntiBot(ctx, step, result)`: Randomizes fingerprint, timezone
- `execRandomMouse(ctx, step)`: Dispatches synthetic mousemove events
- `execHumanTyping(ctx, step)`: Types with random delays between characters

**Debug:**
- `execDebugPause(ctx, step, result)`: Sets _debug_paused flag
- `execDebugStep(ctx, step, result)`, `execDebugResume(ctx, step, result)`

### Key Helpers

- `requireSelector(action, selector) error`: Validates non-empty selector
- `requireValue(action, value) error`: Validates non-empty value
- `parseViewportSize(val) (width, height, error)`: Parses "WIDTHxHEIGHT"
- `sanitizeFilename(name string) string`: Replaces unsafe chars
- `captureAdScreenshot(ctx, result, keyPrefix, label)`: Saves screenshot, tracks path

### Data Flow

**Extracted Data Map:**
- Extract steps populate `result.ExtractedData[key]`
- Variable operations prefix keys with "var_"
- Metadata suffixed with "_selector", "_tag", "_href", etc.
- Debug steps use "_debug_" prefix

### Notable Patterns

1. **JavaScript snippet constants**: adDiscoveryScript, adClickAtScript inlined
2. **Ad metadata extraction before click**: Captures selector, tag, href, coordinates
3. **Coordinate-based click for iframes**: Works around cross-origin restrictions
4. **Conditional element checks inline in step**: if_exists, if_not_exists, if_visible, if_enabled use JS evaluation
5. **Variable interpolation**: {{varName}} replaced in conditions before evaluation

### Limitations

1. **No action chaining**: Each action independent; no composed actions
2. **Ad discovery hardcoded to Google**: Can't customize selectors for other ad networks
3. **No multi-select extraction**: Extracts only first match; no array return
4. **Human typing has fixed delay range** (50-150ms): No behavioral profiles
5. **No element interaction validation**: Doesn't verify click actually executed
6. **Screenshot always full-page**: No element-specific screenshots
7. **Variable math no type coercion**: Fails silently if non-numeric value
8. **No step cancellation mid-execution**: Must wait for timeout
9. **Session serialization via JSON.stringify**: Fails on circular refs, non-serializable types
10. **Cookie/storage limited to document.cookie**: HttpOnly, Secure flags not accessible

---

## 8. internal/browser/conditions.go

### Purpose
Evaluates conditional expressions (if_element, if_text, if_url) used in step control flow.

### Public API Surface

**Main Method: `evaluateCondition(ctx, step, vars) (bool, error)`**
- Dispatches on step.Action (ActionIfElement, ActionIfText, ActionIfURL)

**Condition Types:**

1. **if_element (selector-based):**
   - Checks element exists via chromedp.Nodes(selector, AtLeast(0))
   - Condition "not_exists" → len(nodes)==0, else → len(nodes)>0

2. **if_text (text matching):**
   - Extracts text from selector
   - Evaluates condition against text (see below)

3. **if_url (URL matching):**
   - Gets current URL via chromedp.Location()
   - Evaluates condition against URL

**Text Condition Operators:**
- `contains:substring` → strings.Contains(text, substring)
- `not_contains:substring`
- `equals:exact` → text == exact
- `not_equals:exact`
- `starts_with:prefix`
- `ends_with:suffix`
- `matches:regex` → regexp.Compile().MatchString()
- Plain string (no operator) → default to contains

**Variable Substitution:**
- Conditions can include `{{varName}}` placeholders
- Replaced with values from vars map before evaluation

**Helper Functions:**
- `evaluateTextCondition(condition, text, vars) (bool, error)`: Parses operator, performs substitution
- `buildLabelIndex(steps) map[string]int`: Pre-builds step label → PC mapping
- `findEndLoop(steps, loopPC) int`: Scans forward for matching EndLoop, accounts for nesting

### Notable Patterns

1. **Fail-open on extraction error**: if_text returns (false, nil) on text extraction failure
2. **Regex compilation per evaluation**: No caching; performance hit for repeated conditions
3. **Label indexing upfront**: O(steps) scan in runSteps() before loop starts
4. **Loop depth tracking**: Handles nested loops correctly

### Limitations

1. **No negative lookahead/lookbehind in regex**: Regex limited to Go's re2 syntax
2. **No numeric comparison**: Can't do `< 5`, `>= 10`; only string ops
3. **Variable substitution naive**: Replaces all occurrences, no escaping for special chars
4. **No condition short-circuit**: Evaluates full expression even if early decision possible
5. **Label index global**: Doesn't support dynamic label creation
6. **Loop depth fixed at 100 iterations**: FindEndLoop scans up to 100 steps
7. **No conditional variables**: Can't conditionally set variables

---

## 9. internal/agent/agent.go

### Purpose
Headless background service that polls database for pending tasks, submits them to queue, manages browser runner and proxy health checks.

### Public API Surface

**Type: `Agent`**
- Fields: db, runner, queue, proxyManager, dataDir, pollInterval, cancel

**Type: `Config`**
- DataDir, MaxConcurrency, ProxyConcurrency, PollInterval, HealthCheckInterval, MaxProxyFailures
- CaptureStepLogs, CaptureNetworkLogs, CaptureScreenshots, MaxExecutionLogs

**Methods:**
- `New(cfg)`: Initializes agent with defaults, creates DB/runner/queue/proxyManager
- `Run(ctx)`: Main loop; polls every pollInterval, processes pending tasks
- `Stop()`: Cancels context, stops queue/proxyManager, closes DB

**Default Config Values:**
- DataDir: ~/.flowpilot
- MaxConcurrency: 10
- ProxyConcurrency: max(1, MaxConcurrency/2)
- PollInterval: 30s
- HealthCheckInterval: 300s
- MaxProxyFailures: 3
- MaxExecutionLogs: 250

### Initialization Flow

1. Create/migrate SQLite DB at ~/.flowpilot/tasks.db
2. Init encryption (crypto.InitKey)
3. Create browser Runner with screenshot dir
4. Create Queue with DB + runner
5. Create ProxyManager with health check interval
6. Wire all together: queue.SetProxyManager(pm), runner.SetPool(...), etc.

### Poll Loop

- Starts immediately (no initial delay)
- Every 30s: calls processPending()
  - Queries DB for TaskStatusPending
  - Submits each via queue.Submit()
  - Logs errors but continues
- Blocks on context cancellation

### Integration Points

- **DB**: Reads pending tasks, queue writes status updates
- **Runner**: Executes tasks via queue.RunTask()
- **Queue**: Manages concurrency, retry, proxy allocation
- **ProxyManager**: Rotates proxies, health checks

### Notable Patterns

1. **Staggered worker startup in queue**: Workers warm up 50ms apart
2. **Logging policy applied globally**: All tasks inherit agent's logging config
3. **Proxy concurrency auto-tuned**: ProxyConcurrency = MaxConcurrency/2 if not set
4. **Clean shutdown cascade**: cancel → queue.Stop() → proxyManager.Stop() → db.Close()

### Limitations

1. **Polling model vs push**: No event notifications; polling adds latency up to 30s
2. **No task prioritization in poll query**: All pending tasks fetched, relies on queue priority
3. **No rate limiting on poll**: If 1000 pending tasks, submits all in one poll cycle
4. **No graceful drain**: Stop() immediately cancels, doesn't wait for in-flight tasks
5. **Single poll interval**: Can't adjust per-task urgency
6. **No dead-letter observability**: Failed tasks not tracked separately
7. **No metrics export**: Agent metrics internal only

---

## 10. internal/scheduler/scheduler.go

### Purpose
Cron-based job scheduler. Polls database for due schedules, submits tasks, calculates next run time.

### Public API Surface

**Interfaces:**
- `TaskSubmitter`: SubmitScheduledTask(ctx, schedule) error
- `ScheduleDB`: ListDueSchedules(ctx, now), UpdateScheduleRun(ctx, id, lastRun, nextRun), GetRecordedFlow(ctx, id)

**Type: `Scheduler`**
- Fields: db, submitter, interval, stopCh, mu, running, logf

**Methods:**
- `New(db, submitter, interval)`: Creates scheduler
- `Start(ctx)`: Idempotent, spawns poll goroutine
- `Stop()`: Idempotent, signals stopCh
- `tick(ctx)`: Polls due schedules, submits, updates next run
- `loop(ctx)`: Main loop, ticks every interval or on context/stop

**Cron Parsing:**
- `ParseCron(expr) (*CronSchedule, error)`: Parses "min hour dom month dow" (5 fields)
- `CronSchedule.Next(from time.Time) time.Time`: Calculates next run from given time

**Cron Field Syntax:**
- `*`: All values
- `1,3,5`: Explicit list
- `1-5`: Range
- `*/5`: Step (every 5th value)
- `1-10/2`: Range with step

### Tick Logic

1. Query DB: ListDueSchedules(now) → schedules with nextRun <= now
2. For each schedule:
   - Parse CronSchedule from cron_expr
   - Call submitter.SubmitScheduledTask(schedule)
   - Calculate next run: cronSchedule.Next(now)
   - Update DB: UpdateScheduleRun(id, now, nextRun)
3. Errors logged but don't stop scheduler

### Cron Calculation

- Truncates to minute, advances by 1 minute
- Scans forward up to 525,960 iterations (1 year)
- Checks: month, day-of-month, day-of-week, hour, minute match
- Returns first matching datetime or now+24h (fallback)

### Notable Patterns

1. **Minute-level granularity**: No second-level scheduling
2. **Bit-vector dedup**: dedupSort() uses boolean array for O(n) dedup
3. **No leap-second handling**: Truncates to minute, no subsecond precision
4. **Fallback to +24h**: If no match found in 1 year, reschedules for next day
5. **Logging hook**: logf func for testability

### Limitations

1. **No timezone support**: All times in local TZ; no UTC option
2. **No schedule disable/pause**: Can't suspend schedule without deletion
3. **Simple cron syntax**: No `@daily`, `@weekly` shortcuts
4. **No max concurrency per schedule**: Multiple instances can run simultaneously
5. **No jitter**: Tasks run exactly at scheduled time; no randomization to spread load
6. **No retry on submit failure**: Failed submissions not retried
7. **Minute-level only**: Can't schedule sub-minute intervals
8. **No audit trail**: No tracking of actual vs scheduled run times
9. **No dynamic schedule updates**: Must stop/restart to change cron expr
10. **Database polling required**: No event-driven notifications

---

## Cross-Module Dependencies & Data Flow

```
Agent ──┬─→ Queue ──→ Browser.Runner ──→ BrowserPool
        │              ↓
        │           execute steps (steps.go)
        │              ↓
        │           conditions (conditions.go)
        │              ↓
        │           executor (interface)
        │
        ├─→ ProxyManager ←─ Queue (proxy limiting)
        │
        └─→ Database

Scheduler ──→ TaskSubmitter (app.go wires this)
              ↓
            Queue.SubmitScheduledTask()

Task Lifecycle in Queue:
  Submit → Queued → Running → (Success → Completed) or (Failure → Retrying → Queued → ...)
                              or (Cancel → Cancelled)
```

## Summary Table

| Module | Responsibility | Key Bottleneck | Missing Feature |
|--------|---|---|---|
| queue | Task scheduling, concurrency, retry | O(n) heap removal, no priority escalation | Dead-letter queue, task dependencies |
| browser | Step execution, automation | 30s hard timeout, no granular retry | Element-specific screenshots, network filtering |
| pool | Browser reuse, tab management | No prewarming, idle-only eviction | Memory-aware scaling, connection pooling |
| agent | Background polling service | 30s polling latency, no event push | Graceful drain, rate limiting |
| scheduler | Cron job scheduling | Minute granularity only | Timezone support, schedule disable |

