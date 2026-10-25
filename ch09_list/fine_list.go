package list

import (
	"math"
	"sync"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type fNode struct {
	key  int
	next *fNode
	sync.Mutex
}

// FineList is the fine-grained sorted list from Chapter 9.
//
// Every node has its own lock and the traversal is hand-over-hand: the
// successor is locked before the predecessor is released, so neighbours are
// consistent while a node is inspected or spliced.
//
// idea: remove/add require lock two-adj nodes first => invariant: node won't be removed during locked
type FineList struct {
	head *fNode
}

// NewFineList returns an empty list.
func NewFineList() *FineList {
	tail := &fNode{key: math.MaxInt}
	head := &fNode{key: math.MinInt, next: tail}
	return &FineList{head}
}

// Add inserts v and reports whether it was absent.
func (s *FineList) Add(v int) bool {
	pre, cur := s.traverse(v)
	defer pre.Unlock()
	defer cur.Unlock()

	if cur.key == v {
		return false
	}
	pre.next = &fNode{key: v, next: cur}
	return true
}

// Remove deletes v and reports whether it was present.
func (s *FineList) Remove(v int) bool {
	pre, cur := s.traverse(v)
	defer pre.Unlock()
	defer cur.Unlock()

	if cur.key == v {
		pre.next = cur.next
		return true
	}
	return false
}

// Contains reports whether v is present.
func (s *FineList) Contains(v int) bool {
	pre, cur := s.traverse(v)
	defer pre.Unlock()
	defer cur.Unlock()
	return cur.key == v
}

// traverse till pre.key<v && cur.key >=v
func (s *FineList) traverse(v int) (pre, cur *fNode) {
	s.head.Lock()
	pre, cur = s.head, s.head.next
	cur.Lock()
	for cur.key < v {
		pre.Unlock()
		pre, cur = cur, cur.next
		cur.Lock()
	}
	return pre, cur
}

var _ prim.Set = (*FineList)(nil)
