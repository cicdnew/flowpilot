package queue

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

// assertCounts checks the lifetime failed/cancelled counters.
func assertCounts(t *testing.T, q *Queue, wantFailed, wantCancelled int64) {
	t.Helper()
	m := q.Metrics()
	if m.TotalFailed != wantFailed {
		t.Errorf("TotalFailed: got %d, want %d", m.TotalFailed, wantFailed)
	}
	if m.TotalCancelled != wantCancelled {
		t.Errorf("TotalCancelled: got %d, want %d", m.TotalCancelled, wantCancelled)
	}
}

func assertStatus(t *testing.T, db *database.DB, taskID string, want models.TaskStatus) {
	t.Helper()
	got, err := db.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != want {
		t.Errorf("status: got %q, want %q", got.Status, want)
	}
}

func createTask(t *testing.T, db *database.DB, task models.Task) {
	t.Helper()
	if err := db.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
}

// A running task cancelled by the user whose runner returns context.Canceled is a cancellation, not a failure.
func TestCancelMetricsRunningTaskCountsAsCancelled(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("cm-running")
	createTask(t, db, task)
	markRunning(q, task.ID)
	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	if !q.handleTaskCancellation(context.Background(), task.ID, context.Canceled) {
		t.Fatal("expected context.Canceled to be handled as a cancellation")
	}

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

// Cancelling a running task must be counted once in total: Cancel defers the
// count to the worker so the two paths do not both count it.
func TestCancelMetricsRunningTaskNotDoubleCounted(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()
	task := makeTestTask("cm-running-once")
	createTask(t, db, task)
	q.mu.Lock()
	q.running[task.ID] = func() {}
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	assertCounts(t, q, 0, 0) // worker has not observed the cancellation yet

	q.handleTaskCancellation(context.Background(), task.ID, context.Canceled)
	assertCounts(t, q, 0, 1)
	if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != models.TaskStatusCancelled {
		t.Errorf("terminal events: got %v, want [cancelled]", got)
	}
}

// A queued task never reaches a worker, so Cancel itself must count it, and a
// repeated Cancel of the same task must not count it again.
func TestCancelMetricsQueuedTaskCountedOnce(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("cm-queued")
	createTask(t, db, task)
	q.mu.Lock()
	heap.Push(&q.pq, &heapItem{task: task, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	q.heapSet[task.ID] = heapItemByID(q.pq, task.ID)
	q.mu.Unlock()

	for i := 0; i < 2; i++ {
		if err := q.Cancel(task.ID); err != nil {
			t.Fatalf("Cancel #%d: %v", i+1, err)
		}
	}

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

// Regression: a task cancelled during its final attempt, whose runner returns an
// error that does not wrap context.Canceled, used to be overwritten as failed.
func TestCancelMetricsLastAttemptNotOverwrittenAsFailed(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	task := makeTestTask("cm-last-attempt")
	task.RetryCount = 2
	task.MaxRetries = 2
	createTask(t, db, task)
	q.mu.Lock()
	q.running[task.ID] = func() {}
	q.mu.Unlock()

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ri := q.handleFailure(context.Background(), task, fmt.Errorf("websocket closed"), nil); ri.shouldRetry {
		t.Fatal("cancelled task must not be retried")
	}

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

// A task cancelled during its retry backoff is counted by Cancel exactly once.
func TestCancelMetricsDuringRetryBackoff(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	q.maxPending = 10
	task := makeTestTask("cm-backoff")
	task.MaxRetries = 1
	createTask(t, db, task)

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("transient"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected a retry to be scheduled")
	}
	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	ri.backoff = time.Millisecond
	q.scheduleRetry(context.Background(), ri)

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

// Stopping the queue while a task waits in retry backoff cancels and counts it.
func TestCancelMetricsQueueStopDuringRetryBackoff(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	task := makeTestTask("cm-stop-backoff")
	task.MaxRetries = 1
	createTask(t, db, task)

	ri := q.handleFailure(context.Background(), task, fmt.Errorf("transient"), nil)
	if !ri.shouldRetry {
		t.Fatal("expected a retry to be scheduled")
	}
	q.Stop()
	ri.backoff = time.Hour // only stopCh can fire
	q.scheduleRetry(context.Background(), ri)

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	assertCounts(t, q, 0, 1)
}

// terminalEvents returns the terminal statuses emitted for a task, in order.
func terminalEvents(events []models.TaskEvent, mu *sync.Mutex, taskID string) []models.TaskStatus {
	mu.Lock()
	defer mu.Unlock()
	var out []models.TaskStatus
	for _, e := range events {
		if e.TaskID != taskID {
			continue
		}
		switch e.Status {
		case models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled:
			out = append(out, e.Status)
		}
	}
	return out
}

func markRunning(q *Queue, taskID string) {
	q.mu.Lock()
	q.running[taskID] = func() {}
	q.mu.Unlock()
}

func successResult(taskID string) *models.TaskResult {
	return &models.TaskResult{TaskID: taskID, Success: true, Duration: 10 * time.Millisecond}
}

// Cancel claims a running task first; the runner then reports success. The
// task must stay cancelled: no completed result, event, or count.
func TestCancelBeatsSuccess(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()
	task := makeTestTask("race-cancel-first")
	createTask(t, db, task)
	markRunning(q, task.ID)

	if err := q.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	q.handleSuccess(context.Background(), task, successResult(task.ID))

	assertStatus(t, db, task.ID, models.TaskStatusCancelled)
	if got := q.Metrics().TotalCompleted; got != 0 {
		t.Errorf("TotalCompleted: got %d, want 0", got)
	}
	assertCounts(t, q, 0, 1)
	if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != models.TaskStatusCancelled {
		t.Errorf("terminal events: got %v, want [cancelled]", got)
	}
}

// The worker claims completion first; a Cancel arriving while it finalizes is
// a no-op: the task stays completed and no cancelled event is emitted.
func TestSuccessBeatsCancel(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()
	task := makeTestTask("race-success-first")
	createTask(t, db, task)
	markRunning(q, task.ID)

	q.handleSuccess(context.Background(), task, successResult(task.ID))
	if err := q.Cancel(task.ID); err != nil { // still finalizing: finishExecuteTask not yet run
		t.Fatalf("Cancel: %v", err)
	}

	assertStatus(t, db, task.ID, models.TaskStatusCompleted)
	if got := q.Metrics().TotalCompleted; got != 1 {
		t.Errorf("TotalCompleted: got %d, want 1", got)
	}
	assertCounts(t, q, 0, 0)
	if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != models.TaskStatusCompleted {
		t.Errorf("terminal events: got %v, want [completed]", got)
	}
}

// A Cancel arriving after the worker fully finished (finalizing released) must
// not rewrite a completed or failed task as cancelled.
func TestLateCancelLeavesFinishedTaskUntouched(t *testing.T) {
	for _, tc := range []struct {
		name string
		want models.TaskStatus
	}{
		{"completed", models.TaskStatusCompleted},
		{"failed", models.TaskStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []models.TaskEvent
			var mu sync.Mutex
			q, db := setupTestQueueNoWorkers(t, &events, &mu)
			defer q.Stop()
			task := makeTestTask("late-cancel-" + tc.name)
			task.RetryCount, task.MaxRetries = 0, 0
			createTask(t, db, task)
			markRunning(q, task.ID)

			if tc.want == models.TaskStatusCompleted {
				q.handleSuccess(context.Background(), task, successResult(task.ID))
			} else {
				q.handleFailure(context.Background(), task, fmt.Errorf("boom"), nil)
			}
			proxied := false
			q.finishExecuteTask(task.ID, &proxied)

			if err := q.Cancel(task.ID); err != nil {
				t.Fatalf("Cancel: %v", err)
			}

			assertStatus(t, db, task.ID, tc.want)
			if got := terminalEvents(events, &mu, task.ID); len(got) != 1 || got[0] != tc.want {
				t.Errorf("terminal events: got %v, want [%s]", got, tc.want)
			}
		})
	}
}

// Success and Cancel race for real on many tasks. Whichever wins, every task
// must end with exactly one terminal event that matches its stored status, and
// the counters must agree with the stored statuses.
func TestSuccessCancelRaceIsConsistent(t *testing.T) {
	var events []models.TaskEvent
	var mu sync.Mutex
	q, db := setupTestQueueNoWorkers(t, &events, &mu)
	defer q.Stop()
	const n = 100
	tasks := make([]models.Task, n)
	for i := range tasks {
		tasks[i] = makeTestTask(fmt.Sprintf("race-%d", i))
		createTask(t, db, tasks[i])
		markRunning(q, tasks[i].ID)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, task := range tasks {
		task := task
		taskCtx, cancel := context.WithCancel(context.Background())
		q.setTaskCancel(task.ID, cancel)
		wg.Add(2)
		go func() {
			defer wg.Done()
			defer cancel()
			proxied := false
			defer q.finishExecuteTask(task.ID, &proxied)
			<-start
			q.handleSuccess(taskCtx, task, successResult(task.ID))
		}()
		go func() {
			defer wg.Done()
			<-start
			if err := q.Cancel(task.ID); err != nil {
				t.Errorf("Cancel %s: %v", task.ID, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	var completed, cancelled int64
	for _, task := range tasks {
		got, err := db.GetTask(context.Background(), task.ID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		switch got.Status {
		case models.TaskStatusCompleted:
			completed++
		case models.TaskStatusCancelled:
			cancelled++
		default:
			t.Errorf("%s: unexpected status %q", task.ID, got.Status)
		}
		if ev := terminalEvents(events, &mu, task.ID); len(ev) != 1 || ev[0] != got.Status {
			t.Errorf("%s: terminal events %v do not match stored status %q", task.ID, ev, got.Status)
		}
	}
	m := q.Metrics()
	if m.TotalCompleted != completed || m.TotalCancelled != cancelled || m.TotalFailed != 0 {
		t.Errorf("metrics completed=%d cancelled=%d failed=%d, stored completed=%d cancelled=%d",
			m.TotalCompleted, m.TotalCancelled, m.TotalFailed, completed, cancelled)
	}
	t.Logf("outcomes: %d completed, %d cancelled", completed, cancelled)
}

// When Cancel and queue shutdown are both ready, select picks a branch at
// random. Every task must still be counted exactly once.
func TestCancelMetricsCancelAndStopRaceCountedOnce(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	const n = 64
	retries := make([]retryInfo, 0, n)
	for i := 0; i < n; i++ {
		task := makeTestTask(fmt.Sprintf("cm-race-%d", i))
		task.MaxRetries = 1
		createTask(t, db, task)
		ri := q.handleFailure(context.Background(), task, fmt.Errorf("transient"), nil)
		if !ri.shouldRetry {
			t.Fatalf("task %d: expected a retry to be scheduled", i)
		}
		if err := q.Cancel(task.ID); err != nil {
			t.Fatalf("Cancel %d: %v", i, err)
		}
		ri.backoff = time.Hour
		retries = append(retries, ri)
	}
	q.Stop() // stopCh and every ri.cancelled are now ready together

	for _, ri := range retries {
		q.scheduleRetry(context.Background(), ri)
	}

	assertCounts(t, q, 0, n)
}
