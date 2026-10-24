package prim

// Condition is a condition variable bound to the monitor that created it.
//
// Await atomically releases the monitor, blocks until Signal or SignalAll
// wakes it, and then reacquires the monitor before returning. Callers must
// re-check their predicate in a loop: a wake-up does not guarantee the
// predicate now holds.
type Condition interface {
	Await()
	Signal()
	SignalAll()
}

// Monitor is a blocking lock that hands out condition variables.
//
// It is the Chapter 8 counterpart of Lock: instead of spinning, waiters are
// parked by the scheduler.
type Monitor interface {
	Lock()
	Unlock()
	NewCondition() Condition
}
