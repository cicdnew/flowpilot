package copilot

import (
	"context"
	"sync"
)

// MockLLMProvider is a configurable test double for the LLMProvider interface.
//
// Set the Func fields to inject custom behavior per test case.
// Zero value is safe: produces a "mock response" content with no tool calls.
//
// All method invocations are recorded and retrievable via CallCount.
type MockLLMProvider struct {
	// ChatCompletionFunc controls ChatCompletion behavior.
	// Defaults to returning ChatResponse{Content: "mock response"} with no error.
	ChatCompletionFunc func(ctx context.Context, messages []Message, tools []ToolDefinition) (ChatResponse, error)

	// StreamFunc controls StreamChatCompletion behavior.
	// Defaults to a single-chunk stream emitting "mock response" then Done.
	StreamFunc func(ctx context.Context, messages []Message, tools []ToolDefinition) <-chan StreamChunk

	// ModelsFunc controls ListModels behavior.
	// Defaults to returning a single synthetic "mock-model".
	ModelsFunc func(ctx context.Context) ([]Model, error)

	// FunctionCalling controls SupportsFunctionCalling.
	FunctionCalling bool

	// ModelID is returned by Model().
	ModelID string

	// ProviderID is returned by Provider().
	ProviderID string

	mu    sync.Mutex
	calls []string
}

// Compile-time proof that MockLLMProvider satisfies LLMProvider.
var _ LLMProvider = (*MockLLMProvider)(nil)

// record appends a method name to the call log under the mutex.
func (m *MockLLMProvider) record(method string) {
	m.mu.Lock()
	m.calls = append(m.calls, method)
	m.mu.Unlock()
}

// CallCount returns how many times the given method name was invoked.
func (m *MockLLMProvider) CallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.calls {
		if c == method {
			n++
		}
	}
	return n
}

// AllCalls returns a snapshot of every recorded method call in order.
func (m *MockLLMProvider) AllCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.calls))
	copy(out, m.calls)
	return out
}

// ChatCompletion implements LLMProvider.
func (m *MockLLMProvider) ChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) (ChatResponse, error) {
	m.record("ChatCompletion")
	if m.ChatCompletionFunc != nil {
		return m.ChatCompletionFunc(ctx, messages, tools)
	}
	return ChatResponse{Content: "mock response"}, nil
}

// StreamChatCompletion implements LLMProvider.
func (m *MockLLMProvider) StreamChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) <-chan StreamChunk {
	m.record("StreamChatCompletion")
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, messages, tools)
	}
	return makeStaticStream("mock response")
}

// ListModels implements LLMProvider.
func (m *MockLLMProvider) ListModels(ctx context.Context) ([]Model, error) {
	m.record("ListModels")
	if m.ModelsFunc != nil {
		return m.ModelsFunc(ctx)
	}
	return []Model{
		{
			ID:       "mock-model",
			Name:     "Mock Model",
			Provider: m.ProviderID,
		},
	}, nil
}

// SupportsFunctionCalling implements LLMProvider.
func (m *MockLLMProvider) SupportsFunctionCalling() bool { return m.FunctionCalling }

// Model implements LLMProvider.
func (m *MockLLMProvider) Model() string { return m.ModelID }

// Provider implements LLMProvider.
func (m *MockLLMProvider) Provider() string { return m.ProviderID }

// ── Stream factory helpers ─────────────────────────────────────────────────
//
// Each factory returns a pre-filled, closed-ready channel so tests stay
// deterministic and never block the test goroutine.

// makeStaticStream returns a channel that emits one content chunk then Done.
func makeStaticStream(content string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Content: content}
	ch <- StreamChunk{Done: true}
	return ch
}

// makeMultiStream emits each token as a separate content chunk, then Done.
// Callers control the exact token granularity seen by ProcessStream.
func makeMultiStream(tokens ...string) <-chan StreamChunk {
	ch := make(chan StreamChunk, len(tokens)+1)
	for _, t := range tokens {
		ch <- StreamChunk{Content: t}
	}
	ch <- StreamChunk{Done: true}
	return ch
}

// makeErrorStream emits a single error chunk and closes the channel.
// ProcessStream will surface the error as its return value.
func makeErrorStream(err error) <-chan StreamChunk {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Error: err}
	close(ch)
	return ch
}

// makeToolCallStream emits one ToolCall chunk followed by Done.
// Use this to exercise the tool-dispatch path inside ProcessStream.
func makeToolCallStream(name string, args map[string]any) <-chan StreamChunk {
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{ToolCall: &ToolCall{Name: name, Arguments: args}}
	ch <- StreamChunk{Done: true}
	return ch
}

// makeBlockingStream blocks until ctx is cancelled, then emits the context
// error and closes. Use this to verify that ProcessStream respects
// context cancellation without hanging forever.
func makeBlockingStream(ctx context.Context) <-chan StreamChunk {
	ch := make(chan StreamChunk, 1)
	go func() {
		defer close(ch)
		<-ctx.Done()
		ch <- StreamChunk{Error: ctx.Err()}
	}()
	return ch
}

// ── Minimal CopilotFlow constructors ──────────────────────────────────────

// newTestFlow builds the smallest possible CopilotFlow that can exercise
// Process and ProcessStream: provider wired, empty tool registry, no DB or
// browser. The sync.RWMutex zero value (unlocked) is valid for Go.
func newTestFlow(provider LLMProvider) *CopilotFlow {
	return &CopilotFlow{
		provider: provider,
		tools:    make(map[string]Tool),
	}
}

// withTool registers handler under name on flow and returns flow for chaining.
//
//	flow := withTool(newTestFlow(mock), "my_tool", func(ctx context.Context, args map[string]any) (any, error) {
//	    return "result", nil
//	})
func withTool(
	flow *CopilotFlow,
	name string,
	handler func(context.Context, map[string]any) (any, error),
) *CopilotFlow {
	flow.tools[name] = Tool{Name: name, Handler: handler}
	return flow
}
