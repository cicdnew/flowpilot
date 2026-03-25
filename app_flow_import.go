package main

import "flowpilot/internal/models"

type FlowImportResult struct {
	Tasks    []models.Task `json:"tasks"`
	Warnings []string      `json:"warnings"`
}

func (a *App) ImportFlowWithWarnings(exportPath string) (*FlowImportResult, error) {
	tasks, warnings, err := a.importFlowWithWarnings(exportPath)
	if err != nil {
		return nil, err
	}
	return &FlowImportResult{Tasks: tasks, Warnings: warnings}, nil
}
