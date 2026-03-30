# FlowPilot — Integrations Codemap

> **Freshness:** 2026-03-30

## 0. TURSO / LIBSQL DATABASE (`internal/database/`)

### Files
- **config.go** — `DatabaseType`, `DatabaseConfig`, `DetectType(dsn)`
- **sqlite.go** — `New()` / `NewWithConfig()` constructors with local + Turso branching

### Key Types & Functions

- `DatabaseType` — `DatabaseSQLite` (0) or `DatabaseTurso` (1)
- `DatabaseConfig` — `URL`, `AuthToken`, `LocalPath`
- `DetectType(dsn string) DatabaseType` — returns `DatabaseTurso` when DSN starts with `libsql://`
- `New(dbPath string) (*DB, error)` — backward-compatible local-file constructor
- `NewWithConfig(config DatabaseConfig) (*DB, error)` — unified constructor; dispatches to `newSQLiteDB` or `newTursoDB`
- `newSQLiteDB(dbPath string)` — opens via `libsql` driver with `file:` DSN; applies WAL PRAGMAs; separate read pool
- `newTursoDB(config)` — opens remote Turso connection or embedded replica; skips PRAGMAs; shares one pool for reads/writes

### Dependencies
- `github.com/tursodatabase/libsql-go` (aliased as `github.com/tursodatabase/go-libsql` via `replace` in go.mod)

### Environment Variables
| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | No | Set to `libsql://` to enable Turso mode |
| `TURSO_AUTH_TOKEN` | No | Bearer token for Turso auth |
| `DATABASE_PATH` | No | Local file path; used as embedded replica path when `DATABASE_URL` is set |

### How It Connects
- `app.go` startup reads env vars and calls `database.NewWithConfig(config)` before wiring any other service
- All DAOs (`db_*.go`) receive the single `*DB` instance; they call `db.conn` (write) or `db.Reader()` (read)
- Both modes share the same schema, migrations, and DAO layer — no branching elsewhere in the codebase

### See Also
- [database.md](database.md) — full schema and DAO reference
- [docs/turso-integration.md](../turso-integration.md) — deployment & ops guide

---

## 1. CAPTCHA (`internal/captcha/`)

### Files
- **captcha.go** — Factory and interface definition for captcha solvers
- **anticaptcha.go** — Anti-Captcha.com provider implementation
- **twocaptcha.go** — 2Captcha.com provider implementation

### Key Types & Functions

**captcha.go:**
- `Solver` interface — `Solve(ctx, CaptchaSolveRequest) (*CaptchaSolveResult, error)` and `Balance(ctx) (float64, error)`
- `NewSolver(config CaptchaConfig) (Solver, error)` — Factory that dispatches to TwoCaptcha or AntiCaptcha based on `config.Provider`

**anticaptcha.go:**
- `AntiCaptcha` struct — HTTP client for api.anti-captcha.com with poll-based result retrieval
- `NewAntiCaptcha(apiKey string) *AntiCaptcha`
- Methods: `Solve()`, `Balance()`, `createTask()`, `pollResult()`, `doPost()`
- Supports: RecaptchaV2, RecaptchaV3, HCaptcha, ImageToText captcha types
- Uses JSON API with `antiCaptchaRequest`/`antiCaptchaResponse` internal types

**twocaptcha.go:**
- `TwoCaptcha` struct — HTTP client for 2captcha.com using form-encoded API
- `NewTwoCaptcha(apiKey string) *TwoCaptcha`
- Methods: `Solve()`, `Balance()`, `submit()`, `poll()`
- Same captcha type support as AntiCaptcha; uses `OK|<token>` response format

### Dependencies
- `flowpilot/internal/models` (CaptchaSolveRequest, CaptchaSolveResult, CaptchaConfig, CaptchaProvider constants)
- stdlib: `context`, `fmt`, `net/http`, `encoding/json`, `time`, `strings`

### How It Connects
- App startup (`app.go`) loads active `CaptchaConfig` from DB, creates a `Solver` via `NewSolver()`, and injects it into `browser.Runner` via `runner.SetCaptchaSolver()`
- The runner uses the solver during task execution when it encounters captcha challenges
- Frontend's `CaptchaSettings.svelte` manages config CRUD via Wails bindings (`SaveCaptchaConfig`, `GetCaptchaConfig`, `TestCaptchaConfig`)

---

## 2. PROXY MANAGER (`internal/proxy/manager.go`)

### Files
- **manager.go** — Proxy pool management with selection strategies, health checks, and reservation system

### Key Types & Functions

- `Manager` struct — Core proxy pool manager holding DB ref, config, round-robin index, active reservations map, health check state
- `NewManager(db *database.DB, config ProxyPoolConfig) *Manager`
- `StartHealthChecks(ctx)` — Periodic goroutine that pings all proxies via HTTP
- `Stop()` — Idempotent shutdown
- `SelectProxy(geo string) (*Proxy, error)` — Simple proxy selection
- `SelectProxyWithFallback(geo, fallback) (*Proxy, fallbackUsed, direct, error)` — Selection with geo-filtering and fallback strategies (Strict/AnyHealthy/Direct)
- `ReserveProxy(geo)` / `ReserveProxyWithFallback(geo, fallback)` — Lease-based reservation that tracks active usage count
- `RecordUsage(proxyID, success)` — Updates proxy success/failure stats
- `CountryStats(proxies, activeLocalEndpoints)` — Aggregates per-country proxy statistics
- `Reservation` struct — Lease object with `Proxy()`, `ProxyConfig()`, `Complete(success)`, `Release()` methods

**Selection strategies:** RoundRobin, Random, LeastUsed, LowestLatency (from `models.RotationStrategy`)

### Dependencies
- `flowpilot/internal/database` (ListHealthyProxies, UpdateProxyHealth, IncrementProxyUsage, ListProxies)
- `flowpilot/internal/models` (Proxy, ProxyPoolConfig, ProxyConfig, ProxyStatus, RotationStrategy, ProxyRoutingFallback, ProxyCountryStats)

### How It Connects
- Created in `app.startup()` and `agent.New()`, injected into `Queue` via `queue.SetProxyManager()`
- Queue's `executeTask()` calls `pm.ReserveProxyWithFallback()` to get a proxy for each task
- Reservation is completed after task execution (success/failure tracked)
- `localproxy.Manager` uses `ProxyConfig` from proxy manager to create local SOCKS5 endpoints
- Health checks run in background goroutine, update DB proxy status

---

## 3. LOCAL PROXY (`internal/localproxy/`)

### Files
- **manager.go** — Local SOCKS5 proxy gateway that wraps upstream proxies
- **socks5.go** — SOCKS5 protocol implementation with HTTP CONNECT tunneling

### Key Types & Functions

**manager.go:**
- `Manager` struct — Manages per-upstream local SOCKS5 endpoints with idle timeout reaping
- `NewManager(idleTimeout) *Manager`
- `Endpoint(cfg ProxyConfig) (ProxyConfig, error)` — Returns/creates local SOCKS5 endpoint for an upstream proxy (deduplicates by upstream key)
- `Stop()` — Closes all listeners and waits for connections to drain
- `Stats() LocalProxyGatewayStats` — Returns active endpoint count, creation/reuse/failure stats
- `EndpointStatsByProxy(proxies) map[string]int` — Maps proxy IDs to active local connection counts
- `EndpointAddr(cfg) string` — Returns local address for an upstream config
- Internal: `reaper()` goroutine prunes idle endpoints; `serve()` accepts connections per endpoint

**socks5.go:**
- `handleSOCKS5Client(client, upstream, localUser, localPass)` — Main connection handler: handshake -> CONNECT -> relay
- `performSOCKS5Handshake(conn, user, pass)` — SOCKS5 auth negotiation (no-auth or user/pass)
- `readSOCKS5ConnectRequest(conn)` — Parses CONNECT target (IPv4/IPv6/domain)
- `dialViaUpstream(_, upstream, target)` — Dials through upstream (SOCKS5 via `golang.org/x/net/proxy`, HTTP/HTTPS via CONNECT)

### Dependencies
- `flowpilot/internal/models` (ProxyConfig, Proxy protocol constants, LocalProxyGatewayStats)
- `golang.org/x/net/proxy` — For upstream SOCKS5 dialing
- stdlib: `crypto/rand`, `net`, `crypto/tls`

### How It Connects
- Created in `app.startup()` as `localproxy.NewManager(5 * time.Minute)`, injected into `browser.Runner`
- When the browser runner needs a proxy, it calls `localProxyManager.Endpoint(upstreamConfig)` to get a local SOCKS5 address
- Chrome connects to the local SOCKS5 endpoint, which tunnels to the upstream proxy
- This isolates browser proxy credentials and provides connection pooling per upstream

---

## 4. CRYPTO (`internal/crypto/`)

### Files
- **crypto.go** — AES-256-GCM encryption for sensitive data (proxy passwords, API keys)

### Key Types & Functions

- `InitKey(dataDir string) error` — Loads or generates 32-byte AES key stored in `.encryption_key` file (singleton via sync.Once)
- `InitKeyWithBytes(key []byte) error` — Direct key injection for testing
- `Encrypt(plaintext string) (string, error)` — AES-256-GCM encryption, returns base64-encoded ciphertext
- `Decrypt(encoded string) (string, error)` — Decryption with legacy migration (non-base64 values returned as-is)
- `ResetForTest()` — Resets global state for test isolation

### Dependencies
- stdlib only: `crypto/aes`, `crypto/cipher`, `crypto/rand`, `encoding/base64`, `os`, `sync`

### How It Connects
- Called during startup (`crypto.InitKey(dataDir)`) before DB operations
- Used by `database` package to encrypt/decrypt proxy passwords and captcha API keys before storing in SQLite
- Transparent to other components — the DB layer handles encryption/decryption at the persistence boundary

---

## 5. RECORDER (`internal/recorder/`)

### Files
- **recorder.go** — Main recorder that opens a visible Chrome session and captures user interactions via CDP
- **cdp.go** — CDPClient interface abstracting chromedp for testability
- **injector.go** — JavaScript capture script and binding payload parsing
- **snapshot.go** — DOM HTML + screenshot capture for recorded steps
- **selector.go** — Selector candidate ranking utilities

### Key Types & Functions

**recorder.go:**
- `EventHandler func(step RecordedStep)` — Callback for new recorded steps
- `Recorder` struct — Manages chromedp lifecycle, event listeners, network/WS logging, snapshotting
- `New(parentCtx, flowID, handler) *Recorder`
- `Start(url string) error` — Launches headless=false Chrome, enables CDP domains, injects capture script, navigates to URL
- `Stop()` — Graceful browser shutdown
- `RecordStep(action, selector, value)` — Creates a RecordedStep with optional snapshot, calls handler
- `BrowserCtx()`, `FlowID()`, `NetworkLogs()`, `WebSocketLogs()`, `SetWSCallback()`, `SetSnapshotter()`

**cdp.go:**
- `CDPClient` interface — `Run(ctx, actions...)` and `ListenTarget(ctx, fn)` — enables mocking in tests
- `chromeCDPClient` struct — Production implementation wrapping chromedp

**injector.go:**
- `captureScript` const — JavaScript injected into pages to capture click/type/select events via DOM event listeners with debounced input handling
- `parseBindingPayload(raw) (action, selector, value, error)` — Parses JSON from CDP binding call
- `mapJSAction(jsAction) (StepAction, bool)` — Maps JS action names to `models.StepAction` constants

**snapshot.go:**
- `Snapshotter` struct — Captures DOM HTML and full-page screenshots via CDP
- `NewSnapshotter(outputDir) (*Snapshotter, error)`
- `CaptureSnapshot(ctx, flowID, stepIndex) (DOMSnapshot, error)` — Returns DOMSnapshot with HTML, screenshot path, URL

**selector.go:**
- `RankSelectors(candidates []SelectorCandidate) []SelectorCandidate` — Sorts by stability score descending
- `BestSelector(candidates) string` — Returns highest-ranked selector

### Dependencies
- `flowpilot/internal/models` (RecordedStep, StepAction constants, DOMSnapshot, SelectorCandidate, NetworkLog, WebSocketLog)
- `flowpilot/internal/logs` (NetworkLogger, WebSocketLogger)
- `github.com/chromedp/chromedp`, `github.com/chromedp/cdproto/*` (network, page, runtime, target)

### How It Connects
- Created in `app.startStopRecording()` methods, managed via `app.activeRecorder`
- Frontend `RecorderPanel.svelte` triggers `StartRecording(url)` / `StopRecording()` via Wails bindings
- Recorded steps are stored in `app.recordedSteps`, saved as `RecordedFlow` via `CreateRecordedFlow()`
- `FlowManager.svelte` lists saved flows; `BatchFromFlow.svelte` creates batch tasks from flows
- The `Snapshotter` is optionally attached; snapshots are stored as files with DB references
