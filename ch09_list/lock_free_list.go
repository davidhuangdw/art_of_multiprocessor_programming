package list

import "github.com/davidhuangdw/art_of_multiprocessor_programming/prim"

// LockFreeList is the lock-free sorted list from Chapter 9.
//
// next pointers carry a mark bit, so logical removal and CAS-based physical
// removal can be combined in one word; a traversal that meets a marked node
// unlinks it (that is also how Contains stays lock-free).
type LockFreeList struct {
	// TODO: atomic markable next pointers + sorted nodes with -inf/+inf sentinels
}

// NewLockFreeList returns an empty list.
func NewLockFreeList() *LockFreeList {
	return &LockFreeList{}
}

// Add inserts v and reports whether it was absent.
func (s *LockFreeList) Add(v int) bool {
	panic("list: LockFreeList.Add not implemented")
}

// Remove deletes v and reports whether it was present.
func (s *LockFreeList) Remove(v int) bool {
	panic("list: LockFreeList.Remove not implemented")
}

// Contains reports whether v is present.
func (s *LockFreeList) Contains(v int) bool {
	panic("list: LockFreeList.Contains not implemented")
}

var _ prim.Set = (*LockFreeList)(nil)
