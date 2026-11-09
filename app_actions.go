package main

import "flowpilot/internal/models"

func (a *App) GetSupportedStepActions() []string {
	actions := models.SupportedStepActions()
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, string(action))
	}
	for _, info := range a.pluginRegistry().List() {
		for _, spec := range info.Actions {
			result = append(result, string(spec.Name))
		}
	}
	return result
}
