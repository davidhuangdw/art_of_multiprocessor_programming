package spin

import (
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// stressThreads is deliberately larger than 2. Bugs in the queue locks only
// show up once several goroutines fight for the same lock: with 4 threads a
// shared node is rarely picked twice, but at 8 threads collisions are common.
const stressThreads = 8

type lockFactory struct {
	name string
	// indexed locks take a thread index; the others ignore it.
	indexed bool
	new     func(n int) prim.ThreadLock
}

// indexedLock adapts a plain Lock by ignoring the thread index.
type indexedLock struct {
	lock prim.Lock
}

func (l indexedLock) Lock(int)   { l.lock.Lock() }
func (l indexedLock) Unlock(int) { l.lock.Unlock() }

func lockFactories() []lockFactory {
	return []lockFactory{
		{name: "TASLock", indexed: true, new: func(int) prim.ThreadLock {
			return indexedLock{lock: NewTASLock()}
		}},
		{name: "TTASLock", indexed: true, new: func(int) prim.ThreadLock {
			return indexedLock{lock: NewTTASLock()}
		}},
		{name: "BackoffLock", indexed: true, new: func(int) prim.ThreadLock {
			return indexedLock{lock: NewBackoffLock(time.Microsecond, time.Millisecond)}
		}},
		{name: "AndersonQueueLock", new: func(n int) prim.ThreadLock {
			return NewAndersonQueueLock(n)
		}},
		{name: "CLHLock", new: func(n int) prim.ThreadLock {
			return NewCLHLock(n)
		}},
		{name: "MCSLock", new: func(n int) prim.ThreadLock {
			return NewMCSLock(n)
		}},
		{name: "CompositeLock", new: func(n int) prim.ThreadLock {
			return NewCompositeLock(n)
		}},
	}
}

// TestSpinLockSmoke checks the basic Lock/Unlock path before the stress tests.
func TestSpinLockSmoke(t *testing.T) {
	const n = 2

	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(n)
			for i := 0; i < n; i++ {
				lock.Lock(i)
				lock.Unlock(i)
			}
		})
	}
}

// TestSpinLockInvalidSize checks that locks built for n threads reject n <= 0.
func TestSpinLockInvalidSize(t *testing.T) {
	for _, tc := range lockFactories() {
		if tc.indexed {
			continue // these ignore the thread count
		}
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{0, -1} {
				func() {
					defer func() {
						if recover() == nil {
							t.Errorf("New(n=%d) did not panic", n)
						}
					}()
					tc.new(n)
				}()
			}
		})
	}
}

// TestSpinLockMutualExclusion checks that no two goroutines are ever inside
// the critical section at the same time, and that every attempt completes.
func TestSpinLockMutualExclusion(t *testing.T) {
	const iters = 500

	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(stressThreads)
			crit := check.NewCritical(t)

			check.Stress(t, stressThreads, iters, func(g int) {
				lock.Lock(g)
				crit.Enter()
				crit.Leave()
				lock.Unlock(g)
			})

			if got, want := crit.Entered(), int64(stressThreads*iters); got != want {
				t.Fatalf("critical sections entered = %d, want %d", got, want)
			}
		})
	}
}

// TestSpinLockCounter checks the data-race half of the proof: counter is an
// ordinary int protected only by the lock. Run with `go test -race`; a broken
// lock shows up as a race report or a short count.
func TestSpinLockCounter(t *testing.T) {
	const iters = 1000

	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(stressThreads)
			counter := 0

			check.Stress(t, stressThreads, iters, func(g int) {
				lock.Lock(g)
				counter++
				lock.Unlock(g)
			})

			if got, want := counter, stressThreads*iters; got != want {
				t.Fatalf("counter = %d, want %d", got, want)
			}
		})
	}
}

// TestCompositeLockNodePool is the white-box companion to the stress tests.
// CompositeLock hands out nodes from a fixed pool, so a lock that forgets to
// recycle a node keeps spinning once the pool drains. After all acquisitions
// complete, no node may still be in use, and only the tail may stay RELEASED.
func TestCompositeLockNodePool(t *testing.T) {
	const iters = 200

	l := NewCompositeLock(stressThreads)
	counter := 0

	check.Stress(t, stressThreads, iters, func(g int) {
		l.Lock(g)
		counter++
		l.Unlock(g)
	})

	if got, want := counter, stressThreads*iters; got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}

	tail := l.tail.Load()
	if tail == nil {
		t.Fatal("tail is nil after all Lock/Unlock pairs")
	}

	waiting, released := 0, 0
	for _, n := range l.nodes {
		switch n.state.Load() {
		case cWAITING:
			waiting++
			t.Logf("node %d still WAITING", n.id)
		case cRELEASED:
			released++
		}
	}
	if waiting != 0 {
		t.Fatalf("%d nodes still WAITING, want 0", waiting)
	}
	if released > 1 {
		t.Fatalf("%d nodes still RELEASED, want at most the tail", released)
	}
	if released == 1 && tail.node.state.Load() != cRELEASED {
		t.Fatal("released node is not the tail; the node pool leaked")
	}

	// The pool must keep working after it has been cycled this many times.
	l.Lock(0)
	counter++
	l.Unlock(0)
	if got, want := counter, stressThreads*iters+1; got != want {
		t.Fatalf("counter after reuse = %d, want %d", got, want)
	}
}
