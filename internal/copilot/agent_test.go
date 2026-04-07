package copilot

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/queue"
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

// ── Integration test helpers ───────────────────────────────────────────────
//
// These helpers create real SQLite databases in temporary directories so that
// tool handler implementations can be exercised end-to-end without mocking
// the database layer.

// newTestFlowWithDB creates a CopilotFlow backed by a real SQLite database in a
// temporary directory. registerTools() is called so all tool handlers are wired.
// The DB is closed automatically via t.Cleanup.
func newTestFlowWithDB(t *testing.T, mock LLMProvider) *CopilotFlow {
	t.Helper()
	dir := t.TempDir()

	if err := crypto.InitKey(dir); err != nil {
		t.Fatalf("newTestFlowWithDB: init crypto: %v", err)
	}

	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("newTestFlowWithDB: open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	flow := &CopilotFlow{
		db:       db,
		provider: mock,
		tools:    make(map[string]Tool),
		history:  &ConversationHistory{},
	}
	flow.registerTools()
	return flow
}

// newTestFlowWithQueue creates a CopilotFlow with a real DB and a zero-worker
// queue. Zero workers means submitted tasks are enqueued but never executed,
// keeping tests deterministic and free of browser-launch side effects.
func newTestFlowWithQueue(t *testing.T, mock LLMProvider) *CopilotFlow {
	t.Helper()
	dir := t.TempDir()

	if err := crypto.InitKey(dir); err != nil {
		t.Fatalf("newTestFlowWithQueue: init crypto: %v", err)
	}

	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("newTestFlowWithQueue: open db: %v", err)
	}

	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
	if err != nil {
		t.Fatalf("newTestFlowWithQueue: init runner: %v", err)
	}

	// workerCount=0 ensures no goroutines process tasks — no real browser is launched.
	q := queue.New(db, runner, 0, func(models.TaskEvent) {})

	t.Cleanup(func() {
		q.Stop()
		_ = db.Close()
	})

	flow := &CopilotFlow{
		db:       db,
		queue:    q,
		provider: mock,
		tools:    make(map[string]Tool),
		history:  &ConversationHistory{},
	}
	flow.registerTools()
	return flow
}

// ── Multi-turn conversation history ───────────────────────────────────────

// TestProcess_ConversationHistory_SecondCallIncludesFirstTurn verifies that
// the second Process call prepends the first turn (user + assistant messages)
// in the message slice forwarded to the LLM provider.
func TestProcess_ConversationHistory_SecondCallIncludesFirstTurn(t *testing.T) {
	var capturedMessages [][]Message

	mock := &MockLLMProvider{
		ChatCompletionFunc: func(_ context.Context, messages []Message, _ []ToolDefinition) (ChatResponse, error) {
			snapshot := make([]Message, len(messages))
			copy(snapshot, messages)
			capturedMessages = append(capturedMessages, snapshot)
			return ChatResponse{Content: "ok"}, nil
		},
	}
	flow := newTestFlow(mock)
	flow.history = &ConversationHistory{} // ensure fresh

	if _, err := flow.Process(context.Background(), "first question"); err != nil {
		t.Fatalf("first Process: %v", err)
	}
	if _, err := flow.Process(context.Background(), "second question"); err != nil {
		t.Fatalf("second Process: %v", err)
	}

	if len(capturedMessages) < 2 {
		t.Fatalf("expected 2 LLM calls; got %d", len(capturedMessages))
	}

	secondCall := capturedMessages[1]
	// Must contain: system + user("first question") + assistant("ok") + user("second question")
	if len(secondCall) < 4 {
		t.Errorf("second call has %d messages; want >= 4 (system+prior user+prior assistant+new user)", len(secondCall))
		return
	}

	// The last message must be the current user turn.
	last := secondCall[len(secondCall)-1]
	if last.Role != "user" || last.Content != "second question" {
		t.Errorf("last message = {%s %q}; want {user \"second question\"}", last.Role, last.Content)
	}

	// Prior turns must contain the first user message.
	foundPriorUser := false
	for _, msg := range secondCall[1 : len(secondCall)-1] {
		if msg.Role == "user" && msg.Content == "first question" {
			foundPriorUser = true
		}
	}
	if !foundPriorUser {
		t.Error("second LLM call is missing prior user turn (\"first question\")")
	}
}

// ── Tool registration count ───────────────────────────────────────────────

// TestGetToolDefinitions_CountIs16 asserts that exactly 16 tools are registered
// after registerTools(): 6 original + 10 added in v2.
func TestGetToolDefinitions_CountIs16(t *testing.T) {
	flow := newTestFlow(&MockLLMProvider{})
	flow.registerTools()

	defs := flow.GetToolDefinitions()
	if len(defs) != 16 {
		var names []string
		for _, d := range defs {
			names = append(names, d.Function.Name)
		}
		t.Errorf("tool count = %d; want 16 (6 original + 10 new)\nregistered: %v", len(defs), names)
	}
}

// ── get_task ──────────────────────────────────────────────────────────────

// TestToolGetTask_ReturnsTaskFields verifies that toolGetTask returns the
// expected field set populated with the correct values from the database.
func TestToolGetTask_ReturnsTaskFields(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	now := time.Now()
	startedAt := now.Add(time.Second)
	completedAt := startedAt.Add(2 * time.Second)
	task := models.Task{
		ID:          "tgt-1",
		Name:        "sample task",
		URL:         "https://example.com",
		Status:      models.TaskStatusCompleted,
		Steps:       []models.TaskStep{{Action: models.ActionNavigate}, {Action: models.ActionClick}},
		CreatedAt:   now,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
	}
	if err := flow.db.CreateTask(ctx, task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	result, err := flow.tools["get_task"].Handler(ctx, map[string]any{"task_id": "tgt-1"})
	if err != nil {
		t.Fatalf("toolGetTask: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["id"] != "tgt-1" {
		t.Errorf("id = %v; want %q", m["id"], "tgt-1")
	}
	if m["name"] != "sample task" {
		t.Errorf("name = %v; want %q", m["name"], "sample task")
	}
	if m["url"] != "https://example.com" {
		t.Errorf("url = %v; want %q", m["url"], "https://example.com")
	}
	if got, _ := m["steps_count"].(int); got != 2 {
		t.Errorf("steps_count = %v; want 2", m["steps_count"])
	}
	if _, ok := m["duration_ms"]; !ok {
		t.Error("duration_ms field missing from result")
	}
}

// TestToolGetTask_MissingTaskID verifies that an empty/absent task_id returns
// a descriptive error before any DB call is made.
func TestToolGetTask_MissingTaskID(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	_, err := flow.tools["get_task"].Handler(ctx, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing task_id argument")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Errorf("error = %q; want message mentioning task_id", err.Error())
	}
}

// ── cancel_task ───────────────────────────────────────────────────────────

// TestToolCancelTask_CancelsSuccessfully verifies that toolCancelTask updates
// the task status to cancelled and returns the expected response fields.
func TestToolCancelTask_CancelsSuccessfully(t *testing.T) {
	flow := newTestFlowWithQueue(t, &MockLLMProvider{})
	ctx := context.Background()

	task := models.Task{
		ID:        "tct-1",
		Name:      "cancellable task",
		URL:       "https://example.com",
		Status:    models.TaskStatusPending,
		Steps:     []models.TaskStep{{Action: models.ActionNavigate}},
		CreatedAt: time.Now(),
	}
	if err := flow.db.CreateTask(ctx, task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	result, err := flow.tools["cancel_task"].Handler(ctx, map[string]any{"task_id": "tct-1"})
	if err != nil {
		t.Fatalf("toolCancelTask: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["task_id"] != "tct-1" {
		t.Errorf("task_id = %v; want %q", m["task_id"], "tct-1")
	}
	if _, hasMsg := m["message"]; !hasMsg {
		t.Error("result missing message field")
	}
}

// TestToolCancelTask_MissingTaskID verifies argument validation.
func TestToolCancelTask_MissingTaskID(t *testing.T) {
	flow := newTestFlowWithQueue(t, &MockLLMProvider{})
	ctx := context.Background()

	_, err := flow.tools["cancel_task"].Handler(ctx, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing task_id argument")
	}
}

// ── retry_task ────────────────────────────────────────────────────────────

// TestToolRetryTask_ResetsPendingStatus verifies that toolRetryTask resets a
// failed task's status to pending and clears its retry count.
func TestToolRetryTask_ResetsPendingStatus(t *testing.T) {
	flow := newTestFlowWithQueue(t, &MockLLMProvider{})
	ctx := context.Background()

	task := models.Task{
		ID:         "trt-1",
		Name:       "failed task",
		URL:        "https://example.com",
		Status:     models.TaskStatusFailed,
		Error:      "network error",
		RetryCount: 2,
		MaxRetries: 3,
		Steps:      []models.TaskStep{{Action: models.ActionNavigate}},
		CreatedAt:  time.Now(),
	}
	if err := flow.db.CreateTask(ctx, task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	result, err := flow.tools["retry_task"].Handler(ctx, map[string]any{"task_id": "trt-1"})
	if err != nil {
		t.Fatalf("toolRetryTask: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["task_id"] != "trt-1" {
		t.Errorf("task_id = %v; want %q", m["task_id"], "trt-1")
	}
	if m["status"] != string(models.TaskStatusPending) {
		t.Errorf("status = %v; want %q", m["status"], models.TaskStatusPending)
	}

	// Verify the DB reflects the reset state.
	updated, err := flow.db.GetTask(ctx, "trt-1")
	if err != nil {
		t.Fatalf("get updated task: %v", err)
	}
	if updated.Status != models.TaskStatusPending {
		t.Errorf("DB status = %q; want %q", updated.Status, models.TaskStatusPending)
	}
	if updated.RetryCount != 0 {
		t.Errorf("DB retry_count = %d; want 0", updated.RetryCount)
	}
}

// ── get_batch_progress ────────────────────────────────────────────────────

// TestToolGetBatchProgress_CountsByStatus seeds tasks with different statuses
// into one batch and verifies that the progress counters are correct.
func TestToolGetBatchProgress_CountsByStatus(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	batchID := "batch-progress-1"
	statuses := []models.TaskStatus{
		models.TaskStatusPending,
		models.TaskStatusPending,
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
	}
	for i, s := range statuses {
		task := models.Task{
			ID:        fmt.Sprintf("tbp-%d", i),
			Name:      fmt.Sprintf("batch task %d", i),
			URL:       "https://example.com",
			Status:    s,
			BatchID:   batchID,
			Steps:     []models.TaskStep{{Action: models.ActionNavigate}},
			CreatedAt: time.Now(),
		}
		if err := flow.db.CreateTask(ctx, task); err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}

	result, err := flow.tools["get_batch_progress"].Handler(ctx, map[string]any{"batch_id": batchID})
	if err != nil {
		t.Fatalf("toolGetBatchProgress: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["batch_id"] != batchID {
		t.Errorf("batch_id = %v; want %q", m["batch_id"], batchID)
	}
	if got, _ := m["total"].(int); got != 4 {
		t.Errorf("total = %v; want 4", m["total"])
	}
	if got, _ := m["pending"].(int); got != 2 {
		t.Errorf("pending = %v; want 2", m["pending"])
	}
	if got, _ := m["completed"].(int); got != 1 {
		t.Errorf("completed = %v; want 1", m["completed"])
	}
	if got, _ := m["failed"].(int); got != 1 {
		t.Errorf("failed = %v; want 1", m["failed"])
	}
}

// ── cancel_batch ──────────────────────────────────────────────────────────

// TestToolCancelBatch_CancelsActiveTasks verifies that only pending/queued/running
// tasks in the batch are cancelled, not already-completed ones.
func TestToolCancelBatch_CancelsActiveTasks(t *testing.T) {
	flow := newTestFlowWithQueue(t, &MockLLMProvider{})
	ctx := context.Background()

	batchID := "batch-cancel-1"
	seeds := []struct {
		id     string
		status models.TaskStatus
	}{
		{"tcb-1", models.TaskStatusPending},
		{"tcb-2", models.TaskStatusPending},
		{"tcb-3", models.TaskStatusCompleted}, // must NOT be cancelled
	}
	for _, s := range seeds {
		task := models.Task{
			ID:        s.id,
			Name:      "batch cancel task",
			URL:       "https://example.com",
			Status:    s.status,
			BatchID:   batchID,
			Steps:     []models.TaskStep{{Action: models.ActionNavigate}},
			CreatedAt: time.Now(),
		}
		if err := flow.db.CreateTask(ctx, task); err != nil {
			t.Fatalf("seed %s: %v", s.id, err)
		}
	}

	result, err := flow.tools["cancel_batch"].Handler(ctx, map[string]any{"batch_id": batchID})
	if err != nil {
		t.Fatalf("toolCancelBatch: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["batch_id"] != batchID {
		t.Errorf("batch_id = %v; want %q", m["batch_id"], batchID)
	}
	if got, _ := m["cancelled_count"].(int); got != 2 {
		t.Errorf("cancelled_count = %v; want 2 (only pending tasks)", m["cancelled_count"])
	}
}

// ── get_task_logs ─────────────────────────────────────────────────────────

// TestToolGetTaskLogs_ReturnsStepLogs verifies that step logs are returned with
// the expected field structure.
func TestToolGetTaskLogs_ReturnsStepLogs(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	task := models.Task{
		ID:        "tgl-1",
		Name:      "log task",
		URL:       "https://example.com",
		Status:    models.TaskStatusCompleted,
		Steps:     []models.TaskStep{{Action: models.ActionNavigate}},
		CreatedAt: time.Now(),
	}
	if err := flow.db.CreateTask(ctx, task); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := flow.db.InsertStepLogs(ctx, "tgl-1", []models.StepLog{
		{TaskID: "tgl-1", StepIndex: 0, Action: models.ActionNavigate, DurationMs: 100, StartedAt: time.Now()},
		{TaskID: "tgl-1", StepIndex: 1, Action: models.ActionClick, DurationMs: 50, StartedAt: time.Now()},
	}); err != nil {
		t.Fatalf("seed step logs: %v", err)
	}

	result, err := flow.tools["get_task_logs"].Handler(ctx, map[string]any{"task_id": "tgl-1", "limit": float64(10)})
	if err != nil {
		t.Fatalf("toolGetTaskLogs: %v", err)
	}

	entries, ok := result.([]map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want []map[string]any", result)
	}
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d; want 2", len(entries))
		return
	}
	for _, key := range []string{"step", "action", "status", "message", "duration_ms"} {
		if _, ok := entries[0][key]; !ok {
			t.Errorf("entry missing field %q", key)
		}
	}
}

// TestToolGetTaskLogs_LimitApplied verifies that the limit argument caps the
// number of returned log entries.
func TestToolGetTaskLogs_LimitApplied(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	task := models.Task{
		ID:        "tgl-lim-1",
		Name:      "log limit task",
		URL:       "https://example.com",
		Status:    models.TaskStatusCompleted,
		Steps:     []models.TaskStep{{Action: models.ActionNavigate}},
		CreatedAt: time.Now(),
	}
	if err := flow.db.CreateTask(ctx, task); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	var logSlice []models.StepLog
	for i := 0; i < 5; i++ {
		logSlice = append(logSlice, models.StepLog{
			TaskID: "tgl-lim-1", StepIndex: i, Action: models.ActionNavigate,
			DurationMs: int64(i * 10), StartedAt: time.Now(),
		})
	}
	if err := flow.db.InsertStepLogs(ctx, "tgl-lim-1", logSlice); err != nil {
		t.Fatalf("seed step logs: %v", err)
	}

	result, err := flow.tools["get_task_logs"].Handler(ctx, map[string]any{"task_id": "tgl-lim-1", "limit": float64(2)})
	if err != nil {
		t.Fatalf("toolGetTaskLogs with limit: %v", err)
	}

	entries, ok := result.([]map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want []map[string]any", result)
	}
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d; want 2 (limit applied)", len(entries))
	}
}

// ── add_proxy ─────────────────────────────────────────────────────────────

// TestToolAddProxy_CreatesProxy verifies that toolAddProxy inserts the proxy
// into the database and returns proxy_id and server in the response.
func TestToolAddProxy_CreatesProxy(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	result, err := flow.tools["add_proxy"].Handler(ctx, map[string]any{
		"server":   "proxy.example.com:8080",
		"protocol": "http",
		"username": "user1",
		"password": "pass1",
		"geo":      "US",
	})
	if err != nil {
		t.Fatalf("toolAddProxy: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["server"] != "proxy.example.com:8080" {
		t.Errorf("server = %v; want %q", m["server"], "proxy.example.com:8080")
	}
	proxyID, _ := m["proxy_id"].(string)
	if proxyID == "" {
		t.Error("proxy_id must be a non-empty string")
	}

	// Verify the proxy was persisted.
	proxies, err := flow.db.ListProxies(ctx)
	if err != nil {
		t.Fatalf("list proxies: %v", err)
	}
	if len(proxies) != 1 {
		t.Errorf("proxy count = %d; want 1", len(proxies))
	}
}

// TestToolAddProxy_MissingServer verifies argument validation.
func TestToolAddProxy_MissingServer(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	_, err := flow.tools["add_proxy"].Handler(ctx, map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing server argument")
	}
}

// ── delete_proxy ──────────────────────────────────────────────────────────

// TestToolDeleteProxy_RemovesProxy creates a proxy via toolAddProxy then
// deletes it via toolDeleteProxy, verifying the DB is empty afterwards.
func TestToolDeleteProxy_RemovesProxy(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	addResult, err := flow.tools["add_proxy"].Handler(ctx, map[string]any{
		"server":   "todelete.example.com:3128",
		"protocol": "http",
	})
	if err != nil {
		t.Fatalf("add proxy for delete test: %v", err)
	}
	proxyID := addResult.(map[string]any)["proxy_id"].(string)

	result, err := flow.tools["delete_proxy"].Handler(ctx, map[string]any{"proxy_id": proxyID})
	if err != nil {
		t.Fatalf("toolDeleteProxy: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	if m["proxy_id"] != proxyID {
		t.Errorf("proxy_id = %v; want %q", m["proxy_id"], proxyID)
	}

	// Verify the proxy is gone from the database.
	proxies, err := flow.db.ListProxies(ctx)
	if err != nil {
		t.Fatalf("list proxies after delete: %v", err)
	}
	if len(proxies) != 0 {
		t.Errorf("proxy count after delete = %d; want 0", len(proxies))
	}
}

// ── list_schedules ────────────────────────────────────────────────────────

// TestToolListSchedules_ReturnsSchedules seeds two schedules and verifies that
// toolListSchedules returns them with the required fields.
func TestToolListSchedules_ReturnsSchedules(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		s := models.Schedule{
			ID:        fmt.Sprintf("sched-ls-%d", i),
			Name:      fmt.Sprintf("schedule %d", i),
			CronExpr:  "0 * * * *",
			FlowID:    "flow-1",
			Enabled:   true,
			Priority:  models.PriorityNormal,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := flow.db.CreateSchedule(ctx, s); err != nil {
			t.Fatalf("seed schedule %d: %v", i, err)
		}
	}

	result, err := flow.tools["list_schedules"].Handler(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("toolListSchedules: %v", err)
	}

	entries, ok := result.([]map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want []map[string]any", result)
	}
	if len(entries) != 2 {
		t.Errorf("schedule count = %d; want 2", len(entries))
		return
	}
	for i, e := range entries {
		for _, key := range []string{"id", "name", "flow_id", "cron", "enabled"} {
			if _, ok := e[key]; !ok {
				t.Errorf("entry[%d] missing required field %q", i, key)
			}
		}
	}
}

// ── create_schedule ───────────────────────────────────────────────────────

// TestToolCreateSchedule_CreatesSchedule verifies that toolCreateSchedule
// persists the schedule and returns the expected response fields.
func TestToolCreateSchedule_CreatesSchedule(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	result, err := flow.tools["create_schedule"].Handler(ctx, map[string]any{
		"flow_id": "flow-cs-1",
		"name":    "nightly job",
		"cron":    "0 2 * * *",
	})
	if err != nil {
		t.Fatalf("toolCreateSchedule: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T; want map[string]any", result)
	}
	scheduleID, _ := m["schedule_id"].(string)
	if scheduleID == "" {
		t.Error("schedule_id must be a non-empty string")
	}
	if m["name"] != "nightly job" {
		t.Errorf("name = %v; want %q", m["name"], "nightly job")
	}
	if m["cron"] != "0 2 * * *" {
		t.Errorf("cron = %v; want %q", m["cron"], "0 2 * * *")
	}

	// Verify the schedule was persisted in the database.
	schedules, err := flow.db.ListSchedules(ctx)
	if err != nil {
		t.Fatalf("list schedules: %v", err)
	}
	if len(schedules) != 1 {
		t.Errorf("schedule count = %d; want 1", len(schedules))
	}
}

// TestToolCreateSchedule_MissingFlowID verifies that a missing flow_id
// produces an error before any database call.
func TestToolCreateSchedule_MissingFlowID(t *testing.T) {
	flow := newTestFlowWithDB(t, &MockLLMProvider{})
	ctx := context.Background()

	_, err := flow.tools["create_schedule"].Handler(ctx, map[string]any{
		"name": "bad schedule",
		"cron": "0 * * * *",
	})
	if err == nil {
		t.Fatal("expected error for missing flow_id argument")
	}
}
