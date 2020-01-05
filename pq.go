package goproc

import (
	"container/heap"
)

// Prioritier is the interface implemented by an object that can return a int64 priority.
type Prioritier interface {
	Priority() int64
}

// PriorityQueue is heap-implementation of priority queue.
type PriorityQueue struct {
	heap []Prioritier
	less func(i, j int64) bool
}

// NewPriorityQueue creates a new PriorityQueue.
func NewPriorityQueue(desc bool, size int) *PriorityQueue {
	var less func(i, j int64) bool
	if desc {
		less = ge
	} else {
		less = lt
	}
	pq := &PriorityQueue{
		heap: make([]Prioritier, 0, size),
		less: less,
	}
	heap.Init(pq) // not really necessary, just FYI
	return pq
}

func lt(a, b int64) bool {
	return a < b
}

func ge(a, b int64) bool {
	return b < a
}

// Len implements Len method of sort.Interface.
func (q PriorityQueue) Len() int { return len(q.heap) }

// Swap implements Swap method of sort.Interface.
func (q PriorityQueue) Swap(i, j int) { q.heap[i], q.heap[j] = q.heap[j], q.heap[i] }

// Less implements Less method of sort.Interface.
func (q PriorityQueue) Less(i, j int) bool {
	return q.less(q.heap[i].Priority(), q.heap[j].Priority())
}

// Push implements Push method of heap.Interface.
func (q *PriorityQueue) Push(x interface{}) {
	q.heap = append(q.heap, x.(Prioritier))
}

// Pop implements Pop method of heap.Interface.
func (q *PriorityQueue) Pop() interface{} {
	l := len(q.heap)
	item := q.heap[l-1]
	q.heap = q.heap[:l-1]
	return item
}

// Peek returns the top element of the priority queue. User should ensure the queue is not empty before calling Peek.
func (q *PriorityQueue) Peek() interface{} {
	return q.heap[0]
}

// Clear clears priority queue.
func (q *PriorityQueue) Clear() int {
	l := q.Len()
	q.heap = q.heap[:0]
	heap.Init(q)
	return l
}
