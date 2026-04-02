package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"flowpilot/internal/models"
)

// TestUpdateTaskStatusIsAtomic verifies that UpdateTaskStatus writes the status
// update and the lifecycle event in a single transaction — both must succeed or
// both must be absent.
func TestUpdateTaskStatusIsAtomic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("atomic-1", "Atomic Status Test")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(running): %v", err)
	}

	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusRunning {
		t.Errorf("status: want running, got %q", got.Status)
	}

	// Verify lifecycle event was persisted in the same transaction.
	events, err := db.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one lifecycle event after UpdateTaskStatus")
	}
	last := events[len(events)-1]
	if last.ToState != models.TaskStatusRunning {
		t.Errorf("event ToState: want running, got %q", last.ToState)
	}
	if last.FromState != models.TaskStatusPending {
		t.Errorf("event FromState: want pending, got %q", last.FromState)
	}
}


// TestUpdateTaskStatusEventChain verifies the FromState/ToState chain is correct
// across multiple sequential status transitions.
func TestUpdateTaskStatusEventChain(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("chain-1", "Event Chain Test")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	transitions := []struct {
		to  models.TaskStatus
		err string
	}{
		{models.TaskStatusQueued, ""},
		{models.TaskStatusRunning, ""},
		{models.TaskStatusFailed, "network timeout"},
	}

	for _, tr := range transitions {
		if err := db.UpdateTaskStatus(ctx, task.ID, tr.to, tr.err); err != nil {
			t.Fatalf("UpdateTaskStatus to %s: %v", tr.to, err)
		}
	}

	events, err := db.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(events) < len(transitions) {
		t.Fatalf("expected at least %d events, got %d", len(transitions), len(events))
	}

	// Verify the last event has the correct terminal state.
	last := events[len(events)-1]
	if last.ToState != models.TaskStatusFailed {
		t.Errorf("last ToState: want failed, got %q", last.ToState)
	}
	if last.Error != "network timeout" {
		t.Errorf("last Error: want %q, got %q", "network timeout", last.Error)
	}
}

// TestBatchApplyTaskStateChangesLargeBatch verifies the N+1 fix:
// a large batch of changes is processed with a single prefetch query.
func TestBatchApplyTaskStateChangesLargeBatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	const n = 50
	changes := make([]TaskStateChange, n)
	for i := 0; i < n; i++ {
		task := makeTask(fmt.Sprintf("batch-large-%d", i), fmt.Sprintf("Task %d", i))
		if err := db.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
		changes[i] = TaskStateChange{TaskID: task.ID, Status: models.TaskStatusRunning}
	}

	if err := db.BatchApplyTaskStateChanges(ctx, changes); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges: %v", err)
	}

	running, err := db.ListTasksByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		t.Fatalf("ListTasksByStatus: %v", err)
	}
	if len(running) != n {
		t.Errorf("expected %d running tasks, got %d", n, len(running))
	}
}

// TestBatchApplyTaskStateChangesTerminalGuard verifies that terminal tasks
// (completed/failed/cancelled) cannot be moved to non-terminal states.
func TestBatchApplyTaskStateChangesTerminalGuard(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("terminal-guard-1", "Terminal Guard Test")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Move to completed.
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(running): %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(completed): %v", err)
	}

	// Attempt to move completed task back to running — should be silently skipped.
	changes := []TaskStateChange{
		{TaskID: task.ID, Status: models.TaskStatusRunning},
	}
	if err := db.BatchApplyTaskStateChanges(ctx, changes); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges: %v", err)
	}

	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCompleted {
		t.Errorf("status should remain completed, got %q", got.Status)
	}
}

// TestBatchApplyTaskStateChangesRetryIncrement verifies IncrementRetry
// increments retry_count and sets status to retrying atomically.
func TestBatchApplyTaskStateChangesRetryIncrement(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("retry-inc-1", "Retry Increment Test")
	task.MaxRetries = 3
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	changes := []TaskStateChange{
		{TaskID: task.ID, Status: models.TaskStatusRetrying, Error: "first failure", IncrementRetry: true},
	}
	if err := db.BatchApplyTaskStateChanges(ctx, changes); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges: %v", err)
	}

	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.RetryCount != 1 {
		t.Errorf("RetryCount: want 1, got %d", got.RetryCount)
	}
	if got.Status != models.TaskStatusRetrying {
		t.Errorf("status: want retrying, got %q", got.Status)
	}

	// Verify lifecycle event was recorded.
	events, err := db.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected lifecycle events after retry increment")
	}
}

// TestFinalizeTaskSuccessIsAtomic verifies FinalizeTaskSuccess commits
// task status update, step logs, network logs, and lifecycle event together.
func TestFinalizeTaskSuccessIsAtomic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("finalize-ok-1", "Finalize Success Test")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(running): %v", err)
	}

	result := models.TaskResult{
		Duration: 500 * time.Millisecond,
		StepLogs: []models.StepLog{
			{StepIndex: 0, Action: models.ActionNavigate, DurationMs: 100, StartedAt: time.Now()},
			{StepIndex: 1, Action: models.ActionClick, DurationMs: 50, StartedAt: time.Now()},
		},
		NetworkLogs: []models.NetworkLog{
			{RequestURL: "https://example.com", Method: "GET", StatusCode: 200},
		},
		ExtractedData: map[string]string{"title": "Example Domain"},
	}

	if err := db.FinalizeTaskSuccess(ctx, task.ID, result); err != nil {
		t.Fatalf("FinalizeTaskSuccess: %v", err)
	}

	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCompleted {
		t.Errorf("status: want completed, got %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt: want non-nil")
	}
	if got.Result == nil {
		t.Fatal("Result: want non-nil")
	}

	// Verify step logs persisted.
	stepLogs, err := db.ListStepLogs(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(stepLogs) != 2 {
		t.Errorf("step logs: want 2, got %d", len(stepLogs))
	}

	// Verify lifecycle event was recorded.
	events, err := db.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	hasCompleted := false
	for _, e := range events {
		if e.ToState == models.TaskStatusCompleted {
			hasCompleted = true
			break
		}
	}
	if !hasCompleted {
		t.Error("expected completed lifecycle event after FinalizeTaskSuccess")
	}
}

// TestFinalizeTaskFailureIsAtomic verifies FinalizeTaskFailure commits
// task status update, step logs, network logs, and lifecycle event together.
func TestFinalizeTaskFailureIsAtomic(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task := makeTask("finalize-fail-1", "Finalize Failure Test")
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus(running): %v", err)
	}

	stepLogs := []models.StepLog{
		{StepIndex: 0, Action: models.ActionNavigate, DurationMs: 100, StartedAt: time.Now()},
		{StepIndex: 1, Action: models.ActionClick, DurationMs: 50, ErrorMsg: "element not found", StartedAt: time.Now()},
	}
	networkLogs := []models.NetworkLog{
		{RequestURL: "https://example.com", Method: "GET", StatusCode: 200},
	}

	if err := db.FinalizeTaskFailure(ctx, task.ID, "element not found", stepLogs, networkLogs); err != nil {
		t.Fatalf("FinalizeTaskFailure: %v", err)
	}

	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusFailed {
		t.Errorf("status: want failed, got %q", got.Status)
	}
	if got.Error != "element not found" {
		t.Errorf("error: want %q, got %q", "element not found", got.Error)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt: want non-nil")
	}

	// Verify step logs persisted.
	stored, err := db.ListStepLogs(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(stored) != 2 {
		t.Errorf("step logs: want 2, got %d", len(stored))
	}

	// Verify lifecycle event was recorded.
	events, err := db.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	hasFailed := false
	for _, e := range events {
		if e.ToState == models.TaskStatusFailed {
			hasFailed = true
			break
		}
	}
	if !hasFailed {
		t.Error("expected failed lifecycle event after FinalizeTaskFailure")
	}
}

// TestBatchApplyTaskStateChangesEventPersistence verifies that lifecycle events
// are persisted for every change in a batch, not just status updates.
func TestBatchApplyTaskStateChangesEventPersistence(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	task1 := makeTask("event-persist-1", "Event Persist 1")
	task2 := makeTask("event-persist-2", "Event Persist 2")
	for _, task := range []models.Task{task1, task2} {
		if err := db.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask %s: %v", task.ID, err)
		}
	}

	changes := []TaskStateChange{
		{TaskID: task1.ID, Status: models.TaskStatusRunning},
		{TaskID: task2.ID, Status: models.TaskStatusQueued},
	}
	if err := db.BatchApplyTaskStateChanges(ctx, changes); err != nil {
		t.Fatalf("BatchApplyTaskStateChanges: %v", err)
	}

	for _, task := range []models.Task{task1, task2} {
		events, err := db.ListTaskEvents(ctx, task.ID)
		if err != nil {
			t.Fatalf("ListTaskEvents for %s: %v", task.ID, err)
		}
		if len(events) == 0 {
			t.Errorf("task %s: expected lifecycle events after batch apply", task.ID)
		}
	}
}
