package models

import (
	"errors"
	"testing"
)

func TestClassifyErrorWithContext(t *testing.T) {
	tests := []struct {
		name              string
		err               error
		expectedCode      ErrorCode
		expectedRetryable bool
	}{
		{
			name:              "timeout error",
			err:               errors.New("context deadline exceeded"),
			expectedCode:      ErrCodeTimeout,
			expectedRetryable: true,
		},
		{
			name:              "selector not found",
			err:               errors.New("waiting for selector #not-there"),
			expectedCode:      ErrCodeSelectorNotFnd,
			expectedRetryable: false,
		},
		{
			name:              "navigation failed",
			err:               errors.New("navigation failed"),
			expectedCode:      ErrCodeNavFailed,
			expectedRetryable: true,
		},
		{
			name:              "proxy failed",
			err:               errors.New("proxy error: connection refused"),
			expectedCode:      ErrCodeProxyFailed,
			expectedRetryable: true,
		},
		{
			name:              "network error",
			err:               errors.New("net::err_connection_reset"),
			expectedCode:      ErrCodeNetworkError,
			expectedRetryable: true,
		},
		{
			name:              "nil error",
			err:               nil,
			expectedCode:      "",
			expectedRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ctx := ClassifyErrorWithContext(
				tt.err,
				"task-123",
				0,
				"click",
				".button",
				"proxy.example.com:8080",
				"https://example.com",
				150,
				0,
			)

			if code != tt.expectedCode {
				t.Errorf("got error code %q, want %q", code, tt.expectedCode)
			}

			if tt.err == nil {
				if ctx != nil {
					t.Errorf("expected nil context for nil error, got %v", ctx)
				}
				return
			}

			if ctx == nil {
				t.Errorf("expected non-nil context, got nil")
				return
			}

			if ctx.TaskID != "task-123" {
				t.Errorf("got TaskID %q, want task-123", ctx.TaskID)
			}

			if ctx.Retryable != tt.expectedRetryable {
				t.Errorf("got Retryable %v, want %v", ctx.Retryable, tt.expectedRetryable)
			}

			if ctx.DurationMs != 150 {
				t.Errorf("got DurationMs %d, want 150", ctx.DurationMs)
			}

			if ctx.StepIndex != 0 {
				t.Errorf("got StepIndex %d, want 0", ctx.StepIndex)
			}
		})
	}
}

func TestErrorContextErrorString(t *testing.T) {
	ctx := &ErrorContext{
		TaskID:       "task-123",
		StepIndex:    2,
		Action:       "click",
		Selector:     ".submit-btn",
		DurationMs:   500,
		ErrorCode:    string(ErrCodeSelectorNotFnd),
		ErrorMessage: "element not found",
		Retryable:    false,
		RetryAttempt: 0,
	}

	str := ctx.ErrorString()
	if str == "" {
		t.Errorf("ErrorString returned empty string")
	}

	if !contains(str, "task-123") {
		t.Errorf("ErrorString missing task ID")
	}
	if !contains(str, "click") {
		t.Errorf("ErrorString missing action")
	}
	if !contains(str, ".submit-btn") {
		t.Errorf("ErrorString missing selector")
	}
}

func TestErrorContextLogAttrs(t *testing.T) {
	ctx := &ErrorContext{
		TaskID:       "task-456",
		StepIndex:    1,
		Action:       "navigate",
		ProxyServer:  "proxy.example.com",
		DurationMs:   200,
		ErrorCode:    string(ErrCodeTimeout),
		ErrorMessage: "timed out",
		Retryable:    true,
		RetryAttempt: 1,
	}

	attrs := ctx.LogAttrs()
	if len(attrs) == 0 {
		t.Errorf("LogAttrs returned empty slice")
	}

	// Check that key attributes are present
	foundTaskID := false
	for i := 0; i < len(attrs)-1; i += 2 {
		if attrs[i] == "taskId" {
			foundTaskID = true
			if attrs[i+1] != "task-456" {
				t.Errorf("taskId value incorrect: %v", attrs[i+1])
			}
		}
	}
	if !foundTaskID {
		t.Errorf("LogAttrs missing taskId")
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
