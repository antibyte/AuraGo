package flows

import (
	"context"
	"runtime/debug"
)

const DurableRuntimeUnavailableCode = "runtime_unavailable"

func (r *Runner) durableRuntimeFaulted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.durableFaulted
}

// faultDurableRuntime is process-local and fail-closed. The panic value is
// intentionally omitted because it can contain flow or checkpoint data.
func (r *Runner) faultDurableRuntime(component string, runID ...string) {
	r.mu.Lock()
	if r.durableFaulted {
		r.mu.Unlock()
		return
	}
	r.durableFaulted = true
	r.mu.Unlock()

	r.durableRuntimeStop()
	r.blockAdmissionTokens.cancelMatching(func(a *blockAdmission) bool { return a.durable })
	r.durableActive.cancelMatching(func(*blockAdmission) bool { return true })
	r.invalidateQueuedDurableRuns()
	attrs := []any{"component", component, "stack", string(debug.Stack())}
	if len(runID) > 0 {
		attrs = append(attrs, "run", runID[0])
	}
	r.logger.Error("durable runtime faulted; restart required", attrs...)
}

func (r *Runner) invalidateQueuedDurableRuns() {
	r.startMu.Lock()
	startMuLocked := true
	defer func() {
		if startMuLocked {
			r.startMu.Unlock()
		}
	}()
	var unstarted, resumed []*pendingRun
	decrementLive := make(map[string]int)
	func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for flowID, queue := range r.flowQueue {
			kept := queue[:0]
			for _, p := range queue {
				if !p.rec.Durable {
					kept = append(kept, p)
					continue
				}
				if p.durableResume == nil {
					unstarted = append(unstarted, p)
				} else {
					resumed = append(resumed, p)
				}
			}
			for i := len(kept); i < len(queue); i++ {
				queue[i] = nil
			}
			if len(kept) == 0 {
				delete(r.flowQueue, flowID)
			} else {
				r.flowQueue[flowID] = kept
			}
		}
		waiting := r.waiting
		kept := waiting[:0]
		for _, p := range waiting {
			if !p.rec.Durable {
				kept = append(kept, p)
				continue
			}
			if p.durableResume == nil {
				unstarted = append(unstarted, p)
				if p.counted {
					decrementLive[p.rec.FlowID]++
				}
			} else {
				resumed = append(resumed, p)
			}
		}
		for i := len(kept); i < len(waiting); i++ {
			waiting[i] = nil
		}
		r.waiting = kept
		for flowID, count := range decrementLive {
			remaining := r.live[flowID] - count
			if remaining > 0 {
				r.live[flowID] = remaining
				continue
			}
			delete(r.live, flowID)
			if !r.closed {
				if queue := r.flowQueue[flowID]; len(queue) > 0 {
					next := queue[0]
					queue[0] = nil
					if len(queue) == 1 {
						delete(r.flowQueue, flowID)
					} else {
						r.flowQueue[flowID] = queue[1:]
					}
					r.admitLocked(next)
				}
			}
		}
		r.dispatchLocked()
	}()
	r.startMu.Unlock()
	startMuLocked = false
	for _, p := range resumed {
		r.releaseDurableToken(p)
	}
	for _, p := range unstarted {
		r.finishUnstarted(p, "FLOW_CANCELLED", "durable processing is unavailable")
	}
}

func (s *Service) projectDurableHistory(ctx context.Context, runID string) (err error) {
	return s.runner.callDurableHistoryProjector(ctx, runID, "history_reconcile_projector")
}

func (r *Runner) callDurableHistoryProjector(ctx context.Context, runID, component string) (err error) {
	if r.durableRuntimeFaulted() || r.cfg.DurableHistory == nil {
		return ErrDurableUnavailable
	}
	defer func() {
		if recover() != nil {
			r.faultDurableRuntime(component, runID)
			err = ErrDurableUnavailable
		}
	}()
	return r.cfg.DurableHistory.ProjectDurableRun(ctx, runID)
}
