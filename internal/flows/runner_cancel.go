package flows

import "context"

// Cancel stops a running run or removes a queued one. It returns false for unknown runs.
// A run whose Start has not returned yet is not known to Cancel.
func (r *Runner) Cancel(runID string) bool {
	r.mu.Lock()
	if run, ok := r.cancels[runID]; ok {
		run.cancelled = true
		r.cancels[runID] = run
		r.mu.Unlock()
		run.cancel()
		return true
	}
	p := r.removeQueuedLocked(runID)
	r.mu.Unlock()
	if p == nil {
		return false
	}
	r.finishUnstarted(p, "FLOW_CANCELLED", "the run was cancelled before it started")
	return true
}

func (r *Runner) removeQueuedLocked(runID string) *pendingRun {
	for flowID, q := range r.flowQueue {
		for i, p := range q {
			if p.rec.ID != runID {
				continue
			}
			rest := append(q[:i:i], q[i+1:]...)
			if len(rest) == 0 {
				delete(r.flowQueue, flowID)
			} else {
				r.flowQueue[flowID] = rest
			}
			return p
		}
	}
	for i, p := range r.waiting {
		if p.rec.ID != runID {
			continue
		}
		r.waiting = append(r.waiting[:i:i], r.waiting[i+1:]...)
		if p.counted {
			r.releaseFlowLocked(p.rec.FlowID)
		}
		return p
	}
	return nil
}

// CancelFlow cancels every run of the flow that the runner knows, test runs included,
// and returns how many runs this call cancelled; a running run that Cancel, CancelFlow
// or Shutdown cancelled before is not counted again while it winds down. Running runs
// are cancelled through their context, like Cancel does, and end in the background.
// Runs queued behind the flow's active run or waiting for a global slot end at once
// with FLOW_CANCELLED; OnRunFinished is called for them before CancelFlow returns,
// outside all locks.
//
// CancelFlow first waits for a Start that is writing its run record, like Shutdown, so
// every run whose Start returned before CancelFlow was called is cancelled. A Start that
// begins later is not affected. A caller that deletes the flow therefore calls CancelFlow
// again once the flow row is gone: from then on Start cannot record a run of the flow.
func (r *Runner) CancelFlow(flowID string) int {
	return r.cancelFlowRuns(flowID, func(RunMode) bool { return true })
}

// CancelFlowMode is CancelFlow for the flow's runs of one mode: it cancels the runs of
// that mode the runner knows (running, queued behind the flow's active run, waiting for a
// global slot, or with a Start in progress, which it waits for) and returns how many runs
// this call cancelled, the same way. Runs of other modes keep running and keep their
// places; when a cancelled run that waited for a global slot held the flow's slot, the
// flow's next queued run of another mode is admitted. Mission Control's cancel button
// uses it with ModeLive, so the editor's test runs go on.
func (r *Runner) CancelFlowMode(flowID string, mode RunMode) int {
	return r.cancelFlowRuns(flowID, func(m RunMode) bool { return m == mode })
}

// cancelFlowRuns cancels the flow's runs whose mode match accepts; see CancelFlow.
func (r *Runner) cancelFlowRuns(flowID string, match func(RunMode) bool) int {
	r.startMu.Lock()
	r.mu.Lock()
	var pending, keptQueue []*pendingRun
	for _, p := range r.flowQueue[flowID] {
		if match(p.rec.Mode) {
			pending = append(pending, p)
		} else {
			keptQueue = append(keptQueue, p)
		}
	}
	// A new slice: the old backing array must not keep the cancelled runs' trigger data.
	if len(keptQueue) == 0 {
		delete(r.flowQueue, flowID)
	} else {
		r.flowQueue[flowID] = keptQueue
	}
	released := 0
	kept := r.waiting[:0]
	for _, p := range r.waiting {
		if p.rec.FlowID != flowID || !match(p.rec.Mode) {
			kept = append(kept, p)
			continue
		}
		pending = append(pending, p)
		if p.counted {
			released++
		}
	}
	clear(r.waiting[len(kept):]) // the backing array must not keep the runs' trigger data
	r.waiting = kept
	// Each counted waiting run holds one of the flow's slots, so the flow becomes free with
	// the last release at the earliest. Only then is the next run left in the flow's queue
	// (one of another mode) admitted; with the whole queue cancelled nothing is.
	for ; released > 0; released-- {
		r.releaseFlowLocked(flowID)
	}
	var cancels []context.CancelFunc
	for id, run := range r.cancels {
		if run.flowID == flowID && !run.cancelled && match(run.mode) {
			run.cancelled = true
			r.cancels[id] = run
			cancels = append(cancels, run.cancel)
		}
	}
	r.mu.Unlock()
	r.startMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	for _, p := range pending {
		r.finishUnstarted(p, "FLOW_CANCELLED", "the run was cancelled before it started")
	}
	return len(cancels) + len(pending)
}
