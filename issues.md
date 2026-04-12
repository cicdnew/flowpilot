# SonarCloud Issues — flowpilot (cicdnew_flowpilot)

**Org:** cicdnew | **Branch:** main | **Analysis:** 2026-04-11 | **Quality Gate:** ERROR

## Summary

| Metric | Count |
|--------|-------|
| **Total Issues** | 627 |
| **Total Debt** | 6,407 min |
| Bugs | 2 |
| Vulnerabilities | 8 |
| Code Smells | 617 |

### By Severity

| Severity | Count |
|----------|-------|
| CRITICAL | 503 |
| MAJOR | 61 |
| MINOR | 62 |
| INFO | 1 |

### By Language

| Language | Count |
|----------|-------|
| Go | 541 |
| JavaScript | 55 |
| TypeScript | 27 |
| CSS | 4 |

### Top Impact Areas

| Software Quality | Count |
|-----------------|-------|
| Maintainability | 615 |
| Security | 10 |
| Reliability | 7 |

---

## Issue Breakdown by Rule

| Rule | Count | Description |
|------|-------|-------------|
| **go:S1192** | **402** | Duplicate string literals — define constants instead |
| **go:S1186** | **61** | Empty function/method bodies |
| **javascript:S7764** | 51 | JavaScript issues (mostly frontend/wailsjs) |
| **go:S3776** | 39 | Cognitive complexity too high |
| **typescript:S4144** | 18 | Duplicate function declarations |
| **godre:S8242** | 10 | Go directory-related rules |
| **go:S2068** | 8 | Hard-coded credentials (SECURITY) |
| **go:S107** | 6 | Too many parameters |
| **typescript:S7764** | 5 | TypeScript issues |
| **css:S7924** | 4 | CSS issues |
| **javascript:S107** | 4 | Too many parameters |
| **godre:S8209** | 3 | Go rules |
| **go:S2612** | 2 | Security-sensitive piece of code |
| **godre:S8239** | 2 | Go rules |
| **typescript:S6571** | 2 | TypeScript issues |
| **go:S4144** | 1 | Duplicate code |
| **go:S1479** | 1 | Switch statement too complex |
| **go:S1871** | 1 | Duplicate case clause |
| **go:S1135** | 1 | TODOs should have a reason |
| **godre:S8168** | 1 | Go rule |
| **godre:S8188** | 1 | Go rule |
| **godre:S8193** | 1 | Go rule |
| **typescript:S2871** | 1 | TypeScript |
| **typescript:S6544** | 1 | TypeScript |

---

## Issues by File

### Production Code (Go)

| File | S1192 | S1186 | S3776 | Other | Total |
|------|-------|-------|-------|-------|-------|
| internal/database/sqlite.go | - | - | - | 1 godre | 1 |
| internal/database/migrations.go | - | - | 1 | - | 1 |
| internal/database/db_tasks.go | 8 | - | 2 | 1 S107 | 11 |
| internal/database/db_captcha.go | 1 | - | - | - | 1 |
| internal/database/db_logs.go | 2 | - | - | - | 2 |
| internal/database/db_proxies.go | 1 | - | - | - | 1 |
| internal/database/db_schedules.go | 1 | - | - | - | 1 |
| internal/copilot/provider.go | 7 | - | 2 | 2 godre | 11 |
| internal/copilot/tools.go | 1 | - | 1 | - | 2 |
| internal/copilot/agent.go | - | - | 2 | 1 godre | 3 |
| internal/browser/browser.go | 1 | 1 | 3 | 1 S4144 | 6 |
| internal/browser/steps.go | 1 | - | 1 | 1 S1479 | 3 |
| internal/browser/pool.go | 1 | - | 1 | 4 godre | 6 |
| internal/browser/conditions.go | - | - | 1 | - | 1 |
| internal/captcha/twocaptcha.go | 1 | - | - | - | 1 |
| internal/proxy/manager.go | - | - | - | 1 godre | 1 |
| internal/localproxy/manager.go | - | - | 1 | - | 1 |
| internal/queue/queue.go | 2 | - | 4 | 1 S1871, 1 godre | 8 |
| internal/queue/priority_heap.go | - | - | - | 1 godre | 1 |
| internal/recorder/recorder.go | - | - | 1 | - | 1 |
| internal/validation/validate.go | 1 | - | 1 | - | 2 |
| internal/logs/export.go | - | - | 1 | - | 1 |
| internal/vision/diff.go | - | - | 1 | - | 1 |
| app.go | 1 | - | 2 | - | 3 |
| app_schedules.go | 2 | - | - | 2 S107 | 4 |
| app_tasks.go | 2 | - | - | 2 S107 | 4 |
| app_captcha.go | 2 | - | - | - | 2 |
| main_dev.go | - | 1 | - | - | 1 |
| cmd/copilot/main.go | - | - | 1 | - | 1 |
| cmd/agent/main.go | - | - | - | 1 godre | 1 |
| cmd/copilot/tui/input.go | - | - | - | 1 S1135 | 1 |
| cmd/copilot/tui/styles.go | 1 | - | - | - | 1 |

### Test Code (Go)

| File | S1192 | S1186 | S3776 | S2068 | Other | Total |
|------|-------|-------|-------|-------|-------|-------|
| internal/database/sqlite_test.go | 126 | - | 2 | **6** | - | 134 |
| internal/database/db_task_atomicity_test.go | 7 | - | - | - | - | 7 |
| internal/copilot/agent_test.go | 18 | 5 | - | - | - | 23 |
| internal/copilot/provider_test.go | 4 | - | 1 | - | - | 5 |
| internal/copilot/types_test.go | 2 | - | - | - | - | 2 |
| internal/browser/browser_test.go | 17 | - | - | - | - | 17 |
| internal/browser/actions_test.go | 6 | - | - | - | - | 6 |
| internal/browser/chrome_integration_test.go | 6 | - | - | - | - | 6 |
| internal/browser/conditions_test.go | 2 | - | 1 | - | - | 3 |
| internal/captcha/captcha_test.go | 1 | - | - | - | - | 1 |
| internal/captcha/providers_test.go | 11 | - | 2 | - | - | 13 |
| internal/localproxy/localproxy_extra_test.go | 5 | - | - | - | - | 5 |
| internal/localproxy/manager_test.go | 1 | - | - | - | - | 1 |
| internal/localproxy/socks5_test.go | 10 | - | - | - | - | 10 |
| internal/logs/export_test.go | 10 | - | 2 | - | - | 12 |
| internal/logs/network_test.go | 3 | - | - | - | - | 3 |
| internal/logs/websocket_test.go | 8 | - | - | - | - | 8 |
| internal/proxy/manager_test.go | 5 | - | - | - | - | 5 |
| internal/queue/queue_test.go | 21 | 30 | 1 | - | 1 godre | 53 |
| internal/queue/priority_heap_test.go | - | 12 | - | - | - | 12 |
| internal/recorder/recorder_test.go | 6 | 5 | - | - | - | 11 |
| internal/scheduler/scheduler_test.go | 2 | - | - | - | - | 2 |
| internal/scheduler/scheduler_extra_test.go | 1 | - | - | - | - | 1 |
| internal/validation/validate_test.go | 5 | - | - | - | - | 5 |
| internal/vision/diff_test.go | 4 | - | - | 1 godre | 5 |
| internal/models/models_test.go | 1 | - | - | - | - | 1 |
| internal/agent/agent_test.go | 1 | - | - | - | - | 1 |
| cmd/copilot/tui/commands_test.go | 2 | - | - | - | - | 2 |
| cmd/copilot/tui/model_test.go | 1 | - | - | - | - | 1 |
| app_test.go | 32 | - | - | - | - | 32 |
| app_flow_regression_test.go | 1 | - | - | - | - | 1 |

### Frontend (JS/TS/CSS)

| File | Issues | Rules |
|------|--------|-------|
| frontend/wailsjs/go/models.ts | 12 | typescript:S4144 |
| frontend/src/lib/step-actions.test.ts | 4 | typescript:S7764 |
| frontend/wailsjs/go/main/App.js | 4 | javascript:S107 |
| frontend/src/lib/types.ts | 2 | typescript:S6571 |
| frontend/src/lib/step-actions.ts | 2 | typescript:S6544, typescript:S7764 |
| frontend/src/style.css | 1 | css:S7924 |

---

## Security Issues (Priority Fix)

### go:S2068 — Hard-coded Credentials (6 OPEN)

These are in test files but still should be addressed:

| File | Line | Issue |
|------|------|-------|
| internal/database/sqlite_test.go | 435 | `p.Password = "healthy_pass"` |
| internal/database/sqlite_test.go | 620 | `p.Password = "cleartext_pass"` |
| internal/database/sqlite_test.go | 665 | `task.Proxy.Password = "task_pass"` |
| internal/database/sqlite_test.go | 737 | `task.Proxy.Password = "first_pass"` |
| internal/database/sqlite_test.go | 3927 | `p.Password = "pass"` |
| internal/database/sqlite_test.go | 4125 | `p.Password = "mypass"` |

**Fix:** Extract to a `testPassword` constant.

### go:S2612 — Security-Sensitive Code (2 OPEN)

Check if these are actual secrets or just test data.

---

## Top Duplicate String Literals (go:S1192) by Impact

The **402 S1192 issues** are concentrated in test files. Top offenders in production code:

| File | String | Count | Suggested Constant |
|------|--------|-------|-------------------|
| internal/database/db_tasks.go | "task %s not found" | 9 | errTaskNotFound |
| internal/queue/queue.go | "update status: %v" | 6 | errUpdateStatus |
| internal/queue/queue.go | "cancelled by user" | 4 | msgCancelledByUser |
| internal/browser/steps.go | "click_ad: %v" | 4 | errClickAd |
| internal/browser/pool.go | "browser pool is stopped" | 3 | msgPoolStopped |
| internal/database/db_tasks.go | "marshal logging policy: %w" | 3 | errMarshalLoggingPolicy |
| internal/database/db_tasks.go | "marshal steps: %w" | 3 | errMarshalSteps |
| internal/database/db_tasks.go | "marshal tags: %w" | 3 | errMarshalTags |
| internal/database/db_tasks.go | "encrypt proxy username: %w" | 3 | errEncryptUsername |
| internal/database/db_tasks.go | "encrypt proxy password: %w" | 3 | errEncryptPassword |
| internal/database/db_tasks.go | "scan task row: %w" | 3 | errScanTaskRow |
| internal/database/db_tasks.go | "check update result for task %s: %w" | 3 | errCheckUpdateResult |
| app.go | "startup failed: %v" | 7 | errStartupFailed |

---

## Empty Function Bodies (go:S1186) — 61 issues

All in test files:

| File | Count |
|------|-------|
| internal/queue/queue_test.go | 30 |
| internal/queue/priority_heap_test.go | 12 |
| internal/copilot/agent_test.go | 5 |
| internal/recorder/recorder_test.go | 5 |
| main_dev.go | 1 |
| internal/browser/browser.go | 1 |

**Fix:** Either implement the function or remove it. If intentionally empty for future implementation, add a `// TODO:` comment to silence the rule.

---

## Cognitive Complexity (go:S3776) — 39 issues

Functions that are too complex and should be refactored. Top files:
- internal/queue/queue.go (4 issues)
- internal/browser/browser.go (3 issues)
- internal/copilot/provider.go (2 issues)
- internal/copilot/agent.go (2 issues)
- internal/database/db_tasks.go (2 issues)

**Fix:** Extract sub-functions, simplify conditional logic.

---

## Fixes Applied Locally (not yet pushed)

| File | Changes |
|------|---------|
| internal/copilot/provider.go | Extracted 4 constants: errSendRequest, errAPIError, authBearerPrefix + replaced all 6 duplicate usages |
| internal/copilot/tools.go | Extracted errTaskIDRequired constant + replaced 4 usages |
| cmd/copilot/tui/styles.go | Extracted colorWhite constant + replaced 3 usages |
| internal/database/sqlite_test.go | Added 40+ test constants block + started S2068 fix (in progress) |

**Status after local fixes applied (estimate, not yet reflected in SonarCloud):**
- S1192 in provider.go: RESOLVED
- S1192 in tools.go: RESOLVED  
- S1192 in styles.go: RESOLVED
- S2068 in sqlite_test.go: IN PROGRESS
- S1192 in sqlite_test.go: PARTIALLY DONE (constants added, replacements not yet applied)