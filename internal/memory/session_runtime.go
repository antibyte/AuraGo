package memory

import (
	"context"
	"errors"
	"fmt"
)

var ErrSessionDeleted = errors.New("chat session deleted")

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
