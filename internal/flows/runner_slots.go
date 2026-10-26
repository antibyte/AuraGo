package flows

import "context"

// RunSlot is a run's global slot as the engine sees it (RunRequest.Slot). Release gives it back
// while the run only waits and reports false when the runner keeps it (MaxParkedRuns reached, or
// shutting down). Acquire takes a slot again, in turn behind the runs and parked runs that asked
// before, and returns ctx's error when ctx ends first.
type RunSlot interface {
	Release() bool
	Acquire(ctx context.Context) error
}

// runSlot is the RunSlot of one launched run. held and the runner counters it changes are
// guarded by Runner.mu.
//
// held is false while the run is parked. The invariant Runner.slots == len(Runner.cancels)
// therefore holds only without parked runs: a parked run is in cancels but holds no slot
// (Runner.parked counts them; slots + parked == len(cancels)).
type runSlot struct {
	r       *Runner
	flowID  string
	counted bool
	held    bool
	// ended is set by Runner.release: the run is over, so an abandoned node that still asks
	// for a slot must not get one.
	ended bool
}

// resumeWait is a parked run waiting for a global slot again; ticket orders it with the waiting runs.
type resumeWait struct {
	ticket  uint64
	slot    *runSlot
	ready   chan struct{}
	granted bool
}

// Release gives the slot back (A7) and hands it out at once (dispatchLocked).
func (s *runSlot) Release() bool {
	r := s.r
	r.mu.Lock()
	defer r.mu.Unlock()
	if !s.held || s.ended || r.closed || r.parked >= r.cfg.MaxParkedRuns {
		return false
	}
	s.held = false
	r.parked++
	r.slots--
	if s.counted {
		r.dropFlowSlotLocked(s.flowID)
	}
	r.dispatchLocked()
	return true
}

// Acquire waits for a slot. A grant that races with ctx's end still counts (the slot is held and
// Runner.release gives it back).
func (s *runSlot) Acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err // already over: do not queue
	}
	r := s.r
	r.mu.Lock()
	if s.held {
		r.mu.Unlock()
		return nil
	}
	if s.ended {
		r.mu.Unlock()
		return ErrRunnerClosed
	}
	r.nextTicket++
	w := &resumeWait{ticket: r.nextTicket, slot: s, ready: make(chan struct{})}
	r.resumers = append(r.resumers, w)
	r.dispatchLocked()
	r.mu.Unlock()
	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if w.granted {
		return nil
	}
	for i, x := range r.resumers {
		if x == w {
			r.resumers = append(r.resumers[:i:i], r.resumers[i+1:]...)
			break
		}
	}
	return ctx.Err()
}

// grantLocked gives a parked run its global slot back and wakes its Acquire.
func (r *Runner) grantLocked(w *resumeWait) {
	w.granted = true
	w.slot.held = true
	r.parked--
	r.slots++
	if w.slot.counted {
		r.flowSlots[w.slot.flowID]++
	}
	close(w.ready)
}

// nextResumerLocked returns the index of the first parked run that may take a slot, or -1.
func (r *Runner) nextResumerLocked() int {
	for i, w := range r.resumers {
		if r.fitsLocked(w.slot.flowID, w.slot.counted) {
			return i
		}
	}
	return -1
}

// endSlotLocked is called when the run is over (Runner.release): it forgets a pending Acquire of an
// abandoned node, so the slot cannot be granted to a run that no longer exists. It reports
// whether the run was parked, that is, held no slot.
func (r *Runner) endSlotLocked(s *runSlot) (parked bool) {
	s.ended = true
	kept := r.resumers[:0]
	for _, w := range r.resumers {
		if w.slot != s {
			kept = append(kept, w)
		}
	}
	clear(r.resumers[len(kept):])
	r.resumers = kept
	return !s.held
}
