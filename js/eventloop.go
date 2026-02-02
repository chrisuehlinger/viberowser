package js

import (
	"sync"

	"github.com/dop251/goja"
)

// task represents a queued callback in the event loop.
type task struct {
	callback goja.Callable
	args     []goja.Value
	goFunc   func() // Optional Go function to run
}

// eventLoop manages the JavaScript event loop for microtasks and macrotasks.
type eventLoop struct {
	microtasks []task
	macrotasks []task
	mu         sync.Mutex
}

// newEventLoop creates a new event loop.
func newEventLoop() *eventLoop {
	return &eventLoop{
		microtasks: make([]task, 0),
		macrotasks: make([]task, 0),
	}
}

// queueMicrotask adds a microtask to the queue.
// Microtasks are executed before the next macrotask.
func (el *eventLoop) queueMicrotask(callback goja.Callable, args []goja.Value) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.microtasks = append(el.microtasks, task{callback: callback, args: args})
}

// queueMacrotask adds a macrotask to the queue.
// Macrotasks are executed after all microtasks are complete.
func (el *eventLoop) queueMacrotask(callback goja.Callable, args []goja.Value) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.macrotasks = append(el.macrotasks, task{callback: callback, args: args})
}

// queueGoFunc adds a plain Go function as a macrotask.
// This is useful for internal async operations like XHR.
func (el *eventLoop) queueGoFunc(fn func()) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.macrotasks = append(el.macrotasks, task{goFunc: fn})
}

// runOnce processes one iteration of the event loop.
// It drains all microtasks, then executes one macrotask.
// Returns true if there are more events to process.
func (el *eventLoop) runOnce(r *Runtime) bool {
	// First, drain all microtasks
	for {
		el.mu.Lock()
		if len(el.microtasks) == 0 {
			el.mu.Unlock()
			break
		}
		t := el.microtasks[0]
		el.microtasks = el.microtasks[1:]
		el.mu.Unlock()

		// Execute the microtask
		_, _ = t.callback(goja.Undefined(), t.args...)
	}

	// Process timers
	r.timers.process(r)

	// Then execute one macrotask if available
	el.mu.Lock()
	if len(el.macrotasks) > 0 {
		t := el.macrotasks[0]
		el.macrotasks = el.macrotasks[1:]
		el.mu.Unlock()

		// Execute the macrotask
		if t.goFunc != nil {
			t.goFunc()
		} else if t.callback != nil {
			_, _ = t.callback(goja.Undefined(), t.args...)
		}
		return true
	}
	el.mu.Unlock()

	return el.hasPending() || r.timers.hasPending()
}

// hasPending returns true if there are any pending tasks.
func (el *eventLoop) hasPending() bool {
	el.mu.Lock()
	defer el.mu.Unlock()
	return len(el.microtasks) > 0 || len(el.macrotasks) > 0
}

// clear removes all pending tasks.
func (el *eventLoop) clear() {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.microtasks = el.microtasks[:0]
	el.macrotasks = el.macrotasks[:0]
}
