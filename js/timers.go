package js

import (
	"sync"
	"time"

	"github.com/dop251/goja"
)

// timer represents a scheduled timer (setTimeout or setInterval).
type timer struct {
	id       int
	callback goja.Callable
	args     []goja.Value
	dueTime  time.Time
	interval time.Duration // 0 for setTimeout, >0 for setInterval
	cleared  bool
}

// timerManager manages setTimeout and setInterval timers.
type timerManager struct {
	timers  map[int]*timer
	nextID  int
	mu      sync.Mutex
}

// newTimerManager creates a new timer manager.
func newTimerManager() *timerManager {
	return &timerManager{
		timers: make(map[int]*timer),
		nextID: 1,
	}
}

// setTimeout schedules a one-time callback.
func (tm *timerManager) setTimeout(callback goja.Callable, delay time.Duration, args []goja.Value) int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := tm.nextID
	tm.nextID++

	tm.timers[id] = &timer{
		id:       id,
		callback: callback,
		args:     args,
		dueTime:  time.Now().Add(delay),
		interval: 0,
	}

	return id
}

// setInterval schedules a recurring callback.
func (tm *timerManager) setInterval(callback goja.Callable, interval time.Duration, args []goja.Value) int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := tm.nextID
	tm.nextID++

	tm.timers[id] = &timer{
		id:       id,
		callback: callback,
		args:     args,
		dueTime:  time.Now().Add(interval),
		interval: interval,
	}

	return id
}

// clearTimer clears a timer by ID.
func (tm *timerManager) clearTimer(id int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, ok := tm.timers[id]; ok {
		t.cleared = true
		delete(tm.timers, id)
	}
}

// process checks and executes any due timers.
func (tm *timerManager) process(r *Runtime) {
	tm.mu.Lock()
	now := time.Now()
	var dueTimers []*timer

	for _, t := range tm.timers {
		if !t.cleared && now.After(t.dueTime) || now.Equal(t.dueTime) {
			dueTimers = append(dueTimers, t)
		}
	}
	tm.mu.Unlock()

	// Execute due timers outside the lock
	for _, t := range dueTimers {
		if t.cleared {
			continue
		}

		// Execute the callback
		_, _ = t.callback(goja.Undefined(), t.args...)

		tm.mu.Lock()
		if t.interval > 0 && !t.cleared {
			// Reschedule interval timer
			t.dueTime = time.Now().Add(t.interval)
		} else {
			// Remove one-shot timer
			delete(tm.timers, t.id)
		}
		tm.mu.Unlock()
	}
}

// hasPending returns true if there are any pending timers.
func (tm *timerManager) hasPending() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.timers) > 0
}

// nextDueTime returns the time until the next timer is due.
// Returns 0 if no timers are pending, or if a timer is already due.
func (tm *timerManager) nextDueTime() time.Duration {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.timers) == 0 {
		return 0
	}

	now := time.Now()
	var minDuration time.Duration = -1

	for _, t := range tm.timers {
		if t.cleared {
			continue
		}
		d := t.dueTime.Sub(now)
		if d <= 0 {
			return 0
		}
		if minDuration < 0 || d < minDuration {
			minDuration = d
		}
	}

	if minDuration < 0 {
		return 0
	}
	return minDuration
}
