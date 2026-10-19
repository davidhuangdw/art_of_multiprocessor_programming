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
	return &Bakery{n: n, flag: make([]atomic.Bool, n), label: make([]atomic.Int64, n)}
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (b *Bakery) Lock(i int) {
	b.flag[i].Store(true)
	var maxLabel int64
	for j := range b.n {
		if j != i && b.flag[j].Load() {
			maxLabel = max(maxLabel, b.label[j].Load())
		}
	}
	me := maxLabel + 1
	b.label[i].Store(me)
Retry:
	for {
		for j := range b.n {
			if j != i && b.flag[j].Load() {
				lj := b.label[j].Load()
				lose := lj < me || (lj == me && j < i) // smallest (label[j], j) win
				if lose {
					continue Retry
				}
			}
		}
		return // win all
	}

}

// Unlock releases the lock held by thread i.
func (b *Bakery) Unlock(i int) {
	b.flag[i].Store(false)
	// b.label[i].Store(0)
}
