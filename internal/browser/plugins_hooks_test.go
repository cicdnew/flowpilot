package browser

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"flowpilot/internal/models"
	"flowpilot/internal/plugin"
)

type hookRecorder struct {
	mu     sync.Mutex
	events []string
}

func (h *hookRecorder) plugin(id string, caps []string, fail plugin.HookPoint) *plugin.Builtin {
	return &plugin.Builtin{
		Spec: plugin.Manifest{ID: id, Name: id, Version: "1", Capabilities: caps,
			Hooks: []plugin.HookPoint{plugin.HookPreExecute, plugin.HookPostExecute, plugin.HookOnError}},
		OnHook: func(ctx context.Context, ev plugin.HookEvent) error {
			h.mu.Lock()
			entry := id + ":" + string(ev.Point) + ":" + ev.Context.Step.Action
			if ev.Error != "" {
				entry += ":err"
			}
			if ev.Context.Variables != nil {
				entry += ":vars=" + ev.Context.Variables["user"]
			}
			if ctx.Err() != nil {
				entry += ":ctx-dead"
			}
			h.events = append(h.events, entry)
			h.mu.Unlock()
			if ev.Point == fail {
				return errors.New("hook failed")
			}
			return nil
		},
	}
}

func (h *hookRecorder) take() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := strings.Join(h.events, " | ")
	h.events = nil
	return out
}

func TestRunnerHooksWrapSuccessfulStep(t *testing.T) {
	rec := &hookRecorder{}
	r, mock := newPluginRunner(t, rec.plugin("audit", []string{plugin.CapVariablesRead}, ""), rec.plugin("metrics", nil, ""))
	result := &models.TaskResult{TaskID: "t", ExtractedData: map[string]string{"var_user": "ada"}}

	if err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: models.ActionNavigate, Value: "https://example.com"}, result); err != nil {
		t.Fatalf("step: %v", err)
	}
	want := "audit:pre_execute:navigate:vars=ada | metrics:pre_execute:navigate | audit:post_execute:navigate:vars=ada | metrics:post_execute:navigate"
	if got := rec.take(); got != want {
		t.Fatalf("hook sequence\n got: %s\nwant: %s", got, want)
	}
	if len(mock.calls) != 1 {
		t.Fatalf("step should run once: %v", mock.calls)
	}
}

func TestRunnerOnErrorHooksRunAfterFailure(t *testing.T) {
	rec := &hookRecorder{}
	r, mock := newPluginRunner(t, rec.plugin("audit", nil, ""))
	mock.runErr = errors.New("element not found")
	result := &models.TaskResult{TaskID: "t", ExtractedData: map[string]string{}}

	err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: models.ActionClick, Selector: "#go"}, result)
	if err == nil || !strings.Contains(err.Error(), "element not found") {
		t.Fatalf("step error must be preserved, got %v", err)
	}
	if got := rec.take(); got != "audit:pre_execute:click | audit:on_error:click:err" {
		t.Fatalf("hook sequence: %s", got)
	}
}

func TestRunnerPostHookFailureIsLoggedNotFatal(t *testing.T) {
	rec := &hookRecorder{}
	r, _ := newPluginRunner(t, rec.plugin("flaky", nil, plugin.HookPostExecute))
	result := &models.TaskResult{TaskID: "t", ExtractedData: map[string]string{}}

	if err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: models.ActionReload}, result); err != nil {
		t.Fatalf("post hook failure must not fail the step: %v", err)
	}
	last := result.Logs[len(result.Logs)-1]
	if last.Level != "warn" || !strings.Contains(last.Message, "flaky post_execute hook: hook failed") {
		t.Fatalf("hook failure not logged: %+v", last)
	}
}

func TestRunnerPreHookVetoSkipsStepAndLaterHooks(t *testing.T) {
	rec := &hookRecorder{}
	r, mock := newPluginRunner(t, rec.plugin("guard", nil, plugin.HookPreExecute), rec.plugin("later", nil, ""))
	err := r.runPluginStep(context.Background(), 0, models.TaskStep{Action: models.ActionReload}, &models.TaskResult{ExtractedData: map[string]string{}})
	if !errors.Is(err, plugin.ErrVetoed) {
		t.Fatalf("expected veto, got %v", err)
	}
	if got := rec.take(); got != "guard:pre_execute:reload" {
		t.Fatalf("veto must stop the chain and skip post/error hooks: %s", got)
	}
	if len(mock.calls) != 0 {
		t.Fatalf("vetoed step reached the browser: %v", mock.calls)
	}
}

func TestRunnerOnErrorHooksRunWhenStepContextExpired(t *testing.T) {
	rec := &hookRecorder{}
	r, mock := newPluginRunner(t, rec.plugin("audit", nil, ""))
	ctx, cancel := context.WithCancel(context.Background())
	mock.runErr = context.Canceled
	cancel()
	_ = r.runPluginStep(ctx, 0, models.TaskStep{Action: models.ActionReload}, &models.TaskResult{ExtractedData: map[string]string{}})
	if got := rec.take(); !strings.HasSuffix(got, "audit:on_error:reload:err") {
		t.Fatalf("on_error hook should run with a live context after step expiry: %s", got)
	}
}
