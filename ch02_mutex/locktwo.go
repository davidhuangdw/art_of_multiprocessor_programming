package mutex

import "sync/atomic"

// LockTwo is the two-thread lock from Figure 2.2, built from a single victim
// register instead of two flags.
//
// Mutual exclusion holds: the last thread to write victim waits, so at most
// one thread can pass the spin loop. It is not deadlock-free in practice:
// a thread leaves the loop only when the *other* thread writes victim again.
// If the other thread finishes and never returns, the waiter spins forever,
// and a lone thread blocks immediately.
type LockTwo struct {
	victim atomic.Int32
}

// NewLockTwo returns a two-thread LockTwo.
func NewLockTwo() *LockTwo { return &LockTwo{} }

// Lock acquires the lock on behalf of thread i (0 or 1).
func (l *LockTwo) Lock(i int) {
	// TODO: implement LockTwo.lock from Figure 2.2.
}

// Unlock is a no-op in the book: LockTwo has no flag to clear.
func (l *LockTwo) Unlock(i int) {
	// TODO: implement LockTwo.unlock from Figure 2.2.
}
