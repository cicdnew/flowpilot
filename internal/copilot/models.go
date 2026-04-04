package copilot

import "strings"

// ModelCapability represents LLM capabilities.
type ModelCapability int

const (
	CapabilityToolCalling ModelCapability = 1 << iota
	CapabilityLongContext
	CapabilityReasoning
	CapabilityCoding
	CapabilityVision
	CapabilityFast
)

// Model represents an LLM model with its capabilities.
type Model struct {
	ID           string
	Name         string
	Provider     string
	Capabilities ModelCapability
	MaxContext   int
	Description  string
}

// ModelCatalog contains all supported models.
var ModelCatalog = []Model{
	// OpenAI
	{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Capabilities: CapabilityToolCalling | CapabilityLongContext | CapabilityReasoning | CapabilityCoding | CapabilityVision | CapabilityFast, MaxContext: 128000, Description: "Best general purpose model"},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", Capabilities: CapabilityToolCalling | CapabilityFast, MaxContext: 128000, Description: "Fast and cheap for simple tasks"},

	// OpenRouter
	{ID: "anthropic/claude-3-5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", Capabilities: CapabilityToolCalling | CapabilityLongContext | CapabilityReasoning | CapabilityCoding | CapabilityFast, MaxContext: 200000, Description: "Best balance of quality and speed"},
	{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus", Provider: "openrouter", Capabilities: CapabilityToolCalling | CapabilityLongContext | CapabilityReasoning, MaxContext: 200000, Description: "Highest quality reasoning"},

	// Gemini
	{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Provider: "gemini", Capabilities: CapabilityToolCalling | CapabilityLongContext | CapabilityVision, MaxContext: 1000000, Description: "Longest context window"},
	{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Provider: "gemini", Capabilities: CapabilityToolCalling | CapabilityFast, MaxContext: 1000000, Description: "Extremely fast, low cost"},

	// NVIDIA
	{ID: "nvidia/llama-3.1-nemotron-70b-instruct", Name: "Nemotron 70B", Provider: "nvidia", Capabilities: CapabilityCoding, MaxContext: 131072, Description: "Open source coding model"},

	// Hugging Face
	{ID: "meta-llama/Meta-Llama-3.1-70B-Instruct", Name: "Llama 3.1 70B", Provider: "huggingface", Capabilities: CapabilityCoding, MaxContext: 131072, Description: "Best open source general purpose"},

	// GitHub Models
	{ID: "gpt-4o", Name: "GPT-4o", Provider: "github", Capabilities: CapabilityToolCalling | CapabilityReasoning, MaxContext: 128000, Description: "Azure-hosted GPT-4o"},

	// Kilo Gateway
	{ID: "claude-3-5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "kilo", Capabilities: CapabilityToolCalling | CapabilityReasoning, MaxContext: 200000, Description: "Kilo gateway Claude"},
}

// GetModel returns a model by ID.
func GetModel(modelID string) (Model, bool) {
	for _, m := range ModelCatalog {
		if strings.EqualFold(m.ID, modelID) || strings.EqualFold(m.Name, modelID) {
			return m, true
		}
	}
	return Model{}, false
}

// GetModelsForProvider returns all models for a provider.
func GetModelsForProvider(provider string) []Model {
	var models []Model
	for _, m := range ModelCatalog {
		if strings.EqualFold(m.Provider, provider) {
			models = append(models, m)
		}
	}
	return models
}

// GetRecommendedModel returns the best model for a given task type.
func GetRecommendedModel(provider string, taskType string) string {
	switch taskType {
	case "reasoning", "planning":
		switch provider {
		case "openrouter":
			return "anthropic/claude-3-5-sonnet"
		case "openai", "github":
			return "gpt-4o"
		case "gemini":
			return "gemini-1.5-pro"
		default:
			return "gpt-4o"
		}
	case "fast", "simple":
		switch provider {
		case "openai":
			return "gpt-4o-mini"
		case "gemini":
			return "gemini-1.5-flash"
		default:
			return "anthropic/claude-3-5-sonnet"
		}
	case "coding":
		switch provider {
		case "nvidia":
			return "nvidia/llama-3.1-nemotron-70b-instruct"
		case "huggingface":
			return "meta-llama/Meta-Llama-3.1-70B-Instruct"
		default:
			return "anthropic/claude-3-5-sonnet"
		}
	default:
		switch provider {
		case "openrouter", "kilo":
			return "anthropic/claude-3-5-sonnet"
		case "openai", "github":
			return "gpt-4o"
		case "gemini":
			return "gemini-1.5-pro"
		case "nvidia":
			return "nvidia/llama-3.1-nemotron-70b-instruct"
		case "huggingface":
			return "meta-llama/Meta-Llama-3.1-70B-Instruct"
		default:
			return "gpt-4o"
		}
	}
}

// SupportsToolCalling returns true if the model supports function calling.
func SupportsToolCalling(modelID string) bool {
	model, ok := GetModel(modelID)
	if !ok {
		// Assume popular models support tool calling
		return !strings.Contains(strings.ToLower(modelID), "llama") && 
		       !strings.Contains(strings.ToLower(modelID), "mistral") &&
		       !strings.Contains(strings.ToLower(modelID), "nemotron")
	}
	return model.Capabilities&CapabilityToolCalling != 0
}
