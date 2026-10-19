package mutex

import (
	"testing"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// TestMutualExclusion checks that no two goroutines are ever inside the
// critical section at the same time, and that every attempt completes.
func TestMutualExclusion(t *testing.T) {
	cases := []struct {
		name string
		n    int
		new  func(n int) prim.ThreadLock
	}{
		{"Peterson", 2, func(int) prim.ThreadLock { return NewPeterson() }},
		{"Bakery/2", 2, func(n int) prim.ThreadLock { return NewBakery(n) }},
		{"Bakery/4", 4, func(n int) prim.ThreadLock { return NewBakery(n) }},
		{"Bakery/8", 8, func(n int) prim.ThreadLock { return NewBakery(n) }},
	}

	const iters = 500
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(tc.n)
			crit := check.NewCritical(t)

			check.Stress(t, tc.n, iters, func(g int) {
				lock.Lock(g)
				crit.Enter()
				crit.Leave()
				lock.Unlock(g)
			})

			if got, want := crit.Entered(), int64(tc.n*iters); got != want {
				t.Fatalf("critical sections entered = %d, want %d", got, want)
			}
		})
	}
}

// TestBakeryCounter is the data-race half of the proof: counter is an
// ordinary int protected only by the lock. Run with `go test -race`; any
// missing synchronization or lost update shows up as a race report or a
// short count.
func TestBakeryCounter(t *testing.T) {
	const (
		n     = 8
		iters = 2000
	)

	lock := NewBakery(n)
	counter := 0

	check.Stress(t, n, iters, func(g int) {
		lock.Lock(g)
		counter++
		lock.Unlock(g)
	})

	if want := n * iters; counter != want {
		t.Fatalf("counter = %d, want %d", counter, want)
	}
}
