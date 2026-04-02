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
			RequestURL:  "https://example.com",
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
