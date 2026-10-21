# ch05_consensus — The Relative Power of Synchronization Operations

## Algorithms from the book
- [x] Consensus interface — `prim.Consensus[T]`
- [ ] Consensus number hierarchy
- [ ] Atomic registers (consensus number = 1)
- [x] CAS solves consensus for any number of threads — `cas_consensus.go`
- [ ] FIFO queues / stacks / counters (consensus number = 2; cannot solve 3-consensus)

`prim.Consensus[T]` defines `Decide(value T) T`. `CASConsensus` stores the
first proposal with an `atomic.Pointer[T]`; the first successful CAS decides
the value, and every later caller returns that same value. Each consensus
object is one-shot: create a new one for each consensus round.

`consensus_test.go` covers a single caller, agreement, validity, and
wait-freedom under concurrent calls.

## Key takeaways
- CAS has an unbounded consensus number: it solves wait-free consensus for any
  number of threads.
- `CASConsensus.Decide` performs one CAS and one load, so every caller
  completes in a bounded number of steps.
- The first successful `CompareAndSwap(nil, ...)` wins. Losers load the
  winner's value instead of retrying or blocking.
- CAS is from the "universal" class of synchronization primitives: it is
  stronger than atomic registers, queues, stacks, and counters.

## Go-specific differences / pitfalls
- `&value` passed to `atomic.Pointer[T]` escapes to the heap, so the stored
  pointer remains valid after `Decide` returns. Published values must not be
  mutated afterwards.
- With reference types (`*T`, slices, maps), consensus decides the reference
  value, not a deep copy. Treat the decided value as immutable.
- A `CASConsensus` is not reusable for a second decision round; allocate a new
  object per round.
- Go's `CompareAndSwap` does not fail spuriously; once the pointer is non-nil,
  it never returns to nil, so there is no ABA issue here.

## Verification
```bash
go test ./ch05_consensus -race -count=20
go vet ./...
```

## Benchmark data
Not implemented yet.
