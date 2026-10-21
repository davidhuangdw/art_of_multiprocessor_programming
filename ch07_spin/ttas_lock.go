package spin

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// TTASLock is the test-and-test-and-set spin lock from Chapter 7.
type TTASLock struct {
	busy atomic.Bool
}

// NewTTASLock returns a TTAS spin lock.
func NewTTASLock() *TTASLock {
	return &TTASLock{}
}

// Lock acquires the lock, spinning until it succeeds.
func (l *TTASLock) Lock() {
	for l.busy.Load() || l.busy.Swap(true) { // wait if busy or failed to acquire
	}
}

// Unlock releases the lock.
func (l *TTASLock) Unlock() {
	l.busy.Store(false)
}

var _ prim.Lock = (*TTASLock)(nil)
