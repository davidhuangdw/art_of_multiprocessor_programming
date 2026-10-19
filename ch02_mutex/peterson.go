package mutex

import "sync/atomic"

// Peterson is the two-thread lock from Figure 2.4, combining a flag per
// thread with a victim register.
//
// It satisfies mutual exclusion, is deadlock-free and starvation-free.
// Mutually exclusive entry is guaranteed as long as both threads use indices
// 0 and 1 respectively.
type Peterson struct {
	flag   [2]atomic.Bool
	victim atomic.Int32
}

// NewPeterson returns a two-thread Peterson lock.
func NewPeterson() *Peterson { return &Peterson{} }

// Lock acquires the lock on behalf of thread i (0 or 1).
func (p *Peterson) Lock(i int) {
	// TODO: implement Peterson.lock from Figure 2.4.
}

// Unlock releases the lock held by thread i.
func (p *Peterson) Unlock(i int) {
	// TODO: implement Peterson.unlock from Figure 2.4.
}
