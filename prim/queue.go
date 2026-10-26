package prim

// Queue is the concurrent FIFO queue implemented by the lock-free Chapter 10
// queues.
//
// Enqueue appends v. Dequeue never parks: it reports ok == false when the queue
// is empty.
type Queue interface {
	Enqueue(v int)
	Dequeue() (v int, ok bool)
}

// BlockingQueue is a FIFO queue whose Dequeue parks instead of failing.
//
// Enqueue parks only when the queue is bounded and full. A bufferless queue
// (the "dual" data structure of §10.3) parks it as well, until a Dequeue takes
// the value — there, neither call can complete on its own.
type BlockingQueue interface {
	Enqueue(v int)
	Dequeue() int
}
