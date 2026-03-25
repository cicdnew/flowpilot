package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flowpilot/internal/models"
)

func (a *App) ExportResultsJSON() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	tasks, err := a.db.ListTasksByStatus(a.ctx, models.TaskStatusCompleted)
	if err != nil {
		return "", fmt.Errorf("list completed tasks: %w", err)
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.json", time.Now().Unix()))
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks to JSON: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return exportPath, nil
}

func (a *App) ExportResultsCSV() (_ string, retErr error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	tasks, err := a.db.ListTasksByStatus(a.ctx, models.TaskStatusCompleted)
	if err != nil {
		return "", fmt.Errorf("list completed tasks: %w", err)
	}

	exportPath := filepath.Join(a.dataDir, fmt.Sprintf("export_%d.csv", time.Now().Unix()))
	file, err := os.OpenFile(exportPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create export file: %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && retErr == nil {
			retErr = fmt.Errorf("close export file: %w", cErr)
		}
	}()

	writer := csv.NewWriter(file)

	if err := writer.Write([]string{"ID", "Name", "URL", "Status", "Error", "Duration", "CreatedAt", "CompletedAt"}); err != nil {
		return "", fmt.Errorf("write CSV header: %w", err)
	}

	for _, t := range tasks {
		duration := ""
		if t.Result != nil {
			duration = t.Result.Duration.String()
		}
		completedAt := ""
		if t.CompletedAt != nil {
			completedAt = t.CompletedAt.Format(time.RFC3339)
		}
		if err := writer.Write([]string{
			t.ID, t.Name, t.URL, string(t.Status), t.Error,
			duration, t.CreatedAt.Format(time.RFC3339), completedAt,
		}); err != nil {
			return "", fmt.Errorf("write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("flush CSV writer: %w", err)
	}

	return exportPath, nil
}

func (a *App) ExportTaskLogs(taskID string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	if a.logExporter == nil {
		return "", fmt.Errorf("log exporter unavailable")
	}
	return a.logExporter.ExportTaskLogsZip(a.ctx, taskID)
}

func (a *App) ExportBatchLogs(batchID string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	if a.logExporter == nil {
		return "", fmt.Errorf("log exporter unavailable")
	}
	return a.logExporter.ExportBatchLogs(a.ctx, batchID)
}

func (a *App) ListWebSocketLogs(flowID string) ([]models.WebSocketLog, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if flowID == "" {
		return nil, fmt.Errorf("list websocket logs: flowID is required")
	}
	return a.db.ListWebSocketLogs(a.ctx, flowID)
}

func (a *App) ListTaskEvents(taskID string) ([]models.TaskLifecycleEvent, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if taskID == "" {
		return nil, fmt.Errorf("list task events: taskID is required")
	}
	return a.db.ListTaskEvents(a.ctx, taskID)
}
