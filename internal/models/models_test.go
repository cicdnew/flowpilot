package models

import (
	"errors"
	"testing"
	"time"
)

// --- ClassifyError Tests ---

func TestClassifyErrorNil(t *testing.T) {
	got := ClassifyError(nil)
	if got != "" {
		t.Errorf("ClassifyError(nil): got %q, want empty", got)
	}
}

func TestClassifyErrorTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"context deadline exceeded", "context deadline exceeded"},
		{"timeout keyword", "operation timeout"},
		{"mixed case contains timeout", "the request hit a timeout limit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeTimeout {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeTimeout)
			}
		})
	}
}

func TestClassifyErrorSelectorNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"selector keyword", "selector #btn not found"},
		{"not found keyword", "element not found on page"},
		{"waiting for selector", "waiting for selector #input timed out"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeSelectorNotFnd {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeSelectorNotFnd)
			}
		})
	}
}

func TestClassifyErrorNavFailed(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"navigate keyword", "failed to navigate to page"},
		{"navigation keyword", "navigation error occurred"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeNavFailed {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeNavFailed)
			}
		})
	}
}

func TestClassifyErrorProxyFailed(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"proxy keyword", "proxy connection refused"},
		{"proxy auth", "proxy auth failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeProxyFailed {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeProxyFailed)
			}
		})
	}
}

func TestClassifyErrorNetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"net err prefix", "net::ERR_CONNECTION_REFUSED"},
		{"network keyword", "network error on request"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeNetworkError {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeNetworkError)
			}
		})
	}
}

func TestClassifyErrorEvalBlocked(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"eval keyword", "eval is blocked by CSP"},
		{"allowEval keyword", "allowEval is set to false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeEvalBlocked {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeEvalBlocked)
			}
		})
	}
}

func TestClassifyErrorScreenshotFail(t *testing.T) {
	got := ClassifyError(errors.New("screenshot capture failed"))
	if got != ErrCodeScreenshotFail {
		t.Errorf("ClassifyError: got %q, want %q", got, ErrCodeScreenshotFail)
	}
}

func TestClassifyErrorEvalFailed(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"eval validation", "eval validation failed: script too large"},
		{"eval script", "eval script contains blocked pattern"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeEvalFailed {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeEvalFailed)
			}
		})
	}
}

func TestClassifyErrorAuthRequired(t *testing.T) {
	tests := []struct {
		name string
		err  string
	}{
		{"401 status", "server returned 401"},
		{"unauthorized", "unauthorized access denied"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tc.err))
			if got != ErrCodeAuthRequired {
				t.Errorf("ClassifyError(%q): got %q, want %q", tc.err, got, ErrCodeAuthRequired)
			}
		})
	}
}

func TestClassifyErrorUnknown(t *testing.T) {
	got := ClassifyError(errors.New("some totally random error"))
	if got != ErrCodeUnknown {
		t.Errorf("ClassifyError: got %q, want %q", got, ErrCodeUnknown)
	}
}

// --- containsAny Tests ---

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		substrs []string
		want    bool
	}{
		{"single match", "hello world", []string{"world"}, true},
		{"no match", "hello world", []string{"xyz"}, false},
		{"multiple substrs first matches", "error timeout", []string{"timeout", "other"}, true},
		{"multiple substrs second matches", "error timeout", []string{"other", "timeout"}, true},
		{"empty string", "", []string{"abc"}, false},
		{"empty substrs", "hello", []string{}, false},
		{"exact match", "abc", []string{"abc"}, true},
		{"substr longer than string", "ab", []string{"abcdef"}, false},
		{"empty substr in list", "hello", []string{""}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containsAny(tc.s, tc.substrs...)
			if got != tc.want {
				t.Errorf("containsAny(%q, %v): got %v, want %v", tc.s, tc.substrs, got, tc.want)
			}
		})
	}
}

// --- RecordedStep.ToTaskStep Tests ---

func TestRecordedStepToTaskStep(t *testing.T) {
	rs := RecordedStep{
		Index:    3,
		Action:   ActionClick,
		Selector: "#btn",
		Value:    "click-value",
		Timeout:  5000,
	}

	ts := rs.ToTaskStep()

	if ts.Action != ActionClick {
		t.Errorf("Action: got %q, want %q", ts.Action, ActionClick)
	}
	if ts.Selector != "#btn" {
		t.Errorf("Selector: got %q, want %q", ts.Selector, "#btn")
	}
	if ts.Value != "click-value" {
		t.Errorf("Value: got %q, want %q", ts.Value, "click-value")
	}
	if ts.Timeout != 5000 {
		t.Errorf("Timeout: got %d, want 5000", ts.Timeout)
	}
}

func TestRecordedStepToTaskStepEmpty(t *testing.T) {
	rs := RecordedStep{}
	ts := rs.ToTaskStep()

	if ts.Action != "" {
		t.Errorf("Action: got %q, want empty", ts.Action)
	}
	if ts.Selector != "" {
		t.Errorf("Selector: got %q, want empty", ts.Selector)
	}
	if ts.Value != "" {
		t.Errorf("Value: got %q, want empty", ts.Value)
	}
	if ts.Timeout != 0 {
		t.Errorf("Timeout: got %d, want 0", ts.Timeout)
	}
}

// --- FlowToTaskSteps Tests ---

func TestFlowToTaskSteps(t *testing.T) {
	flow := RecordedFlow{
		ID:   "flow-1",
		Name: "Test Flow",
		Steps: []RecordedStep{
			{Index: 0, Action: ActionNavigate, Value: "https://example.com", Timeout: 10000},
			{Index: 1, Action: ActionClick, Selector: "#btn"},
			{Index: 2, Action: ActionType, Selector: "#input", Value: "hello", Timeout: 3000},
		},
	}

	steps := FlowToTaskSteps(flow)

	if len(steps) != 3 {
		t.Fatalf("FlowToTaskSteps: got %d steps, want 3", len(steps))
	}

	if steps[0].Action != ActionNavigate {
		t.Errorf("steps[0].Action: got %q, want %q", steps[0].Action, ActionNavigate)
	}
	if steps[0].Value != "https://example.com" {
		t.Errorf("steps[0].Value: got %q, want %q", steps[0].Value, "https://example.com")
	}
	if steps[0].Timeout != 10000 {
		t.Errorf("steps[0].Timeout: got %d, want 10000", steps[0].Timeout)
	}

	if steps[1].Action != ActionClick {
		t.Errorf("steps[1].Action: got %q, want %q", steps[1].Action, ActionClick)
	}
	if steps[1].Selector != "#btn" {
		t.Errorf("steps[1].Selector: got %q, want %q", steps[1].Selector, "#btn")
	}

	if steps[2].Action != ActionType {
		t.Errorf("steps[2].Action: got %q, want %q", steps[2].Action, ActionType)
	}
	if steps[2].Value != "hello" {
		t.Errorf("steps[2].Value: got %q, want %q", steps[2].Value, "hello")
	}
}

func TestFlowToTaskStepsEmpty(t *testing.T) {
	flow := RecordedFlow{Steps: nil}
	steps := FlowToTaskSteps(flow)
	if len(steps) != 0 {
		t.Errorf("FlowToTaskSteps(empty): got %d steps, want 0", len(steps))
	}
}

func TestFlowToTaskStepsPreservesOrder(t *testing.T) {
	flow := RecordedFlow{
		Steps: []RecordedStep{
			{Index: 0, Action: ActionNavigate, Value: "https://a.com"},
			{Index: 1, Action: ActionClick, Selector: "#a"},
			{Index: 2, Action: ActionType, Selector: "#b", Value: "text"},
			{Index: 3, Action: ActionScreenshot},
			{Index: 4, Action: ActionScroll, Value: "500"},
		},
	}

	steps := FlowToTaskSteps(flow)
	if len(steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(steps))
	}

	actions := []StepAction{ActionNavigate, ActionClick, ActionType, ActionScreenshot, ActionScroll}
	for i, want := range actions {
		if steps[i].Action != want {
			t.Errorf("steps[%d].Action: got %q, want %q", i, steps[i].Action, want)
		}
	}
}

// --- BatchHeadless Tests ---

func TestBatchHeadlessDefault(t *testing.T) {
	input := AdvancedBatchInput{}
	if !input.BatchHeadless() {
		t.Error("BatchHeadless should default to true when Headless is nil")
	}
}

func TestBatchHeadlessTrue(t *testing.T) {
	val := true
	input := AdvancedBatchInput{Headless: &val}
	if !input.BatchHeadless() {
		t.Error("BatchHeadless should be true when set to true")
	}
}

func TestBatchHeadlessFalse(t *testing.T) {
	val := false
	input := AdvancedBatchInput{Headless: &val}
	if input.BatchHeadless() {
		t.Error("BatchHeadless should be false when set to false")
	}
}

// --- ErrorCode Constants Tests ---

func TestErrorCodeConstants(t *testing.T) {
	codes := []ErrorCode{
		ErrCodeTimeout,
		ErrCodeSelectorNotFnd,
		ErrCodeNavFailed,
		ErrCodeProxyFailed,
		ErrCodeAuthRequired,
		ErrCodeNetworkError,
		ErrCodeEvalBlocked,
		ErrCodeEvalFailed,
		ErrCodeScreenshotFail,
		ErrCodeUnknown,
	}

	seen := make(map[ErrorCode]bool)
	for _, code := range codes {
		if code == "" {
			t.Error("error code should not be empty")
		}
		if seen[code] {
			t.Errorf("duplicate error code: %s", code)
		}
		seen[code] = true
	}
}

// --- TaskStatus Constants Tests ---

func TestTaskStatusConstants(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusQueued,
		TaskStatusRunning,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
		TaskStatusRetrying,
	}

	seen := make(map[TaskStatus]bool)
	for _, s := range statuses {
		if s == "" {
			t.Error("task status should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate task status: %s", s)
		}
		seen[s] = true
	}
}

// --- StepAction Constants Tests ---

func TestStepActionConstants(t *testing.T) {
	actions := []StepAction{
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
	}

	seen := make(map[StepAction]bool)
	for _, a := range actions {
		if a == "" {
			t.Error("step action should not be empty")
		}
		if seen[a] {
			t.Errorf("duplicate step action: %s", a)
		}
		seen[a] = true
	}
}

// --- ProxyProtocol Constants Tests ---

func TestProxyProtocolConstants(t *testing.T) {
	protocols := []ProxyProtocol{ProxyHTTP, ProxyHTTPS, ProxySOCKS5}
	for _, p := range protocols {
		if p == "" {
			t.Error("proxy protocol should not be empty")
		}
	}
}

// --- SelectorType Constants Tests ---

func TestSelectorTypeConstants(t *testing.T) {
	types := []SelectorType{
		SelectorDataTestID,
		SelectorID,
		SelectorRole,
		SelectorCSS,
		SelectorXPath,
	}

	seen := make(map[SelectorType]bool)
	for _, st := range types {
		if st == "" {
			t.Error("selector type should not be empty")
		}
		if seen[st] {
			t.Errorf("duplicate selector type: %s", st)
		}
		seen[st] = true
	}
}

// --- RotationStrategy Constants Tests ---

func TestRotationStrategyConstants(t *testing.T) {
	strategies := []RotationStrategy{
		RotationRoundRobin,
		RotationRandom,
		RotationLeastUsed,
		RotationLowestLatency,
	}

	seen := make(map[RotationStrategy]bool)
	for _, s := range strategies {
		if s == "" {
			t.Error("rotation strategy should not be empty")
		}
		if seen[s] {
			t.Errorf("duplicate rotation strategy: %s", s)
		}
		seen[s] = true
	}
}

// --- MaxBatchSize Tests ---

func TestMaxBatchSize(t *testing.T) {
	if MaxBatchSize <= 0 {
		t.Errorf("MaxBatchSize should be positive, got %d", MaxBatchSize)
	}
	if MaxBatchSize != 10000 {
		t.Errorf("MaxBatchSize: got %d, want 10000", MaxBatchSize)
	}
}

// --- TaskPriority Constants Tests ---

func TestTaskPriorityConstants(t *testing.T) {
	if PriorityLow >= PriorityNormal {
		t.Errorf("PriorityLow (%d) should be less than PriorityNormal (%d)", PriorityLow, PriorityNormal)
	}
	if PriorityNormal >= PriorityHigh {
		t.Errorf("PriorityNormal (%d) should be less than PriorityHigh (%d)", PriorityNormal, PriorityHigh)
	}
}

// --- Struct field default/zero value tests ---

func TestTaskZeroValue(t *testing.T) {
	var task Task
	if task.ID != "" {
		t.Error("zero Task.ID should be empty")
	}
	if task.Status != "" {
		t.Error("zero Task.Status should be empty")
	}
	if task.Steps != nil {
		t.Error("zero Task.Steps should be nil")
	}
	if task.Result != nil {
		t.Error("zero Task.Result should be nil")
	}
	if task.StartedAt != nil {
		t.Error("zero Task.StartedAt should be nil")
	}
	if task.CompletedAt != nil {
		t.Error("zero Task.CompletedAt should be nil")
	}
}

func TestQueueMetricsZeroValue(t *testing.T) {
	var m QueueMetrics
	if m.Running != 0 || m.Queued != 0 || m.Pending != 0 {
		t.Error("zero QueueMetrics should have all zero counts")
	}
	if m.TotalSubmitted != 0 || m.TotalCompleted != 0 || m.TotalFailed != 0 {
		t.Error("zero QueueMetrics should have all zero totals")
	}
}

func TestBatchProgressZeroValue(t *testing.T) {
	var bp BatchProgress
	if bp.Total != 0 || bp.Pending != 0 || bp.Queued != 0 || bp.Running != 0 {
		t.Error("zero BatchProgress should have all zero fields")
	}
	if bp.Completed != 0 || bp.Failed != 0 || bp.Cancelled != 0 {
		t.Error("zero BatchProgress should have all zero fields")
	}
}

func TestDOMSnapshotZeroValue(t *testing.T) {
	var s DOMSnapshot
	if s.ID != "" || s.FlowID != "" || s.HTML != "" || s.URL != "" {
		t.Error("zero DOMSnapshot should have empty strings")
	}
	if s.StepIndex != 0 {
		t.Error("zero DOMSnapshot.StepIndex should be 0")
	}
	if !s.CapturedAt.IsZero() {
		t.Error("zero DOMSnapshot.CapturedAt should be zero time")
	}
}

func TestRecordedFlowZeroValue(t *testing.T) {
	var f RecordedFlow
	if f.ID != "" || f.Name != "" || f.Description != "" || f.OriginURL != "" {
		t.Error("zero RecordedFlow should have empty strings")
	}
	if f.Steps != nil {
		t.Error("zero RecordedFlow.Steps should be nil")
	}
}

// --- TaskLifecycleEvent Tests ---

func TestTaskLifecycleEventFields(t *testing.T) {
	now := time.Now()
	event := TaskLifecycleEvent{
		ID:        "evt-1",
		TaskID:    "task-1",
		BatchID:   "batch-1",
		FromState: TaskStatusPending,
		ToState:   TaskStatusRunning,
		Error:     "",
		Timestamp: now,
	}

	if event.ID != "evt-1" {
		t.Errorf("ID: got %q, want %q", event.ID, "evt-1")
	}
	if event.TaskID != "task-1" {
		t.Errorf("TaskID: got %q, want %q", event.TaskID, "task-1")
	}
	if event.BatchID != "batch-1" {
		t.Errorf("BatchID: got %q, want %q", event.BatchID, "batch-1")
	}
	if event.FromState != TaskStatusPending {
		t.Errorf("FromState: got %q, want %q", event.FromState, TaskStatusPending)
	}
	if event.ToState != TaskStatusRunning {
		t.Errorf("ToState: got %q, want %q", event.ToState, TaskStatusRunning)
	}
	if event.Error != "" {
		t.Errorf("Error: got %q, want empty", event.Error)
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp: got %v, want %v", event.Timestamp, now)
	}
}

// --- ValidateBatchTemplate Tests ---

func TestValidateBatchTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     bool
	}{
		{"plain text", "Task Name", true},
		{"empty", "", true},
		{"valid url", "{{url}}", true},
		{"valid domain", "{{domain}}", true},
		{"valid index", "{{index}}", true},
		{"valid name", "{{name}}", true},
		{"all valid vars", "{{url}} - {{domain}} - {{index}} - {{name}}", true},
		{"mixed text and vars", "Task {{index}}: {{domain}}", true},
		{"invalid var", "{{invalid}}", false},
		{"unclosed brace", "{{url", false},
		{"partial invalid", "{{url}} - {{bad}}", false},
		{"empty var name", "{{}}", false},
		{"nested braces", "{{{{url}}}}", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateBatchTemplate(tc.template)
			if got != tc.want {
				t.Errorf("ValidateBatchTemplate(%q): got %v, want %v", tc.template, got, tc.want)
			}
		})
	}
}

// --- Benchmark ---

func BenchmarkClassifyError(b *testing.B) {
	err := errors.New("context deadline exceeded during navigation")
	for i := 0; i < b.N; i++ {
		ClassifyError(err)
	}
}

func BenchmarkContainsAny(b *testing.B) {
	s := "this is a long string that might contain some timeout information somewhere"
	for i := 0; i < b.N; i++ {
		containsAny(s, "timeout", "selector", "navigate")
	}
}

func BenchmarkFlowToTaskSteps(b *testing.B) {
	flow := RecordedFlow{
		Steps: make([]RecordedStep, 20),
	}
	for i := range flow.Steps {
		flow.Steps[i] = RecordedStep{
			Index:    i,
			Action:   ActionClick,
			Selector: "#btn",
			Value:    "value",
			Timeout:  1000,
		}
	}
	for i := 0; i < b.N; i++ {
		FlowToTaskSteps(flow)
	}
}

// --- TruncatePayload Tests ---

func TestTruncatePayloadShort(t *testing.T) {
	input := "short payload"
	got := TruncatePayload(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestTruncatePayloadExactLimit(t *testing.T) {
	input := make([]byte, MaxWSPayloadSnippet)
	for i := range input {
		input[i] = 'x'
	}
	got := TruncatePayload(string(input))
	if got != string(input) {
		t.Errorf("expected no truncation for exact-limit payload")
	}
}

func TestTruncatePayloadOverLimit(t *testing.T) {
	input := make([]byte, MaxWSPayloadSnippet+100)
	for i := range input {
		input[i] = 'y'
	}
	got := TruncatePayload(string(input))
	if len(got) != MaxWSPayloadSnippet {
		t.Errorf("expected length %d, got %d", MaxWSPayloadSnippet, len(got))
	}
}

func TestTruncatePayloadEmpty(t *testing.T) {
	got := TruncatePayload("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
