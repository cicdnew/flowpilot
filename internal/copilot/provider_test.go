package copilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamChatCompletion(t *testing.T) {
	t.Run("streams content chunks", func(t *testing.T) {
		// Create test server that streams SSE
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			// Stream chunks
			chunks := []string{
				`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
				`data: {"choices":[{"delta":{"content":" world"}}]}`,
				`data: {"choices":[{"delta":{"content":"!"}}]}`,
				`data: [DONE]`,
			}

			for _, chunk := range chunks {
				w.Write([]byte(chunk + "\n\n"))
			}
		}))
		defer server.Close()

		provider := NewOpenAICompatibleProvider("test", "test-key", server.URL, "test-model")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stream := provider.StreamChatCompletion(ctx, []Message{{Role: "user", Content: "test"}}, nil)

		var content strings.Builder
		var doneReceived bool

		for chunk := range stream {
			if chunk.Error != nil {
				t.Fatalf("unexpected error: %v", chunk.Error)
			}
			if chunk.Done {
				doneReceived = true
				break
			}
			content.WriteString(chunk.Content)
		}

		if !doneReceived {
			t.Error("stream did not send Done signal")
		}
		if content.String() != "Hello world!" {
			t.Errorf("content = %q; want %q", content.String(), "Hello world!")
		}
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		provider := NewOpenAICompatibleProvider("test", "test-key", server.URL, "test-model")

		ctx := context.Background()
		stream := provider.StreamChatCompletion(ctx, []Message{{Role: "user", Content: "test"}}, nil)

		chunk := <-stream
		if chunk.Error == nil {
			t.Error("expected error for 500 response")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			// Slow response to test cancellation
			time.Sleep(2 * time.Second)
		}))
		defer server.Close()

		provider := NewOpenAICompatibleProvider("test", "test-key", server.URL, "test-model")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stream := provider.StreamChatCompletion(ctx, []Message{{Role: "user", Content: "test"}}, nil)

		// Should handle cancellation gracefully
		for chunk := range stream {
			if chunk.Error != nil {
				// Expected - context cancelled
				return
			}
		}
	})
}

func TestOpenAICompatibleProvider_SupportsFunctionCalling(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4o", true},
		{"gpt-4", true},
		{"claude-3-opus", true},
		{"llama-3.1-70b", false},
		{"mistral-7b", false},
		{"nemotron", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider := NewOpenAICompatibleProvider("test", "key", "http://test", tt.model)
			if got := provider.SupportsFunctionCalling(); got != tt.expected {
				t.Errorf("SupportsFunctionCalling(%q) = %v; want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestOpenAICompatibleProvider_Model(t *testing.T) {
	provider := NewOpenAICompatibleProvider("test", "key", "http://test", "gpt-4o")
	if got := provider.Model(); got != "gpt-4o" {
		t.Errorf("Model() = %q; want %q", got, "gpt-4o")
	}
}

func TestOpenAICompatibleProvider_Provider(t *testing.T) {
	provider := NewOpenAICompatibleProvider("openai", "key", "http://test", "gpt-4o")
	if got := provider.Provider(); got != "openai" {
		t.Errorf("Provider() = %q; want %q", got, "openai")
	}
}
