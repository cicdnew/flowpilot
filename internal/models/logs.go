package models

import (
	"strings"
	"time"
)

// StepLog captures detailed execution data for a single step within a task run.
type StepLog struct {
	TaskID     string     `json:"taskId"`
	StepIndex  int        `json:"stepIndex"`
	Action     StepAction `json:"action"`
	Selector   string     `json:"selector,omitempty"`
	Value      string     `json:"value,omitempty"`
	SnapshotID string     `json:"snapshotId,omitempty"`
	ErrorCode  string     `json:"errorCode,omitempty"`
	ErrorMsg   string     `json:"errorMsg,omitempty"`
	DurationMs int64      `json:"durationMs"`
	StartedAt  time.Time  `json:"startedAt"`
}

// NetworkLog captures a single network request/response during task execution (HAR-like).
type NetworkLog struct {
	TaskID          string    `json:"taskId"`
	StepIndex       int       `json:"stepIndex"`
	RequestURL      string    `json:"requestUrl"`
	Method          string    `json:"method"`
	StatusCode      int       `json:"statusCode"`
	MimeType        string    `json:"mimeType,omitempty"`
	RequestHeaders  string    `json:"requestHeaders,omitempty"`  // JSON string
	ResponseHeaders string    `json:"responseHeaders,omitempty"` // JSON string
	RequestSize     int64     `json:"requestSize"`
	ResponseSize    int64     `json:"responseSize"`
	DurationMs      int64     `json:"durationMs"`
	Error           string    `json:"error,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

// WebSocketEventType categorizes WebSocket lifecycle events.
type WebSocketEventType string

const (
	WSEventCreated       WebSocketEventType = "created"
	WSEventHandshake     WebSocketEventType = "handshake"
	WSEventFrameSent     WebSocketEventType = "frame_sent"
	WSEventFrameReceived WebSocketEventType = "frame_received"
	WSEventClosed        WebSocketEventType = "closed"
	WSEventError         WebSocketEventType = "error"
)

// MaxWSPayloadSnippet is the maximum bytes stored for a WebSocket frame payload.
const MaxWSPayloadSnippet = 512

// WebSocketLog captures a single WebSocket event during recording.
type WebSocketLog struct {
	FlowID         string             `json:"flowId"`
	StepIndex      int                `json:"stepIndex"`
	RequestID      string             `json:"requestId"`
	URL            string             `json:"url"`
	EventType      WebSocketEventType `json:"eventType"`
	Direction      string             `json:"direction,omitempty"`
	Opcode         int                `json:"opcode,omitempty"`
	PayloadSize    int64              `json:"payloadSize"`
	PayloadSnippet string             `json:"payloadSnippet,omitempty"`
	CloseCode      int                `json:"closeCode,omitempty"`
	CloseReason    string             `json:"closeReason,omitempty"`
	ErrorMessage   string             `json:"errorMessage,omitempty"`
	Timestamp      time.Time          `json:"timestamp"`
}

// TruncatePayload returns a payload string truncated to MaxWSPayloadSnippet bytes.
func TruncatePayload(payload string) string {
	if len(payload) <= MaxWSPayloadSnippet {
		return payload
	}
	return payload[:MaxWSPayloadSnippet]
}

// ErrorCode is a standardized automation failure code.
type ErrorCode string

const (
	ErrCodeTimeout        ErrorCode = "TIMEOUT"
	ErrCodeSelectorNotFnd ErrorCode = "SELECTOR_NOT_FOUND"
	ErrCodeNavFailed      ErrorCode = "NAVIGATION_FAILED"
	ErrCodeProxyFailed    ErrorCode = "PROXY_FAILED"
	ErrCodeAuthRequired   ErrorCode = "AUTH_REQUIRED"
	ErrCodeNetworkError   ErrorCode = "NETWORK_ERROR"
	ErrCodeEvalBlocked    ErrorCode = "EVAL_BLOCKED"
	ErrCodeEvalFailed     ErrorCode = "EVAL_FAILED"
	ErrCodeScreenshotFail ErrorCode = "SCREENSHOT_FAILED"
	ErrCodeUnknown        ErrorCode = "UNKNOWN"
)

// ClassifyError maps a raw error string to a standardized ErrorCode.
func ClassifyError(err error) ErrorCode {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case containsAny(msg, "context deadline exceeded", "timeout"):
		return ErrCodeTimeout
	case containsAny(msg, "selector", "not found", "waiting for selector"):
		return ErrCodeSelectorNotFnd
	case containsAny(msg, "navigate", "navigation"):
		return ErrCodeNavFailed
	case containsAny(msg, "proxy", "proxy auth"):
		return ErrCodeProxyFailed
	case containsAny(msg, "net::err_", "network"):
		return ErrCodeNetworkError
	case containsAny(msg, "eval", "alloweval"):
		return ErrCodeEvalBlocked
	case containsAny(msg, "screenshot"):
		return ErrCodeScreenshotFail
	default:
		return ErrCodeUnknown
	}
}

func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}
