package main

import "flowpilot/internal/models"

func (a *App) GetSupportedStepActions() []string {
	actions := models.SupportedStepActions()
	result := make([]string, len(actions))
	for i, action := range actions {
		result[i] = string(action)
	}
	return result
}
