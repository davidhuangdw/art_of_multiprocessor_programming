# The Art of Multiprocessor Programming — Go Exercises

Go implementations of the algorithms from *The Art of Multiprocessor Programming* (Herlihy & Shavit).

## Layout

| Directory | Purpose |
|---|---|
| `prim/` | Shared interfaces (Lock / Register / Consensus / Queue / Barrier) and TAS/CAS wrappers |
| `check/` | Concurrency test kit: stress loops, invariant checks, linearizability validation |
| `chXX_topic/` | One package per chapter; file names match algorithm names |
| `docs/` | Per-chapter notes (algorithm checklist, takeaways, Go differences, benchmarks) |
| `examples/` | Runnable examples, e.g. `go run ./examples/mutex` |

Dependency direction: `prim` -> `check` -> `chXX`. Later chapters may import earlier ones; the reverse is forbidden.

## Commands

```bash
make race                 # -race across all packages
make stress DIR=./ch07_spin
make bench  DIR=./ch07_spin
make cover
```

## Progress

| Ch | Directory | Topic | Status |
|---|---|---|---|
| 2 | `ch02_mutex` | Mutual exclusion | Done |
| 3 | `ch03_objects` | Concurrent objects | Not started |
| 4 | `ch04_regs` | Foundations of shared memory | Done |
| 5 | `ch05_consensus` | Relative power of synchronization operations | Not started |
| 6 | `ch06_universal` | Universality of primitive synchronization | Not started |
| 7 | `ch07_spin` | Spin locks and contention | Not started |
| 8 | `ch08_monitor` | Monitors and blocking synchronization | Not started |
| 9 | `ch09_list` | Concurrent linked lists | Not started |
| 10 | `ch10_queue` | Concurrent queues and the ABA problem | Not started |
| 11 | `ch11_stack` | Concurrent stacks and elimination | Not started |
| 12 | `ch12_count` | Counting, sorting, and distributed coordination | Not started |
| 13 | `ch13_hash` | Concurrent hashing | Not started |
| 14 | `ch14_skiplist` | Skiplists and balanced search | Not started |
| 15 | `ch15_pqueue` | Priority queues | Not started |
| 16 | `ch16_sched` | Scheduling and work distribution | Not started |
| 17 | `ch17_barrier` | Barriers | Not started |
| 18 | `ch18_tm` | Transactional memory (approximation) | Not started |
