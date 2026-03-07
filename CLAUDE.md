# Project: FlowPilot

## Stack
- **Backend**: Go 1.24, Wails v2 (desktop app framework)
- **Frontend**: Svelte 3 + TypeScript, Vite
- **Browser Automation**: chromedp (Chrome DevTools Protocol)
- **Database**: SQLite (go-sqlite3)
- **Testing**: `go test ./...` (backend), `vitest run` (frontend)

## Commands
- `wails dev` — run in development mode
- `wails build` — build production binary
- `go test ./...` — run Go tests
- `cd frontend && npm run check` — Svelte/TS type checking
- `cd frontend && npm run test` — run frontend tests

## Structure
- `main.go` — app entry point
- `app.go` — main App struct with Wails bindings
- `internal/` — backend packages (browser, crypto, database, models, proxy, queue, validation)
- `frontend/src/` — Svelte frontend
- `frontend/wailsjs/` — auto-generated Wails JS bindings
