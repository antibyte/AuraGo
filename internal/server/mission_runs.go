package server

import (
	"context"
	"sync"
)

// missionRunRegistry tracks the cancellable context of in-flight local
// mission runs. The sync /v1/chat/completions branch intentionally uses a
// detached context so a client disconnect cannot abort a tool chain; missions
// therefore need an explicit handle to be cancelled by the user.
type missionRunRegistry struct {
	mu     sync.Mutex
	runs   map[string]*missionRunEntry
	wg     sync.WaitGroup
	closed bool
}

type missionRunEntry struct {
	cancel    context.CancelFunc
	cancelled bool
}

func newMissionRunRegistry() *missionRunRegistry {
	return &missionRunRegistry{runs: make(map[string]*missionRunEntry)}
}

// begin registers a new run for missionID and returns its context plus a
// release function. A stale entry for the same mission is cancelled and
// replaced (not flagged as a user cancellation). release removes the entry
// unless it was cancelled by the user, so the completion callback can still
// classify the failure via consumeCancelled.
func (r *missionRunRegistry) begin(missionID string) (context.Context, func()) {
	return r.beginContext(context.Background(), missionID)
}

func (r *missionRunRegistry) beginContext(parent context.Context, missionID string) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	entry := &missionRunEntry{cancel: cancel}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		cancel()
		return ctx, func() {}
	}
	r.wg.Add(1)
	if previous, ok := r.runs[missionID]; ok && previous.cancel != nil {
		previous.cancel()
	}
	r.runs[missionID] = entry
	r.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			defer r.wg.Done()
			cancel()
			r.mu.Lock()
			defer r.mu.Unlock()
			if current, ok := r.runs[missionID]; ok && current == entry && !entry.cancelled {
				delete(r.runs, missionID)
			}
		})
	}
	return ctx, release
}

// cancel marks the active run of missionID as cancelled by the user and
// cancels its context. It returns false when no run is registered.
func (r *missionRunRegistry) cancel(missionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[missionID]
	if !ok || entry.cancelled {
		return false
	}
	entry.cancelled = true
	if entry.cancel != nil {
		entry.cancel()
	}
	return true
}

// consumeCancelled reports whether the last run of missionID was cancelled by
// the user and clears the flag (and the entry) so the next run starts clean.
func (r *missionRunRegistry) consumeCancelled(missionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.runs[missionID]
	if !ok || !entry.cancelled {
		return false
	}
	delete(r.runs, missionID)
	return true
}

// close cancels current and replaced generations, then drains their handlers.
func (r *missionRunRegistry) close() {
	r.mu.Lock()
	r.closed = true
	for _, entry := range r.runs {
		entry.cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}
