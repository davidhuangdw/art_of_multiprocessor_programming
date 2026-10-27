package prim

// Stack is the concurrent LIFO stack implemented by the Chapter 11 stacks.
//
// Push adds v on top. Pop removes and returns the top value, reporting ok ==
// false when the stack is empty; it never parks.
type Stack interface {
	Push(v int)
	Pop() (v int, ok bool)
}
