package prim

// Consensus is the wait-free consensus interface from Chapter 5.
//
// Every participating goroutine calls Decide exactly once with its proposed
// value. All calls must return the same value, and that value must be one of
// the proposed values.
type Consensus[T any] interface {
	Decide(value T) T
}
