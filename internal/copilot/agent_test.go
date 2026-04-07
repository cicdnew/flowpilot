package copilot

import (
	"context"
	"testing"
)

func TestStreamChunkType(t *testing.T) {
	t.Run("content chunk", func(t *testing.T) {
		chunk := StreamChunk{Content: "test"}
		if chunk.Content != "test" {
			t.Errorf("Content = %q; want %q", chunk.Content, "test")
		}
		if chunk.Done {
			t.Error("Done should be false for content chunk")
		}
	})

	t.Run("done chunk", func(t *testing.T) {
		chunk := StreamChunk{Done: true}
		if !chunk.Done {
			t.Error("Done should be true")
		}
	})

	t.Run("error chunk", func(t *testing.T) {
		chunk := StreamChunk{Error: context.Canceled}
		if chunk.Error == nil {
			t.Error("Error should be set")
		}
	})

	t.Run("tool call chunk", func(t *testing.T) {
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
	})
}

func TestLLMProviderInterface(t *testing.T) {
	// Verify OpenAICompatibleProvider implements LLMProvider
	var _ LLMProvider = (*OpenAICompatibleProvider)(nil)
}

func TestProcessStream_RequiresConnection(t *testing.T) {
	// Note: This test would require a mock provider
	// In production, you'd inject a mock that implements StreamChatCompletion
	t.Skip("requires mock provider implementation")
}

func TestMessageStructure(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}

	if msg.Role != "user" {
		t.Errorf("Role = %q; want %q", msg.Role, "user")
	}
	if msg.Content != "Hello" {
		t.Errorf("Content = %q; want %q", msg.Content, "Hello")
	}
}

func TestToolDefinitionStructure(t *testing.T) {
	td := ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        "create_batch",
			Description: "Create a batch",
			Parameters: map[string]any{
				"type": "object",
			},
		},
	}

	if td.Type != "function" {
		t.Errorf("Type = %q; want %q", td.Type, "function")
	}
	if td.Function.Name != "create_batch" {
		t.Errorf("Name = %q; want %q", td.Function.Name, "create_batch")
	}
}

func TestChatResponseStructure(t *testing.T) {
	resp := ChatResponse{
		Content: "Hello",
		ToolCalls: []ToolCall{
			{Name: "test", Arguments: map[string]any{"a": 1}},
		},
	}

	if resp.Content != "Hello" {
		t.Errorf("Content = %q; want %q", resp.Content, "Hello")
	}
	if len(resp.ToolCalls) != 1 {
		t.Errorf("ToolCalls length = %d; want 1", len(resp.ToolCalls))
	}
}

func TestToolCallStructure(t *testing.T) {
	tc := ToolCall{
		Name:      "create_batch",
		Arguments: map[string]any{"flow_id": "123"},
	}

	if tc.Name != "create_batch" {
		t.Errorf("Name = %q; want %q", tc.Name, "create_batch")
	}
	if tc.Arguments["flow_id"] != "123" {
		t.Errorf("Arguments[flow_id] = %v; want %q", tc.Arguments["flow_id"], "123")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that Config struct has expected fields
	cfg := Config{
		DataDir:        "/tmp/test",
		MaxConcurrency: 10,
		ModelProvider:  "openai",
		APIKey:         "test-key",
		BaseURL:        "",
		ModelName:      "gpt-4",
	}

	if cfg.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d; want 10", cfg.MaxConcurrency)
	}
}

func TestStreamChunkChannel(t *testing.T) {
	// Test that StreamChunk can be sent over a channel
	ch := make(chan StreamChunk, 10)

	go func() {
		ch <- StreamChunk{Content: "test1"}
		ch <- StreamChunk{Content: "test2"}
		ch <- StreamChunk{Done: true}
	}()

	var received []string
	for chunk := range ch {
		if chunk.Done {
			break
		}
		received = append(received, chunk.Content)
	}

	if len(received) != 2 {
		t.Errorf("received %d chunks; want 2", len(received))
	}
}

func TestStreamChunkBuffer(t *testing.T) {
	// Test buffered channel behavior
	ch := make(chan StreamChunk, 64)

	// Send multiple chunks without blocking
	for i := 0; i < 50; i++ {
		ch <- StreamChunk{Content: "chunk"}
	}
	ch <- StreamChunk{Done: true}

	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count != 51 {
		t.Errorf("received %d chunks; want 51", count)
	}
}
