package copilot

// Model represents an LLM model with its capabilities.
type Model struct {
	ID           string
	Name         string
	Provider     string
	Capabilities ModelCapability
	MaxContext   int
	Description  string
}

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

// ModelListResponse is the standard OpenAI-compatible models response.
type ModelListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}
