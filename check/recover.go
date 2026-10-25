package check

import (
	"runtime/debug"
	"testing"
)

// Recover turns a panic in the calling test into a test failure, so one
// crashing case cannot abort the rest of the package's run.
//
// Use it as the first statement of a subtest:
//
//	t.Run(name, func(t *testing.T) {
//		defer check.Recover(t)
//		...
//	})
//
// Without it a panic in any test kills the whole test binary and the remaining
// tests in that package never run.
func Recover(tb testing.TB) {
	tb.Helper()
	if r := recover(); r != nil {
		tb.Errorf("panic: %v\n%s", r, debug.Stack())
	}
}
