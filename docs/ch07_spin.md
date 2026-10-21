# ch07_spin — Spin Locks and Contention

## Algorithms from the book
- [x] TASLock — `tas_lock.go`
- [x] TTASLock — `ttas_lock.go`
- [x] BackoffLock — `backoff_lock.go`
- [x] AndersonQueueLock — `anderson_lock.go`
- [x] CLHLock — `clh_lock.go`
- [x] MCSLock — `mcs_lock.go`
- [ ] CompositeLock — `composite_lock.go` (stub)

TAS/TTAS/Backoff implement `prim.Lock`. Anderson/CLH/MCS/Composite implement
`prim.ThreadLock` and require a thread index in `[0, n)`; the same index must
be used for `Lock` and `Unlock`.

`spin_test.go` checks a basic Lock/Unlock path, mutual exclusion with
`check.Critical`, and a plain counter protected only by each lock.

`CompositeLock` is scaffolded; `Lock`/`Unlock` currently panic, so the full
`spin_test.go` run is red until it is implemented.

## Key takeaways
- **TASLock**: spins on `Swap(true)`. Every attempt writes the cache line, so
  contention is expensive.
- **TTASLock**: first spins on `Load`, and only attempts `Swap(true)` after
  observing the lock free. This reduces cache-line traffic under contention.
- **BackoffLock**: TTAS plus randomized exponential backoff after a failed
  `Swap`. `prim.Backoff` starts at `minDelay` and doubles up to `maxDelay`.
- **AndersonQueueLock**: `tail.Add(1)` assigns FIFO slots. Each thread waits
  on its own `que` slot; `Unlock` clears the current slot and sets the next one.
- **CLHLock**: each thread publishes its own node at the tail and spins on the
  predecessor's `free` flag. `Unlock` releases its node and reuses the
  predecessor node, so no allocation is needed per acquisition.
- **MCSLock**: each thread spins on its own `free` flag. The predecessor links
  `nxt = me` and then grants the lock by setting `me.free = true`. If there is
  no successor, `Unlock` CASes the tail back to nil; otherwise it waits for the
  successor link and hands off.
- **CompositeLock**: a fast path (typically a TAS-like acquisition) combined
  with a queue of per-thread composite nodes for the contended path; planned,
  not implemented yet.

## Go-specific differences / pitfalls
- Go has no thread ID, so queue locks take an explicit per-goroutine index via
  `prim.ThreadLock`. The same index must be used for `Lock` and `Unlock`.
- Spin loops must read and write atomics; plain variables do not provide the
  required synchronization under the Go memory model.
- `AndersonQueueLock.ind` is a plain `[]int`: it is safe only when each index
  has a single writer, which is exactly the `ThreadLock` contract.
- CLH/MCS reuse per-thread nodes. The grant flag must be reset before the
  predecessor links the node (`pre.nxt.Store`); otherwise a stale `true` lets
  the thread enter while a predecessor still holds the lock.
- MCS `Unlock` is not wait-free: if the tail CAS fails, it spins until the
  successor links itself. This is the standard MCS handoff; acquisition remains
  FIFO.
- `BackoffLock` validates `minDelay`/`maxDelay`. Direct users of
  `prim.NewBackoff` must also pass a positive range, because `Backoff` uses
  `rand.N(limit)` and would panic on a non-positive limit.
- Spin loops burn CPU. Use `-race` plus `check.Stress` and `check.Critical`
  for correctness, and benchmarks for contention behavior.

## Verification
```bash
# Implemented locks only
go test ./ch07_spin -race -count=200 \
  -run 'TestSpinLock(Smoke|MutualExclusion|Counter)/(TASLock|TTASLock|BackoffLock|AndersonQueueLock|CLHLock|MCSLock)$'

# Full suite; red until CompositeLock is implemented
go test ./ch07_spin -race -count=200
go test ./... -count=1
go vet ./...
```

## Benchmark data
Not implemented yet. Intended comparison: TAS/TTAS/Backoff and the queue locks
at increasing contention levels.
