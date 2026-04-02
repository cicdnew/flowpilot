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

// EndStep records completion of a step with duration and error details.
func (l *StepLogger) EndStep(stepIndex int, action models.StepAction, selector, value, snapshotID string, start time.Time, err error, code models.ErrorCode) {
	log := models.StepLog{
		TaskID:     l.taskID,
		StepIndex:  stepIndex,
		Action:     action,
		Selector:   selector,
		Value:      value,
		SnapshotID: snapshotID,
		ErrorCode:  string(code),
		DurationMs: time.Since(start).Milliseconds(),
		StartedAt:  start,
	}
	if err != nil {
		log.ErrorMsg = err.Error()
	}
	l.stepLogs = append(l.stepLogs, log)
}

// Logs returns all captured step logs.
func (l *StepLogger) Logs() []models.StepLog {
	return l.stepLogs
}
