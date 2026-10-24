package monitor

import "sync"

// ReadersWriter is the readers-writer lock from Chapter 8.
// Any number of readers may hold the lock at once; a writer holds it alone.
// writer-prefer
type ReadersWriter struct {
	lock  sync.Locker
	cond  *sync.Cond
	reads int  // entered reads
	write bool // exist undone write-req
}

// NewReadersWriter returns an unlocked readers-writer lock.
func NewReadersWriter() *ReadersWriter {
	lock := &sync.Mutex{}
	return &ReadersWriter{
		lock: lock, cond: sync.NewCond(lock),
	}
}

// ReadLock acquires the lock for reading, blocking while a writer holds it.
func (rw *ReadersWriter) ReadLock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	for rw.write {
		rw.cond.Wait()
	}
	rw.reads++
}

// ReadUnlock releases a read lock.
func (rw *ReadersWriter) ReadUnlock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	rw.reads--
	if rw.reads == 0 && rw.write {
		rw.cond.Signal() // signal one single writer
	}
}

// WriteLock acquires the lock for writing, blocking while anyone holds it.
func (rw *ReadersWriter) WriteLock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()

	for rw.write {
		rw.cond.Wait()
	}
	rw.write = true
	for rw.reads > 0 {
		rw.cond.Wait()
	}
}

// WriteUnlock releases a write lock.
func (rw *ReadersWriter) WriteUnlock() {
	rw.lock.Lock()
	defer rw.lock.Unlock()
	rw.write = false
	rw.cond.Broadcast() // broadcast all next-batch readers
}
