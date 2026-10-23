package spin

import (
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

const (
	cSIZE      = 64
	cMIN_DELAY = time.Microsecond
	cMAX_DELAY = time.Millisecond
)

const (
	cFREE     = iota // free to reuse
	cWAITING         // allocated wait/locked
	cRELEASED        // unlocked
)

type StampedTail struct{ node *Node }

// CompositeLock is the composite fast-path/queue lock from Chapter 7.
type CompositeLock struct {
	nodes [cSIZE]*Node // static nodes for recycling among all threads
	alloc []*Node
	tail  atomic.Pointer[StampedTail] // wrapped-ptr as versioned-value to avoid ABA
}

// NewCompositeLock returns a CompositeLock for n threads.
func NewCompositeLock(n int) *CompositeLock {
	validateThreads(n)
	l := &CompositeLock{
		alloc: make([]*Node, n),
	}
	for i := range l.nodes {
		l.nodes[i] = &Node{id: i}
	}
	l.tail.Store(&StampedTail{})
	return l
}

// Lock acquires the lock on behalf of thread i in [0, n).
// base on CLH lock(spin on prev), free prev & release self
func (l *CompositeLock) Lock(i int) {
	// acquire
	me := l.nodes[rand.N(cSIZE)] // hold a rand node
	backoff := prim.NewBackoff(cMIN_DELAY, cMAX_DELAY)
	// win this node among other holders
	for { // 2 cases to use
		if me.state.CompareAndSwap(cFREE, cWAITING) { // case 1: free node to CAS()
			break
		}

		tailSnap := l.tail.Load()                                 // versioned tail to avoid ABA problem
		if me.state.Load() == cRELEASED && tailSnap.node == me && // case2: 'released && alone' node to CAS()
			l.tail.CompareAndSwap(tailSnap, &StampedTail{}) {
			me.state.Store(cWAITING)
			break
		}
		backoff.Backoff() // backoff for CAS()
	}
	// won with 'WAITING' state
	l.alloc[i] = me

	// enque
	pre := l.tail.Swap(&StampedTail{me}).node // careful: tail become me again, need prevent ABA problem

	// spin
	if pre != nil {
		for pre.state.Load() != cRELEASED {
		}
		// don't forget: free prev to recycle; have 'RELEASED' nodes <= 1
		pre.state.Store(cFREE)
	}
}

// Unlock releases the lock held by thread i.
func (l *CompositeLock) Unlock(i int) {
	me := l.alloc[i]
	me.state.Store(cRELEASED)
}

var _ prim.ThreadLock = (*CompositeLock)(nil)
