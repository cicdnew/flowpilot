package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"

	"github.com/google/uuid"
)

const flowExportVersion = "1.0"

// validateStepActions checks that all step actions are known/supported
func validateStepActions(steps []models.TaskStep) error {
	for i, step := range steps {
		if !models.IsKnownAction(step.Action) {
			return fmt.Errorf("step %d: unsupported action type %q", i+1, step.Action)
		}
	}
	return nil
}

// collectUnknownStepActionWarnings logs and returns warnings for any steps with unknown actions.
// It does not block import — unknown actions are warnings only.
func collectUnknownStepActionWarnings(steps []models.TaskStep, taskIndex int) []string {
	warnings := make([]string, 0)
	for i, step := range steps {
		if !models.IsKnownAction(step.Action) {
			warning := fmt.Sprintf("task %d step %d: unknown action type %q (imported anyway)", taskIndex, i+1, step.Action)
			logs.Logger.Warn("import flow warning", "warning", warning, "task_index", taskIndex, "step_index", i+1)
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (a *App) ExportTask(taskID string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	task, err := a.db.GetTask(a.ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	export := models.TaskExport{
		Version:    flowExportVersion,
		ExportedAt: time.Now(),
		Name:       task.Name,
		Task:       *task,
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("task_%s_%d.json", browser.SanitizeFilename(task.Name), time.Now().Unix()))
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal task to JSON: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return exportPath, nil
}

func (a *App) ImportTask(exportPath string) (*models.Task, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		return nil, fmt.Errorf("read export file: %w", err)
	}

	var export models.TaskExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("unmarshal task export: %w", err)
	}

	if export.Version == "" {
		return nil, fmt.Errorf("invalid export file: missing version")
	}

	task := export.Task

	// Validate step actions
	if err := validateStepActions(task.Steps); err != nil {
		return nil, fmt.Errorf("invalid task steps: %w", err)
	}

	task.ID = uuid.New().String()
	task.Status = models.TaskStatusPending
	task.CreatedAt = time.Now()
	task.StartedAt = nil
	task.CompletedAt = nil
	task.Result = nil
	task.Error = ""

	if err := a.db.CreateTask(a.ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return &task, nil
}

func (a *App) ExportFlow(flowID string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	flow, err := a.db.GetRecordedFlow(a.ctx, flowID)
	if err != nil {
		return "", fmt.Errorf("get flow: %w", err)
	}
	if flow == nil {
		return "", fmt.Errorf("flow not found: %s", flowID)
	}

	export := models.FlowExport{
		Version:       flowExportVersion,
		ExportedAt:    time.Now(),
		FlowName:      flow.Name,
		RecordedSteps: flow.Steps,
		Tasks: []models.Task{
			{
				Name:      flow.Name,
				URL:       flow.OriginURL,
				Steps:     models.FlowToTaskSteps(*flow),
				CreatedAt: flow.CreatedAt,
			},
		},
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("flow_%s_%d.json", browser.SanitizeFilename(flow.Name), time.Now().Unix()))
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal flow to JSON: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return exportPath, nil
}

func (a *App) ImportFlow(exportPath string) ([]models.Task, error) {
	tasks, _, err := a.importFlowWithWarnings(exportPath)
	return tasks, err
}

func (a *App) importFlowWithWarnings(exportPath string) ([]models.Task, []string, error) {
	if err := a.ready(); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read export file: %w", err)
	}

	var export models.FlowExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, nil, fmt.Errorf("unmarshal flow export: %w", err)
	}

	if export.Version == "" {
		return nil, nil, fmt.Errorf("invalid export file: missing version")
	}

	warnings := make([]string, 0)
	now := time.Now()
	for i := range export.Tasks {
		warnings = append(warnings, collectUnknownStepActionWarnings(export.Tasks[i].Steps, i+1)...)
		export.Tasks[i].ID = uuid.New().String()
		export.Tasks[i].Status = models.TaskStatusPending
		export.Tasks[i].CreatedAt = now
		export.Tasks[i].StartedAt = nil
		export.Tasks[i].CompletedAt = nil
		export.Tasks[i].Result = nil
		export.Tasks[i].Error = ""
	}

	created := make([]models.Task, 0, len(export.Tasks))
	for i := range export.Tasks {
		task := export.Tasks[i]
		if err := a.db.CreateTask(a.ctx, task); err != nil {
			return nil, nil, fmt.Errorf("create task %d: %w", i, err)
		}
		created = append(created, task)
	}
	return created, warnings, nil
}
