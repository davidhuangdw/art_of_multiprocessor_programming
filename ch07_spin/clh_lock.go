package spin

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// CLHLock is the CLH queue lock from Chapter 7.
//
// Each caller owns a fixed thread index in [0, n).
type CLHLock struct {
	tail  atomic.Pointer[Node]
	nodes []*Node
	pres  []*Node
}

// NewCLHLock returns a CLH lock for n threads.
func NewCLHLock(n int) *CLHLock {
	validateThreads(n)

	tail := &Node{}
	tail.free.Store(true) // allow first

	nodes := make([]*Node, n)
	for i := range nodes {
		nodes[i] = &Node{}
	}

	l := &CLHLock{nodes: nodes, pres: make([]*Node, n)}
	l.tail.Store(tail)
	return l
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (l *CLHLock) Lock(i int) {
	me := l.nodes[i]
	// me.free.Store(false) // block my next

	pre := l.tail.Swap(me) // enque
	l.pres[i] = pre
	for !pre.free.Load() { // spin wait on pre -- single-reader
	}

	// critical zone begin:
	pre.free.Store(false) // always: free=true node <=1
}

// Unlock releases the lock held by thread i.
func (l *CLHLock) Unlock(i int) {
	l.nodes[i].free.Store(true) // release for my next -- single-writer
	// critical zone ended
	l.nodes[i], l.pres[i] = l.pres[i], nil // reuse pre-node instead of create new one: everyone shift right
}

var _ prim.ThreadLock = (*CLHLock)(nil)
