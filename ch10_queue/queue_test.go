package queue

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
	settleTime      = 50 * time.Millisecond
)

type queueFactory struct {
	name string
	new  func() prim.Queue
}

// queueFactories lists the queues whose Dequeue is non-blocking.
func queueFactories() []queueFactory {
	return []queueFactory{
		{name: "LockFreeQueue", new: func() prim.Queue { return NewLockFreeQueue() }},
		{name: "StampedQueue", new: func() prim.Queue { return NewStampedQueue() }},
	}
}

type blockingFactory struct {
	name string
	new  func() prim.BlockingQueue
}

// blockingFactories lists the queues whose Dequeue parks while empty.
func blockingFactories() []blockingFactory {
	return []blockingFactory{
		{name: "BoundedQueue", new: func() prim.BlockingQueue { return NewBoundedQueue(16) }},
		{name: "DualQueue", new: func() prim.BlockingQueue { return NewDualQueue() }},
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

// safeCall is a goroutine whose panic is captured rather than propagated.
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

// withinDeadline runs body in the background and reports whether it finished in
// time, so a deadlock fails fast instead of hitting the `go test` timeout. A
// panic in body is charged to r, never to t.
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

// startProducers runs producers goroutines, each enqueuing perProducer values
// encoded as g*perProducer+i so the producer of a value is recoverable.
func startProducers(r *reporter, wg *sync.WaitGroup, producers, perProducer int, enqueue func(int)) {
	for g := 0; g < producers; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			r.worker(func() {
				for i := 0; i < perProducer; i++ {
					enqueue(g*perProducer + i)
				}
			})
		}(g)
	}
}

// orderCheck verifies that each producer's values were dequeued in ascending
// order. It is only meaningful with a single consumer: with several consumers
// the moment of dequeue is not observable, and the check itself would race.
type orderCheck struct {
	perProducer int
	last        []int
}

func newOrderCheck(producers, perProducer int) *orderCheck {
	o := &orderCheck{perProducer: perProducer, last: make([]int, producers)}
	for i := range o.last {
		o.last[i] = -1
	}
	return o
}

// see reports a FIFO violation, if any.
func (o *orderCheck) see(v int) string {
	g, idx := v/o.perProducer, v%o.perProducer
	if idx <= o.last[g] {
		return fmt.Sprintf("producer %d: value %d came out after %d: not FIFO", g, idx, o.last[g])
	}
	o.last[g] = idx
	return ""
}

// TestQueueSequential checks FIFO order and the empty case on one goroutine.
func TestQueueSequential(t *testing.T) {
	for _, tc := range queueFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t) // a stub panics: fail this case, keep the rest

			q := tc.new()

			if v, ok := q.Dequeue(); ok {
				t.Fatalf("Dequeue() = (%d, true) on an empty queue", v)
			}

			values := []int{7, -1, 0, 42}
			for _, v := range values {
				q.Enqueue(v)
			}
			for _, want := range values {
				got, ok := q.Dequeue()
				if !ok {
					t.Fatalf("Dequeue() reported empty, want %d", want)
				}
				if got != want {
					t.Fatalf("Dequeue() = %d, want %d: not FIFO", got, want)
				}
			}
			if v, ok := q.Dequeue(); ok {
				t.Fatalf("Dequeue() = (%d, true) after draining the queue", v)
			}

			q.Enqueue(1) // still usable after being drained
			if got, ok := q.Dequeue(); !ok || got != 1 {
				t.Fatalf("Dequeue() = (%d, %v), want (1, true)", got, ok)
			}
		})
	}
}

// TestQueueSingleProducerConsumer checks the exact FIFO sequence of one producer
// feeding one consumer.
func TestQueueSingleProducerConsumer(t *testing.T) {
	const n = 500

	for _, tc := range queueFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			q := tc.new()
			produced := goSafe(func() {
				for i := 0; i < n; i++ {
					q.Enqueue(i)
				}
			})

			got := make([]int, 0, n)
			deadline := time.Now().Add(stressTimeout)
			for len(got) < n {
				v, ok := q.Dequeue()
				if !ok {
					if produced.returned() {
						produced.wait(t, settleTime, "producer never returned")
						t.Fatalf("producer finished but only %d of %d items came out", len(got), n)
					}
					if time.Now().After(deadline) {
						t.Fatalf("only %d of %d items came out", len(got), n)
					}
					runtime.Gosched()
					continue
				}
				got = append(got, v)
			}
			produced.wait(t, stressTimeout, "producer never finished")

			for i, v := range got {
				if v != i {
					t.Fatalf("item %d = %d, want %d: not FIFO", i, v, i)
				}
			}
			if v, ok := q.Dequeue(); ok {
				t.Fatalf("Dequeue() = (%d, true) after the producer finished", v)
			}
		})
	}
}

// TestQueueConcurrentNoLoss runs several producers and consumers at once: every
// value must come out exactly once. Order is checked by the single-consumer test.
func TestQueueConcurrentNoLoss(t *testing.T) {
	const producers, perProducer = stressThreads, stressPerThread
	total := producers * perProducer

	for _, tc := range queueFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			q := tc.new()
			var r reporter

			seen := make(map[int]int, total)
			var mu sync.Mutex
			var dequeued atomic.Int64
			produced := make(chan struct{})

			finished := withinDeadline(&r, func() {
				var consumers, producerWG sync.WaitGroup

				for c := 0; c < stressThreads; c++ {
					consumers.Add(1)
					go func() {
						defer consumers.Done()
						r.worker(func() {
							deadline := time.Now().Add(stressTimeout)
							for dequeued.Load() < int64(total) {
								v, ok := q.Dequeue()
								if !ok {
									select {
									case <-produced:
										// Producers are done: anything still
										// missing has been lost.
										time.Sleep(settleTime)
										return
									default:
									}
									if time.Now().After(deadline) {
										r.addf("stalled after %d of %d items", dequeued.Load(), total)
										return
									}
									runtime.Gosched()
									continue
								}
								dequeued.Add(1)
								mu.Lock()
								seen[v]++
								mu.Unlock()
							}
						})
					}()
				}

				startProducers(&r, &producerWG, producers, perProducer, q.Enqueue)
				go func() {
					producerWG.Wait()
					close(produced)
				}()

				consumers.Wait()
			})
			if !finished {
				r.report(t)
				t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
			}
			r.report(t)

			if got := dequeued.Load(); got != int64(total) {
				t.Fatalf("dequeued %d of %d items", got, total)
			}
			for v := 0; v < total; v++ {
				if n := seen[v]; n != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, n)
				}
			}
		})
	}
}

// TestQueueSingleConsumerOrder checks that every producer's own values come out
// in the order they were enqueued, with the many-producers/one-consumer shape
// where the dequeue order is observable.
func TestQueueSingleConsumerOrder(t *testing.T) {
	const producers, perProducer = stressThreads, stressPerThread
	total := producers * perProducer

	for _, tc := range queueFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			q := tc.new()
			var r reporter
			order := newOrderCheck(producers, perProducer)
			seen := make(map[int]int, total)

			finished := withinDeadline(&r, func() {
				var producerWG sync.WaitGroup
				startProducers(&r, &producerWG, producers, perProducer, q.Enqueue)

				produced := make(chan struct{})
				go func() {
					producerWG.Wait()
					close(produced)
				}()

				deadline := time.Now().Add(stressTimeout)
				collected := 0
				for collected < total {
					v, ok := q.Dequeue()
					if !ok {
						select {
						case <-produced:
							r.addf("producers finished but only %d of %d items came out", collected, total)
							return
						default:
						}
						if time.Now().After(deadline) {
							r.addf("stalled after %d of %d items", collected, total)
							return
						}
						runtime.Gosched()
						continue
					}
					collected++
					seen[v]++
					if msg := order.see(v); msg != "" {
						r.addf("%s", msg)
					}
				}
			})
			if !finished {
				r.report(t)
				t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
			}
			r.report(t)

			for v := 0; v < total; v++ {
				if n := seen[v]; n != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, n)
				}
			}
		})
	}
}

// TestBlockingQueueNoLoss covers the queues whose Dequeue parks: several
// producers and consumers, every value handed over exactly once.
func TestBlockingQueueNoLoss(t *testing.T) {
	const producers, perProducer = stressThreads, stressPerThread
	total := producers * perProducer
	perConsumer := total / stressThreads

	for _, tc := range blockingFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			q := tc.new()
			var r reporter

			seen := make(map[int]int, total)
			var mu sync.Mutex

			finished := withinDeadline(&r, func() {
				var wg sync.WaitGroup
				startProducers(&r, &wg, producers, perProducer, q.Enqueue)

				for c := 0; c < stressThreads; c++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						r.worker(func() {
							for i := 0; i < perConsumer; i++ {
								v := q.Dequeue()
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

			for v := 0; v < total; v++ {
				if n := seen[v]; n != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, n)
				}
			}
		})
	}
}

// TestBlockingQueueSingleConsumerOrder checks per-producer order with one
// consumer, the only shape in which the dequeue order is observable.
func TestBlockingQueueSingleConsumerOrder(t *testing.T) {
	const producers, perProducer = stressThreads, stressPerThread
	total := producers * perProducer

	for _, tc := range blockingFactories() {
		t.Run(tc.name, func(t *testing.T) {
			defer check.Recover(t)

			q := tc.new()
			var r reporter
			order := newOrderCheck(producers, perProducer)
			seen := make(map[int]int, total)

			finished := withinDeadline(&r, func() {
				var wg sync.WaitGroup
				startProducers(&r, &wg, producers, perProducer, q.Enqueue)

				wg.Add(1)
				go func() {
					defer wg.Done()
					r.worker(func() {
						for i := 0; i < total; i++ {
							v := q.Dequeue()
							seen[v]++
							if msg := order.see(v); msg != "" {
								r.addf("%s", msg)
							}
						}
					})
				}()
				wg.Wait()
			})
			if !finished {
				r.report(t)
				t.Fatalf("did not finish within %s: deadlock?", stressTimeout)
			}
			r.report(t)

			for v := 0; v < total; v++ {
				if n := seen[v]; n != 1 {
					t.Fatalf("value %d came out %d times, want 1", v, n)
				}
			}
		})
	}
}

// TestBoundedQueueWraparound drives the ring buffer past its capacity repeatedly.
func TestBoundedQueueWraparound(t *testing.T) {
	defer check.Recover(t)

	const capacity = 3
	q := NewBoundedQueue(capacity)
	next := 0
	for round := 0; round < 100; round++ {
		for i := 0; i < capacity; i++ {
			q.Enqueue(next)
			next++
		}
		for i := 0; i < capacity; i++ {
			want := next - capacity + i
			if got := q.Dequeue(); got != want {
				t.Fatalf("round %d: Dequeue() = %d, want %d", round, got, want)
			}
		}
	}
}

// TestBoundedQueueBlocksWhenFull checks that Enqueue parks on a full queue and
// resumes once a slot is freed.
func TestBoundedQueueBlocksWhenFull(t *testing.T) {
	defer check.Recover(t)

	q := NewBoundedQueue(2)
	q.Enqueue(1)
	q.Enqueue(2)

	enqueued := goSafe(func() { q.Enqueue(3) })
	time.Sleep(settleTime)
	if enqueued.returned() {
		t.Fatal("Enqueue(3) returned on a full queue")
	}

	if got := q.Dequeue(); got != 1 {
		t.Fatalf("Dequeue() = %d, want 1", got)
	}
	enqueued.wait(t, stressTimeout, "Enqueue(3) still parked after a slot was freed")
	if got := q.Dequeue(); got != 2 {
		t.Fatalf("Dequeue() = %d, want 2", got)
	}
	if got := q.Dequeue(); got != 3 {
		t.Fatalf("Dequeue() = %d, want 3", got)
	}
}

// TestBoundedQueueDequeueParks checks that Dequeue parks on an empty queue and
// resumes as soon as an item arrives.
func TestBoundedQueueDequeueParks(t *testing.T) {
	defer check.Recover(t)

	q := NewBoundedQueue(2)

	got := make(chan int, 1)
	dequeued := goSafe(func() { got <- q.Dequeue() })
	time.Sleep(settleTime)
	select {
	case v := <-got:
		t.Fatalf("Dequeue() = %d on an empty queue", v)
	case <-dequeued.done:
		t.Fatalf("Dequeue() panicked: %v", dequeued.err)
	default:
	}

	q.Enqueue(5)
	select {
	case v := <-got:
		if v != 5 {
			t.Fatalf("Dequeue() = %d, want 5", v)
		}
	case <-time.After(stressTimeout):
		t.Fatal("Dequeue() stayed parked after Enqueue(5)")
	}
}

// TestStampedQueueStampAdvances pins what the stamp is for: every published tail
// cell carries a newer version than the one it replaces.
func TestStampedQueueStampAdvances(t *testing.T) {
	defer check.Recover(t)

	q := NewStampedQueue()
	if q.tail.Load() == nil {
		t.Fatal("NewStampedQueue left tail unset: it needs a stamped dummy node")
	}
	prev := q.tail.Load().stamp
	for i := 0; i < 5; i++ {
		q.Enqueue(i)
		cur := q.tail.Load().stamp
		if cur <= prev {
			t.Fatalf("tail stamp %d after Enqueue(%d), want > %d", cur, i, prev)
		}
		prev = cur
	}
}

// dequeueWithin runs Dequeue in the background: ok reports whether it returned a
// value in time, err whether it panicked (a stub).
//
// The done case re-reads out: the goroutine sends before it closes done, so both
// channels are ready at once and a plain select would pick one at random.
func dequeueWithin(q prim.BlockingQueue, d time.Duration) (v int, ok bool, err error) {
	out := make(chan int, 1)
	c := goSafe(func() { out <- q.Dequeue() })
	timeout := time.After(d)

	for {
		select {
		case v = <-out:
			return v, true, nil
		case <-c.done: // finished: either it panicked, or the value is in out
			select {
			case v = <-out:
				return v, true, nil
			default:
				return 0, false, c.err
			}
		case <-timeout:
			return 0, false, nil
		}
	}
}

// TestDualQueueHandoff checks that each operation waits for its counterpart.
func TestDualQueueHandoff(t *testing.T) {
	defer check.Recover(t)

	q := NewDualQueue()

	// A Dequeue with nothing offered must park, not return and not panic.
	departed := make(chan int, 1)
	dequeued := goSafe(func() { departed <- q.Dequeue() })
	time.Sleep(settleTime)
	select {
	case v := <-departed:
		t.Fatalf("Dequeue() = %d with nothing enqueued", v)
	case <-dequeued.done:
		t.Fatalf("Dequeue() panicked: %v", dequeued.err)
	default:
	}

	// Enqueue must not park while a Dequeue is waiting.
	enqueued := goSafe(func() { q.Enqueue(42) })
	select {
	case v := <-departed:
		if v != 42 {
			t.Fatalf("Dequeue() = %d, want 42", v)
		}
	case <-time.After(stressTimeout):
		t.Fatal("Enqueue(42) never handed its value to the waiting Dequeue")
	}
	enqueued.wait(t, stressTimeout, "Enqueue(42) never returned")

	// The symmetric case: Enqueue must park until a Dequeue arrives.
	enqueued = goSafe(func() { q.Enqueue(7) })
	time.Sleep(settleTime)
	if enqueued.returned() {
		t.Fatalf("Enqueue(7) returned with no Dequeue waiting: %v", enqueued.err)
	}
	v, ok, err := dequeueWithin(q, stressTimeout)
	if err != nil {
		t.Fatalf("Dequeue() %v", err)
	}
	if !ok || v != 7 {
		t.Fatalf("Dequeue() = (%d, %v), want (7, true)", v, ok)
	}
	enqueued.wait(t, stressTimeout, "Enqueue(7) never returned after the handoff")
}

// TestDualQueueOffersAreFIFO checks that offers are served in arrival order.
func TestDualQueueOffersAreFIFO(t *testing.T) {
	defer check.Recover(t)

	q := NewDualQueue()
	var offers []*safeCall
	for i := 0; i < 3; i++ {
		i := i
		offers = append(offers, goSafe(func() { q.Enqueue(i) }))
		time.Sleep(settleTime) // let offer i register before offer i+1
	}
	for want := 0; want < 3; want++ {
		got, ok, err := dequeueWithin(q, stressTimeout)
		if err != nil {
			t.Fatalf("Dequeue() %v", err)
		}
		if !ok {
			t.Fatalf("Dequeue() for want %d never returned", want)
		}
		if got != want {
			t.Fatalf("Dequeue() = %d, want %d: offers not served FIFO", got, want)
		}
	}
	for _, offer := range offers {
		offer.wait(t, stressTimeout, "an Enqueue never returned after its handoff")
	}
}
