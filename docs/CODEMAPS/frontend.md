# FlowPilot - Frontend Codemap

## Technology Stack
- **Framework:** Svelte (v3/v4 with TypeScript)
- **Build:** Vite + Svelte plugin
- **Desktop Bridge:** Wails v2 (Go<->JS bindings via `wailsjs/`)
- **Test:** Vitest
- **Styling:** Scoped CSS per component + global `style.css`

## Wails App Structure

### Go Side (`main.go`, `app.go`, `app_*.go`)

**main.go:**
- Wails entry point: creates `App`, configures window (1400x900, min 1024x768)
- Binds `app` to Wails runtime, handles SIGINT/SIGTERM graceful shutdown
- Uses embedded assets (production) or dev assets

**app.go:**
- `App` struct — Central orchestrator holding all internal components:
  - `db`, `runner`, `pool`, `queue`, `proxyManager`, `localProxyManager`, `scheduler`, `batchEngine`, `logExporter`
  - `activeRecorder`, `recordedSteps` for flow recording
- `startup(ctx)` — Initializes all subsystems in order: data dir -> crypto -> DB -> browser runner -> local proxy -> browser pool -> captcha solver -> proxy manager -> queue -> stale task recovery -> batch engine -> log exporter -> scheduler -> retention cleanup
- `shutdown(ctx)` / `cleanup()` — Stops scheduler, queue, pool, proxy managers, DB

**app_*.go files** expose Wails-bound methods:
- `app_tasks.go` — CreateTask, UpdateTask, DeleteTask, GetTask, ListTasks, ListTasksPaginated, StartTask, CancelTask, StartAllPending, GetTaskStats
- `app_batch.go` — CreateBatch, CreateBatchFromFlow, ParseBatchURLs, GetBatchProgress, ListBatchGroups, PauseBatch, ResumeBatch, RetryFailedBatch
- `app_proxy.go` — AddProxy, DeleteProxy, ListProxies, ListProxyCountryStats, CreateProxyRoutingPreset, etc.
- `app_captcha.go` — SaveCaptchaConfig, GetCaptchaConfig, ListCaptchaConfigs, TestCaptchaConfig, DeleteCaptchaConfig
- `app_recorder.go` — StartRecording, StopRecording, IsRecording, PlayRecordedFlow, CreateRecordedFlow, etc.
- `app_schedules.go` — CreateSchedule, UpdateSchedule, DeleteSchedule, ListSchedules, ToggleSchedule, GetSchedule
- `app_flows.go` — CRUD for RecordedFlow
- `app_vision.go` — CreateVisualBaseline, CompareVisual, etc.
- `app_export.go` — ExportResultsCSV, ExportResultsJSON, ExportTaskLogs, ExportBatchLogs
- `app_compliance.go` — GetAuditTrail, PurgeOldData

### Wails Bindings (`frontend/wailsjs/`)

**go/main/App.js** / **App.d.ts:**
- Auto-generated JS bridge exposing all 60+ App methods as async functions
- Each function calls `window['go']['main']['App'][methodName](args)`
- TypeScript definitions for all method signatures

**go/models.ts:**
- Auto-generated TypeScript classes mirroring Go structs: `ProxyConfig`, `Task`, `TaskStep`, `TaskResult`, `BatchGroup`, `BatchProgress`, `Schedule`, `CaptchaConfig`, `RecordedFlow`, `RecordedStep`, `DOMSnapshot`, `VisualBaseline`, `VisualDiff`, etc.
- Each class has `createFrom(source)` factory and `convertValues()` helper

**runtime/runtime.js:**
- Wails runtime: `EventsOn`, `EventsOff`, `EventsEmit`, `WindowSetTitle`, etc.

---

## Frontend Source (`frontend/src/`)

### Entry Points
- **main.ts** — Mounts `App.svelte` to `#app`, sets dark theme
- **App.svelte** — Root component with 6-tab layout (Tasks, Proxies, Recorder, Schedules, Visual, Settings)

### State Management (`lib/store.ts`)
Svelte writable/derived stores:
- `tasks` — Task list
- `proxies` — Proxy list
- `selectedTaskId`, `selectedTask` (derived)
- `activeTab` — Current tab
- `statusFilter`, `tagFilter` — Task filtering
- `recordedFlows`, `isRecording`, `recordingSteps`
- `webSocketLogs`, `schedules`, `captchaConfig`, `visualBaselines`
- Derived: `filteredTasks`, `allTags`, `taskStats`
- Functions: `updateTaskInStore()`, `replaceTaskInStore()`, `removeTaskFromStore()`

### Type Definitions (`lib/types.ts`)
TypeScript interfaces mirroring Go models:
- `Task`, `TaskStep`, `TaskResult`, `TaskStatus`, `TaskEvent`
- `Proxy`, `ProxyConfig`, `ProxyCountryStats`, `ProxyRoutingPreset`
- `RecordedFlow`, `RecordedStep`, `BatchGroup`, `BatchProgress`
- `Schedule`, `CaptchaConfig`, `VisualBaseline`, `VisualDiff`
- `DOMSnapshot`, `StepLog`, `NetworkLog`, `WebSocketLog`, `QueueMetrics`

### Components (`components/`)

| Component | Purpose | Key Wails Bindings Used |
|---|---|---|
| **Header.svelte** | App header with branding | — |
| **TaskToolbar.svelte** | Create task / batch buttons, filters | — |
| **TaskTable.svelte** | Paginated task list table | ListTasks, CancelTask, DeleteTask, StartTask |
| **TaskDetail.svelte** | Selected task detail sidebar | GetTask, GetAuditTrail |
| **CreateTaskModal.svelte** | Single task creation form | CreateTask |
| **BatchCreateModal.svelte** | Batch task creation from URL list | CreateBatch, ParseBatchURLs |
| **ProxyPanel.svelte** | Proxy pool management | ListProxies, AddProxy, DeleteProxy, ListProxyCountryStats |
| **RecorderPanel.svelte** | Flow recording controls | StartRecording, StopRecording, IsRecording |
| **FlowManager.svelte** | Saved flows list | ListRecordedFlows, DeleteRecordedFlow |
| **BatchFromFlow.svelte** | Create batch from recorded flow | CreateBatchFromFlow |
| **LogViewer.svelte** | Step/network log viewer | ListTaskEvents, ExportTaskLogs |
| **BatchProgressPanel.svelte** | Batch progress display | GetBatchProgress |
| **SchedulePanel.svelte** | Cron schedule management | ListSchedules, CreateSchedule, ToggleSchedule |
| **CaptchaSettings.svelte** | Captcha provider config | GetCaptchaConfig, SaveCaptchaConfig, TestCaptchaConfig |
| **VisualDiffViewer.svelte** | Visual regression comparison | ListVisualBaselines, CompareVisual, CreateVisualBaseline |

### App.svelte Architecture
- **Tab system:** 6 tabs (tasks, proxies, recorder, schedules, visual, settings)
- **Real-time updates:** Subscribes to `task:event` via `EventsOn('task:event')`, updates store on status changes
- **Pagination:** `ListTasksPaginated()` with page size 50, reactive to filter changes
- **Deferred refresh:** `scheduleRefresh()` debounces task list reloads (150ms)
- **Modals:** CreateTaskModal, BatchCreateModal, BatchFromFlow shown as overlays

---

## Data Flow: Frontend <-> Go Backend

```
Svelte Component
      │
      ▼
wailsjs/go/main/App.js  (auto-generated bridge)
      │  calls window['go']['main']['App'][method]
      ▼
Go App method (app_tasks.go, app_batch.go, etc.)
      │
      ▼
Internal packages (queue, proxy, browser, database, etc.)
      │
      ▼
SQLite DB / Chrome CDP / HTTP APIs
      │
      ▼
Result returned as JSON via Wails bridge
      │
      ▼
wailsjs/go/models.ts  (auto-generated typed classes)
      │
      ▼
Svelte store update / component render
```

**Real-time events:**
- Go emits: `wailsRuntime.EventsEmit(ctx, "task:event", event)`
- Frontend subscribes: `EventsOn('task:event', callback)`
- Events trigger store updates and debounced task list refresh

---

## Key Integration Points

1. **Task lifecycle:** Frontend creates tasks -> Queue executes -> events stream back -> UI updates
2. **Recording flow:** RecorderPanel starts Chrome -> captures steps -> FlowManager saves -> BatchFromFlow creates batch tasks
3. **Proxy management:** ProxyPanel CRUD -> Proxy Manager health checks -> Queue reserves proxies per task
4. **Scheduling:** SchedulePanel creates cron schedules -> Scheduler polls -> submits to Queue -> tasks execute
5. **Captcha:** CaptchaSettings saves config -> stored encrypted in DB -> loaded at startup -> injected into Runner
