# ch08_monitor — Monitors and Blocking Synchronization

`prim.Monitor` = blocking `Lock`/`Unlock` + `NewCondition() prim.Condition`.
`Await` releases the monitor, parks, and reacquires it: call it while holding
the monitor, and always inside `for !predicate`.

## Algorithms

- [x] Monitor over `sync.Mutex` + `sync.Cond` — `SimpleMonitor`
- [x] Bounded queue (producer-consumer) — `BoundedQueue`
- [x] Readers-writer lock — `ReadersWriter` (writer preference)
- [x] Approximate-FIFO readers-writer — `FifoReadersWriter` (batch FIFO)
- [x] Re-entrant lock — `SimpleReentrantLock`
- [x] Counting semaphore — `Semaphore`

| Type | Built on | Point |
|---|---|---|
| `SimpleMonitor` | `sync.Locker` + `sync.Cond` | `CondWrap` maps `Await→Wait`, `SignalAll→Broadcast` |
| `BoundedQueue` | mutex + `notFull`/`notEmpty` | ring buffer (`head`/`count`), one cond per edge |
| `ReadersWriter` | mutex + one cond | writer preference |
| `FifoReadersWriter` | mutex + cond + writer tickets | approximate (batch) FIFO |
| `SimpleReentrantLock` | mutex + cond + owner token | re-entrant; also a `prim.ThreadLock` |
| `Semaphore` | mutex + cond + counter | one waiter class; anyone may `Release` |

## Rules

- **One cond per waiter class.** `Signal` wakes a single waiter; if its
  predicate is still false it sleeps again and that wake-up is lost.
  `ReadersWriter` deadlocked exactly this way — mixing classes forces
  `Broadcast`.
- `Signal`/`Broadcast` require the monitor held; `Await` requires the `for`
  loop.
- `ReadersWriter` raises `write` *before* waiting for readers to drain, so late
  readers block; `ReadUnlock` broadcasts only when `reads == 0 && write`.
- Re-entrant lock: `Unlock` must panic at `count == 0`; otherwise the count goes
  negative and `Lock` stops excluding.
- A semaphore is not re-entrant: permits are counted, no owner, anyone may
  release.
- No thread ID in Go, so the re-entrant lock keys on an owner token. Recursive
  read locks deadlock, as in `sync.RWMutex`.

## FIFO readers-writer: three levels

| | Order | Cost |
|---|---|---|
| `ReadersWriter` | writers never starve, but a reader may slip between two queued writers | cheapest |
| `FifoReadersWriter` | batch: readers admitted together are unordered, writers are FIFO | one ticket per writer |
| strict FIFO | every request ordered | a broadcast per admission, reader batches sliced |

`FifoReadersWriter` guarantees:

- readers admitted together hold the lock concurrently, in **any order**;
- a reader that arrives while a writer is queued waits for that writer;
- writers keep arrival order among themselves (one ticket each).

It deliberately does not order a reader that is *already waiting* against the
writer queued after it — that would cost a ticket per reader plus a broadcast
per admission, which is what batching avoids.

Strict FIFO needs both:

```go
t := rw.next++                                       // readers take tickets too
for rw.serving != t || rw.writing { rw.cond.Wait() } // writers: reads == 0 too
rw.serving++
rw.readers++
rw.cond.Broadcast()
```

## Tests

`monitor_test.go`: mutual exclusion and a protected counter for every type,
`Signal` vs `SignalAll`, `Await` releasing the monitor, `BoundedQueue`
FIFO/wraparound, reader overlap and writer preference, re-entrancy and
over-unlock, semaphore limit/handshake, and for `FifoReadersWriter`
`TestFifoReadersWriterBatchReadersShare` (readers share) plus
`TestFifoReadersWriterNoBarge` (no reader jumps a queued writer). Stress runs go
through `check.StressTimeout`, so a deadlock fails in 10s instead of hanging
until the `go test` timeout.

```bash
go test ./ch08_monitor -race -count=10
GOMAXPROCS=1 go test ./ch08_monitor -race -count=3
go vet ./...
```

## Benchmark data
Not implemented yet.
