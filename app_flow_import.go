package main

import (
	"fmt"

	"flowpilot/internal/models"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var openFileDialog = wailsRuntime.OpenFileDialog

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

func (a *App) ImportFlowFromDialog() (*FlowImportResult, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	path, err := openFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title:   "Import Flow",
		Filters: []wailsRuntime.FileFilter{{DisplayName: "Flow exports (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return nil, fmt.Errorf("open file dialog: %w", err)
	}
	if path == "" {
		return &FlowImportResult{Tasks: []models.Task{}, Warnings: []string{}}, nil
	}
	return a.ImportFlowWithWarnings(path)
}
