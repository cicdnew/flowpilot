# Turso Integration Guide

> **Added:** 2026-03-30  
> **Applies to:** FlowPilot `feature/runtime-hardening-metrics-tests` and later

FlowPilot supports [Turso](https://turso.tech) as a drop-in distributed SQLite backend alongside the default local embedded SQLite file. Both modes use the same `libsql` Go driver (`github.com/tursodatabase/libsql-go`) and the exact same schema, migrations, and DAO layer.

---

## How It Works

The database layer detects the backend from the DSN format:

| URL format | Driver used | Mode |
|---|---|---|
| `/path/to/file.db` | `libsql` (local) | Embedded SQLite file |
| `file:/path/to/file.db` | `libsql` (local) | Embedded SQLite file |
| `libsql://mydb.turso.io` | `libsql` (remote) | Turso remote database |
| `libsql://mydb.turso.io` + `LocalPath` | `libsql` (embedded replica) | Turso + local replica |

Detection is done by `internal/database.DetectType(dsn)` — any URL beginning with `libsql://` activates Turso mode.

---

## Architecture

```
┌─────────────────────────────────┐
│          app.go startup          │
│  reads DATABASE_URL / env vars   │
│  builds DatabaseConfig           │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│   database.NewWithConfig(cfg)    │
│                                  │
│   DetectType(cfg.URL)            │
│   ├── DatabaseSQLite             │
│   │   └── newSQLiteDB()          │
│   │       sql.Open("libsql",     │
│   │           "file:<path>")     │
│   │       WAL PRAGMAs applied    │
│   │       write conn (1) +       │
│   │       read conn (4)          │
│   └── DatabaseTurso              │
│       ├── no LocalPath →         │
│       │   sql.Open("libsql",     │
│       │       "libsql://...      │
│       │       ?authToken=...")   │
│       └── LocalPath set →        │
│           NewEmbeddedReplica     │
│           Connector(local, url)  │
└─────────────────────────────────┘
             │
             ▼
  Same schema, migrations, DAOs
  for both modes — transparent to
  the rest of the application
```

---

## Quick Start

### 1. Create a Turso database

```bash
turso db create flowpilot
turso db show flowpilot   # copy the libsql URL
turso db tokens create flowpilot
```

### 2. Set environment variables

```bash
export DATABASE_URL=libsql://flowpilot-<org>.turso.io
export TURSO_AUTH_TOKEN=<your-token>
```

### 3. Run FlowPilot

```bash
# Desktop app
wails dev

# Headless agent
go run ./cmd/agent
```

FlowPilot will automatically detect the `libsql://` URL, open a Turso connection, run migrations, and start normally.

---

## Embedded Replica Mode (recommended for desktop use)

An embedded replica keeps a local SQLite copy of the Turso database. Reads are served locally; writes go to Turso and sync back. This gives low-latency reads and offline tolerance while preserving Turso as the source of truth.

```bash
export DATABASE_URL=libsql://flowpilot-<org>.turso.io
export TURSO_AUTH_TOKEN=<your-token>
export DATABASE_PATH=/home/user/.flowpilot/replica.db
```

When both `DATABASE_URL` and `DATABASE_PATH` are set, FlowPilot opens an embedded replica at `DATABASE_PATH` syncing to `DATABASE_URL`.

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | No | _(none)_ | Set to a `libsql://` URL to activate Turso mode. If unset, uses local SQLite. |
| `TURSO_AUTH_TOKEN` | No* | _(none)_ | Bearer auth token. Required for production Turso databases. |
| `DATABASE_PATH` | No | `<dataDir>/tasks.db` | Local file path. Without `DATABASE_URL`, used as the SQLite file. With `DATABASE_URL`, used as the local embedded replica path. |

> *`TURSO_AUTH_TOKEN` is optional when using a public or dev database, but required for all production Turso databases.

---

## Behavior Differences: Local vs Turso

| Feature | Local SQLite | Turso (remote) | Turso (embedded replica) |
|---|---|---|---|
| Driver | `libsql` | `libsql` | `libsql` (embedded) |
| Write pool | 1 connection | 4 connections | 1 connection |
| Read pool | 4 connections | shared | local reads |
| WAL PRAGMAs | ✅ Applied | ❌ Skipped | ✅ Applied locally |
| Offline reads | ✅ | ❌ | ✅ |
| Distributed | ❌ | ✅ | ✅ |
| Latency | Minimal | Network RTT | Minimal (reads) |

---

## Go API

### `internal/database/config.go`

```go
type DatabaseType int

const (
    DatabaseSQLite DatabaseType = iota
    DatabaseTurso
)

type DatabaseConfig struct {
    URL       string // local path or libsql:// URL
    AuthToken string // optional Turso auth token
    LocalPath string // optional local replica file path
}

// DetectType returns DatabaseTurso if dsn starts with "libsql://",
// otherwise DatabaseSQLite.
func DetectType(dsn string) DatabaseType
```

### `internal/database/sqlite.go`

```go
// New is the backward-compatible constructor for local SQLite.
// Equivalent to NewWithConfig(DatabaseConfig{URL: dbPath}).
func New(dbPath string) (*DB, error)

// NewWithConfig creates a DB from config.
// Dispatches to newSQLiteDB or newTursoDB based on DetectType(config.URL).
func NewWithConfig(config DatabaseConfig) (*DB, error)
```

---

## Schema & Migrations

The schema and all named migrations are identical for both modes. Migrations run automatically on startup via `db.migrate()`.

Schema statements are executed one-by-one (not in a single multi-statement batch) for compatibility with the Turso remote protocol.

Named migrations (additive `ALTER TABLE` statements) use `columnExists()` for idempotence — they can be applied repeatedly without error.

---

## Deploying the Headless Agent with Turso

The headless agent (`cmd/agent`) also respects the same environment variables:

```bash
DATABASE_URL=libsql://flowpilot-<org>.turso.io \
TURSO_AUTH_TOKEN=<token> \
DATABASE_PATH=/data/replica.db \
./flowpilot-agent \
  --concurrency 8 \
  --poll 5s \
  --health-interval 60s
```

Multiple agent instances can share the same Turso database safely — Turso handles concurrency at the server layer.

---

## Troubleshooting

### `error code = 2: no such table`
The schema migration failed. Check that `DATABASE_URL` and `TURSO_AUTH_TOKEN` are both set correctly before the process starts.

### `open turso database: ...dial tcp: ...`
Network connectivity issue. Confirm the Turso URL is reachable from the machine.

### Slow reads in remote (non-replica) mode
Switch to embedded replica mode by setting `DATABASE_PATH`. Reads will be served locally after initial sync.

### Auth token rejected
Turso tokens are scoped per database. Confirm the token was generated for the correct database with `turso db tokens create <db-name>`.

---

## See Also

- [database.md](CODEMAPS/database.md) — Schema, DAO, and migration reference
- [integrations.md](CODEMAPS/integrations.md) — All external integrations including Turso
- [Turso documentation](https://docs.turso.tech)
- [go-libsql driver](https://github.com/tursodatabase/go-libsql)
