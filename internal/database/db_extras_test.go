package database

import (
	"context"
	"testing"
	"time"

	"flowpilot/internal/models"
)

// TestCreateAndListProxyRoutingPresets covers the 0% db_proxy_presets functions.
func TestCreateAndListProxyRoutingPresets(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	preset := models.ProxyRoutingPreset{
		ID:              "preset-1",
		Name:            "US Preset",
		RandomByCountry: true,
		Country:         "US",
		Fallback:        models.ProxyFallbackAny,
		CreatedAt:       time.Now().Truncate(time.Second),
	}

	if err := db.CreateProxyRoutingPreset(ctx, preset); err != nil {
		t.Fatalf("CreateProxyRoutingPreset: %v", err)
	}

	presets, err := db.ListProxyRoutingPresets(ctx)
	if err != nil {
		t.Fatalf("ListProxyRoutingPresets: %v", err)
	}
	if len(presets) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(presets))
	}
	if presets[0].ID != preset.ID {
		t.Errorf("expected preset ID %q, got %q", preset.ID, presets[0].ID)
	}
	if !presets[0].RandomByCountry {
		t.Error("expected RandomByCountry=true")
	}
}

// TestDeleteProxyRoutingPreset covers DeleteProxyRoutingPreset.
func TestDeleteProxyRoutingPreset(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	preset := models.ProxyRoutingPreset{
		ID:        "preset-del",
		Name:      "To Delete",
		CreatedAt: time.Now().Truncate(time.Second),
	}
	if err := db.CreateProxyRoutingPreset(ctx, preset); err != nil {
		t.Fatalf("CreateProxyRoutingPreset: %v", err)
	}

	if err := db.DeleteProxyRoutingPreset(ctx, preset.ID); err != nil {
		t.Fatalf("DeleteProxyRoutingPreset: %v", err)
	}

	presets, err := db.ListProxyRoutingPresets(ctx)
	if err != nil {
		t.Fatalf("ListProxyRoutingPresets after delete: %v", err)
	}
	if len(presets) != 0 {
		t.Errorf("expected 0 presets after delete, got %d", len(presets))
	}
}

// TestDeleteProxyRoutingPresetNotFound covers the not-found error path.
func TestDeleteProxyRoutingPresetNotFound(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	err := db.DeleteProxyRoutingPreset(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent preset")
	}
}

// TestBoolToInt covers the boolToInt helper.
func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("expected boolToInt(true) == 1")
	}
	if boolToInt(false) != 0 {
		t.Error("expected boolToInt(false) == 0")
	}
}

// TestListStaleTasks covers the ListStaleTasks function.
func TestListStaleTasks(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create tasks in various states
	pending := makeTask("stale-pending", "Pending Task")
	pending.Status = models.TaskStatusPending
	if err := db.CreateTask(ctx, pending); err != nil {
		t.Fatalf("CreateTask pending: %v", err)
	}

	running := makeTask("stale-running", "Running Task")
	running.Status = models.TaskStatusPending
	if err := db.CreateTask(ctx, running); err != nil {
		t.Fatalf("CreateTask running: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, running.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus running: %v", err)
	}

	stale, err := db.ListStaleTasks(ctx)
	if err != nil {
		t.Fatalf("ListStaleTasks: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale task, got %d", len(stale))
	}
	if stale[0].ID != running.ID {
		t.Errorf("expected stale task %q, got %q", running.ID, stale[0].ID)
	}
}

// TestBatchApplyTaskStateChangesEmpty covers empty input no-op.
func TestBatchApplyTaskStateChangesEmpty(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if err := db.BatchApplyTaskStateChanges(ctx, []TaskStateChange{}); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges empty: %v", err)
	}
}

// TestBatchApplyTaskStateChanges covers bulk status updates.
func TestBatchApplyTaskStateChanges(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task1 := makeTask("batch-t1", "Batch Task 1")
	task2 := makeTask("batch-t2", "Batch Task 2")
	for _, task := range []models.Task{task1, task2} {
		if err := db.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask %s: %v", task.ID, err)
		}
	}

	changes := []TaskStateChange{
		{TaskID: task1.ID, Status: models.TaskStatusRunning},
		{TaskID: task2.ID, Status: models.TaskStatusRunning},
	}
	if err := db.BatchApplyTaskStateChanges(ctx, changes); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges: %v", err)
	}

	tasks, err := db.ListTasksByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		t.Fatalf("ListTasksByStatus: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 running tasks, got %d", len(tasks))
	}
}

// TestBatchUpdateTaskStatus covers the wrapper BatchUpdateTaskStatus.
func TestBatchUpdateTaskStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("bu-t1", "Batch Update Task")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := db.BatchUpdateTaskStatus(ctx, []string{}, models.TaskStatusCancelled, ""); err != nil {
		t.Fatalf("BatchUpdateTaskStatus empty: %v", err)
	}

	if err := db.BatchUpdateTaskStatus(ctx, []string{task.ID}, models.TaskStatusCancelled, "cancelled"); err != nil {
		t.Fatalf("BatchUpdateTaskStatus: %v", err)
	}

	tasks, err := db.ListTasksByStatus(ctx, models.TaskStatusCancelled)
	if err != nil {
		t.Fatalf("ListTasksByStatus: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 cancelled task, got %d", len(tasks))
	}
}

// TestUpdateCaptchaConfig covers UpdateCaptchaConfig (0% coverage).
func TestUpdateCaptchaConfig(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	cfg := models.CaptchaConfig{
		ID:        "cap-1",
		Provider:  "2captcha",
		APIKey:    "original-key",
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Second),
	}
	if err := db.CreateCaptchaConfig(ctx, cfg); err != nil {
		t.Fatalf("CreateCaptchaConfig: %v", err)
	}

	cfg.APIKey = "updated-key"
	cfg.Enabled = false
	if err := db.UpdateCaptchaConfig(ctx, cfg); err != nil {
		t.Fatalf("UpdateCaptchaConfig: %v", err)
	}

	got, err := db.GetCaptchaConfig(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("GetCaptchaConfig: %v", err)
	}
	if got.APIKey != "updated-key" {
		t.Errorf("expected APIKey 'updated-key', got %q", got.APIKey)
	}
	if got.Enabled {
		t.Error("expected Enabled=false after update")
	}
}

// TestListBatchGroups covers ListBatchGroups (0% coverage).
func TestListBatchGroups(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	groups, err := db.ListBatchGroups(ctx)
	if err != nil {
		t.Fatalf("ListBatchGroups: %v", err)
	}
	if groups == nil {
		groups = []models.BatchGroup{}
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 batch groups, got %d", len(groups))
	}
}

// TestDeleteVisualDiff covers DeleteVisualDiff (0% coverage).
func TestDeleteVisualDiff(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	diff := models.VisualDiff{
		ID:        "vd-1",
		TaskID:    "task-x",
		CreatedAt: time.Now().Truncate(time.Second),
	}
	if err := db.CreateVisualDiff(ctx, diff); err != nil {
		t.Fatalf("CreateVisualDiff: %v", err)
	}

	if err := db.DeleteVisualDiff(ctx, diff.ID); err != nil {
		t.Fatalf("DeleteVisualDiff: %v", err)
	}

	diffs, err := db.ListVisualDiffs(ctx, diff.BaselineID)
	if err != nil {
		t.Fatalf("ListVisualDiffs: %v", err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs after delete, got %d", len(diffs))
	}
}

// TestBeginTxAndCreateTaskTx covers BeginTx + CreateTaskTx (0% coverage).
func TestBeginTxAndCreateTaskTx(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}

	task := makeTask("tx-task-1", "Tx Task")
	if err := db.CreateTaskTx(ctx, tx, task); err != nil {
		tx.Rollback()
		t.Fatalf("CreateTaskTx: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	tasks, err := db.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	found := false
	for _, tk := range tasks {
		if tk.ID == task.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task created in tx to be present")
	}
}

// TestBatchGroupTx covers CreateBatchGroupTx.
func TestBatchGroupTx(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}

	group := models.BatchGroup{
		ID:        "bg-tx-1",
		FlowID:    "flow-1",
		Name:      "TX Batch Group",
		Total:     2,
		CreatedAt: time.Now().Truncate(time.Second).Format(time.RFC3339),
	}
	if err := db.CreateBatchGroupTx(ctx, tx, group); err != nil {
		tx.Rollback()
		t.Fatalf("CreateBatchGroupTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	groups, err := db.ListBatchGroups(ctx)
	if err != nil {
		t.Fatalf("ListBatchGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 batch group, got %d", len(groups))
	}
	if groups[0].ID != group.ID {
		t.Errorf("expected group ID %q, got %q", group.ID, groups[0].ID)
	}
}
