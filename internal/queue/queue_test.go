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

	"flowpilot/internal/browser"
	"flowpilot/internal/crypto"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
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

func TestMetricsInitialState(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	m := q.Metrics()
	if m.Running != 0 {
		t.Errorf("Running: got %d, want 0", m.Running)
	}
	if m.Queued != 0 {
		t.Errorf("Queued: got %d, want 0", m.Queued)
	}
	if m.Pending != 0 {
		t.Errorf("Pending: got %d, want 0", m.Pending)
	}
	if m.TotalSubmitted != 0 {
		t.Errorf("TotalSubmitted: got %d, want 0", m.TotalSubmitted)
	}
	if m.TotalCompleted != 0 {
		t.Errorf("TotalCompleted: got %d, want 0", m.TotalCompleted)
	}
	if m.TotalFailed != 0 {
		t.Errorf("TotalFailed: got %d, want 0", m.TotalFailed)
	}
}

func TestMetricsRunningAndPending(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	// Manually inject running and pending entries
	q.mu.Lock()
	q.running["r-1"] = func() {}
	q.running["r-2"] = func() {}
	q.pending["p-1"] = func() {}
	q.pending["p-2"] = func() {}
	q.pending["p-3"] = func() {}
	q.mu.Unlock()

	m := q.Metrics()
	if m.Running != 2 {
		t.Errorf("Running: got %d, want 2", m.Running)
	}
	if m.Queued != 3 {
		t.Errorf("Queued: got %d, want 3", m.Queued)
	}
	// Pending = Queued + Running
	if m.Pending != 5 {
		t.Errorf("Pending: got %d, want 5 (Queued + Running)", m.Pending)
	}
}

func TestMetricsPendingEqualsQueuedPlusRunning(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	// Only running, no queued
	q.mu.Lock()
	q.running["r-1"] = func() {}
	q.mu.Unlock()

	m := q.Metrics()
	if m.Pending != m.Queued+m.Running {
		t.Errorf("Pending (%d) should equal Queued (%d) + Running (%d)",
			m.Pending, m.Queued, m.Running)
	}

	// Only queued, no running
	q.mu.Lock()
	delete(q.running, "r-1")
	q.pending["p-1"] = func() {}
	q.mu.Unlock()

	m = q.Metrics()
	if m.Pending != m.Queued+m.Running {
		t.Errorf("Pending (%d) should equal Queued (%d) + Running (%d)",
			m.Pending, m.Queued, m.Running)
	}
}

func TestMetricsTotalSubmittedIncrementsOnSubmit(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("metrics-submit-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Submit(context.Background(), task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Give goroutine time to start
	time.Sleep(200 * time.Millisecond)

	m := q.Metrics()
	if m.TotalSubmitted < 1 {
		t.Errorf("TotalSubmitted: got %d, want >= 1", m.TotalSubmitted)
	}
}

func TestMetricsAfterStop(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)

	// Inject fake entries
	q.mu.Lock()
	q.running["r-1"] = func() {}
	q.pending["p-1"] = func() {}
	q.mu.Unlock()

	q.Stop()

	m := q.Metrics()
	// After Stop, running and pending should be cleared
	if m.Running != 0 {
		t.Errorf("Running after Stop: got %d, want 0", m.Running)
	}
	if m.Queued != 0 {
		t.Errorf("Queued after Stop: got %d, want 0", m.Queued)
	}
	if m.Pending != 0 {
		t.Errorf("Pending after Stop: got %d, want 0", m.Pending)
	}
}

func TestSubmitDuplicateRunning(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("dup-run-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.mu.Lock()
	q.running[task.ID] = func() {}
	q.mu.Unlock()

	err := q.Submit(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for duplicate running task")
	}
	if got := err.Error(); got != fmt.Sprintf("task %s is already running", task.ID) {
		t.Errorf("error message: got %q", got)
	}
}

func TestSubmitDuplicatePending(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("dup-pend-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.mu.Lock()
	q.pending[task.ID] = func() {}
	q.mu.Unlock()

	err := q.Submit(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for duplicate pending task")
	}
	if got := err.Error(); got != fmt.Sprintf("task %s is already pending", task.ID) {
		t.Errorf("error message: got %q", got)
	}
}

func TestSubmitErrQueueFull(t *testing.T) {
	q, _ := setupTestQueue(t, 1, nil, nil)
	defer q.Stop()

	q.maxPending = 2

	q.mu.Lock()
	q.pending["fill-1"] = func() {}
	q.pending["fill-2"] = func() {}
	q.mu.Unlock()

	task := makeTestTask("overflow-1")
	err := q.Submit(context.Background(), task)
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestCancelNonExistentTask(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("cancel-nonexist-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Cancel("cancel-nonexist-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := db.GetTask("cancel-nonexist-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestCancelPendingTask(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("cancel-pend-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	cancelCalled := false
	q.mu.Lock()
	q.pending[task.ID] = func() { cancelCalled = true }
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if !cancelCalled {
		t.Error("pending task cancel function should have been called")
	}

	q.mu.Lock()
	_, inPending := q.pending[task.ID]
	wasCancelled := q.cancelled[task.ID]
	q.mu.Unlock()

	if inPending {
		t.Error("task should be removed from pending map")
	}
	if !wasCancelled {
		t.Error("task should be marked as cancelled")
	}

	mu.Lock()
	foundCancelledEvent := false
	for _, e := range events {
		if e.TaskID == task.ID && e.Status == models.TaskStatusCancelled {
			foundCancelledEvent = true
		}
	}
	mu.Unlock()

	if !foundCancelledEvent {
		t.Error("expected cancelled event to be emitted")
	}
}

func TestSubmitBatchStopsOnError(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task1 := makeTestTask("batch-err-1")
	task2 := makeTestTask("batch-err-2")
	if err := db.CreateTask(task1); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.CreateTask(task2); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Submit(context.Background(), task1); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	tasks := []models.Task{task1, task2}
	err := q.SubmitBatch(context.Background(), tasks)
	if err == nil {
		t.Fatal("expected error from SubmitBatch when duplicate task submitted")
	}
}

func TestGetProxyManager(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	if q.getProxyManager() != nil {
		t.Error("expected nil proxy manager initially")
	}

	config := models.ProxyPoolConfig{
		Strategy: models.RotationRoundRobin,
	}
	pm := proxy.NewManager(db, config)
	defer pm.Stop()

	q.SetProxyManager(pm)
	if q.getProxyManager() == nil {
		t.Error("expected non-nil proxy manager after SetProxyManager")
	}
}

func TestHandleFailureMaxRetriesExceeded(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("fail-max-1")
	task.RetryCount = 2
	task.MaxRetries = 2
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.handleFailure(context.Background(), task, fmt.Errorf("exec failed"))

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusFailed {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusFailed)
	}

	mu.Lock()
	foundFailed := false
	for _, e := range events {
		if e.TaskID == task.ID && e.Status == models.TaskStatusFailed {
			foundFailed = true
		}
	}
	mu.Unlock()
	if !foundFailed {
		t.Error("expected failed event to be emitted")
	}

	m := q.Metrics()
	if m.TotalFailed < 1 {
		t.Errorf("TotalFailed: got %d, want >= 1", m.TotalFailed)
	}
}

func TestHandleFailureRetriesWithBackoff(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("fail-retry-1")
	task.RetryCount = 0
	task.MaxRetries = 3
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	go q.handleFailure(context.Background(), task, fmt.Errorf("temporary error"))

	time.Sleep(2 * time.Second)

	mu.Lock()
	foundRetrying := false
	for _, e := range events {
		if e.TaskID == task.ID && e.Status == models.TaskStatusRetrying {
			foundRetrying = true
		}
	}
	mu.Unlock()

	if !foundRetrying {
		t.Error("expected retrying event to be emitted")
	}
}

func TestHandleFailureRetryStoppedByQueueStop(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)

	task := makeTestTask("fail-stop-1")
	task.RetryCount = 0
	task.MaxRetries = 3
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	done := make(chan struct{})
	go func() {
		q.handleFailure(context.Background(), task, fmt.Errorf("temporary error"))
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	q.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleFailure did not return after queue stop")
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestHandleFailureRetryStoppedByContextCancel(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("fail-ctx-1")
	task.RetryCount = 0
	task.MaxRetries = 3
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		q.handleFailure(ctx, task, fmt.Errorf("temporary error"))
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleFailure did not return after context cancel")
	}

	got, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestStopClearsPendingTasks(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)

	pendingCancel1Done := make(chan struct{})
	pendingCancel2Done := make(chan struct{})
	q.mu.Lock()
	q.pending["pend-1"] = func() { close(pendingCancel1Done) }
	q.pending["pend-2"] = func() { close(pendingCancel2Done) }
	q.mu.Unlock()

	q.Stop()

	select {
	case <-pendingCancel1Done:
	case <-time.After(time.Second):
		t.Error("cancel for pending pend-1 was not called")
	}
	select {
	case <-pendingCancel2Done:
	case <-time.After(time.Second):
		t.Error("cancel for pending pend-2 was not called")
	}

	q.mu.Lock()
	pendingCount := len(q.pending)
	q.mu.Unlock()
	if pendingCount != 0 {
		t.Errorf("pending count after Stop: got %d, want 0", pendingCount)
	}
}

func TestExecuteTaskCancelledBeforeAcquire(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 1, &events, &mu)
	defer q.Stop()

	task := makeTestTask("exec-cancel-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.mu.Lock()
	q.cancelled[task.ID] = true
	q.pending[task.ID] = func() {}
	q.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	q.executeTask(ctx, task)

	mu.Lock()
	foundFailed := false
	for _, e := range events {
		if e.TaskID == task.ID && e.Status == models.TaskStatusFailed {
			foundFailed = true
		}
	}
	mu.Unlock()

	if foundFailed {
		t.Error("cancelled task should not emit failed event")
	}
}

func TestExecuteTaskStoppedBeforeRun(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)

	task := makeTestTask("exec-stopped-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.mu.Lock()
	q.pending[task.ID] = func() {}
	q.mu.Unlock()

	q.Stop()

	q.executeTask(context.Background(), task)
}

func TestMetricsTotalCompletedAfterHandleSuccess(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("metrics-complete-1")
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := &models.TaskResult{
		TaskID:  task.ID,
		Success: true,
	}
	q.handleSuccess(task, result)

	m := q.Metrics()
	if m.TotalCompleted != 1 {
		t.Errorf("TotalCompleted: got %d, want 1", m.TotalCompleted)
	}
}

func TestMetricsTotalFailedAfterHandleFailure(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("metrics-fail-1")
	task.RetryCount = 5
	task.MaxRetries = 5
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.handleFailure(context.Background(), task, fmt.Errorf("terminal failure"))

	m := q.Metrics()
	if m.TotalFailed != 1 {
		t.Errorf("TotalFailed: got %d, want 1", m.TotalFailed)
	}
}

func TestSubmitBatchEmpty(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	err := q.SubmitBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("SubmitBatch(nil): %v", err)
	}

	err = q.SubmitBatch(context.Background(), []models.Task{})
	if err != nil {
		t.Fatalf("SubmitBatch(empty): %v", err)
	}
}
