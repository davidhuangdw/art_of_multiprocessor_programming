// Package queue implements the concurrent queues from Chapter 10.
package queue

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type qNode[T any] struct {
	v    T
	next atomic.Pointer[qNode[T]]
}

func newQNode[T any](v T, nxt *qNode[T]) *qNode[T] {
	x := &qNode[T]{v: v}
	if nxt != nil {
		x.next.Store(nxt)
	}
	return x
}

// LockFreeQueue is the Michael-Scott lock-free queue from Chapter 10.
//
// A dummy node separates head from tail: Dequeue only CASes head and Enqueue
// only CASes tail.next, so the two never block each other. Nodes are never
// recycled, so the garbage collector rules out ABA — see StampedQueue for the
// versioned variant needed once nodes are reused.
type LockFreeQueue struct {
	head, tail atomic.Pointer[qNode[int]]
}

// NewLockFreeQueue returns an empty queue.
func NewLockFreeQueue() *LockFreeQueue {
	dummy := newQNode(0, nil) // use dummy: real-head-value := head.next.v
	q := &LockFreeQueue{}
	q.head.Store(dummy)
	q.tail.Store(dummy)
	return q
}

// Enqueue appends v.
func (q *LockFreeQueue) Enqueue(v int) {
	add := newQNode(v, nil)
	for {
		pre := q.tail.Load()
		nxt := pre.next.Load()
		if nxt != nil {
			q.tail.CompareAndSwap(pre, nxt) // nonblocking: help forward 'tail'
			continue
		}
		if pre.next.CompareAndSwap(nil, add) {
			q.tail.CompareAndSwap(pre, add)
			return
		}
	}
}

// Dequeue removes the oldest item.
func (q *LockFreeQueue) Dequeue() (int, bool) {
	for {
		node := q.head.Load()
		nxt := node.next.Load()
		if nxt == nil {
			return -1, false
		}
		if q.head.CompareAndSwap(node, nxt) {
			// real-head=head.next due to dummy node
			return nxt.v, true
		}
	}
}

var _ prim.Queue = (*LockFreeQueue)(nil)
