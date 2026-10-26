package queue

import (
	"runtime"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// dNode is one link of the dual queue: either an offer (an Enqueue's value,
// waiting to be taken) or a request (a Dequeue's reservation, waiting to be
// filled).
//
// item is the pairing word, as in the book's SynchronousDualQueue: a request is
// created with a nil item and is filled with the offered value; an offer is
// created holding its value and is emptied when a dequeuer takes it. So nil
// always means "not paired yet".
type dNode[T any] struct {
	request bool // true: a Dequeue's reservation
	item    atomic.Pointer[T]
	next    atomic.Pointer[dNode[T]]
}

// DualQueue is the synchronous dual queue of Chapter 10 §10.3, following the
// offer/request protocol of the book's SynchronousDualQueue.
//
// No buffer: Enqueue returns only once a Dequeue has taken the value, and
// Dequeue returns only once an Enqueue has supplied one. Offers are served in
// FIFO order, and so are requests.
//
// The decision is always taken from head.next, the oldest pending node: if it is
// the opposite kind we pair with it, if it is our own kind (or the queue is
// empty) we queue behind it. Parking on our own node is then safe, because we
// only ever append behind our own kind, so no pending node of the other kind can
// be hidden behind a parked one.
type DualQueue[T any] struct {
	head, tail atomic.Pointer[dNode[T]] // head is a sentinel; head.next is the oldest node
}

// NewDualQueue returns a queue of int, the prim.BlockingQueue implementation.
func NewDualQueue() *DualQueue[int] { return NewDualQueueOf[int]() }

// NewDualQueueOf returns an empty queue of any element type.
func NewDualQueueOf[T any]() *DualQueue[T] {
	sentinel := &dNode[T]{}
	q := &DualQueue[T]{}
	q.head.Store(sentinel)
	q.tail.Store(sentinel)
	return q
}

// appendNode queues n at the tail, reporting whether it was installed.
//
// It refuses to queue behind a node of the opposite kind: that would hide the
// node from the thread that should pair with it, and both sides could end up
// parked forever.
func (q *DualQueue[T]) appendNode(n *dNode[T]) bool {
	t := q.tail.Load()
	if next := t.next.Load(); next != nil { // tail lags: help it forward, then retry
		q.tail.CompareAndSwap(t, next)
		return false
	}
	if t != q.head.Load() && t.request != n.request {
		return false // oldest pending nodes are of the other kind: pair instead
	}
	if !t.next.CompareAndSwap(nil, n) {
		return false
	}
	q.tail.CompareAndSwap(t, n) // best effort; a failure only means someone helped
	return true
}

// Enqueue offers v and blocks until a Dequeue takes it.
func (q *DualQueue[T]) Enqueue(v T) {
	offer := &dNode[T]{}
	offer.item.Store(&v)

	for {
		h := q.head.Load()
		first := h.next.Load()

		if first != nil && first.request { // a dequeuer waits: fulfil the oldest request
			if first.item.CompareAndSwap(nil, &v) {
				q.head.CompareAndSwap(h, first)
				return
			}
			q.head.CompareAndSwap(h, first) // somebody filled it first: help, retry
			continue
		}

		if q.appendNode(offer) {
			// Our offer sits behind offers only, so a dequeuer reaching it takes
			// it; wait for that to happen.
			for offer.item.Load() != nil {
				runtime.Gosched()
			}
			return
		}
	}
}

// Dequeue takes the oldest offered value, blocking until one is offered.
func (q *DualQueue[T]) Dequeue() T {
	request := &dNode[T]{request: true}

	for {
		h := q.head.Load()
		first := h.next.Load()

		if first != nil && !first.request { // an offer waits: take the oldest one
			v := first.item.Load()
			// v == nil means another dequeuer already took it and head has not
			// moved on yet, so there is nothing to swap.
			if v != nil && first.item.CompareAndSwap(v, nil) {
				q.head.CompareAndSwap(h, first)
				return *v
			}
			q.head.CompareAndSwap(h, first) // taken already: help, retry
			continue
		}

		if q.appendNode(request) {
			for request.item.Load() == nil { // an enqueuer fills our request
				runtime.Gosched()
			}
			return *request.item.Load()
		}
	}
}

var _ prim.BlockingQueue = (*DualQueue[int])(nil)
