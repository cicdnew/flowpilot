package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"flowpilot/internal/logs"
	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const defaultTimeout = 30 * time.Second

const (
	// MaxEvalScriptSize is the maximum allowed length of an eval script in bytes.
	MaxEvalScriptSize = 10_000

	// maxEvalScriptSizeDisplay is used in error messages.
	maxEvalScriptSizeDisplay = "10000"
)

// ErrEvalScriptTooLarge is returned when an eval script exceeds MaxEvalScriptSize.
var ErrEvalScriptTooLarge = fmt.Errorf("eval script exceeds maximum allowed size of %s bytes", maxEvalScriptSizeDisplay)

// ErrEvalScriptEmpty is returned when an eval script is empty.
var ErrEvalScriptEmpty = errors.New("eval script must not be empty")

// dangerousPatterns are blocked in eval scripts to reduce sandbox-escape risk.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bchild_process\b`),
	regexp.MustCompile(`(?i)\brequire\s*\(`),
	regexp.MustCompile(`(?i)\bprocess\.exit\b`),
	regexp.MustCompile(`(?i)\bprocess\.env\b`),
	regexp.MustCompile(`(?i)\bfs\s*\.\s*(read|write|unlink|mkdir|rmdir)`),
	regexp.MustCompile(`(?i)\b__dirname\b`),
	regexp.MustCompile(`(?i)\b__filename\b`),
}

// validateEvalScript checks an eval script for size, emptiness, and dangerous patterns.
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
type Runner struct {
	screenshotDir string
	allowEval     atomic.Bool
	forceHeadless atomic.Bool
	exec          Executor
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

// SetAllowEval configures whether the runner permits eval step execution.
func (r *Runner) SetAllowEval(allow bool) {
	r.allowEval.Store(allow)
}

// RunTask executes a single task with its own browser context and proxy.
func (r *Runner) RunTask(ctx context.Context, task models.Task) (*models.TaskResult, error) {
	start := time.Now()
	result := &models.TaskResult{
		TaskID:        task.ID,
		ExtractedData: make(map[string]string),
	}

	allocCtx, allocCancel := r.createAllocator(ctx, task.Proxy, task.Headless)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	netLogger := logs.NewNetworkLogger(task.ID)
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

	if err := ClearCookies(browserCtx); err != nil {
		r.addLog(result, "warn", fmt.Sprintf("clear cookies: %v", err))
	}

	if task.Proxy.Username != "" {
		if err := r.setupProxyAuth(browserCtx, task.Proxy); err != nil {
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("proxy auth setup failed: %v", err)
			r.addLog(result, "error", result.Error)
			return result, err
		}
	}

	if err := r.runSteps(browserCtx, task.Steps, result, netLogger); err != nil {
		result.NetworkLogs = netLogger.Logs()
		result.Duration = time.Since(start)
		return result, err
	}

	result.NetworkLogs = netLogger.Logs()
	result.Success = true
	result.Duration = time.Since(start)
	r.addLog(result, "info", fmt.Sprintf("task completed in %s", result.Duration))
	return result, nil
}

// createAllocator builds a chromedp allocator with safe option copying and optional proxy.
// The headless parameter respects the task's preference unless forceHeadless is enabled.
func (r *Runner) createAllocator(ctx context.Context, proxyConfig models.ProxyConfig, headless bool) (context.Context, context.CancelFunc) {
	// Copy default options to avoid mutating the shared slice.
	opts := make([]chromedp.ExecAllocatorOption, len(chromedp.DefaultExecAllocatorOptions))
	copy(opts, chromedp.DefaultExecAllocatorOptions[:])

	// Respect forceHeadless override; otherwise use the task's headless preference.
	useHeadless := headless
	if r.forceHeadless.Load() {
		useHeadless = true
	}

	opts = append(opts,
		chromedp.Flag("headless", useHeadless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
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

// runSteps iterates through each task step and executes it.
func (r *Runner) runSteps(browserCtx context.Context, steps []models.TaskStep, result *models.TaskResult, netLogger *logs.NetworkLogger) error {
	stepLogger := logs.NewStepLogger(result.TaskID)
	defer func() { result.StepLogs = stepLogger.Logs() }()

	for i, step := range steps {
		netLogger.SetStepIndex(i)
		r.addLog(result, "info", fmt.Sprintf("step %d: %s", i+1, step.Action))

		timeout := defaultTimeout
		if step.Timeout > 0 {
			timeout = time.Duration(step.Timeout) * time.Millisecond
		}

		start := stepLogger.StartStep(i, step.Action, step.Selector, step.Value, "")

		stepCtx, stepCancel := context.WithTimeout(browserCtx, timeout)
		err := r.executeStep(stepCtx, step, result)
		stepCancel()

		var code models.ErrorCode
		if err != nil {
			code = models.ClassifyError(err)
		}
		stepLogger.EndStep(i, step.Action, step.Selector, step.Value, "", start, err, code)

		if err != nil {
			r.addLog(result, "error", fmt.Sprintf("step %d failed: %v", i+1, err))
			result.Error = fmt.Sprintf("step %d (%s) failed: %v", i+1, step.Action, err)
			return err
		}

		r.addLog(result, "info", fmt.Sprintf("step %d completed", i+1))
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

// executeStep dispatches to the appropriate action handler.
func (r *Runner) executeStep(ctx context.Context, step models.TaskStep, result *models.TaskResult) error {
	switch step.Action {
	case models.ActionNavigate:
		return r.execNavigate(ctx, step)
	case models.ActionClick:
		return r.execClick(ctx, step)
	case models.ActionType:
		return r.execType(ctx, step)
	case models.ActionWait:
		return r.execWait(ctx, step)
	case models.ActionScreenshot:
		return r.execScreenshot(ctx, result)
	case models.ActionExtract:
		return r.execExtract(ctx, step, result)
	case models.ActionScroll:
		return r.execScroll(ctx, step)
	case models.ActionSelect:
		return r.execSelect(ctx, step)
	case models.ActionEval:
		return r.execEval(ctx, step)
	case models.ActionTabSwitch:
		return r.execTabSwitch(ctx, step)
	default:
		return fmt.Errorf("unknown action: %s", step.Action)
	}
}

func (r *Runner) execNavigate(ctx context.Context, step models.TaskStep) error {
	return r.exec.Run(ctx, chromedp.Navigate(step.Value))
}

func (r *Runner) execClick(ctx context.Context, step models.TaskStep) error {
	return r.exec.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.Click(step.Selector, chromedp.ByQuery),
	)
}

func (r *Runner) execType(ctx context.Context, step models.TaskStep) error {
	return r.exec.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.Clear(step.Selector, chromedp.ByQuery),
		chromedp.SendKeys(step.Selector, step.Value, chromedp.ByQuery),
	)
}

func (r *Runner) execWait(ctx context.Context, step models.TaskStep) error {
	if step.Selector != "" {
		return r.exec.Run(ctx,
			chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		)
	}
	dur, err := time.ParseDuration(step.Value + "ms")
	if err != nil {
		dur = 1 * time.Second
	}
	// Respect context cancellation during wait.
	timer := time.NewTimer(dur)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) execScreenshot(ctx context.Context, result *models.TaskResult) error {
	var buf []byte
	if err := r.exec.Run(ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return fmt.Errorf("capture screenshot: %w", err)
	}
	sanitizedID := sanitizeFilename(result.TaskID)
	filename := fmt.Sprintf("%s_%d.png", sanitizedID, time.Now().UnixMilli())
	path := filepath.Join(r.screenshotDir, filename)
	if !strings.HasPrefix(path, filepath.Clean(r.screenshotDir)+string(os.PathSeparator)) {
		return fmt.Errorf("screenshot path escapes screenshot directory")
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("save screenshot: %w", err)
	}
	result.Screenshots = append(result.Screenshots, path)
	return nil
}

func sanitizeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '.' || r == '\x00' {
			return '_'
		}
		return r
	}, name)
	return filepath.Base(safe)
}

func (r *Runner) execExtract(ctx context.Context, step models.TaskStep, result *models.TaskResult) error {
	var text string
	if err := r.exec.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.Text(step.Selector, &text, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("extract text: %w", err)
	}
	key := step.Value
	if key == "" {
		key = step.Selector
	}
	result.ExtractedData[key] = text
	return nil
}

func (r *Runner) execScroll(ctx context.Context, step models.TaskStep) error {
	// Validate the scroll value is a number to prevent JS injection.
	if _, err := strconv.Atoi(step.Value); err != nil {
		return fmt.Errorf("invalid scroll value %q: must be an integer", step.Value)
	}
	return r.exec.Run(ctx,
		chromedp.Evaluate(`window.scrollBy(0, `+step.Value+`)`, nil),
	)
}

func (r *Runner) execSelect(ctx context.Context, step models.TaskStep) error {
	return r.exec.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.SetValue(step.Selector, step.Value, chromedp.ByQuery),
	)
}

var ErrEvalNotAllowed = errors.New("eval action is not allowed: runner has allowEval=false")

func (r *Runner) execEval(ctx context.Context, step models.TaskStep) error {
	if !r.allowEval.Load() {
		return ErrEvalNotAllowed
	}
	if err := validateEvalScript(step.Value); err != nil {
		return fmt.Errorf("eval validation failed: %w", err)
	}
	var res any
	return r.exec.Run(ctx,
		chromedp.Evaluate(step.Value, &res),
	)
}

func (r *Runner) execTabSwitch(ctx context.Context, step models.TaskStep) error {
	targets, err := r.exec.Targets(ctx)
	if err != nil {
		return fmt.Errorf("list targets: %w", err)
	}
	for _, t := range targets {
		if t.Type == "page" && t.URL == step.Value {
			return r.exec.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
				return target.ActivateTarget(t.TargetID).Do(c)
			}))
		}
	}
	return fmt.Errorf("tab with URL %q not found", step.Value)
}

func (r *Runner) addLog(result *models.TaskResult, level, message string) {
	result.Logs = append(result.Logs, models.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
}

// ClearCookies clears cookies in a browser context.
func ClearCookies(ctx context.Context) error {
	return chromedp.Run(ctx, network.ClearBrowserCookies())
}
