package browser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"flowpilot/internal/models"
	"flowpilot/internal/plugin"
)

func newPluginRunner(t *testing.T, plugins ...plugin.Plugin) (*Runner, *mockExecutor) {
	t.Helper()
	reg := plugin.NewRegistry()
	for _, p := range plugins {
		if err := reg.Register(context.Background(), p, "test"); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	t.Cleanup(func() { _ = reg.Close(context.Background()) })
	mock := &mockExecutor{}
	r := newMockRunner(t, mock)
	r.SetPluginRegistry(reg)
	return r, mock
}

func newResult() *models.TaskResult {
	return &models.TaskResult{TaskID: "task-1", ExtractedData: map[string]string{"var_name": "Ada", "title": "t"}}
}

func TestRunnerDispatchesPluginAction(t *testing.T) {
	var got plugin.StepContext
	p := &plugin.Builtin{
		Spec: plugin.Manifest{ID: "greeter", Name: "Greeter", Version: "1", Actions: []plugin.ActionSpec{{Name: "greet"}},
			Capabilities: []string{plugin.CapVariablesRead, plugin.CapVariablesWrite, plugin.CapExtractedWrite}},
		OnExecute: func(_ context.Context, sc plugin.StepContext) (plugin.StepResult, error) {
			got = sc
			return plugin.StepResult{
				Variables: map[string]string{"greeting": sc.Step.Value + " " + sc.Variables["name"]},
				Extracted: map[string]string{"greeted": "yes"},
				Logs:      []string{"said hello"},
			}, nil
		},
	}
	r, mock := newPluginRunner(t, p)
	result := newResult()

	step := models.TaskStep{Action: "greet", Value: "Hello", Selector: "#x"}
	if err := r.runPluginStep(context.Background(), 3, step, result); err != nil {
		t.Fatalf("runPluginStep: %v", err)
	}
	if got.TaskID != "task-1" || got.StepIndex != 3 || got.Step.Selector != "#x" || got.Step.Action != "greet" {
		t.Fatalf("plugin got wrong context: %+v", got)
	}
	if _, leaked := got.Variables["title"]; leaked {
		t.Fatal("non-variable extracted data leaked into plugin variables")
	}
	if result.ExtractedData["var_greeting"] != "Hello Ada" || result.ExtractedData["greeted"] != "yes" {
		t.Fatalf("plugin output not merged: %v", result.ExtractedData)
	}
	if len(result.Logs) == 0 || !strings.Contains(result.Logs[len(result.Logs)-1].Message, "[plugin greeter] said hello") {
		t.Fatalf("plugin log missing: %+v", result.Logs)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("plugin action must not hit the browser executor: %v", mock.calls)
	}
}

func TestRunnerPluginActionErrorsAndReservedKeys(t *testing.T) {
	p := &plugin.Builtin{
		Spec: plugin.Manifest{ID: "bad", Name: "Bad", Version: "1", Actions: []plugin.ActionSpec{{Name: "fail_action"}, {Name: "reserved_action"}},
			Capabilities: []string{plugin.CapExtractedWrite}},
		OnExecute: func(_ context.Context, sc plugin.StepContext) (plugin.StepResult, error) {
			if sc.Step.Action == "fail_action" {
				return plugin.StepResult{}, errors.New("upstream down")
			}
			return plugin.StepResult{Extracted: map[string]string{"var_name": "hijack"}}, nil
		},
	}
	r, _ := newPluginRunner(t, p)
	result := newResult()

	if err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: "fail_action"}, result); err == nil || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("expected plugin error, got %v", err)
	}
	if err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: "reserved_action"}, result); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-prefix error, got %v", err)
	}
	if result.ExtractedData["var_name"] != "Ada" {
		t.Fatalf("plugin overwrote a variable through extracted data: %v", result.ExtractedData)
	}
}

func TestRunnerUnknownActionStillFails(t *testing.T) {
	r, _ := newPluginRunner(t)
	err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: "not_registered"}, newResult())
	if err == nil || !strings.Contains(err.Error(), "unknown action: not_registered") {
		t.Fatalf("expected unknown action error, got %v", err)
	}
}

func TestRunnerBuiltinActionsUnaffectedByPlugins(t *testing.T) {
	r, mock := newPluginRunner(t, &plugin.Builtin{Spec: plugin.Manifest{ID: "x", Name: "x", Version: "1", Actions: []plugin.ActionSpec{{Name: "x_action"}}}})
	if err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: models.ActionNavigate, Value: "https://example.com"}, newResult()); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("builtin navigate should run through executor once, calls=%v", mock.calls)
	}
}
