# ch07_spin — Spin Locks and Contention

TAS/TTAS/Backoff implement `prim.Lock`; Anderson/CLH/MCS/Composite implement
`prim.ThreadLock` (caller-owned index in `[0, n)`, same index for `Lock` and
`Unlock`).

- [x] TASLock, TTASLock, BackoffLock, AndersonQueueLock, CLHLock, MCSLock
- [x] CompositeLock — no `ABORTED` state, no fast path yet

| Lock | Idea | Space |
|---|---|---|
| TAS | spin on `Swap(true)`; every attempt writes the cache line | O(1) |
| TTAS | spin on `Load`, `Swap` only when it looks free | O(1) |
| Backoff | TTAS + randomized exponential backoff after a failed `Swap` | O(1) |
| Anderson | `tail.Add(1)` hands out FIFO slots; each waits on its own slot | O(n) |
| CLH | publish own node at the tail, spin on the *predecessor's* `free`, recycle it on `Unlock` | O(n) |
| MCS | spin on *own* `free`; predecessor links `nxt=me`, then grants | O(n) |
| Composite | `CAS(FREE,WAITING)` a node from a 64-node pool, splice, spin on predecessor, recycle it; `case 2` revives a `RELEASED` tail node | O(1) |

## Invariants

- **`tail` is the only MRMW register.** Its atomic RMW gives each thread a
  unique predecessor or slot, which is why `free`/`nxt` need only `Store`:
  one successor at a time, FIFO order, and `Unlock` always hands off to it.
- **Single writer per index** — `ind[i]`, `nodes[i]`, `pres[i]`, `alloc[i]` are
  plain fields. Break the `ThreadLock` contract and they become lost updates.
- **Composite:** `tail` never points at a `FREE` node; at most one node is
  `RELEASED`, and at quiescence it is the tail.
- **Composite:** the `StampedTail` cell is fresh per publication, so the cell
  pointer *is* the version (Go's `AtomicStampedReference`).

## Pitfalls

- **Stale grant (CLH/MCS):** reset `free` to false before the predecessor can
  link the node, or an old grant lets you in while someone else holds.
- **A fast path needs one atomic word.** The `FASTPATH` bit must live in the
  tail cell; a separate `atomic.Bool` misses the window where a queued thread
  splices onto an empty tail and is granted while another thread wins the flag.
- **Don't reuse tail cells.** ABA: two threads both see `RELEASED`, both CAS the
  tail, both think they are alone. The 1 cell (8 B) allocated per Composite
  acquisition is the price.
- **`me` is picked once, outside the retry loop**, so landing on a busy node
  means backing off against that node. And `case 2` is load-bearing: it keeps
  the lock moving once the pool is drained.

## Measurements

M1 Pro, `GOMAXPROCS=10`, empty critical section; contended = median per-op
across all goroutines.

| Lock | uncontended ns/op | n=8 ns/op | allocs/op |
|---|---|---|---|
| CLH | 7 | 265 | 0 |
| MCS | 15 | 270 | 0 |
| Composite | 19 | 287 | 1 × 8 B |
| Anderson | 13 | 298 | 0 |
| TTAS | 11 | 119 | 0 |
| Backoff | 11 | 14 | 0 |

Queue locks sit in one band. Composite is not faster than MCS; what it buys is
O(1) space and abortability (not implemented). Backoff wins on an empty critical
section only because it sleeps instead of thrashing cache lines.

## Verify

`spin_test.go`: 8 goroutines per lock, `check.Critical`, a lock-protected `int`
counter, and `TestCompositeLockNodePool` (no leak, `RELEASED <= 1`).

```bash
go test ./ch07_spin -race -count=10
GOMAXPROCS=1 go test ./ch07_spin -race -count=5
go vet ./...
```
