// Package spin implements the spin locks from Chapter 7.
package spin

import "sync/atomic"

func validateThreads(n int) {
	if n <= 0 {
		panic("spin: thread count must be positive")
	}
}

type Node struct {
	free atomic.Bool
	nxt  atomic.Pointer[Node]
}
