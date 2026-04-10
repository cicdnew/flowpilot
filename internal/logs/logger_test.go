package logs

import (
	"errors"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestNewStepLogger(t *testing.T) {
	l := NewStepLogger("task-1")
	if l.taskID != "task-1" {
		t.Errorf("taskID: got %q, want %q", l.taskID, "task-1")
	}
	if len(l.Logs()) != 0 {
		t.Errorf("initial Logs: got %d entries, want 0", len(l.Logs()))
	}
}

func TestEndStepWithError(t *testing.T) {
	l := NewStepLogger("task-err")
	start := l.StartStep(0, models.ActionClick, "#btn", "", "")

	// Simulate some work time
	time.Sleep(10 * time.Millisecond)

	testErr := errors.New("element not found")
	l.EndStep(EndStepParams{
		StepIndex:  0,
		Action:     models.ActionClick,
		Selector:   "#btn",
		Value:      "",
		SnapshotID: "",
		Start:      start,
		Err:        testErr,
		Code:       models.ErrCodeSelectorNotFnd,
	})

	logs := l.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.TaskID != "task-err" {
		t.Errorf("TaskID: got %q, want %q", log.TaskID, "task-err")
	}
	if log.StepIndex != 0 {
		t.Errorf("StepIndex: got %d, want 0", log.StepIndex)
	}
	if log.Action != models.ActionClick {
		t.Errorf("Action: got %q, want %q", log.Action, models.ActionClick)
	}
	if log.Selector != "#btn" {
		t.Errorf("Selector: got %q, want %q", log.Selector, "#btn")
	}
	if log.ErrorMsg != "element not found" {
		t.Errorf("ErrorMsg: got %q, want %q", log.ErrorMsg, "element not found")
	}
	if log.ErrorCode != string(models.ErrCodeSelectorNotFnd) {
		t.Errorf("ErrorCode: got %q, want %q", log.ErrorCode, models.ErrCodeSelectorNotFnd)
	}
	if log.DurationMs <= 0 {
		t.Errorf("DurationMs: got %d, want > 0", log.DurationMs)
	}
	if log.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}
}

func TestEndStepWithoutError(t *testing.T) {
	l := NewStepLogger("task-ok")
	start := l.StartStep(0, models.ActionNavigate, "", "https://example.com", "snap-1")

	time.Sleep(5 * time.Millisecond)

	l.EndStep(EndStepParams{
		StepIndex:  0,
		Action:     models.ActionNavigate,
		Selector:   "",
		Value:      "https://example.com",
		SnapshotID: "snap-1",
		Start:      start,
		Err:        nil,
		Code:       "",
	})

	logs := l.Logs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if log.ErrorMsg != "" {
		t.Errorf("ErrorMsg should be empty, got %q", log.ErrorMsg)
	}
	if log.Value != "https://example.com" {
		t.Errorf("Value: got %q, want %q", log.Value, "https://example.com")
	}
	if log.SnapshotID != "snap-1" {
		t.Errorf("SnapshotID: got %q, want %q", log.SnapshotID, "snap-1")
	}
	if log.DurationMs <= 0 {
		t.Errorf("DurationMs should be positive, got %d", log.DurationMs)
	}
}

func TestLogsReturnedInOrder(t *testing.T) {
	l := NewStepLogger("task-order")

	for i := 0; i < 5; i++ {
		start := l.StartStep(i, models.ActionClick, "#btn", "", "")
		l.EndStep(EndStepParams{
			StepIndex:  i,
			Action:     models.ActionClick,
			Selector:   "#btn",
			Value:      "",
			SnapshotID: "",
			Start:      start,
			Err:        nil,
			Code:       "",
		})
	}

	logs := l.Logs()
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs, got %d", len(logs))
	}

	for i, log := range logs {
		if log.StepIndex != i {
			t.Errorf("log[%d].StepIndex: got %d, want %d", i, log.StepIndex, i)
		}
	}
}

func TestStartStepReturnsNonZeroTime(t *testing.T) {
	l := NewStepLogger("task-start")
	start := l.StartStep(0, models.ActionType, "#input", "hello", "")
	if start.IsZero() {
		t.Error("StartStep should return non-zero time")
	}
}

func TestMultipleStepsDifferentActions(t *testing.T) {
	l := NewStepLogger("task-multi")

	actions := []models.StepAction{
		models.ActionNavigate,
		models.ActionClick,
		models.ActionType,
		models.ActionScreenshot,
		models.ActionExtract,
	}

	for i, action := range actions {
		start := l.StartStep(i, action, "", "", "")
		l.EndStep(EndStepParams{
			StepIndex:  i,
			Action:     action,
			Selector:   "",
			Value:      "",
			SnapshotID: "",
			Start:      start,
			Err:        nil,
			Code:       "",
		})
	}

	logs := l.Logs()
	if len(logs) != len(actions) {
		t.Fatalf("expected %d logs, got %d", len(actions), len(logs))
	}

	for i, log := range logs {
		if log.Action != actions[i] {
			t.Errorf("log[%d].Action: got %q, want %q", i, log.Action, actions[i])
		}
	}
}
