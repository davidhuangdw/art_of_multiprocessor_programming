# ch09_list — Linked Lists: The Role of Locking

`prim.Set`: `Add`/`Remove`/`Contains`, all callable concurrently. Every
implementation keeps the nodes sorted and returns `true` only when the value
actually changed (inserted / removed).

## Algorithms

- [x] CoarseList — `coarse_list.go`
- [x] FineList — `fine_list.go`
- [x] OptimisticList — `optimistic_list.go`
- [x] LazyList — `lazy_list.go`
- [ ] LockFreeList — `lock_free_list.go` (stub)

| Type | Locking | Cost |
|---|---|---|
| `CoarseList` | 1 mutex | correct but fully serial |
| `FineList` | all nodes visited | 2 locks per step, deadlock-free by ordering |
| `OptimisticList` | traversal lock-free, 2 locks + re-validate | short lock hold, retries on conflicts |
| `LazyList` | traversal lock-free, splice under node lock | `Contains` never locks |
| `LockFreeList` | none | CAS + mark bit, traversal also unlinks |

`FineList` and the later three use different node types (`cNode`/`fNode`/`oNode`
with plain, plain, then atomic links): plain links are fine while every access
holds the node's lock, but `OptimisticList`/`LazyList` read links without any
lock, so there `next` is an `atomic.Pointer` and `removed` an `atomic.Bool`.
`key` stays plain — it is written once before publication, and the `next` store
carries it.

## Contract the tests pin

- `Add(v)` is true exactly once per value; `Remove(v)` is true exactly once.
- `Contains` reflects a completed `Add`/`Remove` on the same goroutine.
- Concurrent `Add` of the same value: exactly one caller wins.
- Disjoint concurrent inserts/removes lose nothing.
- A value nobody touches survives arbitrary churn: no lost node, no broken
  neighbour chain.
- `Contains` is safe while others mutate.

## Go-specific differences / pitfalls
- Go has no `AtomicMarkableReference`; `LockFreeList` needs a markable pointer
  (pack the mark bit in the pointer or keep a `{ptr, marked}` cell swapped by
  `atomic.Pointer`). A plain `atomic.Pointer` is not enough.
- Per-node locks are usually `sync.Mutex` values inside the node; nodes must
  never be copied after first use (`go vet` catches `copylocks`).
- `Remove` under fine-grained locking must not hold a lock while allocating;
  allocate the node before taking any lock.
- `OptimisticList` must never lock while re-traversing to validate: its own
  traversal already holds `pre`+`cur`, and `sync.Mutex` is not reentrant, so a
  locking validate deadlocks the goroutine against itself.
- Sentinels are `math.MinInt`/`math.MaxInt` (Go has no ±∞). `math.MaxInt` is
  therefore *reserved*: `Contains(math.MaxInt)` is always true, and
  `Remove(math.MaxInt)` unlinks the tail sentinel and breaks every later
  traversal. Values used by the tests stay well inside the range.
- Tests include negative values so a sentinel-comparison bug shows up.

## Tests

`list_test.go` runs every implementation through the same contract:
`TestSetSequential`, `TestSetConcurrentDisjointAdds`,
`TestSetConcurrentDuplicateAdd`, `TestSetConcurrentDisjointRemoves`,
`TestSetChurn` (watched value + add/remove churn) and `TestSetConcurrentReaders`.
Stress runs go through `check.StressTimeout`, so a deadlock fails in 10s.

```bash
go test ./ch09_list -race -count=10
GOMAXPROCS=1 go test ./ch09_list -race -count=3
go vet ./...
```

## Benchmark data
Not implemented yet.
