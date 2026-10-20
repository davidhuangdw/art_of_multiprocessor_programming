package prim

// StampedVal pairs a value with a version stamp.
//
// Published StampedVal values must be treated as immutable. The chapter 4
// snapshot algorithms require stamps to be unique per register, which holds
// when each register has a single writer.
type StampedVal[T any] struct {
	Stamp int
	Value T
}

func EqualStampedVal[T any](a, b *StampedVal[T]) bool {
	return a.Stamp == b.Stamp // assume: equal stamp => equal value, because each write increase stamp
}

func EqualStampedArr[T any](a, b []*StampedVal[T]) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !EqualStampedVal(a[i], b[i]) {
			return false
		}
	}
	return true
}
