package stack

import (
	"runtime"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// sNode is one stack node. next is written before the CAS that publishes the
// node, so that CAS carries it.
type sNode struct {
	v    int
	next *sNode
}

// LockFreeStack is the Treiber stack from Chapter 11.
//
// Push and Pop CAS the same word, top, so every thread contends on one cache
// line: correct and lock-free, but it does not scale — that is what elimination
// (§11.3) is for.
//
// Pop's CAS can suffer ABA where nodes are recycled. Go's GC keeps a node alive
// while a reader holds it, so a popped node cannot return as the same pointer
// with a stale successor.
type LockFreeStack struct {
	top atomic.Pointer[sNode]
}

// NewLockFreeStack returns an empty stack.
func NewLockFreeStack() *LockFreeStack {
	return &LockFreeStack{}
}

// Push adds v on top.
func (s *LockFreeStack) Push(v int) {
	add := &sNode{v: v}
	for !s.tryPush(add) {
		backoff()
	}
}

// tryPush links add as the new top. The node stays unpublished unless the CAS
// succeeds, so a caller may retry with it.
func (s *LockFreeStack) tryPush(add *sNode) bool {
	t := s.top.Load()
	add.next = t
	return s.top.CompareAndSwap(t, add)
}

// Pop removes and returns the top value.
func (s *LockFreeStack) Pop() (int, bool) {
	v, succ, retry := s.tryPop()
	for retry {
		backoff()
		v, succ, retry = s.tryPop()
	}
	return v, succ
}

// tryPop takes the top node. retry reports a lost CAS, as opposed to an empty
// stack, which returns retry == false, succ == false.
func (s *LockFreeStack) tryPop() (v int, succ, retry bool) {
	t := s.top.Load()
	if t == nil {
		return 0, false, false
	}
	if s.top.CompareAndSwap(t, t.next) {
		return t.v, true, false
	}
	return 0, false, true

}

// backoff yields the processor: retrying immediately only burns a core.
func backoff() {
	runtime.Gosched()
}

var _ prim.Stack = (*LockFreeStack)(nil)
