// Package mutex implements the mutual-exclusion algorithms of Chapter 2.
//
// Every lock here is thread-indexed: caller i must pass the same index i to
// Lock and Unlock. The algorithms are two-thread (LockOne, LockTwo,
// Peterson) or n-thread (Bakery).
//
// The book assumes sequentially consistent reads and writes of flags.
// Go's memory model does not guarantee that for plain variables, so every
// shared flag here is an atomic.Bool / atomic.Int64.
package mutex

import "sync/atomic"

// LockOne is the naive two-thread lock from Figure 2.1.
//
// Mutual exclusion holds: a thread only enters when the other thread's flag
// is clear, and it sets its own flag before reading. It is NOT deadlock-free:
// if both threads store their flag before either reads the other's, both spin
// forever. It is also not starvation-free.
type LockOne struct {
	flag [2]atomic.Bool
}

// NewLockOne returns a two-thread LockOne.
func NewLockOne() *LockOne { return &LockOne{} }

// Lock acquires the lock on behalf of thread i (0 or 1).
func (l *LockOne) Lock(i int) {
	// TODO: implement LockOne.lock from Figure 2.1.
}

// Unlock releases the lock held by thread i.
func (l *LockOne) Unlock(i int) {
	// TODO: implement LockOne.unlock from Figure 2.1.
}
