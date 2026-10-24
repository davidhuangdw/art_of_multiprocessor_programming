package monitor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/check"
)

const (
	stressThreads = 8
	waitTimeout   = 2 * time.Second
	blockedProbe  = 100 * time.Millisecond
	stressTimeout = 10 * time.Second
	queueDelay    = 20 * time.Millisecond
)

// waitFor blocks until ch is closed, failing the test after waitTimeout.
func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(waitTimeout):
		t.Fatalf("%s: still blocked after %s", what, waitTimeout)
	}
}

// expectBlocked checks that nothing happens on ch within blockedProbe.
func expectBlocked(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s: did not block", what)
	case <-time.After(blockedProbe):
	}
}

// waitForValue polls v until it reaches want.
func waitForValue(t *testing.T, v *atomic.Int64, want int64, what string) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if v.Load() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s: got %d, want %d", what, v.Load(), want)
}

// TestMonitorMutualExclusion checks that the monitor really excludes and that
// the protected counter keeps every update.
func TestMonitorMutualExclusion(t *testing.T) {
	const iters = 500

	m := NewSimpleMonitor()
	crit := check.NewCritical(t)
	counter := 0

	check.Stress(t, stressThreads, iters, func(int) {
		m.Lock()
		crit.Enter()
		counter++
		crit.Leave()
		m.Unlock()
	})

	if got, want := counter, stressThreads*iters; got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}
	if got, want := crit.Entered(), int64(stressThreads*iters); got != want {
		t.Fatalf("critical sections entered = %d, want %d", got, want)
	}
}

// TestMonitorSignalWakesOne checks Signal/All semantics: Signal releases
// exactly one waiter, SignalAll releases the rest.
func TestMonitorSignalWakesOne(t *testing.T) {
	const waiters = 3

	m := NewSimpleMonitor()
	cond := m.NewCondition()

	var queued, entered atomic.Int64
	permit := 0 // guarded by m

	var wg sync.WaitGroup
	wg.Add(waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			m.Lock()
			queued.Add(1)
			for permit == 0 {
				cond.Await()
			}
			permit--
			entered.Add(1)
			m.Unlock()
		}()
	}

	waitForValue(t, &queued, waiters, "waiters queued on the condition")

	m.Lock()
	permit++
	cond.Signal()
	m.Unlock()

	waitForValue(t, &entered, 1, "waiters released by Signal")
	time.Sleep(blockedProbe) // give a broken SignalAll-like wake-up time to show
	if got := entered.Load(); got != 1 {
		t.Fatalf("Signal released %d waiters, want 1", got)
	}

	m.Lock()
	permit += waiters - 1
	cond.SignalAll()
	m.Unlock()

	waitForValue(t, &entered, waiters, "waiters released by SignalAll")
	wg.Wait()
}

// TestMonitorAwaitReleasesLock checks that a waiter does not hold the monitor
// while parked: another goroutine must be able to enter meanwhile.
func TestMonitorAwaitReleasesLock(t *testing.T) {
	m := NewSimpleMonitor()
	cond := m.NewCondition()

	parked := make(chan struct{})
	go func() {
		m.Lock()
		close(parked)
		cond.Await()
		m.Unlock()
	}()
	<-parked

	// While the first goroutine is parked, the monitor must be free.
	done := make(chan struct{})
	go func() {
		m.Lock()
		m.Unlock()
		close(done)
	}()
	waitFor(t, done, "second goroutine acquiring the monitor")

	m.Lock()
	cond.Signal()
	m.Unlock()
}

// TestBoundedQueue checks the producer-consumer buffer: nothing is lost or
// duplicated, and producers block instead of overrunning the capacity.
func TestBoundedQueue(t *testing.T) {
	const (
		capacity  = 4
		producers = 2
		consumers = 2
		perWorker = 500
	)

	q := NewBoundedQueue(capacity)

	var produced, consumed, sum atomic.Int64
	perConsumer := producers * perWorker / consumers

	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				q.Put(1)
				produced.Add(1)
			}
		}()
	}
	for c := 0; c < consumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perConsumer; i++ {
				sum.Add(int64(q.Get()))
				consumed.Add(1)
			}
		}()
	}
	wg.Wait()

	want := int64(producers * perWorker)
	if got := produced.Load(); got != want {
		t.Fatalf("produced = %d, want %d", got, want)
	}
	if got := consumed.Load(); got != want {
		t.Fatalf("consumed = %d, want %d", got, want)
	}
	if got := sum.Load(); got != want {
		t.Fatalf("sum of consumed items = %d, want %d", got, want)
	}
}

// TestReadersWriterOverlap checks that readers share the lock and that a
// writer is excluded from it.
func TestReadersWriterOverlap(t *testing.T) {
	const readers = 2

	rw := NewReadersWriter()
	release := make(chan struct{})
	inside := make(chan struct{}, readers)

	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.ReadLock()
			inside <- struct{}{}
			<-release
			rw.ReadUnlock()
		}()
	}

	for i := 0; i < readers; i++ {
		select {
		case <-inside:
		case <-time.After(waitTimeout):
			t.Fatal("readers did not hold the lock at the same time")
		}
	}

	writerIn := make(chan struct{})
	go func() {
		rw.WriteLock()
		close(writerIn)
		rw.WriteUnlock()
	}()
	expectBlocked(t, writerIn, "writer while readers hold the lock")

	close(release)
	wg.Wait()
	waitFor(t, writerIn, "writer after the readers released")
}

// TestReadersWriterStress checks the read-lock invariant: a value cannot change
// between two reads taken while the same reader holds the lock.
func TestReadersWriterStress(t *testing.T) {
	const (
		readers = 4
		writers = 2
		iters   = 300
	)

	rw := NewReadersWriter()
	value := 0 // guarded by rw

	check.StressTimeout(t, stressTimeout, readers+writers, iters, func(g int) {
		if g < readers {
			rw.ReadLock()
			a, b := value, value
			rw.ReadUnlock()
			if a != b {
				t.Errorf("reader %d saw value change %d -> %d under the read lock", g, a, b)
			}
			return
		}
		rw.WriteLock()
		value++
		rw.WriteUnlock()
	})

	if got, want := value, writers*iters; got != want {
		t.Fatalf("value = %d, want %d", got, want)
	}
}

// assertPanics reports a failure if fn does not panic.
func assertPanics(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", what)
		}
	}()
	fn()
}

// TestReentrantLockReentrant checks that the same owner can re-enter and that
// other owners only get in after the last matching Unlock.
func TestReentrantLockReentrant(t *testing.T) {
	l := NewSimpleReentrantLock()

	l.Lock(0)
	l.Lock(0) // must not block
	l.Lock(0)

	other := make(chan struct{})
	go func() {
		l.Lock(1)
		close(other)
		l.Unlock(1)
	}()
	expectBlocked(t, other, "second owner while the first holds the lock")

	l.Unlock(0)
	l.Unlock(0)
	expectBlocked(t, other, "second owner before the last Unlock")

	l.Unlock(0)
	waitFor(t, other, "second owner after the last Unlock")
}

// TestReentrantLockMutualExclusion checks the lock under contention: every
// goroutine re-enters once, and the counter sees every update.
func TestReentrantLockMutualExclusion(t *testing.T) {
	const iters = 500

	l := NewSimpleReentrantLock()
	crit := check.NewCritical(t)
	counter := 0

	check.Stress(t, stressThreads, iters, func(g int) {
		l.Lock(g)
		crit.Enter()
		counter++
		l.Lock(g) // re-entrant
		counter++
		l.Unlock(g)
		crit.Leave()
		l.Unlock(g)
	})

	if got, want := counter, 2*stressThreads*iters; got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}
}

// TestReentrantLockWrongOwner checks that a non-owner cannot release the lock.
func TestReentrantLockWrongOwner(t *testing.T) {
	l := NewSimpleReentrantLock()
	l.Lock(0)

	assertPanics(t, "Unlock by a non-owner", func() { l.Unlock(1) })

	l.Unlock(0) // the failed Unlock must not have changed the state
}

// TestSemaphoreActsAsMutex checks that a one-permit semaphore is a mutex.
func TestSemaphoreActsAsMutex(t *testing.T) {
	const iters = 500

	s := NewSemaphore(1)
	crit := check.NewCritical(t)
	counter := 0

	check.Stress(t, stressThreads, iters, func(int) {
		s.Acquire()
		crit.Enter()
		counter++
		crit.Leave()
		s.Release()
	})

	if got, want := counter, stressThreads*iters; got != want {
		t.Fatalf("counter = %d, want %d", got, want)
	}
}

// TestSemaphoreLimit checks that at most `permits` goroutines hold the
// semaphore at the same time.
func TestSemaphoreLimit(t *testing.T) {
	const (
		permits = 3
		iters   = 100
	)

	s := NewSemaphore(permits)

	var inside, maxInside atomic.Int64
	check.Stress(t, stressThreads, iters, func(int) {
		s.Acquire()
		n := inside.Add(1)
		for {
			if m := maxInside.Load(); n <= m || maxInside.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Microsecond) // hold long enough to overlap
		inside.Add(-1)
		s.Release()
	})

	if got := maxInside.Load(); got > permits {
		t.Fatalf("%d goroutines held %d permits", got, permits)
	}
	if got := maxInside.Load(); got < 2 {
		t.Logf("never observed overlap (max %d); the limit invariant still held", got)
	}
}

// TestSemaphoreReleaseWakesWaiter checks that Release hands a permit to a
// blocked Acquirer.
func TestSemaphoreReleaseWakesWaiter(t *testing.T) {
	s := NewSemaphore(1)
	s.Acquire()

	done := make(chan struct{})
	go func() {
		s.Acquire()
		close(done)
		s.Release()
	}()
	expectBlocked(t, done, "Acquire while no permit is available")

	s.Release()
	waitFor(t, done, "Acquire after Release")
}

// TestBoundedQueueFIFO checks the ring buffer: items come out in order and the
// head wraps around correctly after several fill/drain rounds.
func TestBoundedQueueFIFO(t *testing.T) {
	const (
		capacity = 2
		rounds   = 3
	)

	q := NewBoundedQueue(capacity)
	for round := 0; round < rounds; round++ {
		for i := 0; i < capacity; i++ {
			q.Put(round*capacity + i)
		}
		for i := 0; i < capacity; i++ {
			if got, want := q.Get(), round*capacity+i; got != want {
				t.Fatalf("round %d: Get() = %d, want %d", round, got, want)
			}
		}
	}
}

// TestReadersWriterWriterNotJumped checks the writer-preference property: once
// a writer is waiting, a later reader must not slip in ahead of it.
func TestReadersWriterWriterNotJumped(t *testing.T) {
	rw := NewReadersWriter()

	rw.ReadLock() // this goroutine holds a read lock

	writerIn := make(chan struct{})
	writerRelease := make(chan struct{})
	go func() {
		rw.WriteLock()
		close(writerIn)
		<-writerRelease
		rw.WriteUnlock()
	}()
	time.Sleep(queueDelay) // let the writer queue and raise its flag

	readerIn := make(chan struct{})
	readerRelease := make(chan struct{})
	go func() {
		rw.ReadLock()
		close(readerIn)
		<-readerRelease
		rw.ReadUnlock()
	}()
	time.Sleep(queueDelay)

	rw.ReadUnlock() // the queued writer must win from here on

	waitFor(t, writerIn, "writer after the last reader left")
	expectBlocked(t, readerIn, "reader that arrived after the writer")

	close(writerRelease)
	waitFor(t, readerIn, "reader after the writer released")
	close(readerRelease)
}

// TestReentrantLockOverUnlock checks that releasing an unlocked lock panics and
// leaves the lock usable.
func TestReentrantLockOverUnlock(t *testing.T) {
	l := NewSimpleReentrantLock()
	l.Lock(0)
	l.Unlock(0)

	assertPanics(t, "Unlock of an unlocked lock", func() { l.Unlock(0) })

	l.Lock(1)
	other := make(chan struct{})
	go func() {
		l.Lock(0)
		close(other)
		l.Unlock(0)
	}()
	expectBlocked(t, other, "second owner after a failed Unlock")

	l.Unlock(1)
	waitFor(t, other, "second owner after the holder released")
}

// TestSemaphoreZeroPermits checks the acquire/release handshake: with no
// permits an Acquirer blocks until somebody releases one.
func TestSemaphoreZeroPermits(t *testing.T) {
	s := NewSemaphore(0)

	acquired := make(chan struct{})
	go func() {
		s.Acquire()
		close(acquired)
		s.Release()
	}()
	expectBlocked(t, acquired, "Acquire with no permits")

	s.Release()
	waitFor(t, acquired, "Acquire after Release")
}

// rwLock is the shape shared by ReadersWriter and FifoReadersWriter, so the
// same assertions can be run against both.
type rwLock interface {
	ReadLock()
	ReadUnlock()
	WriteLock()
	WriteUnlock()
}

// FifoReadersWriter is only *approximately* FIFO: readers admitted together
// are unordered and overlap, a reader arriving after a queued writer waits for
// it, and writers keep arrival order among themselves. The boundary between a
// reader that is already waiting and the writer queued after it is deliberately
// unspecified, so the tests below never assert that order.
//
// TestFifoReadersWriterNoBarge checks the guarantee it does make: a reader
// queued behind a writer cannot enter before that writer finishes.
func TestFifoReadersWriterNoBarge(t *testing.T) {
	var rw rwLock = NewFifoReadersWriter()

	rw.WriteLock() // W0 holds, so the queues below are built deterministically

	writerIn := make(chan struct{})
	writerRelease := make(chan struct{})
	go func() {
		rw.WriteLock()
		close(writerIn)
		<-writerRelease
		rw.WriteUnlock()
	}()
	time.Sleep(queueDelay)

	readerIn := make(chan struct{})
	readerRelease := make(chan struct{})
	go func() {
		rw.ReadLock()
		close(readerIn)
		<-readerRelease
		rw.ReadUnlock()
	}()
	time.Sleep(queueDelay)

	rw.WriteUnlock() // W0 leaves: the queued writer runs first, the reader waits

	waitFor(t, writerIn, "writer queued before the reader")
	expectBlocked(t, readerIn, "reader queued after the writer")

	close(writerRelease)
	waitFor(t, readerIn, "reader after the writer released")
	close(readerRelease)
}

// TestFifoReadersWriterBatchReadersShare checks the batch half of the contract:
// readers admitted together hold the lock concurrently, in any order.
func TestFifoReadersWriterBatchReadersShare(t *testing.T) {
	const readers = 3

	var rw rwLock = NewFifoReadersWriter()
	rw.WriteLock() // hold the lock so all readers queue in the same batch

	inside := make(chan struct{}, readers)
	release := make(chan struct{})
	for i := 0; i < readers; i++ {
		go func() {
			rw.ReadLock()
			inside <- struct{}{}
			<-release
			rw.ReadUnlock()
		}()
	}
	time.Sleep(queueDelay)
	rw.WriteUnlock()

	for i := 0; i < readers; i++ {
		select {
		case <-inside:
		case <-time.After(waitTimeout):
			t.Fatalf("only %d of %d readers were admitted", i, readers)
		}
	}
	close(release)
}

// TestFifoReadersWriterStress checks the read-lock invariant under contention.
func TestFifoReadersWriterStress(t *testing.T) {
	const (
		readers = 4
		writers = 2
		iters   = 300
	)

	var rw rwLock = NewFifoReadersWriter()
	value := 0 // guarded by rw

	check.StressTimeout(t, stressTimeout, readers+writers, iters, func(g int) {
		if g < readers {
			rw.ReadLock()
			a, b := value, value
			rw.ReadUnlock()
			if a != b {
				t.Errorf("reader %d saw value change %d -> %d under the read lock", g, a, b)
			}
			return
		}
		rw.WriteLock()
		value++
		rw.WriteUnlock()
	})

	if got, want := value, writers*iters; got != want {
		t.Fatalf("value = %d, want %d", got, want)
	}
}
