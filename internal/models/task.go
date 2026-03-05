package models

import "time"

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusRetrying  TaskStatus = "retrying"
)

// TaskPriority controls queue ordering.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 1
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 10
)

// StepAction defines what a task step does.
type StepAction string

const (
	ActionNavigate   StepAction = "navigate"
	ActionClick      StepAction = "click"
	ActionType       StepAction = "type"
	ActionWait       StepAction = "wait"
	ActionScreenshot StepAction = "screenshot"
	ActionExtract    StepAction = "extract"
	ActionScroll     StepAction = "scroll"
	ActionSelect     StepAction = "select"
	ActionEval       StepAction = "eval"
)

// TaskStep represents a single browser action within a task.
type TaskStep struct {
	Action   StepAction `json:"action"`
	Selector string     `json:"selector,omitempty"`
	Value    string     `json:"value,omitempty"`
	Timeout  int        `json:"timeout,omitempty"` // milliseconds
}

// ProxyConfig holds proxy connection details for a task.
type ProxyConfig struct {
	Server   string `json:"server"`
	Protocol string `json:"protocol,omitempty"` // http, https, socks5
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Geo      string `json:"geo,omitempty"` // country code
}

// Task represents a single automated browser task.
type Task struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	URL         string       `json:"url"`
	Steps       []TaskStep   `json:"steps"`
	Proxy       ProxyConfig  `json:"proxy"`
	Priority    TaskPriority `json:"priority"`
	Status      TaskStatus   `json:"status"`
	RetryCount  int          `json:"retryCount"`
	MaxRetries  int          `json:"maxRetries"`
	Timeout     int          `json:"timeout,omitempty"` // total task timeout in seconds, 0 = default (5 min)
	Error       string       `json:"error,omitempty"`
	Result      *TaskResult  `json:"result,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	StartedAt   *time.Time   `json:"startedAt,omitempty"`
	CompletedAt *time.Time   `json:"completedAt,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// TaskResult holds the output of a completed task.
type TaskResult struct {
	TaskID        string            `json:"taskId"`
	Success       bool              `json:"success"`
	ExtractedData map[string]string `json:"extractedData,omitempty"`
	Screenshots   []string          `json:"screenshots,omitempty"` // file paths
	Logs          []LogEntry        `json:"logs"`
	Duration      time.Duration     `json:"duration"`
	Error         string            `json:"error,omitempty"`
}

// LogEntry is a single log message from task execution.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
}

// TaskEvent is emitted to the frontend via Wails events.
type TaskEvent struct {
	TaskID string     `json:"taskId"`
	Status TaskStatus `json:"status"`
	Error  string     `json:"error,omitempty"`
	Log    *LogEntry  `json:"log,omitempty"`
}

// BatchTaskInput holds the input fields for creating a single task in a batch.
type BatchTaskInput struct {
	Name     string      `json:"name"`
	URL      string      `json:"url"`
	Steps    []TaskStep  `json:"steps"`
	Proxy    ProxyConfig `json:"proxy"`
	Priority int         `json:"priority"`
}

// BatchConfig is used to create multiple tasks at once.
type BatchConfig struct {
	Tasks       []Task `json:"tasks"`
	Concurrency int    `json:"concurrency"` // max concurrent, default 100
}
