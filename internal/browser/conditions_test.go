package browser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"flowpilot/internal/models"
)

func TestEvaluateTextConditionOperators(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		text      string
		vars      map[string]string
		want      bool
		wantErr   string
	}{
		{name: "default contains", condition: "needle", text: "haystack needle haystack", want: true},
		{name: "contains", condition: "contains:needle", text: "haystack needle haystack", want: true},
		{name: "not contains", condition: "not_contains:needle", text: "haystack", want: true},
		{name: "equals", condition: "equals:exact", text: "exact", want: true},
		{name: "not equals", condition: "not_equals:other", text: "exact", want: true},
		{name: "starts with", condition: "starts_with:pre", text: "prefix-value", want: true},
		{name: "ends with", condition: "ends_with:value", text: "prefix-value", want: true},
		{name: "matches", condition: "matches:^prefix-[a-z]+$", text: "prefix-value", want: true},
		{name: "variable substitution", condition: "equals:{{target}}", text: "exact", vars: map[string]string{"target": "exact"}, want: true},
		{name: "invalid regex", condition: "matches:[", text: "anything", wantErr: "invalid regex in condition"},
		{name: "unknown operator", condition: "unknown:value", text: "value", wantErr: "unknown condition operator"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := evaluateTextCondition(tc.condition, tc.text, tc.vars)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q should contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildLabelIndex(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionNavigate, Label: "start"},
		{Action: models.ActionClick},
		{Action: models.ActionType, Label: "middle"},
		{Action: models.ActionWait, Label: "end"},
	}

	got := buildLabelIndex(steps)
	want := map[string]int{"start": 0, "middle": 2, "end": 3}

	if len(got) != len(want) {
		t.Fatalf("label count: got %d, want %d", len(got), len(want))
	}
	for label, idx := range want {
		if got[label] != idx {
			t.Fatalf("label %q index: got %d, want %d", label, got[label], idx)
		}
	}
}

func TestFindEndLoop(t *testing.T) {
	steps := []models.TaskStep{
		{Action: models.ActionLoop},
		{Action: models.ActionClick},
		{Action: models.ActionLoop},
		{Action: models.ActionType},
		{Action: models.ActionEndLoop},
		{Action: models.ActionEndLoop},
	}

	if got := findEndLoop(steps, 0); got != 5 {
		t.Fatalf("outer loop end: got %d, want 5", got)
	}
	if got := findEndLoop(steps, 2); got != 4 {
		t.Fatalf("inner loop end: got %d, want 4", got)
	}
	if got := findEndLoop([]models.TaskStep{{Action: models.ActionLoop}, {Action: models.ActionClick}}, 0); got != -1 {
		t.Fatalf("missing end loop: got %d, want -1", got)
	}
}

func TestEvaluateConditionUnknownAction(t *testing.T) {
	r := &Runner{}
	_, err := r.evaluateCondition(context.Background(), models.TaskStep{Action: "unknown_action"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown condition action")
	}
	if !strings.Contains(err.Error(), "unknown condition action") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluateConditionIfTextExecutorErrorReturnsError(t *testing.T) {
	r := newMockRunner(t, &mockExecutor{runErr: errors.New("text failed")})
	_, err := r.evaluateCondition(context.Background(), models.TaskStep{Action: models.ActionIfText, Selector: "#msg", Condition: "contains:ok"}, nil)
	if err == nil {
		t.Fatal("expected error when text lookup fails")
	}
	if !strings.Contains(err.Error(), "if_text get text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluateConditionIfURLError(t *testing.T) {
	r := newMockRunner(t, &mockExecutor{runErr: errors.New("location failed")})
	_, err := r.evaluateCondition(context.Background(), models.TaskStep{Action: models.ActionIfURL, Condition: "contains:example"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "if_url get location") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluateConditionIfElementError(t *testing.T) {
	r := newMockRunner(t, &mockExecutor{runErr: errors.New("nodes failed")})
	_, err := r.evaluateCondition(context.Background(), models.TaskStep{Action: models.ActionIfElement, Selector: "#msg", Condition: "exists"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "if_element check") {
		t.Fatalf("unexpected error: %v", err)
	}
}
