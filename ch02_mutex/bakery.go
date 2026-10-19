package mutex

import "sync/atomic"

// Bakery is the n-thread Lamport bakery lock from Figure 2.9.
//
// Each thread takes a ticket one greater than every label it can currently
// observe, then waits until no other thread holds a smaller ticket (ties are
// broken by thread index). It satisfies mutual exclusion, is deadlock-free
// and starvation-free, and grants the lock in first-come-first-served order.
//
// Caveat: maxLabel is a plain scan, not an atomic snapshot. Corollary: this
// is faithful to the book's sequential-consistency assumption, not a
// wait-free algorithm on a real machine.
type Bakery struct {
	n     int
	flag  []atomic.Bool // choosing: thread is picking a ticket
	label []atomic.Int64
}

// NewBakery returns a Bakery lock for n threads.
func NewBakery(n int) *Bakery {
	// TODO: validate n >= 1 and allocate the flag/label slices.
	return &Bakery{n: n}
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (b *Bakery) Lock(i int) {
	// TODO: implement Bakery.lock from Figure 2.9.
}

// Unlock releases the lock held by thread i.
func (b *Bakery) Unlock(i int) {
	// TODO: implement Bakery.unlock from Figure 2.9.
}
