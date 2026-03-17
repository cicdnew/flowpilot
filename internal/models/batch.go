package models

import "strings"

// MaxBatchSize is the maximum number of URLs allowed in a single batch.
const MaxBatchSize = 10000

// AdvancedBatchInput holds the configuration for creating batch tasks
// from a recorded flow with shared steps.
type AdvancedBatchInput struct {
	FlowID         string      `json:"flowId"`
	URLs           []string    `json:"urls"`
	NamingTemplate string      `json:"namingTemplate"` // e.g. "{{index}} - {{domain}}"
	Priority       int         `json:"priority"`
	Proxy          ProxyConfig `json:"proxy"`
	Tags           []string    `json:"tags,omitempty"`
	ProxyCountry   string      `json:"proxyCountry,omitempty"`
	ProxyFallback  string      `json:"proxyFallback,omitempty"`
	AutoStart      bool        `json:"autoStart"`
	Headless       *bool       `json:"headless,omitempty"` // nil defaults to true for backwards compatibility
}

// BatchHeadless returns the effective headless setting.
// Defaults to true when Headless is nil (backwards compatible).
func (i AdvancedBatchInput) BatchHeadless() bool {
	if i.Headless == nil {
		return true
	}
	return *i.Headless
}

// BatchGroup tracks a group of tasks created together from one batch operation.
type BatchGroup struct {
	ID        string   `json:"id"`
	FlowID    string   `json:"flowId"`
	TaskIDs   []string `json:"taskIds"`
	Total     int      `json:"total"`
	Name      string   `json:"name"`
	CreatedAt string   `json:"createdAt"`
}

// BatchProgress reports aggregate execution status for a batch group.
type BatchProgress struct {
	BatchID   string `json:"batchId"`
	Total     int    `json:"total"`
	Pending   int    `json:"pending"`
	Queued    int    `json:"queued"`
	Running   int    `json:"running"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Cancelled int    `json:"cancelled"`
}

// TemplateVariable defines a supported substitution variable for batch naming
// and step value templates.
type TemplateVariable struct {
	Name        string `json:"name"`        // e.g. "url"
	Placeholder string `json:"placeholder"` // e.g. "{{url}}"
	Description string `json:"description"`
}

// SupportedVariables returns all template variables available for substitution.
func SupportedVariables() []TemplateVariable {
	return []TemplateVariable{
		{Name: "url", Placeholder: "{{url}}", Description: "Full URL of the task"},
		{Name: "domain", Placeholder: "{{domain}}", Description: "Domain extracted from URL"},
		{Name: "index", Placeholder: "{{index}}", Description: "1-based index in the batch"},
		{Name: "name", Placeholder: "{{name}}", Description: "Generated task name"},
	}
}

// ValidateBatchTemplate checks that only supported variables are used in a template string.
func ValidateBatchTemplate(template string) bool {
	allowed := []string{"{{url}}", "{{domain}}", "{{index}}", "{{name}}"}
	for strings.Contains(template, "{{") {
		start := strings.Index(template, "{{")
		end := strings.Index(template[start+2:], "}}")
		if end == -1 {
			return false
		}
		end = start + 2 + end
		expr := template[start : end+2]
		valid := false
		for _, a := range allowed {
			if expr == a {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
		template = template[end+2:]
	}
	return true
}
