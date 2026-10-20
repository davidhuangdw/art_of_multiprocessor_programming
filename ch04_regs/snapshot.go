// Package regs implements the shared-memory register algorithms from
// Chapter 4, Foundations of Shared Memory.
package regs

// Snapshot is the interface shared by the snapshot implementations in this
// chapter.
//
// A snapshot holds n registers, each initialized to zero. Update writes one
// register and Scan returns an atomic view of all n registers. The register
// index passed to Update must be in [0, n).
//
// Scan returns a newly allocated slice of length n that the caller owns.
//
// Each index has a single writer: concurrent updates to different indices and
// concurrent Scan calls are safe, but concurrent Update calls for the same
// index are not supported.
type Snapshot interface {
	Update(i, v int)
	Scan() []int
}

func validateSize(n int) {
	if n <= 0 {
		panic("regs: snapshot size must be positive")
	}
}

func checkIndex(i, n int) {
	if i < 0 || i >= n {
		panic("regs: register index out of range")
	}
}
