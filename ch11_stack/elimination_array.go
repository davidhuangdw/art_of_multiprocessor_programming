package stack

import (
	"math/rand/v2"
	"time"
)

// defaultEliminationSlots is the width of the arena by default.
//
// Width cuts both ways: more callers can park at once, but two only pair if
// they pick the *same* slot, so a wider arena makes a given pair less likely.
// 16 measured fastest here, and still 3x slower than a plain Treiber stack at
// 8 threads: elimination needs sustained contention to pay off.
const defaultEliminationSlots = 16

// EliminationArray is the elimination array of Chapter 11 §11.3: a fixed set of
// exchangers, one of which a caller picks at random.
//
// Randomness is what makes rendezvous parallel: round-robin would send a lone
// pusher and a lone popper to different slots forever.
type EliminationArray struct {
	arena []*Exchanger
}

// NewEliminationArray returns an array of size exchangers.
func NewEliminationArray(size int) *EliminationArray {
	if size <= 0 {
		panic("stack: elimination array must be non-empty")
	}
	a := &EliminationArray{arena: make([]*Exchanger, size)}
	for i := range a.arena {
		a.arena[i] = NewExchanger()
	}
	return a
}

// Visit offers p to one randomly chosen exchanger and reports whether a partner
// swapped with it in time.
func (a *EliminationArray) Visit(p *int, timeout time.Duration) (*int, bool) {
	return a.arena[rand.N(len(a.arena))].Exchange(p, timeout)
}
