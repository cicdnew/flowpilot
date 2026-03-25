package queue

import (
	"context"
	"time"

	"flowpilot/internal/models"
)

// heapItem wraps a task with its context and heap metadata.
type heapItem struct {
	task    models.Task
	ctx     context.Context
	cancel  context.CancelFunc
	addedAt time.Time // tiebreaker for same-priority tasks (FIFO)
	index   int       // managed by container/heap
}

// taskHeap implements heap.Interface for priority-based task scheduling.
// Higher priority values are dequeued first. For equal priority, earlier
// addedAt wins (FIFO ordering within the same priority level).
type taskHeap []*heapItem

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	if h[i].task.Priority != h[j].task.Priority {
		return h[i].task.Priority > h[j].task.Priority // higher priority first
	}
	return h[i].addedAt.Before(h[j].addedAt) // FIFO tiebreaker
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x any) {
	item := x.(*heapItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1
	*h = old[:n-1]
	return item
}

// peek returns the highest-priority item without removing it. Returns nil if empty.
func (h taskHeap) peek() *heapItem {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}
