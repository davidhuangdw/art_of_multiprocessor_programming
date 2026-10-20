# ch04_regs — Foundations of Shared Memory

## Algorithms from the book
- [ ] SRSWRegister
- [ ] MRSWRegister
- [ ] MRMWRegister (atomic)
- [x] SimpleSnapshot — obstruction-free, `simple_snapshot.go`
- [x] WFSnapshot — wait-free, `wf_snapshot.go`

`snapshot.go` defines the common `Snapshot` interface. `NewSimpleSnapshot` and
`NewWFSnapshot` build objects with n zero-valued registers; `Update(i, v)`
writes register i and `Scan()` returns a snapshot of all n registers. An index
outside `[0, n)` panics.

Both implementations use MRSW registers: each index must have a single writer.
Updates to different indices and `Scan` calls may run concurrently; concurrent
updates to the same index are not supported. `snapshot_test.go` runs the same
contract tests against both implementations, plus a white-box test that forces
`WFSnapshot.Scan`'s help path.

## Key takeaways
- **SimpleSnapshot**: each update publishes a stamped value. `Scan` repeats
  double collects until two consecutive collects see identical stamps. It is
  obstruction-free, not wait-free.
- **WFSnapshot**: each `Update` first takes a scan and stores it with the new
  value. If `Scan` observes the same register move twice, the second update's
  stored scan was taken within the scan interval and can be returned, making
  `Scan` wait-free.
- Stamps are version numbers used for change detection; they rely on the
  single-writer-per-register assumption to stay unique.

## Go-specific differences / pitfalls
- Published `StampedVal` values are immutable; never mutate a value after
  `atomic.Pointer.Store`, including its `prevScan` slice.
- `WFSnapshot.Scan` returns a fresh `[]int` on both paths: the clean path
  copies the collected values, and the help path clones the stored `prevScan`.
  Callers own the returned slice.
- `SimpleSnapshot.Scan` can starve while updates continue; `WFSnapshot.Scan`
  is wait-free.
- `Update` validates indices; `Scan` always returns a slice of length n.

## Verification
```bash
go test ./ch04_regs -race -count=20
go vet ./...
```

## Benchmark data
Not implemented yet.
