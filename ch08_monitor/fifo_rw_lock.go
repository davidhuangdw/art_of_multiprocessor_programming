package monitor

import "sync"

// FifoReadersWriter is an approximately FIFO readers-writer lock.
//
// Order is enforced per batch, not per request:
//
//   - readers admitted together hold the lock concurrently, in any order;
//   - a reader that arrives while a writer is queued or writing waits for it;
//   - writers are FIFO among themselves, each taking a ticket on arrival.
//
// It deliberately does not order a reader that is *already waiting* against the
// writer queued after it: the writer may run first. Enforcing that costs a
// ticket per request plus a broadcast per admission, which is what batching
// avoids.
//
// Recursive read locking deadlocks, as in sync.RWMutex.
type FifoReadersWriter struct {
	lock               sync.Locker
	cond               *sync.Cond
	reads              int  // readers inside
	write              bool // a writer is inside
	nextWrite, serving int  // writer tickets, serving <= nextWrite
}

// NewFifoReadersWriter returns an unlocked FIFO readers-writer lock.
func NewFifoReadersWriter() *FifoReadersWriter {
	lock := &sync.Mutex{}
	cond := sync.NewCond(lock)
	return &FifoReadersWriter{lock: lock, cond: cond}
}

// ReadLock acquires the lock for reading.
//
// A reader takes no ticket: its position is the ticket of the next writer to be
// issued. While a writer is queued that position is ahead of serving, so the
// reader parks and joins the batch after those writers.
func (rw *FifoReadersWriter) ReadLock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	me := rw.nextWrite
	for me > rw.serving || rw.write {
		rw.cond.Wait()
	}

	rw.reads++
}

// ReadUnlock releases a read lock.
func (rw *FifoReadersWriter) ReadUnlock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	rw.reads--
	if rw.reads == 0 {
		rw.cond.Broadcast()
	}
}

// WriteLock acquires the lock for writing, blocking until its ticket is served,
// no writer holds the lock and every reader has left.
func (rw *FifoReadersWriter) WriteLock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	me := rw.nextWrite
	rw.nextWrite++

	for rw.serving != me || rw.reads > 0 { // rw.serving==me => rw.write==false
		rw.cond.Wait()
	}
	rw.write = true
}

// WriteUnlock releases a write lock.
func (rw *FifoReadersWriter) WriteUnlock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	rw.serving++
	rw.write = false
	rw.cond.Broadcast()
}
