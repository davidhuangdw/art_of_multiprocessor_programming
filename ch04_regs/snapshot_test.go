package regs

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

type snapshotFactory struct {
	name string
	new  func(n int) Snapshot
}

func snapshotFactories() []snapshotFactory {
	return []snapshotFactory{
		{name: "SimpleSnapshot", new: func(n int) Snapshot { return NewSimpleSnapshot(n) }},
		{name: "WFSnapshot", new: func(n int) Snapshot { return NewWFSnapshot(n) }},
	}
}

// TestSnapshotInitialScan checks that every register starts at zero.
func TestSnapshotInitialScan(t *testing.T) {
	const n = 4
	want := make([]int, n)

	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(n)
			if got := s.Scan(); !slices.Equal(got, want) {
				t.Fatalf("Scan() = %v, want %v", got, want)
			}
		})
	}
}

// TestSnapshotUpdateAndScan checks that Scan observes the latest value of
// every register when there is no concurrency.
func TestSnapshotUpdateAndScan(t *testing.T) {
	const n = 3

	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(n)

			for i := 0; i < n; i++ {
				s.Update(i, 10+i)
			}
			if got, want := s.Scan(), []int{10, 11, 12}; !slices.Equal(got, want) {
				t.Fatalf("Scan() = %v, want %v", got, want)
			}

			s.Update(1, 99)
			if got, want := s.Scan(), []int{10, 99, 12}; !slices.Equal(got, want) {
				t.Fatalf("Scan() after second update = %v, want %v", got, want)
			}
		})
	}
}

// TestSnapshotScanReturnsCopy checks that mutating a returned scan cannot
// change the snapshot object.
func TestSnapshotScanReturnsCopy(t *testing.T) {
	const n = 2

	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(n)
			s.Update(0, 7)

			snap := s.Scan()
			snap[0] = -1

			if got, want := s.Scan()[0], 7; got != want {
				t.Fatalf("mutating the returned scan changed register 0: got %d, want %d", got, want)
			}
		})
	}
}

// TestSnapshotInvalidSize checks that constructors reject n <= 0.
func TestSnapshotInvalidSize(t *testing.T) {
	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			assertPanics(t, "NewSnapshot(0)", func() { tc.new(0) })
		})
	}
}

// TestSnapshotInvalidIndex checks that Update rejects out-of-range indices.
func TestSnapshotInvalidIndex(t *testing.T) {
	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(2)
			assertPanics(t, "Update(-1)", func() { s.Update(-1, 0) })
			assertPanics(t, "Update(2)", func() { s.Update(2, 0) })
		})
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	fn()
}

// TestWFSnapshotScanHelpBranch forces Scan's wait-free help path. A writer
// publishes stamped values whose prevScan is a sentinel; once Scan observes
// the same register move twice, it must return that stored snapshot.
func TestWFSnapshotScanHelpBranch(t *testing.T) {
	const sentinel = -1
	s := NewWFSnapshot(1)
	storedScan := []int{sentinel}

	var stop atomic.Bool
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		for stamp := 1; !stop.Load(); stamp++ {
			s.regs[0].Store(&wfSnapEle{
				Stamp: stamp,
				Value: valWithPrevScan{val: stamp, prevScan: storedScan},
			})
		}
	}()
	defer func() {
		stop.Store(true)
		writerWG.Wait()
	}()

	for i := 0; i < 1_000_000; i++ {
		got := s.Scan()
		if len(got) != 1 {
			t.Fatalf("Scan() returned %d registers, want 1", len(got))
		}
		if got[0] == sentinel {
			got[0] = 12345
			if storedScan[0] != sentinel {
				t.Fatalf("mutating the returned help scan changed the stored snapshot: got %d, want %d",
					storedScan[0], sentinel)
			}
			return
		}
	}
	t.Fatal("Scan() never took the help branch")
}

// TestSnapshotAtomicScan uses a single writer that updates every register in
// increasing index order with the current round. If a scan sees register i at
// round r, it must also see every register below i at round >= r. A torn scan
// can therefore be detected without knowing the exact linearization point.
func TestSnapshotAtomicScan(t *testing.T) {
	const (
		n       = 4
		rounds  = 2000
		readers = 4
	)

	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(n)
			stop := make(chan struct{})

			var readerWG sync.WaitGroup
			readerWG.Add(readers)
			for r := 0; r < readers; r++ {
				go func() {
					defer readerWG.Done()

					last := make([]int, n)
					for {
						select {
						case <-stop:
							return
						default:
						}

						snap := s.Scan()
						if len(snap) != n {
							t.Errorf("Scan() returned %d registers, want %d", len(snap), n)
							return
						}
						for i, v := range snap {
							if v < 0 || v > rounds {
								t.Errorf("register %d = %d, want a value in [0, %d]", i, v, rounds)
								return
							}
							if v < last[i] {
								t.Errorf("Scan() moved register %d backwards: %d after %d", i, v, last[i])
								return
							}
							last[i] = v
						}
						for i := 0; i+1 < n; i++ {
							if snap[i] < snap[i+1] {
								t.Errorf("Scan() = %v is not a valid state: register %d = %d < register %d = %d",
									snap, i, snap[i], i+1, snap[i+1])
								return
							}
						}
					}
				}()
			}

			for round := 1; round <= rounds; round++ {
				for i := 0; i < n; i++ {
					s.Update(i, round)
				}
			}
			close(stop)
			readerWG.Wait()

			want := make([]int, n)
			for i := range want {
				want[i] = rounds
			}
			if got := s.Scan(); !slices.Equal(got, want) {
				t.Fatalf("final Scan() = %v, want %v", got, want)
			}
		})
	}
}

// TestSnapshotConcurrentUpdates checks that independent writers can update
// registers concurrently while readers scan, and that each reader never
// observes a register move backwards.
func TestSnapshotConcurrentUpdates(t *testing.T) {
	const (
		n       = 4
		iters   = 1000
		readers = 4
	)

	for _, tc := range snapshotFactories() {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.new(n)

			var writerWG sync.WaitGroup
			writerWG.Add(n)
			for i := 0; i < n; i++ {
				go func(i int) {
					defer writerWG.Done()
					for v := 1; v <= iters; v++ {
						s.Update(i, v)
					}
				}(i)
			}

			stop := make(chan struct{})
			var readerWG sync.WaitGroup
			readerWG.Add(readers)
			for r := 0; r < readers; r++ {
				go func() {
					defer readerWG.Done()

					last := make([]int, n)
					for {
						select {
						case <-stop:
							return
						default:
						}

						snap := s.Scan()
						if len(snap) != n {
							t.Errorf("Scan() returned %d registers, want %d", len(snap), n)
							return
						}
						for i, v := range snap {
							if v < 0 || v > iters {
								t.Errorf("register %d = %d, want a value in [0, %d]", i, v, iters)
								return
							}
							if v < last[i] {
								t.Errorf("Scan() moved register %d backwards: %d after %d", i, v, last[i])
								return
							}
							last[i] = v
						}
					}
				}()
			}

			writerWG.Wait()
			close(stop)
			readerWG.Wait()

			want := make([]int, n)
			for i := range want {
				want[i] = iters
			}
			if got := s.Scan(); !slices.Equal(got, want) {
				t.Fatalf("final Scan() = %v, want %v", got, want)
			}
		})
	}
}
