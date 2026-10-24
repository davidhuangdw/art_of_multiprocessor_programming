package monitor

import (
	"sync"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// SimpleReentrantLock is the reentrant lock from Chapter 8.
//
// Go has no thread ID, so the owner is an explicit token: a goroutine that
// already holds the lock may lock it again (the hold count increases) and must
// unlock it the same number of times.
type SimpleReentrantLock struct {
	owner, count int
	lock         sync.Locker
	cond         *sync.Cond
}

// NewSimpleReentrantLock returns an unlocked reentrant lock.
func NewSimpleReentrantLock() *SimpleReentrantLock {
	lock := &sync.Mutex{}
	cond := sync.NewCond(lock)
	return &SimpleReentrantLock{
		lock: lock, cond: cond,
	}
}

// Lock acquires the lock for owner, blocking while another owner holds it.
// Re-locking from the same owner succeeds immediately.
func (l *SimpleReentrantLock) Lock(owner int) {
	l.lock.Lock()
	defer l.lock.Unlock()

	for l.count > 0 && l.owner != owner {
		l.cond.Wait()
	}

	if l.count == 0 {
		l.owner = owner
	}
	l.count++
}

// Unlock releases one level of ownership. It panics if owner does not hold the
// lock.
func (l *SimpleReentrantLock) Unlock(owner int) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.count == 0 || l.owner != owner {
		panic("unlock when count==0 or not the owner")
	}

	l.count--
	if l.count == 0 {
		l.cond.Signal()
	}
}

var _ prim.ThreadLock = (*SimpleReentrantLock)(nil)
