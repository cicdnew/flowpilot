package copilot

import (
	"testing"
)

func TestModelCapability(t *testing.T) {
	tests := []struct {
		name       string
		caps       ModelCapability
		check      ModelCapability
		shouldHave bool
	}{
		{"has tool calling", CapabilityToolCalling, CapabilityToolCalling, true},
		{"has long context", CapabilityLongContext, CapabilityLongContext, true},
		{"has multiple", CapabilityToolCalling | CapabilityCoding, CapabilityToolCalling, true},
		{"has multiple 2", CapabilityToolCalling | CapabilityCoding, CapabilityCoding, true},
		{"does not have", CapabilityToolCalling, CapabilityVision, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has := tt.caps&tt.check != 0
			if has != tt.shouldHave {
				t.Errorf("capability check = %v; want %v", has, tt.shouldHave)
			}
		})
	}
}

func TestModelStructure(t *testing.T) {
	m := Model{
		ID:          "gpt-4o",
		Name:        "GPT-4o",
		Provider:    "openai",
		Capabilities: CapabilityToolCalling | CapabilityVision,
		MaxContext:  128000,
		Description: "OpenAI's latest model",
	}

	if m.ID != "gpt-4o" {
		t.Errorf("ID = %q; want %q", m.ID, "gpt-4o")
	}
	if m.MaxContext != 128000 {
		t.Errorf("MaxContext = %d; want 128000", m.MaxContext)
	}
}

func TestStreamChunkDone(t *testing.T) {
	chunk := StreamChunk{Done: true}
	if !chunk.Done {
		t.Error("Done should be true")
	}
	if chunk.Content != "" {
		t.Error("Content should be empty for done chunk")
	}
	if chunk.Error != nil {
		t.Error("Error should be nil for done chunk")
	}
}

func TestStreamChunkContent(t *testing.T) {
	chunk := StreamChunk{Content: "Hello"}
	if chunk.Content != "Hello" {
		t.Errorf("Content = %q; want %q", chunk.Content, "Hello")
	}
	if chunk.Done {
		t.Error("Done should be false for content chunk")
	}
}

func TestStreamChunkError(t *testing.T) {
	testErr := assertError("test error")
	chunk := StreamChunk{Error: testErr}

	if chunk.Error == nil {
		t.Error("Error should not be nil")
	}
	if chunk.Error.Error() != "test error" {
		t.Errorf("Error message = %q; want %q", chunk.Error.Error(), "test error")
	}
}

func TestStreamChunkToolCall(t *testing.T) {
	chunk := StreamChunk{
		ToolCall: &ToolCall{
			Name:      "create_batch",
			Arguments: map[string]any{"flow_id": "123"},
		},
	}

	if chunk.ToolCall == nil {
		t.Fatal("ToolCall should not be nil")
	}
	if chunk.ToolCall.Name != "create_batch" {
		t.Errorf("ToolCall.Name = %q; want %q", chunk.ToolCall.Name, "create_batch")
	}
}

// Helper for error assertions
type assertError string

func (e assertError) Error() string {
	return string(e)
}
