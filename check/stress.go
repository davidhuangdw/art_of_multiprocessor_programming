package check

import (
	"sync"
	"testing"
)

// Stress runs fn in goroutines goroutines, iters times per goroutine.
//
// Failures reported from inside a goroutine must use Errorf, never Fatalf:
// Fatalf calls runtime.Goexit and would skip the WaitGroup bookkeeping.
// A panic in any goroutine is recovered and reported as a test error.
func Stress(tb testing.TB, goroutines, iters int, fn func(g int)) {
	tb.Helper()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					tb.Errorf("goroutine %d panicked: %v", g, r)
				}
			}()
			for k := 0; k < iters; k++ {
				fn(g)
			}
		}(g)
	}
	wg.Wait()
}
