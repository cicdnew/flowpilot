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

type EventCallback func(event models.TaskEvent)

// ErrQueueFull is returned when the pending queue has reached its maximum size.
var ErrQueueFull = fmt.Errorf("queue is full: too many pending tasks")

type Queue struct {
	db             *database.DB
	runner         *browser.Runner
	proxyManager   *proxy.Manager
	sem            *semaphore.Weighted
	maxConcurrency int64
	maxPending     int
	onEvent        EventCallback

	mu        sync.Mutex
	running   map[string]context.CancelFunc
	pending   map[string]context.CancelFunc
	cancelled map[string]bool
	stopped   bool
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func New(db *database.DB, runner *browser.Runner, maxConcurrency int, onEvent EventCallback) *Queue {
	return &Queue{
		db:             db,
		runner:         runner,
		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
		maxConcurrency: int64(maxConcurrency),
		maxPending:     maxConcurrency * 10, // default: 10x concurrency limit
		onEvent:        onEvent,
		running:        make(map[string]context.CancelFunc),
		pending:        make(map[string]context.CancelFunc),
		cancelled:      make(map[string]bool),
		stopCh:         make(chan struct{}),
	}
}

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
	q.mu.Unlock()

	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusQueued, ""); err != nil {
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

func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
	for _, task := range tasks {
		if err := q.Submit(ctx, task); err != nil {
			return fmt.Errorf("submit task %s: %w", task.ID, err)
		}
	}
	return nil
}

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

	if err := q.db.UpdateTaskStatus(taskID, models.TaskStatusCancelled, "cancelled by user"); err != nil {
		return fmt.Errorf("update task status to cancelled: %w", err)
	}
	q.emitEvent(taskID, models.TaskStatusCancelled, "cancelled by user")
	return nil
}

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

func (q *Queue) RunningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running)
}

func (q *Queue) executeTask(ctx context.Context, task models.Task) {
	if err := q.sem.Acquire(ctx, 1); err != nil {
		q.mu.Lock()
		delete(q.pending, task.ID)
		wasCancelled := q.cancelled[task.ID]
		q.mu.Unlock()
		if !wasCancelled {
			q.emitEvent(task.ID, models.TaskStatusFailed, "failed to acquire queue slot")
		}
		return
	}
	defer q.sem.Release(1)

	q.mu.Lock()
	delete(q.pending, task.ID)
	if q.stopped || q.cancelled[task.ID] {
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

	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusRunning, ""); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusRunning, "")

	result, err := q.runner.RunTask(taskCtx, task)

	if selectedProxyID != "" {
		if pm := q.getProxyManager(); pm != nil {
			if recordErr := pm.RecordUsage(selectedProxyID, err == nil); recordErr != nil {
				q.emitEvent(task.ID, task.Status, fmt.Sprintf("proxy usage recording failed: %v", recordErr))
			}
		}
	}

	if err != nil {
		q.handleFailure(ctx, task, err)
		return
	}

	q.handleSuccess(task, result)
}

func (q *Queue) handleFailure(parentCtx context.Context, task models.Task, execErr error) {
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

		timer := time.NewTimer(backoff)
		select {
		case <-timer.C:
		case <-q.stopCh:
			timer.Stop()
			if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCancelled, "cancelled during retry backoff (queue stopped)"); err != nil {
				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
			}
			return
		case <-parentCtx.Done():
			timer.Stop()
			if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCancelled, "cancelled during retry backoff"); err != nil {
				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
			}
			return
		}

		q.mu.Lock()
		if q.stopped {
			q.mu.Unlock()
			if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusCancelled, "cancelled during retry (queue stopped)"); err != nil {
				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
			}
			return
		}

		retryTask := task
		retryTask.RetryCount++
		retryTask.Status = models.TaskStatusPending
		retryTask.Steps = make([]models.TaskStep, len(task.Steps))
		copy(retryTask.Steps, task.Steps)

		retryCtx, retryCancel := context.WithCancel(parentCtx)
		q.pending[retryTask.ID] = retryCancel
		q.mu.Unlock()

		go q.executeTask(retryCtx, retryTask)
		return
	}

	if err := q.db.UpdateTaskStatus(task.ID, models.TaskStatusFailed, execErr.Error()); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusFailed, execErr.Error())
}

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
