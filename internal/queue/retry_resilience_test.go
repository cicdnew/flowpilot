package queue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/monitoring"
)

func TestHandleFailurePersistsRetryAttemptLogs(t *testing.T) {
	q, db := setupTestQueue(t, 1, nil, nil)
	defer q.Stop()

	task := makeTestTask("retry-logs-1")
	task.MaxRetries = 2
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := &models.TaskResult{
		TaskID: task.ID,
		StepLogs: []models.StepLog{{
			TaskID:     task.ID,
			StepIndex:  0,
			Action:     models.ActionNavigate,
			DurationMs: 25,
			StartedAt:  time.Now(),
		}},
		NetworkLogs: []models.NetworkLog{{
			TaskID:      task.ID,
			StepIndex:   0,
			RequestURL:  "http://chhotu-bin.infy.uk",
			Method:      "GET",
			StatusCode:  500,
			DurationMs:  10,
			Timestamp:   time.Now(),
		}},
	}

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("temporary failure"), result)
	if !ri.shouldRetry {
		t.Fatal("expected retry info to request retry")
	}

	stepLogs, err := db.ListStepLogs(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListStepLogs: %v", err)
	}
	if len(stepLogs) != 1 {
		t.Fatalf("expected 1 persisted retry step log, got %d", len(stepLogs))
	}

	networkLogs, err := db.ListNetworkLogs(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListNetworkLogs: %v", err)
	}
	if len(networkLogs) != 1 {
		t.Fatalf("expected 1 persisted retry network log, got %d", len(networkLogs))
	}
}

func TestStopCancelsInFlightWebhook(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)

	task := makeTestTask("webhook-stop-1")
	task.WebhookURL = "http://example.invalid"
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var started atomic.Bool
	// unblockHandler is closed by the test to let the handler return,
	// avoiding a deadlock between httptest.Server.Close() and a blocking handler.
	unblockHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started.Store(true)
		select {
		case <-r.Context().Done():
		case <-unblockHandler:
		}
	}))
	task.WebhookURL = server.URL

	result := &models.TaskResult{TaskID: task.ID, Success: true, Duration: 50 * time.Millisecond}
	q.handleSuccess(context.Background(), task, result)

	deadline := time.Now().Add(2 * time.Second)
	for !started.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !started.Load() {
		t.Fatal("webhook handler never received request")
	}

	start := time.Now()
	q.Stop()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Stop took too long waiting for webhook cancellation: %v", elapsed)
	}

	// Unblock the handler so the server can close cleanly.
	close(unblockHandler)
	server.Close()
}

func TestCancelDuringRetryBackoffPreventsResubmit(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	q.maxPending = 10
	ctx := context.Background()

	task := makeTestTask("retry-cancel-backoff")
	task.MaxRetries = 1
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ri := q.handleFailure(ctx, task, fmt.Errorf("transient failure"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected a retry to be scheduled")
	}
	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	ri.backoff = time.Millisecond
	q.scheduleRetry(ctx, ri)

	q.mu.Lock()
	queued := q.isTaskInHeap(task.ID)
	q.mu.Unlock()
	if queued {
		t.Fatal("task cancelled during retry backoff was re-enqueued")
	}
	got, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Fatalf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestCancelWhileAttemptRunningSkipsRetry(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	ctx := context.Background()

	task := makeTestTask("retry-cancel-running")
	task.MaxRetries = 1
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	q.mu.Lock()
	q.running[task.ID] = func() {}
	q.mu.Unlock()
	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if ri := q.handleFailure(ctx, task, fmt.Errorf("transient failure"), nil); ri.shouldRetry {
		t.Fatal("retry scheduled for a task cancelled while its attempt was running")
	}
}

func TestTaskOutcomesUpdateMetricsAndMonitor(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	ctx := context.Background()
	monitor := monitoring.New()
	q.SetMonitor(monitor)
	monitor.RecordRepeatBatchCreated("repeat-batch", 2, "counter")

	ok := makeTestTask("outcome-success")
	ok.BatchID = "repeat-batch"
	bad := makeTestTask("outcome-failure")
	bad.BatchID = "repeat-batch"
	bad.MaxRetries = 0
	for _, task := range []models.Task{ok, bad} {
		if err := db.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
	}

	q.handleSuccess(ctx, ok, &models.TaskResult{TaskID: ok.ID, Success: true, Duration: 2 * time.Second})
	q.handleFailure(ctx, bad, fmt.Errorf("permanent failure"), nil)

	metrics := q.TaskMetrics()
	if metrics.Completed != 1 || metrics.Failed != 1 || metrics.AvgDurationMs != 2000 {
		t.Fatalf("task metrics: got %+v, want 1 completed, 1 failed, 2000ms average", metrics)
	}
	repeat := monitor.GetRepeatTaskMetrics()
	if repeat.CompletedRepeatedTasks != 1 || repeat.FailedRepeatedTasks != 1 {
		t.Fatalf("repeat metrics: got %+v", repeat)
	}
	if repeat.ActiveRepeatedBatches != 0 {
		t.Fatalf("active repeated batches: got %d, want 0 after all tasks finished", repeat.ActiveRepeatedBatches)
	}
	system := monitor.GetSystemMetrics()
	if system.TotalRequests != 2 || system.TotalErrors != 1 {
		t.Fatalf("system metrics: got %d requests, %d errors; want 2 and 1", system.TotalRequests, system.TotalErrors)
	}
}
