package batch

import (
	"fmt"
	"net/url"
	"strings"
)

// TemplateVars holds substitutions for batch variables.
type TemplateVars struct {
	URL    string
	Domain string
	Index  int
	Name   string
}

// ApplyTemplate replaces supported variables in a template string.
func ApplyTemplate(template string, vars TemplateVars) string {
	replacements := map[string]string{
		"{{url}}":    vars.URL,
		"{{domain}}": vars.Domain,
		"{{index}}":  fmt.Sprintf("%d", vars.Index),
		"{{name}}":   vars.Name,
	}
	result := template
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}

// ExtractDomain extracts the domain from a URL.
func ExtractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
