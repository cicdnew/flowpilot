# FlowPilot Architecture Summary

## Overview
Comprehensive analysis of 14 core internal modules covering models, proxy management, recording, batch processing, validation, and cryptography.

---

## 1. internal/models/task.go

**Purpose**: Defines the core Task, TaskStep, and related execution models.

**Key Data Structures**:
- `TaskStatus`: Enum (pending, queued, running, completed, failed, cancelled, retrying)
- `TaskPriority`: Int enum (Low=1, Normal=5, High=10)
- `StepAction`: 60+ action types including click, type, navigate, extract, loops, conditions, variables, etc.
- `TaskStep`: Single browser action with selector, value, timeout, condition, label, jump targets
- `Task`: Main execution unit with ID, steps, proxy config, priority, status, retry info, tags, batch/flow IDs
- `TaskResult`: Holds success flag, extracted data, screenshots, logs (execution + network + steps)
- `ProxyConfig`: Server, protocol, username, password, geo, fallback mode
- `TaskLoggingPolicy`: Optional logging controls (capture steps/network/screenshots, max logs)

**Public API**:
- Task & TaskStep structs (JSON serializable)
- `ExecutableStepActions()`, `ControlFlowStepActions()`, `SupportedStepActions()` helper functions
- Status & priority constants

**Patterns**:
- 60+ step actions organized into executable vs control-flow categories
- Comprehensive result tracking with log aggregation
- Proxy routing with fallback strategies (strict/any_healthy/direct)

**Limitations**:
- No validation logic here (delegated to `internal/validation`)
- TaskResult has a LogLimit field marked as json:"-" (not serialized)
- No built-in immutability guarantees

**TODOs/Bottlenecks**:
- Step actions list is hardcoded and long (could be generated or split)
- No action metadata (required fields, supported payloads)

---

## 2. internal/models/flow.go

**Purpose**: Models for recording captured user interactions into reusable flows.

**Key Data Structures**:
- `RecordedFlow`: ID, name, description, steps, originURL, createdAt, updatedAt
- `RecordedStep`: Index, action, selector, value, timeout, snapshotID, timestamp, selectorCandidates
- `SelectorCandidate`: Selector string, strategy (data-testid, id, role, css, xpath), score (1-100 stability)
- `DOMSnapshot`: ID, flowID, stepIndex, HTML, screenshotPath, URL, capturedAt

**Public API**:
- `RecordedStep.ToTaskStep()`: Converts recorded step to executable step
- `FlowToTaskSteps()`: Batch converts all flow steps to task steps
- SelectorType enum (5 strategies)

**Patterns**:
- Snapshot per step with alternative selectors ranked by stability
- Minimal selector mutation (no value/timeout transformation)

**Limitations**:
- No flow versioning
- DOMSnapshot stores raw HTML (potentially large)
- Selector candidates are optional, not populated by default

**TODOs/Bottlenecks**:
- No selector validation at capture time
- No detection of stale/broken selectors post-capture
- Missing selector update/repair mechanisms

---

## 3. internal/models/batch.go

**Purpose**: Batch task creation from a single flow with URL/template substitution.

**Key Data Structures**:
- `AdvancedBatchInput`: FlowID, URLs, namingTemplate, priority, proxy, tags, proxyCountry, proxyFallback, autoStart, headless
- `BatchGroup`: ID, flowID, taskIDs, total, name, createdAt
- `BatchProgress`: Aggregate counts (pending, queued, running, completed, failed, cancelled)
- Template variables: `{{url}}`, `{{domain}}`, `{{index}}`, `{{name}}`

**Public API**:
- `BatchHeadless()`: Returns effective headless mode (nil defaults to true)
- `ValidateBatchTemplate()`: Validates template syntax
- MaxBatchSize constant = 10,000

**Patterns**:
- Simple string-based template substitution
- Headless backward compatibility via nil check

**Limitations**:
- No complex templating (no conditionals, functions, loops)
- Template parsing is basic string scanning
- No template escaping/sanitization
- Batch name derived from flow name, no customization

**TODOs/Bottlenecks**:
- Template variable whitelist is hardcoded
- No domain extraction built-in (delegated to batch engine)
- Missing template variable history/audit

---

## 4. internal/models/proxy.go

**Purpose**: Proxy pool management, selection strategies, and health tracking.

**Key Data Structures**:
- `ProxyStatus`: Enum (healthy, unhealthy, unknown)
- `ProxyProtocol`: Enum (http, https, socks5)
- `Proxy`: Full proxy record with ID, server, protocol, credentials, geo, status, latency, successRate, totalUsed, lastChecked, localEndpoint info
- `ProxyCountryStats`: Aggregated stats per country (total, healthy, activeReservations, totalUsed, fallbackAssignments, activeLocalEndpoints)
- `ProxyRoutingPreset`: Reusable routing profile with randomByCountry, country, fallback strategy
- `LocalProxyGatewayStats`: Runtime gateway health (activeEndpoints, creations, reuses, authFailures, upstreamFailures)
- `RotationStrategy`: Enum (round_robin, random, least_used, lowest_latency)

**Public API**:
- `Proxy.ToProxyConfig()`: Converts pool proxy to task-level config
- ProxyPoolConfig struct for manager initialization

**Patterns**:
- Rich metadata (latency, success rate, usage counters)
- Country-based filtering with geo fallback
- Local proxy endpoint abstraction

**Limitations**:
- No proxy authentication audit/logs
- LastChecked is optional (may be nil)
- No explicit proxy group/pool isolation

**TODOs/Bottlenecks**:
- ProxyRoutingPreset defined but unclear if wired into selection logic
- No built-in proxy credential rotation
- Missing proxy pool capacity planning helpers

---

## 5. internal/models/schedule.go

**Purpose**: Scheduled task execution configuration.

**Key Data Structures**:
- `Schedule`: ID, name, cronExpr, flowID, URL, proxyConfig, priority, headless, tags, enabled, lastRunAt, nextRunAt, createdAt, updatedAt
- MaxSchedules constant = 100

**Public API**:
- Schedule struct (JSON serializable)

**Patterns**:
- Cron-based scheduling
- Per-schedule proxy + priority config
- Enabled flag for soft disable

**Limitations**:
- No timezone support (cron is UTC-only)
- No schedule versioning or change history
- Missing validation for cron expression syntax
- No retry/backoff configuration per schedule

**TODOs/Bottlenecks**:
- Cron expression parsing delegated elsewhere (likely app.go)
- nextRunAt calculation not in this model
- No schedule conflict detection (e.g., overlapping runs)

---

## 6. internal/models/vision.go

**Purpose**: Visual regression testing and screenshot comparison.

**Key Data Structures**:
- `VisualBaseline`: ID, name, taskID, URL, screenshotPath, width, height, createdAt
- `VisualDiff`: ID, baselineID, taskID, screenshotPath, diffImagePath, diffPercent, pixelCount, threshold, passed, width, height, createdAt
- `DiffRequest`: baselineID, taskID, threshold

**Public API**:
- VisualBaseline, VisualDiff, DiffRequest structs

**Patterns**:
- Screenshot-based comparison with pixel-level reporting
- Pass/fail determination via threshold

**Limitations**:
- No multi-resolution baseline support
- Diff calculation logic not in model (delegated to compute layer)
- Missing screenshot metadata (browser, viewport, color depth)
- No version tracking for baselines

**TODOs/Bottlenecks**:
- Very minimal implementation (no diffing algorithm)
- Threshold is per-diff, no default strategy
- Missing visual baseline deprecation/cleanup logic

---

## 7. internal/models/event.go

**Purpose**: Task lifecycle events and queue metrics.

**Key Data Structures**:
- `TaskLifecycleEvent`: ID, taskID, batchID, fromState, toState, error, timestamp
- `QueueMetrics`: Comprehensive queue snapshot with:
  - Operational (running, queued, pending)
  - Lifetime counters (totalSubmitted, totalCompleted, totalFailed)
  - Proxy-specific (runningProxied, proxyConcurrencyLimit)
  - Persistence queue info (depth, capacity, batchSize)

**Public API**:
- TaskLifecycleEvent & QueueMetrics structs

**Patterns**:
- Immutable event records
- Rich metrics with proxy-specific insights
- Separation of operational vs lifetime counters

**Limitations**:
- No event filtering/querying helpers
- Metrics are point-in-time snapshots (no time-series)
- Missing event retention policy constants

**TODOs/Bottlenecks**:
- No event aggregation/analytics helpers
- Metrics lack timestamp (when captured)
- No built-in alerting thresholds

---

## 8. internal/models/logs.go

**Purpose**: Detailed execution, network, and WebSocket logging.

**Key Data Structures**:
- `StepLog`: TaskID, stepIndex, action, selector, value, snapshotID, errorCode, errorMsg, durationMs, startedAt
- `NetworkLog`: TaskID, stepIndex, requestURL, method, statusCode, mimeType, requestHeaders (JSON string), responseHeaders (JSON string), requestSize, responseSize, durationMs, error, timestamp
- `WebSocketLog`: FlowID, stepIndex, requestID, URL, eventType, direction, opcode, payloadSize, payloadSnippet (512B max), closeCode, closeReason, errorMessage, timestamp
- `ErrorCode`: Enum (TIMEOUT, SELECTOR_NOT_FOUND, NAVIGATION_FAILED, PROXY_FAILED, AUTH_REQUIRED, NETWORK_ERROR, EVAL_BLOCKED, EVAL_FAILED, SCREENSHOT_FAILED, UNKNOWN)

**Public API**:
- `ClassifyError()`: Maps error strings to standardized ErrorCode
- `TruncatePayload()`: Limits WebSocket payloads to 512B
- 10 ErrorCode constants

**Patterns**:
- Case-insensitive error classification
- HAR-like network logging (headers as JSON strings)
- WebSocket event stream capture with truncation
- Error code inference from error message substrings

**Limitations**:
- Error classification uses substring matching (brittle)
- Headers stored as JSON strings (not parsed objects)
- WebSocket payload always truncated (no full payload option)
- Missing request/response body capture for HTTP

**TODOs/Bottlenecks**:
- Error classification needs regex/pattern library
- WebSocket opcode not type-safe (int only)
- Missing network log request body capture
- MaxWSPayloadSnippet is hardcoded to 512B
- No automatic PII redaction in logs

---

## 9. internal/proxy/manager.go

**Purpose**: Proxy pool selection, rotation, health checking, and reservation management.

**Key Data Structures**:
- `Manager`: DB reference, config, mutex, round-robin index, active reservations map, country fallback counters, stop channel
- `Reservation`: Lightweight lease wrapper with oneshot release guarantee

**Public API**:
- `NewManager()`: Create with DB and config
- `StartHealthChecks()`: Begin periodic health polling
- `Stop()`: Halt health checks
- `SelectProxy()`: Get proxy by geo with strict fallback
- `SelectProxyWithFallback()`: Advanced selection with fallback strategy
- `ReserveProxy()`, `ReserveProxyWithFallback()`: Reserve with active lease tracking
- `Reservation.Complete(success)`, `Reservation.Release()`: Finalize reservation
- `RecordUsage()`: Manual usage tracking
- `ActiveReservations()`: Query current lease count
- `CountryStats()`: Aggregated per-country stats

**Patterns**:
- Reservation with oneshot once-guard for idempotent release
- Multiple rotation strategies (RR, random, least-used, lowest-latency)
- Geo-aware selection with fallback chaining
- Health check with HTTP GET + latency measurement
- Active reservation tracking for load balancing

**Limitations**:
- Health check URL hardcoded to httpbin.org (no customization at runtime)
- Health check assumes HTTP client (no SOCKS5 proxy health checks)
- Fallback counter counts all fallbacks globally (not per-proxy)
- Round-robin index never resets (monotonic increase)
- No connection pooling optimization in health checks

**Error Handling**:
- `ErrNoHealthyProxies` sentinel error
- Health check failures silently mark proxy unhealthy
- DB write errors logged but not propagated

**TODOs/Bottlenecks**:
- Health check parallelization using WaitGroup (no concurrency limit)
- Reservation counter incremented without checking proxy health state
- Missing proxy quarantine/circuit-breaker pattern
- No metrics emission (counters exist but no export)
- DB write errors during health checks not bubbled up

---

## 10. internal/localproxy/manager.go

**Purpose**: Local SOCKS5 proxy gateway for credential masking and connection pooling.

**Key Data Structures**:
- `Manager`: Endpoints map (upstream config → endpoint), idle timeout, stop channel, waitgroup, error counters
- `endpointEntry`: Upstream config, TCP listener, address, credentials, active connections, lastUsed time, stopping flag

**Public API**:
- `NewManager()`: Create with idle timeout (default 5 min)
- `Endpoint()`: Get or create local SOCKS5 endpoint for upstream proxy
- `Stop()`: Gracefully shutdown all endpoints
- `Stats()`: Get LocalProxyGatewayStats
- `EndpointStatsByProxy()`: Map proxy IDs to active connection counts
- `EndpointAddr()`: Query local address for proxy config
- `RecordAuthFailure()`, `RecordUpstreamFailure()`: Error tracking

**Patterns**:
- Lazy endpoint creation on first use
- Automatic idle endpoint reaping (1-minute intervals)
- SOCKS5 server per upstream proxy (shared via key dedup)
- Credential generation (random 8-byte tokens with prefix)
- Safe concurrent access with mutex-guarded maps

**Limitations**:
- No connection limits per endpoint
- Idle timeout hardcoded minimum (ignores <0 values)
- Credential format hardcoded (fp-*, tok-*)
- SOCKS5 handler error detection uses substring matching (fragile)
- No TLS support for local endpoints

**Error Handling**:
- `handleSOCKS5Client` errors classified by substring (auth vs upstream)
- Listener accept errors silently treated as shutdown if stopping flag set
- No error propagation for initial listen failure (silently returned)

**TODOs/Bottlenecks**:
- SOCKS5 handler is external function (not shown, likely in another file)
- Race condition: double-check on endpoint creation (check → lock → check pattern)
- Active connection counter never reset on endpoint stop
- No connection rate limiting
- Missing metrics emission for connection establishment

---

## 11. internal/recorder/recorder.go

**Purpose**: Live recording of user interactions via Chrome DevTools Protocol.

**Key Data Structures**:
- `Recorder`: Context management, CDP client, flow/step tracking, event handler, network/WS loggers, snapshotter
- `EventHandler`: Callback signature for recorded steps
- `CDPClient`: Interface (implemented by chromeCDPClient)

**Public API**:
- `New()`: Create with parent context, flowID, handler
- `Start(url)`: Launch headless=false Chrome, enable CDP domains, inject capture script, navigate to URL
- `Stop()`: Gracefully shutdown Chrome
- `BrowserCtx()`: Get browser context
- `FlowID()`: Get flow ID
- `RecordStep()`: Manual step recording with snapshot capture
- `NetworkLogs()`, `WebSocketLogs()`: Retrieve logs
- `SetSnapshotter()`: Inject snapshot capturer
- `SetWSCallback()`: Set WebSocket event callback

**Patterns**:
- Singleton allocator context per recorder (headless=false)
- CDP event listener registration at start
- JavaScript injection via runtime binding + capture script
- Step index auto-increment with snapshot capture async
- Mutex protection for shared state (stepIndex, browserCtx)

**Limitations**:
- Headless=false required (no headless support)
- GPU disabled + unsafe swiftshader forced (no options)
- Capture script injection happens twice (AddScriptToEvaluateOnNewDocument + Evaluate)
- Step recording blocks on snapshotter calls
- No event filtering or rate limiting

**Error Handling**:
- Errors in Start() wrapped with context
- Step recording silently skips if handler/browserCtx nil
- Snapshot capture errors silently ignored (step recorded without snapshotID)

**TODOs/Bottlenecks**:
- `captureScript` and `bindingName` not defined in file (external JS)
- `parseBindingPayload` not shown (likely external)
- `Snapshotter` interface/implementation not in file
- No replay/pause capability
- Missing detection of navigation/tab-switch races

---

## 12. internal/batch/batch.go

**Purpose**: Task creation engine for batch processing with template substitution.

**Key Data Structures**:
- `Engine`: DB reference
- `TemplateVars`: URL, domain, index, name (inferred from template functions)

**Public API**:
- `New(db)`: Create batch engine
- `CreateBatchFromFlow()`: Create tasks from flow with template substitution
- Helper functions (inferred): `ValidateTemplate()`, `ApplyTemplate()`, `ExtractDomain()`, `DefaultNameTemplate()`

**Patterns**:
- Transaction-based task creation (all-or-nothing)
- Per-URL template substitution in name, step values, selectors
- Auto-injection of URL into first navigate step if empty
- Proxy country override with clear/replace semantics
- Proxy fallback string-to-enum conversion

**Limitations**:
- No validation of template results (substituted values)
- No deduplication of generated task names
- URL extraction for domain not shown (external function)
- Fallback enum conversion uses string() cast only (error checking absent)
- No batch size validation at start (delegated to input validation)

**Error Handling**:
- Transaction begin/commit failures wrapped with context
- Task creation failures include URL index for debugging
- Invalid template treated as error (not fallback to default)

**TODOs/Bottlenecks**:
- Helper functions not shown (ValidateTemplate, ApplyTemplate, ExtractDomain)
- No idempotency key for batch creation
- Missing batch cancellation mid-transaction
- No progress reporting during large batch creation
- Proxy country override clears server/username/password (opaque semantics)

---

## 13. internal/validation/validate.go

**Purpose**: Comprehensive input validation for tasks, proxies, batches, and schedules.

**Key Data Structures**:
- Error constants: 45+ validation error types (all exported)
- Validation maps: validActions, selectorRequiredActions, valueRequiredActions, validPriorities, validProtocols, validProxyFallbacks, validStatuses

**Public API**:
- `ValidateTaskName()`: Non-empty, ≤255 chars, no control chars
- `ValidateTaskURL()`: Valid http/https URL with non-empty host
- `ValidateTaskSteps()`: Action validity, field requirements, eval allowlist, navigate URL validation
- `ValidateProxyServer()`: host:port format validation
- `ValidatePriority()`: Enum check (1, 5, 10)
- `ValidateProxyProtocol()`: http, https, socks5 only
- `ValidateTags()`: ≤20 tags, each ≤50 chars, no control chars
- `ValidateTimeout()`: 0-3600 seconds (0 = default)
- `ValidateTaskLoggingPolicy()`: maxExecutionLogs 0-5000
- `ValidateTask()`: Composite validation (name, URL, steps, priority)
- `ValidateProxy()`: Composite (server, protocol)
- `ValidateProxyFallback()`: strict/any_healthy/direct
- `ValidateProxyConfig()`: Optional proxy with semantic constraints
- `ValidateStatus()`: Task status enum check
- `ValidatePagination()`: page ≥1, pageSize 1-200, status/tag filter validation
- `ValidateBatchInput()`: URLs present, within limit, valid priorities, template syntax

**Patterns**:
- Centralized validation maps for quick lookup
- Control character filtering using unicode.IsControl()
- Semantic validation (e.g., selector required for click, not for navigate)
- Composite validators wrapping multiple checks with context wrapping
- Error wrapping with fmt.Errorf for stack traces

**Limitations**:
- No async validation (e.g., URL reachability)
- Control character check broad (may reject valid Unicode)
- Eval allowlist is binary (no granular script validation)
- Proxy fallback validation accepts string enum only (no conversion)
- Missing cross-field validation (e.g., consistency between multiple steps)

**Error Handling**:
- All errors are custom types (no standard library errors)
- Error messages include indices for batch/URL context
- No error aggregation (fails on first error)

**TODOs/Bottlenecks**:
- validStatuses hardcoded (not derived from models.TaskStatus consts)
- selectorRequiredActions missing some actions (e.g., ActionHover, ActionDragDrop)
- validActions derived at runtime (could be cached)
- No validation for step timeouts being positive
- Missing validation for max loop iterations, variable names, etc.
- No URL resolution/DNS validation

---

## 14. internal/crypto/crypto.go

**Purpose**: AES-256-GCM encryption for sensitive data (proxy credentials).

**Key Data Structures**:
- Global state: globalKey (32 bytes), globalOnce, globalErr (sync.Once pattern)

**Public API**:
- `InitKey(dataDir)`: Load or generate encryption key (one-time init)
- `InitKeyWithBytes(key)`: Set key directly for testing
- `Encrypt(plaintext)`: AES-256-GCM → base64, returns empty string for empty input
- `Decrypt(encoded)`: base64 → AES-256-GCM, returns plaintext for non-base64
- `ResetForTest()`: Clear global state (testing only)

**Patterns**:
- sync.Once for single initialization (thread-safe)
- AES-256-GCM with random nonce per encryption
- Base64 encoding for transport/storage
- Legacy migration: non-base64 values treated as plaintext
- Empty string short-circuit (no encryption overhead)

**Limitations**:
- No key rotation mechanism
- Global key stored in memory (not key derivation per operation)
- Nonce generated via crypto/rand (good randomness but no checking for collisions)
- Legacy migration path silently accepts unencrypted data (no audit)
- No versioning for encryption format (breaks if changing algorithm)

**Error Handling**:
- Key initialization errors captured in globalErr (deferred to InitKey return)
- Decrypt returns plaintext on base64 decode error (silent migration)
- Ciphertext too short error (length check before open)
- GCM open failures (authentication tag mismatch)

**Security Properties**:
- AES-256 symmetric (single global key)
- GCM provides authentication + confidentiality
- Random nonce per message (good)
- No key derivation or hardening
- Key file stored with 0o600 permissions (UNIX only)

**TODOs/Bottlenecks**:
- No key rotation or versioning
- No re-encryption on key change
- Missing encrypted storage for key (file system only)
- No audit log of encryption/decryption operations
- Legacy unencrypted data not migrated/tracked
- Key file path hardcoded as `.encryption_key`
- No support for envelope encryption (key wrapping)

---

## Cross-File Patterns & Dependencies

**Validation Chain**:
- Models define structs
- Validation package checks constraints (at API boundaries)
- Batch/recorder use validation before DB operations

**Proxy Routing Flow**:
- ProxyConfig (models) + RotationStrategy → Manager (proxy)
- Manager reserves/selects → LocalProxy creates SOCKS5 wrapper
- Task execution uses reserved proxy or direct connection

**Recording & Playback**:
- Recorder (CDP) → RecordedFlow (models) + DOMSnapshot
- RecordedFlow → Batch → Task (with substitution)
- Task execution replays steps from RecordedStep

**Encryption**:
- Crypto initializes once per app
- Proxy credentials encrypted at rest
- Decrypt on read, encrypt on write (app.go)

**Logging & Auditing**:
- StepLog, NetworkLog, WebSocketLog (models) via executing tasks
- TaskLifecycleEvent (models) for state transitions
- ErrorCode classification for standardization

---

## Summary of Missing Features & Bottlenecks

| Component | Gap | Impact |
|-----------|-----|--------|
| Task.go | No step action metadata | Validation must hardcode requirements |
| Flow.go | No selector validation/repair | Stale selectors undetected post-recording |
| Batch.go | Basic string templating only | No complex logic in batch names/values |
| Proxy.go | No circuit breaker pattern | Unhealthy proxies hammered with requests |
| Schedule.go | No timezone support | Only UTC, no cron DSL validation |
| Vision.go | No diff algorithm in model | Computation deferred, no baselines cleanup |
| Event.go | No time-series metrics | Snapshots only, no trend analysis |
| Logs.go | Substring error classification | Brittle, misclassifies errors |
| Manager (proxy) | Fallback counters global | Can't isolate per-proxy fallback rates |
| Manager (localproxy) | No connection limits | Risk of resource exhaustion |
| Recorder.go | No event filtering/rate limiting | High volume during complex interactions |
| Batch.go | No batch cancellation | Long-running batches can't stop early |
| Validation | No async/cross-field checks | URL reachability, step consistency unknown |
| Crypto.go | No key rotation | Manual migration required on algorithm change |

