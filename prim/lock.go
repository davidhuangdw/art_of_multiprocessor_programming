// Package prim holds the interfaces shared by the chapter implementations.
package prim

// Lock is the classic mutex interface used from Chapter 7 onward.
type Lock interface {
	Lock()
	Unlock()
}

// ThreadLock is the thread-indexed mutex interface used in Chapter 2.
//
// Each caller owns a fixed thread index in [0, n). The lock must be used with
// that index for every call; passing a different index for the same goroutine
// breaks the algorithm's invariants.
type ThreadLock interface {
	Lock(i int)
	Unlock(i int)
}
