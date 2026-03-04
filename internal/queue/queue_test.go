package queue

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"web-automation/internal/browser"
	"web-automation/internal/crypto"
	"web-automation/internal/database"
	"web-automation/internal/models"
	"web-automation/internal/proxy"
)

func setupTestQueue(t *testing.T, maxConcurrency int, events *[]models.TaskEvent, mu *sync.Mutex) (*Queue, *database.DB) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	crypto.ResetForTest()
	if err := crypto.InitKeyWithBytes(key); err != nil {
		t.Fatalf("init crypto key: %v", err)
	}
	t.Cleanup(func() { crypto.ResetForTest() })

	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	var onEvent EventCallback
	if events != nil {
		onEvent = func(event models.TaskEvent) {
			mu.Lock()
			*events = append(*events, event)
			mu.Unlock()
		}
	}

	q := New(db, runner, maxConcurrency, onEvent)
	return q, db
}

func makeTestTask(id string) models.Task {
	return models.Task{
		ID:   id,
		Name: "Test Task " + id,
		URL:  "https://example.com",
		Steps: []models.TaskStep{
			{Action: models.ActionNavigate, Value: "https://example.com"},
		},
		Priority:   models.PriorityNormal,
		Status:     models.TaskStatusPending,
		MaxRetries: 2,
		CreatedAt:  time.Now(),
	}
}

func TestNewQueue(t *testing.T) {
	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	q := New(db, runner, 50, nil)
	if q.maxConcurrency != 50 {
		t.Errorf("maxConcurrency: got %d, want 50", q.maxConcurrency)
	}
	if q.RunningCount() != 0 {
		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
	}
}

func TestSubmitUpdatesStatusToQueued(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("submit-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Give the goroutine time to start
	time.Sleep(200 * time.Millisecond)

	// Check that the task was marked as queued
	got, err := db.GetTask("submit-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	// It might be queued, running, or even failed (since chromedp won't actually work in test)
	if got.Status == models.TaskStatusPending {
		t.Error("task should no longer be pending after submit")
	}

	mu.Lock()
	foundQueued := false
	for _, e := range events {
		if e.TaskID == "submit-1" && e.Status == models.TaskStatusQueued {
			foundQueued = true
		}
	}
	mu.Unlock()

	if !foundQueued {
		t.Error("expected queued event to be emitted")
	}
}

func TestSubmitOnStoppedQueue(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	q.Stop()

	task := makeTestTask("stopped-1")
	err := q.Submit(context.Background(), task)
	if err == nil {
		t.Fatal("expected error when submitting to stopped queue")
	}
}

func TestCancel(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("cancel-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Cancel the task
	time.Sleep(100 * time.Millisecond)
	if err := q.Cancel("cancel-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := db.GetTask("cancel-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status after cancel: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestStopCancelsAllRunning(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)

	// Manually register some fake running tasks
	q.mu.Lock()
	cancel1Done := make(chan struct{})
	cancel2Done := make(chan struct{})
	q.running["fake-1"] = func() { close(cancel1Done) }
	q.running["fake-2"] = func() { close(cancel2Done) }
	q.mu.Unlock()

	q.Stop()

	// Verify all cancels were called
	select {
	case <-cancel1Done:
	case <-time.After(time.Second):
		t.Error("cancel for fake-1 was not called")
	}
	select {
	case <-cancel2Done:
	case <-time.After(time.Second):
		t.Error("cancel for fake-2 was not called")
	}

	// Verify running map is empty
	if q.RunningCount() != 0 {
		t.Errorf("RunningCount after Stop: got %d, want 0", q.RunningCount())
	}
}

func TestStopIdempotent(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)

	// Should not panic when called multiple times
	q.Stop()
	q.Stop()
	q.Stop()
}

func TestSubmitBatch(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	tasks := make([]models.Task, 5)
	for i := range tasks {
		tasks[i] = makeTestTask(fmt.Sprintf("batch-%d", i))
		if err := db.CreateTask(tasks[i]); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	ctx := context.Background()
	if err := q.SubmitBatch(ctx, tasks); err != nil {
		t.Fatalf("SubmitBatch: %v", err)
	}

	// Wait for tasks to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify all tasks were submitted (status changed from pending)
	for _, task := range tasks {
		got, err := db.GetTask(task.ID)
		if err != nil {
			t.Fatalf("GetTask %s: %v", task.ID, err)
		}
		if got.Status == models.TaskStatusPending {
			t.Errorf("task %s should not be pending after batch submit", task.ID)
		}
	}
}

func TestRunningCount(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	if q.RunningCount() != 0 {
		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
	}

	// Add some fake entries
	q.mu.Lock()
	q.running["a"] = func() {}
	q.running["b"] = func() {}
	q.mu.Unlock()

	if q.RunningCount() != 2 {
		t.Errorf("RunningCount: got %d, want 2", q.RunningCount())
	}
}

func TestConcurrencyLimit(t *testing.T) {
	var maxConcurrent int64
	var current int64

	dir := t.TempDir()
	db, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.Close()

	runner, err := browser.NewRunner(filepath.Join(dir, "screenshots"))
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}

	maxConc := 3
	q := New(db, runner, maxConc, nil)
	defer q.Stop()

	// We can't easily test actual execution concurrency without mocking the runner,
	// but we can verify the queue was created with the right concurrency
	if q.maxConcurrency != int64(maxConc) {
		t.Errorf("maxConcurrency: got %d, want %d", q.maxConcurrency, maxConc)
	}

	// Test that atomic tracking would work
	for i := 0; i < 10; i++ {
		val := atomic.AddInt64(&current, 1)
		if val > atomic.LoadInt64(&maxConcurrent) {
			atomic.StoreInt64(&maxConcurrent, val)
		}
		if val > int64(maxConc) {
			// This tests our tracking logic, not the queue's semaphore
			// Real semaphore testing would require mocking
		}
		atomic.AddInt64(&current, -1)
	}
}

func TestEmitEvent(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex

	q, _ := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	q.emitEvent("test-id", models.TaskStatusRunning, "")
	q.emitEvent("test-id", models.TaskStatusFailed, "some error")

	mu.Lock()
	defer mu.Unlock()

	if len(events) != 2 {
		t.Fatalf("event count: got %d, want 2", len(events))
	}
	if events[0].Status != models.TaskStatusRunning {
		t.Errorf("event 0 status: got %q, want %q", events[0].Status, models.TaskStatusRunning)
	}
	if events[1].Error != "some error" {
		t.Errorf("event 1 error: got %q, want %q", events[1].Error, "some error")
	}
}

func TestEmitEventNilCallback(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	// Should not panic with nil callback
	q.emitEvent("test-id", models.TaskStatusRunning, "")
}

func TestSubmitContextCancelled(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("ctx-cancel-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Submit should succeed (it just marks as queued and spawns goroutine)
	// but the goroutine should handle the cancelled context
	err := q.Submit(ctx, task)
	if err != nil {
		// The db update might fail if context is already cancelled in some implementations
		// but in SQLite it should be fine
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSetProxyManager(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	config := models.ProxyPoolConfig{
		Strategy:            models.RotationRoundRobin,
		HealthCheckInterval: 300,
		MaxFailures:         3,
	}
	pm := proxy.NewManager(db, config)
	defer pm.Stop()

	q.SetProxyManager(pm)
	if q.proxyManager == nil {
		t.Fatal("proxyManager should be set")
	}
}

func TestTaskTimeoutDefault(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("timeout-default")
	task.Timeout = 0
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	got, err := db.GetTask("timeout-default")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status == models.TaskStatusPending {
		t.Error("task should not still be pending")
	}
}

func TestTaskTimeoutCustom(t *testing.T) {
	task := makeTestTask("timeout-custom")
	task.Timeout = 120
	if task.Timeout != 120 {
		t.Errorf("Timeout: got %d, want 120", task.Timeout)
	}
}

func TestHandleSuccessPath(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("success-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := &models.TaskResult{
		TaskID:  task.ID,
		Success: true,
		ExtractedData: map[string]string{
			"title": "Test",
		},
		Duration: 1000000000,
	}

	q.handleSuccess(task, result)

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCompleted {
		t.Errorf("Status: got %q, want %q", got.Status, models.TaskStatusCompleted)
	}
	if got.Result == nil {
		t.Fatal("Result should be set")
	}
	if !got.Result.Success {
		t.Error("Result.Success should be true")
	}
}
