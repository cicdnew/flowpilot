# FlowPilot — Implementation Plans Index

This directory contains detailed, actionable implementation plans for FlowPilot's next phase of development. Each plan references exact file paths, function names, and includes Go/TypeScript code snippets.

---

## Plans Overview

| # | Plan | Focus | Effort | Risk |
|---|------|-------|--------|------|
| [01](./01-scalability.md) | **Scalability** | 1000+ concurrent tasks, O(log n) cancel, virtual scroll | 3–4 weeks | Medium |
| [02](./02-auth-rls.md) | **Auth & RLS** | Multi-user auth, row-level security, roles | 5–12 weeks | High |
| [03](./03-distributed.md) | **Distributed** | Multi-node worker cluster, remote browsers | 7–10 weeks | Very High |
| [04](./04-plugin-system.md) | **Plugin System** | Custom step actions, WASM/gRPC extensions, marketplace | 6–8 weeks | High |
| [05](./05-observability.md) | **Observability** | Structured logs, in-process metrics, alerting | 2–3 weeks | Low |
| [06](./06-frontend-polish.md) | **Frontend Polish** | Virtual scroll, bulk ops, log viewer, cron builder | 3–4 weeks | Low |

---

## Priority Matrix

```
                    HIGH VALUE
                         ▲
                         │
  06 Frontend Polish ────┼──── 01 Scalability
  05 Observability        │
                         │
  LOW EFFORT ────────────┼──────────────── HIGH EFFORT
                         │
                         │    04 Plugin System
                         │    02 Auth & RLS
                         │
                         │         03 Distributed
                         ▼
                    LOW VALUE
```

### Recommended Execution Order

#### Immediate (< 1 month) — Low effort, high payoff
1. **[01 Scalability](./01-scalability.md)** — Phase 1 quick wins (O(n) cancel fix, pagination, SQLite tuning) can be completed in 2 days with zero risk. These are pure bug fixes.
2. **[06 Frontend Polish](./06-frontend-polish.md)** — Virtual scrolling and bulk operations directly improve daily usability. Phases 1–2 are self-contained.
3. **[05 Observability](./05-observability.md)** — Phase 1 (structured logging) provides immediate debugging value and unlocks all later phases.

#### Short-term (1–3 months) — Medium effort, clear value
4. **[01 Scalability](./01-scalability.md)** — Phases 2–4 (queue optimization, archival, pool pre-warming)
5. **[05 Observability](./05-observability.md)** — Phases 2–4 (metrics registry, alerting, log search UI)
6. **[06 Frontend Polish](./06-frontend-polish.md)** — Phases 3–9 (visual diff viewer, cron builder, keyboard shortcuts)

#### Medium-term (3–6 months) — High effort, architectural change
7. **[04 Plugin System](./04-plugin-system.md)** — Phases 0–3 (WASM step plugins, lifecycle, SDK). Unlocks community contributions.
8. **[02 Auth & RLS](./02-auth-rls.md)** — Phase 1 (user model + login screen). Required before any SaaS/team deployment.

#### Long-term (6+ months) — Very high effort, major architectural shift
9. **[03 Distributed](./03-distributed.md)** — Requires Auth (02) and Observability (05) as prerequisites.
10. **[02 Auth & RLS](./02-auth-rls.md)** — Phases 2–6 (RBAC, GDPR export, JWT API, OIDC)
11. **[04 Plugin System](./04-plugin-system.md)** — Phases 4–7 (marketplace, sandboxing, audit)

---

## Dependency Graph

```
01 Scalability ──────────────────────────────► 03 Distributed
                                                      ▲
05 Observability ────────────────────────────────────┤
      ▲                                               │
      └──── required for production alerting         │
                                                      │
02 Auth & RLS ───────────────────────────────────────┘
      ▲
      └──── required for multi-user / SaaS

04 Plugin System ────► (independent, but easier after 01 Scalability)

06 Frontend Polish ──► (independent, immediate wins)
```

**Hard prerequisites:**
- **03 Distributed** requires **02 Auth** (user isolation per node) and **05 Observability** (distributed health checks)
- **02 Auth Phase 5** (JWT API for agent) requires **05 Observability** (request logging)
- **04 Plugin Phase 5** (marketplace) benefits from **02 Auth** (plugin ownership)

---

## Current Architecture Snapshot

```
┌─────────────────────────────────────────────────────────────┐
│                   FlowPilot Desktop App                     │
│                                                             │
│  Wails v2 (Go + Svelte)                                     │
│  ┌────────────────────────┐  ┌──────────────────────────┐  │
│  │   Frontend (Svelte 3)  │  │   Backend (Go 1.24)      │  │
│  │                        │◄►│                          │  │
│  │  App.svelte            │  │  app.go (App struct)     │  │
│  │  TaskTable.svelte      │  │  app_tasks.go            │  │
│  │  FlowManager.svelte    │  │  app_flows.go            │  │
│  │  SchedulePanel.svelte  │  │  app_schedules.go        │  │
│  │  ProxyPanel.svelte     │  │  app_batch.go            │  │
│  │  BatchCreateModal.svelte│  │  app_recorder.go        │  │
│  │  LogViewer.svelte      │  │  app_proxy.go            │  │
│  │  VisualDiffViewer.svelte│  │  app_vision.go          │  │
│  └────────────────────────┘  │  app_captcha.go          │  │
│                               │  app_export.go           │  │
│                               │  app_compliance.go       │  │
│                               └──────────┬───────────────┘  │
│                                          │                   │
│              ┌───────────────────────────┼───────────┐      │
│              │                           │           │      │
│       ┌──────▼──────┐  ┌────────────┐  ┌▼─────────┐ │      │
│       │   Queue     │  │  Browser   │  │ Database  │ │      │
│       │ (priority   │  │  Pool      │  │ (SQLite   │ │      │
│       │  heap,      │  │ (chromedp  │  │  WAL,     │ │      │
│       │  200 workers│  │  100 procs)│  │  1 writer)│ │      │
│       └──────┬──────┘  └─────┬──────┘  └──────────┘ │      │
│              │               │                        │      │
│       ┌──────▼───────────────▼──────┐                │      │
│       │        Proxy Manager        │                │      │
│       │  (round-robin, health check)│                │      │
│       └─────────────────────────────┘                │      │
│              internal/scheduler/ (cron)               │      │
│              internal/recorder/ (CDP)                 │      │
│              internal/batch/   (bulk tasks)           │      │
│              internal/agent/   (headless mode)        │      │
└─────────────────────────────────────────────────────────────┘
```

---

## Key Metrics to Track

These metrics should be instrumented as part of [Plan 05 — Observability](./05-observability.md) and used to validate [Plan 01 — Scalability](./01-scalability.md):

| Metric | Current Baseline | Target |
|--------|-----------------|--------|
| Max concurrent tasks | ~200 | 1,000+ |
| Task cancel latency (10K queue) | O(n) | O(log n) < 1ms |
| Frontend frame time at 1K rows | > 100ms (jank) | < 16ms |
| SQLite write throughput | ~200 updates/s | 500+ updates/s |
| Memory at 1K tasks | ~800 MB | < 2 GB |
| p95 step duration visibility | None | < 200ms query |
| Alert-to-notification latency | None | < 60s |
| Schedule run history visibility | None | Last 10 runs |

---

## Plan File Conventions

Each plan follows this structure:

```
# Implementation Plan: [Name]
## Overview           — 2–3 sentence summary
## Requirements       — Functional + non-functional checklist
## Current Bottlenecks / Limitations  — Exact file:function references
## Architecture Changes  — Before/after diagrams
## Implementation Steps  — Phased, with complexity and risk per step
## Testing Strategy   — Unit, integration, load tests
## Risks & Mitigations  — Table format
## Success Criteria   — Checkboxes
```

All code snippets use the actual package names (`flowpilot/internal/...`) and reference real structs/functions from the codebase.

---

## Quick Reference: Key Files by Plan

| Plan | Primary Files Modified |
|------|----------------------|
| 01 Scalability | `internal/queue/queue.go`, `internal/queue/priority_heap.go`, `internal/database/db_tasks.go`, `internal/database/sqlite.go`, `internal/browser/pool.go`, `frontend/src/components/TaskTable.svelte` |
| 02 Auth & RLS | `app.go`, `internal/database/sqlite.go`, all `internal/database/db_*.go`, `frontend/src/App.svelte`, new `internal/auth/`, new `frontend/src/components/LoginScreen.svelte` |
| 03 Distributed | `internal/agent/agent.go`, `internal/queue/queue.go`, `internal/proxy/manager.go`, new `internal/coordinator/`, new `cmd/worker/` |
| 04 Plugin System | `internal/browser/steps.go`, `internal/validation/validate.go`, `internal/recorder/recorder.go`, new `internal/plugin/`, new `frontend/src/components/PluginPanel.svelte` |
| 05 Observability | new `internal/observability/`, `internal/browser/browser.go`, `internal/queue/queue.go`, `app.go`, new `app_alerts.go`, `frontend/src/components/LogViewer.svelte`, new `frontend/src/components/MetricsDashboard.svelte` |
| 06 Frontend Polish | `frontend/src/components/TaskTable.svelte`, `frontend/src/components/LogViewer.svelte`, `frontend/src/components/VisualDiffViewer.svelte`, `frontend/src/components/SchedulePanel.svelte`, `frontend/src/lib/store.ts`, new `frontend/src/components/CronBuilder.svelte`, new `frontend/src/components/ToastContainer.svelte` |
