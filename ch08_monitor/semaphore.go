package monitor

import "sync"

// Semaphore is the counting semaphore from Chapter 8.
//
// Acquire takes a permit, blocking while none is available; Release returns
// one. A semaphore with one permit behaves like a mutex.
type Semaphore struct {
	permits int
	lock    sync.Locker
	cond    *sync.Cond
}

// NewSemaphore returns a semaphore with the given number of permits.
func NewSemaphore(permits int) *Semaphore {
	if permits < 0 {
		panic("monitor: semaphore permits must not be negative")
	}
	lock := &sync.Mutex{}
	cond := sync.NewCond(lock)
	return &Semaphore{permits: permits, lock: lock, cond: cond}
}

// Acquire takes one permit, blocking while the semaphore is empty.
func (s *Semaphore) Acquire() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for s.permits <= 0 {
		s.cond.Wait()
	}
	s.permits--
}

// Release returns one permit and wakes a waiter.
func (s *Semaphore) Release() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.permits++
	s.cond.Signal()
}
