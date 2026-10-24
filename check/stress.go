package check

import (
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// Stress runs fn in goroutines goroutines, iters times per goroutine.
//
// Failures reported from inside a goroutine must use Errorf, never Fatalf:
// Fatalf calls runtime.Goexit and would skip the WaitGroup bookkeeping.
// A panic in any goroutine is recovered and reported as a test error together
// with its stack trace, so the panicking file:line stays visible.
func Stress(tb testing.TB, goroutines, iters int, fn func(g int)) {
	tb.Helper()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()

			var it int
			defer func() {
				if r := recover(); r != nil {
					tb.Errorf("goroutine %d, iteration %d: panic: %v\n%s",
						g, it, r, debug.Stack())
				}
			}()

			for it = 0; it < iters; it++ {
				fn(g)
			}
		}(g)
	}
	wg.Wait()
}

// StressTimeout runs Stress in the background and reports a failure if it has
// not finished within timeout.
//
// A deadlocked lock otherwise shows up only as the `go test` timeout, minutes
// later and without pointing at the lock. The goroutines started by Stress are
// abandoned on timeout; they are parked inside the lock and do not touch tb
// again, so the test can finish normally.
func StressTimeout(tb testing.TB, timeout time.Duration, goroutines, iters int, fn func(g int)) {
	tb.Helper()

	done := make(chan struct{})
	go func() {
		Stress(tb, goroutines, iters, fn)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		tb.Errorf("stress (%d goroutines x %d iterations) did not finish within %s: deadlock?",
			goroutines, iters, timeout)
	}
}
