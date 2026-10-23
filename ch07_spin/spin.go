// Package spin implements the spin locks from Chapter 7.
package spin

import "sync/atomic"

func validateThreads(n int) {
	if n <= 0 {
		panic("spin: thread count must be positive")
	}
}

type Node struct {
	free  atomic.Bool          // for CLH, MCS: bool state
	nxt   atomic.Pointer[Node] // for MCS
	id    int                  // for debug
	state atomic.Int32         // for composite lock: more states
}
