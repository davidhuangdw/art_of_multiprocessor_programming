package spin

import "github.com/davidhuangdw/art_of_multiprocessor_programming/prim"

// CompositeLock is the composite fast-path/queue lock from Chapter 7.
//
// Each caller owns a fixed thread index in [0, n).
type CompositeLock struct{}

// NewCompositeLock returns a CompositeLock for n threads.
func NewCompositeLock(n int) *CompositeLock {
	validateThreads(n)
	return &CompositeLock{}
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (l *CompositeLock) Lock(i int) {
	panic("spin: CompositeLock.Lock not implemented")
}

// Unlock releases the lock held by thread i.
func (l *CompositeLock) Unlock(i int) {
	panic("spin: CompositeLock.Unlock not implemented")
}

var _ prim.ThreadLock = (*CompositeLock)(nil)
