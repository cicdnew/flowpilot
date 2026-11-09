package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestCancelBeforeExecutionCountedOnce(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	task := makeTestTask("cancel-before-execute")
	createTask(t, db, task)
	markRunning(q, task.ID)

	if err := q.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	q.executeTask(context.Background(), task, false, false)

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
	if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != models.TaskStatusCancelled {
		t.Errorf("terminal events: got %v, want [cancelled]", got)
	}
}

func TestCancelDuringTaskContextHandoff(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	task := makeTestTask("cancel-context-handoff")
	createTask(t, db, task)
	markRunning(q, task.ID)

	if q.shouldSkipExecution(task.ID) {
		t.Fatal("task should still be runnable")
	}
	if err := q.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.setTaskCancel(task.ID, cancel)
	if ctx.Err() != context.Canceled {
		t.Fatal("new execution context was not cancelled")
	}
	q.handleTaskCancellation(ctx, task.ID, ctx.Err())

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

func TestCancelDuringRetryWorkerHandoff(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	task := makeTestTask("cancel-retry-handoff")
	createTask(t, db, task)
	markRunning(q, task.ID)

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("transient"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected retry")
	}
	if err := q.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	proxied := false
	q.finishExecuteTask(task.ID, &proxied)
	ri.backoff = time.Millisecond
	q.scheduleRetry(context.Background(), ri)

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
	q.mu.Lock()
	_, registered := q.retrying[task.ID]
	queued := q.pq.Len()
	q.mu.Unlock()
	if registered || queued != 0 {
		t.Errorf("cancelled retry still active: registered=%t queued=%d", registered, queued)
	}
}

func TestSuccessClaimSurvivesCancelBeforePersistence(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	task := makeTestTask("success-before-persistence")
	createTask(t, db, task)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.setTaskCancel(task.ID, cancel)

	tx, err := db.Conn().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	done := make(chan struct{})
	go func() {
		defer close(done)
		q.handleSuccess(ctx, task, successResult(task.ID))
	}()
	t.Cleanup(func() {
		_ = tx.Rollback()
		<-done
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		q.mu.Lock()
		_, claimed := q.finalizing[task.ID]
		q.mu.Unlock()
		if claimed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not claim finalization")
		}
		time.Sleep(time.Millisecond)
	}
	if err := q.Cancel(task.ID); err != nil {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("late Cancel interrupted completion persistence")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	<-done

	assertStatus(t, db, task.ID, models.TaskStatusCompleted)
	assertCounts(t, q, 0, 0)
	if got := q.Metrics().TotalCompleted; got != 1 {
		t.Errorf("TotalCompleted: got %d, want 1", got)
	}
	if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != models.TaskStatusCompleted {
		t.Errorf("terminal events: got %v, want [completed]", got)
	}
}
