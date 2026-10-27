package stack

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Exchange states: the cell is the whole handshake, so a transition is one CAS.
const (
	slotEmpty = iota
	slotWaiting
	slotBusy
)

// slot is the immutable cell an exchange publishes. A fresh cell per transition
// makes pointer identity the version, so a stale CAS cannot win.
type slot struct {
	state int
	item  *int // nil: the holder is a popper asking for a value
}

// Exchanger is the lock-free exchanger of Chapter 11 §11.4: a caller offers a
// payload and waits for a partner to swap with it. Push offers a pointer to its
// value, Pop offers nil — the book's "null" sentinel, kept unambiguous by
// exchanging a pointer. ok == false means nobody came: fall back to the stack.
type Exchanger struct {
	slot atomic.Pointer[slot]
}

// NewExchanger returns an empty exchanger.
func NewExchanger() *Exchanger {
	x := &Exchanger{}
	x.slot.Store(&slot{state: slotEmpty})
	return x
}

// Exchange offers p and returns the partner's payload.
func (x *Exchanger) Exchange(p *int, timeout time.Duration) (*int, bool) {
	till := time.Now().Add(timeout)

	for {
		if time.Now().After(till) {
			return nil, false
		}

		cur := x.slot.Load()
		switch cur.state {
		case slotEmpty: // park my payload and wait for a partner to take it
			mine := &slot{state: slotWaiting, item: p}
			if !x.slot.CompareAndSwap(cur, mine) { // empty->waiting
				continue
			}
			for {
				got := x.slot.Load()
				if got.state == slotBusy { // a partner took mine and left its own
					x.slot.Store(&slot{state: slotEmpty}) // busy->empty
					return got.item, true
				}
				if time.Now().After(till) {
					return x.withdraw(mine)
				}
				runtime.Gosched()
			}

		case slotWaiting: // take the waiting payload, leave mine for its owner
			mine := &slot{state: slotBusy, item: p}
			if x.slot.CompareAndSwap(cur, mine) { // waiting->busy
				return cur.item, true
			}
		}
	}
}

// withdraw retires my WAITING cell: either nobody came (slot back to empty),
// or a partner took my payload and I collect its own. The CAS must target *my
// own* cell, not the last observed one — if the wait loop never ran, that
// observation is still the old EMPTY cell and the comparison would misfire.
func (x *Exchanger) withdraw(mine *slot) (*int, bool) {
	if x.slot.CompareAndSwap(mine, &slot{state: slotEmpty}) { // waiting->empty
		return nil, false
	}
	// Failing the CAS means a partner turned my cell into BUSY (nobody else writes
	// the slot, and only its owner resets BUSY). Anything else is a broken state
	// machine, and returning would let a push deliver its value twice: assert.
	got := x.slot.Load()
	if got.state != slotBusy {
		panic("stack: exchanger slot changed without a partner")
	}
	x.slot.Store(&slot{state: slotEmpty}) // busy->empty
	return got.item, true
}
