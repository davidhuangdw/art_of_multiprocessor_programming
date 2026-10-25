package list

import (
	"math"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type eVersioned struct {
	removed bool
	next    *eNode
}

type eNode struct {
	key int
	v   atomic.Pointer[eVersioned]
}

func newENode(v int, nxt *eNode) *eNode {
	x := &eNode{key: v}
	x.v.Store(&eVersioned{next: nxt})
	return x
}

// LockFreeList is the lock-free sorted list from Chapter 9.
//
// A node's link is an immutable {removed, next} cell behind atomic.Pointer, so
// "marked?" and "successor?" come from a single load — the Go stand-in for
// AtomicMarkableReference. Every write publishes a fresh cell, so the cell
// pointer doubles as a version and CAS cannot hit ABA. Traversal splices marked
// nodes out as it walks, and that is what keeps the list lock-free.
type LockFreeList struct {
	head *eNode
}

// NewLockFreeList returns an empty list.
func NewLockFreeList() *LockFreeList {
	tail := newENode(math.MaxInt, nil)
	head := newENode(math.MinInt, tail)
	return &LockFreeList{head}
}

// Add inserts v and reports whether it was absent.
func (s *LockFreeList) Add(v int) bool {
	for {
		pre, cur, pv, _ := s.traverseTillSucc(v)
		if cur.key == v {
			return false
		}
		add := newENode(v, cur)
		if pre.v.CompareAndSwap(pv, &eVersioned{next: add}) {
			return true
		}
	}
}

// Remove deletes v and reports whether it was present.
func (s *LockFreeList) Remove(v int) bool {
	for {
		pre, cur, pv, cv := s.traverseTillSucc(v)
		if cur.key != v {
			return false
		}
		// mark cur as removed
		if !cur.v.CompareAndSwap(cv, &eVersioned{next: cv.next, removed: true}) {
			continue
		}
		// unlink: best effort, a later traversal finishes it
		pre.v.CompareAndSwap(pv, &eVersioned{next: cv.next})
		return true
	}
}

// Contains reports whether v is present.
func (s *LockFreeList) Contains(v int) bool {
	cur := s.head.v.Load().next
	for cur.key < v {
		cur = cur.v.Load().next
	}
	return cur.key == v && !cur.v.Load().removed
}

func (s *LockFreeList) traverseTillSucc(v int) (pre, cur *eNode, pv, cv *eVersioned) {
Retry:
	for {
		pre, pv = s.head, s.head.v.Load()
		cur = pv.next
		for {
			cv = cur.v.Load()
			if cv.removed { // snip: unlink the marked node
				if !pre.v.CompareAndSwap(pv, &eVersioned{next: cv.next}) {
					continue Retry // pre changed under us, walk again
				}
				cur = cv.next
				continue
			}
			if v <= cur.key {
				return pre, cur, pv, cv
			}
			pre, pv, cur = cur, cv, cv.next
		}
	}
}

var _ prim.Set = (*LockFreeList)(nil)
