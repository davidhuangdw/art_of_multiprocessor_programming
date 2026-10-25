package prim

// Set is the sorted-set interface implemented by the Chapter 9 linked lists.
//
// Add returns true when v was absent and is now present, Remove returns true
// when v was present and is now absent, and Contains reports membership. All
// three may run concurrently.
type Set interface {
	Add(v int) bool
	Remove(v int) bool
	Contains(v int) bool
}
