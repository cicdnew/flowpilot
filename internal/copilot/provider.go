package copilot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const errCreateRequest = "create request: %w"

// NewProvider creates a new LLM provider based on the provider name.
func NewProvider(provider, apiKey, baseURL, modelName string) (LLMProvider, error) {
	switch strings.ToLower(provider) {
	case "openai", "openrouter", "gemini", "nvidia", "huggingface", "github", "github-models", "kilo":
		return NewOpenAICompatibleProvider(provider, apiKey, baseURL, modelName), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// OpenAICompatibleProvider implements LLMProvider for OpenAI-compatible APIs.
type OpenAICompatibleProvider struct {
	provider string
	apiKey   string
	baseURL  string
	model    string
	client   *http.Client
}

// NewOpenAICompatibleProvider creates a new OpenAI-compatible provider.
func NewOpenAICompatibleProvider(provider, apiKey, baseURL, modelName string) *OpenAICompatibleProvider {
	if baseURL == "" {
		switch provider {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "openrouter":
			baseURL = "https://openrouter.ai/api/v1"
		case "gemini":
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		case "nvidia":
			baseURL = "https://integrate.api.nvidia.com/v1"
		case "huggingface":
			baseURL = "https://api-inference.huggingface.co/v1"
		case "github", "github-models":
			baseURL = "https://models.inference.ai.azure.com"
		case "kilo":
			baseURL = "https://api.kilo.dev/v1"
		}
	}

	return &OpenAICompatibleProvider{
		provider: provider,
		apiKey:   apiKey,
		baseURL:  baseURL,
		model:    modelName,
		client:   &http.Client{},
	}
}

// ChatCompletion implements LLMProvider.
func (p *OpenAICompatibleProvider) ChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) (ChatResponse, error) {
	body := map[string]any{
		"model":    p.model,
		"messages": messages,
	}

	// Only include tools if the model supports it
	if len(tools) > 0 && p.SupportsFunctionCalling() {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return ChatResponse{}, fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // best-effort: used only for error message
		return ChatResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("no choices in response")
	}

	var toolCalls []ToolCall
	for _, tc := range result.Choices[0].Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return ChatResponse{
		Content:   result.Choices[0].Message.Content,
		ToolCalls: toolCalls,
	}, nil
}

// ListModels dynamically fetches available models from the provider API.
func (p *OpenAICompatibleProvider) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // best-effort: used only for error message
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result ModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var models []Model
	for _, m := range result.Data {
		models = append(models, Model{
			ID:           m.ID,
			Name:         m.ID,
			Provider:     p.provider,
			Capabilities: 0,
			MaxContext:   detectMaxContext(m.ID),
			Description:  fmt.Sprintf("Model from %s API", p.provider),
		})
	}

	return models, nil
}

// SupportsFunctionCalling returns true if the provider supports function calling.
func (p *OpenAICompatibleProvider) SupportsFunctionCalling() bool {
	modelLower := strings.ToLower(p.model)
	return !strings.Contains(modelLower, "llama") && 
	       !strings.Contains(modelLower, "mistral") &&
	       !strings.Contains(modelLower, "nemotron")
}

// Model returns the current model name.
func (p *OpenAICompatibleProvider) Model() string {
	return p.model
}

// Provider returns the provider name.
func (p *OpenAICompatibleProvider) Provider() string {
	return p.provider
}

// detectMaxContext guesses context window based on model name patterns.
func detectMaxContext(modelID string) int {
	modelLower := strings.ToLower(modelID)
	switch {
	case strings.Contains(modelLower, "gpt-4o"):
		return 128000
	case strings.Contains(modelLower, "claude-3-5-sonnet"):
		return 200000
	case strings.Contains(modelLower, "claude-3-opus"):
		return 200000
	case strings.Contains(modelLower, "gemini-1.5-pro"):
		return 1000000
	case strings.Contains(modelLower, "gemini-1.5-flash"):
		return 1000000
	case strings.Contains(modelLower, "llama-3.1"):
		return 131072
	case strings.Contains(modelLower, "nemotron"):
		return 131072
	default:
		return 128000
	}
}

// StreamChatCompletion sends a streaming chat completion request.
// Returns a channel that emits StreamChunk values until Done=true or Error.
// processStreamLine parses a single SSE line from the LLM stream and sends
// any resulting chunks to out. Returns (done=true, nil) when the stream is
// finished, (false, err) on a fatal parse error, or (false, nil) to continue.
func (p *OpenAICompatibleProvider) processStreamLine(line string, out chan<- StreamChunk) (bool, error) {
	if !strings.HasPrefix(line, "data: ") {
		return false, nil
	}
	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		out <- StreamChunk{Done: true}
		return true, nil
	}

	var streamResp struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string         `json:"name"`
						Arguments map[string]any `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		return false, nil // skip malformed chunks
	}
	if len(streamResp.Choices) == 0 {
		return false, nil
	}

	choice := streamResp.Choices[0]
	if choice.Delta.Content != "" {
		out <- StreamChunk{Content: choice.Delta.Content}
	}
	if len(choice.Delta.ToolCalls) > 0 {
		tc := choice.Delta.ToolCalls[0]
		out <- StreamChunk{ToolCall: &ToolCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments}}
	}
	if choice.FinishReason != "" {
		out <- StreamChunk{Done: true}
		return true, nil
	}
	return false, nil
}

func (p *OpenAICompatibleProvider) StreamChatCompletion(ctx context.Context, messages []Message, tools []ToolDefinition) <-chan StreamChunk {
	out := make(chan StreamChunk, 64)

	go func() {
		defer close(out)

		body := map[string]any{
			"model":    p.model,
			"messages": messages,
			"stream":   true,
		}

		if len(tools) > 0 && p.SupportsFunctionCalling() {
			body["tools"] = tools
			body["tool_choice"] = "auto"
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			out <- StreamChunk{Error: fmt.Errorf("marshal request: %w", err)}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			out <- StreamChunk{Error: fmt.Errorf(errCreateRequest, err)}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := p.client.Do(req)
		if err != nil {
			out <- StreamChunk{Error: fmt.Errorf("send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			out <- StreamChunk{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			done, err := p.processStreamLine(scanner.Text(), out)
			if err != nil {
				out <- StreamChunk{Error: err}
				return
			}
			if done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			out <- StreamChunk{Error: fmt.Errorf("scanner error: %w", err)}
		}
	}()

	return out
}
