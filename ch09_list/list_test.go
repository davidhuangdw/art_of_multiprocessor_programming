package list

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

const (
	stressThreads = 8
	stressTimeout = 10 * time.Second
)

type setFactory struct {
	name string
	new  func() prim.Set
}

func setFactories() []setFactory {
	return []setFactory{
		{name: "CoarseList", new: func() prim.Set { return NewCoarseList() }},
		{name: "FineList", new: func() prim.Set { return NewFineList() }},
		{name: "OptimisticList", new: func() prim.Set { return NewOptimisticList() }},
		{name: "LazyList", new: func() prim.Set { return NewLazyList() }},
		{name: "LockFreeList", new: func() prim.Set { return NewLockFreeList() }},
	}
}

// TestSetSequential checks the set contract on one goroutine, including
// negative and duplicate values (the sentinels are -inf/+inf).
func TestSetSequential(t *testing.T) {
	values := []int{-7, 0, 3, 42}

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()

			for _, v := range values {
				if !s.Add(v) {
					t.Fatalf("Add(%d) = false on an empty set", v)
				}
				if s.Add(v) {
					t.Fatalf("Add(%d) = true for a duplicate", v)
				}
				if !s.Contains(v) {
					t.Fatalf("Contains(%d) = false after Add", v)
				}
			}
			for _, v := range []int{-8, 1, 41, 43} {
				if s.Contains(v) {
					t.Fatalf("Contains(%d) = true for a value never added", v)
				}
				if s.Remove(v) {
					t.Fatalf("Remove(%d) = true for a value never added", v)
				}
			}
			for _, v := range values {
				if !s.Remove(v) {
					t.Fatalf("Remove(%d) = false for a present value", v)
				}
				if s.Remove(v) {
					t.Fatalf("Remove(%d) = true for an already removed value", v)
				}
				if s.Contains(v) {
					t.Fatalf("Contains(%d) = true after Remove", v)
				}
			}
		})
	}
}

// TestSetConcurrentDisjointAdds checks that no insert is lost when threads add
// disjoint ranges at the same time.
func TestSetConcurrentDisjointAdds(t *testing.T) {
	const perThread = 200

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()

			check.StressTimeout(t, stressTimeout, stressThreads, 1, func(g int) {
				for i := 0; i < perThread; i++ {
					v := g*perThread + i
					if !s.Add(v) {
						t.Errorf("Add(%d) = false, want true", v)
					}
				}
			})

			for g := 0; g < stressThreads; g++ {
				for i := 0; i < perThread; i++ {
					v := g*perThread + i
					if !s.Contains(v) {
						t.Fatalf("Contains(%d) = false after a successful Add", v)
					}
				}
			}
		})
	}
}

// TestSetConcurrentDuplicateAdd checks linearizability of Add: exactly one of
// the racing inserts of the same value may report true.
func TestSetConcurrentDuplicateAdd(t *testing.T) {
	const (
		perThread = 100
		value     = 7
	)

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()
			var wins atomic.Int64

			check.StressTimeout(t, stressTimeout, stressThreads, perThread, func(int) {
				if s.Add(value) {
					wins.Add(1)
				}
			})

			if got := wins.Load(); got != 1 {
				t.Fatalf("Add(%d) returned true %d times, want exactly 1", value, got)
			}
		})
	}
}

// TestSetConcurrentDisjointRemoves checks that every concurrent Remove of a
// present value reports true exactly once.
func TestSetConcurrentDisjointRemoves(t *testing.T) {
	const perThread = 100

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()
			for g := 0; g < stressThreads; g++ {
				for i := 0; i < perThread; i++ {
					if !s.Add(g*perThread + i) {
						t.Fatalf("Add(%d) = false while filling", g*perThread+i)
					}
				}
			}

			check.StressTimeout(t, stressTimeout, stressThreads, 1, func(g int) {
				for i := 0; i < perThread; i++ {
					v := g*perThread + i
					if !s.Remove(v) {
						t.Errorf("Remove(%d) = false for a present value", v)
					}
					if s.Remove(v) {
						t.Errorf("Remove(%d) = true a second time", v)
					}
				}
			})

			for g := 0; g < stressThreads; g++ {
				for i := 0; i < perThread; i++ {
					v := g*perThread + i
					if s.Contains(v) {
						t.Fatalf("Contains(%d) = true after Remove", v)
					}
				}
			}
		})
	}
}

// TestSetChurn checks that a value nobody touches survives concurrent churn,
// and that each thread's own range is gone afterwards.
func TestSetChurn(t *testing.T) {
	const (
		perThread = 100
		rounds    = 5
		watched   = 1 << 20 // far outside every worker range
	)

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()
			if !s.Add(watched) {
				t.Fatal("Add(watched) = false")
			}

			check.StressTimeout(t, stressTimeout, stressThreads, rounds, func(g int) {
				base := g * perThread
				for i := 0; i < perThread; i++ {
					v := base + i
					if !s.Add(v) {
						t.Errorf("Add(%d) = false", v)
					}
					if !s.Contains(v) {
						t.Errorf("Contains(%d) = false right after Add", v)
					}
					if !s.Remove(v) {
						t.Errorf("Remove(%d) = false", v)
					}
					if s.Contains(v) {
						t.Errorf("Contains(%d) = true right after Remove", v)
					}
				}
			})

			if !s.Contains(watched) {
				t.Fatal("Contains(watched) = false: concurrent churn corrupted the list")
			}
			for g := 0; g < stressThreads; g++ {
				if s.Contains(g * perThread) {
					t.Fatalf("Contains(%d) = true after add/remove churn", g*perThread)
				}
			}
		})
	}
}

// TestSetConcurrentReaders checks that Contains is safe (and correct) while
// other goroutines mutate the list.
func TestSetConcurrentReaders(t *testing.T) {
	const (
		writers = 2
		readers = 4
		rounds  = 200
		watched = 1 << 20
	)

	for _, tc := range setFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()
			s.Add(watched)

			var wg sync.WaitGroup
			wg.Add(writers + readers)

			for w := 0; w < writers; w++ {
				go func(w int) {
					defer wg.Done()
					for i := 0; i < rounds; i++ {
						v := w*rounds + i
						s.Add(v)
						s.Remove(v)
					}
				}(w)
			}
			for r := 0; r < readers; r++ {
				go func() {
					defer wg.Done()
					for i := 0; i < rounds; i++ {
						if !s.Contains(watched) {
							t.Errorf("Contains(watched) = false while others mutate")
							return
						}
					}
				}()
			}

			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(stressTimeout):
				t.Fatal("concurrent readers/writers did not finish: deadlock?")
			}
		})
	}
}

// TestLockFreeMarkedNodeDoesNotBlock pins the progress guarantee: a node that is
// already marked but not yet unlinked must not stall other goroutines.
// Traversal has to splice marked nodes out (the book's "snip") instead of
// retrying forever on the same node.
//
// It pokes at the internals because "mark without unlinking" is only reachable
// from inside the package.
func TestLockFreeMarkedNodeDoesNotBlock(t *testing.T) {
	defer check.Recover(t)

	s := NewLockFreeList()
	if !s.Add(5) || !s.Add(9) {
		t.Fatal("setup Add failed")
	}

	// Mark 5 and leave it linked: as if a Remove were preempted between
	// marking the node and unlinking it.
	var marked *eNode
	for n := s.head.v.Load().next; n != nil; n = n.v.Load().next {
		if n.key == 5 {
			marked = n
			break
		}
	}
	if marked == nil {
		t.Fatal("Add(5) did not link a node with key 5")
	}
	cur := marked.v.Load()
	if !marked.v.CompareAndSwap(cur, &eVersioned{next: cur.next, removed: true}) {
		t.Fatal("could not mark node 5")
	}
	if s.Contains(5) {
		t.Fatal("Contains(5) = true for a marked node")
	}

	done := make(chan bool, 1)
	go func() { done <- s.Add(3) }() // 3 < 5, so the traversal meets the marked node
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("Add(3) = false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Add(3) blocked on a marked node: traversal never spliced it (not lock-free)")
	}

	if s.Contains(5) {
		t.Fatal("Contains(5) = true after the marked node was spliced")
	}
	if !s.Contains(3) || !s.Contains(9) {
		t.Fatal("neighbour lost while splicing the marked node")
	}
}
