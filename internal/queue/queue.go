package queue

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/database"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"

	"golang.org/x/sync/semaphore"
)

// EventCallback is invoked when a task's status changes.
type EventCallback func(event models.TaskEvent)

// ErrQueueFull is returned when the pending queue has reached its maximum size.
var ErrQueueFull = errors.New("queue is full: too many pending tasks")

// Queue manages task scheduling, concurrency limiting, and execution.
type Queue struct {
	db             *database.DB
	runner         *browser.Runner
	proxyManager   *proxy.Manager
	sem            *semaphore.Weighted
	maxConcurrency int64
	maxPending     int
	onEvent        EventCallback
	metrics        models.QueueMetrics

	mu        sync.Mutex
	running   map[string]context.CancelFunc
	pending   map[string]context.CancelFunc
	cancelled map[string]bool
	stopped   bool
	stopOnce  sync.Once
	stopCh    chan struct{}
}

// New creates a Queue with the given concurrency limit and event callback.
func New(db *database.DB, runner *browser.Runner, maxConcurrency int, onEvent EventCallback) *Queue {
	return &Queue{
		db:             db,
		runner:         runner,
		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
		maxConcurrency: int64(maxConcurrency),
		maxPending:     maxConcurrency * 10, // default: 10x concurrency limit
		onEvent:        onEvent,
		metrics:        models.QueueMetrics{},
		running:        make(map[string]context.CancelFunc),
		pending:        make(map[string]context.CancelFunc),
		cancelled:      make(map[string]bool),
		stopCh:         make(chan struct{}),
	}
}

// SetProxyManager attaches a proxy manager for automatic proxy selection.
func (q *Queue) SetProxyManager(pm *proxy.Manager) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.proxyManager = pm
}

func (q *Queue) getProxyManager() *proxy.Manager {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.proxyManager
}

// Submit enqueues a task for execution. Returns ErrQueueFull if at capacity.
func (q *Queue) Submit(ctx context.Context, task models.Task) error {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return fmt.Errorf("queue is stopped")
	}
	if _, ok := q.running[task.ID]; ok {
		q.mu.Unlock()
		return fmt.Errorf("task %s is already running", task.ID)
	}
	if _, ok := q.pending[task.ID]; ok {
		q.mu.Unlock()
		return fmt.Errorf("task %s is already pending", task.ID)
	}
	if q.maxPending > 0 && len(q.pending) >= q.maxPending {
		q.mu.Unlock()
		return ErrQueueFull
	}

	taskCtx, cancel := context.WithCancel(ctx)
	q.pending[task.ID] = cancel
	q.metrics.TotalSubmitted++
	q.mu.Unlock()

	if err := q.db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusQueued, ""); err != nil {
		q.mu.Lock()
		delete(q.pending, task.ID)
		q.mu.Unlock()
		cancel()
		return fmt.Errorf("update task status to queued: %w", err)
	}
	q.emitEvent(task.ID, models.TaskStatusQueued, "")

	go q.executeTask(taskCtx, task)
	return nil
}

// SubmitBatch enqueues multiple tasks. Stops on first error.
func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
	for _, task := range tasks {
		if err := q.Submit(ctx, task); err != nil {
			return fmt.Errorf("submit task %s: %w", task.ID, err)
		}
	}
	return nil
}

// Cancel stops a running or pending task and marks it as cancelled.
func (q *Queue) Cancel(taskID string) error {
	q.mu.Lock()

	if cancel, ok := q.running[taskID]; ok {
		q.cancelled[taskID] = true
		cancel()
		delete(q.running, taskID)
		q.mu.Unlock()
	} else if cancel, ok := q.pending[taskID]; ok {
		q.cancelled[taskID] = true
		cancel()
		delete(q.pending, taskID)
		q.mu.Unlock()
	} else {
		q.mu.Unlock()
	}

	if err := q.db.UpdateTaskStatus(context.Background(), taskID, models.TaskStatusCancelled, "cancelled by user"); err != nil {
		return fmt.Errorf("update task status to cancelled: %w", err)
	}
	q.emitEvent(taskID, models.TaskStatusCancelled, "cancelled by user")
	return nil
}

// Stop cancels all running and pending tasks and prevents new submissions.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopped = true
		for id, cancel := range q.running {
			cancel()
			delete(q.running, id)
		}
		for id, cancel := range q.pending {
			cancel()
			delete(q.pending, id)
		}
		q.mu.Unlock()
		close(q.stopCh)
	})
}

// RunningCount returns the number of currently executing tasks.
func (q *Queue) RunningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running)
}

// Metrics returns a snapshot of queue metrics.
// Queued = tasks waiting for a concurrency slot.
// Running = tasks currently executing.
// Pending = Queued + Running (total in-flight).
func (q *Queue) Metrics() models.QueueMetrics {
	q.mu.Lock()
	defer q.mu.Unlock()
	metrics := q.metrics
	metrics.Running = len(q.running)
	metrics.Queued = len(q.pending)
	metrics.Pending = len(q.pending) + len(q.running)
	return metrics
}

type retryInfo struct {
	shouldRetry bool
	task        models.Task
	backoff     time.Duration
	parentCtx   context.Context
}

func (q *Queue) executeTask(ctx context.Context, task models.Task) {
	if err := q.sem.Acquire(ctx, 1); err != nil {
		q.mu.Lock()
		delete(q.pending, task.ID)
		wasCancelled := q.cancelled[task.ID]
		delete(q.cancelled, task.ID)
		q.metrics.TotalFailed++
		q.mu.Unlock()
		if !wasCancelled {
			_ = q.db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusFailed, "failed to acquire queue slot")
			q.emitEvent(task.ID, models.TaskStatusFailed, "failed to acquire queue slot")
		}
		return
	}
	defer q.sem.Release(1)

	q.mu.Lock()
	delete(q.pending, task.ID)
	if q.stopped || q.cancelled[task.ID] {
		delete(q.cancelled, task.ID)
		q.mu.Unlock()
		return
	}

	taskTimeout := 5 * time.Minute
	if task.Timeout > 0 {
		taskTimeout = time.Duration(task.Timeout) * time.Second
	}
	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)

	q.running[task.ID] = cancel
	q.mu.Unlock()

	defer func() {
		cancel()
		q.mu.Lock()
		delete(q.running, task.ID)
		delete(q.cancelled, task.ID)
		q.mu.Unlock()
	}()

	var selectedProxyID string
	pm := q.getProxyManager()
	if task.Proxy.Server == "" && pm != nil {
		if p, err := pm.SelectProxy(task.Proxy.Geo); err == nil {
			selectedProxyID = p.ID
			task.Proxy = p.ToProxyConfig()
		}
	}

	if err := q.db.UpdateTaskStatus(taskCtx, task.ID, models.TaskStatusRunning, ""); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusRunning, "")

	result, err := q.runner.RunTask(taskCtx, task)

	if result != nil && len(result.StepLogs) > 0 {
		if slErr := q.db.InsertStepLogs(ctx, task.ID, result.StepLogs); slErr != nil {
			q.emitEvent(task.ID, task.Status, fmt.Sprintf("persist step logs: %v", slErr))
		}
	}

	if result != nil && len(result.NetworkLogs) > 0 {
		if nlErr := q.db.InsertNetworkLogs(ctx, task.ID, result.NetworkLogs); nlErr != nil {
			q.emitEvent(task.ID, task.Status, fmt.Sprintf("persist network logs: %v", nlErr))
		}
	}

	if selectedProxyID != "" {
		if pm := q.getProxyManager(); pm != nil {
			if recordErr := pm.RecordUsage(selectedProxyID, err == nil); recordErr != nil {
				q.emitEvent(task.ID, task.Status, fmt.Sprintf("proxy usage recording failed: %v", recordErr))
			}
		}
	}

	var retry retryInfo
	if err != nil {
		retry = q.handleFailure(ctx, task, err)
	} else {
		q.handleSuccess(task, result)
	}

	if retry.shouldRetry {
		go q.scheduleRetry(retry)
	}
}

func (q *Queue) handleFailure(parentCtx context.Context, task models.Task, execErr error) retryInfo {
	if task.RetryCount < task.MaxRetries {
		if err := q.db.IncrementRetry(context.Background(), task.ID); err != nil {
			q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("increment retry: %v", err))
			return retryInfo{}
		}
		q.emitEvent(task.ID, models.TaskStatusRetrying, execErr.Error())

		backoffSec := math.Pow(2, float64(task.RetryCount))
		if backoffSec > 60 {
			backoffSec = 60
		}
		backoff := time.Duration(backoffSec) * time.Second

		retryTask := task
		retryTask.RetryCount++
		retryTask.Status = models.TaskStatusPending
		retryTask.Steps = make([]models.TaskStep, len(task.Steps))
		copy(retryTask.Steps, task.Steps)

		return retryInfo{
			shouldRetry: true,
			task:        retryTask,
			backoff:     backoff,
			parentCtx:   parentCtx,
		}
	}

	if err := q.db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusFailed, execErr.Error()); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return retryInfo{}
	}
	q.mu.Lock()
	q.metrics.TotalFailed++
	q.mu.Unlock()
	q.emitEvent(task.ID, models.TaskStatusFailed, execErr.Error())
	return retryInfo{}
}

func (q *Queue) scheduleRetry(ri retryInfo) {
	timer := time.NewTimer(ri.backoff)
	select {
	case <-timer.C:
	case <-q.stopCh:
		timer.Stop()
		if err := q.db.UpdateTaskStatus(context.Background(), ri.task.ID, models.TaskStatusCancelled, "cancelled during retry backoff (queue stopped)"); err != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		}
		return
	case <-ri.parentCtx.Done():
		timer.Stop()
		if err := q.db.UpdateTaskStatus(context.Background(), ri.task.ID, models.TaskStatusCancelled, "cancelled during retry backoff"); err != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		}
		return
	}

	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		if err := q.db.UpdateTaskStatus(context.Background(), ri.task.ID, models.TaskStatusCancelled, "cancelled during retry (queue stopped)"); err != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		}
		return
	}

	retryCtx, retryCancel := context.WithCancel(ri.parentCtx)
	q.pending[ri.task.ID] = retryCancel
	q.mu.Unlock()

	q.executeTask(retryCtx, ri.task)
}

func (q *Queue) handleSuccess(task models.Task, result *models.TaskResult) {
	if err := q.db.UpdateTaskResult(context.Background(), task.ID, *result); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("save result: %v", err))
		return
	}
	if err := q.db.UpdateTaskStatus(context.Background(), task.ID, models.TaskStatusCompleted, ""); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.mu.Lock()
	q.metrics.TotalCompleted++
	q.mu.Unlock()
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
