# Code Deletion Log

## [2026-03-24] Refactor Session — Dead Code & Unused Export Cleanup

### Unused Exports Removed

- `internal/models/task.go` — `TaskContext` struct and `NewTaskContext()` function
  - Reason: Defined but never referenced in any production code path. No callers outside the models package itself.

- `internal/models/batch.go` — `TemplateVariable` struct and `SupportedVariables()` function
  - Reason: Only consumed by tests (`models_test.go`). Not used in any production code path. The corresponding test was also removed.

- `internal/logs/export.go` — `Exporter.ExportTaskLogs()` method (non-zip, returning `(string, string, error)`)
  - Reason: `app_export.go` only calls `ExportTaskLogsZip`. The non-zip variant was dead code never reachable from the application layer. Its two tests (`TestExportTaskLogs`, `TestExportTaskLogsNoData`) were also removed.

### Unexported Internal-Only Symbols

- `internal/browser/browser.go` — `ErrEvalScriptTooLarge` → `errEvalScriptTooLarge`
  - Reason: Only used internally within `browser.go` (`validateEvalScript`). No callers in any other package. Unexported to match Go convention for package-private sentinels.

- `internal/browser/browser.go` — `ErrEvalScriptEmpty` → `errEvalScriptEmpty`
  - Reason: Same as above — only used internally within `validateEvalScript`. Unexported.

- `internal/browser/pool.go` — `PoolStats` → `poolStats`, `BrowserPool.Stats()` → `BrowserPool.stats()`
  - Reason: `Stats()` was only called from `pool_test.go` (same package). No external callers in application code. Unexported to reflect package-private use. Test updated accordingly.

### Duplicate Code Analysis

- `ErrEvalNotAllowed` is defined in both `internal/browser/browser.go` and `internal/validation/validate.go` with slightly different messages. Each is used exclusively within its own package (browser enforces it at execution time; validation enforces it at input validation time). These serve distinct layers and are intentionally separate — **not consolidated** as removing either would change semantics.

### Unused Dependencies Removed

- None. `go mod tidy` confirmed all module dependencies are transitively required.

### Unused Imports Removed

- None additional. `go vet` and `go build` confirmed all imports are in use.

### Impact

- Files modified: 7
  - `internal/models/task.go`
  - `internal/models/batch.go`
  - `internal/models/models_test.go`
  - `internal/logs/export.go`
  - `internal/logs/export_test.go`
  - `internal/browser/browser.go`
  - `internal/browser/pool.go`
  - `internal/browser/pool_test.go`
- Lines of production code removed: ~60
- Lines of test code removed: ~100

### Testing

- All tests passing: `go test -tags=dev ./...` — all 17 packages OK
- `go vet` clean: `go vet -tags=dev ./...` — no issues
- `go build` clean: `go build -tags=dev ./...` — no errors
