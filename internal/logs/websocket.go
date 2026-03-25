package logs

import (
	"sync"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
)

const defaultMaxWSLogs = 10000

// WSEventCallback is called for each captured WebSocket event (for real-time streaming).
type WSEventCallback func(log models.WebSocketLog)

// WebSocketLogger captures WebSocket lifecycle and frame events via CDP.
type WebSocketLogger struct {
	mu        sync.Mutex
	flowID    string
	stepIndex int
	maxLogs   int
	urls      map[network.RequestID]string
	logs      []models.WebSocketLog
	callback  WSEventCallback
}

// NewWebSocketLogger creates a WebSocket logger for a recording flow.
func NewWebSocketLogger(flowID string) *WebSocketLogger {
	return &WebSocketLogger{
		flowID:  flowID,
		maxLogs: defaultMaxWSLogs,
		urls:    make(map[network.RequestID]string),
		logs:    []models.WebSocketLog{},
	}
}

// SetStepIndex associates upcoming events with a recorder step.
func (w *WebSocketLogger) SetStepIndex(stepIndex int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stepIndex = stepIndex
}

// SetMaxLogs overrides the maximum number of stored log entries.
func (w *WebSocketLogger) SetMaxLogs(max int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if max > 0 {
		w.maxLogs = max
	}
}

// SetCallback registers an optional event callback for real-time streaming.
func (w *WebSocketLogger) SetCallback(cb WSEventCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callback = cb
}

func (w *WebSocketLogger) append(log models.WebSocketLog) {
	if len(w.logs) >= w.maxLogs {
		return
	}
	w.logs = append(w.logs, log)
}

func (w *WebSocketLogger) appendAndNotify(log models.WebSocketLog) {
	w.mu.Lock()
	if len(w.logs) >= w.maxLogs {
		w.mu.Unlock()
		return
	}
	w.logs = append(w.logs, log)
	cb := w.callback
	w.mu.Unlock()
	if cb != nil {
		cb(log)
	}
}

// HandleCreated records a new WebSocket connection.
func (w *WebSocketLogger) HandleCreated(ev *network.EventWebSocketCreated) {
	if ev == nil {
		return
	}
	w.mu.Lock()
	w.urls[ev.RequestID] = ev.URL
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:    w.flowID,
		StepIndex: stepIdx,
		RequestID: string(ev.RequestID),
		URL:       ev.URL,
		EventType: models.WSEventCreated,
		Timestamp: time.Now(),
	})
}

// HandleHandshake records a WebSocket handshake response.
func (w *WebSocketLogger) HandleHandshake(ev *network.EventWebSocketHandshakeResponseReceived) {
	if ev == nil {
		return
	}
	w.mu.Lock()
	url := w.urls[ev.RequestID]
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:    w.flowID,
		StepIndex: stepIdx,
		RequestID: string(ev.RequestID),
		URL:       url,
		EventType: models.WSEventHandshake,
		Timestamp: time.Now(),
	})
}

// HandleFrameSent records a WebSocket frame sent by the browser.
func (w *WebSocketLogger) HandleFrameSent(ev *network.EventWebSocketFrameSent) {
	if ev == nil || ev.Response == nil {
		return
	}
	w.mu.Lock()
	url := w.urls[ev.RequestID]
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:         w.flowID,
		StepIndex:      stepIdx,
		RequestID:      string(ev.RequestID),
		URL:            url,
		EventType:      models.WSEventFrameSent,
		Direction:      "send",
		Opcode:         int(ev.Response.Opcode),
		PayloadSize:    int64(len(ev.Response.PayloadData)),
		PayloadSnippet: models.TruncatePayload(ev.Response.PayloadData),
		Timestamp:      time.Now(),
	})
}

// HandleFrameReceived records a WebSocket frame received by the browser.
func (w *WebSocketLogger) HandleFrameReceived(ev *network.EventWebSocketFrameReceived) {
	if ev == nil || ev.Response == nil {
		return
	}
	w.mu.Lock()
	url := w.urls[ev.RequestID]
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:         w.flowID,
		StepIndex:      stepIdx,
		RequestID:      string(ev.RequestID),
		URL:            url,
		EventType:      models.WSEventFrameReceived,
		Direction:      "receive",
		Opcode:         int(ev.Response.Opcode),
		PayloadSize:    int64(len(ev.Response.PayloadData)),
		PayloadSnippet: models.TruncatePayload(ev.Response.PayloadData),
		Timestamp:      time.Now(),
	})
}

// HandleClosed records a WebSocket connection closure.
func (w *WebSocketLogger) HandleClosed(ev *network.EventWebSocketClosed) {
	if ev == nil {
		return
	}
	w.mu.Lock()
	url := w.urls[ev.RequestID]
	delete(w.urls, ev.RequestID)
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:    w.flowID,
		StepIndex: stepIdx,
		RequestID: string(ev.RequestID),
		URL:       url,
		EventType: models.WSEventClosed,
		Timestamp: time.Now(),
	})
}

// HandleFrameError records a WebSocket frame error.
func (w *WebSocketLogger) HandleFrameError(ev *network.EventWebSocketFrameError) {
	if ev == nil {
		return
	}
	w.mu.Lock()
	url := w.urls[ev.RequestID]
	stepIdx := w.stepIndex
	w.mu.Unlock()
	w.appendAndNotify(models.WebSocketLog{
		FlowID:       w.flowID,
		StepIndex:    stepIdx,
		RequestID:    string(ev.RequestID),
		URL:          url,
		EventType:    models.WSEventError,
		ErrorMessage: ev.ErrorMessage,
		Timestamp:    time.Now(),
	})
}

// Logs returns a copy of all accumulated WebSocket logs.
func (w *WebSocketLogger) Logs() []models.WebSocketLog {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]models.WebSocketLog, len(w.logs))
	copy(result, w.logs)
	return result
}
