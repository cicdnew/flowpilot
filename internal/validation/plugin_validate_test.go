package validation

import (
	"context"
	"errors"
	"testing"

	"flowpilot/internal/models"
	"flowpilot/internal/plugin"
)

func TestValidateTaskStepsPluginActions(t *testing.T) {
	steps := []models.TaskStep{{Action: "plugin_lookup", Selector: "#q", Value: "x"}}
	if err := ValidateTaskSteps(steps, false); !errors.Is(err, ErrInvalidStepAction) {
		t.Fatalf("unregistered plugin action should be rejected, got %v", err)
	}

	p := &plugin.Builtin{Spec: plugin.Manifest{ID: "lookup", Name: "Lookup", Version: "1",
		Actions: []plugin.ActionSpec{{Name: "plugin_lookup", SelectorRequired: true, ValueRequired: true}}}}
	if err := plugin.Default().Register(context.Background(), p, "test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = plugin.Default().Unload(context.Background(), "lookup") })

	if err := ValidateTaskSteps(steps, false); err != nil {
		t.Fatalf("registered plugin action rejected: %v", err)
	}
	if err := ValidateTaskSteps([]models.TaskStep{{Action: "plugin_lookup", Value: "x"}}, false); !errors.Is(err, ErrStepMissingSelector) {
		t.Fatalf("selector requirement not enforced: %v", err)
	}
	if err := ValidateTaskSteps([]models.TaskStep{{Action: "plugin_lookup", Selector: "#q"}}, false); !errors.Is(err, ErrStepMissingValue) {
		t.Fatalf("value requirement not enforced: %v", err)
	}
}
