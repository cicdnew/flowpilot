package models

import "time"

// TaskLifecycleEvent records a single state transition in a task's lifecycle.
type TaskLifecycleEvent struct {
	ID        string     `json:"id"`
	TaskID    string     `json:"taskId"`
	BatchID   string     `json:"batchId,omitempty"`
	FromState TaskStatus `json:"fromState"`
	ToState   TaskStatus `json:"toState"`
	Error     string     `json:"error,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// QueueMetrics provides a snapshot of queue state.
//
// Field semantics:
//   - Running: tasks currently executing in browser workers.
//   - Queued: tasks waiting for a concurrency slot (submitted but not yet running).
//   - Pending: total tasks not yet finished (Queued + Running).
//   - TotalSubmitted/TotalCompleted/TotalFailed: lifetime counters since queue creation.
type QueueMetrics struct {
	Running                  int   `json:"running"`
	Queued                   int   `json:"queued"`
	Pending                  int   `json:"pending"`
	TotalSubmitted           int64 `json:"totalSubmitted"`
	TotalCompleted           int64 `json:"totalCompleted"`
	TotalFailed              int64 `json:"totalFailed"`
	TotalRetried             int64 `json:"totalRetried"`
	RunningProxied           int   `json:"runningProxied"`
	ProxyConcurrencyLimit    int   `json:"proxyConcurrencyLimit"`
	PersistenceQueueDepth    int   `json:"persistenceQueueDepth"`
	PersistenceQueueCapacity int   `json:"persistenceQueueCapacity"`
	PersistenceBatchSize     int   `json:"persistenceBatchSize"`
	WorkerUtilizationPercent float64 `json:"workerUtilizationPercent"`
	AvgStepDurationMs        float64 `json:"avgStepDurationMs"`
	LastUpdated              time.Time `json:"lastUpdated"`
}

// ErrorContext captures detailed information about an error during task execution.
type ErrorContext struct {
	TaskID        string        `json:"taskId"`
	StepIndex     int           `json:"stepIndex,omitempty"`
	Action        string        `json:"action,omitempty"`
	Selector      string        `json:"selector,omitempty"`
	ProxyServer   string        `json:"proxyServer,omitempty"`
	URL           string        `json:"url,omitempty"`
	DurationMs    int64         `json:"durationMs"`
	Timestamp     time.Time     `json:"timestamp"`
	ErrorCode     string        `json:"errorCode"`
	ErrorMessage  string        `json:"errorMessage"`
	StackTrace    string        `json:"stackTrace,omitempty"`
	Retryable     bool          `json:"retryable"`
	RetryAttempt  int           `json:"retryAttempt,omitempty"`
}
