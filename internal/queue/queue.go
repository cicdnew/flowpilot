package queue

import (
	"container/heap"
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
)

// EventCallback is invoked when a task's status changes.
type EventCallback func(event models.TaskEvent)

// ErrQueueFull is returned when the pending queue has reached its maximum size.
var ErrQueueFull = errors.New("queue is full: too many pending tasks")

// ErrBatchPaused is returned when attempting to submit to a paused batch.
var ErrBatchPaused = errors.New("batch is paused")

// Queue manages task scheduling, concurrency limiting, and execution using
// a fixed worker pool with a priority heap. Higher-priority tasks are
// dequeued first; within the same priority level, tasks are processed FIFO.
type taskStateWrite struct {
	change database.TaskStateChange
}

type Queue struct {
	db                    *database.DB
	runner                *browser.Runner
	proxyManager          *proxy.Manager
	workerCount           int
	maxPending            int
	onEvent               EventCallback
	metrics               models.QueueMetrics
	proxyConcurrencyLimit int
	persistenceBatchSize  int
	persistenceInterval   time.Duration
	persistenceCh         chan taskStateWrite
	persistenceWg         sync.WaitGroup

	mu             sync.Mutex
	cond           *sync.Cond
	pq             taskHeap            // main priority queue
	pausedPQ       taskHeap            // tasks from paused batches
	heapSet        map[string]struct{} // O(1) lookup for pq
	pausedSet      map[string]struct{} // O(1) lookup for pausedPQ
	running        map[string]context.CancelFunc
	cancelled      map[string]bool
	paused         map[string]bool // batchID → paused
	runningProxied int
	stopped        bool
	stopOnce       sync.Once
	stopCh         chan struct{}
	workerWg       sync.WaitGroup
}

// New creates a Queue with the given concurrency limit and event callback.
// It spawns workerCount fixed workers with a staggered 100ms warm-up delay.
func New(db *database.DB, runner *browser.Runner, maxConcurrency int, onEvent EventCallback) *Queue {
	q := &Queue{
		db:                   db,
		runner:               runner,
		workerCount:          maxConcurrency,
		maxPending:           maxConcurrency * 10, // default: 10x concurrency limit
		onEvent:              onEvent,
		metrics:              models.QueueMetrics{},
		persistenceBatchSize: max(16, maxConcurrency),
		persistenceInterval:  100 * time.Millisecond,
		persistenceCh:        make(chan taskStateWrite, max(64, maxConcurrency*4)),
		pq:                   make(taskHeap, 0),
		pausedPQ:             make(taskHeap, 0),
		heapSet:              make(map[string]struct{}, maxConcurrency*10),
		pausedSet:            make(map[string]struct{}, maxConcurrency*10),
		running:              make(map[string]context.CancelFunc),
		cancelled:            make(map[string]bool),
		paused:               make(map[string]bool),
		stopCh:               make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	heap.Init(&q.pq)
	heap.Init(&q.pausedPQ)

	q.persistenceWg.Add(1)
	go q.persistenceWorker()

	// Start fixed worker pool with staggered warm-up.
	for i := 0; i < maxConcurrency; i++ {
		q.workerWg.Add(1)
		workerID := i
		go func() {
			if workerID > 0 {
				stagger := time.Duration(workerID) * 50 * time.Millisecond
				if stagger > 2*time.Second {
					stagger = 2 * time.Second
				}
				time.Sleep(stagger)
			}
			q.worker(workerID)
		}()
	}

	return q
}

// SetProxyManager attaches a proxy manager for automatic proxy selection.
func (q *Queue) SetProxyManager(pm *proxy.Manager) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.proxyManager = pm
}

func (q *Queue) SetProxyConcurrencyLimit(limit int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.proxyConcurrencyLimit = limit
	q.cond.Broadcast()
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
	if q.isTaskEnqueued(task.ID) {
		q.mu.Unlock()
		return fmt.Errorf("task %s is already running", task.ID)
	}
	if q.isTaskInHeap(task.ID) {
		q.mu.Unlock()
		return fmt.Errorf("task %s is already pending", task.ID)
	}
	if q.maxPending > 0 && q.pq.Len()+q.pausedPQ.Len() >= q.maxPending {
		q.mu.Unlock()
		return ErrQueueFull
	}

	taskCtx, cancel := context.WithCancel(ctx)
	item := &heapItem{
		task:    task,
		ctx:     taskCtx,
		cancel:  cancel,
		addedAt: time.Now(),
	}
	heap.Push(&q.pq, item)
	q.heapSet[item.task.ID] = struct{}{}
	q.metrics.TotalSubmitted++
	q.mu.Unlock()

	if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusQueued}); err != nil {
		q.mu.Lock()
		q.removeFromHeap(task.ID)
		q.mu.Unlock()
		cancel()
		return fmt.Errorf("update task status to queued: %w", err)
	}
	q.emitEvent(task.ID, models.TaskStatusQueued, "")

	// Signal one worker that a task is available.
	q.cond.Signal()
	return nil
}

// SubmitBatch enqueues multiple tasks with a single lock acquisition and DB transaction.
func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return fmt.Errorf("queue is stopped")
	}

	if q.maxPending > 0 && q.pq.Len()+q.pausedPQ.Len()+len(tasks) > q.maxPending {
		q.mu.Unlock()
		return ErrQueueFull
	}

	added := make([]models.Task, 0, len(tasks))
	for _, task := range tasks {
		if q.isTaskEnqueued(task.ID) || q.isTaskInHeap(task.ID) {
			continue
		}
		taskCtx, cancel := context.WithCancel(ctx)
		item := &heapItem{
			task:    task,
			ctx:     taskCtx,
			cancel:  cancel,
			addedAt: time.Now(),
		}
		heap.Push(&q.pq, item)
		q.heapSet[item.task.ID] = struct{}{}
		q.metrics.TotalSubmitted++
		added = append(added, task)
	}
	q.mu.Unlock()

	if len(added) == 0 {
		return nil
	}

	taskIDs := make([]string, len(added))
	for i, t := range added {
		taskIDs[i] = t.ID
	}
	changes := make([]database.TaskStateChange, 0, len(taskIDs))
	for _, id := range taskIDs {
		changes = append(changes, database.TaskStateChange{TaskID: id, Status: models.TaskStatusQueued})
	}
	if err := q.enqueueTaskStateChanges(changes); err != nil {
		q.mu.Lock()
		for _, t := range added {
			q.removeFromHeap(t.ID)
		}
		q.mu.Unlock()
		return fmt.Errorf("batch update task status: %w", err)
	}

	for _, t := range added {
		q.emitEvent(t.ID, models.TaskStatusQueued, "")
	}
	q.cond.Broadcast()
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
	} else if q.removeFromHeap(taskID) {
		q.cancelled[taskID] = true
		q.mu.Unlock()
	} else if q.removeFromPausedHeap(taskID) {
		q.cancelled[taskID] = true
		q.mu.Unlock()
	} else {
		q.mu.Unlock()
	}

	if err := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{{TaskID: taskID, Status: models.TaskStatusCancelled, Error: "cancelled by user"}}); err != nil {
		return fmt.Errorf("update task status to cancelled: %w", err)
	}
	q.emitEvent(taskID, models.TaskStatusCancelled, "cancelled by user")
	return nil
}

// PauseBatch pauses all pending tasks for the given batch. Running tasks
// continue to completion but new tasks from this batch won't be picked up.
func (q *Queue) PauseBatch(batchID string) {
	q.mu.Lock()
	q.paused[batchID] = true
	// Move items from main heap to paused heap for this batch.
	var remaining []*heapItem
	for q.pq.Len() > 0 {
		item := heap.Pop(&q.pq).(*heapItem)
		delete(q.heapSet, item.task.ID)
		if item.task.BatchID == batchID {
			heap.Push(&q.pausedPQ, item)
			q.pausedSet[item.task.ID] = struct{}{}
		} else {
			remaining = append(remaining, item)
		}
	}
	for _, item := range remaining {
		heap.Push(&q.pq, item)
		q.heapSet[item.task.ID] = struct{}{}
	}
	q.mu.Unlock()
}

// ResumeBatch resumes a paused batch. Paused tasks are moved back into the
// main priority queue and workers are signaled.
func (q *Queue) ResumeBatch(batchID string) {
	q.mu.Lock()
	delete(q.paused, batchID)
	// Move items back from paused heap to main heap.
	var remaining []*heapItem
	movedCount := 0
	for q.pausedPQ.Len() > 0 {
		item := heap.Pop(&q.pausedPQ).(*heapItem)
		delete(q.pausedSet, item.task.ID)
		if item.task.BatchID == batchID {
			heap.Push(&q.pq, item)
			q.heapSet[item.task.ID] = struct{}{}
			movedCount++
		} else {
			remaining = append(remaining, item)
		}
	}
	for _, item := range remaining {
		heap.Push(&q.pausedPQ, item)
		q.pausedSet[item.task.ID] = struct{}{}
	}
	q.mu.Unlock()

	// Wake workers for the resumed tasks.
	if movedCount > 0 {
		q.cond.Broadcast()
	}
}

func (q *Queue) enqueueTaskStateChange(change database.TaskStateChange) error {
	return q.enqueueTaskStateChanges([]database.TaskStateChange{change})
}

func (q *Queue) enqueueTaskStateChanges(changes []database.TaskStateChange) error {
	if len(changes) == 0 {
		return nil
	}

	q.mu.Lock()
	stopped := q.stopped
	q.mu.Unlock()
	if stopped {
		return q.db.BatchApplyTaskStateChanges(context.Background(), changes)
	}

	for _, change := range changes {
		select {
		case q.persistenceCh <- taskStateWrite{change: change}:
		default:
			if err := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{change}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *Queue) persistenceWorker() {
	defer q.persistenceWg.Done()

	ticker := time.NewTicker(q.persistenceInterval)
	defer ticker.Stop()

	batch := make([]database.TaskStateChange, 0, q.persistenceBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		pending := append([]database.TaskStateChange(nil), batch...)
		batch = batch[:0]
		if err := q.db.BatchApplyTaskStateChanges(context.Background(), pending); err != nil {
			for _, change := range pending {
				q.emitEvent(change.TaskID, models.TaskStatusFailed, fmt.Sprintf("persist state change: %v", err))
			}
		}
	}

	for {
		select {
		case write, ok := <-q.persistenceCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, write.change)
			if len(batch) >= q.persistenceBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// Stop cancels all running and pending tasks and prevents new submissions.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopped = true

		// Cancel all running tasks.
		for id, cancel := range q.running {
			cancel()
			delete(q.running, id)
		}
		q.runningProxied = 0

		// Cancel all tasks in the main heap.
		for q.pq.Len() > 0 {
			item := heap.Pop(&q.pq).(*heapItem)
			delete(q.heapSet, item.task.ID)
			item.cancel()
		}

		// Cancel all tasks in the paused heap.
		for q.pausedPQ.Len() > 0 {
			item := heap.Pop(&q.pausedPQ).(*heapItem)
			delete(q.pausedSet, item.task.ID)
			item.cancel()
		}
		q.mu.Unlock()

		// Wake all workers so they can exit.
		q.cond.Broadcast()
		close(q.stopCh)

		// Wait for all workers to finish.
		q.workerWg.Wait()
		close(q.persistenceCh)
		q.persistenceWg.Wait()
	})
}

// RunningCount returns the number of currently executing tasks.
func (q *Queue) RunningCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.running)
}

// Metrics returns a snapshot of queue metrics.
// Queued = tasks waiting in the priority heap (main + paused).
// Running = tasks currently executing.
// Pending = Queued + Running (total in-flight).
func (q *Queue) Metrics() models.QueueMetrics {
	q.mu.Lock()
	defer q.mu.Unlock()
	metrics := q.metrics
	metrics.Running = len(q.running)
	metrics.Queued = q.pq.Len() + q.pausedPQ.Len()
	metrics.Pending = metrics.Queued + metrics.Running
	metrics.RunningProxied = q.runningProxied
	metrics.ProxyConcurrencyLimit = q.proxyConcurrencyLimit
	metrics.PersistenceQueueDepth = len(q.persistenceCh)
	metrics.PersistenceQueueCapacity = cap(q.persistenceCh)
	metrics.PersistenceBatchSize = q.persistenceBatchSize
	return metrics
}

// RecoverStaleTasks finds tasks stuck in "running" or "queued" status
// (from a previous crash), resets them to "pending", and re-submits them.
func (q *Queue) RecoverStaleTasks(ctx context.Context) error {
	stale, err := q.db.ListStaleTasks(ctx)
	if err != nil {
		return fmt.Errorf("list stale tasks: %w", err)
	}
	for _, task := range stale {
		if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusPending, Error: "recovered after restart"}); err != nil {
			return fmt.Errorf("reset stale task %s: %w", task.ID, err)
		}
		task.Status = models.TaskStatusPending
		if err := q.Submit(ctx, task); err != nil {
			return fmt.Errorf("re-submit stale task %s: %w", task.ID, err)
		}
	}
	return nil
}

// worker is the main loop for a fixed pool worker. It blocks on the
// condition variable until a task is available, then executes it.
func (q *Queue) worker(_ int) {
	defer q.workerWg.Done()

	for {
		q.mu.Lock()
		for !q.stopped {
			item, countsAgainstProxyLimit, autoProxy := q.dequeueRunnableLocked()
			if item != nil {
				q.mu.Unlock()
				q.executeTask(item.ctx, item.task, countsAgainstProxyLimit, autoProxy)
				goto next
			}
			q.cond.Wait()
		}
		q.mu.Unlock()
		return
	next:
	}
}

func (q *Queue) dequeueRunnableLocked() (*heapItem, bool, bool) {
	deferred := make([]*heapItem, 0)
	for q.pq.Len() > 0 {
		item := heap.Pop(&q.pq).(*heapItem)
		delete(q.heapSet, item.task.ID)

		if q.cancelled[item.task.ID] {
			delete(q.cancelled, item.task.ID)
			item.cancel()
			continue
		}
		if item.task.BatchID != "" && q.paused[item.task.BatchID] {
			heap.Push(&q.pausedPQ, item)
			q.pausedSet[item.task.ID] = struct{}{}
			continue
		}

		autoProxy := item.task.Proxy.Server == "" && q.proxyManager != nil
		countsAgainstProxyLimit := item.task.Proxy.Server != "" || autoProxy
		if countsAgainstProxyLimit && q.proxyConcurrencyLimit > 0 && q.runningProxied >= q.proxyConcurrencyLimit {
			deferred = append(deferred, item)
			continue
		}

		for _, pending := range deferred {
			heap.Push(&q.pq, pending)
			q.heapSet[pending.task.ID] = struct{}{}
		}
		q.running[item.task.ID] = item.cancel
		if countsAgainstProxyLimit {
			q.runningProxied++
		}
		return item, countsAgainstProxyLimit, autoProxy
	}

	for _, pending := range deferred {
		heap.Push(&q.pq, pending)
		q.heapSet[pending.task.ID] = struct{}{}
	}
	return nil, false, false
}

type retryInfo struct {
	shouldRetry bool
	task        models.Task
	backoff     time.Duration
	parentCtx   context.Context
}

func (q *Queue) executeTask(ctx context.Context, task models.Task, countsAgainstProxyLimit bool, autoProxy bool) {
	defer func() {
		q.mu.Lock()
		delete(q.running, task.ID)
		delete(q.cancelled, task.ID)
		if countsAgainstProxyLimit && q.runningProxied > 0 {
			q.runningProxied--
		}
		q.mu.Unlock()
		q.cond.Broadcast()
	}()

	q.mu.Lock()
	if q.stopped || q.cancelled[task.ID] {
		delete(q.cancelled, task.ID)
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()

	taskTimeout := 5 * time.Minute
	if task.Timeout > 0 {
		taskTimeout = time.Duration(task.Timeout) * time.Second
	}
	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	// Update the cancel func so Cancel() uses the timeout-aware one.
	q.mu.Lock()
	if q.stopped || q.cancelled[task.ID] {
		delete(q.cancelled, task.ID)
		q.mu.Unlock()
		return
	}
	q.running[task.ID] = cancel
	q.mu.Unlock()

	pm := q.getProxyManager()
	var reservation *proxy.Reservation
	if autoProxy && pm != nil {
		fallback := task.Proxy.Fallback
		if fallback == "" {
			fallback = models.ProxyFallbackStrict
		}
		lease, fallbackUsed, direct, err := pm.ReserveProxyWithFallback(task.Proxy.Geo, fallback)
		if err == nil {
			if lease != nil {
				reservation = lease
				task.Proxy = lease.ProxyConfig()
				task.Proxy.Fallback = fallback
			} else if direct {
				task.Proxy = models.ProxyConfig{}
				q.mu.Lock()
				if countsAgainstProxyLimit && q.runningProxied > 0 {
					q.runningProxied--
					countsAgainstProxyLimit = false
				}
				q.mu.Unlock()
				q.cond.Broadcast()
			}
			if fallbackUsed {
				q.emitEvent(task.ID, models.TaskStatusQueued, fmt.Sprintf("proxy country fallback applied for %s", task.Proxy.Geo))
			}
		} else if errors.Is(err, proxy.ErrNoHealthyProxies) {
			q.mu.Lock()
			if countsAgainstProxyLimit && q.runningProxied > 0 {
				q.runningProxied--
				countsAgainstProxyLimit = false
			}
			q.mu.Unlock()
			q.cond.Broadcast()
			q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("reserve proxy: %v", err))
			return
		} else {
			q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("reserve proxy: %v", err))
			return
		}
	}

	if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusRunning}); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		return
	}
	q.emitEvent(task.ID, models.TaskStatusRunning, "")

	result, err := q.runner.RunTask(taskCtx, task)

	if reservation != nil {
		if completeErr := reservation.Complete(err == nil); completeErr != nil {
			q.emitEvent(task.ID, task.Status, fmt.Sprintf("proxy usage recording failed: %v", completeErr))
		}
	}

	var retry retryInfo
	if err != nil {
		// Check if this is a cancellation error
		if errors.Is(err, context.Canceled) {
			// Task was cancelled, update status accordingly
			if err := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{{TaskID: task.ID, Status: models.TaskStatusCancelled, Error: "cancelled by user"}}); err != nil {
				q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
			} else {
				q.mu.Lock()
				q.metrics.TotalFailed++ // Still count as failed for metrics
				q.mu.Unlock()
			}
			q.emitEvent(task.ID, models.TaskStatusCancelled, "cancelled by user")
			return
		}
		retry = q.handleFailure(ctx, task, err, result)
	} else {
		q.handleSuccess(task, result)
	}

	if retry.shouldRetry {
		go q.scheduleRetry(retry)
	}
}

func (q *Queue) handleFailure(parentCtx context.Context, task models.Task, execErr error, result *models.TaskResult) retryInfo {
	if task.RetryCount < task.MaxRetries {
		if result != nil {
			if len(result.StepLogs) > 0 {
				if slErr := q.db.InsertStepLogs(parentCtx, task.ID, result.StepLogs); slErr != nil {
					q.emitEvent(task.ID, task.Status, fmt.Sprintf("persist step logs: %v", slErr))
				}
			}
			if len(result.NetworkLogs) > 0 {
				if nlErr := q.db.InsertNetworkLogs(parentCtx, task.ID, result.NetworkLogs); nlErr != nil {
					q.emitEvent(task.ID, task.Status, fmt.Sprintf("persist network logs: %v", nlErr))
				}
			}
		}
		if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusRetrying, Error: execErr.Error(), IncrementRetry: true}); err != nil {
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

	stepLogs := []models.StepLog(nil)
	networkLogs := []models.NetworkLog(nil)
	if result != nil {
		stepLogs = result.StepLogs
		networkLogs = result.NetworkLogs
	}
	if err := q.db.FinalizeTaskFailure(context.Background(), task.ID, execErr.Error(), stepLogs, networkLogs); err != nil {
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
		if err := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{{TaskID: ri.task.ID, Status: models.TaskStatusCancelled, Error: "cancelled during retry backoff (queue stopped)"}}); err != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		}
		return
	case <-ri.parentCtx.Done():
		timer.Stop()
		if err := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{{TaskID: ri.task.ID, Status: models.TaskStatusCancelled, Error: "cancelled during retry backoff"}}); err != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err))
		}
		return
	}

	// Re-submit via the heap instead of spawning another goroutine.
	if err := q.Submit(ri.parentCtx, ri.task); err != nil {
		if err2 := q.db.BatchApplyTaskStateChanges(context.Background(), []database.TaskStateChange{{TaskID: ri.task.ID, Status: models.TaskStatusFailed, Error: fmt.Sprintf("retry re-submit: %v", err)}}); err2 != nil {
			q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("update status: %v", err2))
		}
		q.emitEvent(ri.task.ID, models.TaskStatusFailed, fmt.Sprintf("retry re-submit: %v", err))
	}
}

func (q *Queue) handleSuccess(task models.Task, result *models.TaskResult) {
	if result == nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, "save result: missing task result")
		return
	}
	if err := q.db.FinalizeTaskSuccess(context.Background(), task.ID, *result); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("save result: %v", err))
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

// isTaskEnqueued checks if a task is currently running. Must be called with mu held.
func (q *Queue) isTaskEnqueued(taskID string) bool {
	_, ok := q.running[taskID]
	return ok
}

// isTaskInHeap checks if a task is in the main or paused heap. Must be called with mu held.
func (q *Queue) isTaskInHeap(taskID string) bool {
	_, inMain := q.heapSet[taskID]
	_, inPaused := q.pausedSet[taskID]
	return inMain || inPaused
}

// removeFromHeap removes a task from the main heap. Returns true if found.
// Must be called with mu held.
func (q *Queue) removeFromHeap(taskID string) bool {
	for i, item := range q.pq {
		if item.task.ID == taskID {
			item.cancel()
			heap.Remove(&q.pq, i)
			delete(q.heapSet, taskID)
			return true
		}
	}
	return false
}

// removeFromPausedHeap removes a task from the paused heap. Returns true if found.
// Must be called with mu held.
func (q *Queue) removeFromPausedHeap(taskID string) bool {
	for i, item := range q.pausedPQ {
		if item.task.ID == taskID {
			item.cancel()
			heap.Remove(&q.pausedPQ, i)
			delete(q.pausedSet, taskID)
			return true
		}
	}
	return false
}
