package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestImportFlowWithWarningsReturnsUnknownActionWarnings(t *testing.T) {
	app := setupTestApp(t)
	export := models.FlowExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		FlowName:   "Imported",
		Tasks: []models.Task{{
			Name:      "Imported Task",
			URL:       "https://example.com",
			Steps:     []models.TaskStep{{Action: models.StepAction("legacy_action")}},
			CreatedAt: time.Now(),
		}},
	}
	path := filepath.Join(t.TempDir(), "flow.json")
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write export: %v", err)
	}

	result, err := app.ImportFlowWithWarnings(path)
	if err != nil {
		t.Fatalf("ImportFlowWithWarnings: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("tasks len = %d, want 1", len(result.Tasks))
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1", len(result.Warnings))
	}
}

func TestImportFlowWithWarningsCollectsMultipleWarnings(t *testing.T) {
	app := setupTestApp(t)
	export := models.FlowExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		FlowName:   "Imported",
		Tasks: []models.Task{
			{
				Name:      "Task A",
				URL:       "https://example.com/a",
				Steps:     []models.TaskStep{{Action: models.StepAction("legacy_a")}, {Action: models.ActionNavigate, Value: "https://example.com/a"}},
				CreatedAt: time.Now(),
			},
			{
				Name:      "Task B",
				URL:       "https://example.com/b",
				Steps:     []models.TaskStep{{Action: models.StepAction("legacy_b")}},
				CreatedAt: time.Now(),
			},
		},
	}
	path := filepath.Join(t.TempDir(), "flow-multi.json")
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write export: %v", err)
	}

	result, err := app.ImportFlowWithWarnings(path)
	if err != nil {
		t.Fatalf("ImportFlowWithWarnings: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("tasks len = %d, want 2", len(result.Tasks))
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings len = %d, want 2", len(result.Warnings))
	}
}
