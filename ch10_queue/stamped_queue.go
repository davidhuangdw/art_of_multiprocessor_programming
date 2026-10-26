package queue

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// stampedCell names a queue node together with a version counter.
//
// A CAS replaces the whole cell, so pointer identity carries the version: that
// is Go's stand-in for AtomicStampedReference, and it is what makes
// "same pointer, different life" impossible to confuse. Chapter 9's
// LockFreeList uses the same trick for its {removed, next} cells.
type stampedCell struct {
	stamp int
	node  *sNode
}

// sNode is a queue node whose link is a stamped cell.
type sNode struct {
	v    int
	next atomic.Pointer[stampedCell]
}

func newStampedPtr(stamp int, ptr *sNode) *stampedCell {
	return &stampedCell{stamp, ptr}
}

func stampedCAS(ptr *atomic.Pointer[stampedCell], old, new *stampedCell) bool {
	if old == nil { // allow nil
		return ptr.CompareAndSwap(old, new)
	}
	return ptr.Load().stamp == old.stamp && ptr.CompareAndSwap(old, new)
}

// StampedQueue is the Michael-Scott queue with a stamped tail, the Chapter 10
// answer to ABA.
//
// Go needs this only when nodes are recycled (a free list, a pool): a recycled
// node whose address reappears together with the same successor would otherwise
// let a stale CAS succeed on a queue that has moved on.
type StampedQueue struct {
	head, tail atomic.Pointer[stampedCell]
}

// NewStampedQueue returns an empty queue.
func NewStampedQueue() *StampedQueue {
	dummy := &sNode{}
	q := &StampedQueue{}
	dummyPtr := newStampedPtr(0, dummy)
	q.head.Store(dummyPtr)
	q.tail.Store(dummyPtr)
	return q
}

// Enqueue appends v.
func (q *StampedQueue) Enqueue(v int) {
	for {
		tailPtr := q.tail.Load()
		tail := tailPtr.node
		nxt := tail.next.Load()
		if nxt != nil {
			stampedCAS(&q.tail, tailPtr, tail.next.Load()) // nonblocking: help forward tail
			continue
		}
		add := newStampedPtr(tailPtr.stamp+1, &sNode{v: v})
		if stampedCAS(&tail.next, nil, add) {
			stampedCAS(&q.tail, tailPtr, add)
			return
		}
	}

}

// Dequeue removes the oldest item.
func (q *StampedQueue) Dequeue() (int, bool) {
	for {
		headPtr := q.head.Load()
		nxt := headPtr.node.next.Load()
		if nxt == nil {
			return -1, false
		}
		if stampedCAS(&q.head, headPtr, nxt) {
			return nxt.node.v, true
		}
	}
}

var _ prim.Queue = (*StampedQueue)(nil)
