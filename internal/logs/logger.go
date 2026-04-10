package logs

import (
	"time"

	"flowpilot/internal/models"
)

// StepLogger accumulates step logs for a task execution.
type StepLogger struct {
	taskID   string
	stepLogs []models.StepLog
}

// NewStepLogger creates a step logger for a task.
func NewStepLogger(taskID string) *StepLogger {
	return &StepLogger{taskID: taskID, stepLogs: []models.StepLog{}}
}

// StartStep records the start time for a step.
func (l *StepLogger) StartStep(stepIndex int, action models.StepAction, selector, value, snapshotID string) time.Time {
	return time.Now()
}

// EndStepParams holds parameters for EndStep to reduce parameter count (S8209)
type EndStepParams struct {
	StepIndex  int
	Action     models.StepAction
	Selector   string
	Value      string
	SnapshotID string
	Start      time.Time
	Err        error
	Code       models.ErrorCode
}

// EndStep records completion of a step with duration and error details.
func (l *StepLogger) EndStep(p EndStepParams) {
	log := models.StepLog{
		TaskID:     l.taskID,
		StepIndex:  p.StepIndex,
		Action:     p.Action,
		Selector:   p.Selector,
		Value:      p.Value,
		SnapshotID: p.SnapshotID,
		ErrorCode:  string(p.Code),
		DurationMs: time.Since(p.Start).Milliseconds(),
		StartedAt:  p.Start,
	}
	if p.Err != nil {
		log.ErrorMsg = p.Err.Error()
	}
	l.stepLogs = append(l.stepLogs, log)
}

// Logs returns all captured step logs.
func (l *StepLogger) Logs() []models.StepLog {
	return l.stepLogs
}
