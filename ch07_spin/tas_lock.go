package spin

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// TASLock is the test-and-set spin lock from Chapter 7.
type TASLock struct {
	busy atomic.Bool
}

// NewTASLock returns a TAS spin lock.
func NewTASLock() *TASLock {
	return &TASLock{}
}

// Lock acquires the lock, spinning until it succeeds.
func (l *TASLock) Lock() {
	for l.busy.Swap(true) { // just getAndSet(), no need CAS()
	}
}

// Unlock releases the lock.
func (l *TASLock) Unlock() {
	l.busy.Store(false)
}

var _ prim.Lock = (*TASLock)(nil)
