package spin

import (
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type lockFactory struct {
	name string
	new  func(n int) prim.ThreadLock
}

// indexedLock adapts a plain Lock by ignoring the thread index.
type indexedLock struct {
	lock prim.Lock
}

func (l indexedLock) Lock(int)   { l.lock.Lock() }
func (l indexedLock) Unlock(int) { l.lock.Unlock() }

func lockFactories() []lockFactory {
	return []lockFactory{
		{name: "TASLock", new: func(int) prim.ThreadLock {
			return indexedLock{lock: NewTASLock()}
		}},
		{name: "TTASLock", new: func(int) prim.ThreadLock {
			return indexedLock{lock: NewTTASLock()}
		}},
		{name: "BackoffLock", new: func(int) prim.ThreadLock {
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
	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(2)
			lock.Lock(0)
			lock.Unlock(0)
		})
	}
}

// TestSpinLockMutualExclusion checks that no two goroutines are ever inside
// the critical section at the same time, and that every attempt completes.
func TestSpinLockMutualExclusion(t *testing.T) {
	const (
		threads = 4
		iters   = 500
	)

	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(threads)
			crit := check.NewCritical(t)

			check.Stress(t, threads, iters, func(g int) {
				lock.Lock(g)
				crit.Enter()
				crit.Leave()
				lock.Unlock(g)
			})

			if got, want := crit.Entered(), int64(threads*iters); got != want {
				t.Fatalf("critical sections entered = %d, want %d", got, want)
			}
		})
	}
}

// TestSpinLockCounter checks the data-race half of the proof: counter is an
// ordinary int protected only by the lock.
func TestSpinLockCounter(t *testing.T) {
	const (
		threads = 4
		iters   = 1000
	)

	for _, tc := range lockFactories() {
		t.Run(tc.name, func(t *testing.T) {
			lock := tc.new(threads)
			counter := 0

			check.Stress(t, threads, iters, func(g int) {
				lock.Lock(g)
				counter++
				lock.Unlock(g)
			})

			if got, want := counter, threads*iters; got != want {
				t.Fatalf("counter = %d, want %d", got, want)
			}
		})
	}
}
