package models

// TaskMetrics is a lightweight in-memory snapshot of task execution counters.
type TaskMetrics struct {
	Completed    int `json:"completed"`
	Failed       int `json:"failed"`
	AvgDurationMs int `json:"avgDurationMs"`
	QueueDepth   int `json:"queueDepth"`
}
