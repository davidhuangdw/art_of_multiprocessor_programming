// Package list implements the concurrent linked lists from Chapter 9.
package list

import (
	"math"
	"sync"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type cNode struct {
	key  int
	next *cNode
}

// CoarseList is the coarse-grained sorted list from Chapter 9.
//
// A single mutex guards the whole list, so every operation is sequential and
// trivially linearizable — but nothing runs concurrently.
type CoarseList struct {
	head *cNode
	lock sync.Mutex
}

// NewCoarseList returns an empty list.
func NewCoarseList() *CoarseList {
	tail := &cNode{key: math.MaxInt}
	head := &cNode{key: math.MinInt, next: tail}
	return &CoarseList{head: head}
}

// Add inserts v and reports whether it was absent.
func (s *CoarseList) Add(v int) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	pre, cur := s.head, s.head.next
	for cur.key < v {
		pre, cur = cur, cur.next
	}
	if cur.key == v {
		return false
	}
	pre.next = &cNode{key: v, next: cur}
	return true
}

// Remove deletes v and reports whether it was present.
func (s *CoarseList) Remove(v int) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	pre, cur := s.head, s.head.next
	for cur.key < v {
		pre, cur = cur, cur.next
	}
	if cur.key != v {
		return false
	}
	pre.next = cur.next
	return true
}

// Contains reports whether v is present.
func (s *CoarseList) Contains(v int) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	cur := s.head.next
	for cur.key < v {
		cur = cur.next
	}
	return cur.key == v
}

var _ prim.Set = (*CoarseList)(nil)
