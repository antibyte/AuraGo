package agent

import (
	"context"
	"fmt"
)

// Context outlives the parent request and ends at terminal registry shutdown.
func (r *CoAgentRegistry) Context() context.Context { return r.ctx }

// releaseSlotLocked retains physical capacity until the worker has returned.
func (r *CoAgentRegistry) releaseSlotLocked(a *CoAgentInfo) {
	if !a.workerActive && a.slotHeld {
		a.slotHeld = false
		r.runningCount.Add(-1)
	}
}

// WorkerDone must be the outermost defer of every accepted worker.
func (r *CoAgentRegistry) WorkerDone(id string) {
	r.FailIfActive(id, "worker ended without a result")
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.agents[id]
	if a == nil || !a.workerActive {
		return
	}
	a.workerActive = false
	r.releaseSlotLocked(a)
	r.promoteQueuedLocked()
	r.workers.Done()
}

// Shutdown permanently refuses new work, cancels active and queued work and
// joins actual workers before callers may close their dependencies.
func (r *CoAgentRegistry) Shutdown() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.StopAll()
	r.cancel()
	r.StopCleanupLoop()
	r.workers.Wait()
}

func (r *CoAgentRegistry) owns(session, id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a := r.agents[id]
	return session != "" && a != nil && a.ownerSession == session
}

func (r *CoAgentRegistry) ListForSession(session string) []map[string]interface{} {
	return r.list(&session)
}

func (r *CoAgentRegistry) GetStatusForSession(session, id string) (map[string]interface{}, error) {
	if !r.owns(session, id) {
		return nil, fmt.Errorf("co-agent not found")
	}
	return r.GetStatus(id)
}

func (r *CoAgentRegistry) WaitForResultForSession(ctx context.Context, session, id string) (map[string]interface{}, error) {
	if !r.owns(session, id) {
		return nil, fmt.Errorf("co-agent not found")
	}
	return r.WaitForResult(ctx, id)
}

func (r *CoAgentRegistry) StopForSession(session, id string) error {
	if !r.owns(session, id) {
		return fmt.Errorf("co-agent not found")
	}
	return r.Stop(id)
}

func (r *CoAgentRegistry) StopAllForSession(session string) int {
	return r.stopAll(&session)
}
