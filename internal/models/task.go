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

const DefaultMaxRetries = 3

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
	ActionNavigate        StepAction = "navigate"
	ActionClick           StepAction = "click"
	ActionType            StepAction = "type"
	ActionWait            StepAction = "wait"
	ActionScreenshot      StepAction = "screenshot"
	ActionExtract         StepAction = "extract"
	ActionScroll          StepAction = "scroll"
	ActionSelect          StepAction = "select"
	ActionEval            StepAction = "eval"
	ActionTabSwitch       StepAction = "tab_switch"
	ActionIfElement       StepAction = "if_element"
	ActionIfText          StepAction = "if_text"
	ActionIfURL           StepAction = "if_url"
	ActionLoop            StepAction = "loop"
	ActionEndLoop         StepAction = "end_loop"
	ActionBreakLoop       StepAction = "break_loop"
	ActionGoto            StepAction = "goto"
	ActionSolveCaptcha    StepAction = "solve_captcha"
	ActionDoubleClick     StepAction = "double_click"
	ActionFileUpload      StepAction = "file_upload"
	ActionNavigateBack    StepAction = "navigate_back"
	ActionNavigateForward StepAction = "navigate_forward"
	ActionReload          StepAction = "reload"
	ActionScrollIntoView  StepAction = "scroll_into_view"
	ActionSubmitForm      StepAction = "submit_form"
	ActionWaitNotPresent  StepAction = "wait_not_present"
	ActionWaitEnabled     StepAction = "wait_enabled"
	ActionWaitFunction    StepAction = "wait_function"
	ActionEmulateDevice   StepAction = "emulate_device"
	ActionGetTitle        StepAction = "get_title"
	ActionGetAttributes   StepAction = "get_attributes"
	ActionClickAd         StepAction = "click_ad"
	ActionHover           StepAction = "hover"
	ActionDragDrop        StepAction = "drag_drop"
	ActionContextClick    StepAction = "context_click"
	ActionHighlight       StepAction = "highlight"
	ActionGetCookies      StepAction = "get_cookies"
	ActionSetCookie       StepAction = "set_cookie"
	ActionDeleteCookies   StepAction = "delete_cookies"
	ActionGetStorage      StepAction = "get_storage"
	ActionSetStorage      StepAction = "set_storage"
	ActionDeleteStorage   StepAction = "delete_storage"
	ActionDownload        StepAction = "download"
	ActionSelectRandom    StepAction = "select_random"
	ActionWhile           StepAction = "while_condition"
	ActionEndWhile        StepAction = "end_while"
	ActionIfExists        StepAction = "if_exists"
	ActionIfNotExists     StepAction = "if_not_exists"
	ActionIfVisible       StepAction = "if_visible"
	ActionIfEnabled       StepAction = "if_enabled"
	ActionVariableSet     StepAction = "variable_set"
	ActionVariableMath    StepAction = "variable_math"
	ActionVariableString  StepAction = "variable_string"
	ActionDebugStep       StepAction = "debug_step"
	ActionDebugPause      StepAction = "debug_pause"
	ActionDebugResume     StepAction = "debug_resume"
	ActionAntiBot         StepAction = "anti_bot"
	ActionRandomMouse     StepAction = "random_mouse"
	ActionHumanTyping     StepAction = "human_typing"
	ActionGetSession      StepAction = "get_session"
	ActionSetSession      StepAction = "set_session"
	ActionLoadSession     StepAction = "load_session"
	ActionSaveSession     StepAction = "save_session"
	ActionCacheGet        StepAction = "cache_get"
	ActionCacheSet        StepAction = "cache_set"
	ActionCacheClear      StepAction = "cache_clear"
)

func ExecutableStepActions() []StepAction {
	return []StepAction{
		ActionNavigate,
		ActionClick,
		ActionType,
		ActionWait,
		ActionScreenshot,
		ActionExtract,
		ActionScroll,
		ActionSelect,
		ActionEval,
		ActionTabSwitch,
		ActionSolveCaptcha,
		ActionDoubleClick,
		ActionFileUpload,
		ActionNavigateBack,
		ActionNavigateForward,
		ActionReload,
		ActionScrollIntoView,
		ActionSubmitForm,
		ActionWaitNotPresent,
		ActionWaitEnabled,
		ActionWaitFunction,
		ActionEmulateDevice,
		ActionGetTitle,
		ActionGetAttributes,
		ActionClickAd,
		ActionHover,
		ActionDragDrop,
		ActionContextClick,
		ActionHighlight,
		ActionGetCookies,
		ActionSetCookie,
		ActionDeleteCookies,
		ActionGetStorage,
		ActionSetStorage,
		ActionDeleteStorage,
		ActionDownload,
		ActionSelectRandom,
		ActionVariableSet,
		ActionVariableMath,
		ActionVariableString,
		ActionDebugStep,
		ActionDebugPause,
		ActionDebugResume,
		ActionAntiBot,
		ActionRandomMouse,
		ActionHumanTyping,
		ActionGetSession,
		ActionSetSession,
		ActionLoadSession,
		ActionSaveSession,
		ActionCacheGet,
		ActionCacheSet,
		ActionCacheClear,
	}
}

func ControlFlowStepActions() []StepAction {
	return []StepAction{
		ActionIfElement,
		ActionIfText,
		ActionIfURL,
		ActionLoop,
		ActionEndLoop,
		ActionBreakLoop,
		ActionGoto,
		ActionWhile,
		ActionEndWhile,
		ActionIfExists,
		ActionIfNotExists,
		ActionIfVisible,
		ActionIfEnabled,
	}
}

func SupportedStepActions() []StepAction {
	actions := make([]StepAction, 0, len(ExecutableStepActions())+len(ControlFlowStepActions()))
	actions = append(actions, ExecutableStepActions()...)
	actions = append(actions, ControlFlowStepActions()...)
	return actions
}

var knownActionsMap = func() map[StepAction]bool {
	m := make(map[StepAction]bool, len(SupportedStepActions()))
	for _, a := range SupportedStepActions() {
		m[a] = true
	}
	return m
}()

func IsKnownAction(action StepAction) bool {
	return knownActionsMap[action]
}

// TaskStep represents a single browser action within a task.
type TaskStep struct {
	Action    StepAction `json:"action"`
	Selector  string     `json:"selector,omitempty"`
	Value     string     `json:"value,omitempty"`
	Timeout   int        `json:"timeout,omitempty"`
	Condition string     `json:"condition,omitempty"`
	Label     string     `json:"label,omitempty"`
	JumpTo    string     `json:"jumpTo,omitempty"`
	VarName   string     `json:"varName,omitempty"`
	Operator  string     `json:"operator,omitempty"`
	MaxLoops  int        `json:"maxLoops,omitempty"`
	Target    string     `json:"target,omitempty"`
	Source    string     `json:"source,omitempty"`
	Keys      string     `json:"keys,omitempty"`
	Duration  int        `json:"duration,omitempty"`
	Domain    string     `json:"domain,omitempty"`
	Name      string     `json:"name,omitempty"`
	Path      string     `json:"path,omitempty"`
	Data      string     `json:"data,omitempty"`
	Strategy  string     `json:"strategy,omitempty"`
}

// ProxyConfig holds proxy connection details for a task.
type ProxyRoutingFallback string

const (
	ProxyFallbackStrict ProxyRoutingFallback = "strict"
	ProxyFallbackAny    ProxyRoutingFallback = "any_healthy"
	ProxyFallbackDirect ProxyRoutingFallback = "direct"
)

type ProxyConfig struct {
	Server   string               `json:"server"`
	Protocol ProxyProtocol        `json:"protocol,omitempty"`
	Username string               `json:"username,omitempty"`
	Password string               `json:"password,omitempty"`
	Geo      string               `json:"geo,omitempty"`
	Fallback ProxyRoutingFallback `json:"fallback,omitempty"`
}

type TaskLoggingPolicy struct {
	CaptureStepLogs    *bool `json:"captureStepLogs,omitempty"`
	CaptureNetworkLogs *bool `json:"captureNetworkLogs,omitempty"`
	CaptureScreenshots *bool `json:"captureScreenshots,omitempty"`
	MaxExecutionLogs   int   `json:"maxExecutionLogs,omitempty"`
}

// Task represents a single automated browser task.
type Task struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	URL            string             `json:"url"`
	Steps          []TaskStep         `json:"steps"`
	Proxy          ProxyConfig        `json:"proxy"`
	Priority       TaskPriority       `json:"priority"`
	Status         TaskStatus         `json:"status"`
	RetryCount     int                `json:"retryCount"`
	MaxRetries     int                `json:"maxRetries"`
	Timeout        int                `json:"timeout,omitempty"` // total task timeout in seconds, 0 = default (5 min)
	Error          string             `json:"error,omitempty"`
	Result         *TaskResult        `json:"result,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	StartedAt      *time.Time         `json:"startedAt,omitempty"`
	CompletedAt    *time.Time         `json:"completedAt,omitempty"`
	Tags           []string           `json:"tags,omitempty"`
	BatchID        string             `json:"batchId,omitempty"`
	FlowID         string             `json:"flowId,omitempty"`
	Headless       bool               `json:"headless"`
	LoggingPolicy  *TaskLoggingPolicy `json:"loggingPolicy,omitempty"`
	WebhookURL     string             `json:"webhookUrl,omitempty"`
	WebhookEvents  []string           `json:"webhookEvents,omitempty"`
}

// TaskResult holds the output of a completed task.
type TaskResult struct {
	TaskID        string            `json:"taskId"`
	Success       bool              `json:"success"`
	ExtractedData map[string]string `json:"extractedData,omitempty"`
	Screenshots   []string          `json:"screenshots,omitempty"` // file paths
	Logs          []LogEntry        `json:"logs"`
	StepLogs      []StepLog         `json:"stepLogs,omitempty"`
	NetworkLogs   []NetworkLog      `json:"networkLogs,omitempty"`
	Duration      time.Duration     `json:"duration"`
	Error         string            `json:"error,omitempty"`
	LogLimit      int               `json:"-"`
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
	Name          string             `json:"name"`
	URL           string             `json:"url"`
	Steps         []TaskStep         `json:"steps"`
	Proxy         ProxyConfig        `json:"proxy"`
	Priority      int                `json:"priority"`
	Timeout       int                `json:"timeout,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	LoggingPolicy *TaskLoggingPolicy `json:"loggingPolicy,omitempty"`
	Headless      bool               `json:"headless"`
}

// PaginatedTasks holds a page of tasks with metadata.
type PaginatedTasks struct {
	Tasks      []Task `json:"tasks"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	TotalPages int    `json:"totalPages"`
}

type TaskExport struct {
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	Name       string    `json:"name"`
	Task       Task      `json:"task"`
}

type FlowExport struct {
	Version       string         `json:"version"`
	ExportedAt    time.Time      `json:"exportedAt"`
	FlowName      string         `json:"flowName"`
	RecordedSteps []RecordedStep `json:"recordedSteps,omitempty"`
	Tasks         []Task         `json:"tasks"`
}
