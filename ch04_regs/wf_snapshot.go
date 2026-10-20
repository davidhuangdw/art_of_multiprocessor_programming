package regs

import (
	"slices"
	"sync/atomic"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type valWithPrevScan struct {
	val      int
	prevScan []int
}

type wfSnapEle = prim.StampedVal[valWithPrevScan]

// WFSnapshot is a single-writer wait-free snapshot.
type WFSnapshot struct {
	n    int
	regs []atomic.Pointer[wfSnapEle]
}

// NewWFSnapshot returns a WFSnapshot with n zero-valued registers.
func NewWFSnapshot(n int) *WFSnapshot {
	validateSize(n)
	res := &WFSnapshot{n: n, regs: make([]atomic.Pointer[wfSnapEle], n)}
	for i := range res.regs {
		res.regs[i].Store(&wfSnapEle{})
	}
	return res
}

// Update writes v to register i.
func (s *WFSnapshot) Update(i, v int) {
	checkIndex(i, s.n)
	snap := s.Scan()
	old := s.regs[i].Load()
	s.regs[i].Store(&wfSnapEle{
		Stamp: old.Stamp + 1,
		Value: valWithPrevScan{val: v, prevScan: snap},
	})
}

// Scan returns an atomic snapshot of all n registers.
func (s *WFSnapshot) Scan() []int {
	oldCopy, newCopy := s.collect(), s.collect()
	changed := make([]bool, s.n)
ReCompare:
	for {
		for i := range s.n {
			if oldCopy[i].Stamp != newCopy[i].Stamp {
				if changed[i] { // changed again
					// prevScan(of update) happened fully in this scan()'s time interval => valid to reuse
					return slices.Clone(newCopy[i].Value.prevScan)
				} else {
					changed[i] = true
					oldCopy, newCopy = newCopy, s.collect()
					continue ReCompare
				}
			}
		}
		// twice copies are equal => valid snapshot to return
		res := make([]int, s.n)
		for i, d := range newCopy {
			res[i] = d.Value.val
		}
		return res
	}
}

func (s *WFSnapshot) collect() []*wfSnapEle {
	res := make([]*wfSnapEle, s.n)
	for i := range s.n {
		res[i] = s.regs[i].Load()
	}
	return res
}
