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

var ErrEvalScriptTooLarge = fmt.Errorf("eval script exceeds maximum allowed size of %s bytes", maxEvalScriptSizeDisplay)

var ErrEvalScriptEmpty = errors.New("eval script must not be empty")

var ErrEvalNotAllowed = errors.New("eval action is not allowed: runner has allowEval=false")

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bchild_process\b`),
	regexp.MustCompile(`(?i)\brequire\s*\(`),
	regexp.MustCompile(`(?i)\bprocess\.exit\b`),
	regexp.MustCompile(`(?i)\bprocess\.env\b`),
	regexp.MustCompile(`(?i)\bfs\s*\.\s*(read|write|unlink|mkdir|rmdir)`),
	regexp.MustCompile(`(?i)\b__dirname\b`),
	regexp.MustCompile(`(?i)\b__filename\b`),
}

func validateEvalScript(script string) error {
	if strings.TrimSpace(script) == "" {
		return ErrEvalScriptEmpty
	}
	if len(script) > MaxEvalScriptSize {
		return ErrEvalScriptTooLarge
	}
	for _, pat := range dangerousPatterns {
		if pat.MatchString(script) {
			return fmt.Errorf("eval script contains blocked pattern: %s", pat.String())
		}
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
	screenshotDir        string
	allowEval            atomic.Bool
	forceHeadless        atomic.Bool
	exec                 Executor
	captchaSolver        captcha.Solver
	pool                 *BrowserPool
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
	return r, nil
}

// SetForceHeadless enforces headless mode on all tasks when enabled.
func (r *Runner) SetForceHeadless(force bool) {
	r.forceHeadless.Store(force)
}

// SetCaptchaSolver sets the CAPTCHA solver used by solve_captcha steps.
func (r *Runner) SetCaptchaSolver(solver captcha.Solver) {
	r.captchaSolver = solver
}

// SetAllowEval configures whether the runner permits eval step execution.
func (r *Runner) SetAllowEval(allow bool) {
	r.allowEval.Store(allow)
}

// SetPool attaches a browser pool for reusing Chrome processes across tasks.
func (r *Runner) SetPool(p *BrowserPool) {
	r.pool = p
}

func (r *Runner) SetLocalProxyManager(m *localproxy.Manager) {
	r.localProxyManager = m
}

func (r *Runner) SetDefaultLoggingPolicy(policy models.TaskLoggingPolicy) {
	r.defaultLoggingPolicy = policy
}

func (r *Runner) resolveLoggingPolicy(task models.Task) resolvedLoggingPolicy {
	resolved := resolvedLoggingPolicy{
		captureStepLogs:    true,
		captureNetworkLogs: true,
		captureScreenshots: true,
		maxExecutionLogs:   1000,
	}
	if r.defaultLoggingPolicy.CaptureStepLogs != nil {
		resolved.captureStepLogs = *r.defaultLoggingPolicy.CaptureStepLogs
	}
	if r.defaultLoggingPolicy.CaptureNetworkLogs != nil {
		resolved.captureNetworkLogs = *r.defaultLoggingPolicy.CaptureNetworkLogs
	}
	if r.defaultLoggingPolicy.CaptureScreenshots != nil {
		resolved.captureScreenshots = *r.defaultLoggingPolicy.CaptureScreenshots
	}
	if r.defaultLoggingPolicy.MaxExecutionLogs > 0 {
		resolved.maxExecutionLogs = r.defaultLoggingPolicy.MaxExecutionLogs
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

	var browserCtx context.Context
	var browserCancel context.CancelFunc
	var poolRelease func()

	effectiveProxy := task.Proxy
	if r.localProxyManager != nil && task.Proxy.Server != "" {
		if localProxy, err := r.localProxyManager.Endpoint(task.Proxy); err == nil {
			effectiveProxy = localProxy
		} else {
			r.addLog(result, "warn", fmt.Sprintf("local proxy endpoint unavailable, using upstream proxy directly: %v", err))
		}
	}

	if r.pool != nil && effectiveProxy.Server == "" {
		var err error
		browserCtx, poolRelease, err = r.pool.Acquire(ctx)
		if err != nil {
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("acquire browser from pool: %v", err)
			r.addLog(result, "error", result.Error)
			return result, err
		}
		defer poolRelease()
		browserCancel = func() {} // no-op; poolRelease handles cleanup
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

	labelIndex := buildLabelIndex(steps)

	type loopFrame struct {
		startPC     int
		maxIter     int
		currentIter int
	}
	var loopStack []loopFrame

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
			maxIter, _ := strconv.Atoi(step.Value)
			if maxIter <= 0 {
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
			return err
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
	result.Logs = append(result.Logs, models.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
	if result.LogLimit > 0 && len(result.Logs) > result.LogLimit {
		result.Logs = append([]models.LogEntry(nil), result.Logs[len(result.Logs)-result.LogLimit:]...)
	}
}

// ClearCookies clears cookies in a browser context.
func ClearCookies(ctx context.Context) error {
	return chromedp.Run(ctx, network.ClearBrowserCookies())
}
