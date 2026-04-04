package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"flowpilot/internal/captcha"
	"flowpilot/internal/localproxy"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const defaultTimeout = 30 * time.Second

const (
	MaxEvalScriptSize        = 10_000
	maxEvalScriptSizeDisplay = "10000"
)

var errEvalScriptTooLarge = fmt.Errorf("eval script exceeds maximum allowed size of %s bytes", maxEvalScriptSizeDisplay)

var errEvalScriptEmpty = errors.New("eval script must not be empty")

var ErrEvalNotAllowed = errors.New("eval action is not allowed: runner has allowEval=false")

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bchild_process\b`),
	regexp.MustCompile(`(?i)\brequire\s*\(`),
	regexp.MustCompile(`(?i)\bprocess\s*\.\s*exit\b`),
	regexp.MustCompile(`(?i)\bprocess\s*\.\s*env\b`),
	regexp.MustCompile(`(?i)\bfs\s*\.\s*(read|write|unlink|mkdir|rmdir)`),
	regexp.MustCompile(`(?i)\b__dirname\b`),
	regexp.MustCompile(`(?i)\b__filename\b`),
}

type debugController struct {
	pauseCh  chan struct{} // closed when resuming
	stepOnce chan struct{} // sends one signal for step
	mu       sync.Mutex
	paused   bool
	pauseNew chan struct{}
}

func newDebugController() *debugController {
	return &debugController{
		pauseNew: make(chan struct{}, 1),
	}
}

func (dc *debugController) pause() {
	dc.mu.Lock()
	if !dc.paused {
		dc.paused = true
		dc.pauseCh = make(chan struct{})
	}
	dc.mu.Unlock()
}

func (dc *debugController) resume() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.paused {
		dc.paused = false
		close(dc.pauseCh)
		dc.pauseCh = nil
	}
}

func (dc *debugController) step() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.paused {
		dc.paused = false
		close(dc.pauseCh)
		dc.pauseCh = nil
	}
}

func (dc *debugController) waitIfPaused(ctx context.Context) error {
	dc.mu.Lock()
	ch := dc.pauseCh
	dc.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

var errEvalScriptTooManyFunctions = errors.New("eval script contains too many nested function declarations (max 5)")

var functionKeywordPattern = regexp.MustCompile(`\bfunction\b`)

func validateEvalScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return errEvalScriptEmpty
	}
	if len(script) > MaxEvalScriptSize {
		return errEvalScriptTooLarge
	}
	for _, pat := range dangerousPatterns {
		if pat.MatchString(script) {
			return fmt.Errorf("eval script contains blocked pattern: %s", pat.String())
		}
	}
	if matches := functionKeywordPattern.FindAllString(script, -1); len(matches) > 5 {
		return errEvalScriptTooManyFunctions
	}
	return nil
}

// Runner executes browser automation tasks using chromedp.
type resolvedLoggingPolicy struct {
	captureStepLogs    bool
	captureNetworkLogs bool
	captureScreenshots bool
	maxExecutionLogs   int
}

type Runner struct {
	screenshotDir string
	allowEval     atomic.Bool
	forceHeadless atomic.Bool
	exec          Executor
	debugCtrl     *debugController

	mu                   sync.Mutex
	captchaSolver        captcha.Solver
	pool                 *BrowserPool
	proxyPools           map[string]*BrowserPool
	localProxyManager    *localproxy.Manager
	defaultLoggingPolicy models.TaskLoggingPolicy
}

// NewRunner creates a new browser runner. Eval steps are blocked by default.
func NewRunner(screenshotDir string) (*Runner, error) {
	if err := os.MkdirAll(screenshotDir, 0o700); err != nil {
		return nil, fmt.Errorf("create screenshot dir: %w", err)
	}
	r := &Runner{screenshotDir: screenshotDir, exec: chromeExecutor{}}
	r.allowEval.Store(false)
	r.debugCtrl = newDebugController()
	return r, nil
}

// SetForceHeadless enforces headless mode on all tasks when enabled.
func (r *Runner) SetForceHeadless(force bool) {
	r.forceHeadless.Store(force)
}

// SetCaptchaSolver sets the CAPTCHA solver used by solve_captcha steps.
func (r *Runner) SetCaptchaSolver(solver captcha.Solver) {
	r.mu.Lock()
	r.captchaSolver = solver
	r.mu.Unlock()
}

// SetAllowEval configures whether the runner permits eval step execution.
func (r *Runner) SetAllowEval(allow bool) {
	r.allowEval.Store(allow)
}

// SetPool attaches a browser pool for reusing Chrome processes across tasks.
func (r *Runner) SetPool(p *BrowserPool) {
	r.stopProxyPools()
	r.mu.Lock()
	r.pool = p
	if r.proxyPools == nil {
		r.proxyPools = make(map[string]*BrowserPool)
	}
	r.mu.Unlock()
}

func (r *Runner) SetLocalProxyManager(m *localproxy.Manager) {
	r.mu.Lock()
	r.localProxyManager = m
	r.mu.Unlock()
}

func (r *Runner) SetDefaultLoggingPolicy(policy models.TaskLoggingPolicy) {
	r.mu.Lock()
	r.defaultLoggingPolicy = policy
	r.mu.Unlock()
}

func (r *Runner) resolveLoggingPolicy(task models.Task) resolvedLoggingPolicy {
	resolved := resolvedLoggingPolicy{
		captureStepLogs:    true,
		captureNetworkLogs: true,
		captureScreenshots: true,
		maxExecutionLogs:   1000,
	}
	r.mu.Lock()
	defaultPolicy := r.defaultLoggingPolicy
	r.mu.Unlock()
	if defaultPolicy.CaptureStepLogs != nil {
		resolved.captureStepLogs = *defaultPolicy.CaptureStepLogs
	}
	if defaultPolicy.CaptureNetworkLogs != nil {
		resolved.captureNetworkLogs = *defaultPolicy.CaptureNetworkLogs
	}
	if defaultPolicy.CaptureScreenshots != nil {
		resolved.captureScreenshots = *defaultPolicy.CaptureScreenshots
	}
	if defaultPolicy.MaxExecutionLogs > 0 {
		resolved.maxExecutionLogs = defaultPolicy.MaxExecutionLogs
	}
	if task.LoggingPolicy != nil {
		if task.LoggingPolicy.CaptureStepLogs != nil {
			resolved.captureStepLogs = *task.LoggingPolicy.CaptureStepLogs
		}
		if task.LoggingPolicy.CaptureNetworkLogs != nil {
			resolved.captureNetworkLogs = *task.LoggingPolicy.CaptureNetworkLogs
		}
		if task.LoggingPolicy.CaptureScreenshots != nil {
			resolved.captureScreenshots = *task.LoggingPolicy.CaptureScreenshots
		}
		if task.LoggingPolicy.MaxExecutionLogs > 0 {
			resolved.maxExecutionLogs = task.LoggingPolicy.MaxExecutionLogs
		}
	}
	return resolved
}

// RunTask executes a single task with its own browser context and proxy.
func (r *Runner) RunTask(ctx context.Context, task models.Task) (*models.TaskResult, error) {
	start := time.Now()
	policy := r.resolveLoggingPolicy(task)
	result := &models.TaskResult{
		TaskID:        task.ID,
		ExtractedData: make(map[string]string),
		LogLimit:      policy.maxExecutionLogs,
	}

	// Recover from chromedp double close panic known upstream bug.
	// Re-panics for any other unexpected panic so the caller sees a real failure.
	defer func() {
		if p := recover(); p != nil {
			err, ok := p.(error)
			if ok && strings.Contains(err.Error(), "close of closed channel") {
				r.addLog(result, "warn", "chromedp upstream panic recovered: close of closed channel")
				return
			}
			panic(p)
		}
	}()

	var browserCtx context.Context
	var browserCancel context.CancelFunc
	var poolRelease func()

	r.mu.Lock()
	basePool := r.pool
	localProxyManager := r.localProxyManager
	r.mu.Unlock()

	effectiveProxy := task.Proxy
	if localProxyManager != nil && task.Proxy.Server != "" {
		if localProxy, err := localProxyManager.Endpoint(task.Proxy); err == nil {
			effectiveProxy = localProxy
		} else {
			r.addLog(result, "warn", fmt.Sprintf("local proxy endpoint unavailable, using upstream proxy directly: %v", err))
		}
	}

	pool, err := r.getPoolForProxy(ctx, effectiveProxy)
	if err != nil {
		result.Duration = time.Since(start)
		result.Error = fmt.Sprintf("resolve browser pool: %v", err)
		r.addLog(result, "error", result.Error)
		return result, err
	}
	if pool != nil {
		browserCtx, poolRelease, err = pool.Acquire(ctx)
		if err != nil {
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("acquire browser from pool: %v", err)
			r.addLog(result, "error", result.Error)
			return result, err
		}
		defer poolRelease()
		browserCancel = func() {} // no-op; poolRelease handles cleanup
		if effectiveProxy.Server != "" {
			r.addLog(result, "info", fmt.Sprintf("using proxy browser pool for %s", effectiveProxy.Server))
		} else if basePool != nil {
			r.addLog(result, "info", "using shared browser pool")
		}
	} else {
		allocCtx, allocCancel := r.createAllocator(ctx, effectiveProxy, task.Headless)
		defer allocCancel()
		browserCtx, browserCancel = chromedp.NewContext(allocCtx)
	}
	defer browserCancel()

	var netLogger *logs.NetworkLogger
	if policy.captureNetworkLogs {
		netLogger = logs.NewNetworkLogger(task.ID)
		chromedp.ListenTarget(browserCtx, func(ev interface{}) {
			switch e := ev.(type) {
			case *network.EventRequestWillBeSent:
				netLogger.HandleRequestWillBeSent(e)
			case *network.EventResponseReceived:
				netLogger.HandleResponseReceived(e)
			case *network.EventLoadingFinished:
				netLogger.HandleLoadingFinished(e, nil)
			case *network.EventLoadingFailed:
				netLogger.HandleLoadingFailed(e.RequestID)
			}
		})

		if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
			r.addLog(result, "warn", fmt.Sprintf("enable network logging: %v", err))
		}
	}

	if err := ClearCookies(browserCtx); err != nil {
		r.addLog(result, "warn", fmt.Sprintf("clear cookies: %v", err))
	}

	if effectiveProxy.Username != "" {
		if err := r.setupProxyAuth(browserCtx, effectiveProxy); err != nil {
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("proxy auth setup failed: %v", err)
			r.addLog(result, "error", result.Error)
			return result, err
		}
	}

	if err := r.runSteps(browserCtx, task.Steps, result, netLogger, policy); err != nil {
		if netLogger != nil {
			result.NetworkLogs = netLogger.Logs()
		}
		result.Duration = time.Since(start)
		return result, err
	}

	if netLogger != nil {
		result.NetworkLogs = netLogger.Logs()
	}
	result.Success = true
	result.Duration = time.Since(start)
	r.addLog(result, "info", fmt.Sprintf("task completed in %s", result.Duration))
	return result, nil
}

// createAllocator builds a chromedp allocator with safe option copying and optional proxy.
func (r *Runner) createAllocator(ctx context.Context, proxyConfig models.ProxyConfig, headless bool) (context.Context, context.CancelFunc) {
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	useHeadless := headless
	if r.forceHeadless.Load() {
		useHeadless = true
	}

	if useHeadless {
		opts = append(opts, chromedp.Headless)
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}
	opts = append(opts,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("js-flags", "--max-old-space-size=512"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
	)

	if proxyConfig.Server != "" {
		proxyAddr := proxyConfig.Server
		if proxyConfig.Protocol != "" && proxyConfig.Protocol != models.ProxyHTTP {
			proxyAddr = string(proxyConfig.Protocol) + "://" + proxyConfig.Server
		}
		opts = append(opts, chromedp.ProxyServer(proxyAddr))
	}

	return chromedp.NewExecAllocator(ctx, opts...)
}

// runSteps executes task steps using a program-counter (PC) based approach
// to support conditional logic, loops, and goto jumps.
func (r *Runner) runSteps(browserCtx context.Context, steps []models.TaskStep, result *models.TaskResult, netLogger *logs.NetworkLogger, policy resolvedLoggingPolicy) error {
	var stepLogger *logs.StepLogger
	if policy.captureStepLogs {
		stepLogger = logs.NewStepLogger(result.TaskID)
		defer func() { result.StepLogs = stepLogger.Logs() }()
	}

	logs.Logger.Debug("running steps",
		"task_id", result.TaskID,
		"step_count", len(steps),
	)

	labelIndex := buildLabelIndex(steps)

	type loopFrame struct {
		startPC     int
		maxIter     int
		currentIter int
	}
	var loopStack []loopFrame

	type whileFrame struct {
		startPC   int
		maxIter   int
		itersDone int
		condition string
	}
	var whileStack []whileFrame

	vars := result.ExtractedData

	pc := 0
	for pc < len(steps) {
		step := steps[pc]
		if netLogger != nil {
			netLogger.SetStepIndex(pc)
		}
		r.addLog(result, "info", fmt.Sprintf("step %d: %s", pc+1, step.Action))

		switch step.Action {
		case models.ActionLoop:
			maxIter, err := strconv.Atoi(step.Value)
			if err != nil || maxIter <= 0 {
				r.addLog(result, "warn", fmt.Sprintf("step %d: invalid loop count %q, defaulting to 100", pc+1, step.Value))
				maxIter = 100
			}
			loopStack = append(loopStack, loopFrame{startPC: pc, maxIter: maxIter, currentIter: 0})
			pc++
			continue

		case models.ActionEndLoop:
			if len(loopStack) == 0 {
				return fmt.Errorf("step %d: end_loop without matching loop", pc+1)
			}
			top := &loopStack[len(loopStack)-1]
			top.currentIter++
			if top.currentIter < top.maxIter {
				pc = top.startPC + 1
				continue
			}
			loopStack = loopStack[:len(loopStack)-1]
			pc++
			continue

		case models.ActionWhile:
			maxIter := step.MaxLoops
			if maxIter <= 0 {
				maxIter = 1000
			}
			condMet, err := r.evaluateCondition(browserCtx, step, vars)
			if err != nil {
				r.addLog(result, "warn", fmt.Sprintf("step %d while condition error: %v", pc+1, err))
				condMet = false
			}
			if condMet {
				whileStack = append(whileStack, whileFrame{startPC: pc, maxIter: maxIter, itersDone: 0, condition: step.Condition})
				pc++
				continue
			}
			endPC := findEndWhile(steps, pc)
			if endPC < 0 {
				return fmt.Errorf("step %d: no matching end_while found", pc+1)
			}
			pc = endPC + 1
			continue

		case models.ActionEndWhile:
			if len(whileStack) == 0 {
				return fmt.Errorf("step %d: end_while without matching while_condition", pc+1)
			}
			top := &whileStack[len(whileStack)-1]
			top.itersDone++
			if top.itersDone >= top.maxIter {
				whileStack = whileStack[:len(whileStack)-1]
				pc++
				continue
			}
			condStep := steps[top.startPC]
			condMet, err := r.evaluateCondition(browserCtx, condStep, vars)
			if err != nil {
				r.addLog(result, "warn", fmt.Sprintf("step %d end_while condition error: %v", pc+1, err))
				condMet = false
			}
			if condMet {
				pc = top.startPC + 1
				continue
			}
			whileStack = whileStack[:len(whileStack)-1]
			pc++
			continue

		case models.ActionBreakLoop:
			if len(loopStack) == 0 {
				return fmt.Errorf("step %d: break_loop without matching loop", pc+1)
			}
			endPC := findEndLoop(steps, loopStack[len(loopStack)-1].startPC)
			if endPC < 0 {
				return fmt.Errorf("step %d: no matching end_loop found", pc+1)
			}
			loopStack = loopStack[:len(loopStack)-1]
			pc = endPC + 1
			continue

		case models.ActionGoto:
			target, ok := labelIndex[step.JumpTo]
			if !ok {
				return fmt.Errorf("step %d: goto label %q not found", pc+1, step.JumpTo)
			}
			pc = target
			continue

		case models.ActionIfElement, models.ActionIfText, models.ActionIfURL:
			condMet, err := r.evaluateCondition(browserCtx, step, vars)
			if err != nil {
				r.addLog(result, "warn", fmt.Sprintf("step %d condition error: %v", pc+1, err))
				condMet = false
			}
			if condMet && step.JumpTo != "" {
				target, ok := labelIndex[step.JumpTo]
				if !ok {
					return fmt.Errorf("step %d: jumpTo label %q not found", pc+1, step.JumpTo)
				}
				pc = target
				continue
			}
			pc++
			continue
		}

		timeout := defaultTimeout
		if step.Timeout > 0 {
			timeout = time.Duration(step.Timeout) * time.Millisecond
		}
		if step.Action == models.ActionScreenshot && !policy.captureScreenshots {
			r.addLog(result, "info", fmt.Sprintf("step %d screenshot skipped by logging policy", pc+1))
			pc++
			continue
		}

		var startedAt time.Time
		if stepLogger != nil {
			startedAt = stepLogger.StartStep(pc, step.Action, step.Selector, step.Value, "")
		}
		stepCtx, stepCancel := context.WithTimeout(browserCtx, timeout)
		err := r.executeStep(stepCtx, step, result)
		stepCancel()

		if stepLogger != nil {
			var code models.ErrorCode
			if err != nil {
				code = models.ClassifyError(err)
			}
			stepLogger.EndStep(pc, step.Action, step.Selector, step.Value, "", startedAt, err, code)
		}

		if err != nil {
			r.addLog(result, "error", fmt.Sprintf("step %d failed: %v", pc+1, err))
			result.Error = fmt.Sprintf("step %d (%s) failed: %v", pc+1, step.Action, err)
			logs.Logger.Error("step failed",
				"task_id", result.TaskID,
				"step_index", pc,
				"action", string(step.Action),
				"error", err.Error(),
			)
			return err
		}

		if r.debugCtrl != nil {
			if err := r.debugCtrl.waitIfPaused(browserCtx); err != nil {
				return err
			}
		}

		if step.Action == models.ActionExtract && step.VarName != "" {
			if val, ok := result.ExtractedData[step.Value]; ok {
				vars[step.VarName] = val
			} else if val, ok := result.ExtractedData[step.Selector]; ok {
				vars[step.VarName] = val
			}
		}

		r.addLog(result, "info", fmt.Sprintf("step %d completed", pc+1))
		pc++
	}
	return nil
}

func (r *Runner) setupProxyAuth(ctx context.Context, proxyConfig models.ProxyConfig) error {
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *fetch.EventAuthRequired:
			go func() {
				select {
				case <-ctx.Done():
					return
				default:
				}
				execCtx := chromedp.FromContext(ctx)
				if execCtx == nil || execCtx.Target == nil {
					return
				}
				c := cdp.WithExecutor(ctx, execCtx.Target)
				if err := fetch.ContinueWithAuth(e.RequestID, &fetch.AuthChallengeResponse{
					Response: fetch.AuthChallengeResponseResponseProvideCredentials,
					Username: proxyConfig.Username,
					Password: proxyConfig.Password,
				}).Do(c); err != nil {
					log.Printf("proxy auth continue failed: %v", err)
				}
			}()
		case *fetch.EventRequestPaused:
			go func() {
				select {
				case <-ctx.Done():
					return
				default:
				}
				execCtx := chromedp.FromContext(ctx)
				if execCtx == nil || execCtx.Target == nil {
					return
				}
				c := cdp.WithExecutor(ctx, execCtx.Target)
				if err := fetch.ContinueRequest(e.RequestID).Do(c); err != nil {
					log.Printf("proxy request continue failed: %v", err)
				}
			}()
		}
	})

	if err := r.exec.Run(ctx, fetch.Enable().WithHandleAuthRequests(true)); err != nil {
		return fmt.Errorf("enable fetch for proxy auth: %w", err)
	}
	return nil
}

func (r *Runner) addLog(result *models.TaskResult, level, message string) {
	timestamp := time.Now()
	result.Logs = append(result.Logs, models.LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
	})
	if result.LogLimit > 0 && len(result.Logs) > result.LogLimit {
		result.Logs = append([]models.LogEntry(nil), result.Logs[len(result.Logs)-result.LogLimit:]...)
	}
	attrs := []any{"task_id", result.TaskID, "level", level, "message", message}
	switch level {
	case "error":
		logs.Logger.Error("task log", attrs...)
	case "warn":
		logs.Logger.Warn("task log", attrs...)
	case "debug":
		logs.Logger.Debug("task log", attrs...)
	default:
		logs.Logger.Info("task log", attrs...)
	}
}

// ClearCookies clears cookies in a browser context.
func ClearCookies(ctx context.Context) error {
	return chromedp.Run(ctx, network.ClearBrowserCookies())
}
