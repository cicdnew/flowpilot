# FlowPilot - Workers / Queue / Scheduling Codemap

## 1. QUEUE (`internal/queue/`)

### Files
- **queue.go** — Main task queue with priority heap, fixed worker pool, retry logic, and batch state management
- **priority_heap.go** — `taskHeap` implementing `container/heap.Interface` for priority-based scheduling

### Key Types & Functions

**queue.go:**
- `EventCallback func(event TaskEvent)` — Called on task status changes
- `Queue` struct — Core orchestrator: manages priority heap, paused heap, running map, worker pool, proxy concurrency limits, and async DB persistence
- `New(db, runner, maxConcurrency, onEvent) *Queue` — Creates queue with fixed worker pool (staggered warm-up), persistence worker goroutine
- `Submit(ctx, task) error` — Enqueues single task (dedup check, heap push, DB status update via persistence channel)
- `SubmitBatch(ctx, tasks) error` — Batch enqueue with single lock acquisition
- `Cancel(taskID) error` — Cancels running or pending task
- `PauseBatch(batchID)` / `ResumeBatch(batchID)` — Moves tasks between main heap and paused heap
- `Stop()` — Cancels all tasks, drains workers, closes persistence channel
- `RunningCount() int`, `Metrics() QueueMetrics` — Introspection
- `RecoverStaleTasks(ctx) error` — Post-crash recovery: resets running/queued tasks to pending
- `SetProxyManager(pm)`, `SetProxyConcurrencyLimit(limit)` — Proxy integration

**Internal flow:**
- `worker(id)` — Fixed pool goroutine: blocks on condition variable, dequeues via `dequeueRunnableLocked()`, calls `executeTask()`
- `dequeueRunnableLocked()` — Pops from priority heap, skips cancelled/paused tasks, respects proxy concurrency limit (defers over-limit items)
- `executeTask(ctx, task, countsProxy, autoProxy)` — Reserves proxy if needed, runs via `runner.RunTask()`, handles success/failure/retry
- `handleFailure()` — Persists step/network logs, increments retry, calculates exponential backoff (2^n capped at 60s)
- `handleSuccess()` — Persists result via `db.FinalizeTaskSuccess()`
- `scheduleRetry()` — Timer-based retry re-submission via `Submit()`
- `persistenceWorker()` — Async DB writer: batches `TaskStateChange` writes with configurable batch size and interval (default 100ms)

**priority_heap.go:**
- `heapItem` struct — Wraps `Task` with context, cancel func, addedAt timestamp, heap index
- `taskHeap` type — Max-heap by `task.Priority`; FIFO tiebreaker by `addedAt` for equal priority
- Implements `heap.Interface`: `Len()`, `Less()`, `Swap()`, `Push()`, `Pop()`, `peek()`

### Dependencies
- `flowpilot/internal/database` (TaskStateChange, BatchApplyTaskStateChanges, FinalizeTaskSuccess, FinalizeTaskFailure, InsertStepLogs, InsertNetworkLogs, ListStaleTasks)
- `flowpilot/internal/browser` (Runner.RunTask)
- `flowpilot/internal/models` (Task, TaskStatus, TaskEvent, QueueMetrics, ProxyConfig)
- `flowpilot/internal/proxy` (Manager, Reservation)

### How It Connects
- Created in `app.startup()` with DB, Runner, and concurrency config; event callback emits `task:event` via Wails runtime
- `app.StartTask()`, `StartAllPending()`, `CreateBatch()` methods submit tasks to the queue
- `app.GetQueueMetrics()` / `GetRunningCount()` expose queue state to frontend
- `Scheduler` submits tasks via `SubmitScheduledTask()` which calls `queue.Submit()`
- `Agent` polls DB for pending tasks and submits them to its own queue instance

---

## 2. SCHEDULER (`internal/scheduler/`)

### Files
- **scheduler.go** — Cron-based task scheduling engine with full 5-field cron parser

### Key Types & Functions

- `TaskSubmitter` interface — `SubmitScheduledTask(ctx, Schedule) error` — abstraction for task submission
- `ScheduleDB` interface — `ListDueSchedules(ctx, now)`, `UpdateScheduleRun(ctx, id, lastRun, nextRun)`, `GetRecordedFlow(ctx, id)`
- `Scheduler` struct — Polls DB for due schedules on interval, submits tasks, updates next run time
- `New(db, submitter, interval) *Scheduler`
- `Start(ctx)` / `Stop()` — Lifecycle management
- `loop(ctx)` / `tick(ctx)` — Main polling loop

**Cron parser:**
- `CronSchedule` struct — Parsed fields: minutes, hours, daysOfMonth, months, daysOfWeek
- `ParseCron(expr string) (*CronSchedule, error)` — Parses standard 5-field cron (supports `*`, ranges `-`, steps `/`, lists `,`)
- `CronSchedule.Next(from time.Time) time.Time` — Finds next matching minute (up to 1 year lookahead)
- Internal helpers: `parseField()`, `parsePart()`, `parseStep()`, `parseRange()`, `rangeSlice()`, `dedupSort()`

### Dependencies
- `flowpilot/internal/models` (Schedule, RecordedFlow)

### How It Connects
- `App` implements `TaskSubmitter` interface with `SubmitScheduledTask()` method
- Created in `app.startup()` with 30-second poll interval: `scheduler.New(a.db, a, 30*time.Second)`
- `app.CreateSchedule()` / `UpdateSchedule()` / `ToggleSchedule()` manage schedules in DB
- Frontend `SchedulePanel.svelte` manages CRUD via Wails bindings
- Due schedules trigger `app.SubmitScheduledTask()` which loads the flow, converts to task steps, and submits to the queue

---

## 3. BATCH (`internal/batch/`)

### Files
- **batch.go** — Batch task creation engine: generates multiple tasks from a recorded flow
- **csv.go** — URL list and CSV parsing utilities
- **variables.go** — Template variable substitution for batch task naming and step values
- **naming.go** — Default naming template and validation

### Key Types & Functions

**batch.go:**
- `Engine` struct — Holds DB reference for transactional batch creation
- `New(db) *Engine`
- `CreateBatchFromFlow(ctx, flow, input) (BatchGroup, []Task, error)` — Main batch creation:
  1. Validates input via `validation.ValidateBatchInput()`
  2. Converts flow steps to task steps via `models.FlowToTaskSteps()`
  3. Generates batch ID (UUID), applies naming template
  4. For each URL: applies variable substitution to name/steps/selectors, creates Task with proxy config, stores in DB (transactional)
  5. Creates BatchGroup record linking all tasks

**csv.go:**
- `ParseURLList(input string) ([]string, error)` — Newline-separated URL parsing
- `ParseCSVURLs(reader) ([]string, error)` — Extracts URLs from first CSV column

**variables.go:**
- `TemplateVars` struct — URL, Domain, Index, Name
- `ApplyTemplate(template, vars) string` — Replaces `{{url}}`, `{{domain}}`, `{{index}}`, `{{name}}`
- `ExtractDomain(rawURL) string` — URL hostname extraction

**naming.go:**
- `DefaultNameTemplate() string` — Returns `"Task {{index}} - {{domain}}"`
- `ValidateTemplate(template) bool` — Delegates to `models.ValidateBatchTemplate()`

### Dependencies
- `flowpilot/internal/database` (BeginTx, CreateTaskTx, CreateBatchGroupTx)
- `flowpilot/internal/models` (RecordedFlow, AdvancedBatchInput, BatchGroup, Task, FlowToTaskSteps, ProxyConfig)
- `flowpilot/internal/validation` (ValidateBatchInput)
- `github.com/google/uuid`

### How It Connects
- Created in `app.startup()` as `batch.New(db)`
- `app.CreateBatch()` / `CreateBatchFromFlow()` call `engine.CreateBatchFromFlow()` then submit tasks to queue
- `app.GetBatchProgress()` aggregates task statuses by batch ID for progress tracking
- Frontend `BatchCreateModal.svelte` (manual URL list) and `BatchFromFlow.svelte` (from recorded flow) create batches
- `BatchProgressPanel.svelte` shows real-time batch completion stats

---

## 4. AGENT (`internal/agent/` and `cmd/agent/`)

### Files
- **internal/agent/agent.go** — Headless background service that polls for pending tasks
- **cmd/agent/main.go** — CLI entry point for the standalone agent binary

### Key Types & Functions

**agent.go:**
- `Config` struct — DataDir, MaxConcurrency, ProxyConcurrency, PollInterval, HealthCheckInterval, MaxProxyFailures, logging flags
- `Agent` struct — Composes DB, Runner, Queue, ProxyManager
- `New(cfg) (*Agent, error)` — Initializes all components: creates data dir, inits crypto key, opens DB, creates headless browser runner, creates queue with proxy manager
- `Run(ctx) error` — Starts proxy health checks, polls for pending tasks on interval, submits them to queue
- `Stop()` — Shuts down queue, proxy manager, DB
- `processPending(ctx)` — Lists pending tasks from DB, submits each to queue

**cmd/agent/main.go:**
- CLI flags: `--data-dir`, `--concurrency` (default 10), `--poll` (30s), `--health-interval` (300s), `--max-failures` (3), `--version`
- Creates Agent, handles SIGINT/SIGTERM for graceful shutdown

### Dependencies
- `flowpilot/internal/agent` (Config, Agent)
- `flowpilot/internal/browser`, `flowpilot/internal/crypto`, `flowpilot/internal/database`, `flowpilot/internal/models`, `flowpilot/internal/proxy`, `flowpilot/internal/queue`

### How It Connects
- The agent is a standalone CLI binary (separate from the Wails desktop app)
- Shares the same internal packages as the desktop app but runs without GUI
- Designed for server/CI environments where tasks are created via external means (e.g., API, DB inserts) and the agent picks them up
- Forces headless mode (`runner.SetForceHeadless(true)`)
- Uses the same queue, proxy manager, and browser runner as the desktop app

---

## Architecture: How Workers Connect

```
                    ┌─────────────────────────────────┐
                    │         App (Wails GUI)          │
                    │  or Agent (Headless CLI)         │
                    └──────────┬──────────────────────-┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
        ┌──────────┐    ┌──────────┐    ┌──────────────┐
        │ Scheduler│    │  Queue   │    │ Batch Engine │
        │ (cron)   │───>│ (priority│<───│ (flow->tasks)│
        │          │    │  heap)   │    │              │
        └──────────┘    └────┬─────┘    └──────────────┘
                             │
                    ┌────────┼────────┐
                    │        │        │
                    ▼        ▼        ▼
              ┌─────────┐ ┌──────┐ ┌────────────┐
              │  Proxy   │ │Runner│ │   Local    │
              │ Manager  │ │      │ │   Proxy    │
              │(select/  │ │(CDP) │ │  Manager   │
              │ reserve) │ │      │ │ (SOCKS5)   │
              └─────────┘ └──────┘ └────────────┘
```

- **Scheduler** polls DB for due cron schedules, submits tasks to **Queue**
- **Batch Engine** creates tasks transactionally from flows, user submits to **Queue**
- **Queue** manages priority-ordered execution with fixed worker pool
- Workers call **Proxy Manager** to reserve proxies, which may route through **Local Proxy Manager**
- Workers call **Browser Runner** to execute tasks via chromedp
- **Agent** is an alternative entry point that polls DB directly instead of using Wails GUI
