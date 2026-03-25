package queue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
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

func setupTestQueueNoWorkers(t *testing.T, events *[]models.TaskEvent, mu *sync.Mutex) (*Queue, *database.DB) {
	t.Helper()
	return setupTestQueue(t, 0, events, mu)
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
	if q.workerCount != 50 {
		t.Errorf("workerCount: got %d, want 50", q.workerCount)
	}
	if q.RunningCount() != 0 {
		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
	}
	q.Stop()
}

func TestSubmitUpdatesStatusToQueued(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("submit-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	got, err := db.GetTask(context.Background(), "submit-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := q.Cancel("cancel-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := db.GetTask(context.Background(), "cancel-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status after cancel: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestStopCancelsAllRunning(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)

	q.mu.Lock()
	cancel1Done := make(chan struct{})
	cancel2Done := make(chan struct{})
	q.running["fake-1"] = func() { close(cancel1Done) }
	q.running["fake-2"] = func() { close(cancel2Done) }
	q.mu.Unlock()

	q.Stop()

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

	if q.RunningCount() != 0 {
		t.Errorf("RunningCount after Stop: got %d, want 0", q.RunningCount())
	}
}

func TestStopIdempotent(t *testing.T) {
	q, _ := setupTestQueue(t, 10, nil, nil)
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
		if err := db.CreateTask(context.Background(), tasks[i]); err != nil {
			t.Fatalf("CreateTask %d: %v", i, err)
		}
	}

	ctx := context.Background()
	if err := q.SubmitBatch(ctx, tasks); err != nil {
		t.Fatalf("SubmitBatch: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	for _, task := range tasks {
		got, err := db.GetTask(context.Background(), task.ID)
		if err != nil {
			t.Fatalf("GetTask %s: %v", task.ID, err)
		}
		if got.Status == models.TaskStatusPending {
			t.Errorf("task %s should not be pending after batch submit", task.ID)
		}
	}
}

func TestRunningCount(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	if q.RunningCount() != 0 {
		t.Errorf("initial RunningCount: got %d, want 0", q.RunningCount())
	}

	q.mu.Lock()
	q.running["a"] = func() {}
	q.running["b"] = func() {}
	q.mu.Unlock()

	if q.RunningCount() != 2 {
		t.Errorf("RunningCount: got %d, want 2", q.RunningCount())
	}
}

func TestConcurrencyLimit(t *testing.T) {
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

	if q.workerCount != maxConc {
		t.Errorf("workerCount: got %d, want %d", q.workerCount, maxConc)
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
	q.emitEvent("test-id", models.TaskStatusRunning, "")
}

func TestSubmitContextCancelled(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("ctx-cancel-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := q.Submit(ctx, task)
	if err != nil {
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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx := context.Background()
	if err := q.Submit(ctx, task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	got, err := db.GetTask(context.Background(), "timeout-default")
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
	if err := db.CreateTask(context.Background(), task); err != nil {
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

	q.handleSuccess(context.Background(), task, result)

	got, err := db.GetTask(context.Background(), task.ID)
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

func TestMetricsRunningAndQueued(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	q.mu.Lock()
	q.running["r-1"] = func() {}
	q.running["r-2"] = func() {}
	now := time.Now()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "p-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["p-1"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "p-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["p-2"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "p-3", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["p-3"] = struct{}{}
	q.mu.Unlock()

	m := q.Metrics()
	if m.Running != 2 {
		t.Errorf("Running: got %d, want 2", m.Running)
	}
	if m.Queued != 3 {
		t.Errorf("Queued: got %d, want 3", m.Queued)
	}
	if m.Pending != 5 {
		t.Errorf("Pending: got %d, want 5 (Queued + Running)", m.Pending)
	}
}

func TestMetricsPendingEqualsQueuedPlusRunning(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	q.mu.Lock()
	q.running["r-1"] = func() {}
	q.mu.Unlock()

	m := q.Metrics()
	if m.Pending != m.Queued+m.Running {
		t.Errorf("Pending (%d) should equal Queued (%d) + Running (%d)", m.Pending, m.Queued, m.Running)
	}

	q.mu.Lock()
	delete(q.running, "r-1")
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "p-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet["p-1"] = struct{}{}
	q.mu.Unlock()

	m = q.Metrics()
	if m.Pending != m.Queued+m.Running {
		t.Errorf("Pending (%d) should equal Queued (%d) + Running (%d)", m.Pending, m.Queued, m.Running)
	}
}

func TestMetricsTotalSubmittedIncrementsOnSubmit(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("metrics-submit-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Submit(context.Background(), task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	m := q.Metrics()
	if m.TotalSubmitted < 1 {
		t.Errorf("TotalSubmitted: got %d, want >= 1", m.TotalSubmitted)
	}
}

func TestMetricsAfterStop(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)

	q.mu.Lock()
	q.running["r-1"] = func() {}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "p-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet["p-1"] = struct{}{}
	q.mu.Unlock()

	q.Stop()

	m := q.Metrics()
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
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	task := makeTestTask("dup-run-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
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
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	task := makeTestTask("dup-pend-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: task.ID, Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[task.ID] = struct{}{}
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
	q, db := setupTestQueue(t, 1, nil, nil)
	defer q.Stop()

	q.maxPending = 2

	// Use paused heap so workers don't pop these items.
	q.mu.Lock()
	q.paused["fill-batch"] = true
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "fill-1", BatchID: "fill-batch", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.pausedSet["fill-1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "fill-2", BatchID: "fill-batch", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.pausedSet["fill-2"] = struct{}{}
	q.mu.Unlock()

	task := makeTestTask("overflow-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	err := q.Submit(context.Background(), task)
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestCancelNonExistentTask(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("cancel-nonexist-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Cancel("cancel-nonexist-1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := db.GetTask(context.Background(), "cancel-nonexist-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestCancelPendingTaskInHeap(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()

	task := makeTestTask("cancel-heap-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	cancelCalled := false
	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{
		task:    task,
		ctx:     context.Background(),
		cancel:  func() { cancelCalled = true },
		addedAt: time.Now(),
	})
	q.heapSet[task.ID] = struct{}{}
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if !cancelCalled {
		t.Error("cancel function should have been called")
	}

	q.mu.Lock()
	inHeap := q.isTaskInHeap(task.ID)
	wasCancelled := q.cancelled[task.ID]
	q.mu.Unlock()

	if inHeap {
		t.Error("task should be removed from heap")
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

func TestCancelRunningTaskUpdatesStateAndEmitsEvent(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()

	task := makeTestTask("cancel-running-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	cancelDone := make(chan struct{})
	q.mu.Lock()
	q.running[task.ID] = func() { close(cancelDone) }
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	select {
	case <-cancelDone:
	case <-time.After(time.Second):
		t.Fatal("running task cancel function was not called")
	}

	q.mu.Lock()
	_, stillRunning := q.running[task.ID]
	wasCancelled := q.cancelled[task.ID]
	q.mu.Unlock()
	if stillRunning {
		t.Error("task should be removed from running map")
	}
	if !wasCancelled {
		t.Error("task should be marked as cancelled")
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Fatalf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}

	mu.Lock()
	defer mu.Unlock()
	foundCancelledEvent := false
	for _, e := range events {
		if e.TaskID == task.ID && e.Status == models.TaskStatusCancelled {
			foundCancelledEvent = true
			break
		}
	}
	if !foundCancelledEvent {
		t.Error("expected cancelled event to be emitted")
	}
}

func TestSubmitBatchSkipsDuplicates(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task1 := makeTestTask("batch-dup-1")
	task2 := makeTestTask("batch-dup-2")
	if err := db.CreateTask(context.Background(), task1); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.CreateTask(context.Background(), task2); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := q.Submit(context.Background(), task1); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	tasks := []models.Task{task1, task2}
	err := q.SubmitBatch(context.Background(), tasks)
	if err != nil {
		t.Fatalf("SubmitBatch should skip duplicates silently, got: %v", err)
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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.handleFailure(context.Background(), task, fmt.Errorf("exec failed"), nil)

	got, err := db.GetTask(context.Background(), task.ID)
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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("temporary error"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected shouldRetry to be true")
	}

	go q.scheduleRetry(ri)

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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("temporary error"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected shouldRetry to be true")
	}

	done := make(chan struct{})
	go func() {
		q.scheduleRetry(ri)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	q.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduleRetry did not return after queue stop")
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestHandleFailureRetryResubmitsTask(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueue(t, 10, &events, &mu)
	defer q.Stop()

	task := makeTestTask("fail-retry-success-1")
	task.Steps = nil
	task.RetryCount = 0
	task.MaxRetries = 3
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("temporary error"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected shouldRetry to be true")
	}

	ri.backoff = 10 * time.Millisecond
	q.scheduleRetry(ri)

	deadline := time.Now().Add(3 * time.Second)
	for {
		got, err := db.GetTask(context.Background(), task.ID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if got.Status == models.TaskStatusCompleted {
			if got.RetryCount != 1 {
				t.Fatalf("retry_count: got %d, want 1", got.RetryCount)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not complete after retry, last status=%s retryCount=%d", got.Status, got.RetryCount)
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	foundRetrying := false
	foundCompleted := false
	for _, e := range events {
		if e.TaskID != task.ID {
			continue
		}
		if e.Status == models.TaskStatusRetrying {
			foundRetrying = true
		}
		if e.Status == models.TaskStatusCompleted {
			foundCompleted = true
		}
	}
	if !foundRetrying {
		t.Error("expected retrying event to be emitted")
	}
	if !foundCompleted {
		t.Error("expected completed event after retry re-submit")
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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ri := q.handleFailure(ctx, task, fmt.Errorf("temporary error"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected shouldRetry to be true")
	}

	done := make(chan struct{})
	go func() {
		q.scheduleRetry(ri)
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduleRetry did not return after context cancel")
	}

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != models.TaskStatusCancelled {
		t.Errorf("status: got %q, want %q", got.Status, models.TaskStatusCancelled)
	}
}

func TestStopClearsHeapTasks(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)

	cancel1Done := make(chan struct{})
	cancel2Done := make(chan struct{})
	q.mu.Lock()
	// Use paused heap to prevent workers from popping these items before Stop().
	q.paused["stop-batch"] = true
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "pend-1", BatchID: "stop-batch", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() { close(cancel1Done) }, addedAt: time.Now()})
	q.pausedSet["pend-1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "pend-2", BatchID: "stop-batch", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() { close(cancel2Done) }, addedAt: time.Now()})
	q.pausedSet["pend-2"] = struct{}{}
	q.mu.Unlock()

	q.Stop()

	select {
	case <-cancel1Done:
	case <-time.After(time.Second):
		t.Error("cancel for pend-1 was not called")
	}
	select {
	case <-cancel2Done:
	case <-time.After(time.Second):
		t.Error("cancel for pend-2 was not called")
	}

	q.mu.Lock()
	heapLen := q.pq.Len()
	q.mu.Unlock()
	if heapLen != 0 {
		t.Errorf("heap length after Stop: got %d, want 0", heapLen)
	}
}

func TestMetricsTotalCompletedAfterHandleSuccess(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task := makeTestTask("metrics-complete-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	result := &models.TaskResult{
		TaskID:  task.ID,
		Success: true,
	}
	q.handleSuccess(context.Background(), task, result)

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
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	q.handleFailure(context.Background(), task, fmt.Errorf("terminal failure"), nil)

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

func TestPriorityOrdering(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	low := makeTestTask("prio-low")
	low.Priority = models.PriorityLow
	normal := makeTestTask("prio-normal")
	normal.Priority = models.PriorityNormal
	high := makeTestTask("prio-high")
	high.Priority = models.PriorityHigh

	for _, task := range []models.Task{low, normal, high} {
		if err := db.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask %s: %v", task.ID, err)
		}
	}

	q.mu.Lock()
	now := time.Now()
	heap.Push(&q.pq, &heapItem{task: low, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet[low.ID] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: normal, ctx: context.Background(), cancel: func() {}, addedAt: now.Add(time.Millisecond)})
	q.heapSet[normal.ID] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: high, ctx: context.Background(), cancel: func() {}, addedAt: now.Add(2 * time.Millisecond)})
	q.heapSet[high.ID] = struct{}{}

	first := heap.Pop(&q.pq).(*heapItem)
	second := heap.Pop(&q.pq).(*heapItem)
	third := heap.Pop(&q.pq).(*heapItem)
	q.mu.Unlock()

	if first.task.ID != "prio-high" {
		t.Errorf("first should be high priority, got %s", first.task.ID)
	}
	if second.task.ID != "prio-normal" {
		t.Errorf("second should be normal priority, got %s", second.task.ID)
	}
	if third.task.ID != "prio-low" {
		t.Errorf("third should be low priority, got %s", third.task.ID)
	}
}

func TestPauseBatch(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	now := time.Now()
	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b1-t1", BatchID: "batch-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b1-t1"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b1-t2", BatchID: "batch-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b1-t2"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b2-t1", BatchID: "batch-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b2-t1"] = struct{}{}
	q.mu.Unlock()

	q.PauseBatch("batch-1")

	q.mu.Lock()
	mainLen := q.pq.Len()
	pausedLen := q.pausedPQ.Len()
	isPaused := q.paused["batch-1"]
	q.mu.Unlock()

	if mainLen != 1 {
		t.Errorf("main heap after pause: got %d, want 1", mainLen)
	}
	if pausedLen != 2 {
		t.Errorf("paused heap after pause: got %d, want 2", pausedLen)
	}
	if !isPaused {
		t.Error("batch-1 should be marked as paused")
	}
}

func TestResumeBatch(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	now := time.Now()
	q.mu.Lock()
	// Place batch-1 items directly in paused heap to simulate already-paused state.
	q.paused["batch-1"] = true
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "b1-t1", BatchID: "batch-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.pausedSet["b1-t1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "b1-t2", BatchID: "batch-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.pausedSet["b1-t2"] = struct{}{}
	// batch-2 in paused heap too (not paused, but safe from workers).
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "b2-t1", BatchID: "batch-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.pausedSet["b2-t1"] = struct{}{}
	q.mu.Unlock()

	q.ResumeBatch("batch-1")

	q.mu.Lock()
	mainLen := q.pq.Len()
	pausedLen := q.pausedPQ.Len()
	_, isPaused := q.paused["batch-1"]
	q.mu.Unlock()

	// batch-1 items (2) moved to main heap; batch-2 item stays in paused.
	// Workers may have consumed some from main, so check total is correct.
	total := mainLen + pausedLen
	if total > 3 {
		t.Errorf("total items should be <= 3, got main=%d paused=%d", mainLen, pausedLen)
	}
	if pausedLen != 1 {
		t.Errorf("paused heap should have 1 (batch-2 item), got %d", pausedLen)
	}
	if isPaused {
		t.Error("batch-1 should not be paused after resume")
	}
}

func TestPauseBatchDoesNotAffectOther(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	now := time.Now()
	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b1-t1", BatchID: "batch-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b1-t1"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b2-t1", BatchID: "batch-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b2-t1"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "b2-t2", BatchID: "batch-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	q.heapSet["b2-t2"] = struct{}{}
	q.mu.Unlock()

	q.PauseBatch("batch-1")

	q.mu.Lock()
	mainLen := q.pq.Len()
	pausedLen := q.pausedPQ.Len()
	q.mu.Unlock()

	if mainLen != 2 {
		t.Errorf("main heap: got %d, want 2 (batch-2 tasks)", mainLen)
	}
	if pausedLen != 1 {
		t.Errorf("paused heap: got %d, want 1 (batch-1 task)", pausedLen)
	}
}

func TestRecoverStaleTasks(t *testing.T) {
	q, db := setupTestQueue(t, 10, nil, nil)
	defer q.Stop()

	task1 := makeTestTask("stale-running")
	task1.Status = models.TaskStatusPending
	if err := db.CreateTask(context.Background(), task1); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), task1.ID, models.TaskStatusRunning, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	task2 := makeTestTask("stale-queued")
	task2.Status = models.TaskStatusPending
	if err := db.CreateTask(context.Background(), task2); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), task2.ID, models.TaskStatusQueued, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	task3 := makeTestTask("not-stale")
	task3.Status = models.TaskStatusPending
	if err := db.CreateTask(context.Background(), task3); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(context.Background(), task3.ID, models.TaskStatusCompleted, ""); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}

	if err := q.RecoverStaleTasks(context.Background()); err != nil {
		t.Fatalf("RecoverStaleTasks: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	got3, err := db.GetTask(context.Background(), "not-stale")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got3.Status != models.TaskStatusCompleted {
		t.Errorf("non-stale task status changed: got %q", got3.Status)
	}
}

func TestStopClearsPausedHeap(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)

	cancelDone := make(chan struct{})
	q.mu.Lock()
	heap.Push(&q.pausedPQ, &heapItem{
		task:    models.Task{ID: "paused-1", BatchID: "b1", Priority: models.PriorityNormal},
		ctx:     context.Background(),
		cancel:  func() { close(cancelDone) },
		addedAt: time.Now(),
	})
	q.pausedSet["paused-1"] = struct{}{}
	q.mu.Unlock()

	q.Stop()

	select {
	case <-cancelDone:
	case <-time.After(time.Second):
		t.Error("cancel for paused item was not called")
	}

	q.mu.Lock()
	pausedLen := q.pausedPQ.Len()
	q.mu.Unlock()
	if pausedLen != 0 {
		t.Errorf("paused heap after Stop: got %d, want 0", pausedLen)
	}
}

func TestStopClearsMainHeapTasks(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)

	cancel1Done := make(chan struct{})
	cancel2Done := make(chan struct{})
	q.mu.Lock()
	now := time.Now()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "main-stop-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() { close(cancel1Done) }, addedAt: now})
	q.heapSet["main-stop-1"] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "main-stop-2", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() { close(cancel2Done) }, addedAt: now.Add(time.Millisecond)})
	q.heapSet["main-stop-2"] = struct{}{}
	q.mu.Unlock()

	q.Stop()

	select {
	case <-cancel1Done:
	case <-time.After(time.Second):
		t.Error("cancel for main-stop-1 was not called")
	}
	select {
	case <-cancel2Done:
	case <-time.After(time.Second):
		t.Error("cancel for main-stop-2 was not called")
	}

	q.mu.Lock()
	pqLen := q.pq.Len()
	_, exists1 := q.heapSet["main-stop-1"]
	_, exists2 := q.heapSet["main-stop-2"]
	q.mu.Unlock()
	if pqLen != 0 {
		t.Errorf("main heap after Stop: got %d, want 0", pqLen)
	}
	if exists1 || exists2 {
		t.Error("heapSet entries should be removed on Stop")
	}
}

func TestCancelTaskInPausedHeap(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	task := makeTestTask("cancel-paused-1")
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	cancelCalled := false
	q.mu.Lock()
	heap.Push(&q.pausedPQ, &heapItem{
		task:    task,
		ctx:     context.Background(),
		cancel:  func() { cancelCalled = true },
		addedAt: time.Now(),
	})
	q.pausedSet[task.ID] = struct{}{}
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if !cancelCalled {
		t.Error("cancel for paused item should be called")
	}

	q.mu.Lock()
	pausedLen := q.pausedPQ.Len()
	q.mu.Unlock()

	if pausedLen != 0 {
		t.Errorf("paused heap after cancel: got %d, want 0", pausedLen)
	}
}

func TestQueueFullCountsBothHeaps(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	q.maxPending = 3

	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "m-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet["m-1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "p-1", BatchID: "b1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.pausedSet["p-1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "p-2", BatchID: "b1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.pausedSet["p-2"] = struct{}{}
	q.mu.Unlock()

	task := makeTestTask("overflow-both")
	err := q.Submit(context.Background(), task)
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull when both heaps count toward limit, got %v", err)
	}
}

func TestMetricsIncludesPausedInQueued(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: models.Task{ID: "m-1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet["m-1"] = struct{}{}
	heap.Push(&q.pausedPQ, &heapItem{task: models.Task{ID: "p-1", BatchID: "b1", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.pausedSet["p-1"] = struct{}{}
	q.mu.Unlock()

	m := q.Metrics()
	if m.Queued != 2 {
		t.Errorf("Queued should include paused: got %d, want 2", m.Queued)
	}
}

func TestDequeueRunnableLockedSkipsProxyWhenProxySlotsFull(t *testing.T) {
	q, _ := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	proxied := makeTestTask("proxy-task")
	proxied.Proxy.Server = "http://proxy.example"
	normal := makeTestTask("normal-task")
	normal.Priority = models.PriorityLow

	q.mu.Lock()
	q.proxyConcurrencyLimit = 1
	q.runningProxied = 1
	heap.Push(&q.pq, &heapItem{task: proxied, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[proxied.ID] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: normal, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[normal.ID] = struct{}{}

	item, isProxied, autoProxy := q.dequeueRunnableLocked()
	q.mu.Unlock()

	if item == nil {
		t.Fatal("expected runnable task")
	}
	if item.task.ID != normal.ID {
		t.Fatalf("expected non-proxy task to run while proxy slots are full, got %s", item.task.ID)
	}
	if isProxied {
		t.Fatal("expected dequeued task to be non-proxied")
	}
	if autoProxy {
		t.Fatal("expected dequeued task to not be auto-proxy")
	}

	q.mu.Lock()
	deferredStillQueued := q.isTaskInHeap(proxied.ID)
	q.mu.Unlock()
	if !deferredStillQueued {
		t.Fatal("expected proxied task to remain queued")
	}
}

func TestDequeueRunnableLockedTreatsAutoProxyTasksAsProxyLimited(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	pm := proxy.NewManager(db, models.ProxyPoolConfig{Strategy: models.RotationRoundRobin})
	defer pm.Stop()
	q.SetProxyManager(pm)

	autoProxyTask := makeTestTask("auto-proxy-task")
	autoProxyTask.Proxy.Geo = "US"
	autoProxyTask.Proxy.Fallback = models.ProxyFallbackStrict
	anotherAutoProxyTask := makeTestTask("auto-proxy-task-2")
	anotherAutoProxyTask.Proxy.Geo = "CA"
	anotherAutoProxyTask.Proxy.Fallback = models.ProxyFallbackAny
	anotherAutoProxyTask.Priority = models.PriorityLow

	q.mu.Lock()
	q.proxyConcurrencyLimit = 1
	q.runningProxied = 1
	heap.Push(&q.pq, &heapItem{task: autoProxyTask, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[autoProxyTask.ID] = struct{}{}
	heap.Push(&q.pq, &heapItem{task: anotherAutoProxyTask, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[anotherAutoProxyTask.ID] = struct{}{}

	item, countsAgainstProxyLimit, autoProxy := q.dequeueRunnableLocked()
	q.mu.Unlock()

	if item != nil {
		t.Fatalf("expected no runnable task while all pending tasks require proxy slots, got %s", item.task.ID)
	}
	if countsAgainstProxyLimit {
		t.Fatal("expected no selected task when proxy slots are full")
	}
	if autoProxy {
		t.Fatal("expected no selected task when proxy slots are full")
	}

	q.mu.Lock()
	stillQueued := q.isTaskInHeap(autoProxyTask.ID) && q.isTaskInHeap(anotherAutoProxyTask.ID)
	q.mu.Unlock()
	if !stillQueued {
		t.Fatal("expected auto-proxy tasks to remain queued while proxy slots are full")
	}
}

func TestDequeueRunnableLockedDoesNotAutoProxyEmptyProxyConfig(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()

	pm := proxy.NewManager(db, models.ProxyPoolConfig{Strategy: models.RotationRoundRobin})
	defer pm.Stop()
	q.SetProxyManager(pm)

	directTask := makeTestTask("direct-task")

	q.mu.Lock()
	q.proxyConcurrencyLimit = 1
	q.runningProxied = 1
	heap.Push(&q.pq, &heapItem{task: directTask, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[directTask.ID] = struct{}{}

	item, countsAgainstProxyLimit, autoProxy := q.dequeueRunnableLocked()
	q.mu.Unlock()

	if item == nil {
		t.Fatal("expected direct task to remain runnable without proxy routing hints")
	}
	if item.task.ID != directTask.ID {
		t.Fatalf("expected direct task, got %s", item.task.ID)
	}
	if countsAgainstProxyLimit {
		t.Fatal("expected direct task to bypass proxy concurrency limit")
	}
	if autoProxy {
		t.Fatal("expected empty proxy config to mean direct connection, not auto proxy")
	}
}
