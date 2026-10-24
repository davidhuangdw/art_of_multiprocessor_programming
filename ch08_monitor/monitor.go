// Package monitor implements the monitor-based synchronization from
// Chapter 8.
package monitor

import (
	"sync"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

// SimpleMonitor is a monitor built from a blocking mutex plus condition
// variables.
type SimpleMonitor struct {
	lock sync.Locker
}

// NewSimpleMonitor returns an unlocked monitor.
func NewSimpleMonitor() *SimpleMonitor {
	lock := &sync.Mutex{}
	return &SimpleMonitor{lock: lock}
}

// Lock acquires the monitor, blocking while another goroutine holds it.
func (m *SimpleMonitor) Lock() {
	m.lock.Lock()
}

// Unlock releases the monitor.
func (m *SimpleMonitor) Unlock() {
	m.lock.Unlock()
}

type CondWrap struct {
	*sync.Cond
}

func (c *CondWrap) Await() {
	c.Wait()
}
func (c *CondWrap) SignalAll() {
	c.Broadcast()
}

// NewCondition returns a condition variable bound to this monitor.
func (m *SimpleMonitor) NewCondition() prim.Condition {
	return &CondWrap{sync.NewCond(m.lock)}
}

var _ prim.Monitor = (*SimpleMonitor)(nil)
