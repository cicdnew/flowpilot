package queue

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"flowpilot/internal/models"
)

func TestHeapOrdering(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	items := []*heapItem{
		{task: models.Task{ID: "low", Priority: models.PriorityLow}, ctx: context.Background(), cancel: func() {}, addedAt: now},
		{task: models.Task{ID: "high", Priority: models.PriorityHigh}, ctx: context.Background(), cancel: func() {}, addedAt: now.Add(time.Second)},
		{task: models.Task{ID: "normal", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now},
	}
	for _, item := range items {
		heap.Push(h, item)
	}

	first := heap.Pop(h).(*heapItem)
	if first.task.ID != "high" {
		t.Errorf("expected high priority first, got %s", first.task.ID)
	}
	second := heap.Pop(h).(*heapItem)
	if second.task.ID != "normal" {
		t.Errorf("expected normal priority second, got %s", second.task.ID)
	}
	third := heap.Pop(h).(*heapItem)
	if third.task.ID != "low" {
		t.Errorf("expected low priority third, got %s", third.task.ID)
	}
}

func TestHeapFIFOTiebreaker(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	heap.Push(h, &heapItem{task: models.Task{ID: "a", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now})
	heap.Push(h, &heapItem{task: models.Task{ID: "b", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now.Add(time.Millisecond)})
	heap.Push(h, &heapItem{task: models.Task{ID: "c", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now.Add(2 * time.Millisecond)})

	first := heap.Pop(h).(*heapItem)
	if first.task.ID != "a" {
		t.Errorf("expected FIFO order: first=a, got %s", first.task.ID)
	}
	second := heap.Pop(h).(*heapItem)
	if second.task.ID != "b" {
		t.Errorf("expected FIFO order: second=b, got %s", second.task.ID)
	}
	third := heap.Pop(h).(*heapItem)
	if third.task.ID != "c" {
		t.Errorf("expected FIFO order: third=c, got %s", third.task.ID)
	}
}

func TestHeapPeek(t *testing.T) {
	h := taskHeap{}
	heap.Init(&h)

	if h.peek() != nil {
		t.Error("peek on empty heap should return nil")
	}

	heap.Push(&h, &heapItem{task: models.Task{ID: "x", Priority: models.PriorityHigh}, ctx: context.Background(), cancel: func() {}, addedAt: time.Now()})
	p := h.peek()
	if p == nil || p.task.ID != "x" {
		t.Error("peek should return top item without removing it")
	}
	if h.Len() != 1 {
		t.Errorf("peek should not remove item, len=%d", h.Len())
	}
}

func TestHeapRemove(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	items := []*heapItem{
		{task: models.Task{ID: "a", Priority: models.PriorityNormal}, ctx: context.Background(), cancel: func() {}, addedAt: now},
		{task: models.Task{ID: "b", Priority: models.PriorityHigh}, ctx: context.Background(), cancel: func() {}, addedAt: now},
		{task: models.Task{ID: "c", Priority: models.PriorityLow}, ctx: context.Background(), cancel: func() {}, addedAt: now},
	}
	for _, item := range items {
		heap.Push(h, item)
	}

	// Remove the middle-priority item (index varies after heap ops)
	for i, item := range *h {
		if item.task.ID == "a" {
			heap.Remove(h, i)
			break
		}
	}

	if h.Len() != 2 {
		t.Errorf("after remove, len=%d, want 2", h.Len())
	}

	first := heap.Pop(h).(*heapItem)
	if first.task.ID != "b" {
		t.Errorf("after remove, expected high first, got %s", first.task.ID)
	}
}

func TestHeapSwapUpdatesIndex(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	a := &heapItem{task: models.Task{ID: "a", Priority: models.PriorityLow}, ctx: context.Background(), cancel: func() {}, addedAt: now}
	b := &heapItem{task: models.Task{ID: "b", Priority: models.PriorityHigh}, ctx: context.Background(), cancel: func() {}, addedAt: now}

	heap.Push(h, a)
	heap.Push(h, b)

	// After push, indices should be valid
	for i, item := range *h {
		if item.index != i {
			t.Errorf("item %s: index=%d, position=%d", item.task.ID, item.index, i)
		}
	}
}
