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
	RunningProxied           int   `json:"runningProxied"`
	ProxyConcurrencyLimit    int   `json:"proxyConcurrencyLimit"`
	PersistenceQueueDepth    int   `json:"persistenceQueueDepth"`
	PersistenceQueueCapacity int   `json:"persistenceQueueCapacity"`
	PersistenceBatchSize     int   `json:"persistenceBatchSize"`
}
