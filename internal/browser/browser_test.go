package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"flowpilot/internal/captcha"
	"flowpilot/internal/localproxy"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func TestNewRunner(t *testing.T) {
	dir := t.TempDir()
	screenshotDir := filepath.Join(dir, "screenshots")

	runner, err := NewRunner(screenshotDir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	if runner.screenshotDir != screenshotDir {
		t.Errorf("screenshotDir: got %q, want %q", runner.screenshotDir, screenshotDir)
	}

	// Verify directory was created
	info, err := os.Stat(screenshotDir)
	if err != nil {
		t.Fatalf("screenshot dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("screenshot path is not a directory")
	}
}

func TestNewRunnerExistingDir(t *testing.T) {
	dir := t.TempDir()
	screenshotDir := filepath.Join(dir, "screenshots")
	os.MkdirAll(screenshotDir, 0o755)

	runner, err := NewRunner(screenshotDir)
	if err != nil {
		t.Fatalf("NewRunner with existing dir: %v", err)
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}
}

func TestNewRunnerInvalidPath(t *testing.T) {
	// Create a file where we'd want a directory
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	os.WriteFile(filePath, []byte("data"), 0o644)

	_, err := NewRunner(filepath.Join(filePath, "screenshots"))
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestAddLog(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID: "test-log",
	}

	runner.addLog(result, "info", "test message 1")
	runner.addLog(result, "error", "test error")
	runner.addLog(result, "info", "test message 2")

	if len(result.Logs) != 3 {
		t.Fatalf("log count: got %d, want 3", len(result.Logs))
	}

	if result.Logs[0].Level != "info" {
		t.Errorf("log[0].Level: got %q, want %q", result.Logs[0].Level, "info")
	}
	if result.Logs[0].Message != "test message 1" {
		t.Errorf("log[0].Message: got %q, want %q", result.Logs[0].Message, "test message 1")
	}
	if result.Logs[1].Level != "error" {
		t.Errorf("log[1].Level: got %q, want %q", result.Logs[1].Level, "error")
	}

	// Timestamps should be ordered
	for i := 1; i < len(result.Logs); i++ {
		if result.Logs[i].Timestamp.Before(result.Logs[i-1].Timestamp) {
			t.Error("log timestamps are not ordered")
		}
	}
}

func TestExecuteStepUnknownAction(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "test-unknown",
		ExtractedData: make(map[string]string),
	}

	step := models.TaskStep{
		Action: "nonexistent_action",
	}

	err := runner.executeStep(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if err.Error() != "unknown action: nonexistent_action" {
		t.Errorf("error message: got %q", err.Error())
	}
}

func TestExecScrollValidation(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid positive", "500", false},
		{"valid negative", "-200", false},
		{"valid zero", "0", false},
		{"invalid text", "abc", true},
		{"invalid float", "1.5", true},
		{"invalid empty", "", true},
		{"invalid js injection", "0); alert('xss", true},
		{"invalid semicolon", "100; document.cookie", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			step := models.TaskStep{
				Action: models.ActionScroll,
				Value:  tc.value,
			}

			// Note: execScroll will validate the value, but will fail on chromedp.Run
			// in a test environment without a browser. We test that validation catches
			// injection attempts before reaching chromedp.
			err := runner.execScroll(context.Background(), step)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			}
			// For valid values, we expect a chromedp error (no browser),
			// NOT a validation error
			if !tc.wantErr && err != nil {
				// The error should be from chromedp, not from our validation
				if err.Error() == "invalid scroll value \""+tc.value+"\": must be an integer" {
					t.Errorf("got unexpected validation error for valid value %q", tc.value)
				}
			}
		})
	}
}

func TestExecWaitContextCancellation(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	ctx, cancel := context.WithCancel(context.Background())

	step := models.TaskStep{
		Action: models.ActionWait,
		Value:  "10000", // 10 seconds
	}

	done := make(chan error, 1)
	go func() {
		done <- runner.execWait(ctx, step)
	}()

	// Cancel after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected context.Canceled error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execWait did not respect context cancellation")
	}
}

func TestExecWaitWithDuration(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	step := models.TaskStep{
		Action: models.ActionWait,
		Value:  "100", // 100ms
	}

	start := time.Now()
	err := runner.execWait(context.Background(), step)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("execWait: %v", err)
	}

	if elapsed < 90*time.Millisecond {
		t.Errorf("wait too short: %v", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("wait too long: %v", elapsed)
	}
}

func TestExecWaitWithSelector(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	step := models.TaskStep{
		Action:   models.ActionWait,
		Selector: "#element",
	}

	// With a selector, it tries to use chromedp which will fail without a browser
	err := runner.execWait(context.Background(), step)
	if err == nil {
		t.Error("expected error when running chromedp without browser")
	}
}

func TestExecWaitInvalidDuration(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	step := models.TaskStep{
		Action: models.ActionWait,
		Value:  "not-a-number",
	}

	// Should default to 1 second and not error
	start := time.Now()
	err := runner.execWait(context.Background(), step)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("execWait with invalid duration: %v", err)
	}
	// Should default to 1 second
	if elapsed < 900*time.Millisecond {
		t.Errorf("should default to 1s, got %v", elapsed)
	}
}

func TestRunStepsEmptySteps(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "empty-steps",
		ExtractedData: make(map[string]string),
	}

	nl := logs.NewNetworkLogger(result.TaskID)
	err := runner.runSteps(context.Background(), nil, result, nl, runner.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("runSteps with nil steps: %v", err)
	}

	err = runner.runSteps(context.Background(), []models.TaskStep{}, result, nl, runner.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("runSteps with empty steps: %v", err)
	}
}

func TestRunStepsStopsOnError(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "stop-on-error",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: "invalid_action_1"},
		{Action: "invalid_action_2"},
	}

	err := runner.runSteps(context.Background(), steps, result, logs.NewNetworkLogger(result.TaskID), runner.resolveLoggingPolicy(models.Task{}))
	if err == nil {
		t.Fatal("expected error from invalid steps")
	}

	// Should have logged the attempt and failure of step 1
	// but NOT attempted step 2
	infoCount := 0
	errorCount := 0
	for _, log := range result.Logs {
		if log.Level == "info" {
			infoCount++
		}
		if log.Level == "error" {
			errorCount++
		}
	}

	// 1 info (start of step 1) + 1 error (failure of step 1)
	if infoCount != 1 {
		t.Errorf("expected 1 info log, got %d", infoCount)
	}
	if errorCount != 1 {
		t.Errorf("expected 1 error log, got %d", errorCount)
	}
}

func TestCreateAllocatorWithoutProxy(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	ctx := context.Background()
	allocCtx, allocCancel := runner.createAllocator(ctx, models.ProxyConfig{}, true)
	defer allocCancel()

	if allocCtx == nil {
		t.Fatal("allocator context is nil")
	}
}

func TestCreateAllocatorWithProxy(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	proxy := models.ProxyConfig{
		Server:   "proxy.example.com:8080",
		Username: "user",
		Password: "pass",
	}

	ctx := context.Background()
	allocCtx, allocCancel := runner.createAllocator(ctx, proxy, true)
	defer allocCancel()

	if allocCtx == nil {
		t.Fatal("allocator context is nil")
	}
}

func TestCreateAllocatorDoesNotMutateDefaults(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	originalLen := len(chromedp.DefaultExecAllocatorOptions)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, cancel := runner.createAllocator(ctx, models.ProxyConfig{
			Server: "proxy.example.com:8080",
		}, true)
		cancel()
	}

	if len(chromedp.DefaultExecAllocatorOptions) != originalLen {
		t.Errorf("DefaultExecAllocatorOptions mutated: original len %d, now %d",
			originalLen, len(chromedp.DefaultExecAllocatorOptions))
	}
}

func TestExecuteStepDispatchesCorrectly(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "dispatch-test",
		ExtractedData: make(map[string]string),
	}

	// All valid actions should not return "unknown action" error.
	// They will fail with chromedp errors (no browser), but that's expected.
	validActions := []models.StepAction{
		models.ActionNavigate,
		models.ActionClick,
		models.ActionType,
		models.ActionScreenshot,
		models.ActionExtract,
		models.ActionSelect,
		models.ActionEval,
	}

	for _, action := range validActions {
		step := models.TaskStep{
			Action:   action,
			Selector: "#test",
			Value:    "test-value",
		}
		err := runner.executeStep(context.Background(), step, result)
		if err != nil && err.Error() == "unknown action: "+string(action) {
			t.Errorf("action %s was not dispatched correctly", action)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"normal uuid", "550e8400-e29b-41d4-a716-446655440000"},
		{"path traversal", "../../etc/cron.d/evil"},
		{"backslash traversal", `..\..\windows\system32\evil`},
		{"dotdot only", ".."},
		{"slashes in middle", "foo/bar/baz"},
		{"null byte", "task\x00id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeFilename(tc.input)
			if strings.Contains(result, "/") || strings.Contains(result, "\\") || strings.Contains(result, "..") {
				t.Errorf("SanitizeFilename(%q) = %q, still contains path components", tc.input, result)
			}
			if result == "" {
				t.Errorf("SanitizeFilename(%q) returned empty string", tc.input)
			}
		})
	}
}

func TestExecScreenshotPathTraversal(t *testing.T) {
	dir := t.TempDir()
	_ = &Runner{screenshotDir: dir, exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "../../etc/cron.d/evil",
		ExtractedData: make(map[string]string),
	}

	filename := SanitizeFilename(result.TaskID)
	fullPath := filepath.Join(dir, filename+"_test.png")

	if !strings.HasPrefix(fullPath, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatal("sanitized path should stay within screenshot directory")
	}
	if strings.Contains(filename, "..") {
		t.Errorf("sanitized filename still contains path traversal: %q", filename)
	}
}

func TestExecEvalBlockedByDefault(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	step := models.TaskStep{
		Action: models.ActionEval,
		Value:  "document.title",
	}

	err := runner.execEval(context.Background(), step)
	if err == nil {
		t.Fatal("expected error when eval is not allowed")
	}
	if err != ErrEvalNotAllowed {
		t.Errorf("expected ErrEvalNotAllowed, got: %v", err)
	}
}

func TestExecEvalAllowedWhenEnabled(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	runner.allowEval.Store(true)

	step := models.TaskStep{
		Action: models.ActionEval,
		Value:  "1 + 1",
	}

	err := runner.execEval(context.Background(), step)
	if err == ErrEvalNotAllowed {
		t.Fatal("eval should be allowed when allowEval is true")
	}
}

func TestSetAllowEval(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	if runner.allowEval.Load() {
		t.Fatal("allowEval should default to false")
	}

	runner.SetAllowEval(true)
	if !runner.allowEval.Load() {
		t.Fatal("allowEval should be true after SetAllowEval(true)")
	}

	runner.SetAllowEval(false)
	if runner.allowEval.Load() {
		t.Fatal("allowEval should be false after SetAllowEval(false)")
	}
}

func TestValidateEvalScript(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantErr bool
		errMsg  string
	}{
		{"valid simple expression", "1 + 1", false, ""},
		{"valid DOM access", "document.querySelector('#test').textContent", false, ""},
		{"valid JSON extraction", "JSON.stringify(window.performance.timing)", false, ""},
		{"empty string", "", true, "eval script must not be empty"},
		{"whitespace only", "   \t\n  ", true, "eval script must not be empty"},
		{"too large", strings.Repeat("a", MaxEvalScriptSize+1), true, "eval script exceeds maximum allowed size"},
		{"exactly max size", strings.Repeat("a", MaxEvalScriptSize), false, ""},
		{"blocked require", "const fs = require('fs')", true, "blocked pattern"},
		{"blocked require spaced", "require ( 'child_process' )", true, "blocked pattern"},
		{"blocked process.exit", "process.exit(1)", true, "blocked pattern"},
		{"blocked process.exit spaced", "process . exit(1)", true, "blocked pattern"},
		{"blocked process.env", "console.log(process.env.SECRET)", true, "blocked pattern"},
		{"blocked process.env spaced", "console.log(process . env.SECRET)", true, "blocked pattern"},
		{"blocked child_process", "child_process.exec('ls')", true, "blocked pattern"},
		{"blocked fs.readFile", "fs.readFile('/etc/passwd')", true, "blocked pattern"},
		{"blocked fs.writeFile", "fs.writeFile('/tmp/x', 'data')", true, "blocked pattern"},
		{"blocked __dirname", "console.log(__dirname)", true, "blocked pattern"},
		{"blocked __filename", "console.log(__filename)", true, "blocked pattern"},
		{"case insensitive require", "REQUIRE('fs')", true, "blocked pattern"},
		{"case insensitive process", "Process.Exit(0)", true, "blocked pattern"},
		{"five functions allowed", "function a(){}function b(){}function c(){}function d(){}function e(){}", false, ""},
		{"six functions rejected", "function a(){}function b(){}function c(){}function d(){}function e(){}function f(){}", true, "too many nested function"},
		{"exactly six rejected", strings.Repeat("function(){} ", 6), true, "too many nested function"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEvalScript(tc.script)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tc.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExecEvalValidationIntegration(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	runner.allowEval.Store(true)

	// Blocked pattern should be rejected even when eval is allowed
	step := models.TaskStep{
		Action: models.ActionEval,
		Value:  "require('child_process').exec('ls')",
	}
	err := runner.execEval(context.Background(), step)
	if err == nil {
		t.Fatal("expected validation error for dangerous pattern")
	}
	if !strings.Contains(err.Error(), "eval validation failed") {
		t.Errorf("expected 'eval validation failed' error, got: %v", err)
	}

	// Empty script should be rejected
	step.Value = ""
	err = runner.execEval(context.Background(), step)
	if err == nil {
		t.Fatal("expected validation error for empty script")
	}
	if !strings.Contains(err.Error(), "eval validation failed") {
		t.Errorf("expected 'eval validation failed' error, got: %v", err)
	}
}

func TestExecEvalReenabledAfterDisable(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	// Enable then disable
	runner.SetAllowEval(true)
	runner.SetAllowEval(false)

	step := models.TaskStep{
		Action: models.ActionEval,
		Value:  "document.title",
	}
	err := runner.execEval(context.Background(), step)
	if err != ErrEvalNotAllowed {
		t.Errorf("expected ErrEvalNotAllowed after re-disabling, got: %v", err)
	}
}

func TestCreateAllocatorRespectsHeadless(t *testing.T) {
	tests := []struct {
		name          string
		taskHeadless  bool
		forceHeadless bool
	}{
		{"headless true, force false", true, false},
		{"headless false, force false", false, false},
		{"headless false, force true overrides", false, true},
		{"headless true, force true", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
			runner.forceHeadless.Store(tc.forceHeadless)

			ctx := context.Background()
			allocCtx, allocCancel := runner.createAllocator(ctx, models.ProxyConfig{}, tc.taskHeadless)
			defer allocCancel()

			if allocCtx == nil {
				t.Fatal("allocator context should not be nil")
			}
			// Verify the allocator was created successfully with no panic.
			// The headless flag is set inside chromedp options which are not directly inspectable,
			// but we verify the code path doesn't error.
		})
	}
}

func TestSetForceHeadless(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	if runner.forceHeadless.Load() {
		t.Fatal("forceHeadless should default to false")
	}

	runner.SetForceHeadless(true)
	if !runner.forceHeadless.Load() {
		t.Fatal("forceHeadless should be true after SetForceHeadless(true)")
	}

	runner.SetForceHeadless(false)
	if runner.forceHeadless.Load() {
		t.Fatal("forceHeadless should be false after SetForceHeadless(false)")
	}
}

type mockExecutor struct {
	mu          sync.Mutex
	calls       []string
	runErr      error
	targErr     error
	targets     []*target.Info
	runResponse *network.Response
	runRespErr  error
}

func (m *mockExecutor) Run(ctx context.Context, actions ...chromedp.Action) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("Run(%d actions)", len(actions)))
	return m.runErr
}

func (m *mockExecutor) RunResponse(ctx context.Context, actions ...chromedp.Action) (*network.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("RunResponse(%d actions)", len(actions)))
	if m.runRespErr != nil {
		return nil, m.runRespErr
	}
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.runResponse, nil
}

func (m *mockExecutor) Targets(ctx context.Context) ([]*target.Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "Targets")
	return m.targets, m.targErr
}

func (m *mockExecutor) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func newMockRunner(t *testing.T, exec *mockExecutor) *Runner {
	t.Helper()
	return &Runner{screenshotDir: t.TempDir(), exec: exec}
}

func TestExecNavigateWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Action: models.ActionNavigate, Value: "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecNavigateError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("nav failed")}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err == nil || err.Error() != "nav failed" {
		t.Fatalf("expected 'nav failed', got: %v", err)
	}
}

func TestExecClickWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execClick(context.Background(), models.TaskStep{Selector: "#btn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecClickError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("click failed")}
	r := newMockRunner(t, mock)

	err := r.execClick(context.Background(), models.TaskStep{Selector: "#btn"})
	if err == nil || err.Error() != "click failed" {
		t.Fatalf("expected 'click failed', got: %v", err)
	}
}

func TestExecTypeWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execType(context.Background(), models.TaskStep{Selector: "#input", Value: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecTypeError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("type failed")}
	r := newMockRunner(t, mock)

	err := r.execType(context.Background(), models.TaskStep{Selector: "#input", Value: "hello"})
	if err == nil || err.Error() != "type failed" {
		t.Fatalf("expected 'type failed', got: %v", err)
	}
}

func TestExecSelectWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execSelect(context.Background(), models.TaskStep{Selector: "select#opt", Value: "val1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecSelectError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("select failed")}
	r := newMockRunner(t, mock)

	err := r.execSelect(context.Background(), models.TaskStep{Selector: "select#opt", Value: "val1"})
	if err == nil || err.Error() != "select failed" {
		t.Fatalf("expected 'select failed', got: %v", err)
	}
}

func TestExecScrollWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execScroll(context.Background(), models.TaskStep{Value: "500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecScrollError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("scroll failed")}
	r := newMockRunner(t, mock)

	err := r.execScroll(context.Background(), models.TaskStep{Value: "500"})
	if err == nil || err.Error() != "scroll failed" {
		t.Fatalf("expected 'scroll failed', got: %v", err)
	}
}

func TestExecEvalWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.allowEval.Store(true)

	err := r.execEval(context.Background(), models.TaskStep{Value: "1+1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecEvalError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("eval failed")}
	r := newMockRunner(t, mock)
	r.allowEval.Store(true)

	err := r.execEval(context.Background(), models.TaskStep{Value: "1+1"})
	if err == nil || err.Error() != "eval failed" {
		t.Fatalf("expected 'eval failed', got: %v", err)
	}
}

func TestExecScreenshotWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "test-ss", ExtractedData: make(map[string]string)}

	err := r.execScreenshot(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Screenshots) != 1 {
		t.Fatalf("expected 1 screenshot path, got %d", len(result.Screenshots))
	}
	if !strings.HasPrefix(result.Screenshots[0], r.screenshotDir) {
		t.Errorf("screenshot path %q not under %q", result.Screenshots[0], r.screenshotDir)
	}
}

func TestExecScreenshotRunError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("screenshot failed")}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "test-ss-err", ExtractedData: make(map[string]string)}

	err := r.execScreenshot(context.Background(), result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "capture screenshot") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestExecScreenshotPathTraversalSanitized(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "../../etc/evil", ExtractedData: make(map[string]string)}

	err := r.execScreenshot(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Screenshots) != 1 {
		t.Fatal("expected screenshot to be saved")
	}
	if !strings.HasPrefix(result.Screenshots[0], r.screenshotDir) {
		t.Errorf("path escaped screenshot dir: %s", result.Screenshots[0])
	}
}

func TestExecExtractWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "test-extract", ExtractedData: make(map[string]string)}

	err := r.execExtract(context.Background(), models.TaskStep{Selector: "#title", Value: "pageTitle"}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.ExtractedData["pageTitle"]; !ok {
		t.Error("expected 'pageTitle' key in extracted data")
	}
}

func TestExecExtractDefaultKey(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "test-extract-key", ExtractedData: make(map[string]string)}

	err := r.execExtract(context.Background(), models.TaskStep{Selector: "#title", Value: ""}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.ExtractedData["#title"]; !ok {
		t.Error("expected selector as key when value is empty")
	}
}

func TestExecExtractError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("extract failed")}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "test-extract-err", ExtractedData: make(map[string]string)}

	err := r.execExtract(context.Background(), models.TaskStep{Selector: "#title", Value: "key"}, result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "extract text") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestExecTabSwitchFound(t *testing.T) {
	mock := &mockExecutor{
		targets: []*target.Info{
			{Type: "page", URL: "https://example.com"},
			{Type: "page", URL: "https://other.com"},
		},
	}
	r := newMockRunner(t, mock)

	err := r.execTabSwitch(context.Background(), models.TaskStep{Value: "https://other.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 2 {
		t.Fatalf("expected 2 calls (Targets + Run), got %d", mock.callCount())
	}
}

func TestExecTabSwitchNotFound(t *testing.T) {
	mock := &mockExecutor{
		targets: []*target.Info{
			{Type: "page", URL: "https://example.com"},
		},
	}
	r := newMockRunner(t, mock)

	err := r.execTabSwitch(context.Background(), models.TaskStep{Value: "https://missing.com"})
	if err == nil {
		t.Fatal("expected error for missing tab")
	}
	if !strings.Contains(err.Error(), "tab with URL") {
		t.Errorf("expected 'tab with URL' error, got: %v", err)
	}
}

func TestExecTabSwitchTargetsError(t *testing.T) {
	mock := &mockExecutor{targErr: errors.New("targets failed")}
	r := newMockRunner(t, mock)

	err := r.execTabSwitch(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list targets") {
		t.Errorf("expected 'list targets' error, got: %v", err)
	}
}

func TestExecTabSwitchSkipsNonPage(t *testing.T) {
	mock := &mockExecutor{
		targets: []*target.Info{
			{Type: "background_page", URL: "https://example.com"},
			{Type: "service_worker", URL: "https://example.com"},
		},
	}
	r := newMockRunner(t, mock)

	err := r.execTabSwitch(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err == nil {
		t.Fatal("expected not-found error when only non-page targets exist")
	}
}

func TestRunStepsWithMockSuccess(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "steps-ok", ExtractedData: make(map[string]string)}

	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#btn"},
		{Action: models.ActionType, Selector: "#input", Value: "hello"},
	}

	err := r.runSteps(context.Background(), steps, result, logs.NewNetworkLogger(result.TaskID), r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 3 {
		t.Fatalf("expected 3 executor calls, got %d", mock.callCount())
	}
	if len(result.Logs) != 6 {
		t.Fatalf("expected 6 log entries (start+end per step), got %d", len(result.Logs))
	}
}

func TestRunStepsWithMockStopsOnError(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "steps-err", ExtractedData: make(map[string]string)}

	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: "bad_action"},
		{Action: models.ActionClick, Selector: "#btn"},
	}

	err := r.runSteps(context.Background(), steps, result, logs.NewNetworkLogger(result.TaskID), r.resolveLoggingPolicy(models.Task{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 executor call (only first step), got %d", mock.callCount())
	}
}

func TestRunStepsCustomTimeout(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "steps-timeout", ExtractedData: make(map[string]string)}

	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com", Timeout: 5000},
	}

	err := r.runSteps(context.Background(), steps, result, logs.NewNetworkLogger(result.TaskID), r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddLogRespectsLimit(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{TaskID: "capped-log", LogLimit: 2}

	runner.addLog(result, "info", "first")
	runner.addLog(result, "info", "second")
	runner.addLog(result, "info", "third")

	if len(result.Logs) != 2 {
		t.Fatalf("log count: got %d, want 2", len(result.Logs))
	}
	if result.Logs[0].Message != "second" || result.Logs[1].Message != "third" {
		t.Fatalf("expected last two logs to be retained, got %#v", result.Logs)
	}
}

func TestRunStepsSkipsScreenshotWhenDisabled(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "skip-ss", ExtractedData: make(map[string]string)}
	captureScreenshots := false
	policy := r.resolveLoggingPolicy(models.Task{LoggingPolicy: &models.TaskLoggingPolicy{CaptureScreenshots: &captureScreenshots}})

	err := r.runSteps(context.Background(), []models.TaskStep{{Action: models.ActionScreenshot}}, result, logs.NewNetworkLogger(result.TaskID), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 0 {
		t.Fatalf("expected no executor calls when screenshot capture is disabled, got %d", mock.callCount())
	}
	if len(result.Screenshots) != 0 {
		t.Fatalf("expected no screenshots, got %d", len(result.Screenshots))
	}
}

func TestSetupProxyAuthWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	allocCtx, allocCancel := r.createAllocator(context.Background(), models.ProxyConfig{Server: "proxy:8080"}, true)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	err := r.setupProxyAuth(browserCtx, models.ProxyConfig{
		Server:   "proxy:8080",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 Run call for fetch.Enable, got %d", mock.callCount())
	}
}

func TestSetupProxyAuthError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("fetch enable failed")}
	r := newMockRunner(t, mock)

	allocCtx, allocCancel := r.createAllocator(context.Background(), models.ProxyConfig{Server: "proxy:8080"}, true)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	err := r.setupProxyAuth(browserCtx, models.ProxyConfig{
		Server:   "proxy:8080",
		Username: "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enable fetch for proxy auth") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestRunTaskNoStepsNoProxy(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	task := models.Task{
		ID:       "task-1",
		Headless: true,
		Steps:    []models.TaskStep{},
	}

	result, err := r.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.TaskID != "task-1" {
		t.Errorf("expected taskID 'task-1', got %q", result.TaskID)
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestRunTaskWithSteps(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	task := models.Task{
		ID:       "task-2",
		Headless: true,
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
			{Action: models.ActionClick, Selector: "#btn"},
		},
	}

	result, err := r.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if mock.callCount() != 2 {
		t.Fatalf("expected 2 executor calls, got %d", mock.callCount())
	}
}

func TestRunTaskWithProxy(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	task := models.Task{
		ID:       "task-proxy",
		Headless: true,
		Proxy: models.ProxyConfig{
			Server:   "proxy:8080",
			Username: "user",
			Password: "pass",
		},
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
		},
	}

	result, err := r.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if mock.callCount() != 2 {
		t.Fatalf("expected 2 calls (fetch.Enable + navigate), got %d", mock.callCount())
	}
}

func TestRunTaskProxyAuthFails(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("fetch enable failed")}
	r := newMockRunner(t, mock)

	task := models.Task{
		ID:       "task-proxy-fail",
		Headless: true,
		Proxy: models.ProxyConfig{
			Server:   "proxy:8080",
			Username: "user",
			Password: "pass",
		},
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
		},
	}

	result, err := r.RunTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Error("expected success=false")
	}
	if !strings.Contains(result.Error, "proxy auth setup failed") {
		t.Errorf("expected proxy auth error in result, got: %q", result.Error)
	}
}

func TestRunTaskStepFails(t *testing.T) {
	callCount := 0
	mock := &mockExecutor{}
	mock.runErr = nil
	r := newMockRunner(t, mock)

	failOnSecond := &mockExecutor{}
	r.exec = &callCountExecutor{
		onCall: func(n int) error {
			callCount = n
			if n == 2 {
				return errors.New("step 2 failed")
			}
			return nil
		},
	}

	task := models.Task{
		ID:       "task-step-fail",
		Headless: true,
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
			{Action: models.ActionClick, Selector: "#btn"},
		},
	}
	_ = failOnSecond

	result, err := r.RunTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Success {
		t.Error("expected success=false")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

type callCountExecutor struct {
	mu     sync.Mutex
	count  int
	onCall func(n int) error
}

func (c *callCountExecutor) Run(ctx context.Context, actions ...chromedp.Action) error {
	c.mu.Lock()
	c.count++
	n := c.count
	c.mu.Unlock()
	return c.onCall(n)
}

func (c *callCountExecutor) RunResponse(ctx context.Context, actions ...chromedp.Action) (*network.Response, error) {
	c.mu.Lock()
	c.count++
	n := c.count
	c.mu.Unlock()
	return nil, c.onCall(n)
}

func (c *callCountExecutor) Targets(ctx context.Context) ([]*target.Info, error) {
	return nil, nil
}

func TestCreateAllocatorWithProtocol(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	proxy := models.ProxyConfig{
		Server:   "proxy.example.com:1080",
		Protocol: models.ProxySOCKS5,
	}

	allocCtx, allocCancel := r.createAllocator(context.Background(), proxy, true)
	defer allocCancel()

	if allocCtx == nil {
		t.Fatal("allocator context is nil")
	}
}

func TestExecWaitSelectorWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execWait(context.Background(), models.TaskStep{Selector: "#elem"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecWaitSelectorError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("wait failed")}
	r := newMockRunner(t, mock)

	err := r.execWait(context.Background(), models.TaskStep{Selector: "#elem"})
	if err == nil || err.Error() != "wait failed" {
		t.Fatalf("expected 'wait failed', got: %v", err)
	}
}

func TestExecuteStepAllActionsWithMock(t *testing.T) {
	tests := []struct {
		action   models.StepAction
		selector string
		value    string
	}{
		{models.ActionNavigate, "", "https://example.com"},
		{models.ActionClick, "#btn", ""},
		{models.ActionType, "#input", "text"},
		{models.ActionWait, "#elem", ""},
		{models.ActionScreenshot, "", ""},
		{models.ActionExtract, "#data", "key"},
		{models.ActionScroll, "", "100"},
		{models.ActionSelect, "#sel", "opt1"},
	}

	for _, tc := range tests {
		t.Run(string(tc.action), func(t *testing.T) {
			mock := &mockExecutor{}
			r := newMockRunner(t, mock)
			result := &models.TaskResult{TaskID: "dispatch", ExtractedData: make(map[string]string)}

			step := models.TaskStep{Action: tc.action, Selector: tc.selector, Value: tc.value}
			err := r.executeStep(context.Background(), step, result)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.action, err)
			}
			if mock.callCount() < 1 {
				t.Fatalf("expected at least 1 executor call for %s", tc.action)
			}
		})
	}
}

func TestExecuteStepTabSwitchWithMock(t *testing.T) {
	mock := &mockExecutor{
		targets: []*target.Info{{Type: "page", URL: "https://example.com"}},
	}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "tab", ExtractedData: make(map[string]string)}

	step := models.TaskStep{Action: models.ActionTabSwitch, Value: "https://example.com"}
	err := r.executeStep(context.Background(), step, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteStepEvalWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.allowEval.Store(true)
	result := &models.TaskResult{TaskID: "eval", ExtractedData: make(map[string]string)}

	step := models.TaskStep{Action: models.ActionEval, Value: "1+1"}
	err := r.executeStep(context.Background(), step, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecNavigateHTTPStatus404(t *testing.T) {
	mock := &mockExecutor{
		runResponse: &network.Response{Status: 404, StatusText: "Not Found"},
	}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Value: "https://example.com/missing"})
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 in error, got: %v", err)
	}
}

func TestExecNavigateHTTPStatus200(t *testing.T) {
	mock := &mockExecutor{
		runResponse: &network.Response{Status: 200, StatusText: "OK"},
	}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecNavigateNilResponse(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error for nil response: %v", err)
	}
}

func TestExecNavigateRunResponseError(t *testing.T) {
	mock := &mockExecutor{runRespErr: errors.New("connection refused")}
	r := newMockRunner(t, mock)

	err := r.execNavigate(context.Background(), models.TaskStep{Value: "https://example.com"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected 'connection refused', got: %v", err)
	}
}

func TestSetCaptchaSolver(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	var solver captcha.Solver
	r.SetCaptchaSolver(solver)
	r.mu.Lock()
	if r.captchaSolver != solver {
		t.Error("captcha solver not set correctly")
	}
	r.mu.Unlock()
}

func TestSetPool(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	pool := &BrowserPool{poolSize: 5}
	r.SetPool(pool)
	r.mu.Lock()
	if r.pool != pool {
		t.Error("pool not set correctly")
	}
	r.mu.Unlock()
}

func TestSetLocalProxyManager(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	var pm *localproxy.Manager
	r.SetLocalProxyManager(pm)
	r.mu.Lock()
	if r.localProxyManager != pm {
		t.Error("local proxy manager not set correctly")
	}
	r.mu.Unlock()
}

func TestSetDefaultLoggingPolicy(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	policy := models.TaskLoggingPolicy{MaxExecutionLogs: 500}
	r.SetDefaultLoggingPolicy(policy)
	r.mu.Lock()
	if r.defaultLoggingPolicy.MaxExecutionLogs != 500 {
		t.Error("logging policy not set correctly")
	}
	r.mu.Unlock()
}

func TestResolveLoggingPolicy(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}

	task := models.Task{}
	policy := r.resolveLoggingPolicy(task)
	if !policy.captureStepLogs || !policy.captureNetworkLogs || !policy.captureScreenshots {
		t.Error("default logging policy should enable all capture")
	}
	if policy.maxExecutionLogs != 1000 {
		t.Errorf("default max logs should be 1000, got %d", policy.maxExecutionLogs)
	}

	falseVal := false
	task.LoggingPolicy = &models.TaskLoggingPolicy{
		CaptureStepLogs:  &falseVal,
		MaxExecutionLogs: 200,
	}
	policy = r.resolveLoggingPolicy(task)
	if policy.captureStepLogs {
		t.Error("task policy should override default")
	}
	if policy.maxExecutionLogs != 200 {
		t.Errorf("task max logs should be 200, got %d", policy.maxExecutionLogs)
	}
}

func TestAddLogWithLimit(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: chromeExecutor{}}
	result := &models.TaskResult{
		TaskID:        "test-limit",
		LogLimit:      3,
		ExtractedData: make(map[string]string),
	}

	r.addLog(result, "info", "log 1")
	r.addLog(result, "info", "log 2")
	r.addLog(result, "info", "log 3")
	r.addLog(result, "info", "log 4")
	r.addLog(result, "info", "log 5")

	if len(result.Logs) != 3 {
		t.Errorf("log limit not respected: got %d logs, want 3", len(result.Logs))
	}
	if result.Logs[0].Message != "log 3" || result.Logs[2].Message != "log 5" {
		t.Error("log rotation not working correctly")
	}
}

func TestRunStepsWithLoop(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "loop-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionLoop, Value: "2"},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionEndLoop},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("loop execution failed: %v", err)
	}
	if mock.callCount() != 2 {
		t.Errorf("loop should execute body twice, got %d executor calls", mock.callCount())
	}
}

func TestRunStepsWithBreakLoop(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "break-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionLoop, Value: "10"},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionBreakLoop},
		{Action: models.ActionEndLoop},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("break loop execution failed: %v", err)
	}
	if mock.callCount() != 1 {
		t.Errorf("break should exit loop after 1 iteration, got %d calls", mock.callCount())
	}
}

func TestRunStepsWithGoto(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "goto-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionGoto, JumpTo: "skipme"},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionNavigate, Value: "https://example.com", Label: "skipme"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("goto execution failed: %v", err)
	}
	if mock.callCount() != 2 {
		t.Errorf("goto should skip middle navigate, got %d calls", mock.callCount())
	}
}

func TestRunStepsGotoLabelNotFound(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: &mockExecutor{}}
	result := &models.TaskResult{
		TaskID:        "goto-notfound",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionGoto, JumpTo: "nonexistent"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected label not found error, got: %v", err)
	}
}

func TestRunStepsEndLoopWithoutLoop(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: &mockExecutor{}}
	result := &models.TaskResult{
		TaskID:        "endloop-mismatch",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionEndLoop},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err == nil || !strings.Contains(err.Error(), "without matching loop") {
		t.Errorf("expected mismatch error, got: %v", err)
	}
}

func TestRunStepsBreakLoopWithoutLoop(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: &mockExecutor{}}
	result := &models.TaskResult{
		TaskID:        "break-mismatch",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionBreakLoop},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err == nil || !strings.Contains(err.Error(), "without matching loop") {
		t.Errorf("expected mismatch error, got: %v", err)
	}
}

func TestRunStepsWithWhile(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "while-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionWhile, Selector: "#elem", Condition: "exists", MaxLoops: 2},
		{Action: models.ActionWait, Value: "1"},
		{Action: models.ActionEndWhile},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("while execution failed: %v", err)
	}
}

func TestRunStepsEndWhileWithoutWhile(t *testing.T) {
	r := &Runner{screenshotDir: t.TempDir(), exec: &mockExecutor{}}
	result := &models.TaskResult{
		TaskID:        "endwhile-mismatch",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionEndWhile},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err == nil || !strings.Contains(err.Error(), "without matching while_condition") {
		t.Errorf("expected mismatch error, got: %v", err)
	}
}

func TestRunStepsIfElement(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "if-element-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionIfElement, Selector: "#elem", Condition: "exists"},
		{Action: models.ActionWait, Value: "1"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("if_element execution failed: %v", err)
	}
}

func TestRunStepsIfElementWithJumpTo(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "if-element-jump",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionIfElement, Selector: "#elem", Condition: "not_exists", JumpTo: "target"},
		{Action: models.ActionWait, Value: "1"},
		{Action: models.ActionWait, Value: "1", Label: "target"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("if_element jump execution failed: %v", err)
	}
}

func TestRunStepsIfText(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "if-text-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionIfText, Selector: "#text", Condition: "contains:hello"},
		{Action: models.ActionWait, Value: "1"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("if_text execution failed: %v", err)
	}
}

func TestRunStepsIfURL(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{
		TaskID:        "if-url-test",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: models.ActionIfURL, Condition: "contains:example"},
		{Action: models.ActionWait, Value: "1"},
	}

	err := r.runSteps(context.Background(), steps, result, nil, r.resolveLoggingPolicy(models.Task{}))
	if err != nil {
		t.Fatalf("if_url execution failed: %v", err)
	}
}

func TestClearCookies(t *testing.T) {
	err := ClearCookies(context.Background())
	if err == nil {
		t.Fatal("expected error when calling ClearCookies without a browser")
	}
}
func TestNotificationHub(t *testing.T) {
	hub := NewNotificationHub()
	if hub == nil {
		t.Fatal("NewNotificationHub returned nil")
	}
	hub.Send("hello")
	hub.Send("world")
	ch := hub.Ch()
	if ch == nil {
		t.Fatal("Ch() returned nil")
	}
	msg := <-ch
	if msg != "hello" {
		t.Errorf("got %q, want %q", msg, "hello")
	}
	hub.Close()
	hub.Close()             // idempotent
	hub.Send("after close") // no-op, should not panic
}

func TestRunnerSettersNoPool(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRunner(dir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	r.SetForceHeadless(true)
	r.SetAllowEval(true)
	r.SetCaptchaSolver(nil)
	r.SetDefaultLoggingPolicy(models.TaskLoggingPolicy{MaxExecutionLogs: 100})
	// Note: SetPool and StopProxyPools require a live pool; skip here
}

func TestExecWhileWithMock(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionWhile, Condition: "text_contains", Selector: "#el", Value: "hello", MaxLoops: 5}
	if err := runner.execWhile(context.Background(), step, result); err != nil {
		t.Fatalf("execWhile: %v", err)
	}
	if result.ExtractedData["_while_max_loops"] != "5" {
		t.Errorf("max_loops: got %q, want %q", result.ExtractedData["_while_max_loops"], "5")
	}
}

func TestExecWhileNoCondition(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionWhile}
	err := runner.execWhile(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for missing condition")
	}
}

func TestExecEndWhile(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionEndWhile}
	if err := runner.execEndWhile(context.Background(), step, result); err != nil {
		t.Fatalf("execEndWhile: %v", err)
	}
}

func TestExecVariableStringOps(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	ctx := context.Background()
	cases := []struct {
		op       string
		initial  string
		value    string
		cond     string
		expected string
	}{
		{"concat", "hello", " world", "", "hello world"},
		{"prepend", "world", "hello ", "", "hello world"},
		{"upper", "hello", "", "", "HELLO"},
		{"lower", "HELLO", "", "", "hello"},
		{"trim", "  hi  ", "", "", "hi"},
		{"length", "hello", "", "", "5"},
		{"replace", "hello world", "new", "world", "hello new"},
		{"unknown", "hello", "", "", "hello"},
	}
	for _, tc := range cases {
		result := &models.TaskResult{ExtractedData: map[string]string{"var_x": tc.initial}}
		step := models.TaskStep{Action: models.ActionVariableString, VarName: "x", Operator: tc.op, Value: tc.value, Condition: tc.cond}
		if err := runner.execVariableString(ctx, step, result); err != nil {
			t.Errorf("op %s: %v", tc.op, err)
		}
		if result.ExtractedData["var_x"] != tc.expected {
			t.Errorf("op %s: got %q, want %q", tc.op, result.ExtractedData["var_x"], tc.expected)
		}
	}
}

func TestExecVariableStringNoVarName(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionVariableString}
	if err := runner.execVariableString(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing varName")
	}
}

func TestJumpToLabel(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	if err := runner.jumpToLabel("my-label", result); err != nil {
		t.Fatalf("jumpToLabel: %v", err)
	}
	if result.ExtractedData["_jump_to_label"] != "my-label" {
		t.Errorf("label: got %q, want %q", result.ExtractedData["_jump_to_label"], "my-label")
	}
}

func TestExecIfExistsNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionIfExists, Selector: ""}
	if err := runner.execIfExists(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecIfNotExistsNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionIfNotExists, Selector: ""}
	if err := runner.execIfNotExists(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecIfVisibleNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionIfVisible, Selector: ""}
	if err := runner.execIfVisible(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecIfEnabledNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionIfEnabled, Selector: ""}
	if err := runner.execIfEnabled(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecVariableMathOps(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	ctx := context.Background()
	ops := []struct {
		op       string
		initial  string
		value    string
		expected string
	}{
		{"add", "10", "5", "15"},
		{"subtract", "10", "3", "7"},
		{"multiply", "4", "3", "12"},
		{"divide", "10", "2", "5"},
	}
	for _, tc := range ops {
		result := &models.TaskResult{ExtractedData: map[string]string{"var_n": tc.initial}}
		step := models.TaskStep{Action: models.ActionVariableMath, VarName: "n", Operator: tc.op, Value: tc.value}
		if err := runner.execVariableMath(ctx, step, result); err != nil {
			t.Errorf("op %s: %v", tc.op, err)
		}
		if result.ExtractedData["var_n"] != tc.expected {
			t.Errorf("op %s: got %q, want %q", tc.op, result.ExtractedData["var_n"], tc.expected)
		}
	}
}

func TestExecCacheSetNoKey(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionCacheSet, Selector: "", Value: "myval"}
	err := runner.execCacheSet(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for missing cache key")
	}
}

func TestExecDebugPauseCancelledCtx(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionDebugPause}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled context so debugPause exits immediately
	_ = runner.execDebugPause(ctx, step, result)
}

func TestExecSelectRandomNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionSelectRandom, Selector: ""}
	if err := runner.execSelectRandom(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecHumanTypingValidation(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionHumanTyping, Selector: ""}
	if err := runner.execHumanTyping(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
	step2 := models.TaskStep{Action: models.ActionHumanTyping, Selector: "#input", Value: ""}
	if err := runner.execHumanTyping(context.Background(), step2); err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestExecLoadSessionNoVarName(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionLoadSession, VarName: ""}
	if err := runner.execLoadSession(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing varName")
	}
}

func TestExecSelectNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionSelect, Selector: ""}
	if err := runner.execSelect(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecDownloadNoURL(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionDownload, Value: ""}
	if err := runner.execDownload(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestSetLocalProxyManagerAfterCreate(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRunner(dir)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	lpm := localproxy.NewManager(time.Minute)
	defer lpm.Stop()
	r.SetLocalProxyManager(lpm)
}

func TestExecSolveCaptchaNoSolver(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	runner.SetCaptchaSolver(nil)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionSolveCaptcha, Value: string(models.CaptchaTypeRecaptchaV2)}
	err := runner.execSolveCaptcha(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error when no solver configured")
	}
	if !strings.Contains(err.Error(), "captcha solver not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockCaptchaSolver struct{ token string }

func (m *mockCaptchaSolver) Solve(_ context.Context, _ models.CaptchaSolveRequest) (*models.CaptchaSolveResult, error) {
	return &models.CaptchaSolveResult{Token: m.token}, nil
}
func (m *mockCaptchaSolver) Balance(_ context.Context) (float64, error) { return 1.0, nil }

func TestExecSolveCaptchaWithSolver(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	runner.SetCaptchaSolver(&mockCaptchaSolver{token: "test-token-123"})
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionSolveCaptcha, Value: string(models.CaptchaTypeRecaptchaV2), Selector: "site-key", VarName: "my_token"}
	err := runner.execSolveCaptcha(context.Background(), step, result)
	if err != nil {
		t.Fatalf("execSolveCaptcha: %v", err)
	}
	if result.ExtractedData["my_token"] != "test-token-123" {
		t.Errorf("token: got %q, want %q", result.ExtractedData["my_token"], "test-token-123")
	}
}

func TestExecDownloadBadPath(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionDownload, Selector: "a.link", Path: "/nonexistent/dir/that/doesnt/exist"}
	err := runner.execDownload(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for non-existent download path")
	}
}

func TestExecLoadSessionWithData(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{
		"default_session_data": `{"cookies":"session=abc","localStorage":{"key":"value"}}`,
	}}
	step := models.TaskStep{Action: models.ActionLoadSession, VarName: ""}
	err := runner.execLoadSession(context.Background(), step, result)
	if err != nil {
		t.Fatalf("execLoadSession with data: %v", err)
	}
}

func TestExecLoadSessionNoData(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionLoadSession, VarName: "mysession"}
	err := runner.execLoadSession(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for missing session data")
	}
}

func TestExecVariableSetOps(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	ctx := context.Background()
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionVariableSet, VarName: "myvar", Value: "hello"}
	if err := runner.execVariableSet(ctx, step, result); err != nil {
		t.Fatalf("execVariableSet: %v", err)
	}
	if result.ExtractedData["var_myvar"] != "hello" {
		t.Errorf("var: got %q, want %q", result.ExtractedData["var_myvar"], "hello")
	}
}

func TestExecVariableSetNoVarName(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionVariableSet, VarName: ""}
	if err := runner.execVariableSet(context.Background(), step, result); err == nil {
		t.Fatal("expected error for missing varName")
	}
}

func TestResolveExistingDirWithinBaseValid(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	_, resolved, err := resolveExistingDirWithinBase(base, sub)
	if err != nil {
		t.Fatalf("resolveExistingDirWithinBase: %v", err)
	}
	if resolved == "" {
		t.Error("expected non-empty resolved path")
	}
}

func TestResolveExistingDirWithinBaseEscape(t *testing.T) {
	base := t.TempDir()
	other := t.TempDir()
	_, _, err := resolveExistingDirWithinBase(base, other)
	if err == nil {
		t.Fatal("expected error for path escaping base")
	}
}

func TestResolveExistingDirWithinBaseNotDir(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, _, err := resolveExistingDirWithinBase(base, file)
	if err == nil {
		t.Fatal("expected error for file path (not a directory)")
	}
}

func TestExecVariableMathDivideByZero(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{"var_n": "10"}}
	step := models.TaskStep{Action: models.ActionVariableMath, VarName: "n", Operator: "divide", Value: "0"}
	err := runner.execVariableMath(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for divide by zero")
	}
}

func TestExecVariableMathNoVarName(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionVariableMath, VarName: ""}
	err := runner.execVariableMath(context.Background(), step, result)
	if err == nil {
		t.Fatal("expected error for missing varName")
	}
}

func TestExecDoubleClick(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionDoubleClick, Selector: "#btn"}
	if err := runner.execDoubleClick(context.Background(), step); err != nil {
		t.Fatalf("execDoubleClick: %v", err)
	}
}

func TestExecDoubleClickNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionDoubleClick, Selector: ""}
	if err := runner.execDoubleClick(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecScrollIntoView(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionScrollIntoView, Selector: "#elem"}
	if err := runner.execScrollIntoView(context.Background(), step); err != nil {
		t.Fatalf("execScrollIntoView: %v", err)
	}
}

func TestExecScrollIntoViewNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionScrollIntoView, Selector: ""}
	if err := runner.execScrollIntoView(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecSubmitForm(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionSubmitForm, Selector: "form#main"}
	if err := runner.execSubmitForm(context.Background(), step); err != nil {
		t.Fatalf("execSubmitForm: %v", err)
	}
}

func TestExecSubmitFormNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionSubmitForm, Selector: ""}
	if err := runner.execSubmitForm(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecWaitNotPresent(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionWaitNotPresent, Selector: "#loading"}
	if err := runner.execWaitNotPresent(context.Background(), step); err != nil {
		t.Fatalf("execWaitNotPresent: %v", err)
	}
}

func TestExecWaitNotPresentNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionWaitNotPresent, Selector: ""}
	if err := runner.execWaitNotPresent(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestExecWaitEnabled(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionWaitEnabled, Selector: "#submit-btn"}
	if err := runner.execWaitEnabled(context.Background(), step); err != nil {
		t.Fatalf("execWaitEnabled: %v", err)
	}
}

func TestExecWaitEnabledNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	step := models.TaskStep{Action: models.ActionWaitEnabled, Selector: ""}
	if err := runner.execWaitEnabled(context.Background(), step); err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestAddLogFiveMessages(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	for i := 0; i < 5; i++ {
		runner.addLog(result, "info", fmt.Sprintf("msg %d", i))
	}
	if len(result.Logs) != 5 {
		t.Errorf("expected 5 logs, got %d", len(result.Logs))
	}
}

func TestBrowserPoolStatsMethod(t *testing.T) {
	pool := &BrowserPool{}
	s := pool.stats()
	_ = s
}

func TestRunStepsLoopAndEndLoop(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "loop-test"}
	steps := []models.TaskStep{
		{Action: models.ActionLoop, Value: "2"},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionEndLoop},
	}
	policy := resolvedLoggingPolicy{}
	if err := runner.runSteps(context.Background(), steps, result, nil, policy); err != nil {
		t.Fatalf("runSteps loop: %v", err)
	}
}

func TestRunStepsGoto(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "goto-test"}
	steps := []models.TaskStep{
		{Action: models.ActionGoto, Value: "end"},
		{Action: models.ActionClick, Selector: "#never"},
		{Action: models.ActionGoto, Value: "end"},
	}
	policy := resolvedLoggingPolicy{}
	err := runner.runSteps(context.Background(), steps, result, nil, policy)
	_ = err // goto to unknown label returns error; that's ok
}

func TestRunStepsEndLoopNoMatchNew(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "bad-el"}
	steps := []models.TaskStep{{Action: models.ActionEndLoop}}
	policy := resolvedLoggingPolicy{}
	err := runner.runSteps(context.Background(), steps, result, nil, policy)
	if err == nil {
		t.Fatal("expected error for end_loop without loop")
	}
}

func TestRunStepsWhileCondNoCondition(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "while-cond"}
	steps := []models.TaskStep{
		{Action: models.ActionWhile, Condition: "", Selector: "", MaxLoops: 1},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionEndWhile},
	}
	policy := resolvedLoggingPolicy{}
	_ = runner.runSteps(context.Background(), steps, result, nil, policy)
}

func TestEvaluateConditionIfURL(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	vars := map[string]string{}
	step := models.TaskStep{Action: models.ActionIfURL, Condition: "contains", Value: "example"}
	_, err := runner.evaluateCondition(context.Background(), step, vars)
	if err != nil {
		t.Fatalf("evaluateCondition IfURL: %v", err)
	}
}

func TestEvaluateConditionWhileWithSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	vars := map[string]string{}
	step := models.TaskStep{Action: models.ActionWhile, Condition: "exists", Selector: "#elem"}
	_, err := runner.evaluateCondition(context.Background(), step, vars)
	if err != nil {
		t.Fatalf("evaluateCondition While+selector: %v", err)
	}
}

func TestEvaluateConditionWhileNoSelector(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	vars := map[string]string{}
	step := models.TaskStep{Action: models.ActionWhile, Condition: "contains", Selector: "", Value: "example"}
	_, err := runner.evaluateCondition(context.Background(), step, vars)
	if err != nil {
		t.Fatalf("evaluateCondition While no selector: %v", err)
	}
}

func TestEvaluateTextConditionsAll(t *testing.T) {
	vars := map[string]string{}
	cases := []struct {
		cond string
		text string
		want bool
	}{
		{"contains:hello", "hello world", true},
		{"not_contains:xyz", "hello world", true},
		{"starts_with:hello", "hello world", true},
		{"ends_with:world", "hello world", true},
		{"equals:hello", "hello", true},
		{"not_equals:world", "hello", true},
		{"matches:[a-z]+", "abc123", true},
		{"hello", "hello world", true}, // fallback: strings.Contains
	}
	for _, tc := range cases {
		got, err := evaluateTextCondition(tc.cond, tc.text, vars)
		if err != nil {
			t.Errorf("cond %q: %v", tc.cond, err)
		}
		if got != tc.want {
			t.Errorf("cond %q on %q: got %v, want %v", tc.cond, tc.text, got, tc.want)
		}
	}
}

func TestExecVariableStringReplaceNotFound(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{"var_x": "hello world"}}
	step := models.TaskStep{Action: models.ActionVariableString, VarName: "x", Operator: "replace", Value: "newval", Condition: "notfound"}
	if err := runner.execVariableString(context.Background(), step, result); err != nil {
		t.Fatalf("replace (not found): %v", err)
	}
	if result.ExtractedData["var_x"] != "hello world" {
		t.Errorf("unexpected change: got %q", result.ExtractedData["var_x"])
	}
}

func TestRunStepsLoopZeroIterations(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "loop-zero"}
	steps := []models.TaskStep{
		{Action: models.ActionLoop, Value: "0"},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionEndLoop},
	}
	policy := resolvedLoggingPolicy{}
	if err := runner.runSteps(context.Background(), steps, result, nil, policy); err != nil {
		t.Fatalf("runSteps loop 0: %v", err)
	}
}

func TestRunStepsWhileLoop(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "while-loop"}
	steps := []models.TaskStep{
		{Action: models.ActionWhile, Condition: "exists", Selector: "#btn", MaxLoops: 1},
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionEndWhile},
	}
	policy := resolvedLoggingPolicy{}
	if err := runner.runSteps(context.Background(), steps, result, nil, policy); err != nil {
		t.Fatalf("runSteps while: %v", err)
	}
}

func TestRunStepsGotoKnownLabel(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "goto-known"}
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionGoto, Value: "end"},
		{Action: models.ActionClick, Selector: "#skipped"},
		{Action: models.ActionNavigate, Value: "https://end.com", Label: "end"},
	}
	policy := resolvedLoggingPolicy{}
	_ = runner.runSteps(context.Background(), steps, result, nil, policy)
}

func TestRunStepsPauseAndResume(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	runner.SetForceHeadless(false)
	result := &models.TaskResult{ExtractedData: map[string]string{}, TaskID: "pause-res"}
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
	}
	policy := resolvedLoggingPolicy{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = runner.runSteps(ctx, steps, result, nil, policy)
}

func TestExecGetCookiesDefaultVarName(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionGetCookies, VarName: ""}
	// empty VarName defaults to "cookie" prefix; mock returns empty cookies, no error
	if err := runner.execGetCookies(context.Background(), step, result); err != nil {
		t.Fatalf("execGetCookies empty varName: %v", err)
	}
	if _, ok := result.ExtractedData["cookie_count"]; !ok {
		t.Error("expected cookie_count in extracted data")
	}
}

func TestExecDebugStepCancelledCtx(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionDebugStep}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_ = runner.execDebugStep(ctx, step, result)
}

func TestExecDebugResumeAfterPause(t *testing.T) {
	exec := &mockExecutor{}
	runner := newMockRunner(t, exec)
	result := &models.TaskResult{ExtractedData: map[string]string{}}
	step := models.TaskStep{Action: models.ActionDebugResume}
	if err := runner.execDebugResume(context.Background(), step, result); err != nil {
		t.Fatalf("execDebugResume: %v", err)
	}
}

// panicExecutor is a test-only Executor that panics on every Run call with a
// non-chromedp error, simulating an unexpected browser crash.  It is used to
// verify that RunTask converts such panics into a deterministic error return
// rather than re-panicking and leaving the task stuck in "running" state.
type panicExecutor struct{}

func (p *panicExecutor) Run(_ context.Context, _ ...chromedp.Action) error {
	panic(errors.New("simulated unknown browser panic"))
}

func (p *panicExecutor) RunResponse(_ context.Context, _ ...chromedp.Action) (*network.Response, error) {
	return nil, nil
}

func (p *panicExecutor) Targets(_ context.Context) ([]*target.Info, error) {
	return nil, nil
}

// TestRunTask_UnknownPanic_ReturnsErrorNotPanic verifies that an unknown panic
// inside RunTask produces a non-nil error and a populated result.Error instead
// of re-panicking and leaving the task stuck in "running" state.
//
// Strategy: panicExecutor panics on Run.  A proxy with credentials causes
// RunTask to call r.exec.Run via setupProxyAuth, triggering the panic before
// any steps are executed.
func TestRunTask_UnknownPanic_ReturnsErrorNotPanic(t *testing.T) {
	r := &Runner{
		screenshotDir: t.TempDir(),
		exec:          &panicExecutor{},
	}

	task := models.Task{
		ID:       "panic-recovery-task",
		Headless: true,
		// Credentials trigger setupProxyAuth → r.exec.Run → panic.
		Proxy: models.ProxyConfig{
			Server:   "proxy:8080",
			Username: "user",
			Password: "pass",
		},
		Steps: []models.TaskStep{},
	}

	// The test itself must NOT panic; the test runner would fail if it did.
	result, err := r.RunTask(context.Background(), task)

	if err == nil {
		t.Fatal("expected non-nil error from unknown panic, got nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result (named returns preserve the partially-built result)")
	}
	// Named-return fix sets result.Error to "unexpected browser panic: ..." and
	// returns err = fmt.Errorf("browser panic: %v", p).
	if !strings.Contains(result.Error, "browser panic") {
		t.Errorf("result.Error must contain 'browser panic', got: %q", result.Error)
	}
}
