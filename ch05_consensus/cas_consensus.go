// Package consensus implements the consensus protocols from Chapter 5.
package consensus

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// CASConsensus is the wait-free consensus protocol built on CAS.
//
// Each participating goroutine must call Decide exactly once.
type CASConsensus[T any] struct {
	ptr atomic.Pointer[T]
}

// NewCASConsensus returns a CAS-based consensus object.
func NewCASConsensus[T any]() *CASConsensus[T] {
	return &CASConsensus[T]{}
}

// Decide proposes value and returns the agreed value.
func (c *CASConsensus[T]) Decide(value T) T {
	c.ptr.CompareAndSwap(nil, &value)
	return *c.ptr.Load()
}

var _ prim.Consensus[int] = (*CASConsensus[int])(nil)
