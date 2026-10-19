package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrSessionDeleted = errors.New("chat session deleted")

// CancelSessionRuns stops active work without fencing or deleting conversation data.
// A later human turn can register normally in the same conversation.
func (s *SQLiteMemory) CancelSessionRuns(sessionID string) {
	s.sessionRuntimeMu.Lock()
	defer s.sessionRuntimeMu.Unlock()
	for _, cancel := range s.sessionRuns[sessionID] {
		cancel()
	}
}

type sessionTurnKey struct {
	store *SQLiteMemory
	id    string
}

type sessionTurnGate struct {
	token chan struct{}
	refs  int
}

var sessionTurnGates = struct {
	sync.Mutex
	entries map[sessionTurnKey]*sessionTurnGate
}{entries: make(map[sessionTurnKey]*sessionTurnGate)}

// LockSessionTurn serializes history construction through final publication.
// Register the run before waiting so Stop also cancels queued turns.
func (s *SQLiteMemory) LockSessionTurn(ctx context.Context, sessionID string) (func(), error) {
	key := sessionTurnKey{s, sessionID}
	sessionTurnGates.Lock()
	gate := sessionTurnGates.entries[key]
	if gate == nil {
		gate = &sessionTurnGate{token: make(chan struct{}, 1)}
		sessionTurnGates.entries[key] = gate
	}
	gate.refs++
	sessionTurnGates.Unlock()
	drop := func() {
		sessionTurnGates.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(sessionTurnGates.entries, key)
		}
		sessionTurnGates.Unlock()
	}
	select {
	case gate.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-gate.token
			drop()
			return nil, err
		}
		var once sync.Once
		return func() { once.Do(func() { <-gate.token; drop() }) }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

// BeginSessionTurn registers cancellation before waiting for the shared history
// gate. Call at ingress, before reading or inserting messages, and release only
// after durable publication and delivery. The agent executor must not relock it.
func (s *SQLiteMemory) BeginSessionTurn(ctx context.Context, sessionID string) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	releaseRun := func() {}
	if s != nil {
		var err error
		ctx, releaseRun, err = s.RegisterSessionRun(ctx, sessionID)
		if err != nil {
			return ctx, nil, err
		}
	}
	releaseTurn, err := s.LockSessionTurn(ctx, sessionID)
	if err != nil {
		releaseRun()
		return ctx, nil, err
	}
	var once sync.Once
	return ctx, func() { once.Do(func() { releaseTurn(); releaseRun() }) }, nil
}

// RegisterSessionRun makes deletion cancel every active run before removing its plan.
func (s *SQLiteMemory) RegisterSessionRun(ctx context.Context, sessionID string) (context.Context, func(), error) {
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return ctx, nil, err
	}
	defer s.sessionWriteMu.Unlock()
	if err := s.checkSessionWritable(ctx, sessionID); err != nil {
		return ctx, nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.sessionRuntimeMu.Lock()
	if s.closingSessions[sessionID] > 0 {
		s.sessionRuntimeMu.Unlock()
		cancel()
		return ctx, nil, ErrSessionDeleted
	}
	if s.sessionRuns == nil {
		s.sessionRuns = make(map[string]map[uint64]context.CancelFunc)
	}
	if s.sessionRuns[sessionID] == nil {
		s.sessionRuns[sessionID] = make(map[uint64]context.CancelFunc)
	}
	s.nextSessionRun++
	id := s.nextSessionRun
	s.sessionRuns[sessionID][id] = cancel
	s.sessionRuntimeMu.Unlock()
	release := func() {
		cancel()
		s.sessionRuntimeMu.Lock()
		delete(s.sessionRuns[sessionID], id)
		if len(s.sessionRuns[sessionID]) == 0 {
			delete(s.sessionRuns, sessionID)
		}
		s.sessionRuntimeMu.Unlock()
	}
	return runCtx, release, nil
}

// WithSessionWrite fences the complete durable publication, including KG writes.
// Call it before acquiring automaticMemoryMu; deletion takes locks in that order.
func (s *SQLiteMemory) WithSessionWrite(ctx context.Context, sessionID string, write func() error) error {
	return s.WithSessionWrites(ctx, []string{sessionID}, write)
}

// LockSessionWrite protects a tool mutation until its caller releases the fence.
func (s *SQLiteMemory) LockSessionWrite(ctx context.Context, sessionID string) (func(), error) {
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return nil, err
	}
	if err := s.checkSessionWritable(ctx, sessionID); err != nil {
		s.sessionWriteMu.Unlock()
		return nil, err
	}
	return s.sessionWriteMu.Unlock, nil
}

// WithSessionWrites publishes one extraction batch only while every source survives.
func (s *SQLiteMemory) WithSessionWrites(ctx context.Context, sessionIDs []string, write func() error) error {
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return err
	}
	defer s.sessionWriteMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, sessionID := range sessionIDs {
		if err := s.checkSessionWritable(ctx, sessionID); err != nil {
			return err
		}
	}
	return write()
}

func (s *SQLiteMemory) checkSessionWritable(ctx context.Context, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.sessionRuntimeMu.Lock()
	closing := s.closingSessions[sessionID] > 0
	s.sessionRuntimeMu.Unlock()
	if closing {
		return ErrSessionDeleted
	}
	deleted, err := s.IsSessionDeleted(sessionID)
	if err != nil {
		return fmt.Errorf("check session write fence: %w", err)
	}
	if deleted {
		return ErrSessionDeleted
	}
	return nil
}

func (s *SQLiteMemory) cancelSessionRuns(sessionID string) {
	s.sessionRuntimeMu.Lock()
	if s.closingSessions == nil {
		s.closingSessions = make(map[string]int)
	}
	s.closingSessions[sessionID]++
	var cancels []context.CancelFunc
	for _, cancel := range s.sessionRuns[sessionID] {
		cancels = append(cancels, cancel)
	}
	s.sessionRuntimeMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *SQLiteMemory) finishSessionDeletion(sessionID string) {
	s.sessionRuntimeMu.Lock()
	s.closingSessions[sessionID]--
	if s.closingSessions[sessionID] <= 0 {
		delete(s.closingSessions, sessionID)
	}
	s.sessionRuntimeMu.Unlock()
}
