# ch18_tm — Transactional Memory (Approximation)

Go has no language-level transactional memory. This chapter approximates it with versioned reads plus optimistic retries; the notes must spell out how it differs from hardware TM (no atomic multi-address commit, no hardware conflict detection).

## Algorithms from the book
- [ ] Versioned reads/writes with a retry loop (simulated TM)
- [ ] Compare against `sync.Mutex` / `sync.RWMutex`

## Key takeaways

## Go-specific differences / pitfalls

## Benchmark data
