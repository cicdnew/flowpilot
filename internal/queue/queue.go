package queue

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"web-automation/internal/browser"
	"web-automation/internal/database"
	"web-automation/internal/models"
	"web-automation/internal/proxy"

	"golang.org/x/sync/semaphore"
)

// EventCallback is called when a task event occurs.
type EventCallback func(event models.TaskEvent)

// Queue manages task execution with concurrency control.
type Queue struct {
	db             *database.DB
	runner         *browser.Runner
	proxyManager   *proxy.Manager
	sem            *semaphore.Weighted
	maxConcurrency int64
	onEvent        EventCallback

	mu       sync.Mutex
	running  map[string]context.CancelFunc // taskID -> cancel
	stopped  bool
	stopOnce sync.Once
	stopCh   chan struct{}
}

// New creates a new task queue.
func New(db *database.DB, runner *browser.Runner, maxConcurrency int, onEvent EventCallback) *Queue {
	return &Queue{
		db:             db,
		runner:         runner,
		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
		maxConcurrency: int64(maxConcurrency),
		onEvent:        onEvent,
		running:        make(map[string]context.CancelFunc),
		stopCh:         make(chan struct{}),
	}
}

// SetProxyManager attaches a proxy manager for pool-based proxy selection.
func (q *Queue) SetProxyManager(pm *proxy.Manager) {
	q.proxyManager = pm
}

// Submit adds a task to the queue and starts executing it.
func (q *Queue) Submit(ctx context.Context, task models.Task) error {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return fmt.Errorf("queue is stopped")
	}
	q.mu.Unlock()

	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusQueued, ""); err != nil {
		return fmt.Errorf("update task status to queued: %w", err)
	}
	q.emitEvent(task.ID, models.TaskStatusQueued, "")

	go q.executeTask(ctx, task)
	return nil
}

// SubmitBatch submits multiple tasks.
func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
	for _, task := range tasks {
		if err := q.Submit(ctx, task); err != nil {
			return fmt.Errorf("submit task %s: %w", task.ID, err)
		}
	}
	return nil
}

// Cancel cancels a running task.
func (q *Queue) Cancel(taskID string) error {
	q.mu.Lock()
	cancel, ok := q.running[taskID]
	q.mu.Unlock()

	if ok {
		cancel()
	}

	if err := q.db.UpdateTaskStatus(taskID, models.TaskStatusCancelled, "cancelled by user"); err != nil {
		return fmt.Errorf("update task status to cancelled: %w", err)
	}
	return nil
}

// Stop stops all running tasks and prevents new submissions.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopped = true
		for id, cancel := range q.running {
			cancel()
			delete(q.running, id)
		}
		q.mu.Unlock()
		close(q.stopCh)
	})
}

// RunningCount returns how many tasks are currently running.
func (q *Queue) RunningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running)
}

func (q *Queue) executeTask(ctx context.Context, task models.Task) {
	if err := q.sem.Acquire(ctx, 1); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, "failed to acquire queue slot")
		return
	}
	defer q.sem.Release(1)

	taskTimeout := 5 * time.Minute
	if task.Timeout > 0 {
		taskTimeout = time.Duration(task.Timeout) * time.Second
	}
	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	q.mu.Lock()
	q.running[task.ID] = cancel
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		delete(q.running, task.ID)
		q.mu.Unlock()
	}()

	if task.Proxy.Server == "" && q.proxyManager != nil {
		if p, err := q.proxyManager.SelectProxy(task.Proxy.Geo); err == nil {
			task.Proxy = p.ToProxyConfig()
		}
	}

	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusRunning, ""); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusRunning, "")

	result, err := q.runner.RunTask(taskCtx, task)

	if task.Proxy.Server != "" && q.proxyManager != nil {
		_ = q.proxyManager.RecordUsage(task.ID, err == nil)
	}

	if err != nil {
		q.handleFailure(ctx, taskCtx, task, err)
		return
	}

	q.handleSuccess(task, result)
}

// handleFailure processes a task execution error, retrying if allowed.
func (q *Queue) handleFailure(parentCtx, taskCtx context.Context, task models.Task, execErr error) {
	if task.RetryCount < task.MaxRetries {
		if err := q.db.IncrementRetry(task.ID); err != nil {
			q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("increment retry: %v", err))
			return
		}
		q.emitEvent(task.ID, models.TaskStatusRetrying, execErr.Error())

		backoffSec := math.Pow(2, float64(task.RetryCount))
		if backoffSec > 60 {
			backoffSec = 60
		}
		backoff := time.Duration(backoffSec) * time.Second

		// Wait for backoff respecting context cancellation.
		select {
		case <-time.After(backoff):
		case <-taskCtx.Done():
			if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCancelled, "cancelled during retry backoff"); err != nil {
				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
			}
			return
		}

		// Retry: create a copy with incremented count and re-submit.
		retryTask := task
		retryTask.RetryCount++
		retryTask.Status = models.TaskStatusPending
		// Deep copy steps to avoid shared slice mutation.
		retryTask.Steps = make([]models.TaskStep, len(task.Steps))
		copy(retryTask.Steps, task.Steps)

		go q.executeTask(parentCtx, retryTask)
		return
	}

	// Max retries exceeded.
	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusFailed, execErr.Error()); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusFailed, execErr.Error())
}

// handleSuccess records a successful task result.
func (q *Queue) handleSuccess(task models.Task, result *models.TaskResult) {
	if err := q.db.UpdateTaskResult(task.ID, *result); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("save result: %v", err))
		return
	}
	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCompleted, ""); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusCompleted, "")
}

func (q *Queue) emitEvent(taskID string, status models.TaskStatus, errMsg string) {
	if q.onEvent != nil {
		q.onEvent(models.TaskEvent{
			TaskID: taskID,
			Status: status,
			Error:  errMsg,
		})
	}
}
