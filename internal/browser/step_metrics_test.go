package browser

import (
	"context"
	"errors"
	"testing"

	"flowpilot/internal/models"
	"flowpilot/internal/monitoring"
)

func TestRunTaskRecordsPerActionStepMetrics(t *testing.T) {
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	mon := monitoring.New()
	r.SetMonitor(mon)

	task := models.Task{ID: "obs-1", Headless: true, Steps: []models.TaskStep{
		{Action: models.ActionNavigate, Value: "https://example.com"},
		{Action: models.ActionClick, Selector: "#a"},
		{Action: models.ActionClick, Selector: "#b"},
	}}
	if _, err := r.RunTask(context.Background(), task); err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	mock.runErr = errors.New("element missing")
	failing := models.Task{ID: "obs-2", Headless: true, Steps: []models.TaskStep{{Action: models.ActionClick, Selector: "#gone"}}}
	_, _ = r.RunTask(context.Background(), failing)

	byAction := map[string]monitoring.ActionStepStats{}
	for _, s := range mon.Steps().Stats() {
		byAction[s.Action] = s
	}
	if byAction["navigate"].Count != 1 || byAction["navigate"].Errors != 0 {
		t.Fatalf("navigate stats: %+v", byAction["navigate"])
	}
	if byAction["click"].Count < 3 || byAction["click"].Errors < 1 {
		t.Fatalf("click stats should include the failed step: %+v", byAction["click"])
	}
}
