package queue

import (
	"sync"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// BoundedQueue is the array-based queue from Chapter 10 with one lock per end.
//
// tail is written only under enqLock and head only under deqLock, so the two
// ends never contend. size is the one value both sides touch, hence atomic; a
// plain int there would force a single lock again. Each end has its own
// condition variable, and a wake-up must be signalled while holding the
// *other* end's lock, otherwise the woken side can miss it.
type BoundedQueue struct {
	capacity, head, tail int
	items                []int
	size                 atomic.Int32
	enqLock, deqLock     sync.Mutex
	notFull, notEmpty    *sync.Cond
}

// NewBoundedQueue returns a queue that holds at most capacity items.
func NewBoundedQueue(capacity int) *BoundedQueue {
	if capacity <= 0 {
		panic("queue: capacity must be positive")
	}
	q := &BoundedQueue{
		capacity: capacity,
		items:    make([]int, capacity),
	}
	q.notEmpty = sync.NewCond(&q.deqLock)
	q.notFull = sync.NewCond(&q.enqLock)
	return q
}

// Enqueue appends v, blocking while the queue is full.
func (q *BoundedQueue) Enqueue(v int) {
	q.enqLock.Lock() // ensure single enq
	for int(q.size.Load()) == q.capacity {
		q.notFull.Wait()
	}
	q.items[q.tail] = v
	q.tail = (q.tail + 1) % q.capacity
	emptyBefore := q.size.Add(1) == 1
	q.enqLock.Unlock()

	if emptyBefore { // a consumer may be parked on an empty queue
		q.deqLock.Lock()
		q.notEmpty.Broadcast()
		q.deqLock.Unlock()
	}
}

// Dequeue removes the oldest item, reporting ok == false when the queue is empty.
func (q *BoundedQueue) Dequeue() int {
	q.deqLock.Lock()
	for q.size.Load() == 0 {
		q.notEmpty.Wait()
	}
	res := q.items[q.head]
	q.head = (q.head + 1) % q.capacity
	fullBefore := q.size.Add(-1)+1 == int32(q.capacity)
	q.deqLock.Unlock()

	if fullBefore { // a producer may be parked on a full queue
		q.enqLock.Lock()
		q.notFull.Broadcast()
		q.enqLock.Unlock()
	}
	return res
}

var _ prim.BlockingQueue = (*BoundedQueue)(nil)
