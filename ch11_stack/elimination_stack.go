package stack

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// The pairing rule is the only thing between elimination and a lost or
// invented value, so it lives in one place. A handoff completes an operation
// only against the *opposite* kind: a push is done when the partner took its
// value (nil payload), a pop when the partner handed it one. Two pushes that
// meet must both fall back to the stack, and so must two poppers.
func pushPaired(other *int) bool { return other == nil }
func popPaired(other *int) bool  { return other != nil }

// EliminationStack is the elimination backoff stack of Chapter 11 §11.5.
//
// Push and Pop try the shared stack first and back off to the arena only when
// their CAS fails, so elimination answers contention instead of taxing every
// operation. Arena waits adapt (see rangePolicy).
type EliminationStack struct {
	stack      *LockFreeStack
	arena      *EliminationArray
	policy     *rangePolicy
	eliminated atomic.Int64 // diagnostics: pairings served by the arena
}

// NewEliminationStack returns an empty stack.
func NewEliminationStack() *EliminationStack {
	return newEliminationStack(defaultEliminationSlots)
}

func newEliminationStack(slots int) *EliminationStack {
	return &EliminationStack{
		stack:  NewLockFreeStack(),
		arena:  NewEliminationArray(slots),
		policy: newRangePolicy(),
	}
}

// Push adds v on top, handing it straight to a waiting Pop when it can.
func (s *EliminationStack) Push(v int) {
	for {
		if s.stack.tryPush(&sNode{v: v}) {
			return
		}
		other, paired := s.arena.Visit(&v, s.policy.wait())
		if paired {
			s.policy.success()
			if pushPaired(other) {
				s.eliminated.Add(1)
				return
			}
			continue // met another pusher: both fall back
		}
		s.policy.timeout()
	}
}

// Pop removes and returns the top value.
func (s *EliminationStack) Pop() (int, bool) {
	for {
		v, ok, retry := s.stack.tryPop()
		if !retry {
			return v, ok
		}
		other, paired := s.arena.Visit(nil, s.policy.wait())
		if paired {
			s.policy.success()
			if popPaired(other) {
				s.eliminated.Add(1)
				return *other, true
			}
			continue // met another popper: both fall back
		}
		s.policy.timeout()
	}
}

var _ prim.Stack = (*EliminationStack)(nil)
