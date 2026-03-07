# AGENTS.md — Web Automation Dashboard

Orients agentic coding tools working in this repository.

## Repository Overview

- **Backend:** Go 1.24, Wails v2 desktop framework, module `web-automation`
- **Frontend:** Svelte 3 + TypeScript (Vite), in `frontend/`
- **Automation:** chromedp (Chrome DevTools Protocol)
- **DB:** SQLite via go-sqlite3, encrypted proxy credentials (`internal/crypto`)
- **Key deps:** chromedp, wails/v2, google/uuid, golang.org/x/sync

Key directories:
| Path | Purpose |
|------|---------|
| `main.go`, `app.go` | Wails app entry + all frontend-bound API methods |
| `internal/` | Backend packages: browser, recorder, queue, database, logs, models, agent, batch, crypto, proxy, validation |
| `frontend/src/` | Svelte UI components, stores, types |
| `frontend/wailsjs/` | Auto-generated Wails bindings — **never hand-edit** |

## Build / Run Commands

```sh
# Desktop app
wails dev                     # dev mode with hot-reload
wails build                   # production binary

# Go tests (use -tags=dev to skip the frontend/dist embed requirement)
go test -tags=dev ./...                                # all tests
go test -tags=dev ./internal/queue -run TestQueueSubmit           # single test
go test -tags=dev ./internal/queue -run TestQueueSubmit/ShouldCancel  # subtest
go test -tags=dev -race -coverprofile=cover.out ./...  # with race + coverage

# Go lint / vet
go vet -tags=dev ./...        # only linter configured
gofmt -w <file.go>           # format a Go file

# Frontend (run from frontend/ directory)
npm run dev                   # Vite dev server
npm run build                 # production build
npm run check                 # svelte-check + TypeScript
npm run test                  # Vitest (runs once by default)
npm run test -- --run src/lib/store.test.ts   # single test file
npm run test -- --run -t "store updates"      # single test by name
```

## Code Style & Conventions

### Go

**Imports** — three groups separated by blank lines:
1. Standard library
2. External (`github.com/...`)
3. Internal (`web-automation/internal/...`)

**Naming:**
- Exported: `CamelCase`; unexported: `camelCase`
- Files: `snake_case.go`
- Step actions: use `models.ActionClick`, `models.ActionNavigate`, etc.

**Error handling:**
- Always wrap: `fmt.Errorf("<context>: %w", err)`
- Validate at API boundaries (`internal/validation`)
- Never panic; return errors
- Use `models.ClassifyError` for standardized error codes

**Concurrency:**
- Pass `context.Context` explicitly; respect cancellation/timeouts
- Guard shared state with mutexes (see `app.go` `recorderMu` pattern)
- Recorder must use chromedp ExecAllocator context, not plain `context.Context`

**DB schema:** migrations live in `internal/database/sqlite.go` `migrate()`. Do not change schema without updating that function.

**Wails bindings:** methods on `App` must be exported, return JSON-serializable types. `wailsjs/` is regenerated automatically.

### TypeScript / Svelte

**Imports** — external first, then local modules.

**Naming:** `camelCase` for variables/functions, `PascalCase` for types/components.

**State management:**
- All stores in `frontend/src/lib/store.ts`, update immutably (spread, map)
- Types in `frontend/src/lib/types.ts` — keep in sync with Go models
- Prefer derived stores for computed state

**No `console.log`** in committed code; use Wails runtime logging.

### General

- Small focused functions (<50 lines), avoid deep nesting (>4 levels)
- Do not mutate input objects in TS/JS
- Do not add comments unless explicitly asked
- No hardcoded secrets; use env vars

## Testing Conventions

**Go tests** live alongside packages (e.g., `internal/queue/queue_test.go`).
- Tests requiring encryption must call `crypto.ResetForTest()` + `crypto.InitKeyWithBytes()`
- Use `t.TempDir()` for DB paths, `t.Cleanup` for teardown
- Pattern: `setupTestDB(t)` / `setupTestApp(t)` helpers

**Frontend tests** use `@testing-library/svelte` + Vitest + jsdom.
- Setup file: `frontend/vitest.setup.ts`
- Tests in `frontend/src/lib/store.test.ts`

## Security & Compliance

- Proxy credentials encrypted at rest (`internal/crypto`); masked in frontend via `maskCredential()` in `app.go`
- Eval scripts blocked by default (`allowEval=false`); dangerous patterns rejected in `browser.go`
- Input validation for URLs, tags, proxies, steps (`internal/validation`)
- 90-day data retention: `PurgeOldRecords` runs daily via goroutine in `app.go`
- Audit trail via `task_events` table + `ListAuditTrail` API

## Architecture Notes

- **Recorder** (`internal/recorder/`): opens headless=false Chrome, injects JS capture script via CDP bindings, records click/type/navigate/tab-switch/select events. Wires `NetworkLogger` for HAR-like network capture and `Snapshotter` for DOM screenshots per step.
- **Queue** (`internal/queue/`): semaphore-based concurrency limiter with retry backoff, proxy rotation, and lifecycle event emission.
- **Agent** (`internal/agent/`): headless background service that polls pending tasks without GUI.
- **Batch** (`internal/batch/`): creates task groups from recorded flows with URL/template substitution.
- **CI:** `.github/workflows/ci.yml` runs `go vet`, `go test`, `npm run check`, `npm run test`.

## Agent TODOs (All Complete)

1. ~~Wire recorder browser launch~~ **DONE**
2. ~~Capture click/type/navigation events~~ **DONE**
3. ~~Multi-tab recording~~ **DONE** — `ActionTabSwitch`, `target.EventTargetInfoChanged`
4. ~~Network request logging~~ **DONE** — `NetworkLogger` in recorder
5. ~~DOM snapshots + selector generation~~ **DONE** — `Snapshotter` per step
6. ~~Playback engine~~ **DONE** — `PlayRecordedFlow` API
7. ~~Compliance + audit trail + 90-day retention~~ **DONE**
8. ~~Mask proxy credentials~~ **DONE** — `ListProxies` masks creds
9. ~~Background agent~~ **DONE** — `internal/agent`
10. ~~CI pipeline, pagination, UX~~ **DONE** — CI, `ListTasksPaginated`
