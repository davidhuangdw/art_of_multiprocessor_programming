package check

import (
	"sync/atomic"
	"testing"
)

// Critical tracks how many goroutines are inside a critical section and
// reports a test error as soon as more than one is observed at the same time.
//
// It is intentionally cheap and only detects overlap; it cannot prove
// absence of mutual-exclusion violations. Pair it with `go test -race` and
// with an invariant test (for example a plain counter that must equal the
// expected total).
type Critical struct {
	tb      testing.TB
	inside  atomic.Int64
	entered atomic.Int64
}

// NewCritical returns a checker bound to tb.
func NewCritical(tb testing.TB) *Critical {
	tb.Helper()
	return &Critical{tb: tb}
}

// Enter must be called after acquiring the lock.
func (c *Critical) Enter() {
	if v := c.inside.Add(1); v != 1 {
		c.tb.Errorf("mutual exclusion violated: %d goroutines inside the critical section", v)
	}
}

// Leave must be called before releasing the lock.
func (c *Critical) Leave() {
	c.inside.Add(-1)
	c.entered.Add(1)
}

// Entered reports how many times the critical section was completed.
func (c *Critical) Entered() int64 { return c.entered.Load() }
