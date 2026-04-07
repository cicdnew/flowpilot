package copilot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ── StreamChunk type tests ─────────────────────────────────────────────────

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

// ── Interface compliance ───────────────────────────────────────────────────

func TestLLMProviderInterface(t *testing.T) {
	// Verify OpenAICompatibleProvider implements LLMProvider.
	var _ LLMProvider = (*OpenAICompatibleProvider)(nil)
}

// ── Struct shape tests (non-behavioral) ───────────────────────────────────

func TestMessageStructure(t *testing.T) {
	msg := Message{Role: "user", Content: "Hello"}
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
			Parameters:  map[string]any{"type": "object"},
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
	cfg := Config{
		DataDir:        "/tmp/test",
		MaxConcurrency: 10,
		ModelProvider:  "openai",
		APIKey:         "test-key",
		ModelName:      "gpt-4",
	}
	if cfg.MaxConcurrency != 10 {
		t.Errorf("MaxConcurrency = %d; want 10", cfg.MaxConcurrency)
	}
}

// ── StreamChunk channel mechanics ─────────────────────────────────────────

func TestStreamChunkChannel(t *testing.T) {
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
	ch := make(chan StreamChunk, 64)
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

// ── MockLLMProvider self-tests ─────────────────────────────────────────────

func TestMockProvider_DefaultChatCompletion(t *testing.T) {
	mock := &MockLLMProvider{}
	resp, err := mock.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "mock response" {
		t.Errorf("Content = %q; want %q", resp.Content, "mock response")
	}
}

func TestMockProvider_DefaultStream(t *testing.T) {
	mock := &MockLLMProvider{}
	stream := mock.StreamChatCompletion(context.Background(), nil, nil)

	var content strings.Builder
	for chunk := range stream {
		if chunk.Done {
			break
		}
		content.WriteString(chunk.Content)
	}
	if content.String() != "mock response" {
		t.Errorf("stream content = %q; want %q", content.String(), "mock response")
	}
}

func TestMockProvider_CallCount(t *testing.T) {
	mock := &MockLLMProvider{}

	if mock.CallCount("ChatCompletion") != 0 {
		t.Error("CallCount should start at zero")
	}

	mock.ChatCompletion(context.Background(), nil, nil) //nolint:errcheck
	mock.ChatCompletion(context.Background(), nil, nil) //nolint:errcheck
	mock.StreamChatCompletion(context.Background(), nil, nil)

	if mock.CallCount("ChatCompletion") != 2 {
		t.Errorf("ChatCompletion call count = %d; want 2", mock.CallCount("ChatCompletion"))
	}
	if mock.CallCount("StreamChatCompletion") != 1 {
		t.Errorf("StreamChatCompletion call count = %d; want 1", mock.CallCount("StreamChatCompletion"))
	}
}

func TestMockProvider_AllCalls_Order(t *testing.T) {
	mock := &MockLLMProvider{}
	mock.ChatCompletion(context.Background(), nil, nil) //nolint:errcheck
	mock.StreamChatCompletion(context.Background(), nil, nil)
	mock.ListModels(context.Background()) //nolint:errcheck

	calls := mock.AllCalls()
	want := []string{"ChatCompletion", "StreamChatCompletion", "ListModels"}
	if len(calls) != len(want) {
		t.Fatalf("AllCalls length = %d; want %d", len(calls), len(want))
	}
	for i, c := range calls {
		if c != want[i] {
			t.Errorf("calls[%d] = %q; want %q", i, c, want[i])
		}
	}
}

func TestMockProvider_IdentityMethods(t *testing.T) {
	mock := &MockLLMProvider{
		ModelID:         "gpt-4o",
		ProviderID:      "openai",
		FunctionCalling: true,
	}
	if mock.Model() != "gpt-4o" {
		t.Errorf("Model() = %q; want %q", mock.Model(), "gpt-4o")
	}
	if mock.Provider() != "openai" {
		t.Errorf("Provider() = %q; want %q", mock.Provider(), "openai")
	}
	if !mock.SupportsFunctionCalling() {
		t.Error("SupportsFunctionCalling() should be true")
	}
}

// ── Process (non-streaming) ────────────────────────────────────────────────

func TestProcess_NoProvider(t *testing.T) {
	flow := &CopilotFlow{tools: make(map[string]Tool)}

	_, err := flow.Process(context.Background(), "hello")

	if err == nil {
		t.Fatal("expected error when no provider is set")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "not connected")
	}
}

func TestProcess_ReturnsContent(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{Content: "automation complete"}, nil
		},
	}
	flow := newTestFlow(mock)

	result, err := flow.Process(context.Background(), "run automation")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "automation complete" {
		t.Errorf("result = %q; want %q", result, "automation complete")
	}
	if mock.CallCount("ChatCompletion") != 1 {
		t.Errorf("ChatCompletion called %d times; want 1", mock.CallCount("ChatCompletion"))
	}
}

func TestProcess_APIError(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{}, fmt.Errorf("rate limit exceeded")
		},
	}
	flow := newTestFlow(mock)

	_, err := flow.Process(context.Background(), "test")

	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "rate limit exceeded")
	}
}

func TestProcess_ToolCall_Success(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{
				ToolCalls: []ToolCall{
					{Name: "count_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}
	flow := withTool(newTestFlow(mock), "count_tool", func(_ context.Context, _ map[string]any) (any, error) {
		return 42, nil
	})

	result, err := flow.Process(context.Background(), "count something")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "count_tool result: 42") {
		t.Errorf("result = %q; want to contain %q", result, "count_tool result: 42")
	}
}

func TestProcess_ToolCall_HandlerError(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{
				ToolCalls: []ToolCall{
					{Name: "err_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}
	flow := withTool(newTestFlow(mock), "err_tool", func(_ context.Context, _ map[string]any) (any, error) {
		return nil, fmt.Errorf("internal failure")
	})

	result, err := flow.Process(context.Background(), "trigger error tool")

	if err != nil {
		t.Fatalf("tool handler errors are embedded in result, not returned: %v", err)
	}
	if !strings.Contains(result, "Error in err_tool: internal failure") {
		t.Errorf("result = %q; want to contain tool error message", result)
	}
}

func TestProcess_ToolCall_UnknownTool(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{
				ToolCalls: []ToolCall{
					{Name: "ghost_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}
	flow := newTestFlow(mock) // no tools registered

	result, err := flow.Process(context.Background(), "use unregistered tool")

	if err != nil {
		t.Fatalf("unknown tool errors are embedded in result, not returned: %v", err)
	}
	if !strings.Contains(result, "Unknown tool: ghost_tool") {
		t.Errorf("result = %q; want to contain %q", result, "Unknown tool: ghost_tool")
	}
}

func TestProcess_ToolCall_WithContent(t *testing.T) {
	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) (ChatResponse, error) {
			return ChatResponse{
				Content: "Here is the result:",
				ToolCalls: []ToolCall{
					{Name: "info_tool", Arguments: map[string]any{}},
				},
			}, nil
		},
	}
	flow := withTool(newTestFlow(mock), "info_tool", func(_ context.Context, _ map[string]any) (any, error) {
		return "some info", nil
	})

	result, err := flow.Process(context.Background(), "get info with narration")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Here is the result:") {
		t.Errorf("result = %q; want to contain LLM content prefix", result)
	}
	if !strings.Contains(result, "info_tool result: some info") {
		t.Errorf("result = %q; want to contain tool result", result)
	}
}

// ── ProcessStream (streaming) ──────────────────────────────────────────────

func TestProcessStream_NoProvider(t *testing.T) {
	flow := &CopilotFlow{tools: make(map[string]Tool)}

	err := flow.ProcessStream(context.Background(), "hello", func(string) {})

	if err == nil {
		t.Fatal("expected error when no provider is set")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "not connected")
	}
}

func TestProcessStream_DeliversTokens(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeMultiStream("Hello", ",", " world", "!")
		},
	}
	flow := newTestFlow(mock)

	var received []string
	err := flow.ProcessStream(context.Background(), "say hello", func(token string) {
		received = append(received, token)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Join(received, ""); got != "Hello, world!" {
		t.Errorf("assembled content = %q; want %q", got, "Hello, world!")
	}
	if len(received) != 4 {
		t.Errorf("token count = %d; want 4 (one per makeMultiStream argument)", len(received))
	}
	if mock.CallCount("StreamChatCompletion") != 1 {
		t.Errorf("StreamChatCompletion called %d times; want 1", mock.CallCount("StreamChatCompletion"))
	}
}

func TestProcessStream_SingleToken(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeStaticStream("single chunk response")
		},
	}
	flow := newTestFlow(mock)

	var received []string
	err := flow.ProcessStream(context.Background(), "test", func(token string) {
		received = append(received, token)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 1 || received[0] != "single chunk response" {
		t.Errorf("received = %v; want [\"single chunk response\"]", received)
	}
}

func TestProcessStream_ToolCall_Success(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeToolCallStream("echo_tool", map[string]any{"input": "hello"})
		},
	}
	flow := withTool(newTestFlow(mock), "echo_tool", func(_ context.Context, args map[string]any) (any, error) {
		return args["input"], nil
	})

	var tokens []string
	err := flow.ProcessStream(context.Background(), "echo hello", func(token string) {
		tokens = append(tokens, token)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	combined := strings.Join(tokens, "")
	if !strings.Contains(combined, "echo_tool result: hello") {
		t.Errorf("output = %q; want to contain %q", combined, "echo_tool result: hello")
	}
}

func TestProcessStream_ToolCall_HandlerError(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeToolCallStream("fail_tool", nil)
		},
	}
	flow := withTool(newTestFlow(mock), "fail_tool", func(_ context.Context, _ map[string]any) (any, error) {
		return nil, fmt.Errorf("tool failure")
	})

	var tokens []string
	err := flow.ProcessStream(context.Background(), "trigger failing tool", func(token string) {
		tokens = append(tokens, token)
	})

	// Handler errors are surfaced via onToken, not as a returned error.
	if err != nil {
		t.Fatalf("handler errors must be delivered via onToken, not returned: %v", err)
	}
	combined := strings.Join(tokens, "")
	if !strings.Contains(combined, "Error in fail_tool: tool failure") {
		t.Errorf("output = %q; want to contain %q", combined, "Error in fail_tool: tool failure")
	}
}

func TestProcessStream_ToolCall_UnknownTool(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeToolCallStream("nonexistent_tool", nil)
		},
	}
	flow := newTestFlow(mock) // no tools registered

	var tokens []string
	err := flow.ProcessStream(context.Background(), "use unknown tool", func(token string) {
		tokens = append(tokens, token)
	})

	if err != nil {
		t.Fatalf("unknown tool errors must be delivered via onToken, not returned: %v", err)
	}
	combined := strings.Join(tokens, "")
	if !strings.Contains(combined, "Unknown tool: nonexistent_tool") {
		t.Errorf("output = %q; want to contain %q", combined, "Unknown tool: nonexistent_tool")
	}
}

func TestProcessStream_StreamError(t *testing.T) {
	sentinelErr := fmt.Errorf("provider failure")
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeErrorStream(sentinelErr)
		},
	}
	flow := newTestFlow(mock)

	err := flow.ProcessStream(context.Background(), "test", func(string) {})

	if err == nil {
		t.Fatal("expected error from stream error chunk")
	}
	if !errors.Is(err, sentinelErr) {
		t.Errorf("error = %v; want %v (errors.Is check failed)", err, sentinelErr)
	}
}

func TestProcessStream_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so makeBlockingStream unblocks immediately

	mock := &MockLLMProvider{
		StreamFunc: func(ctx context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			return makeBlockingStream(ctx)
		},
	}
	flow := newTestFlow(mock)

	done := make(chan error, 1)
	go func() {
		done <- flow.ProcessStream(ctx, "test", func(string) {})
	}()

	select {
	case err := <-done:
		// ProcessStream must have returned; verify the cause is context-related.
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v; want context.Canceled or nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ProcessStream did not return after context cancellation — possible goroutine leak")
	}
}

func TestProcessStream_EmitsNoTokensOnEmptyContent(t *testing.T) {
	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, _ []Message, _ []ToolDefinition) <-chan StreamChunk {
			// Stream that has no content — only a Done signal.
			ch := make(chan StreamChunk, 1)
			ch <- StreamChunk{Done: true}
			return ch
		},
	}
	flow := newTestFlow(mock)

	callCount := 0
	err := flow.ProcessStream(context.Background(), "test", func(string) {
		callCount++
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 0 {
		t.Errorf("onToken called %d times; want 0 for empty stream", callCount)
	}
}

func TestProcessStream_PassesInputAsUserMessage(t *testing.T) {
	const wantInput = "automate login flow"
	var capturedMessages []Message

	mock := &MockLLMProvider{
		StreamFunc: func(_ context.Context, messages []Message, _ []ToolDefinition) <-chan StreamChunk {
			capturedMessages = messages
			return makeStaticStream("ok")
		},
	}
	flow := newTestFlow(mock)

	flow.ProcessStream(context.Background(), wantInput, func(string) {}) //nolint:errcheck

	// Expect at least a system message and the user message.
	if len(capturedMessages) < 2 {
		t.Fatalf("message count = %d; want at least 2 (system + user)", len(capturedMessages))
	}
	last := capturedMessages[len(capturedMessages)-1]
	if last.Role != "user" {
		t.Errorf("last message role = %q; want %q", last.Role, "user")
	}
	if last.Content != wantInput {
		t.Errorf("last message content = %q; want %q", last.Content, wantInput)
	}
}

func TestProcess_PassesInputAsUserMessage(t *testing.T) {
	const wantInput = "list all tasks"
	var capturedMessages []Message

	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, messages []Message, _ []ToolDefinition) (ChatResponse, error) {
			capturedMessages = messages
			return ChatResponse{Content: "ok"}, nil
		},
	}
	flow := newTestFlow(mock)

	flow.Process(context.Background(), wantInput) //nolint:errcheck

	if len(capturedMessages) < 2 {
		t.Fatalf("message count = %d; want at least 2 (system + user)", len(capturedMessages))
	}
	last := capturedMessages[len(capturedMessages)-1]
	if last.Role != "user" {
		t.Errorf("last message role = %q; want %q", last.Role, "user")
	}
	if last.Content != wantInput {
		t.Errorf("last message content = %q; want %q", last.Content, wantInput)
	}
}
