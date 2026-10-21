package spin

import (
	"sync/atomic"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// BackoffLock is the exponential-backoff spin lock from Chapter 7.
type BackoffLock struct {
	busy               atomic.Bool
	minDelay, maxDelay time.Duration
}

// NewBackoffLock returns a backoff lock with the given delay bounds.
func NewBackoffLock(minDelay, maxDelay time.Duration) *BackoffLock {
	if minDelay <= 0 || maxDelay < minDelay {
		panic("spin: invalid backoff range")
	}
	return &BackoffLock{minDelay: minDelay, maxDelay: maxDelay}
}

// Lock acquires the lock, backing off while it is contended.
func (l *BackoffLock) Lock() {
	b := prim.NewBackoff(l.minDelay, l.maxDelay)
	for {
		for l.busy.Load() {
		}
		if !l.busy.Swap(true) { // succeed
			return
		}
		b.Backoff() // backoff for each Swap()
	}
}

// Unlock releases the lock.
func (l *BackoffLock) Unlock() {
	l.busy.Store(false)
}
