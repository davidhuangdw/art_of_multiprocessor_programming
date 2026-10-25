package list

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// OptimisticList is the optimistic sorted list from Chapter 9.
//
// Traversal is lock-free (nodes the traversal passes become stale at any
// moment, which is harmless because nothing is written yet). Add/Remove then
// lock pre and cur, re-traverse from head and validate that pre is still the
// predecessor of cur; only then do they splice. Retrying on a failed validation
// is what makes the operations linearizable.
//
// idea: improve fine-lock that don't lock any during the long traverse
// by: in critical-zone(lock both nodes) check again 'prev reachable & point to cur'
type oNode struct {
	key  int                   // not must atomic now, since oNode.next.Store() will help release key's visible
	next atomic.Pointer[oNode] // atomic for without-lock traverse/read, no atomic leads to read initial 'nil'
	sync.Mutex
	removed atomic.Bool // only used by LazyList; immutable in this file
}

func newONode(key int, nxt *oNode) *oNode {
	res := &oNode{key: key} // key is written before publication, so it needs no atomic
	res.next.Store(nxt)
	return res
}

type OptimisticList struct {
	head *oNode
}

// NewOptimisticList returns an empty list.
func NewOptimisticList() *OptimisticList {
	tail := &oNode{key: math.MaxInt}
	head := &oNode{key: math.MinInt}
	head.next.Store(tail)
	return &OptimisticList{head}
}

// Add inserts v and reports whether it was absent.
func (s *OptimisticList) Add(v int) bool {
	pre, cur := s.traverseTillSucc(v)
	defer pre.Unlock()
	defer cur.Unlock()
	if cur.key == v {
		return false
	}
	pre.next.Store(newONode(v, cur))
	return true
}

// Remove deletes v and reports whether it was present.
func (s *OptimisticList) Remove(v int) bool {
	pre, cur := s.traverseTillSucc(v)
	defer pre.Unlock()
	defer cur.Unlock()
	if cur.key != v {
		return false
	}
	pre.next.Store(cur.next.Load())
	return true
}

// Contains reports whether v is present.
func (s *OptimisticList) Contains(v int) bool {
	pre, cur := s.traverseTillSucc(v)
	defer pre.Unlock()
	defer cur.Unlock()
	return cur.key == v
}

func (s *OptimisticList) traverseTillSucc(v int) (pre, cur *oNode) {
	pre, cur = s.traverseWithoutLock(v)
	pre.Lock()
	cur.Lock()
	for {
		_pre, _cur := s.traverseWithoutLock(v) // again during locked pre&cur
		if pre == _pre && cur == _cur {        // validate
			return pre, cur
		}
		cur.Unlock()
		pre.Unlock()
		pre, cur = _pre, _cur
		pre.Lock()
		cur.Lock()
	}
}

func (s *OptimisticList) traverseWithoutLock(v int) (pre, cur *oNode) {
	pre, cur = s.head, s.head.next.Load()
	for cur.key < v {
		pre, cur = cur, cur.next.Load()
	}
	return pre, cur
}

var _ prim.Set = (*OptimisticList)(nil)
