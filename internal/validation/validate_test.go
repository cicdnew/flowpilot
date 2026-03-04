package validation

import (
	"errors"
	"strings"
	"testing"

	"web-automation/internal/models"
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
