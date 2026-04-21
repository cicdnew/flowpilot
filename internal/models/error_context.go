package models

import (
	"fmt"
	"runtime/debug"
	"time"
)

// ClassifyErrorWithContext classifies an error and captures rich contextual information.
// This is the primary function for error handling in task execution.
func ClassifyErrorWithContext(err error, taskID string, stepIndex int, action string, selector string, proxyServer string, url string, durationMs int64, retryAttempt int) (ErrorCode, *ErrorContext) {
	if err == nil {
		return "", nil
	}

	errCode := ClassifyError(err)
	retryable := isRetryable(errCode)

	ctx := &ErrorContext{
		TaskID:       taskID,
		StepIndex:    stepIndex,
		Action:       action,
		Selector:     selector,
		ProxyServer:  proxyServer,
		URL:          url,
		DurationMs:   durationMs,
		Timestamp:    time.Now(),
		ErrorCode:    string(errCode),
		ErrorMessage: err.Error(),
		Retryable:    retryable,
		RetryAttempt: retryAttempt,
	}

	// Capture stack trace for certain error types
	if errCode == ErrCodeUnknown {
		ctx.StackTrace = string(debug.Stack())
	}

	return errCode, ctx
}

// isRetryable determines if an error code is retryable.
func isRetryable(errCode ErrorCode) bool {
	retryableCodes := map[ErrorCode]bool{
		ErrCodeNetworkError:   true,
		ErrCodeTimeout:        true,
		ErrCodeProxyFailed:    true,
		ErrCodeNavFailed:      true,
	}
	return retryableCodes[errCode]
}

// ErrorString returns a formatted string representation of the error context.
func (ec *ErrorContext) ErrorString() string {
	if ec == nil {
		return ""
	}
	s := fmt.Sprintf("Task %s (step %d, %s): %s [%s] after %dms",
		ec.TaskID, ec.StepIndex, ec.Action, ec.ErrorMessage, ec.ErrorCode, ec.DurationMs)
	if ec.Selector != "" {
		s += fmt.Sprintf(" (selector: %s)", ec.Selector)
	}
	if ec.ProxyServer != "" {
		s += fmt.Sprintf(" via %s", ec.ProxyServer)
	}
	if ec.RetryAttempt > 0 {
		s += fmt.Sprintf(" [attempt %d]", ec.RetryAttempt)
	}
	return s
}

// LogAttrs returns structured log attributes for this error context.
// Useful for passing to structured logging functions.
func (ec *ErrorContext) LogAttrs() []interface{} {
	if ec == nil {
		return nil
	}
	attrs := []interface{}{
		"taskId", ec.TaskID,
		"errorCode", ec.ErrorCode,
		"durationMs", ec.DurationMs,
		"retryable", ec.Retryable,
	}
	if ec.StepIndex >= 0 {
		attrs = append(attrs, "stepIndex", ec.StepIndex)
	}
	if ec.Action != "" {
		attrs = append(attrs, "action", ec.Action)
	}
	if ec.Selector != "" {
		attrs = append(attrs, "selector", ec.Selector)
	}
	if ec.ProxyServer != "" {
		attrs = append(attrs, "proxyServer", ec.ProxyServer)
	}
	if ec.URL != "" {
		attrs = append(attrs, "url", ec.URL)
	}
	if ec.RetryAttempt > 0 {
		attrs = append(attrs, "retryAttempt", ec.RetryAttempt)
	}
	return attrs
}
