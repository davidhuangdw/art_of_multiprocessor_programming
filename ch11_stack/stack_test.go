package stack

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

const (
	stressThreads   = 8
	stressPerThread = 100
	stressTimeout   = 10 * time.Second
	shortTimeout    = 3 * time.Millisecond
	exchangeTimeout = 2 * time.Second
)

type stackFactory struct {
	name string
	new  func() prim.Stack
}

func stackFactories() []stackFactory {
	return []stackFactory{
		{name: "LockedStack", new: func() prim.Stack { return NewLockedStack() }},
		{name: "LockFreeStack", new: func() prim.Stack { return NewLockFreeStack() }},
		{name: "EliminationStack", new: func() prim.Stack { return NewEliminationStack() }},
	}
}

// reporter collects failures from worker goroutines, which must never touch
// testing.T: a goroutine outliving its test would panic with "Fail in goroutine
// after test has completed".
type reporter struct {
	mu    sync.Mutex
	notes []string
}

func (r *reporter) addf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notes = append(r.notes, fmt.Sprintf(format, args...))
}

// worker runs fn, turning a panic into a note instead of killing the binary.
func (r *reporter) worker(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			r.addf("panic in goroutine: %v\n%s", p, debug.Stack())
		}
	}()
	fn()
}

func (r *reporter) report(t *testing.T) {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.notes {
		t.Error(n)
	}
}

// withinDeadline runs body in the background and reports whether it finished in
// time, so a deadlock fails fast instead of hitting the `go test` timeout.
func withinDeadline(r *reporter, body func()) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.worker(body)
	}()

	select {
	case <-done:
		return true
	case <-time.After(stressTimeout):
		return false
	}
}

// safeCall is a goroutine whose panic is captured rather than propagated: a stub
// panics, and that must fail one test instead of killing the binary.
type safeCall struct {
	done chan struct{}
	err  error
}

func goSafe(fn func()) *safeCall {
	c := &safeCall{done: make(chan struct{})}
	go func() {
		defer close(c.done)
		defer func() {
			if p := recover(); p != nil {
				c.err = fmt.Errorf("panic in goroutine: %v\n%s", p, debug.Stack())
			}
		}()
		fn()
	}()
	return c
}

// returned reports whether the call has already finished.
func (c *safeCall) returned() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// wait blocks until the call finishes, reporting a panic or a timeout.
func (c *safeCall) wait(t *testing.T, d time.Duration, timeoutMsg string) {
	t.Helper()

	select {
	case <-c.done:
	case <-time.After(d):
		t.Fatal(timeoutMsg)
	}
	if c.err != nil {
		t.Error(c.err)
	}
}

// TestStackSequential checks LIFO order and the empty case on one goroutine.
func TestStackSequential(t *testing.T) {
	for _, tc := range stackFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			s := tc.new()

			if v, ok := s.Pop(); ok {
				t.Fatalf("Pop() = (%d, true) on an empty stack", v)
			}

			values := []int{7, -1, 0, 42}
			for _, v := range values {
				s.Push(v)
			}
			for i := len(values) - 1; i >= 0; i-- {
				got, ok := s.Pop()
				if !ok {
					t.Fatalf("Pop() reported empty, want %d", values[i])
				}
				if got != values[i] {
					t.Fatalf("Pop() = %d, want %d: not LIFO", got, values[i])
				}
			}
			if v, ok := s.Pop(); ok {
				t.Fatalf("Pop() = (%d, true) after draining the stack", v)
			}

			s.Push(1) // still usable after being drained
			if got, ok := s.Pop(); !ok || got != 1 {
				t.Fatalf("Pop() = (%d, %v), want (1, true)", got, ok)
			}
		})
	}
}

// TestStackLIFOWithDepth checks the LIFO discipline at depth: a stack that is
// filled and then drained must yield exactly the reverse order.
func TestStackLIFOWithDepth(t *testing.T) {
	const n = 500

	for _, tc := range stackFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			s := tc.new()
			for i := 0; i < n; i++ {
				s.Push(i)
			}
			for i := 0; i < n; i++ {
				want := n - 1 - i
				got, ok := s.Pop()
				if !ok {
					t.Fatalf("Pop() reported empty, want %d", want)
				}
				if got != want {
					t.Fatalf("pop %d = %d, want %d: not LIFO", i, got, want)
				}
			}
			if v, ok := s.Pop(); ok {
				t.Fatalf("Pop() = (%d, true) after draining the stack", v)
			}
		})
	}
}

// TestStackSingleProducerConsumerNoLoss checks a concurrent pusher/popper pair:
// every value must come out exactly once. Order is deliberately not asserted
// here — a pop can return a value pushed after the previous pop, so the sequence
// is free to ascend.
func TestStackSingleProducerConsumerNoLoss(t *testing.T) {
	const n = 500

	for _, tc := range stackFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			s := tc.new()
			var r reporter
			seen := make(map[int]int, n)
			popped := 0

			if !withinDeadline(&r, func() {
				pushed := goSafe(func() {
					for i := 0; i < n; i++ {
						s.Push(i)
					}
				})
				for popped < n {
					v, ok := s.Pop()
					if !ok {
						if pushed.returned() {
							pushed.wait(t, shortTimeout, "pusher never returned")
							r.addf("pusher finished after only %d of %d values came out", popped, n)
							return
						}
						runtime.Gosched()
						continue
					}
					popped++
					seen[v]++
				}
				pushed.wait(t, stressTimeout, "pusher never finished")
			}) {
				r.report(t)
				t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
			}
			r.report(t)

			if popped != n {
				t.Fatalf("popped %d of %d values", popped, n)
			}
			for v := 0; v < n; v++ {
				if c := seen[v]; c != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, c)
				}
			}
		})
	}
}

// TestStackConcurrentNoLoss runs several pushers and poppers at once: every
// value must come out exactly once, and none may be invented.
func TestStackConcurrentNoLoss(t *testing.T) {
	const pushers, perPusher = stressThreads, stressPerThread
	const poppers = stressThreads
	total := pushers * perPusher

	for _, tc := range stackFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			s := tc.new()
			var r reporter

			seen := make(map[int]int, total)
			var mu sync.Mutex
			var popped atomic.Int64
			pushed := make(chan struct{})

			finished := withinDeadline(&r, func() {
				var popWG, pushWG sync.WaitGroup

				quota := make([]int, poppers)
				for i := range quota {
					quota[i] = total / poppers
				}
				for i := 0; i < total%poppers; i++ {
					quota[i]++
				}

				for p := 0; p < poppers; p++ {
					popWG.Add(1)
					go func(p int) {
						defer popWG.Done()
						r.worker(func() {
							deadline := time.Now().Add(stressTimeout)
							for got := 0; got < quota[p]; {
								v, ok := s.Pop()
								if !ok {
									select {
									case <-pushed:
										// Nothing can arrive any more: the rest
										// has been lost.
										time.Sleep(shortTimeout)
										if got < quota[p] {
											r.addf("popped %d of %d values: lost", got, quota[p])
										}
										return
									default:
									}
									if time.Now().After(deadline) {
										r.addf("stalled after %d of %d pops", got, quota[p])
										return
									}
									runtime.Gosched()
									continue
								}
								got++
								popped.Add(1)
								mu.Lock()
								seen[v]++
								mu.Unlock()
							}
						})
					}(p)
				}

				for p := 0; p < pushers; p++ {
					pushWG.Add(1)
					go func(p int) {
						defer pushWG.Done()
						r.worker(func() {
							for i := 0; i < perPusher; i++ {
								s.Push(p*perPusher + i)
							}
						})
					}(p)
				}
				go func() {
					pushWG.Wait()
					close(pushed)
				}()

				popWG.Wait()
			})
			if !finished {
				r.report(t)
				t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
			}
			r.report(t)

			if got := popped.Load(); got != int64(total) {
				t.Fatalf("popped %d of %d values", got, total)
			}
			for v := 0; v < total; v++ {
				if n := seen[v]; n != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, n)
				}
			}
		})
	}
}

// TestExchangerPair checks that two callers swap: the popper gets the pusher's
// value, the pusher gets the popper's nil payload.
func TestExchangerPair(t *testing.T) {
	defer check.Recover(t)

	x := NewExchanger()
	value := 7

	type result struct {
		got *int
		ok  bool
	}
	pushedBack := make(chan result, 1)
	popped := make(chan result, 1)

	push := goSafe(func() {
		got, ok := x.Exchange(&value, exchangeTimeout)
		pushedBack <- result{got, ok}
	})
	pop := goSafe(func() {
		got, ok := x.Exchange(nil, exchangeTimeout)
		popped <- result{got, ok}
	})
	push.wait(t, stressTimeout, "pusher never returned")
	pop.wait(t, stressTimeout, "popper never returned")
	if push.err != nil || pop.err != nil {
		return // wait already failed the test
	}

	if res := <-popped; !res.ok || res.got == nil || *res.got != value {
		t.Fatalf("popper got (%v, %v), want %d", res.got, res.ok, value)
	}
	if res := <-pushedBack; !res.ok {
		t.Fatal("pusher Exchange() timed out, want a partner")
	} else if res.got != nil {
		t.Fatalf("pusher got %d, want nil (partner was a popper)", *res.got)
	}
}

// TestExchangerTimeout checks that a lone caller gives up and leaves nothing
// behind: the book's withdrawn WAITING slot must return to EMPTY.
func TestExchangerTimeout(t *testing.T) {
	defer check.Recover(t)

	x := NewExchanger()
	value := 1

	if got, ok := x.Exchange(&value, shortTimeout); ok {
		t.Fatalf("Exchange() = (%d, true) with no partner", *got)
	}
	if got, ok := x.Exchange(&value, shortTimeout); ok {
		t.Fatalf("second Exchange() = (%d, true): a timed-out slot was left WAITING", *got)
	}
}

// TestExchangerTinyTimeout checks a timeout shorter than one scheduling round:
// the wait loop never runs, so withdrawing must not depend on anything the loop
// observed.
func TestExchangerTinyTimeout(t *testing.T) {
	defer check.Recover(t)

	x := NewExchanger()
	v := 1
	for i := 0; i < 2000; i++ {
		if got, ok := x.Exchange(&v, time.Nanosecond); ok {
			t.Fatalf("Exchange() = (%d, true) with no partner", *got)
		}
	}
}

// TestExchangerReusableAfterTimeout checks that a timed-out exchange does not
// wedge the exchanger: a later pair must still swap.
func TestExchangerReusableAfterTimeout(t *testing.T) {
	defer check.Recover(t)

	x := NewExchanger()
	value := 9

	if _, ok := x.Exchange(&value, shortTimeout); ok {
		t.Fatal("Exchange() succeeded with no partner")
	}

	out := make(chan int, 1)
	partner := goSafe(func() {
		if got, ok := x.Exchange(nil, exchangeTimeout); ok && got != nil {
			out <- *got
		}
		close(out)
	})
	defer partner.wait(t, shortTimeout, "popper goroutine never returned")
	if got, ok := x.Exchange(&value, exchangeTimeout); !ok {
		t.Fatal("partner Exchange() timed out on a recovered exchanger")
	} else if got != nil {
		t.Fatalf("pusher got %d, want nil", *got)
	}
	select {
	case got, open := <-out:
		if !open {
			t.Fatal("popper never received the value")
		}
		if got != value {
			t.Fatalf("popper got %d, want %d", got, value)
		}
	case <-time.After(stressTimeout):
		t.Fatal("popper never returned")
	}
}

// TestExchangerConservesPayloads checks the exchanger's real contract: it swaps
// any two callers, so every payload offered must come out exactly once. It does
// not care about kinds — deciding that two pushers must fall back is the stack's
// job, not the exchanger's.
func TestExchangerConservesPayloads(t *testing.T) {
	defer check.Recover(t)

	const n = 8

	x := NewExchanger()
	var r reporter
	received := make(map[int]int, n)
	var nils atomic.Int64
	var mu sync.Mutex

	finished := withinDeadline(&r, func() {
		var wg sync.WaitGroup
		record := func(got *int) {
			if got == nil {
				nils.Add(1)
				return
			}
			mu.Lock()
			received[*got]++
			mu.Unlock()
		}

		for i := 0; i < n; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.worker(func() {
					v := i
					got, ok := x.Exchange(&v, exchangeTimeout)
					if !ok {
						r.addf("push %d: Exchange() timed out", i)
						return
					}
					record(got)
				})
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				r.worker(func() {
					got, ok := x.Exchange(nil, exchangeTimeout)
					if !ok {
						r.addf("pop: Exchange() timed out")
						return
					}
					record(got)
				})
			}()
		}
		wg.Wait()
	})
	if !finished {
		r.report(t)
		t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
	}
	r.report(t)

	for v := 0; v < n; v++ {
		if c := received[v]; c != 1 {
			t.Fatalf("value %d came out %d times, want 1", v, c)
		}
	}
	if c := nils.Load(); c != n {
		t.Fatalf("%d nil payloads came out, want %d: a payload was dropped or duplicated", c, n)
	}
}

// TestEliminationArrayPair checks the array path with a single slot, where a
// partner is guaranteed to land on the same exchanger.
func TestEliminationArrayPair(t *testing.T) {
	defer check.Recover(t)

	a := NewEliminationArray(1)
	value := 5

	got := make(chan int, 1)
	partner := goSafe(func() {
		if p, ok := a.Visit(nil, exchangeTimeout); ok && p != nil {
			got <- *p
		}
		close(got)
	})
	defer partner.wait(t, shortTimeout, "popper goroutine never returned")
	if p, ok := a.Visit(&value, exchangeTimeout); !ok {
		t.Fatal("Visit() timed out with a partner inside the array")
	} else if p != nil {
		t.Fatalf("pusher got %d, want nil", *p)
	}
	select {
	case v, open := <-got:
		if !open || v != value {
			t.Fatalf("popper got (%d, %v), want %d", v, open, value)
		}
	case <-time.After(stressTimeout):
		t.Fatal("popper never returned")
	}
}

// TestEliminationArrayLoneVisit checks that visiting a wide array alone times
// out instead of hanging, and that the array stays usable.
func TestEliminationArrayLoneVisit(t *testing.T) {
	defer check.Recover(t)

	a := NewEliminationArray(4)
	value := 3
	for i := 0; i < 20; i++ {
		if p, ok := a.Visit(&value, shortTimeout); ok {
			t.Fatalf("Visit() = (%d, true) with no partner", *p)
		}
	}
}

// TestEliminationStackHandoff checks the property that makes this stack
// different from a plain Treiber stack: under contention, push/pop pairs are
// served by the arena instead of the shared top.
//
// Two things make this awkward to pin. The stack is tried first (as in the book),
// so elimination only fires once a CAS on top actually fails — with one or two Ps
// goroutines barely interleave inside a CAS and the count stays at 0, hence the
// skip. And which of several contended runs happens to produce a pairing is up to
// the scheduler (measured 6..222 pairings per run), so the workload is repeated
// until one does rather than relying on a single lucky schedule.
func TestEliminationStackHandoff(t *testing.T) {
	defer check.Recover(t)

	if p := runtime.GOMAXPROCS(0); p < 4 {
		t.Skipf("elimination needs real parallelism: with %d Ps a stack CAS rarely fails, so the arena is never visited", p)
	}

	const (
		threads  = 8
		perThr   = 500
		attempts = 20
	)
	total := threads * perThr

	for attempt := 0; attempt < attempts; attempt++ {
		s := newEliminationStack(1) // one slot: a rendezvous is likely once both sides visit
		var r reporter
		seen := make(map[int]int, total)
		var mu sync.Mutex
		var popped atomic.Int64

		finished := withinDeadline(&r, func() {
			var wg sync.WaitGroup
			for i := 0; i < threads; i++ {
				i := i
				wg.Add(1)
				go func() {
					defer wg.Done()
					r.worker(func() {
						for k := 0; k < perThr; k++ {
							s.Push(i*perThr + k)
							v, ok := s.Pop()
							if !ok {
								r.addf("Pop() reported empty right after Push")
								continue
							}
							popped.Add(1)
							mu.Lock()
							seen[v]++
							mu.Unlock()
						}
					})
				}()
			}
			wg.Wait()
		})
		if !finished {
			r.report(t)
			t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
		}
		r.report(t)

		if got := popped.Load(); got != int64(total) {
			t.Fatalf("popped %d of %d values", got, total)
		}
		for v, c := range seen {
			if c != 1 {
				t.Fatalf("value %d came out %d times, want 1", v, c)
			}
		}

		if s.eliminated.Load() > 0 {
			return // the arena served at least one pair: the path is wired up
		}
	}
	t.Fatalf("after %d contended runs no push/pop pair was served by the elimination array", attempts)
}

// TestPairingRule pins the elimination rule deterministically: an arena handoff
// completes an operation only against the opposite kind. Getting this wrong
// either drops a value (a push that "succeeds" against another pusher) or invents
// one (a pop that "succeeds" against another popper).
func TestPairingRule(t *testing.T) {
	defer check.Recover(t)

	value := 1
	if !pushPaired(nil) {
		t.Fatal("a push must finish when a popper took its value")
	}
	if pushPaired(&value) {
		t.Fatal("a push must not finish when it meets another pusher: the value would be dropped")
	}
	if !popPaired(&value) {
		t.Fatal("a pop must finish when a pusher handed it a value")
	}
	if popPaired(nil) {
		t.Fatal("a pop must not finish when it meets another popper: a value would be invented")
	}
}

// TestEliminationStackConcurrentPushesNoLoss checks the push side of the pairing
// rule: when two pushers meet in the arena, *both* must fall back to the stack.
// Accepting any pairing as success would drop one of the two values.
//
// Only concurrent pushes can catch that: a lone push succeeds on the stack CAS
// and never visits the arena, so the same-kind path needs contention to happen.
func TestEliminationStackConcurrentPushesNoLoss(t *testing.T) {
	defer check.Recover(t)

	// More goroutines than Ps on purpose: the same-kind pairing only happens once
	// stack CASes start failing, and measured loss under this shape is in the
	// hundreds, versus a handful at 8 goroutines.
	const (
		pushers = 16
		perThr  = 300
	)
	total := pushers * perThr

	s := newEliminationStack(1)
	var r reporter

	finished := withinDeadline(&r, func() {
		var wg sync.WaitGroup
		for i := 0; i < pushers; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.worker(func() {
					for k := 0; k < perThr; k++ {
						s.Push(i*perThr + k)
					}
				})
			}()
		}
		wg.Wait()
	})
	if !finished {
		r.report(t)
		t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
	}
	r.report(t)

	seen := make(map[int]int, total)
	for {
		v, ok := s.Pop()
		if !ok {
			break
		}
		if v < 0 || v >= total {
			t.Fatalf("Pop() = %d: invented a value", v)
		}
		seen[v]++
	}
	if len(seen) != total {
		t.Fatalf("stack held %d of %d pushed values: a pairing dropped one", len(seen), total)
	}
	for v, c := range seen {
		if c != 1 {
			t.Fatalf("value %d came out %d times, want 1", v, c)
		}
	}
}

// TestEliminationStackConcurrentPopsNoInvention checks the pop side of the same
// rule: two poppers that meet must both fall back, never hand each other a nil.
func TestEliminationStackConcurrentPopsNoInvention(t *testing.T) {
	defer check.Recover(t)

	const total = 400

	s := newEliminationStack(1)
	for i := 0; i < total; i++ {
		s.Push(i)
	}

	var r reporter
	seen := make(map[int]int, total)
	var mu sync.Mutex
	var popped atomic.Int64

	finished := withinDeadline(&r, func() {
		var wg sync.WaitGroup
		for i := 0; i < stressThreads; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.worker(func() {
					for {
						v, ok := s.Pop()
						if !ok {
							return
						}
						popped.Add(1)
						mu.Lock()
						seen[v]++
						mu.Unlock()
					}
				})
			}()
		}
		wg.Wait()
	})
	if !finished {
		r.report(t)
		t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
	}
	r.report(t)

	if got := popped.Load(); got != total {
		t.Fatalf("popped %d of %d values", got, total)
	}
	for v := 0; v < total; v++ {
		if c := seen[v]; c != 1 {
			t.Fatalf("value %d came out %d times, want 1", v, c)
		}
	}
}
