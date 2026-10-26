# ch10_queue — Concurrent Queues and the ABA Problem

`prim.Queue`: `Dequeue` reports `ok == false` when empty, never parks.
`prim.BlockingQueue`: `Dequeue` parks while empty. Both are FIFO, so one
goroutine's own enqueues always come out in that order.

| Type | File | Locking | Progress |
|---|---|---|---|
| `BoundedQueue` | `bounded_queue.go` | one lock per end + atomic `size` + one cond per end | blocking, capacity-limited |
| `LockFreeQueue` | `lock_free_queue.go` | none: CAS on `tail.next` and `head` | lock-free (Michael-Scott) |
| `StampedQueue` | `stamped_queue.go` | none: CAS on versioned `{stamp, node}` cells | lock-free, ABA-proof |
| `DualQueue[T]` | `dual_queue.go` | none: one `item` CAS per pairing | lock-free, no buffer |

## Key takeaways

- **One lock per end** (`BoundedQueue`). `tail` is written only under `enqLock`,
  `head` only under `deqLock`. `size` is the one word both sides touch, so it is
  atomic; a plain `int` would be a data race and would force a single lock. The
  array slots differ while `size < capacity`, so `items` needs no lock either.
- **A wake-up has to cross the two locks.** A consumer parks on `notEmpty`
  (built on `deqLock`) and only the enqueue side can release it: the producer
  signals *after* releasing `enqLock`, while holding `deqLock`. That order is
  what makes the wake-up unmissable. Symmetric for `notFull`/`enqLock`.
- **`Broadcast`, not `Signal`.** Wake-ups happen only on the edge (empty →
  non-empty). One edge may have to release several parked callers: one wins the
  item, the rest must be woken now, because a later enqueue no longer crosses the
  edge.
- **A dummy node is what makes a queue lock-free.** `head` points at a dummy
  whose `next` is the oldest item, so `Dequeue` CASes `head` while `Enqueue`
  CASes `tail.next`: no shared word. Two steps still need *helping* — a thread
  that finds `tail.next != nil` advances `tail` for the other one, otherwise one
  stalled thread stalls everyone.
- **ABA needs node reuse.** A pointer CAS is fooled if the pointer leaves and
  returns unchanged. Go's GC never reuses a node, so `LockFreeQueue` is safe;
  recycle nodes (a free list, a pool) and ABA returns. `StampedQueue` fixes it by
  publishing a **fresh `{stamp, node}` cell** per write, so pointer identity *is*
  the version — the same trick as Chapter 9's `{removed, next}`.
- **Parking is safe only while the queue stays homogeneous.** A call queues
  behind its own kind, so no opposite-kind node can be hidden behind a parked
  one; that is what lets it wait on its own `item`. A call also publishes one node
  or none — never pair elsewhere after publishing, or the value can be delivered
  twice (`TestBlockingQueueNoLoss` catches it).
- **A dual node's `item` is the pairing word.** `nil` means "not paired yet", so
  it must never also mean "already taken": a taken offer still sits at
  `head.next` and would then look like a waiting reservation. Read `v := item` and
  only `CAS(v, nil)` when `v != nil`; otherwise help the head forward and retry.

## The trick: a cheap hint, then validate at the commit point

One CAS protects only its own word, so "decide then act" cannot be atomic when the
decision needs two words. The CAS here is therefore not "publish a value" but
"assert and publish in one step" — the only operation that can freeze an instant.
Split the roles instead:

| | reads | stale allowed? |
|---|---|---|
| hint: pair, or append? | kind of `head.next` | yes — a wrong hint costs one retry |
| commit: is this append legal? | `t.request`, then `t.next` CAS | no |

Neither half of that validation comes from the CAS itself: `next` only ever goes
nil → non-nil, so a winning CAS proves `t` was the last node *at commit time*; and
`kind` never changes, so reading it *before* the CAS equals reading it *at* the
CAS. Together they check a two-word precondition with one word — which is also the
constraint: only what that word can see is validated, so any other field must be
immutable (as `kind` is) or packed into it. Make `kind` mutable and the argument
collapses; the field then has to move into the same word (`StampedQueue`'s
`{stamp, node}`, Chapter 9's `{removed, next}`) or be pinned by a lock (Chapter 8,
`LazyList`). The same hint-then-validate shape drives Michael-Scott
(`t == tail.Load()` re-read) and `LazyList` (`pre.next == cur` under the lock).

## Go notes / pitfalls

- `sync.Cond` needs one cond per waiter class *and* its own lock, hence two conds
  on two mutexes. Signalling a cond whose lock you do not hold loses wake-ups.
- Re-read and compare before linking in a Michael-Scott loop
  (`t := q.tail.Load(); ... if t != q.tail.Load() { continue }`), or you append
  to a node that has already been unlinked.
- `next.item` may be read *before* winning the CAS on `head`: `item` is written
  once before publication, so the acquiring `Load` of the link carries it (same
  rule as Chapter 9's `key`).
- Never call `t.Errorf` from a worker goroutine that may outlive its test — it
  panics ("Fail in goroutine after test has completed"). `queue_test.go` funnels
  worker failures through a `reporter` and reports them after joining.
- Order is only observable with a **single consumer**; with several racing
  consumers the check itself races. `...NoLoss` checks the multiset,
  `...SingleConsumerOrder` checks FIFO order. `settleTime` is the only way to
  observe "has parked" from outside.

## Tests

Split by interface, because only `BoundedQueue` and `DualQueue` park.

- `prim.Queue` (`LockFreeQueue`, `StampedQueue`): `TestQueueSequential`,
  `TestQueueSingleProducerConsumer`, `TestQueueConcurrentNoLoss`,
  `TestQueueSingleConsumerOrder`.
- `prim.BlockingQueue` (`BoundedQueue`, `DualQueue`): `TestBlockingQueueNoLoss`,
  `TestBlockingQueueSingleConsumerOrder`.
- Per type: `TestBoundedQueueWraparound`, `TestBoundedQueueBlocksWhenFull`,
  `TestBoundedQueueDequeueParks`, `TestStampedQueueStampAdvances`,
  `TestDualQueueHandoff`, `TestDualQueueOffersAreFIFO`.

Every concurrent body runs under a 10s deadline, so a lost item or a deadlock
fails the test instead of hanging until the `go test` timeout.

```bash
go test ./ch10_queue -race -count=10
GOMAXPROCS=1 go test ./ch10_queue -race -count=3
go vet ./...
```

## Benchmark data
Not implemented yet.
