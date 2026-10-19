package memory

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStopCancelsRunsWithoutDeletionFence(t *testing.T) {
	s, err := NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, release, err := s.RegisterSessionRun(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	unlock, err := s.LockSessionTurn(ctx, "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	queued, releaseQueued, err := s.RegisterSessionRun(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseQueued()
	done := make(chan error, 1)
	go func() {
		release, err := s.LockSessionTurn(queued, "stop-test")
		if release != nil {
			release()
		}
		done <- err
	}()
	s.CancelSessionRuns("stop-test")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("active run survived Stop: %v", ctx.Err())
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued turn survived Stop: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued turn remained blocked")
	}
	fresh, releaseFresh, err := s.RegisterSessionRun(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFresh()
	if fresh.Err() != nil {
		t.Fatal(fresh.Err())
	}
	if err := s.WithSessionWrite(fresh, "stop-test", func() error { return nil }); err != nil {
		t.Fatalf("Stop installed deletion fence: %v", err)
	}
}

func TestSessionTurnLockSerializesAndHonorsCancellation(t *testing.T) {
	s := &SQLiteMemory{}
	release, err := s.LockSessionTurn(context.Background(), "same")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan error, 1)
	go func() {
		unlock, err := s.LockSessionTurn(ctx, "same")
		if unlock != nil {
			unlock()
		}
		entered <- err
	}()
	select {
	case <-entered:
		t.Fatal("same session entered concurrently")
	case <-time.After(20 * time.Millisecond):
	}
	other, err := s.LockSessionTurn(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	other()
	cancel()
	if err := <-entered; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued error = %v", err)
	}
	release()
	release()
	reacquire, err := s.LockSessionTurn(context.Background(), "same")
	if err != nil {
		t.Fatal(err)
	}
	reacquire()
}
