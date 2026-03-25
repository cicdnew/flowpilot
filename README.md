# FlowPilot

**Last Updated:** 2026-03-24

A desktop browser automation dashboard built with Go, Wails, and Svelte. Record, replay, and batch-execute web browser flows with proxy management, task queuing, and network capture.

## Quick Start

```bash
# Prerequisites: Go 1.24+, Node.js, Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Live development (hot reload)
wails dev

# Dev server (browser-based)
# Open http://localhost:34115

# Production build
wails build
```

## Architecture

```
FlowPilot
├── app.go                    # Wails app entry + lifecycle
├── app_*.go                  # Domain-specific Wails bindings (10 files)
├── main.go                   # Application bootstrap
├── cmd/agent/main.go         # Standalone headless CLI agent
└── internal/
    ├── agent/                # Headless execution mode
    ├── batch/                # Transactional batch task creation
    ├── browser/              # Chromedp browser pool + actions
    ├── captcha/              # AntiCaptcha / 2Captcha providers
    ├── crypto/               # AES-256-GCM encryption
    ├── database/             # SQLite persistence (9 domain files)
    ├── localproxy/           # SOCKS5 local proxy gateway
    ├── logs/                 # JSONL/CSV export + WebSocket streaming
    ├── models/               # Shared domain types
    ├── proxy/                # Remote proxy pool management
    ├── queue/                # Priority queue + worker pool
    ├── recorder/             # CDP interaction recording
    ├── scheduler/            # Cron-based task scheduling
    ├── validation/           # Input validation helpers
    └── vision/               # Visual diff comparison
```

### Tech Stack
- **Backend:** Go 1.24, Wails v2 (Go <-> JS bridge)
- **Frontend:** Svelte + TypeScript + Vite
- **Database:** SQLite (embedded)
- **Browser:** chromedp (Chrome DevTools Protocol)

## Documentation

- [Architecture Overview](docs/CODEMAPS/INDEX.md)
- [Backend](docs/CODEMAPS/backend.md) — App lifecycle, HTTP handlers, Wails bindings
- [Browser Automation](docs/CODEMAPS/browser.md) — Pool, actions, chromedp integration
- [Database](docs/CODEMAPS/database.md) — Schema, models, migrations
- [Integrations](docs/CODEMAPS/integrations.md) — Captcha, proxy, crypto, recorder
- [Workers](docs/CODEMAPS/workers.md) — Queue, scheduler, batch processing
- [Frontend](docs/CODEMAPS/frontend.md) — Svelte components, event streaming
- [Deletion Log](docs/DELETION_LOG.md) — Dead code cleanup history

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.
