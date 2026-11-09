package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"flowpilot/internal/models"
)

func TestRunnerCanceledWithoutUserCancelIsRetried(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()
	task := makeTestTask("spurious-cancel-retry")
	createTask(t, db, task)
	markRunning(q, task.ID)
	runErr := fmt.Errorf("new tab: %w", context.Canceled)

	if q.handleTaskCancellation(context.Background(), task.ID, runErr) {
		t.Fatal("runner context.Canceled without a user cancel was treated as a cancellation")
	}
	ri := q.handleFailure(context.Background(), task, runErr, nil)
	if !ri.shouldRetry {
		t.Fatal("expected the spurious cancellation to be retried")
	}
	q.unregisterRetry(task.ID)

	assertStatus(t, db, task.ID, models.TaskStatusRetrying)
	assertCounts(t, q, 0, 0)
	for _, status := range terminalEvents(events, &mu, task.ID) {
		if status == models.TaskStatusCancelled {
			t.Fatal("spurious cancellation emitted a cancelled event")
		}
	}
}

func TestRunnerCanceledWithoutUserCancelFailsOnFinalAttempt(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("spurious-cancel-final")
	task.RetryCount = task.MaxRetries
	createTask(t, db, task)
	markRunning(q, task.ID)

	if q.handleTaskCancellation(context.Background(), task.ID, context.Canceled) {
		t.Fatal("runner context.Canceled without a user cancel was treated as a cancellation")
	}
	q.handleFailure(context.Background(), task, context.Canceled, nil)

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusFailed || got.Error == errCancelledByUser {
		t.Fatalf("stored outcome: status=%q error=%q, want failed without %q", got.Status, got.Error, errCancelledByUser)
	}
	assertCounts(t, q, 1, 0)
}

func TestRunnerCanceledByParentContextIsCancellation(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("parent-cancelled")
	createTask(t, db, task)
	markRunning(q, task.ID)
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	if !q.handleTaskCancellation(parent, task.ID, context.Canceled) {
		t.Fatal("cancelled parent context must be handled as a cancellation")
	}
	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

func TestRunnerCanceledWhileQueueStoppingIsCancellation(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("queue-stopping")
	createTask(t, db, task)
	markRunning(q, task.ID)
	q.mu.Lock()
	q.stopped = true
	q.mu.Unlock()

	if !q.handleTaskCancellation(context.Background(), task.ID, context.Canceled) {
		t.Fatal("context.Canceled while the queue is stopping must be handled as a cancellation")
	}
	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
}
