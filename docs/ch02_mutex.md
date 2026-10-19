# ch02_mutex — Mutual Exclusion

Package: `ch02_mutex` (package `mutex`)

## Algorithms
- [x] Peterson — two threads, `peterson.go`
- [x] Bakery — n threads, `bakery.go`
- [ ] LockOne / LockTwo — omitted: both can spin forever under arbitrary interleaving

`mutex_test.go` checks mutual exclusion and a lock-protected counter.

## Key takeaways
- **Peterson**: one flag per thread plus a `victim` register; the last writer of
  `victim` waits, so exactly one thread enters. Deadlock- and starvation-free.
- **Bakery**: ticket = `1 + max` active label; wait until no participant has a
  smaller `(label, index)`. Ties are broken by thread index, and waiting threads
  keep their ticket visible, so entry is first-come-first-served.

## Implementation notes
- Shared flags are `atomic.Bool` / `atomic.Int64`; plain variables are not
  sequentially consistent under the Go memory model.
- Callers must pass the same index `i` in `[0, n)` to `Lock` and `Unlock`.
- `flag` means "participating", and `label == 0` means "choosing"; the wait
  condition must keep treating `0` as highest priority.
- Spin loops only read atomics, so they are preemptible but burn CPU. Backoff
  and queue locks come in Chapter 7.
- Tests use `-race`, `check.Stress`, `check.Critical`, and a plain `int`
  counter that the lock must protect.

## Verification
```bash
go test ./ch02_mutex -race -count=20
GOMAXPROCS=1 go test ./ch02_mutex -race -count=5
go run ./examples/mutex
```

## Benchmarks
Not implemented yet. Intended comparison: Peterson and Bakery versus
`sync.Mutex` at 2, 4 and 8 threads.

