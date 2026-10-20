package regs

import (
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type simpleSnapEle = prim.StampedVal[int]

// SimpleSnapshot is a single-writer obstruction-free snapshot.
type SimpleSnapshot struct {
	n    int
	regs []atomic.Pointer[simpleSnapEle]
}

// NewSimpleSnapshot returns a SimpleSnapshot with n zero-valued registers.
func NewSimpleSnapshot(n int) *SimpleSnapshot {
	validateSize(n)
	res := &SimpleSnapshot{n: n, regs: make([]atomic.Pointer[simpleSnapEle], n)}
	for i := range res.regs {
		res.regs[i].Store(&simpleSnapEle{})
	}
	return res
}

// Update writes v to register i.
func (s *SimpleSnapshot) Update(i, v int) {
	checkIndex(i, s.n)
	old := s.regs[i].Load()
	s.regs[i].Store(&simpleSnapEle{Stamp: old.Stamp + 1, Value: v})
}

// Scan returns an atomic snapshot of all n registers.
func (s *SimpleSnapshot) Scan() []int {
	oldCopy, newCopy := s.collect(), s.collect()
	for !prim.EqualStampedArr(oldCopy, newCopy) {
		oldCopy, newCopy = newCopy, s.collect()
	}
	// twice copies are equal => valid snapshot
	res := make([]int, s.n)
	for i := range newCopy {
		res[i] = newCopy[i].Value
	}
	return res
}

func (s *SimpleSnapshot) collect() []*simpleSnapEle {
	res := make([]*simpleSnapEle, s.n)
	for i := range s.n {
		res[i] = s.regs[i].Load()
	}
	return res
}
