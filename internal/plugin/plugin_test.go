package plugin

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"flowpilot/internal/models"
	sdk "flowpilot/sdk/go/plugin"
)

func testManifest(id string, actions ...models.StepAction) Manifest {
	m := Manifest{ID: id, Name: id, Version: "1.0.0"}
	for _, a := range actions {
		m.Actions = append(m.Actions, ActionSpec{Name: a})
	}
	return m
}

func TestManifestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		ok     bool
	}{
		{"valid builtin", func(*Manifest) {}, true},
		{"valid process", func(m *Manifest) { m.Type, m.Entrypoint = TypeProcess, "./bin" }, true},
		{"bad id", func(m *Manifest) { m.ID = "Bad ID" }, false},
		{"missing version", func(m *Manifest) { m.Version = "" }, false},
		{"unknown type", func(m *Manifest) { m.Type = "wasm" }, false},
		{"process without entrypoint", func(m *Manifest) { m.Type = TypeProcess }, false},
		{"bad action name", func(m *Manifest) { m.Actions[0].Name = "Bad-Action" }, false},
		{"duplicate action", func(m *Manifest) { m.Actions = append(m.Actions, m.Actions[0]) }, false},
		{"unknown hook", func(m *Manifest) { m.Hooks = []HookPoint{"before_everything"} }, false},
		{"unknown capability", func(m *Manifest) { m.Capabilities = []string{"filesystem.write"} }, false},
		{"nothing declared", func(m *Manifest) { m.Actions = nil }, false},
		{"negative timeout", func(m *Manifest) { m.TimeoutMs = -1 }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testManifest("demo", "demo_action")
			m.Type = TypeBuiltin
			tc.mutate(&m)
			err := m.Validate()
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("expected ErrInvalidManifest, got %v", err)
			}
		})
	}
}

func TestRegistryRegisterLookupListUnload(t *testing.T) {
	r := NewRegistry()
	var initConfig map[string]any
	shutdowns := 0
	p := &Builtin{
		Spec:       Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", Actions: []ActionSpec{{Name: "demo_action", ValueRequired: true}}, Config: map[string]any{"k": "v"}},
		OnInit:     func(_ context.Context, cfg map[string]any) error { initConfig = cfg; return nil },
		OnShutdown: func(context.Context) error { shutdowns++; return nil },
	}
	if err := r.Register(context.Background(), p, "test"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if initConfig["k"] != "v" {
		t.Fatalf("Init config not passed: %v", initConfig)
	}
	spec, ok := r.Action("demo_action")
	if !ok || !spec.ValueRequired {
		t.Fatalf("Action lookup: %+v %v", spec, ok)
	}
	if list := r.List(); len(list) != 1 || list[0].ID != "demo" || list[0].Source != "test" {
		t.Fatalf("List: %+v", list)
	}
	if err := r.Unload(context.Background(), "demo"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if shutdowns != 1 {
		t.Fatalf("shutdowns: got %d, want 1", shutdowns)
	}
	if _, ok := r.Action("demo_action"); ok {
		t.Fatal("action still registered after unload")
	}
	if err := r.Unload(context.Background(), "demo"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Unload: %v", err)
	}
}

func TestRegistryRejectsConflicts(t *testing.T) {
	r := NewRegistry()
	ctx := context.Background()
	if err := r.Register(ctx, &Builtin{Spec: testManifest("a", "shared_action")}, ""); err != nil {
		t.Fatal(err)
	}
	cases := map[string]Plugin{
		"duplicate id":     &Builtin{Spec: testManifest("a", "other_action")},
		"duplicate action": &Builtin{Spec: testManifest("b", "shared_action")},
		"builtin action":   &Builtin{Spec: testManifest("c", models.ActionClick)},
	}
	for name, p := range cases {
		if err := r.Register(ctx, p, ""); !errors.Is(err, ErrConflict) {
			t.Errorf("%s: expected ErrConflict, got %v", name, err)
		}
	}
	if len(r.List()) != 1 {
		t.Fatalf("conflicting plugins were registered: %+v", r.List())
	}
}

func TestRegistryInitFailureIsNotRegistered(t *testing.T) {
	r := NewRegistry()
	shutdown := false
	p := &Builtin{
		Spec:       testManifest("broken", "broken_action"),
		OnInit:     func(context.Context, map[string]any) error { return errors.New("boom") },
		OnShutdown: func(context.Context) error { shutdown = true; return nil },
	}
	if err := r.Register(context.Background(), p, ""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected init error, got %v", err)
	}
	if _, ok := r.Action("broken_action"); ok || len(r.List()) != 0 {
		t.Fatal("failed plugin was registered")
	}
	if !shutdown {
		t.Fatal("failed plugin was not shut down")
	}
}

func TestRegistryExecuteEnforcesCapabilities(t *testing.T) {
	r := NewRegistry()
	var seenVars map[string]string
	result := StepResult{Variables: map[string]string{"out": "1"}}
	p := &Builtin{Spec: testManifest("caps", "caps_action"), OnExecute: func(_ context.Context, sc StepContext) (StepResult, error) {
		seenVars = sc.Variables
		return result, nil
	}}
	if err := r.Register(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}
	sc := StepContext{Step: sdk.Step{Action: "caps_action"}}
	vars := map[string]string{"secret": "x"}
	if _, _, err := r.Execute(context.Background(), sc, vars); err == nil || !strings.Contains(err.Error(), CapVariablesWrite) {
		t.Fatalf("expected missing write capability error, got %v", err)
	}
	if seenVars != nil {
		t.Fatalf("variables leaked without %s: %v", CapVariablesRead, seenVars)
	}

	r2 := NewRegistry()
	p.Spec.Capabilities = []string{CapVariablesRead, CapVariablesWrite}
	if err := r2.Register(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}
	res, _, err := r2.Execute(context.Background(), sc, vars)
	if err != nil || res.Variables["out"] != "1" || seenVars["secret"] != "x" {
		t.Fatalf("capable plugin: res=%+v vars=%v err=%v", res, seenVars, err)
	}
}

func TestRegistryExecuteAppliesTimeout(t *testing.T) {
	r := NewRegistry()
	m := testManifest("slow", "slow_action")
	m.TimeoutMs = 50
	p := &Builtin{Spec: m, OnExecute: func(ctx context.Context, _ StepContext) (StepResult, error) {
		<-ctx.Done()
		return StepResult{}, ctx.Err()
	}}
	if err := r.Register(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, _, err := r.Execute(context.Background(), StepContext{Step: sdk.Step{Action: "slow_action"}}, nil)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("expected prompt deadline error, got %v after %s", err, time.Since(start))
	}
}

func TestRegistryHooksOrderVetoAndErrors(t *testing.T) {
	r := NewRegistry()
	var mu sync.Mutex
	var calls []string
	hookPlugin := func(id string, veto bool) *Builtin {
		return &Builtin{Spec: Manifest{ID: id, Name: id, Version: "1", Hooks: []HookPoint{HookPreExecute, HookOnError}}, OnHook: func(_ context.Context, ev HookEvent) error {
			mu.Lock()
			calls = append(calls, id+":"+string(ev.Point)+":"+ev.Error)
			mu.Unlock()
			if veto {
				return errors.New("blocked")
			}
			return nil
		}}
	}
	for _, p := range []*Builtin{hookPlugin("first", false), hookPlugin("second", true), hookPlugin("third", false)} {
		if err := r.Register(context.Background(), p, ""); err != nil {
			t.Fatal(err)
		}
	}
	err := r.RunHooks(context.Background(), HookPreExecute, StepContext{}, nil, nil)
	if !errors.Is(err, ErrVetoed) || !strings.Contains(err.Error(), "second") {
		t.Fatalf("expected veto from second, got %v", err)
	}
	if strings.Join(calls, ",") != "first:pre_execute:,second:pre_execute:" {
		t.Fatalf("hook order / short-circuit wrong: %v", calls)
	}
	calls = nil
	err = r.RunHooks(context.Background(), HookOnError, StepContext{}, nil, errors.New("step failed"))
	if err == nil || errors.Is(err, ErrVetoed) {
		t.Fatalf("on_error hook errors should be reported but not veto: %v", err)
	}
	if len(calls) != 3 || calls[2] != "third:on_error:step failed" {
		t.Fatalf("on_error must reach every hook in order: %v", calls)
	}
}

func TestRegistryCloseShutsDownInReverseOrder(t *testing.T) {
	r := NewRegistry()
	var order []string
	for _, id := range []string{"one", "two"} {
		p := &Builtin{Spec: testManifest(id, models.StepAction(id+"_action")), OnShutdown: func(context.Context) error { order = append(order, id); return nil }}
		if err := r.Register(context.Background(), p, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "two,one" || len(r.List()) != 0 {
		t.Fatalf("shutdown order %v, remaining %d", order, len(r.List()))
	}
}
