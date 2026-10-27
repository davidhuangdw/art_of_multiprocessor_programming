// Package stack implements the concurrent stacks from Chapter 11.
package stack

import (
	"sync"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// LockedStack is the coarse-grained stack: one mutex around a slice.
//
// It is the sequential baseline the lock-free stacks are compared against, not
// an algorithm from the book.
type LockedStack struct {
	mu    sync.Mutex
	items []int
}

// NewLockedStack returns an empty stack.
func NewLockedStack() *LockedStack {
	return &LockedStack{}
}

// Push adds v on top.
func (s *LockedStack) Push(v int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, v)
}

// Pop removes and returns the top value.
func (s *LockedStack) Pop() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := len(s.items)
	if n == 0 {
		return 0, false
	}
	v := s.items[n-1]
	s.items = s.items[:n-1]
	return v, true
}

var _ prim.Stack = (*LockedStack)(nil)
