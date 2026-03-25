package validation

import (
	"errors"
	"strings"
	"testing"

	"flowpilot/internal/models"
)

func TestValidateTaskName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid simple", "My Task", nil},
		{"valid with numbers", "Task 123", nil},
		{"valid unicode", "Tarea de prueba", nil},
		{"empty string", "", ErrEmptyName},
		{"whitespace only", "   ", ErrEmptyName},
		{"too long", strings.Repeat("a", 256), ErrNameTooLong},
		{"exactly 255", strings.Repeat("b", 255), nil},
		{"control char tab", "task\ttab", ErrNameControlChars},
		{"control char newline", "task\nnewline", ErrNameControlChars},
		{"control char null", "task\x00null", ErrNameControlChars},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskName(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTaskURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid https", "https://example.com", nil},
		{"valid http", "http://example.com/path", nil},
		{"valid with port", "https://example.com:8080/path", nil},
		{"empty string", "", ErrEmptyURL},
		{"whitespace only", "   ", ErrEmptyURL},
		{"no scheme", "example.com", ErrInvalidURL},
		{"ftp scheme", "ftp://example.com", ErrInvalidURLScheme},
		{"javascript scheme", "javascript:alert(1)", ErrInvalidURLScheme},
		{"data scheme", "data:text/html,<h1>hi</h1>", ErrInvalidURLScheme},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskURL(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTaskSteps(t *testing.T) {
	tests := []struct {
		name      string
		steps     []models.TaskStep
		allowEval bool
		wantErr   error
	}{
		{
			"valid navigate and click",
			[]models.TaskStep{
				{Action: models.ActionNavigate, Value: "https://example.com"},
				{Action: models.ActionClick, Selector: "#btn"},
			},
			false,
			nil,
		},
		{
			"valid type step",
			[]models.TaskStep{
				{Action: models.ActionType, Selector: "#input", Value: "hello"},
			},
			false,
			nil,
		},
		{
			"valid wait step",
			[]models.TaskStep{
				{Action: models.ActionWait, Value: "1000"},
			},
			false,
			nil,
		},
		{
			"valid screenshot step",
			[]models.TaskStep{
				{Action: models.ActionScreenshot},
			},
			false,
			nil,
		},
		{
			"valid scroll step",
			[]models.TaskStep{
				{Action: models.ActionScroll, Value: "500"},
			},
			false,
			nil,
		},
		{
			"empty steps",
			[]models.TaskStep{},
			false,
			ErrNoSteps,
		},
		{
			"nil steps",
			nil,
			false,
			ErrNoSteps,
		},
		{
			"invalid action",
			[]models.TaskStep{
				{Action: "bogus"},
			},
			false,
			ErrInvalidStepAction,
		},
		{
			"navigate missing value",
			[]models.TaskStep{
				{Action: models.ActionNavigate},
			},
			false,
			ErrStepMissingValue,
		},
		{
			"navigate invalid url",
			[]models.TaskStep{
				{Action: models.ActionNavigate, Value: "not-a-url"},
			},
			false,
			ErrStepInvalidURL,
		},
		{
			"click missing selector",
			[]models.TaskStep{
				{Action: models.ActionClick},
			},
			false,
			ErrStepMissingSelector,
		},
		{
			"type missing selector",
			[]models.TaskStep{
				{Action: models.ActionType, Value: "hello"},
			},
			false,
			ErrStepMissingSelector,
		},
		{
			"extract missing selector",
			[]models.TaskStep{
				{Action: models.ActionExtract},
			},
			false,
			ErrStepMissingSelector,
		},
		{
			"select missing selector",
			[]models.TaskStep{
				{Action: models.ActionSelect, Value: "opt1"},
			},
			false,
			ErrStepMissingSelector,
		},
		{
			"eval blocked by default",
			[]models.TaskStep{
				{Action: models.ActionEval, Value: "document.title"},
			},
			false,
			ErrEvalNotAllowed,
		},
		{
			"eval allowed when enabled",
			[]models.TaskStep{
				{Action: models.ActionEval, Value: "document.title"},
			},
			true,
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskSteps(tc.steps, tc.allowEval)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateProxyServer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid host port", "proxy.example.com:8080", nil},
		{"valid ip port", "192.168.1.1:3128", nil},
		{"valid localhost", "localhost:8080", nil},
		{"empty string", "", ErrEmptyServer},
		{"whitespace only", "   ", ErrEmptyServer},
		{"missing port", "proxy.example.com", ErrInvalidServer},
		{"missing host", ":8080", ErrInvalidServer},
		{"no colon", "proxyexample", ErrInvalidServer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProxyServer(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name    string
		input   models.TaskPriority
		wantErr error
	}{
		{"low", models.PriorityLow, nil},
		{"normal", models.PriorityNormal, nil},
		{"high", models.PriorityHigh, nil},
		{"zero", 0, ErrInvalidPriority},
		{"negative", -1, ErrInvalidPriority},
		{"two", 2, ErrInvalidPriority},
		{"hundred", 100, ErrInvalidPriority},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePriority(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateProxyProtocol(t *testing.T) {
	tests := []struct {
		name    string
		input   models.ProxyProtocol
		wantErr error
	}{
		{"http", models.ProxyHTTP, nil},
		{"https", models.ProxyHTTPS, nil},
		{"socks5", models.ProxySOCKS5, nil},
		{"empty", "", ErrInvalidProtocol},
		{"socks4", models.ProxyProtocol("socks4"), ErrInvalidProtocol},
		{"ftp", models.ProxyProtocol("ftp"), ErrInvalidProtocol},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProxyProtocol(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTask(t *testing.T) {
	validSteps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#btn"},
	}

	tests := []struct {
		name     string
		taskName string
		url      string
		steps    []models.TaskStep
		priority models.TaskPriority
		wantErr  error
	}{
		{"valid task", "My Task", "https://example.com", validSteps, models.PriorityNormal, nil},
		{"invalid name", "", "https://example.com", validSteps, models.PriorityNormal, ErrEmptyName},
		{"invalid url", "Task", "bad-url", validSteps, models.PriorityNormal, ErrInvalidURL},
		{"invalid steps", "Task", "https://example.com", nil, models.PriorityNormal, ErrNoSteps},
		{"invalid priority", "Task", "https://example.com", validSteps, 99, ErrInvalidPriority},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTask(tc.taskName, tc.url, tc.steps, tc.priority, false)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateProxy(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		protocol models.ProxyProtocol
		wantErr  error
	}{
		{"valid proxy", "proxy.example.com:8080", models.ProxyHTTP, nil},
		{"invalid server", "nope", models.ProxyHTTP, ErrInvalidServer},
		{"invalid protocol", "proxy.example.com:8080", "bogus", ErrInvalidProtocol},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProxy(tc.server, tc.protocol)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// --- ValidateTags Tests ---

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		wantErr error
	}{
		{"nil tags", nil, nil},
		{"empty tags", []string{}, nil},
		{"single valid tag", []string{"web"}, nil},
		{"multiple valid tags", []string{"web", "automation", "test"}, nil},
		{"max 20 tags", make20Tags(), nil},
		{"too many tags", make21Tags(), ErrTooManyTags},
		{"empty tag", []string{"good", ""}, ErrTagEmpty},
		{"whitespace-only tag", []string{"good", "   "}, ErrTagEmpty},
		{"tag too long", []string{strings.Repeat("x", 51)}, ErrTagTooLong},
		{"tag exactly 50 chars", []string{strings.Repeat("y", 50)}, nil},
		{"tag with control chars", []string{"bad\ttag"}, ErrTagControlChars},
		{"tag with newline", []string{"bad\ntag"}, ErrTagControlChars},
		{"tag with null byte", []string{"bad\x00tag"}, ErrTagControlChars},
		{"valid unicode tag", []string{"日本語タグ"}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTags(tc.tags)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func make20Tags() []string {
	tags := make([]string, 20)
	for i := range tags {
		tags[i] = "tag-" + strings.Repeat("a", i+1)
	}
	return tags
}

func make21Tags() []string {
	tags := make([]string, 21)
	for i := range tags {
		tags[i] = "tag-" + strings.Repeat("b", i+1)
	}
	return tags
}

// --- ValidateStatus Tests ---

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr error
	}{
		{"pending", "pending", nil},
		{"queued", "queued", nil},
		{"running", "running", nil},
		{"completed", "completed", nil},
		{"failed", "failed", nil},
		{"cancelled", "cancelled", nil},
		{"retrying", "retrying", nil},
		{"empty", "", ErrInvalidStatus},
		{"invalid", "unknown", ErrInvalidStatus},
		{"uppercase", "PENDING", ErrInvalidStatus},
		{"mixed case", "Running", ErrInvalidStatus},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStatus(tc.status)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// --- ValidateBatchInput Tests ---

func TestValidateBatchInput(t *testing.T) {
	validInput := models.AdvancedBatchInput{
		FlowID:         "flow-1",
		URLs:           []string{"https://example.com", "https://example.org"},
		NamingTemplate: "Task {{index}} - {{domain}}",
		Priority:       5,
	}

	tests := []struct {
		name    string
		input   models.AdvancedBatchInput
		wantErr error
	}{
		{"valid input", validInput, nil},
		{"empty urls", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{}, Priority: 5}, ErrEmptyURL},
		{"valid with flow id", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5}, nil},
		{"invalid priority", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 99}, ErrInvalidPriority},
		{"invalid url in list", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com", "not-a-url"}, Priority: 5}, ErrInvalidURL},
		{"invalid template", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5, NamingTemplate: "{{invalid}}"}, ErrInvalidTemplate},
		{"valid template with all vars", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5, NamingTemplate: "{{url}} - {{domain}} - {{index}} - {{name}}"}, nil},
		{"empty template", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5, NamingTemplate: ""}, nil},
		{"plain text template", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5, NamingTemplate: "Task Name"}, nil},
		{"tags too many", models.AdvancedBatchInput{FlowID: "f1", URLs: []string{"https://example.com"}, Priority: 5, Tags: make21Tags()}, ErrTooManyTags},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBatchInput(tc.input)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// --- ValidateBatchTemplate Tests ---

func TestValidateBatchTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     bool
	}{
		{"plain text", "Task Name", true},
		{"empty", "", true},
		{"valid url", "{{url}}", true},
		{"valid domain", "{{domain}}", true},
		{"valid index", "{{index}}", true},
		{"valid name", "{{name}}", true},
		{"all valid vars", "{{url}} - {{domain}} - {{index}} - {{name}}", true},
		{"mixed text and vars", "Task {{index}}: {{domain}}", true},
		{"invalid var", "{{invalid}}", false},
		{"unclosed brace", "{{url", false},
		{"partial invalid", "{{url}} - {{bad}}", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := models.ValidateBatchTemplate(tc.template)
			if got != tc.want {
				t.Errorf("ValidateBatchTemplate(%q): got %v, want %v", tc.template, got, tc.want)
			}
		})
	}
}

// --- ValidateTaskSteps edge cases ---

func TestValidateTaskStepsTabSwitch(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionTabSwitch, Value: "1"},
	}
	err := ValidateTaskSteps(steps, false)
	if err != nil {
		t.Errorf("tab_switch step should be valid: %v", err)
	}
}

func TestValidateTaskStepsSupportsAllModelActions(t *testing.T) {
	for _, action := range models.SupportedStepActions() {
		t.Run(string(action), func(t *testing.T) {
			step := validTaskStepForValidation(action)
			allowEval := action == models.ActionEval || action == models.ActionWaitFunction
			if err := ValidateTaskSteps([]models.TaskStep{step}, allowEval); err != nil {
				t.Fatalf("action %s should validate, got: %v", action, err)
			}
		})
	}
}

func validTaskStepForValidation(action models.StepAction) models.TaskStep {
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
		models.ActionEndLoop, models.ActionBreakLoop, models.ActionGetTitle:
		return models.TaskStep{Action: action}
	case models.ActionScroll:
		return models.TaskStep{Action: action, Value: "100"}
	case models.ActionSelect:
		return models.TaskStep{Action: action, Selector: "#target", Value: "option"}
	case models.ActionEval:
		return models.TaskStep{Action: action, Value: "1 + 1"}
	case models.ActionTabSwitch:
		return models.TaskStep{Action: action, Value: "https://example.com/tab"}
	case models.ActionIfElement:
		return models.TaskStep{Action: action, Selector: "#target"}
	case models.ActionIfText:
		return models.TaskStep{Action: action, Selector: "#target", Condition: "contains:ok"}
	case models.ActionIfURL:
		return models.TaskStep{Action: action, Condition: "contains:example.com"}
	case models.ActionLoop:
		return models.TaskStep{Action: action, Value: "3"}
	case models.ActionGoto:
		return models.TaskStep{Action: action, JumpTo: "done"}
	case models.ActionSolveCaptcha:
		return models.TaskStep{Action: action, Selector: "site-key", Value: "recaptcha_v2"}
	case models.ActionFileUpload:
		return models.TaskStep{Action: action, Selector: "#file", Value: "/tmp/test.txt"}
	case models.ActionWaitFunction:
		return models.TaskStep{Action: action, Value: "document.readyState === 'complete'"}
	case models.ActionEmulateDevice:
		return models.TaskStep{Action: action, Value: "375x812"}
	default:
		return models.TaskStep{Action: action}
	}
}

func TestValidateTaskStepsMultipleErrors(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: "bogus"},
	}
	err := ValidateTaskSteps(steps, false)
	if !errors.Is(err, ErrInvalidStepAction) {
		t.Errorf("expected ErrInvalidStepAction, got: %v", err)
	}
}

func TestValidateTaskStepsUnknownActionMessage(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#btn"},
		{Action: "xyzzy"},
	}
	err := ValidateTaskSteps(steps, false)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	want := "step 3: unknown action xyzzy"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q should contain %q", err.Error(), want)
	}
	if !errors.Is(err, ErrInvalidStepAction) {
		t.Errorf("expected ErrInvalidStepAction in chain, got: %v", err)
	}
}

func TestValidateTaskStepsUnknownActionFirstStep(t *testing.T) {
	steps := []models.TaskStep{
		{Action: "bad_action"},
	}
	err := ValidateTaskSteps(steps, false)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	want := "step 1: unknown action bad_action"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q should contain %q", err.Error(), want)
	}
}

func TestValidateTaskStepsSelectMissingSelector(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionSelect, Value: "option1", Selector: ""},
	}
	err := ValidateTaskSteps(steps, false)
	if !errors.Is(err, ErrStepMissingSelector) {
		t.Errorf("expected ErrStepMissingSelector for select without selector, got: %v", err)
	}
}

func TestValidateBatchInputMissingFlowID(t *testing.T) {
	input := models.AdvancedBatchInput{
		FlowID:   "",
		URLs:     []string{"https://example.com"},
		Priority: 5,
	}
	err := ValidateBatchInput(input)
	if err == nil {
		t.Error("expected error for missing flow ID")
	}
}

func TestValidateBatchInputWhitespaceFlowID(t *testing.T) {
	input := models.AdvancedBatchInput{
		FlowID:   "   ",
		URLs:     []string{"https://example.com"},
		Priority: 5,
	}
	err := ValidateBatchInput(input)
	if err == nil {
		t.Error("expected error for whitespace-only flow ID")
	}
}

func TestValidatePagination(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		status   string
		tag      string
		wantErr  error
	}{
		{"valid defaults", 1, 20, "", "", nil},
		{"valid with status", 1, 50, "running", "", nil},
		{"valid with tag", 2, 10, "", "web", nil},
		{"valid with both", 3, 100, "completed", "automation", nil},
		{"status all", 1, 20, "all", "", nil},
		{"max page size", 1, 200, "", "", nil},
		{"min page size", 1, 1, "", "", nil},
		{"page zero", 0, 20, "", "", ErrInvalidPage},
		{"page negative", -1, 20, "", "", ErrInvalidPage},
		{"pageSize zero", 1, 0, "", "", ErrInvalidPageSize},
		{"pageSize negative", 1, -5, "", "", ErrInvalidPageSize},
		{"pageSize too large", 1, 201, "", "", ErrInvalidPageSize},
		{"invalid status", 1, 20, "bogus", "", ErrInvalidFilterStatus},
		{"uppercase status", 1, 20, "RUNNING", "", ErrInvalidFilterStatus},
		{"tag too long", 1, 20, "", strings.Repeat("x", 51), ErrTagFilterTooLong},
		{"tag exactly 50", 1, 20, "", strings.Repeat("y", 50), nil},
		{"tag control char", 1, 20, "", "bad\ttag", ErrTagFilterControl},
		{"tag newline", 1, 20, "", "bad\ntag", ErrTagFilterControl},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePagination(tc.page, tc.pageSize, tc.status, tc.tag)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTaskURLWithQueryParams(t *testing.T) {
	err := ValidateTaskURL("https://example.com/path?query=value&foo=bar")
	if err != nil {
		t.Errorf("URL with query params should be valid: %v", err)
	}
}

func TestValidateTaskURLWithFragment(t *testing.T) {
	err := ValidateTaskURL("https://example.com/path#section")
	if err != nil {
		t.Errorf("URL with fragment should be valid: %v", err)
	}
}

// --- ValidateTimeout Tests ---

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		wantErr error
	}{
		{"zero (default)", 0, nil},
		{"valid 60 seconds", 60, nil},
		{"valid 300 seconds", 300, nil},
		{"valid max 3600", 3600, nil},
		{"valid 1 second", 1, nil},
		{"negative", -1, ErrInvalidTimeout},
		{"negative large", -100, ErrInvalidTimeout},
		{"too large", 3601, ErrInvalidTimeout},
		{"way too large", 99999, ErrInvalidTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTimeout(tc.timeout)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTaskLoggingPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  *models.TaskLoggingPolicy
		wantErr error
	}{
		{"nil policy", nil, nil},
		{"default via zero", &models.TaskLoggingPolicy{MaxExecutionLogs: 0}, nil},
		{"valid custom", &models.TaskLoggingPolicy{MaxExecutionLogs: 250}, nil},
		{"valid max", &models.TaskLoggingPolicy{MaxExecutionLogs: 5000}, nil},
		{"negative", &models.TaskLoggingPolicy{MaxExecutionLogs: -1}, ErrInvalidMaxExecLogs},
		{"too large", &models.TaskLoggingPolicy{MaxExecutionLogs: 5001}, ErrInvalidMaxExecLogs},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskLoggingPolicy(tc.policy)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateProxyConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     models.ProxyConfig
		wantErr error
	}{
		{"empty direct", models.ProxyConfig{}, nil},
		{"explicit proxy", models.ProxyConfig{Server: "proxy.example.com:8080", Protocol: models.ProxyHTTP}, nil},
		{"auto proxy by geo", models.ProxyConfig{Geo: "US", Fallback: models.ProxyFallbackStrict}, nil},
		{"fallback only", models.ProxyConfig{Fallback: models.ProxyFallbackDirect}, nil},
		{"protocol without server", models.ProxyConfig{Protocol: models.ProxyHTTP}, ErrEmptyServer},
		{"username without server", models.ProxyConfig{Username: "user"}, ErrEmptyServer},
		{"invalid server", models.ProxyConfig{Server: "bad-server", Protocol: models.ProxyHTTP}, ErrInvalidServer},
		{"invalid fallback", models.ProxyConfig{Fallback: models.ProxyRoutingFallback("bogus")}, ErrInvalidProxyFallback},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProxyConfig(tc.cfg)
			if tc.wantErr == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("got error %v, want %v", err, tc.wantErr)
			}
		})
	}
}
