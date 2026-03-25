# FlowPilot Coverage Report

Generated: 2026-03-24

## Summary

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| **Total coverage** | ~62% (est.) | **63.8%** | +1.8% |
| `internal/scheduler` | 80.0% | **97.1%** | **+17.1%** |
| `internal/localproxy` | 32.0% | **56.8%** | **+24.8%** |
| `internal/database` | 63.0% | **74.5%** | **+11.5%** |

All 16 packages pass. `go vet` reports no issues.

---

## Per-Package Coverage (After)

| Package | Coverage |
|---------|----------|
| `flowpilot` (app.go / app_*.go) | 22.6% |
| `flowpilot/cmd/agent` | 0.0% |
| `internal/agent` | 85.7% |
| `internal/batch` | 84.7% |
| `internal/browser` | 55.0% |
| `internal/captcha` | 80.2% |
| `internal/crypto` | 85.0% |
| `internal/database` | 74.5% |
| `internal/localproxy` | 56.8% |
| `internal/logs` | 85.7% |
| `internal/models` | 88.9% |
| `internal/proxy` | 67.5% |
| `internal/queue` | 76.2% |
| `internal/recorder` | 85.5% |
| `internal/scheduler` | 97.1% |
| `internal/validation` | 97.8% |
| `internal/vision` | 88.3% |

---

## New Tests Added

### `internal/scheduler/scheduler_extra_test.go`
Coverage: 80.0% → **97.1%** (+17.1%)

| Test | What it covers |
|------|----------------|
| `TestParseCronInvalidFields` | 17 invalid cron expressions (out-of-range, bad steps, bad ranges, wrong field count) |
| `TestParseCronValidExpressions` | 11 valid expressions incl. step-from-range, step-from-value |
| `TestCronNextAdvancesCorrectly` | Table-driven: midnight, 30-min, hourly boundary, step |
| `TestSchedulerDoesNotStartTwice` | Idempotent `Start()` |
| `TestSchedulerStopIdempotent` | `Stop()` without prior `Start()` |
| `TestSchedulerContextCancellation` | Context cancel exits loop |
| `TestParseCronStepFromRange` | `9-17/2` → hours 9,11,13,15,17 |
| `TestParseCronStepFromValue` | `9/3` → hours 9,12,15,18,21 |
| `TestParseCronDedup` | `0,0,0` deduped to single value |

Previously-uncovered functions now hit: `parseStep` (range branch, value branch), `Start`/`Stop`/`loop` edge cases, `ParseCron` error paths.

---

### `internal/localproxy/localproxy_extra_test.go`
Coverage: 32.0% → **56.8%** (+24.8%)

| Test | What it covers |
|------|----------------|
| `TestManagerEmptyServerPassthrough` | Empty server config is passed through unchanged |
| `TestManagerWhitespaceServerPassthrough` | Whitespace-only server is passed through |
| `TestManagerDefaultIdleTimeout` | Zero timeout defaults to 5 minutes |
| `TestManagerStats` | `Stats()` creation/reuse counters |
| `TestManagerRecordAuthFailure` | `RecordAuthFailure()` counter |
| `TestManagerRecordAuthFailureWithError` | `RecordAuthFailure()` last error string |
| `TestManagerRecordUpstreamFailure` | `RecordUpstreamFailure()` counter + last error |
| `TestManagerEndpointAddr` | `EndpointAddr()` for active endpoint |
| `TestManagerEndpointAddrMissing` | `EndpointAddr()` returns "" for unknown |
| `TestManagerEndpointStatsByProxy` | `EndpointStatsByProxy()` active count mapping |
| `TestManagerStopClearsEndpoints` | `Stop()` removes all endpoints |
| `TestManagerPruneIdle` | `pruneIdle()` removes expired endpoints |
| `TestManagerDifferentUpstreamsGetDifferentEndpoints` | Two upstreams → two distinct local endpoints |
| `TestSOCKS5HandshakeNoAuth` | No-auth method negotiation |
| `TestSOCKS5HandshakeNoAcceptableMethod` | No-compatible-method returns 0xFF |
| `TestSOCKS5HandshakeWrongVersion` | SOCKS4 header → version error |
| `TestSOCKS5AuthWrongVersion` | Auth sub-version 0x02 → error |
| `TestReadSOCKS5ConnectRequestIPv4` | IPv4 address type (0x01) parsing |
| `TestReadSOCKS5ConnectRequestDomain` | Domain address type (0x03) parsing |
| `TestReadSOCKS5ConnectRequestIPv6` | IPv6 address type (0x04) parsing |
| `TestReadSOCKS5ConnectRequestUnsupportedCmd` | BIND command → error |
| `TestReadSOCKS5ConnectRequestUnsupportedAddrType` | Unknown ATYP 0xFF → error |
| `TestDialViaUpstreamUnsupportedProtocol` | `ftp://` protocol → error |

Previously-uncovered functions now hit: `EndpointStatsByProxy`, `RecordAuthFailure`, `RecordUpstreamFailure`, `Stats`, `EndpointAddr`, `pruneIdle`, `readSOCKS5ConnectRequest` (all address types), `performSOCKS5Handshake` (no-auth + no-method branches), `authenticateSOCKS5UserPass` (wrong version), `dialViaUpstream` (unsupported protocol).

---

### `internal/database/db_extras_test.go`
Coverage: 63.0% → **74.5%** (+11.5%)

| Test | What it covers |
|------|----------------|
| `TestCreateAndListProxyRoutingPresets` | `CreateProxyRoutingPreset`, `ListProxyRoutingPresets` (both were 0%) |
| `TestDeleteProxyRoutingPreset` | `DeleteProxyRoutingPreset` (was 0%) |
| `TestDeleteProxyRoutingPresetNotFound` | Not-found error path |
| `TestBoolToInt` | `boolToInt()` helper (was 0%) |
| `TestListStaleTasks` | `ListStaleTasks` (was 0%) |
| `TestBatchApplyTaskStateChangesEmpty` | Empty input no-op (was 0%) |
| `TestBatchApplyTaskStateChanges` | Bulk running-status updates (was 0%) |
| `TestBatchUpdateTaskStatus` | Wrapper + empty input (was 0%) |
| `TestUpdateCaptchaConfig` | `UpdateCaptchaConfig` (was 0%) |
| `TestListBatchGroups` | `ListBatchGroups` (was 0%) |
| `TestDeleteVisualDiff` | `DeleteVisualDiff` (was 0%) |
| `TestBeginTxAndCreateTaskTx` | `BeginTx` + `CreateTaskTx` (both 0%) |
| `TestBatchGroupTx` | `CreateBatchGroupTx` (was 0%) |

Previously 0%-covered functions now hit: `CreateProxyRoutingPreset`, `ListProxyRoutingPresets`, `DeleteProxyRoutingPreset`, `boolToInt`, `ListStaleTasks`, `BatchApplyTaskStateChanges`, `BatchUpdateTaskStatus`, `UpdateCaptchaConfig`, `ListBatchGroups`, `DeleteVisualDiff`, `BeginTx`, `CreateTaskTx`, `CreateBatchGroupTx`.

---

## Remaining 0%-Coverage Functions (Notable)

These are intentionally out of scope for this pass (require Wails runtime, real Chrome, or external services):

| Function | Reason |
|----------|--------|
| `app.go: startup/shutdown/cleanup` | Requires Wails runtime context |
| `app_recorder.go: StartRecording/PlayRecordedFlow` | Requires live Chrome/CDP |
| `app_batch.go: CreateBatchFromFlow` et al. | Requires full Wails app setup |
| `browser/pool.go: newTabContext/createBrowser` | Requires real Chrome process |
| `browser/steps.go: execWhile/execIfExists` et al. | Requires live browser page |
| `cmd/agent: main` | CLI entry point |

## Future Coverage Opportunities

1. **`internal/proxy`** (67.5%): Add tests for `normalizeCountry`, `CountryStats`, `recordFallback`, `ProxyConfig()` on reservation
2. **`internal/browser`** (55.0%): Mock CDP client to test `execWhile`, `execIfExists`, `execVariableMath` etc.
3. **`flowpilot` (app layer)** (22.6%): Integration tests with `setupTestApp()` for flow CRUD, schedule management, export functions
4. **`internal/queue`** (76.2%): Add tests for `SetProxyConcurrencyLimit`, deeper `executeTask` error paths
