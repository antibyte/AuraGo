package memory

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionDeletionCancelsBeforeWaitingForWriterAndPersistsFence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	s, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.CreatePlan("private", "private plan", "", "", 2, []PlanTaskInput{{Title: "running"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, release, err := s.RegisterSessionRun(context.Background(), "private")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	entered, writerDone := make(chan struct{}), make(chan error, 1)
	go func() {
		writerDone <- s.WithSessionWrite(ctx, "private", func() error { close(entered); <-ctx.Done(); return ctx.Err() })
	}()
	<-entered
	deleted := make(chan error, 1)
	go func() { deleted <- s.PurgeChatSession("private") }()
	select {
	case err := <-writerDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deletion waited on writer without canceling run")
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM plans WHERE id=?`, plan.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted plan retained: %d %v", count, err)
	}
	if _, err := s.db.Exec(`INSERT INTO plan_events(plan_id,event_type,message) VALUES (?,'late','late')`, plan.ID); err == nil {
		t.Fatal("orphan plan event accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	called := false
	err = s.WithSessionWrites(context.Background(), []string{"survivor", "private"}, func() error { called = true; return nil })
	if !errors.Is(err, ErrSessionDeleted) || called {
		t.Fatalf("late KG batch accepted: %v %v", called, err)
	}
	if _, release, err := s.RegisterSessionRun(context.Background(), "private"); !errors.Is(err, ErrSessionDeleted) {
		if release != nil {
			release()
		}
		t.Fatalf("deleted session started: %v", err)
	}
}

func TestFailedSessionDeletionCancelsAllRunsButAllowsRetry(t *testing.T) {
	s := newTestPlansDB(t)
	if _, err := s.InsertMessage("retry", "user", "retained", false, false); err != nil {
		t.Fatal(err)
	}
	var runs []context.Context
	for i := 0; i < 2; i++ {
		ctx, release, err := s.RegisterSessionRun(context.Background(), "retry")
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		runs = append(runs, ctx)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_purge BEFORE DELETE ON messages BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.PurgeChatSession("retry"); err == nil {
		t.Fatal("purge unexpectedly succeeded")
	}
	for _, ctx := range runs {
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("run not canceled")
		}
	}
	if deleted, err := s.IsSessionDeleted("retry"); err != nil || deleted {
		t.Fatalf("failed purge persisted tombstone: %v %v", deleted, err)
	}
	_, release, err := s.RegisterSessionRun(context.Background(), "retry")
	if err != nil {
		t.Fatalf("failed purge permanently fenced session: %v", err)
	}
	release()
	s.sessionRuntimeMu.Lock()
	n := len(s.closingSessions)
	s.sessionRuntimeMu.Unlock()
	if n != 0 {
		t.Fatal("closing registry leaked")
	}
}

func TestFreshUserIngressAdvancesDeletedFixedSessionAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	s, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	base := "default"
	first, err := s.ResolveUserSession(context.Background(), base)
	if err != nil || first != base {
		t.Fatalf("changed existing channel: %q %v", first, err)
	}
	if err := s.PurgeChatSession(first); err != nil {
		t.Fatal(err)
	}
	next, err := s.ResolveUserSession(context.Background(), base)
	if err != nil || next == first {
		t.Fatalf("fresh user input still blocked: %q %v", next, err)
	}
	if session, err := s.GetChatSession(next); err != nil || session == nil {
		t.Fatalf("fresh default generation hidden from history: %v %v", session, err)
	}
	if _, err := s.InsertMessage(next, "user", "fresh input", false, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.ResolveUserSession(context.Background(), base)
	if err != nil || got != next {
		t.Fatalf("mapping did not persist: %q %v", got, err)
	}
	called := false
	if err := s.WithSessionWrite(context.Background(), first, func() error { called = true; return nil }); !errors.Is(err, ErrSessionDeleted) || called {
		t.Fatalf("old delayed writer revived: %v %v", called, err)
	}
	if err := s.PurgeChatSession(next); err != nil {
		t.Fatal(err)
	}
	third, err := s.ResolveUserSession(context.Background(), base)
	if err != nil || third == next || third == first {
		t.Fatalf("second deletion reused session: %q %v", third, err)
	}
}
