package logs

import (
	"encoding/json"
	"sync"
	"time"

	"flowpilot/internal/models"

	"github.com/chromedp/cdproto/network"
)

const (
	MaxNetworkLogEntries = 10_000
	MaxPendingRequests   = 5_000
)

// NetworkLogger captures HAR-like network timing logs.
type NetworkLogger struct {
	mu         sync.Mutex
	taskID     string
	stepIndex  int
	startTimes map[network.RequestID]time.Time
	requests   map[network.RequestID]*network.Request
	responses  map[network.RequestID]*network.Response
	logs       []models.NetworkLog
	dropped    int
}

// NewNetworkLogger creates a network logger for a task.
func NewNetworkLogger(taskID string) *NetworkLogger {
	return &NetworkLogger{
		taskID:     taskID,
		startTimes: make(map[network.RequestID]time.Time, 64),
		requests:   make(map[network.RequestID]*network.Request, 64),
		responses:  make(map[network.RequestID]*network.Response, 64),
		logs:       make([]models.NetworkLog, 0, 128),
	}
}

// SetStepIndex associates upcoming requests with a step.
func (n *NetworkLogger) SetStepIndex(stepIndex int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stepIndex = stepIndex
}

// HandleRequestWillBeSent records the start time of a request.
func (n *NetworkLogger) HandleRequestWillBeSent(ev *network.EventRequestWillBeSent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.startTimes) >= MaxPendingRequests {
		return
	}
	n.startTimes[ev.RequestID] = time.Now()
	n.requests[ev.RequestID] = ev.Request
}

// HandleResponseReceived stores response metadata for a request.
func (n *NetworkLogger) HandleResponseReceived(ev *network.EventResponseReceived) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.responses[ev.RequestID] = ev.Response
}

// HandleLoadingFinished records completion and builds a network log entry.
func (n *NetworkLogger) HandleLoadingFinished(ev *network.EventLoadingFinished, response *network.Response) {
	n.mu.Lock()
	defer n.mu.Unlock()
	start, ok := n.startTimes[ev.RequestID]
	if !ok {
		start = time.Now()
	}
	delete(n.startTimes, ev.RequestID)
	req := n.requests[ev.RequestID]
	resp := n.responses[ev.RequestID]
	if response != nil {
		resp = response
	}
	if resp == nil || req == nil {
		return
	}
	if len(n.logs) >= MaxNetworkLogEntries {
		n.dropped++
		delete(n.requests, ev.RequestID)
		delete(n.responses, ev.RequestID)
		return
	}
	log := models.NetworkLog{
		TaskID:     n.taskID,
		StepIndex:  n.stepIndex,
		RequestURL: resp.URL,
		Method:     req.Method,
		StatusCode: int(resp.Status),
		MimeType:   resp.MimeType,
		DurationMs: time.Since(start).Milliseconds(),
		Timestamp:  time.Now(),
	}
	if headers, err := json.Marshal(resp.Headers); err == nil {
		log.ResponseHeaders = string(headers)
	}
	if reqHeaders, err := json.Marshal(req.Headers); err == nil {
		log.RequestHeaders = string(reqHeaders)
	}
	log.ResponseSize = int64(ev.EncodedDataLength)
	n.logs = append(n.logs, log)
	delete(n.requests, ev.RequestID)
	delete(n.responses, ev.RequestID)
}

// HandleLoadingFailed cleans up tracking maps for failed or cancelled requests
// that will never receive a LoadingFinished event.
func (n *NetworkLogger) HandleLoadingFailed(requestID network.RequestID) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.startTimes, requestID)
	delete(n.requests, requestID)
	delete(n.responses, requestID)
}

// Logs returns accumulated network logs.
func (n *NetworkLogger) Logs() []models.NetworkLog {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]models.NetworkLog, len(n.logs))
	copy(result, n.logs)
	return result
}
