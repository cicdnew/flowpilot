# FlowPilot — Browser Automation Codemap

> **Freshness:** 2026-03-24  
> **Cross-refs:** [INDEX.md](INDEX.md) | [backend.md](backend.md) | [workers.md](workers.md) | [integrations.md](integrations.md)

## Overview

The `internal/browser` package is the execution engine for all automation tasks. It wraps **chromedp** (Chrome DevTools Protocol) and provides a `Runner` that executes flows step-by-step. A `BrowserPool` reuses Chrome processes across tasks for performance.

The `internal/recorder` package provides the recording side: it opens a headed Chrome, injects a JavaScript capture script, and converts user interactions into `RecordedStep` structs.

---

## Package Structure

```
internal/browser/
  browser.go      Runner struct — launches chromedp, runs step loop
  pool.go         BrowserPool — reuses Chrome allocators across tasks
  executor.go     Executor interface — abstraction for testability
  steps.go        executeStep() — 50+ action type handlers
  conditions.go   evaluateCondition() — if_element / if_text / if_url logic

internal/recorder/
  recorder.go     Recorder struct — manages a recording session
  cdp.go          CDPClient interface — wraps chromedp for testability
  injector.go     InjectCaptureScript() — JS event binding via CDP
  selector.go     rankSelectors() — stability-scored CSS selector ranking
  snapshot.go     Snapshotter — DOM HTML + full-page screenshot per step
```

---

## BrowserPool (`pool.go`)

The pool avoids spawning a new Chrome process for every task (expensive).

```
┌─────────────────────────────────────┐
│           BrowserPool               │
│  size        int                    │
│  idleTimeout time.Duration          │
│  entries     []*poolEntry           │
│    └── allocCtx  context.Context    │
│        cancelFn  context.CancelFunc │
│        lastUsed  time.Time          │
│        inUse     bool               │
└──────────────┬──────────────────────┘
               │ Acquire() / Release()
               ▼
     chromedp.NewExecAllocator(ctx, opts...)
```

### Key Methods

| Method | Description |
|--------|-------------|
| `NewPool(size, idleTimeout, opts)` | Creates pool with Chrome launch options |
| `Acquire(ctx)` | Returns an idle allocator context (or creates new one) |
| `Release(allocCtx)` | Marks entry as idle, records lastUsed timestamp |
| `cleanup()` | Background goroutine — terminates Chrome processes idle beyond `idleTimeout` |
| `Close()` | Cancels all allocator contexts, terminates all Chrome processes |

### Chrome Launch Options Applied
- `--no-sandbox`, `--disable-gpu`, `--disable-dev-shm-usage`
- `--disable-blink-features=AutomationControlled` (anti-bot)
- Proxy flag injected per-task: `--proxy-server=socks5://127.0.0.1:<port>`

---

## Runner (`browser.go`)

The `Runner` is instantiated per task execution and drives the step loop.

```go
type Runner struct {
    db          *database.DB
    captcha     captcha.Solver      // optional
    proxyMgr    *proxy.Manager      // optional
    localProxy  *localproxy.Manager // SOCKS5 gateway
    pool        *BrowserPool
    allowEval   bool                // eval script gate (security)
}
```

### Execution Flow

```
Runner.Run(ctx, task) error
  │
  ├── proxyMgr.Rotate(task) → selects proxy for task
  ├── localProxy.Start(proxy) → SOCKS5 listener on 127.0.0.1:random
  ├── pool.Acquire(ctx) → allocator context with --proxy-server flag
  ├── chromedp.NewContext(allocCtx) → browser tab context
  ├── StepLogger.New(task.ID) → logs each step
  ├── NetworkLogger.Attach(ctx) → captures fetch/XHR/WS events
  │
  ├── navigate to task.StartURL
  │
  └── for each step in task.Steps:
        ├── evaluateCondition() → skip step if condition false
        ├── executeStep(ctx, step, logger, ...)
        │     └── dispatches on step.Action (50+ cases)
        └── on error: retry logic, or abort
  
  └── TaskResult { Success, Duration, Screenshots, Logs }
```

### Logging Policy (`TaskLoggingPolicy`)

Controlled per-task:
```go
type TaskLoggingPolicy struct {
    LogSteps        bool
    LogNetwork      bool
    LogWebSocket    bool
    CaptureScreenshots bool
}
```

---

## Step Actions (`steps.go`)

`executeStep()` is a giant switch on `step.Action`. All handlers receive `chromedp.Context`.

### Navigation & Page Control

| Action | Description |
|--------|-------------|
| `ActionNavigate` | `chromedp.Navigate(url)` |
| `ActionBack` | `chromedp.NavigateBack()` |
| `ActionForward` | `chromedp.NavigateForward()` |
| `ActionReload` | `chromedp.Reload()` |
| `ActionWait` | `time.Sleep(duration)` |
| `ActionWaitForElement` | `chromedp.WaitVisible(selector)` |
| `ActionWaitForURL` | polls `chromedp.Location()` |
| `ActionWaitForText` | polls `chromedp.Text()` |

### Interaction

| Action | Description |
|--------|-------------|
| `ActionClick` | `chromedp.Click(selector)` |
| `ActionDoubleClick` | `chromedp.DoubleClick(selector)` |
| `ActionRightClick` | `chromedp.MouseClickXY` with right button |
| `ActionHover` | `chromedp.MouseMoveXY` |
| `ActionType` | `chromedp.SendKeys(selector, text)` |
| `ActionClear` | clears input value then types |
| `ActionSelect` | `chromedp.SetAttributeValue` for `<select>` |
| `ActionCheck` / `ActionUncheck` | sets checkbox state |
| `ActionFocus` | `chromedp.Focus(selector)` |
| `ActionBlur` | fires blur event via JS |
| `ActionDragAndDrop` | dispatches mousedown/mousemove/mouseup events |

### Keyboard & Mouse

| Action | Description |
|--------|-------------|
| `ActionKeyPress` | `chromedp.KeyEvent(key)` |
| `ActionScrollTo` | `chromedp.ScrollIntoView(selector)` |
| `ActionScrollBy` | executes `window.scrollBy(x,y)` |
| `ActionScrollToPosition` | `window.scrollTo(x,y)` |

### Extraction & Assertions

| Action | Description |
|--------|-------------|
| `ActionExtractText` | reads element `.textContent` |
| `ActionExtractAttribute` | reads element attribute |
| `ActionExtractHTML` | reads element `.innerHTML` |
| `ActionAssertText` | asserts text contains/equals expected |
| `ActionAssertElement` | asserts element exists/visible |
| `ActionAssertURL` | asserts current URL matches |
| `ActionAssertTitle` | asserts document.title |

### Screenshots & Vision

| Action | Description |
|--------|-------------|
| `ActionScreenshot` | `chromedp.FullScreenshot()` → stored in TaskResult |
| `ActionCaptureBaseline` | saves screenshot as visual baseline in DB |
| `ActionCompareBaseline` | diffs against baseline via `vision.Diff()` |

### Browser State

| Action | Description |
|--------|-------------|
| `ActionGetCookies` | `chromedp.Cookies()` |
| `ActionSetCookie` | `network.SetCookie()` |
| `ActionDeleteCookie` | `network.DeleteCookies()` |
| `ActionGetLocalStorage` | executes `localStorage.getItem()` |
| `ActionSetLocalStorage` | executes `localStorage.setItem()` |
| `ActionClearLocalStorage` | executes `localStorage.clear()` |
| `ActionGetSessionStorage` | executes `sessionStorage.getItem()` |
| `ActionSetSessionStorage` | executes `sessionStorage.setItem()` |

### JavaScript

| Action | Description |
|--------|-------------|
| `ActionEval` | `chromedp.Evaluate(script)` — blocked if `allowEval=false` |
| `ActionRunScript` | alias for Eval with result capture |

### Network & Session

| Action | Description |
|--------|-------------|
| `ActionSetHeader` | sets request header via CDP Network.setExtraHTTPHeaders |
| `ActionSetUserAgent` | sets UA string |
| `ActionBlockURL` | blocks network request patterns |
| `ActionSetViewport` | `emulation.SetDeviceMetricsOverride` |

### CAPTCHA

| Action | Description |
|--------|-------------|
| `ActionSolveCaptcha` | detects captcha type, calls `captcha.Solver.Solve()`, injects token |

### Tab Management

| Action | Description |
|--------|-------------|
| `ActionNewTab` | `chromedp.NewContext(parent)` creates new tab |
| `ActionSwitchTab` | switches active chromedp context |
| `ActionCloseTab` | cancels tab context |

### Conditional & Control Flow

| Action | Description |
|--------|-------------|
| `ActionIfElement` | evaluates condition; skips block if false |
| `ActionIfText` | text-based condition |
| `ActionIfURL` | URL-based condition |
| `ActionEndIf` | marks end of conditional block |
| `ActionRepeat` | repeats nested steps N times |

---

## Conditions (`conditions.go`)

```go
func evaluateCondition(ctx, condition ConditionConfig) (bool, error)
```

`ConditionConfig` fields:
- `Type`: `"if_element"`, `"if_text"`, `"if_url"`
- `Selector`: CSS selector (for element/text types)
- `Operator`: `"contains"`, `"equals"`, `"not_contains"`, `"not_equals"`, `"matches_regex"`, `"exists"`, `"not_exists"`
- `Value`: expected value string

---

## Recorder (`recorder/`)

### Recorder Struct

```go
type Recorder struct {
    allocCtx  context.Context      // ExecAllocator context (headed Chrome)
    cancelFn  context.CancelFunc
    steps     []models.RecordedStep
    stepMu    sync.Mutex
    netLogger *logs.NetworkLogger
    wsLogger  *logs.WebSocketLogger
    snapshotter *Snapshotter
    client    CDPClient
}
```

### Recording Session Lifecycle

```
Start(startURL string)
  ├── chromedp.NewExecAllocator(ctx, headless=false, ...)
  ├── chromedp.NewContext(allocCtx) → tab context
  ├── chromedp.Navigate(startURL)
  ├── InjectCaptureScript(ctx, cb) → binds JS → Go CDP binding
  ├── NetworkLogger.Attach(ctx)
  └── WebSocketLogger.Attach(ctx)

(user interacts in visible Chrome window)

Stop() → RecordedFlow{Steps: [...], NetworkLog: [...]}
  └── cancelFn() → Chrome exits
```

### JS Injector (`injector.go`)

`InjectCaptureScript()` uses `runtime.AddBinding` to expose a Go callback to JS, then evaluates a capture script that hooks:

- `document.addEventListener('click', ...)` → `ActionClick` step
- `input.addEventListener('change', ...)` → `ActionType` step  
- `select.addEventListener('change', ...)` → `ActionSelect` step
- `window.addEventListener('popstate', ...)` → `ActionNavigate` step
- `MutationObserver` on `<title>` → tab switch detection

Each event generates a `RecordedStep` with:
- `Action` — what happened
- `Selector` — ranked CSS selector (see `selector.go`)
- `Value` — typed text / URL / option value
- `Timestamp` — epoch ms

### Selector Ranking (`selector.go`)

`rankSelectors(candidates []SelectorCandidate) string`

Scores selector stability:
1. `[data-testid]`, `[data-cy]`, `[aria-label]` → highest score
2. `#id` — high score
3. `.class` — medium
4. Tag-only (`button`, `input`) → low
5. Long XPath chains → lowest

Returns the highest-scored selector string.

### Snapshotter (`snapshot.go`)

```go
type Snapshotter struct{ db *database.DB }

CaptureSnapshot(ctx, flowID, stepIndex int) error
  ├── chromedp.OuterHTML("html") → DOM string → db.SaveDOMSnapshot()
  └── chromedp.FullScreenshot(quality=90) → PNG bytes → db.SaveStepScreenshot()
```

---

## See Also

- [workers.md](workers.md) — Queue submits tasks to Runner
- [integrations.md](integrations.md) — CAPTCHA solver, proxy SOCKS5 gateway
- [database.md](database.md) — DOM snapshots, network logs stored in SQLite
