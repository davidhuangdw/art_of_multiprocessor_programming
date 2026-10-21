package spin

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// AndersonQueueLock is the array-based queue lock from Chapter 7.
//
// Each caller owns a fixed thread index in [0, n).
type AndersonQueueLock struct {
	n    int
	tail atomic.Int64
	que  []atomic.Bool
	ind  []int
}

// NewAndersonQueueLock returns an Anderson queue lock for n threads.
func NewAndersonQueueLock(n int) *AndersonQueueLock {
	validateThreads(n)
	que := make([]atomic.Bool, n)
	que[0].Store(true) // allow the first
	return &AndersonQueueLock{
		n:   n,
		que: que,
		ind: make([]int, n),
	}
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (l *AndersonQueueLock) Lock(i int) {
	me := int(l.tail.Add(1)-1) % l.n
	l.ind[i] = me
	for !l.que[me].Load() {
	}
}

// Unlock releases the lock held by thread i.
func (l *AndersonQueueLock) Unlock(i int) {
	me := l.ind[i]
	l.que[me].Store(false)        // cleanup
	l.que[(me+1)%l.n].Store(true) // then unlock next
}

var _ prim.ThreadLock = (*AndersonQueueLock)(nil)
