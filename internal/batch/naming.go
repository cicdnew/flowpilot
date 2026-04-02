package batch

import "flowpilot/internal/models"

// defaultNameTemplate returns the fallback naming template for batch tasks.
func defaultNameTemplate() string {
	return "Task {{index}} - {{domain}}"
}

// validateTemplate checks that only supported variables are used.
func validateTemplate(template string) bool {
	return models.ValidateBatchTemplate(template)
}

