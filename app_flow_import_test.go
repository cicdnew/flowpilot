package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flowpilot/internal/models"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestImportFlowWithWarningsReturnsUnknownActionWarnings(t *testing.T) {
	app := setupTestApp(t)
	export := models.FlowExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		FlowName:   "Imported",
		Tasks: []models.Task{{
			Name:      "Imported Task",
			URL:       "http://chhotu-bin.infy.uk",
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
				URL:       "http://chhotu-bin.infy.uk/a",
				Steps:     []models.TaskStep{{Action: models.StepAction("legacy_a")}, {Action: models.ActionNavigate, Value: "http://chhotu-bin.infy.uk/a"}},
				CreatedAt: time.Now(),
			},
			{
				Name:      "Task B",
				URL:       "http://chhotu-bin.infy.uk/b",
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

func stubOpenFileDialog(t *testing.T, fn func(context.Context, wailsRuntime.OpenDialogOptions) (string, error)) {
	t.Helper()
	original := openFileDialog
	openFileDialog = fn
	t.Cleanup(func() { openFileDialog = original })
}

func TestImportFlowFromDialogImportsSelectedFile(t *testing.T) {
	app := setupTestApp(t)
	export := models.FlowExport{
		Version:    "1.0",
		ExportedAt: time.Now(),
		FlowName:   "Dialog",
		Tasks: []models.Task{{
			Name:      "Dialog Task",
			URL:       "http://chhotu-bin.infy.uk/dialog",
			Steps:     []models.TaskStep{{Action: models.ActionNavigate, Value: "http://chhotu-bin.infy.uk/dialog"}},
			CreatedAt: time.Now(),
		}},
	}
	path := filepath.Join(t.TempDir(), "dialog.json")
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write export: %v", err)
	}
	var gotOptions wailsRuntime.OpenDialogOptions
	stubOpenFileDialog(t, func(_ context.Context, options wailsRuntime.OpenDialogOptions) (string, error) {
		gotOptions = options
		return path, nil
	})

	result, err := app.ImportFlowFromDialog()
	if err != nil {
		t.Fatalf("ImportFlowFromDialog: %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Name != "Dialog Task" {
		t.Fatalf("tasks = %+v, want one task named Dialog Task", result.Tasks)
	}
	if len(gotOptions.Filters) != 1 || gotOptions.Filters[0].Pattern != "*.json" {
		t.Fatalf("filters = %+v, want *.json filter", gotOptions.Filters)
	}
}

func TestImportFlowFromDialogCancelledReturnsEmptyResult(t *testing.T) {
	app := setupTestApp(t)
	stubOpenFileDialog(t, func(context.Context, wailsRuntime.OpenDialogOptions) (string, error) {
		return "", nil
	})

	result, err := app.ImportFlowFromDialog()
	if err != nil {
		t.Fatalf("ImportFlowFromDialog: %v", err)
	}
	if result == nil || len(result.Tasks) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("result = %+v, want empty result", result)
	}
}

func TestImportFlowFromDialogPropagatesDialogError(t *testing.T) {
	app := setupTestApp(t)
	dialogErr := errors.New("dialog unavailable")
	stubOpenFileDialog(t, func(context.Context, wailsRuntime.OpenDialogOptions) (string, error) {
		return "", dialogErr
	})

	if _, err := app.ImportFlowFromDialog(); !errors.Is(err, dialogErr) {
		t.Fatalf("err = %v, want %v", err, dialogErr)
	}
}
