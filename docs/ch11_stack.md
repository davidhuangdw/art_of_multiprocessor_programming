# ch11_stack — Concurrent Stacks and Elimination

`prim.Stack`: `Push` / `Pop` (`ok == false` when empty, never parks). LIFO, so a
stack filled and then drained yields the exact reverse order.

| Type | File | Locking | Progress |
|---|---|---|---|
| `LockedStack` | `locked_stack.go` | one mutex around a slice | blocking (baseline) |
| `LockFreeStack` | `lock_free_stack.go` | none: one CAS on `top` | lock-free (Treiber) |
| `Exchanger` | `exchanger.go` | none: three-state slot CAS | lock-free pairing |
| `EliminationArray` | `elimination_array.go` | none: random `Exchanger` per visit | lock-free |
| `EliminationStack` | `elimination_stack.go` | stack first, arena on CAS failure | lock-free |

## Key takeaways

- **One hot word is the whole cost.** `LockFreeStack` CASes `top` in both
  operations, so every thread contends on one cache line: correct, but it does not
  scale. Elimination moves the waiting off that word.
- **ABA needs node reuse.** `Pop` CASes `top` from `old` to `old.next`; if a node
  could be freed and reallocated in between, the CAS could succeed on a stack that
  has moved on. Go's GC keeps a node alive while a reader holds it — the hazard
  comes back with a free list.
- **An exchanger is a three-state machine, not a queue.** `EMPTY` / `WAITING` (one
  payload parked) / `BUSY` (the partner took it and left its own). Only one payload
  can wait, so a pairing is a single CAS, and `BUSY` is how the waiter collects the
  partner's payload without a second rendezvous.
- **Withdrawing must CAS against your own cell.** On timeout the waiter retires the
  cell *it* published, never "the cell it last read": when the wait loop never ran
  (short timeout, preemption) that observation is still the old `EMPTY` cell and the
  CAS misses. The one bug this chapter produced. The unreachable "not BUSY" branch
  after it asserts rather than returns — returning would let a push deliver twice.
- **Start the arena range small.** A short wait sends the caller back to the stack —
  and back to a *new random slot* — within microseconds, so callers collide often.
  Measured, 8 threads, 16 slots: fixed 1 ms gave **0** pairings, an adaptive
  1 µs…1 ms range gave some. That is the book's RangePolicy, here one per stack
  instead of one per thread.
- **The pairing rule is the only thing preventing a lost or invented value.** An
  exchanger swaps *any* two callers, so push+push and pop+pop also happen and both
  must fall back to the stack. Kept in one place (`pushPaired`/`popPaired`) because
  it is then testable without scheduling luck.
- **Elimination is contention relief, not a default win.** The stack is tried first,
  so a lone caller never reaches the arena; below ~16 threads it measured ~3x
  *slower*. Benchmark data has the two failure modes and the four conditions.

## Go notes / pitfalls

- The book exchanges `T` with `null` as "I am a popper". An `int` has no spare value,
  so `Exchanger` exchanges `*int` with `nil` as the sentinel.
- A fresh immutable cell per state transition makes pointer identity the version
  (as in Chapter 10's `{stamp, node}`).
- Spin loops call `runtime.Gosched()`: otherwise a timeout becomes a hang under
  `GOMAXPROCS=1`.
- Never call `t.Errorf` from a worker goroutine that may outlive its test — it
  panics ("Fail in goroutine after test has completed"). `stack_test.go` funnels
  worker failures through a `reporter`.
- LIFO is not observable across a *concurrent* pusher/popper: a pop may return a
  value pushed after the previous pop, so the sequence can ascend. Test order after
  quiescence, and test concurrency only for "each value exactly once".
- Elimination assertions are scheduling-dependent: with 1–2 Ps no stack CAS fails,
  so the arena is never visited (measured 0). `TestEliminationStackHandoff` skips
  below 4 Ps and repeats the contended workload until a pairing happens.

## Tests

Contract, over all three stack types: `TestStackSequential`,
`TestStackLIFOWithDepth` (500 in, 500 out, exactly descending),
`TestStackSingleProducerConsumerNoLoss`, `TestStackConcurrentNoLoss`
(8 pushers × 100 values, none lost or duplicated).

Elimination machinery:

- `TestPairingRule` — the four cases of the pairing rule, deterministically.
- `TestExchangerPair`, `TestExchangerTimeout` (a lone caller gives up **and** leaves
  no `WAITING` slot), `TestExchangerTinyTimeout` (timeout shorter than a scheduling
  round), `TestExchangerReusableAfterTimeout`, `TestExchangerConservesPayloads`.
- `TestEliminationArrayPair` (single slot), `TestEliminationArrayLoneVisit`.
- `TestEliminationStackHandoff` — the arena really serves pairs under contention.
- `TestEliminationStackConcurrentPushesNoLoss` / `...ConcurrentPopsNoInvention` —
  same-kind pairings must fall back; needs more goroutines than Ps to fire.

```bash
go test ./ch11_stack -race -count=10
GOMAXPROCS=1 go test ./ch11_stack -race -count=3
go vet ./...
```

## Benchmark data

**Unit**: millions of push+pop *pairs* per second (1 pair = 1 `Push` + 1 `Pop`),
each goroutine running 20k pairs. Ad-hoc probe — not `go test -bench`, no `-race`,
Go 1.27, arm64, 10 logical CPUs (8P+2E), single run: read magnitudes and gaps, not
the last digit. `Elimination slots=N` is the arena width (default 16, adaptive
range 1µs…1ms).

| Threads | `LockFreeStack` | slots=4 | slots=16 |
|---|---|---|---|
| 2 | 16.1M | 15.4M | 17.7M |
| 4 | 20.8M | 8.8M | 10.0M |
| 8 | 17.2M | 3.3M | 13.3M |
| 16 | 16.0M | 3.1M | 9.1M |

(The two tables are separate runs and differ by ~20% — that is the run-to-run
noise; only the gaps within one table are meaningful.)

### Why it loses here

Instrumented run (20k pairs per goroutine), counting `top` CAS failures and how
often a caller found its arena slot `BUSY`:

| Threads | plain | CAS failures | slots=4 | BUSY hits | pairings | slots=64 | BUSY hits |
|---|---|---|---|---|---|---|---|
| 8 | 21.6M | 8.2% | 7.0M | 203k | 4.9k | 24.7M | 0 |
| 32 | 21.6M | 8.5% | 9.6M | 788k | 5.8k | 20.1M | 204k |
| 128 | 21.7M | 8.2% | 9.9M | 2.9M | 3.5k | 21.3M | 1.5M |

Two failure modes, one on each side of the optimum:

- **Too narrow → BUSY convoy.** A slot holds one handshake at a time: the first
  caller parks `WAITING`, the second turns it into `BUSY`, and a third can only
  spin — only the original waiter clears `BUSY`. At `slots=4` that is ~42 wasted
  spins per successful pairing.
- **Too wide → wasted probes.** Collisions get rare, so almost every probe waits
  out its range and falls back to the stack: overhead, but small (`slots=64` ≈
  plain).

The optimum sits near the number of callers inside the arena at once.

### When it does win

Only when the baseline has already degraded, i.e. the shared word is the
bottleneck. All four have to hold:

| # | Condition | Here |
|---|---|---|
| 1 | the plain CAS saturates: threads ≫ cores, or many cores / NUMA, dropping throughput below its uncontended speed | ❌ flat at 21.6M from 8 to 128 goroutines — 8% failures at ~30 ns never queue up |
| 2 | a balanced supply of the opposite kind, or nothing pairs | only the paired workload |
| 3 | arena width ≈ concurrent visitors | `slots=64` ≈ plain, `slots=4` −60% |
| 4 | the shared point costs more than one CAS (ABA stamping, node reuse, a lock), so one handoff replaces the lot | single CAS here, so no amplification |

Treat it as a **latency/fairness** device — a long retry becomes a short wait
served by the partner — not a throughput device, unless condition 1 holds.
