package list

import "github.com/davidhuangdw/art_of_multiprocessor_programming/prim"

// LazyList is the lazy sorted list from Chapter 9.
//
// A node is physically removed only after it has been logically removed: the
// marked flag is set under the node's lock, and a Remove that finds a marked
// node knows somebody else already deleted it. Contains walks without locking.

// idea: 'marked' flag to know 'been removed before really locked'
type LazyList struct {
	*OptimisticList
}

// NewLazyList returns an empty list.
func NewLazyList() *LazyList {
	return &LazyList{NewOptimisticList()}
}

// Add inserts v and reports whether it was absent.
func (s *LazyList) Add(v int) bool {
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
func (s *LazyList) Remove(v int) bool {
	pre, cur := s.traverseTillSucc(v)
	defer pre.Unlock()
	defer cur.Unlock()

	if cur.key != v {
		return false
	}
	cur.removed.Store(true)
	pre.next.Store(cur.next.Load())
	return true
}

// Contains reports whether v is present.
func (s *LazyList) Contains(v int) bool {
	_, cur := s.traverseWithoutLock(v)
	return !cur.removed.Load() && cur.key == v // check 'removed' in case allow remove() not always physicaly remove
}

func (s *LazyList) traverseTillSucc(v int) (pre, cur *oNode) {
	pre, cur = s.traverseWithoutLock(v)
	pre.Lock()
	cur.Lock()
	for {
		if !pre.removed.Load() && !cur.removed.Load() && pre.next.Load() == cur { // validate reachable
			// careful: need pre.next==cur to avoid insert between
			return pre, cur
		}

		cur.Unlock()
		pre.Unlock()
		pre, cur = s.traverseWithoutLock(v)
		pre.Lock()
		cur.Lock()
	}
}
func (s *LazyList) traverseWithoutLock(v int) (pre, cur *oNode) {
	pre, cur = s.head, s.head.next.Load()
	for cur.key < v {
		pre, cur = cur, cur.next.Load()
	}
	return pre, cur

}

var _ prim.Set = (*LazyList)(nil)
