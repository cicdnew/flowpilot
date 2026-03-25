# FlowPilot — Codebase Index

> **Freshness:** 2026-03-24  
> **Module:** `flowpilot` (Go 1.24, Wails v2)

## What Is FlowPilot?

FlowPilot is a **desktop browser-automation platform** built with Go + Wails (webview desktop app) and a Svelte/TypeScript frontend. Users record browser interactions via Chrome DevTools Protocol, replay them as scheduled or batch tasks, manage proxies, solve CAPTCHAs, and view visual regression diffs — all from a native desktop UI.

---

## Package Map

```
flowpilot/                          (module root)
├── main.go                         Wails bootstrap, graceful shutdown
├── main_dev.go                     Dev-mode asset serving override
├── app.go                          App struct, startup/shutdown lifecycle, retention goroutine
├── app_tasks.go                    Task CRUD + start/cancel/retry APIs
├── app_flows.go                    RecordedFlow management + PlayRecordedFlow
├── app_batch.go                    Batch creation, pause/resume/retry
├── app_recorder.go                 StartRecording / StopRecording session
├── app_schedules.go                Schedule CRUD + enable/disable
├── app_proxy.go                    Proxy CRUD + routing preset management
├── app_captcha.go                  CaptchaConfig CRUD + test solver
├── app_compliance.go               ListAuditTrail, PurgeOldData
├── app_export.go                   ExportResults, ExportLogs (JSON/CSV/ZIP)
├── app_flow_io.go                  ImportFlow, ExportFlow (JSON round-trip)
├── app_vision.go                   CaptureBaseline, CompareBaseline
├── assets_dev.go / assets_prod.go  Frontend asset embedding (build-tag switched)
│
├── cmd/agent/main.go               Headless agent CLI entry point
│
└── internal/
    ├── agent/          Background polling agent (no GUI)
    ├── batch/          Batch task construction from flows + CSV
    ├── browser/        chromedp Runner, BrowserPool, 50+ step actions
    ├── captcha/        CAPTCHA solver interface + AntiCaptcha / 2Captcha
    ├── crypto/         AES-256-GCM encryption for credentials
    ├── database/       SQLite schema, migrations, per-domain DAOs
    ├── localproxy/     SOCKS5 gateway that routes through upstream proxies
    ├── logs/           StepLogger, NetworkLogger, WebSocketLogger, exporter
    ├── models/         Shared data types (task, flow, proxy, batch, …)
    ├── proxy/          Proxy pool with health checks & rotation strategies
    ├── queue/          Priority-heap work queue with semaphores + retry
    ├── recorder/       CDP-based interaction recorder, JS injector, snapshots
    ├── scheduler/      Cron expression parser + next-fire calculator
    ├── validation/     Input validation (URLs, proxies, steps, tags, …)
    └── vision/         Image pixel-diff for visual regression testing
```

---

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                     Wails Desktop Shell                          │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │             Svelte/TypeScript Frontend (Webview)           │  │
│  │  App.svelte → TaskTable, FlowManager, BatchCreateModal,    │  │
│  │               SchedulePanel, ProxyPanel, RecorderPanel,    │  │
│  │               VisualDiffViewer, LogViewer, …               │  │
│  └───────────────────────┬────────────────────────────────────┘  │
│                          │  Wails IPC (JS ↔ Go bindings)         │
│  ┌───────────────────────▼────────────────────────────────────┐  │
│  │                    App struct (app*.go)                    │  │
│  │  Tasks │ Flows │ Batch │ Recorder │ Schedules │ Proxy │    │  │
│  │  Captcha │ Compliance │ Export │ Vision │ FlowIO           │  │
│  └──┬──────────┬──────────┬──────────┬─────────┬─────────────┘  │
│     │          │          │          │         │                  │
└─────┼──────────┼──────────┼──────────┼─────────┼────────────────┘
      │          │          │          │         │
      ▼          ▼          ▼          ▼         ▼
 ┌─────────┐ ┌──────┐ ┌────────┐ ┌────────┐ ┌──────────┐
 │  Queue  │ │  DB  │ │Recorder│ │Scheduler│ │  Proxy   │
 │(queue/) │ │(db/) │ │(rec/)  │ │(sched/)│ │(proxy/,  │
 │priority │ │SQLite│ │CDP+JS  │ │ cron   │ │localprxy)│
 │heap,    │ │WAL   │ │inject  │ └───┬────┘ └────┬─────┘
 │semaphore│ │DAO   │ │snap    │     │            │
 └────┬────┘ └──────┘ └────────┘     │            │
      │                              │            │
      ▼                              ▼            ▼
 ┌─────────┐                   ┌──────────┐ ┌─────────┐
 │ Browser │◄──────────────────│  Batch   │ │ CAPTCHA │
 │ Runner  │                   │(batch/)  │ │(captcha)│
 │(browser)│                   │CSV parse │ │AntiCap  │
 │chromedp │                   │template  │ │2Captcha │
 │50+steps │                   │subst.    │ └────┬────┘
 └────┬────┘                   └──────────┘      │
      │                                          │
      ▼                                          ▼
 ┌─────────┐                              ┌──────────┐
 │BrowserPool                             │  Crypto  │
 │(pool.go)│                              │AES-256GCM│
 │reuse    │                              │(crypto/) │
 │Chrome   │                              └──────────┘
 └─────────┘

       ┌──────────────────────────────────┐
       │       cmd/agent/main.go          │
       │  Headless CLI agent: polls tasks │
       │  via internal/agent/agent.go     │
       │  (no GUI, same queue+browser)    │
       └──────────────────────────────────┘
```

---

## Key Data Flow: Task Execution

```
User clicks "Run Task"
        │
        ▼
  App.StartTask()          ← app_tasks.go
        │
        ▼
  queue.Submit(task)       ← internal/queue/queue.go
        │  priority-heap enqueue
        ▼
  Worker goroutine dequeues
        │  acquires semaphore (concurrency limit)
        │  rotates proxy via proxy.Manager
        ▼
  browser.Runner.Run()     ← internal/browser/browser.go
        │  launches chromedp context
        │  sets proxy via localproxy.Manager (SOCKS5)
        ▼
  executeStep() loop       ← internal/browser/steps.go
        │  50+ action types dispatched
        │  CAPTCHA → captcha.Solver
        │  Logs → StepLogger, NetworkLogger
        ▼
  Task result persisted    ← internal/database/db_tasks.go
        │
        ▼
  TaskLifecycleEvent emitted → Wails EventsEmit → Frontend store update
```

---

## Key Data Flow: Recording

```
User clicks "Start Recording"
        │
        ▼
  App.StartRecording()     ← app_recorder.go
        │
        ▼
  recorder.Recorder.Start() ← internal/recorder/recorder.go
        │  opens non-headless Chrome (ExecAllocator)
        │  injects JS capture script (injector.go)
        │  binds CDP event handler
        ▼
  User interacts in Chrome
        │  JS fires: click, type, navigate, select events
        │  CDP → Go callback → RecordedStep appended
        │  NetworkLogger captures HAR-like logs
        │  Snapshotter saves DOM + screenshot per step
        ▼
  App.StopRecording()
        │
        ▼
  Recorder.Stop() → RecordedFlow returned
        │
        ▼
  db.SaveFlow()    → persisted to SQLite
```

---

## Codemap Cross-References

| Topic | Codemap |
|-------|---------|
| App lifecycle, API handlers | [backend.md](backend.md) |
| Browser automation, step actions | [browser.md](browser.md) |
| Database schema, migrations, DAOs | [database.md](database.md) |
| Captcha, proxy, crypto, vision | [integrations.md](integrations.md) |
| Queue, scheduler, batch | [workers.md](workers.md) |
| Svelte frontend, stores, types | [frontend.md](frontend.md) |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop shell | Wails v2 (webview2 / WebKit) |
| Backend language | Go 1.24 |
| Browser automation | chromedp (Chrome DevTools Protocol) |
| Database | SQLite (go-sqlite3, WAL mode) |
| Encryption | AES-256-GCM (internal/crypto) |
| Frontend framework | Svelte 3 + TypeScript |
| Build tool | Vite |
| Testing | Go stdlib testing + Vitest |
| CI | GitHub Actions (.github/workflows/ci.yml) |
