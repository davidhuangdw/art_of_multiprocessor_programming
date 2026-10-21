package prim

import (
	"math/rand/v2"
	"time"
)

type Backoff struct {
	min, max, limit time.Duration
}

func NewBackoff(min, max time.Duration) *Backoff {
	if min <= 0 || max < min {
		panic("prim: invalid backoff range")
	}
	return &Backoff{
		min:   min,
		max:   max,
		limit: min,
	}
}

func (b *Backoff) Backoff() {
	time.Sleep(rand.N(b.limit))
	b.limit = min(b.limit*2, b.max)
}
