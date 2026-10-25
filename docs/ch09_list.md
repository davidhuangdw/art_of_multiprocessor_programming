# ch09_list — Linked Lists: The Role of Locking

`prim.Set`: `Add`/`Remove`/`Contains`, all callable concurrently. Nodes stay
sorted and an operation returns `true` only when the value actually changed.

| Type | File | Locking | Progress |
|---|---|---|---|
| `CoarseList` | `coarse_list.go` | one mutex for the whole list | blocking |
| `FineList` | `fine_list.go` | hand-over-hand, 2 node locks per step | blocking |
| `OptimisticList` | `optimistic_list.go` | lock-free walk, then lock `pre`+`cur` and re-validate | blocking |
| `LazyList` | `lazy_list.go` | lock-free walk, mark + splice under node locks | blocking |
| `LockFreeList` | `lock_free_list.go` | none: CAS on `{removed, next}` cells | lock-free |

Node types differ on purpose: `cNode`/`fNode` keep a plain `next` (every access
holds that node's lock), while `oNode` uses `atomic.Pointer`/`atomic.Bool` and
`eNode` a `{removed, next}` cell, because their traversals read links with no
lock held. `key` stays plain in all of them: it is written once before the node
is published, and the release store that publishes the node carries it.

## Contract the tests pin

- `Add`/`Remove` report `true` exactly once per value; concurrent `Add` of the
  same value has exactly one winner.
- A completed `Add`/`Remove` is visible to `Contains` on the same goroutine.
- Disjoint concurrent inserts/removes lose nothing, and a value nobody touches
  survives churn with its neighbours intact.
- `Contains` is safe while others mutate.

## Go notes / pitfalls

- No `AtomicMarkableReference`: `LockFreeList` represents the link as an
  immutable `{removed, next}` cell behind `atomic.Pointer`, so "marked?" and
  "successor?" come from one load. Publishing a **fresh cell per write** makes
  the pointer itself the version, which is what rules out ABA.
- A marked-but-still-linked node must never block anybody: traversal splices
  marked nodes out (the book's snip). Merely *skipping* them still spins forever
  in validation — `TestLockFreeMarkedNodeDoesNotBlock` pins this.
- Marking is the linearization point, so `LockFreeList.Remove` returns `true`
  once the mark CAS wins even if the unlink CAS loses; the next traversal
  finishes the unlink. Never panic because the marked node is already gone.
- Lock-free reads require atomics: without them arm64 hands out a
  half-published node (`key=0, next=nil`). `make race` is the gate.
- `OptimisticList` validates by re-walking **without** locking: it already holds
  `pre`+`cur`, and `sync.Mutex` is not reentrant, so a locking validate
  deadlocks the goroutine against itself.
- Per-node locks live inside the node, so a node must never be copied after its
  first use (`go vet` catches `copylocks`); allocate before locking.
- Sentinels are `math.MinInt`/`math.MaxInt` (Go has no ±∞), so `MaxInt` is
  reserved: `Contains(MaxInt)` is always true and `Remove(MaxInt)` unlinks the
  tail sentinel and breaks every later traversal.

## Tests

`list_test.go` runs all five through one contract: `TestSetSequential`,
`TestSetConcurrentDisjointAdds`, `TestSetConcurrentDuplicateAdd`,
`TestSetConcurrentDisjointRemoves`, `TestSetChurn` (watched value + add/remove
churn) and `TestSetConcurrentReaders`, plus `TestLockFreeMarkedNodeDoesNotBlock`
(progress; `LockFreeList` only). Stress runs go through `check.StressTimeout`,
so a deadlock fails in 10s instead of hanging until the `go test` timeout.

```bash
go test ./ch09_list -race -count=10
GOMAXPROCS=1 go test ./ch09_list -race -count=3
go vet ./...
```

## Benchmark data
Not implemented yet.
