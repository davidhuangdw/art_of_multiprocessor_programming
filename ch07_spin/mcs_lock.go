package spin

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// CLH vs. MCS
// CLH: spin on prev.state
// MCS: spin on self.state(set by prev -- need link prev.nxt:=me ) && take care me.nxt

// MCSLock is the MCS queue lock from Chapter 7.
// Each caller owns a fixed thread index in [0, n).
type MCSLock struct {
	tail  atomic.Pointer[Node]
	nodes []*Node
}

// NewMCSLock returns an MCS lock for n threads.
func NewMCSLock(n int) *MCSLock {
	validateThreads(n)
	nodes := make([]*Node, n)
	for i := range nodes {
		nodes[i] = &Node{}
	}
	return &MCSLock{nodes: nodes}
}

// Lock acquires the lock on behalf of thread i in [0, n).
func (l *MCSLock) Lock(i int) {
	me := l.nodes[i]
	pre := l.tail.Swap(me)
	if pre == nil { // pre not exist
		return
	}
	me.free.Store(false) // init
	pre.nxt.Store(me)
	for !me.free.Load() { // wait for pre to release me
	}
}

// Unlock releases the lock held by thread i.
func (l *MCSLock) Unlock(i int) {
	me := l.nodes[i]
	if me.nxt.Load() == nil && l.tail.CompareAndSwap(me, nil) { // me is tail
		return
	}
	// me is not tail: has next
	for me.nxt.Load() == nil { // wait my next to link me
	}
	me.nxt.Load().free.Store(true) // release him
	me.nxt.Store(nil)              // cleanup
}

var _ prim.ThreadLock = (*MCSLock)(nil)
