package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"web-automation/internal/models"

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
	runner := &Runner{screenshotDir: t.TempDir()}
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
	runner := &Runner{screenshotDir: t.TempDir()}
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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}
	result := &models.TaskResult{
		TaskID:        "empty-steps",
		ExtractedData: make(map[string]string),
	}

	err := runner.runSteps(context.Background(), nil, result)
	if err != nil {
		t.Fatalf("runSteps with nil steps: %v", err)
	}

	err = runner.runSteps(context.Background(), []models.TaskStep{}, result)
	if err != nil {
		t.Fatalf("runSteps with empty steps: %v", err)
	}
}

func TestRunStepsStopsOnError(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir()}
	result := &models.TaskResult{
		TaskID:        "stop-on-error",
		ExtractedData: make(map[string]string),
	}

	steps := []models.TaskStep{
		{Action: "invalid_action_1"},
		{Action: "invalid_action_2"},
	}

	err := runner.runSteps(context.Background(), steps, result)
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
	runner := &Runner{screenshotDir: t.TempDir()}

	ctx := context.Background()
	allocCtx, allocCancel := runner.createAllocator(ctx, models.ProxyConfig{}, true)
	defer allocCancel()

	if allocCtx == nil {
		t.Fatal("allocator context is nil")
	}
}

func TestCreateAllocatorWithProxy(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}
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
			result := sanitizeFilename(tc.input)
			if strings.Contains(result, "/") || strings.Contains(result, "\\") || strings.Contains(result, "..") {
				t.Errorf("sanitizeFilename(%q) = %q, still contains path components", tc.input, result)
			}
			if result == "" {
				t.Errorf("sanitizeFilename(%q) returned empty string", tc.input)
			}
		})
	}
}

func TestExecScreenshotPathTraversal(t *testing.T) {
	dir := t.TempDir()
	_ = &Runner{screenshotDir: dir}
	result := &models.TaskResult{
		TaskID:        "../../etc/cron.d/evil",
		ExtractedData: make(map[string]string),
	}

	filename := sanitizeFilename(result.TaskID)
	fullPath := filepath.Join(dir, filename+"_test.png")

	if !strings.HasPrefix(fullPath, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatal("sanitized path should stay within screenshot directory")
	}
	if strings.Contains(filename, "..") {
		t.Errorf("sanitized filename still contains path traversal: %q", filename)
	}
}

func TestExecEvalBlockedByDefault(t *testing.T) {
	runner := &Runner{screenshotDir: t.TempDir()}

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
	runner := &Runner{screenshotDir: t.TempDir()}
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
	runner := &Runner{screenshotDir: t.TempDir()}
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
		{"blocked process.env", "console.log(process.env.SECRET)", true, "blocked pattern"},
		{"blocked child_process", "child_process.exec('ls')", true, "blocked pattern"},
		{"blocked fs.readFile", "fs.readFile('/etc/passwd')", true, "blocked pattern"},
		{"blocked fs.writeFile", "fs.writeFile('/tmp/x', 'data')", true, "blocked pattern"},
		{"blocked __dirname", "console.log(__dirname)", true, "blocked pattern"},
		{"blocked __filename", "console.log(__filename)", true, "blocked pattern"},
		{"case insensitive require", "REQUIRE('fs')", true, "blocked pattern"},
		{"case insensitive process", "Process.Exit(0)", true, "blocked pattern"},
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
	runner := &Runner{screenshotDir: t.TempDir()}
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
	runner := &Runner{screenshotDir: t.TempDir()}

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
			runner := &Runner{screenshotDir: t.TempDir()}
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
	runner := &Runner{screenshotDir: t.TempDir()}

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
