package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/memory"
)

// beginChatSessionTurn chooses the run lifetime before registering or waiting.
// Sync work survives client disconnects, but never session Stop or shutdown.
func (s *Server) beginChatSessionTurn(ctx context.Context, sessionID string, stream bool, missionID string) (context.Context, func(), error) {
	if stream {
		return s.lockChatSessionTurn(ctx, sessionID)
	}
	owner, releaseOwner := missionRunBaseContext(s, missionID)
	if backgroundOwner, ok := ctx.Value(backgroundExecutionOwnerKey{}).(context.Context); ok {
		owner = backgroundOwner
	}
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
	stopOwner := context.AfterFunc(owner, cancel)
	stopLifetime := func() bool { return false }
	if s.integrationCtx != nil {
		stopLifetime = context.AfterFunc(s.integrationCtx, cancel)
	}
	s.httpDrainMu.Lock()
	if s.httpDrainCtx == nil {
		s.httpDrainCtx, s.httpDrainCancel = context.WithCancel(context.Background())
	}
	drainCtx, draining := s.httpDrainCtx, s.httpDraining
	s.httpDrainMu.Unlock()
	stopDrain := context.AfterFunc(drainCtx, cancel)
	if draining || drainCtx.Err() != nil || owner.Err() != nil || (s.integrationCtx != nil && s.integrationCtx.Err() != nil) {
		cancel()
	}
	cleanup := func() {
		cancel()
		stopOwner()
		stopLifetime()
		stopDrain()
		releaseOwner()
	}
	turnCtx, releaseTurn, err := s.lockChatSessionTurn(runCtx, sessionID)
	if err != nil {
		cleanup()
		return turnCtx, nil, err
	}
	var once sync.Once
	return turnCtx, func() { once.Do(func() { releaseTurn(); cleanup() }) }, nil
}

// lockChatSessionTurn shares the store's ingress gate with bots and loopback.
// Activity refs remain server telemetry only, never a second execution mutex.
func (s *Server) lockChatSessionTurn(ctx context.Context, sessionID string) (context.Context, func(), error) {
	start := time.Now()
	muSessionRequestLocks.Lock()
	state := sessionRequestLocks[sessionID]
	if state == nil {
		state = &sessionRequestLock{}
		sessionRequestLocks[sessionID] = state
	}
	state.refs++
	queued := state.refs > 1
	muSessionRequestLocks.Unlock()
	drop := func() {
		muSessionRequestLocks.Lock()
		state.refs--
		if state.refs == 0 && sessionID != "default" && sessionRequestLocks[sessionID] == state {
			delete(sessionRequestLocks, sessionID)
		}
		muSessionRequestLocks.Unlock()
	}
	var store *memory.SQLiteMemory
	if s != nil {
		store = s.ShortTermMem
	}
	turnCtx, release, err := store.BeginSessionTurn(ctx, sessionID)
	if err != nil {
		drop()
		return turnCtx, nil, err
	}
	if s != nil && s.Logger != nil {
		s.Logger.Debug("[SessionLock] Request acquired shared session turn", "session_id", sessionID, "wait_ms", time.Since(start).Milliseconds(), "queued", queued)
	}
	var once sync.Once
	return turnCtx, func() { once.Do(func() { release(); drop() }) }, nil
}
