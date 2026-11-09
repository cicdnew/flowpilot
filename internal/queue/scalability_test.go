package queue

import (
	"container/heap"
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"flowpilot/internal/database"
	"flowpilot/internal/models"
)

func fillHeaps(q *Queue, mainN, pausedN int) []string {
	ids := make([]string, 0, mainN+pausedN)
	base := time.Now()
	for i := range mainN {
		task := models.Task{ID: fmt.Sprintf("m-%d", i), Priority: models.TaskPriority(i % 3 * 5)}
		item, _ := q.addTaskToHeap(task, context.Background())
		item.addedAt = base.Add(time.Duration(i))
		ids = append(ids, task.ID)
	}
	for i := range pausedN {
		task := models.Task{ID: fmt.Sprintf("p-%d", i), Priority: models.TaskPriority(i % 3 * 5)}
		_, cancel := context.WithCancel(context.Background())
		item := &heapItem{task: task, ctx: context.Background(), cancel: cancel, addedAt: base.Add(time.Duration(i))}
		heap.Push(&q.pausedPQ, item)
		q.pausedSet[task.ID] = item
		ids = append(ids, task.ID)
	}
	return ids
}

func newHeapOnlyQueue(capacity int) *Queue {
	return &Queue{heapSet: make(map[string]*heapItem, capacity), pausedSet: make(map[string]*heapItem, capacity)}
}

func timeRemovals(q *Queue, ids []string, n int) time.Duration {
	rng := rand.New(rand.NewPCG(1, 2))
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	start := time.Now()
	for _, id := range ids[:n] {
		if !q.removeFromHeap(id) && !q.removeFromPausedHeap(id) {
			panic("task not found: " + id)
		}
	}
	return time.Since(start)
}

func assertHeapOrder(t *testing.T, h *taskHeap) {
	t.Helper()
	var prev *heapItem
	for h.Len() > 0 {
		item := heap.Pop(h).(*heapItem)
		if prev != nil && (prev.task.Priority < item.task.Priority ||
			(prev.task.Priority == item.task.Priority && prev.addedAt.After(item.addedAt))) {
			t.Fatalf("heap order violated: %s(p%d) before %s(p%d)", prev.task.ID, prev.task.Priority, item.task.ID, item.task.Priority)
		}
		prev = item
	}
}

func TestCancelLargeQueueRemovesCorrectlyAndKeepsHeapOrder(t *testing.T) {
	q := newHeapOnlyQueue(12000)
	ids := fillHeaps(q, 10000, 2000)
	timeRemovals(q, ids, 6000)
	if got := len(q.pq) + len(q.pausedPQ); got != 6000 {
		t.Fatalf("remaining items: got %d, want 6000", got)
	}
	if len(q.heapSet) != len(q.pq) || len(q.pausedSet) != len(q.pausedPQ) {
		t.Fatalf("lookup maps out of sync: heapSet=%d pq=%d pausedSet=%d pausedPQ=%d", len(q.heapSet), len(q.pq), len(q.pausedSet), len(q.pausedPQ))
	}
	for id, item := range q.heapSet {
		if q.pq[item.index] != item {
			t.Fatalf("stale index for %s", id)
		}
	}
	if q.removeFromHeap(ids[0]) || q.removeFromPausedHeap(ids[0]) {
		t.Fatal("removing an already-removed task must report false")
	}
	assertHeapOrder(t, &q.pq)
	assertHeapOrder(t, &q.pausedPQ)
}

func TestCancelCostDoesNotGrowLinearlyWithQueueDepth(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	const removals = 1000
	measure := func(depth int) time.Duration {
		best := time.Duration(1<<63 - 1)
		for range 3 {
			q := newHeapOnlyQueue(depth)
			ids := fillHeaps(q, depth, 0)
			best = min(best, timeRemovals(q, ids, removals))
		}
		return best
	}
	small, large := measure(2000), measure(200000)
	ratio := float64(large) / float64(small)
	t.Logf("%d cancels: depth 2k=%s depth 200k=%s ratio=%.1fx (linear scan would be ~100x)", removals, small, large, ratio)
	if ratio > 25 {
		t.Fatalf("cancel cost grew %.1fx for a 100x deeper queue; expected logarithmic growth", ratio)
	}
	if perOp := large / removals; perOp > time.Millisecond {
		t.Fatalf("cancel at depth 200k took %s per task; target < 1ms", perOp)
	}
}

func BenchmarkCancelFromQueue(b *testing.B) {
	for _, depth := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			q := newHeapOnlyQueue(depth)
			fillHeaps(q, depth, 0)
			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				id := fmt.Sprintf("m-%d", i%depth)
				if q.removeFromHeap(id) {
					b.StopTimer()
					item, _ := q.addTaskToHeap(models.Task{ID: id}, context.Background())
					_ = item
					b.StartTimer()
				}
			}
		})
	}
}

func TestPersistenceBackpressurePreservesWriteOrder(t *testing.T) {
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	task := makeTestTask("ordered-writes")
	createTask(t, db, task)

	const writes = 3000
	changes := make([]database.TaskStateChange, 0, writes+1)
	for i := range writes {
		status := models.TaskStatusQueued
		if i%2 == 1 {
			status = models.TaskStatusRunning
		}
		changes = append(changes, database.TaskStateChange{TaskID: task.ID, Status: status})
	}
	changes = append(changes, database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusCompleted})
	if err := q.enqueueTaskStateChanges(changes); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	backpressure := q.Metrics().PersistenceBackpressure
	q.Stop()

	got, err := db.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.TaskStatusCompleted {
		t.Fatalf("final persisted status: got %q, want %q (writes applied out of order)", got.Status, models.TaskStatusCompleted)
	}
	if backpressure == 0 {
		t.Fatalf("expected the %d-write burst to exceed channel capacity %d and apply backpressure", writes, cap(q.persistenceCh))
	}
	t.Logf("burst of %d writes: backpressure waits=%d, channel capacity=%d", writes+1, backpressure, cap(q.persistenceCh))

	if err := q.enqueueTaskStateChange(database.TaskStateChange{TaskID: task.ID, Status: models.TaskStatusFailed}); err != nil {
		t.Fatalf("write after Stop must be applied synchronously, not panic: %v", err)
	}
	if got, _ := db.GetTask(context.Background(), task.ID); got.Status != models.TaskStatusFailed {
		t.Fatalf("post-stop write not applied: %q", got.Status)
	}
}

func heapItemByID(h taskHeap, id string) *heapItem {
	for _, item := range h {
		if item.task.ID == id {
			return item
		}
	}
	panic("heapItemByID: task not in heap: " + id)
}

func TestSubmit1000TasksP95Latency(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	q, db := setupTestQueueNoWorkers(t, nil, nil)
	defer q.Stop()
	const n = 1000
	tasks := make([]models.Task, n)
	for i := range tasks {
		tasks[i] = makeTestTask(fmt.Sprintf("load-%04d", i))
		createTask(t, db, tasks[i])
	}
	latencies := make([]time.Duration, n)
	for i, task := range tasks {
		start := time.Now()
		if err := q.Submit(context.Background(), task); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		latencies[i] = time.Since(start)
	}
	slices.Sort(latencies)
	p50, p95, p99 := latencies[n/2], latencies[n*95/100], latencies[n*99/100]
	t.Logf("submit latency for %d tasks: p50=%s p95=%s p99=%s", n, p50, p95, p99)
	if p95 > 100*time.Millisecond {
		t.Fatalf("p95 submit latency %s exceeds the 100ms target", p95)
	}
	if got := q.Metrics().Queued; got != n {
		t.Fatalf("queued: got %d, want %d", got, n)
	}
	start := time.Now()
	for _, task := range tasks[:n/2] {
		if err := q.Cancel(task.ID); err != nil {
			t.Fatalf("cancel: %v", err)
		}
	}
	t.Logf("cancelled %d queued tasks in %s", n/2, time.Since(start))
}
