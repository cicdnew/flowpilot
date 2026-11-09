package main

import (
	"context"
	"slices"
	"testing"

	"flowpilot/internal/plugin"
)

func TestGetSupportedStepActionsIncludesPluginActions(t *testing.T) {
	app := &App{plugins: plugin.NewRegistry()}
	before := app.GetSupportedStepActions()
	if !slices.Contains(before, "navigate") || slices.Contains(before, "text_transform") {
		t.Fatalf("unexpected base catalog: %v", before)
	}
	p := &plugin.Builtin{Spec: plugin.Manifest{ID: "tt", Name: "tt", Version: "1", Actions: []plugin.ActionSpec{{Name: "text_transform"}}}}
	if err := app.plugins.Register(context.Background(), p, ""); err != nil {
		t.Fatal(err)
	}
	after := app.GetSupportedStepActions()
	if len(after) != len(before)+1 || after[len(after)-1] != "text_transform" {
		t.Fatalf("plugin action missing from catalog: %v", after[len(before):])
	}
	if err := app.plugins.Unload(context.Background(), "tt"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(app.GetSupportedStepActions(), "text_transform") {
		t.Fatal("unloaded plugin action still in catalog")
	}
}
