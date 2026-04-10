package queue

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"flowpilot/internal/browser"
	"flowpilot/internal/database"
	"flowpilot/internal/logs"
	"flowpilot/internal/models"
	"flowpilot/internal/proxy"
)

// EventCallback is invoked when a task's status changes.
type EventCallback func(event models.TaskEvent)

// ErrQueueFull is returned when the pending queue has reached its maximum size.
var ErrQueueFull = errors.New("queue is full: too many pending tasks")

var webhookClient = &http.Client{Timeout: 10 * time.Second}

// ErrBatchPaused is returned when attempting to submit to a paused batch.
var ErrBatchPaused = errors.New("batch is paused")

const errCancelledByUser = "cancelled by user"
const errUpdateStatusFmt = "update status: %v"

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
	retryBackoffBaseMs    int
	drainTimeout          time.Duration
	persistenceBatchSize  int
	persistenceInterval   time.Duration
	persistenceCh         chan taskStateWrite
	persistenceWg         sync.WaitGroup

	taskMetrics atomic.Value // stores models.TaskMetrics

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
	proxyWakeTimer *time.Timer
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

func (q *Queue) dbWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	const dbWriteTimeout = 5 * time.Second
	return context.WithTimeout(parent, dbWriteTimeout)
}

func emitUpdateStatusError(q *Queue, taskID string, err error) {
	q.emitEvent(taskID, models.TaskStatusFailed, fmt.Sprintf(errUpdateStatusFmt, err))
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

func (q *Queue) SetRetryBackoffBaseMs(ms int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retryBackoffBaseMs = ms
}

// SetDrainTimeout configures how long Stop() will wait for running tasks to
// complete before force-cancelling them. Zero means no drain (cancel immediately).
func (q *Queue) SetDrainTimeout(d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.drainTimeout = d
}

func (q *Queue) getProxyManager() *proxy.Manager {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.proxyManager
}

// Submit enqueues a task for execution. Returns ErrQueueFull if at capacity.
// validateSubmitTask checks if a task can be submitted to the queue (S3776)
func (q *Queue) validateSubmitTask(taskID string) error {
	if q.stopped {
		return fmt.Errorf("queue is stopped")
	}
	if q.isTaskEnqueued(taskID) {
		return fmt.Errorf("task %s is already running", taskID)
	}
	if q.isTaskInHeap(taskID) {
		return fmt.Errorf("task %s is already pending", taskID)
	}
	if q.maxPending > 0 && q.pq.Len()+q.pausedPQ.Len() >= q.maxPending {
		return ErrQueueFull
	}
	return nil
}

// addTaskToHeap adds a task to the priority queue (S3776)
func (q *Queue) addTaskToHeap(task models.Task, ctx context.Context) (*heapItem, context.CancelFunc) {
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
	return item, cancel
}

func (q *Queue) Submit(ctx context.Context, task models.Task) error {
	q.mu.Lock()
	if err := q.validateSubmitTask(task.ID); err != nil {
		q.mu.Unlock()
		return err
	}

	item, cancel := q.addTaskToHeap(task, ctx)
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

// validateBatchSubmit checks if a batch of tasks can be submitted (S3776)
func (q *Queue) validateBatchSubmit(taskCount int) error {
	if q.stopped {
		return fmt.Errorf("queue is stopped")
	}
	if q.maxPending > 0 && q.pq.Len()+q.pausedPQ.Len()+taskCount > q.maxPending {
		return ErrQueueFull
	}
	return nil
}

// addTasksToBatch adds eligible tasks to the queue (S3776)
func (q *Queue) addTasksToBatch(ctx context.Context, tasks []models.Task) []models.Task {
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
	return added
}

// SubmitBatch enqueues multiple tasks with a single lock acquisition and DB transaction.
func (q *Queue) SubmitBatch(ctx context.Context, tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	q.mu.Lock()
	if err := q.validateBatchSubmit(len(tasks)); err != nil {
		q.mu.Unlock()
		return err
	}

	added := q.addTasksToBatch(ctx, tasks)
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
	} else if q.removeFromHeap(taskID) || q.removeFromPausedHeap(taskID) {
		q.cancelled[taskID] = true
	}
	q.mu.Unlock()

	dbCtx, cancel := q.dbWriteContext(context.TODO())
	defer cancel()
	if err := q.db.BatchApplyTaskStateChanges(dbCtx, []database.TaskStateChange{{TaskID: taskID, Status: models.TaskStatusCancelled, Error: errCancelledByUser}}); err != nil {
		return fmt.Errorf("update task status to cancelled: %w", err)
	}
	q.emitEvent(taskID, models.TaskStatusCancelled, errCancelledByUser)
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
		dbCtx, cancel := q.dbWriteContext(context.TODO())
		defer cancel()
		return q.db.BatchApplyTaskStateChanges(dbCtx, changes)
	}

	for _, change := range changes {
		select {
		case q.persistenceCh <- taskStateWrite{change: change}:
		default:
			dbCtx, cancel := q.dbWriteContext(context.TODO())
			err := q.db.BatchApplyTaskStateChanges(dbCtx, []database.TaskStateChange{change})
			cancel()
			if err != nil {
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
		dbCtx, cancel := q.dbWriteContext(context.TODO())
		err := q.db.BatchApplyTaskStateChanges(dbCtx, pending)
		cancel()
		if err != nil {
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

// Stop prevents new submissions and drains running tasks. If a drain timeout
// was configured via SetDrainTimeout, it waits up to that long for running
// tasks to finish naturally before force-cancelling them. Pending (queued)
// tasks are cancelled immediately. Stop is idempotent.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopped = true
		drainTimeout := q.drainTimeout

		// Cancel all pending tasks in the main heap.
		for q.pq.Len() > 0 {
			item := heap.Pop(&q.pq).(*heapItem)
			delete(q.heapSet, item.task.ID)
			item.cancel()
		}

		// Cancel all pending tasks in the paused heap.
		for q.pausedPQ.Len() > 0 {
			item := heap.Pop(&q.pausedPQ).(*heapItem)
			delete(q.pausedSet, item.task.ID)
			item.cancel()
		}

		// Snapshot the IDs of currently running tasks for drain tracking.
		runningIDs := make([]string, 0, len(q.running))
		for id := range q.running {
			runningIDs = append(runningIDs, id)
		}
		q.mu.Unlock()

		// Wake all workers so they can exit their wait loops.
		q.cond.Broadcast()
		close(q.stopCh)

		q.drainRunningTasks(drainTimeout, runningIDs)

		q.mu.Lock()
		if q.proxyWakeTimer != nil {
			q.proxyWakeTimer.Stop()
			q.proxyWakeTimer = nil
		}
		q.mu.Unlock()
		close(q.persistenceCh)
		q.persistenceWg.Wait()
	})
}

// drainRunningTasks waits for running tasks to finish, optionally with a timeout.
func (q *Queue) drainRunningTasks(drainTimeout time.Duration, runningIDs []string) {
	if drainTimeout > 0 && len(runningIDs) > 0 {
		drained := make(chan struct{})
		go func() {
			q.workerWg.Wait()
			close(drained)
		}()

		timer := time.NewTimer(drainTimeout)
		defer timer.Stop()

		select {
		case <-drained:
			log.Printf("queue drain: all %d running tasks finished cleanly", len(runningIDs))
		case <-timer.C:
			q.mu.Lock()
			forceCancelled := make([]string, 0, len(q.running))
			for id, cancel := range q.running {
				cancel()
				delete(q.running, id)
				forceCancelled = append(forceCancelled, id)
			}
			q.runningProxied = 0
			q.mu.Unlock()
			q.cond.Broadcast()
			if len(forceCancelled) > 0 {
				log.Printf("queue drain: timeout after %s, force-cancelled %d task(s): %v", drainTimeout, len(forceCancelled), forceCancelled)
			}
			q.workerWg.Wait()
		}
	} else {
		q.mu.Lock()
		for id, cancel := range q.running {
			cancel()
			delete(q.running, id)
		}
		q.runningProxied = 0
		q.mu.Unlock()
		q.cond.Broadcast()
		q.workerWg.Wait()
	}
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
func (q *Queue) scheduleProxyWake(delay time.Duration) {
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	if q.stopped {
		return
	}
	if q.proxyWakeTimer != nil {
		q.proxyWakeTimer.Stop()
	}
	q.proxyWakeTimer = time.AfterFunc(delay, func() {
		q.mu.Lock()
		q.proxyWakeTimer = nil
		q.mu.Unlock()
		q.cond.Broadcast()
	})
}

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

		if q.handleCancelledItemLocked(item) {
			continue
		}
		if q.handlePausedItemLocked(item) {
			continue
		}

		autoProxy, countsAgainstProxyLimit := q.proxyUsageForItemLocked(item)
		if q.proxyLimitReached(countsAgainstProxyLimit) {
			deferred = append(deferred, item)
			continue
		}
		if q.deferForUnavailableProxyLocked(item, autoProxy, &deferred) {
			continue
		}

		q.restoreDeferredLocked(deferred)
		q.markRunningLocked(item, countsAgainstProxyLimit)
		return item, countsAgainstProxyLimit, autoProxy
	}

	q.restoreDeferredLocked(deferred)
	return nil, false, false
}

type retryInfo struct {
	shouldRetry bool
	task        models.Task
	backoff     time.Duration
}

func (q *Queue) handleCancelledItemLocked(item *heapItem) bool {
	if !q.cancelled[item.task.ID] {
		return false
	}
	delete(q.cancelled, item.task.ID)
	item.cancel()
	return true
}

func (q *Queue) handlePausedItemLocked(item *heapItem) bool {
	if item.task.BatchID == "" || !q.paused[item.task.BatchID] {
		return false
	}
	heap.Push(&q.pausedPQ, item)
	q.pausedSet[item.task.ID] = struct{}{}
	return true
}

func (q *Queue) proxyUsageForItemLocked(item *heapItem) (bool, bool) {
	autoProxy := item.task.Proxy.Server == "" && q.proxyManager != nil && (item.task.Proxy.Geo != "" || item.task.Proxy.Fallback != "")
	countsAgainstProxyLimit := item.task.Proxy.Server != "" || autoProxy
	return autoProxy, countsAgainstProxyLimit
}

func (q *Queue) proxyLimitReached(countsAgainstProxyLimit bool) bool {
	return countsAgainstProxyLimit && q.proxyConcurrencyLimit > 0 && q.runningProxied >= q.proxyConcurrencyLimit
}

func (q *Queue) deferForUnavailableProxyLocked(item *heapItem, autoProxy bool, deferred *[]*heapItem) bool {
	if !autoProxy || q.proxyManager == nil {
		return false
	}
	fallback := item.task.Proxy.Fallback
	if fallback == "" {
		fallback = models.ProxyFallbackStrict
	}
	available, wait, err := q.proxyManager.HasAvailableProxy(item.ctx, item.task.Proxy.Geo, fallback)
	if err != nil || available {
		return false
	}
	*deferred = append(*deferred, item)
	q.scheduleProxyWake(wait)
	return true
}

func (q *Queue) restoreDeferredLocked(deferred []*heapItem) {
	for _, pending := range deferred {
		heap.Push(&q.pq, pending)
		q.heapSet[pending.task.ID] = struct{}{}
	}
}

func (q *Queue) markRunningLocked(item *heapItem, countsAgainstProxyLimit bool) {
	q.running[item.task.ID] = item.cancel
	if countsAgainstProxyLimit {
		q.runningProxied++
	}
}

func (q *Queue) executeTask(ctx context.Context, task models.Task, countsAgainstProxyLimit bool, autoProxy bool) {
	defer q.finishExecuteTask(task.ID, &countsAgainstProxyLimit)
	if q.shouldSkipExecution(task.ID) {
		return
	}

	taskCtx, cancel := q.newTaskContext(ctx, task.Timeout)
	defer cancel()
	if q.shouldSkipExecution(task.ID) {
		return
	}
	q.setTaskCancel(task.ID, cancel)

	var reservation *proxy.Reservation
	var ok bool
	task, reservation, countsAgainstProxyLimit, ok = q.prepareAutoProxy(taskCtx, task, countsAgainstProxyLimit, autoProxy)
	if !ok {
		return
	}

	if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusRunning}); err != nil {
		emitUpdateStatusError(q, task.ID, err)
		return
	}
	q.emitEvent(task.ID, models.TaskStatusRunning, "")

	result, err := q.runner.RunTask(taskCtx, task)
	q.completeTaskReservation(task, reservation, err)
	if err == nil {
		q.handleSuccess(taskCtx, task, result)
		return
	}
	if q.handleTaskCancellation(ctx, task.ID, err) {
		return
	}

	retry := q.handleFailure(ctx, task, err, result)
	if retry.shouldRetry {
		q.workerWg.Add(1)
		go func() {
			defer q.workerWg.Done()
			q.scheduleRetry(ctx, retry)
		}()
	}
}

func (q *Queue) finishExecuteTask(taskID string, countsAgainstProxyLimit *bool) {
	q.mu.Lock()
	delete(q.running, taskID)
	delete(q.cancelled, taskID)
	if *countsAgainstProxyLimit && q.runningProxied > 0 {
		q.runningProxied--
	}
	q.mu.Unlock()
	q.cond.Broadcast()
}

func (q *Queue) shouldSkipExecution(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.stopped && !q.cancelled[taskID] {
		return false
	}
	delete(q.cancelled, taskID)
	return true
}

func (q *Queue) newTaskContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	taskTimeout := 5 * time.Minute
	if timeoutSeconds > 0 {
		taskTimeout = time.Duration(timeoutSeconds) * time.Second
	}
	return context.WithTimeout(parent, taskTimeout)
}

func (q *Queue) setTaskCancel(taskID string, cancel context.CancelFunc) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.running[taskID] = cancel
}

func (q *Queue) prepareAutoProxy(taskCtx context.Context, task models.Task, countsAgainstProxyLimit bool, autoProxy bool) (models.Task, *proxy.Reservation, bool, bool) {
	pm := q.getProxyManager()
	if !autoProxy || pm == nil {
		return task, nil, countsAgainstProxyLimit, true
	}
	fallback := task.Proxy.Fallback
	if fallback == "" {
		fallback = models.ProxyFallbackStrict
	}
	lease, fallbackUsed, direct, err := pm.ReserveProxyWithFallback(taskCtx, task.Proxy.Geo, fallback)
	if err != nil {
		if errors.Is(err, proxy.ErrNoHealthyProxies) {
			q.releaseExecutionProxySlot(&countsAgainstProxyLimit)
		}
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("reserve proxy: %v", err))
		return task, nil, countsAgainstProxyLimit, false
	}
	if lease != nil {
		task.Proxy = lease.ProxyConfig()
		task.Proxy.Fallback = fallback
	}
	if direct {
		task.Proxy = models.ProxyConfig{}
		q.releaseExecutionProxySlot(&countsAgainstProxyLimit)
	}
	if fallbackUsed {
		q.emitEvent(task.ID, models.TaskStatusQueued, fmt.Sprintf("proxy country fallback applied for %s", task.Proxy.Geo))
	}
	return task, lease, countsAgainstProxyLimit, true
}

func (q *Queue) releaseExecutionProxySlot(countsAgainstProxyLimit *bool) {
	q.mu.Lock()
	if *countsAgainstProxyLimit && q.runningProxied > 0 {
		q.runningProxied--
		*countsAgainstProxyLimit = false
	}
	q.mu.Unlock()
	q.cond.Broadcast()
}

func (q *Queue) completeTaskReservation(task models.Task, reservation *proxy.Reservation, runErr error) {
	if reservation == nil {
		return
	}
	if completeErr := reservation.Complete(runErr == nil); completeErr != nil {
		q.emitEvent(task.ID, task.Status, fmt.Sprintf("proxy usage recording failed: %v", completeErr))
	}
}

func (q *Queue) handleTaskCancellation(ctx context.Context, taskID string, runErr error) bool {
	if !errors.Is(runErr, context.Canceled) {
		return false
	}
	// ctx is cancelled at this point; use WithoutCancel to allow the DB write
	// to complete while still propagating parent values.
	dbCtx, cancel := q.dbWriteContext(context.WithoutCancel(ctx))
	err := q.db.BatchApplyTaskStateChanges(dbCtx, []database.TaskStateChange{{TaskID: taskID, Status: models.TaskStatusCancelled, Error: errCancelledByUser}})
	cancel()
	if err != nil {
		emitUpdateStatusError(q, taskID, err)
	} else {
		q.mu.Lock()
		q.metrics.TotalFailed++
		q.mu.Unlock()
	}
	q.emitEvent(taskID, models.TaskStatusCancelled, errCancelledByUser)
	return true
}

// UpdateMetrics updates the in-memory TaskMetrics snapshot.
func (q *Queue) UpdateMetrics(completed, failed, avgDurationMs, queueDepth int) {
	q.taskMetrics.Store(models.TaskMetrics{
		Completed:     completed,
		Failed:        failed,
		AvgDurationMs: avgDurationMs,
		QueueDepth:    queueDepth,
	})
}

// TaskMetrics returns the latest TaskMetrics snapshot.
func (q *Queue) TaskMetrics() models.TaskMetrics {
	if v := q.taskMetrics.Load(); v != nil {
		return v.(models.TaskMetrics)
	}
	return models.TaskMetrics{}
}

// persistRetryLogs saves step and network logs for a task that is about to be retried.
func (q *Queue) persistRetryLogs(ctx context.Context, taskID string, result *models.TaskResult) {
	if result == nil {
		return
	}
	dbCtx, cancel := q.dbWriteContext(ctx)
	defer cancel()
	if len(result.StepLogs) > 0 {
		if err := q.db.InsertStepLogs(dbCtx, taskID, result.StepLogs); err != nil {
			q.emitEvent(taskID, models.TaskStatusRetrying, fmt.Sprintf("persist retry step logs: %v", err))
		}
	}
	if len(result.NetworkLogs) > 0 {
		if err := q.db.InsertNetworkLogs(dbCtx, taskID, result.NetworkLogs); err != nil {
			q.emitEvent(taskID, models.TaskStatusRetrying, fmt.Sprintf("persist retry network logs: %v", err))
		}
	}
}

func (q *Queue) handleFailure(parentCtx context.Context, task models.Task, execErr error, result *models.TaskResult) retryInfo {
	if task.RetryCount < task.MaxRetries {
		return q.prepareRetry(parentCtx, task, execErr, result)
	}

	stepLogs, networkLogs, duration, extractedDataKeys := failureArtifacts(result)
	dbCtx, cancel := q.dbWriteContext(parentCtx)
	defer cancel()
	if err := q.db.FinalizeTaskFailure(dbCtx, task.ID, execErr.Error(), stepLogs, networkLogs); err != nil {
		emitUpdateStatusError(q, task.ID, err)
		return retryInfo{}
	}
	q.mu.Lock()
	q.metrics.TotalFailed++
	q.mu.Unlock()
	logs.Logger.Error("task failed",
		slog.String("task_id", task.ID),
		slog.String("action", "failure"),
		slog.String("error", execErr.Error()),
		slog.Int64("duration_ms", duration.Milliseconds()),
	)
	q.emitEvent(task.ID, models.TaskStatusFailed, execErr.Error())
	if task.WebhookURL != "" && webhookEventEnabled(task.WebhookEvents, string(models.TaskStatusFailed)) {
		q.dispatchWebhook(task.WebhookURL, task.ID, models.TaskStatusFailed, duration, execErr.Error(), extractedDataKeys)
	}
	return retryInfo{}
}

func (q *Queue) prepareRetry(parentCtx context.Context, task models.Task, execErr error, result *models.TaskResult) retryInfo {
	q.persistRetryLogs(parentCtx, task.ID, result)
	if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusRetrying, Error: execErr.Error(), IncrementRetry: true}); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("increment retry: %v", err))
		return retryInfo{}
	}
	q.emitEvent(task.ID, models.TaskStatusRetrying, execErr.Error())

	backoff := q.retryBackoff(task.RetryCount)
	logs.Logger.Info("task retrying",
		slog.String("task_id", task.ID),
		slog.String("action", "retry"),
		slog.Int("retry_count", task.RetryCount+1),
		slog.Int("max_retries", task.MaxRetries),
		slog.String("error", execErr.Error()),
	)

	retryTask := task
	retryTask.RetryCount++
	retryTask.Status = models.TaskStatusPending
	retryTask.Steps = make([]models.TaskStep, len(task.Steps))
	copy(retryTask.Steps, task.Steps)

	return retryInfo{shouldRetry: true, task: retryTask, backoff: backoff}
}

func (q *Queue) retryBackoff(retryCount int) time.Duration {
	q.mu.Lock()
	baseMs := q.retryBackoffBaseMs
	q.mu.Unlock()
	if baseMs <= 0 {
		baseMs = 5000
	}
	backoffMs := float64(baseMs) * math.Pow(2, float64(retryCount))
	const maxBackoffMs = 300000
	if backoffMs > maxBackoffMs {
		backoffMs = maxBackoffMs
	}
	return time.Duration(backoffMs) * time.Millisecond
}

func failureArtifacts(result *models.TaskResult) ([]models.StepLog, []models.NetworkLog, time.Duration, []string) {
	if result == nil {
		return nil, nil, 0, nil
	}
	extractedDataKeys := make([]string, 0, len(result.ExtractedData))
	for k := range result.ExtractedData {
		extractedDataKeys = append(extractedDataKeys, k)
	}
	return result.StepLogs, result.NetworkLogs, result.Duration, extractedDataKeys
}

func (q *Queue) handleSuccess(execCtx context.Context, task models.Task, result *models.TaskResult) {
	if result == nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, "save result: missing task result")
		return
	}
	dbCtx, cancel := q.dbWriteContext(execCtx)
	defer cancel()
	if err := q.db.FinalizeTaskSuccess(dbCtx, task.ID, *result); err != nil {
		q.emitEvent(task.ID, models.TaskStatusFailed, fmt.Sprintf("save result: %v", err))
		return
	}
	q.mu.Lock()
	q.metrics.TotalCompleted++
	q.mu.Unlock()
	logs.Logger.Info("task completed",
		slog.String("task_id", task.ID),
		slog.String("action", "success"),
		slog.Int64("duration_ms", result.Duration.Milliseconds()),
	)
	q.emitEvent(task.ID, models.TaskStatusCompleted, "")
	if task.WebhookURL != "" && webhookEventEnabled(task.WebhookEvents, string(models.TaskStatusCompleted)) {
		var extractedDataKeys []string
		for k := range result.ExtractedData {
			extractedDataKeys = append(extractedDataKeys, k)
		}
		q.dispatchWebhook(task.WebhookURL, task.ID, models.TaskStatusCompleted, result.Duration, "", extractedDataKeys)
	}
}

func (q *Queue) scheduleRetry(ctx context.Context, ri retryInfo) {
	timer := time.NewTimer(ri.backoff)
	select {
	case <-timer.C:
	case <-q.stopCh:
		timer.Stop()
		q.markRetryCancelled(ctx, ri.task.ID, "cancelled during retry backoff (queue stopped)")
		return
	case <-ctx.Done():
		timer.Stop()
		q.markRetryCancelled(ctx, ri.task.ID, "cancelled during retry backoff")
		return
	}

	// Re-submit via the heap instead of spawning another goroutine.
	if err := q.Submit(ctx, ri.task); err != nil {
		q.failRetryResubmit(ctx, ri.task.ID, err)
	}
}

func (q *Queue) markRetryCancelled(ctx context.Context, taskID, reason string) {
	// ctx may be cancelled; use WithoutCancel to preserve parent values while
	// allowing the DB write to complete.
	dbCtx, cancel := q.dbWriteContext(context.WithoutCancel(ctx))
	err := q.db.BatchApplyTaskStateChanges(dbCtx, []database.TaskStateChange{{TaskID: taskID, Status: models.TaskStatusCancelled, Error: reason}})
	cancel()
	if err != nil {
		emitUpdateStatusError(q, taskID, err)
	}
}

func (q *Queue) failRetryResubmit(ctx context.Context, taskID string, submitErr error) {
	dbCtx, cancel := q.dbWriteContext(ctx)
	err := q.db.BatchApplyTaskStateChanges(dbCtx, []database.TaskStateChange{{TaskID: taskID, Status: models.TaskStatusFailed, Error: fmt.Sprintf("retry re-submit: %v", submitErr)}})
	cancel()
	if err != nil {
		emitUpdateStatusError(q, taskID, err)
	}
	q.emitEvent(taskID, models.TaskStatusFailed, fmt.Sprintf("retry re-submit: %v", submitErr))
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

func webhookEventEnabled(events []string, event string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

type webhookPayload struct {
	TaskID            string   `json:"taskId"`
	Status            string   `json:"status"`
	DurationMs        int64    `json:"durationMs"`
	Error             string   `json:"error,omitempty"`
	ExtractedDataKeys []string `json:"extractedDataKeys,omitempty"`
}

func (q *Queue) dispatchWebhook(url, taskID string, status models.TaskStatus, duration time.Duration, errMsg string, extractedDataKeys []string) {
	q.workerWg.Add(1)
	go func() {
		defer q.workerWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		go func() {
			select {
			case <-q.stopCh:
				cancel()
			case <-ctx.Done():
			}
		}()
		fireWebhook(ctx, url, taskID, status, duration, errMsg, extractedDataKeys)
	}()
}

func fireWebhook(ctx context.Context, url, taskID string, status models.TaskStatus, duration time.Duration, errMsg string, extractedDataKeys []string) {
	payload := webhookPayload{
		TaskID:            taskID,
		Status:            string(status),
		DurationMs:        duration.Milliseconds(),
		Error:             errMsg,
		ExtractedDataKeys: extractedDataKeys,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook marshal error for task %s: %v", taskID, err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook request creation error for task %s: %v", taskID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := webhookClient.Do(req)
	if err != nil {
		log.Printf("webhook POST error for task %s: %v", taskID, err)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // drain so connection can be reused
		resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("webhook non-2xx response for task %s: %d", taskID, resp.StatusCode)
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
