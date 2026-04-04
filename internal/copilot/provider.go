package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// NewProvider creates a new LLM provider based on the provider name.
func NewProvider(provider string, apiKey string, baseURL string, modelName string) (LLMProvider, error) {
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
func NewOpenAICompatibleProvider(provider string, apiKey string, baseURL string, modelName string) *OpenAICompatibleProvider {
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

	if modelName == "" {
		modelName = GetRecommendedModel(provider, "general")
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
		return ChatResponse{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
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

// SupportsFunctionCalling returns true if the provider supports function calling.
func (p *OpenAICompatibleProvider) SupportsFunctionCalling() bool {
	return SupportsToolCalling(p.model)
}

// Model returns the current model name.
func (p *OpenAICompatibleProvider) Model() string {
	return p.model
}

// Provider returns the provider name.
func (p *OpenAICompatibleProvider) Provider() string {
	return p.provider
}
