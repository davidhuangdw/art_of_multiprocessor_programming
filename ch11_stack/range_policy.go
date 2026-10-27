package stack

import (
	"sync/atomic"
	"time"
)

// Bounds of the adaptive elimination range.
const (
	minRange = time.Microsecond
	maxRange = time.Millisecond
)

// rangePolicy is the book's RangePolicy: how long an operation waits in the
// arena. Start small — a short wait sends the caller back to the stack, and
// back to a *new random slot*, within microseconds, so callers collide often.
// A long wait parks each in one slot where the other may never arrive: with a
// fixed 1ms wait, a 16-slot arena paired nobody at 8 threads.
//
// One policy per stack, not the book's one per thread: a simplification.
type rangePolicy struct {
	ns atomic.Int64
}

func newRangePolicy() *rangePolicy {
	p := &rangePolicy{}
	p.ns.Store(int64(minRange))
	return p
}

func (p *rangePolicy) wait() time.Duration { return time.Duration(p.ns.Load()) }

func (p *rangePolicy) success() { p.scale(0.5) } // paired: wait less
func (p *rangePolicy) timeout() { p.scale(2) }   // nobody came: wait longer

func (p *rangePolicy) scale(f float64) {
	for {
		old := p.ns.Load()
		next := min(int64(maxRange), max(int64(minRange), int64(float64(old)*f)))
		if p.ns.CompareAndSwap(old, next) {
			return
		}
	}
}
