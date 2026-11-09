package plugin

import "encoding/json"

const ProtocolVersion = 1

type HookPoint string

const (
	HookPreExecute  HookPoint = "pre_execute"
	HookPostExecute HookPoint = "post_execute"
	HookOnError     HookPoint = "on_error"
)

const (
	MethodInit     = "init"
	MethodExecute  = "execute"
	MethodHook     = "hook"
	MethodShutdown = "shutdown"
)

type Step struct {
	Action    string `json:"action"`
	Selector  string `json:"selector,omitempty"`
	Value     string `json:"value,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	Condition string `json:"condition,omitempty"`
	Label     string `json:"label,omitempty"`
	JumpTo    string `json:"jumpTo,omitempty"`
	VarName   string `json:"varName,omitempty"`
	Operator  string `json:"operator,omitempty"`
	MaxLoops  int    `json:"maxLoops,omitempty"`
	Target    string `json:"target,omitempty"`
	Source    string `json:"source,omitempty"`
	Keys      string `json:"keys,omitempty"`
	Duration  int    `json:"duration,omitempty"`
}

type StepContext struct {
	TaskID    string            `json:"taskId"`
	StepIndex int               `json:"stepIndex"`
	Step      Step              `json:"step"`
	Variables map[string]string `json:"variables,omitempty"`
}

type StepResult struct {
	Variables map[string]string `json:"variables,omitempty"`
	Extracted map[string]string `json:"extracted,omitempty"`
	Logs      []string          `json:"logs,omitempty"`
}

type HookEvent struct {
	Point   HookPoint   `json:"point"`
	Context StepContext `json:"context"`
	Error   string      `json:"error,omitempty"`
}

type InitParams struct {
	ProtocolVersion int            `json:"protocolVersion"`
	Config          map[string]any `json:"config,omitempty"`
}

type Request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}
