# FlowPilot — Database Codemap

> **Freshness:** 2026-03-30  
> **Cross-refs:** [INDEX.md](INDEX.md) | [backend.md](backend.md) | [workers.md](workers.md)

## Overview

FlowPilot uses **SQLite / Turso** (via `tursodatabase/libsql-go`) for all persistence. The same driver supports both a local embedded file and a remote [Turso](https://turso.tech) distributed database. The `internal/database` package contains:
- `config.go` — `DatabaseConfig`, `DatabaseType`, and `DetectType()` URL classifier
- `sqlite.go` — connection management, schema creation, migrations, Turso/local branching
- `db_*.go` files — one DAO file per domain

The `internal/models` package defines all shared data structs (no ORM; raw `database/sql`).

---

## Configuration (`config.go`)

```go
// DatabaseType selects the backend driver.
type DatabaseType int
const (
    DatabaseSQLite DatabaseType = iota  // local file or :memory:
    DatabaseTurso                       // libsql:// remote URL
)

// DatabaseConfig is the single config value passed to NewWithConfig.
type DatabaseConfig struct {
    URL       string  // local file path OR libsql:// URL
    AuthToken string  // Turso auth token (optional)
    LocalPath string  // local replica path for embedded replica mode (optional)
}

// DetectType inspects the DSN prefix to pick the driver.
func DetectType(dsn string) DatabaseType
```

`DetectType` returns `DatabaseTurso` when the URL starts with `libsql://`; otherwise `DatabaseSQLite`.

---

## Connection & Initialization (`sqlite.go`)

```go
type DB struct {
    conn     *sql.DB  // writer
    readConn *sql.DB  // reader (== conn for Turso)
}

// Backward-compatible: opens a local SQLite file.
func New(dbPath string) (*DB, error)

// Config-based: supports local SQLite and Turso.
func NewWithConfig(config DatabaseConfig) (*DB, error)
```

### Local SQLite path
- Single shared pool (max 1 open) for both reads and writes via `libsql` driver
- Using two separate `*sql.DB` handles on the same file causes `database is locked`; a shared pool avoids this
- PRAGMAs applied: `synchronous=NORMAL`, `cache_size=-64000`, `mmap_size=268435456`, `temp_store=MEMORY`

### Turso / remote path
- Single shared `*sql.DB` for reads and writes (remote is stateless)
- Auth token appended as `?authToken=` query parameter
- PRAGMAs **not applied** (unsupported over remote connection)
- Embedded replica mode: `libsql.NewEmbeddedReplicaConnector(localPath, remoteURL)` — offline-capable local SQLite that syncs periodically to Turso

### Environment variables
| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(none)_ | `libsql://` URL — activates Turso mode |
| `TURSO_AUTH_TOKEN` | _(none)_ | Auth token passed to Turso |
| `DATABASE_PATH` | `<dataDir>/tasks.db` | Local file; doubles as embedded replica path when `DATABASE_URL` is set |

---

## Schema

### `tasks` table

```sql
CREATE TABLE tasks (
    id           TEXT PRIMARY KEY,   -- UUID
    name         TEXT NOT NULL,
    url          TEXT NOT NULL,
    status       TEXT NOT NULL,      -- pending|queued|running|done|failed|cancelled
    priority     INTEGER DEFAULT 0,
    tags         TEXT,               -- JSON array string
    steps        TEXT,               -- JSON array of TaskStep
    proxy_config TEXT,               -- JSON ProxyConfig
    logging_policy TEXT,             -- JSON TaskLoggingPolicy
    flow_id      TEXT,               -- FK → recorded_flows.id (nullable)
    batch_id     TEXT,               -- FK → batch_groups.id (nullable)
    schedule_id  TEXT,               -- FK → schedules.id (nullable)
    result       TEXT,               -- JSON TaskResult (nullable)
    error        TEXT,
    retry_count  INTEGER DEFAULT 0,
    max_retries  INTEGER DEFAULT 0,
    created_at   DATETIME,
    updated_at   DATETIME,
    started_at   DATETIME,
    completed_at DATETIME
)
```

### `recorded_flows` table

```sql
CREATE TABLE recorded_flows (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    steps       TEXT NOT NULL,     -- JSON []RecordedStep
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `dom_snapshots` table

```sql
CREATE TABLE dom_snapshots (
    id         TEXT PRIMARY KEY,
    flow_id    TEXT NOT NULL REFERENCES recorded_flows(id),
    step_index INTEGER NOT NULL,
    html       TEXT NOT NULL,
    screenshot BLOB,               -- PNG bytes
    created_at DATETIME
)
```

### `batch_groups` table

```sql
CREATE TABLE batch_groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    flow_id     TEXT,              -- FK → recorded_flows.id (nullable)
    total_tasks INTEGER DEFAULT 0,
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `schedules` table

```sql
CREATE TABLE schedules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    cron_expr   TEXT NOT NULL,
    task_input  TEXT NOT NULL,     -- JSON TaskInput template
    enabled     INTEGER DEFAULT 1,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `proxies` table

```sql
CREATE TABLE proxies (
    id          TEXT PRIMARY KEY,
    label       TEXT NOT NULL,
    host        TEXT NOT NULL,
    port        INTEGER NOT NULL,
    protocol    TEXT NOT NULL,     -- http|https|socks4|socks5
    username    TEXT,              -- encrypted (AES-256-GCM)
    password    TEXT,              -- encrypted (AES-256-GCM)
    status      TEXT DEFAULT 'active',
    last_used_at DATETIME,
    last_checked_at DATETIME,
    fail_count  INTEGER DEFAULT 0,
    tags        TEXT,              -- JSON array
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `proxy_routing_presets` table

```sql
CREATE TABLE proxy_routing_presets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    rules       TEXT NOT NULL,     -- JSON []RoutingRule
    strategy    TEXT NOT NULL,     -- round_robin|random|least_used|sticky
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `captcha_configs` table

```sql
CREATE TABLE captcha_configs (
    id          TEXT PRIMARY KEY,
    provider    TEXT NOT NULL,     -- anticaptcha|2captcha
    api_key     TEXT NOT NULL,     -- encrypted
    enabled     INTEGER DEFAULT 1,
    created_at  DATETIME,
    updated_at  DATETIME
)
```

### `task_events` table (Audit Trail)

```sql
CREATE TABLE task_events (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks(id),
    event_type TEXT NOT NULL,      -- created|started|completed|failed|cancelled|retried
    metadata   TEXT,               -- JSON additional info
    created_at DATETIME
)
```

### `step_logs` table

```sql
CREATE TABLE step_logs (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks(id),
    step_index INTEGER NOT NULL,
    action     TEXT NOT NULL,
    selector   TEXT,
    value      TEXT,
    status     TEXT NOT NULL,      -- ok|error|skipped
    error      TEXT,
    duration_ms INTEGER,
    created_at DATETIME
)
```

### `network_logs` table

```sql
CREATE TABLE network_logs (
    id            TEXT PRIMARY KEY,
    task_id       TEXT NOT NULL REFERENCES tasks(id),
    request_id    TEXT,
    url           TEXT NOT NULL,
    method        TEXT NOT NULL,
    status_code   INTEGER,
    request_headers  TEXT,         -- JSON
    response_headers TEXT,         -- JSON
    request_body  TEXT,
    response_body TEXT,
    duration_ms   INTEGER,
    created_at    DATETIME
)
```

### `websocket_logs` table

```sql
CREATE TABLE websocket_logs (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES tasks(id),
    request_id  TEXT,
    url         TEXT NOT NULL,
    event_type  TEXT NOT NULL,     -- opened|closed|message_sent|message_received
    payload     TEXT,
    created_at  DATETIME
)
```

### `visual_baselines` table

```sql
CREATE TABLE visual_baselines (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL REFERENCES tasks(id),
    step_index  INTEGER NOT NULL,
    image       BLOB NOT NULL,     -- PNG bytes
    created_at  DATETIME,
    updated_at  DATETIME,
    UNIQUE(task_id, step_index)
)
```

### `visual_diffs` table

```sql
CREATE TABLE visual_diffs (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL REFERENCES tasks(id),
    step_index   INTEGER NOT NULL,
    baseline_id  TEXT NOT NULL REFERENCES visual_baselines(id),
    diff_image   BLOB,             -- PNG diff overlay
    diff_percent REAL NOT NULL,    -- 0.0–100.0
    passed       INTEGER NOT NULL, -- 1=pass, 0=fail
    threshold    REAL NOT NULL,
    created_at   DATETIME
)
```

---

## Migrations (`sqlite.go` → `migrate()`)

> Schema statements are executed one-by-one (not as a single multi-statement batch) for compatibility with the libsql remote driver.

The `migrate()` function runs once at startup, using `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` patterns. It is idempotent — safe to run on existing databases.

Migration strategy: additive only (no destructive changes). Version is not tracked explicitly; the `IF NOT EXISTS` guards ensure safety.

---

## DAO Files

### `db_tasks.go`

| Function | Description |
|----------|-------------|
| `CreateTask(task)` | INSERT into tasks |
| `GetTask(id)` | SELECT by PK |
| `ListTasks(filter)` | SELECT with WHERE clause built from filter |
| `ListTasksPaginated(filter, page, size)` | + LIMIT/OFFSET |
| `UpdateTask(id, fields)` | UPDATE specific columns |
| `UpdateTaskStatus(id, status)` | UPDATE status + timestamps |
| `DeleteTask(id)` | DELETE by PK |
| `SetTaskResult(id, result)` | UPDATE result JSON |
| `IncrementRetryCount(id)` | UPDATE retry_count++ |

### `db_flows.go`

| Function | Description |
|----------|-------------|
| `SaveFlow(flow)` | INSERT OR REPLACE |
| `GetFlow(id)` | SELECT + parse steps JSON |
| `ListFlows()` | SELECT metadata (no steps) |
| `DeleteFlow(id)` | DELETE + cascade dom_snapshots |
| `SaveDOMSnapshot(snap)` | INSERT into dom_snapshots |
| `GetDOMSnapshot(flowID, stepIndex)` | SELECT by composite key |

### `db_batch.go`

| Function | Description |
|----------|-------------|
| `CreateBatchGroup(group)` | INSERT batch_groups |
| `GetBatchGroup(id)` | SELECT by PK |
| `ListBatchGroups()` | SELECT all |
| `GetBatchProgress(batchID)` | COUNT tasks by status |
| `UpdateBatchTaskCount(id, n)` | UPDATE total_tasks |

### `db_logs.go`

| Function | Description |
|----------|-------------|
| `InsertTaskEvent(event)` | INSERT into task_events |
| `ListAuditTrail(filter)` | SELECT task_events with filter |
| `InsertStepLog(log)` | INSERT into step_logs |
| `ListStepLogs(taskID)` | SELECT step_logs for task |
| `InsertNetworkLog(log)` | INSERT into network_logs |
| `ListNetworkLogs(taskID)` | SELECT network_logs for task |
| `InsertWebSocketLog(log)` | INSERT into websocket_logs |
| `ListWebSocketLogs(taskID)` | SELECT websocket_logs for task |

### `db_proxies.go`

| Function | Description |
|----------|-------------|
| `AddProxy(proxy)` | INSERT with encrypted credentials |
| `ListProxies()` | SELECT all (credentials decrypted in manager) |
| `GetProxy(id)` | SELECT by PK |
| `UpdateProxy(id, fields)` | UPDATE |
| `DeleteProxy(id)` | DELETE |
| `MarkProxyUsed(id)` | UPDATE last_used_at, fail_count reset |
| `MarkProxyFailed(id)` | UPDATE fail_count++ |
| `ListActiveSProxies()` | SELECT WHERE status='active' |

### `db_retention.go`

| Function | Description |
|----------|-------------|
| `PurgeOldData(days)` | DELETE tasks/logs/events older than N days |

Cascade: deletes tasks → cascades to step_logs, network_logs, websocket_logs, task_events, visual_diffs (via FK ON DELETE CASCADE).

### `db_vision.go`

| Function | Description |
|----------|-------------|
| `SaveBaseline(baseline)` | INSERT OR REPLACE into visual_baselines |
| `GetBaseline(taskID, stepIndex)` | SELECT by composite key |
| `SaveVisualDiff(diff)` | INSERT into visual_diffs |
| `ListVisualDiffs(taskID)` | SELECT for task |

---

## Models (`internal/models/`)

### `task.go`

```go
type Task struct {
    ID            string
    Name          string
    URL           string
    Status        TaskStatus        // "pending"|"queued"|"running"|"done"|"failed"|"cancelled"
    Priority      int
    Tags          []string
    Steps         []TaskStep
    ProxyConfig   *ProxyConfig
    LoggingPolicy TaskLoggingPolicy
    FlowID        string
    BatchID       string
    ScheduleID    string
    Result        *TaskResult
    Error         string
    RetryCount    int
    MaxRetries    int
    CreatedAt     time.Time
    // ... timestamps
}

type TaskStep struct {
    Action    string     // models.ActionClick etc.
    Selector  string
    Value     string
    Condition *ConditionConfig
    Options   map[string]any
}

// Action constants (50+):
const (
    ActionClick      = "click"
    ActionType       = "type"
    ActionNavigate   = "navigate"
    ActionScreenshot = "screenshot"
    ActionSolveCaptcha = "solve_captcha"
    // ...
)
```

### `flow.go`

```go
type RecordedFlow struct {
    ID          string
    Name        string
    Description string
    Steps       []RecordedStep
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type RecordedStep struct {
    Index     int
    Action    string
    Selector  string
    Value     string
    URL       string
    Timestamp int64
    Selectors []SelectorCandidate
}
```

### `proxy.go`

```go
type Proxy struct {
    ID       string
    Label    string
    Host     string
    Port     int
    Protocol ProxyProtocol   // "http"|"https"|"socks4"|"socks5"
    Username string          // decrypted in memory
    Password string          // decrypted in memory
    Status   ProxyStatus     // "active"|"inactive"|"failed"
    // ...
}

type ProxyRoutingPreset struct {
    ID       string
    Name     string
    Rules    []RoutingRule
    Strategy RotationStrategy  // "round_robin"|"random"|"least_used"|"sticky"
}
```

### `logs.go` — Error Classification

```go
func ClassifyError(err error) ErrorCode {
    // maps error strings to:
    // ErrorCodeTimeout, ErrorCodeNetworkError, ErrorCodeElementNotFound,
    // ErrorCodeCaptchaFailed, ErrorCodeProxyFailed, ErrorCodeUnknown
}
```

---

## See Also

- [turso-integration.md](../turso-integration.md) — Full Turso setup, env vars, and deployment guide
- [backend.md](backend.md) — App methods that call these DAOs
- [integrations.md](integrations.md) — credential encryption via crypto package
- [workers.md](workers.md) — queue reads/writes task status
