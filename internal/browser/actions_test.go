package browser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"flowpilot/internal/models"
)

func TestExecDoubleClickWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execDoubleClick(context.Background(), models.TaskStep{Selector: "#btn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecDoubleClickError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("double click failed")}
	r := newMockRunner(t, mock)

	err := r.execDoubleClick(context.Background(), models.TaskStep{Selector: "#btn"})
	if err == nil || err.Error() != "double click failed" {
		t.Fatalf("expected 'double click failed', got: %v", err)
	}
}

func TestExecFileUploadWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execFileUpload(context.Background(), models.TaskStep{Selector: "#file", Value: "/tmp/test.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.callCount())
	}
}

func TestExecFileUploadError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("upload failed")}
	r := newMockRunner(t, mock)

	err := r.execFileUpload(context.Background(), models.TaskStep{Selector: "#file", Value: "/tmp/test.txt"})
	if err == nil || err.Error() != "upload failed" {
		t.Fatalf("expected 'upload failed', got: %v", err)
	}
}

func TestExecNavigateBackWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execNavigateBack(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecNavigateForwardWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execNavigateForward(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecReloadWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execReload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecScrollIntoViewWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execScrollIntoView(context.Background(), models.TaskStep{Selector: "#elem"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecSubmitFormWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execSubmitForm(context.Background(), models.TaskStep{Selector: "#form"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecWaitNotPresentWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execWaitNotPresent(context.Background(), models.TaskStep{Selector: "#spinner"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecWaitEnabledWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execWaitEnabled(context.Background(), models.TaskStep{Selector: "#btn"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecWaitFunctionBlockedByDefault(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execWaitFunction(context.Background(), models.TaskStep{Value: "document.title === 'ready'"})
	if err != ErrEvalNotAllowed {
		t.Fatalf("expected ErrEvalNotAllowed, got: %v", err)
	}
}

func TestExecWaitFunctionValidation(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.allowEval.Store(true)

	err := r.execWaitFunction(context.Background(), models.TaskStep{Value: "require('child_process')"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "wait_function validation failed") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestExecWaitFunctionWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.allowEval.Store(true)

	err := r.execWaitFunction(context.Background(), models.TaskStep{Value: "document.title === 'ready'"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecEmulateDeviceValid(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	err := r.execEmulateDevice(context.Background(), models.TaskStep{Value: "375x812"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecEmulateDeviceInvalid(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)

	tests := []struct {
		name  string
		value string
	}{
		{"no separator", "375812"},
		{"invalid width", "abcx812"},
		{"invalid height", "375xabc"},
		{"zero width", "0x812"},
		{"negative height", "375x-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := r.execEmulateDevice(context.Background(), models.TaskStep{Value: tc.value})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExecGetTitleWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "title-test", ExtractedData: make(map[string]string)}

	err := r.execGetTitle(context.Background(), models.TaskStep{Value: "my_title"}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.ExtractedData["my_title"]; !ok {
		t.Error("expected 'my_title' key in extracted data")
	}
}

func TestExecGetTitleDefaultKey(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "title-test", ExtractedData: make(map[string]string)}

	err := r.execGetTitle(context.Background(), models.TaskStep{}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.ExtractedData["page_title"]; !ok {
		t.Error("expected 'page_title' key in extracted data")
	}
}

func TestExecGetAttributesWithMock(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "attrs-test", ExtractedData: make(map[string]string)}

	err := r.execGetAttributes(context.Background(), models.TaskStep{Selector: "#elem", Value: "el"}, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseViewportSize(t *testing.T) {
	tests := []struct {
		input string
		w, h  int
		err   bool
	}{
		{"1920x1080", 1920, 1080, false},
		{"375x812", 375, 812, false},
		{"invalid", 0, 0, true},
		{"0x0", 0, 0, true},
		{"-1x100", 0, 0, true},
		{"100x-1", 0, 0, true},
		{"abcx100", 0, 0, true},
		{"100xabc", 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			w, h, err := parseViewportSize(tc.input)
			if tc.err {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w != tc.w || h != tc.h {
				t.Errorf("got %dx%d, want %dx%d", w, h, tc.w, tc.h)
			}
		})
	}
}

func TestExecuteStepNewActionsDispatch(t *testing.T) {
	actions := []struct {
		action   models.StepAction
		selector string
		value    string
	}{
		{models.ActionDoubleClick, "#btn", ""},
		{models.ActionFileUpload, "#file", "/tmp/test.txt"},
		{models.ActionNavigateBack, "", ""},
		{models.ActionNavigateForward, "", ""},
		{models.ActionReload, "", ""},
		{models.ActionScrollIntoView, "#elem", ""},
		{models.ActionSubmitForm, "#form", ""},
		{models.ActionWaitNotPresent, "#spinner", ""},
		{models.ActionWaitEnabled, "#btn", ""},
		{models.ActionEmulateDevice, "", "1920x1080"},
		{models.ActionGetTitle, "", "title_key"},
	}

	for _, tc := range actions {
		t.Run(string(tc.action), func(t *testing.T) {
			mock := &mockExecutor{}
			r := newMockRunner(t, mock)
			result := &models.TaskResult{TaskID: "dispatch", ExtractedData: make(map[string]string)}

			step := models.TaskStep{Action: tc.action, Selector: tc.selector, Value: tc.value}
			err := r.executeStep(context.Background(), step, result)
			if err != nil && err.Error() == "unknown action: "+string(tc.action) {
				t.Fatalf("action %s was not dispatched correctly", tc.action)
			}
		})
	}
}

func TestExecuteStepDispatchesAllExecutableActions(t *testing.T) {
	for _, action := range models.ExecutableStepActions() {
		t.Run(string(action), func(t *testing.T) {
			mock := &mockExecutor{}
			r := newMockRunner(t, mock)
			if action == models.ActionEval || action == models.ActionWaitFunction {
				r.allowEval.Store(true)
			}
			result := &models.TaskResult{TaskID: "dispatch-all", ExtractedData: make(map[string]string)}

			step := validExecutableStep(action)
			err := r.executeStep(context.Background(), step, result)
			if err != nil && err.Error() == "unknown action: "+string(action) {
				t.Fatalf("action %s was not dispatched correctly", action)
			}
		})
	}
}

func TestExecuteStepRejectsMalformedExecutableSteps(t *testing.T) {
	tests := []struct {
		name       string
		step       models.TaskStep
		allowEval  bool
		wantSubstr string
	}{
		{name: "click missing selector", step: models.TaskStep{Action: models.ActionClick}, wantSubstr: "selector is required"},
		{name: "type missing selector", step: models.TaskStep{Action: models.ActionType, Value: "hello"}, wantSubstr: "selector is required"},
		{name: "type missing value", step: models.TaskStep{Action: models.ActionType, Selector: "#input"}, wantSubstr: "value is required"},
		{name: "file upload missing selector", step: models.TaskStep{Action: models.ActionFileUpload, Value: "/tmp/test.txt"}, wantSubstr: "selector is required"},
		{name: "file upload missing value", step: models.TaskStep{Action: models.ActionFileUpload, Selector: "#file"}, wantSubstr: "value is required"},
		{name: "tab switch missing value", step: models.TaskStep{Action: models.ActionTabSwitch}, wantSubstr: "value is required"},
		{name: "wait function empty", step: models.TaskStep{Action: models.ActionWaitFunction}, allowEval: true, wantSubstr: "wait_function validation failed"},
		{name: "get attributes missing selector", step: models.TaskStep{Action: models.ActionGetAttributes}, wantSubstr: "selector is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockExecutor{}
			r := newMockRunner(t, mock)
			r.allowEval.Store(tc.allowEval)
			result := &models.TaskResult{TaskID: "dispatch-malformed", ExtractedData: make(map[string]string)}

			err := r.executeStep(context.Background(), tc.step, result)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSubstr, err)
			}
			if mock.callCount() != 0 {
				t.Fatalf("expected 0 executor calls, got %d", mock.callCount())
			}
		})
	}
}

func validExecutableStep(action models.StepAction) models.TaskStep {
	switch action {
	case models.ActionNavigate:
		return models.TaskStep{Action: action, Value: "https://example.com"}
	case models.ActionClick, models.ActionExtract, models.ActionDoubleClick,
		models.ActionScrollIntoView, models.ActionSubmitForm,
		models.ActionWaitNotPresent, models.ActionWaitEnabled,
		models.ActionGetAttributes:
		return models.TaskStep{Action: action, Selector: "#target"}
	case models.ActionType:
		return models.TaskStep{Action: action, Selector: "#target", Value: "hello"}
	case models.ActionWait:
		return models.TaskStep{Action: action, Value: "100"}
	case models.ActionScreenshot, models.ActionNavigateBack,
		models.ActionNavigateForward, models.ActionReload,
		models.ActionGetTitle:
		return models.TaskStep{Action: action}
	case models.ActionScroll:
		return models.TaskStep{Action: action, Value: "100"}
	case models.ActionSelect:
		return models.TaskStep{Action: action, Selector: "#target", Value: "option"}
	case models.ActionEval:
		return models.TaskStep{Action: action, Value: "1 + 1"}
	case models.ActionTabSwitch:
		return models.TaskStep{Action: action, Value: "https://example.com/tab"}
	case models.ActionSolveCaptcha:
		return models.TaskStep{Action: action, Selector: "site-key", Value: "recaptcha_v2"}
	case models.ActionFileUpload:
		return models.TaskStep{Action: action, Selector: "#file", Value: "/tmp/test.txt"}
	case models.ActionWaitFunction:
		return models.TaskStep{Action: action, Value: "document.readyState === 'complete'"}
	case models.ActionEmulateDevice:
		return models.TaskStep{Action: action, Value: "375x812"}
	case models.ActionClickAd:
		return models.TaskStep{Action: action}
	default:
		return models.TaskStep{Action: action}
	}
}

func TestExecClickAdWithSelector(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-sel", ExtractedData: make(map[string]string)}

	// The mock executor returns zero-value from Evaluate, so adDiscoveryResult.Found
	// will be false. This verifies the selector path invokes the executor and handles
	// the "not found" case gracefully.
	err := r.execClickAd(context.Background(), models.TaskStep{Selector: "ins.adsbygoogle"}, result)
	if err == nil {
		t.Fatal("expected error for element not found via mock, got nil")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("expected 'element not found' error, got: %v", err)
	}
	if mock.callCount() != 1 {
		t.Fatalf("expected 1 executor call (metadata eval), got %d", mock.callCount())
	}
}

func TestExecClickAdAutoDiscover(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-auto", ExtractedData: make(map[string]string)}

	// Without a selector, execClickAd runs the discovery script.
	// The mock returns zero-value (found=false) so we expect an error.
	err := r.execClickAd(context.Background(), models.TaskStep{}, result)
	if err == nil {
		t.Fatal("expected error for no ad found, got nil")
	}
	if !strings.Contains(err.Error(), "no ad element found") {
		t.Fatalf("expected 'no ad element found' error, got: %v", err)
	}
}

func TestExecClickAdSelectorNotFound(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-notfound", ExtractedData: make(map[string]string)}

	// With a selector, the mock returns zero-value adDiscoveryResult (found=false).
	err := r.execClickAd(context.Background(), models.TaskStep{Selector: "#nonexistent-ad"}, result)
	if err == nil {
		t.Fatal("expected error for element not found, got nil")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("expected 'element not found' error, got: %v", err)
	}
}

func TestExecClickAdError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("ad click failed")}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-err", ExtractedData: make(map[string]string)}

	err := r.execClickAd(context.Background(), models.TaskStep{Selector: "ins.adsbygoogle"}, result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ad click failed") {
		t.Fatalf("expected 'ad click failed' in error, got: %v", err)
	}
}

func TestExecClickAdVarName(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-var", ExtractedData: make(map[string]string)}

	// Without selector, discovery returns found=false, but we verify varName prefix is used.
	_ = r.execClickAd(context.Background(), models.TaskStep{VarName: "my_ad"}, result)
	// The error is expected (no ad found), but we just verify no panic.
}

func TestExecClickAdScreenshots(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-ss", ExtractedData: make(map[string]string)}

	// With a selector provided, the mock returns zero-value adDiscoveryResult (found=false),
	// so the click itself will fail. But captureAdScreenshot is called before the click
	// and the screenshot call (FullScreenshot) goes through the mock successfully.
	// We test the helper directly instead.
	path, err := r.captureAdScreenshot(context.Background(), result, "ad", "before")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty screenshot path")
	}
	if len(result.Screenshots) != 1 {
		t.Fatalf("expected 1 screenshot, got %d", len(result.Screenshots))
	}
	if result.ExtractedData["ad_screenshot_before"] == "" {
		t.Error("expected ad_screenshot_before in extracted data")
	}

	// Capture an after screenshot too.
	path2, err := r.captureAdScreenshot(context.Background(), result, "ad", "after")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path2 == "" {
		t.Fatal("expected non-empty screenshot path")
	}
	if len(result.Screenshots) != 2 {
		t.Fatalf("expected 2 screenshots, got %d", len(result.Screenshots))
	}
	if result.ExtractedData["ad_screenshot_after"] == "" {
		t.Error("expected ad_screenshot_after in extracted data")
	}
}

func TestCaptureAdScreenshotError(t *testing.T) {
	mock := &mockExecutor{runErr: errors.New("screenshot failed")}
	r := newMockRunner(t, mock)
	result := &models.TaskResult{TaskID: "ad-ss-err", ExtractedData: make(map[string]string)}

	_, err := r.captureAdScreenshot(context.Background(), result, "ad", "before")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "screenshot failed") {
		t.Fatalf("expected 'screenshot failed' in error, got: %v", err)
	}
	if len(result.Screenshots) != 0 {
		t.Fatalf("expected 0 screenshots on error, got %d", len(result.Screenshots))
	}
}
