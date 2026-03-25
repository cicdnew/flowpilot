package recorder

import (
	"testing"

	"flowpilot/internal/models"
)

func TestParseBindingPayloadClick(t *testing.T) {
	action, selector, value, err := parseBindingPayload(`{"action":"click","selector":"#btn","value":""}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != models.ActionClick {
		t.Errorf("action: got %q, want %q", action, models.ActionClick)
	}
	if selector != "#btn" {
		t.Errorf("selector: got %q, want %q", selector, "#btn")
	}
	if value != "" {
		t.Errorf("value: got %q, want empty", value)
	}
}

func TestParseBindingPayloadType(t *testing.T) {
	action, selector, value, err := parseBindingPayload(`{"action":"type","selector":"#input","value":"hello world"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != models.ActionType {
		t.Errorf("action: got %q, want %q", action, models.ActionType)
	}
	if selector != "#input" {
		t.Errorf("selector: got %q, want %q", selector, "#input")
	}
	if value != "hello world" {
		t.Errorf("value: got %q, want %q", value, "hello world")
	}
}

func TestParseBindingPayloadSelect(t *testing.T) {
	action, _, value, err := parseBindingPayload(`{"action":"select","selector":"#dropdown","value":"option2"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != models.ActionSelect {
		t.Errorf("action: got %q, want %q", action, models.ActionSelect)
	}
	if value != "option2" {
		t.Errorf("value: got %q, want %q", value, "option2")
	}
}

func TestParseBindingPayloadNavigate(t *testing.T) {
	action, _, value, err := parseBindingPayload(`{"action":"navigate","selector":"","value":"https://example.com"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != models.ActionNavigate {
		t.Errorf("action: got %q, want %q", action, models.ActionNavigate)
	}
	if value != "https://example.com" {
		t.Errorf("value: got %q, want %q", value, "https://example.com")
	}
}

func TestParseBindingPayloadInvalidJSON(t *testing.T) {
	_, _, _, err := parseBindingPayload(`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseBindingPayloadUnknownAction(t *testing.T) {
	_, _, _, err := parseBindingPayload(`{"action":"hover","selector":"#x","value":""}`)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestMapJSActionAllValid(t *testing.T) {
	tests := []struct {
		js   string
		want models.StepAction
	}{
		{"click", models.ActionClick},
		{"type", models.ActionType},
		{"select", models.ActionSelect},
		{"navigate", models.ActionNavigate},
	}
	for _, tc := range tests {
		got, ok := mapJSAction(tc.js)
		if !ok {
			t.Errorf("mapJSAction(%q): expected ok=true", tc.js)
		}
		if got != tc.want {
			t.Errorf("mapJSAction(%q): got %q, want %q", tc.js, got, tc.want)
		}
	}
}

func TestMapJSActionInvalid(t *testing.T) {
	_, ok := mapJSAction("unknown")
	if ok {
		t.Error("mapJSAction(unknown): expected ok=false")
	}
}

func TestCaptureScriptNotEmpty(t *testing.T) {
	if len(captureScript) == 0 {
		t.Fatal("captureScript should not be empty")
	}
}

func TestBindingNameConstant(t *testing.T) {
	if bindingName != "__recordStep" {
		t.Errorf("bindingName: got %q, want %q", bindingName, "__recordStep")
	}
}

func TestHandleBindingCallEmitsStep(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-binding",
	}

	r.handleBindingCall(`{"action":"click","selector":"#submit","value":""}`)
	r.handleBindingCall(`{"action":"type","selector":"#email","value":"test@example.com"}`)

	if len(captured) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(captured))
	}
	if captured[0].Action != models.ActionClick {
		t.Errorf("step[0].Action: got %q, want %q", captured[0].Action, models.ActionClick)
	}
	if captured[0].Selector != "#submit" {
		t.Errorf("step[0].Selector: got %q, want %q", captured[0].Selector, "#submit")
	}
	if captured[1].Action != models.ActionType {
		t.Errorf("step[1].Action: got %q, want %q", captured[1].Action, models.ActionType)
	}
	if captured[1].Value != "test@example.com" {
		t.Errorf("step[1].Value: got %q", captured[1].Value)
	}
	if captured[0].Index != 0 || captured[1].Index != 1 {
		t.Errorf("step indices: got %d, %d; want 0, 1", captured[0].Index, captured[1].Index)
	}
}

func TestHandleBindingCallInvalidPayload(t *testing.T) {
	var captured []models.RecordedStep
	r := &Recorder{
		handler: func(step models.RecordedStep) {
			captured = append(captured, step)
		},
		flowID: "flow-invalid",
	}

	r.handleBindingCall(`invalid json`)
	r.handleBindingCall(`{"action":"unknown","selector":"#x","value":""}`)

	if len(captured) != 0 {
		t.Errorf("expected 0 steps for invalid payloads, got %d", len(captured))
	}
}
