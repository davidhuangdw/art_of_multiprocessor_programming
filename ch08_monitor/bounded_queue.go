package monitor

import "sync"

// BoundedQueue is the producer-consumer buffer from Chapter 8.
//
// Put blocks while the queue is full and Get blocks while it is empty; both
// wait on condition variables of an internal monitor.
type BoundedQueue struct {
	capacity          int
	items             []int
	head, count       int
	lock              sync.Locker
	notFull, notEmpty *sync.Cond
}

// NewBoundedQueue returns a queue that holds at most capacity items.
func NewBoundedQueue(capacity int) *BoundedQueue {
	if capacity <= 0 {
		panic("monitor: queue capacity must be positive")
	}
	lock := &sync.Mutex{}
	return &BoundedQueue{
		capacity: capacity, items: make([]int, capacity),
		lock: lock, notFull: sync.NewCond(lock), notEmpty: sync.NewCond(lock)}
}

// Put appends v, blocking while the queue is full.
func (q *BoundedQueue) Put(v int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for q.count >= q.capacity { // need loop check: maybe poped by other before awake
		q.notFull.Wait()
	}
	q.items[(q.head+q.count)%q.capacity] = v
	q.count++
	q.notEmpty.Signal()
}

// Get removes and returns the oldest item, blocking while the queue is empty.
func (q *BoundedQueue) Get() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	for q.count <= 0 {
		q.notEmpty.Wait()
	}
	res := q.items[q.head]
	q.head = (q.head + 1) % q.capacity
	q.count--
	q.notFull.Signal()
	return res
}
