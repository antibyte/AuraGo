package flows

import (
	"errors"
	"sync"
)

var (
	// ErrRunsActive is returned by HoldDispatch while a run holds a slot, that is, while it is running (a
	// parked run that only waits gave its slot back).
	ErrRunsActive = errors.New("a flow run is running")
	// ErrDispatchHeld is returned when the dispatch is already on hold.
	ErrDispatchHeld = errors.New("flow runs are already on hold")
)

// HoldDispatch stops the runner from starting any run, new or queued or resuming, until release is
// called. It refuses while a run is running. The check and the hold happen under Runner.mu, so no run
// can start in between. Runs started during the hold are recorded as queued and start on release.
func (r *Runner) HoldDispatch() (release func(), err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.closed:
		return nil, ErrRunnerClosed
	case r.dispatchHeld:
		return nil, ErrDispatchHeld
	case r.slots > 0:
		return nil, ErrRunsActive
	}
	r.dispatchHeld = true
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.dispatchHeld = false
			r.dispatchLocked()
			r.mu.Unlock()
		})
	}, nil
}
