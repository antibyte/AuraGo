package server

import (
	"aurago/internal/memory"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestServerIngressSharesMemoryTurnGateAndCancelsQueue(t *testing.T) {
	store, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := &Server{ShortTermMem: store}
	const id = "shared-server-and-bot"
	releaseExternal, err := store.LockSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseExternal()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, release, err := s.lockChatSessionTurn(ctx, id)
		if release != nil {
			release()
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("server bypassed another channel's gate: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if !sessionRequestActive(id) {
		t.Fatal("queued ingress missing activity marker")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queue error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled server waiter remained blocked")
	}
	if sessionRequestActive(id) {
		t.Fatal("canceled queue leaked activity marker")
	}
	releaseExternal()
	_, release, err := s.lockChatSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !sessionRequestActive(id) {
		t.Fatal("active ingress missing marker")
	}
	release()
	release()
	if sessionRequestActive(id) {
		t.Fatal("release leaked activity marker")
	}
}

func newChatTurnLifetimeTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &Server{ShortTermMem: store}
}

func TestSyncChatTurnStopInPreExecutionGapCannotStartFreshRun(t *testing.T) {
	s := newChatTurnLifetimeTestServer(t)
	const id = "sync-pre-execute-stop"
	ctx, release, err := s.beginChatSessionTurn(context.Background(), id, false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	// Stop after ingress/history ownership, before ExecuteAgentLoop registers
	// its child. That child must inherit cancellation rather than resurrect work.
	s.ShortTermMem.CancelSessionRuns(id)
	child, releaseChild, err := s.ShortTermMem.RegisterSessionRun(ctx, id)
	if releaseChild != nil {
		defer releaseChild()
	}
	if !errors.Is(err, context.Canceled) || !errors.Is(child.Err(), context.Canceled) {
		t.Fatalf("a new execution escaped the stopped ingress: context=%v registration=%v", child.Err(), err)
	}
}

func TestSyncChatTurnQueuedStopSurvivesClientDetach(t *testing.T) {
	s := newChatTurnLifetimeTestServer(t)
	const id = "sync-queued-stop"
	releaseOtherChannel, err := s.ShortTermMem.LockSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOtherChannel()
	requestCtx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	done := make(chan error, 1)
	go func() {
		_, release, err := s.beginChatSessionTurn(requestCtx, id, false, "")
		if release != nil {
			release()
		}
		done <- err
	}()
	deadline := time.After(time.Second)
	for !sessionRequestActive(id) {
		select {
		case <-deadline:
			t.Fatal("sync ingress did not enter the shared turn queue")
		case <-time.After(time.Millisecond):
		}
	}
	disconnect()
	select {
	case err := <-done:
		t.Fatalf("client disconnect aborted detached queued work: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	s.ShortTermMem.CancelSessionRuns(id)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop did not cancel detached queued work: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("detached queued turn survived Stop")
	}
	if sessionRequestActive(id) {
		t.Fatal("cancelled detached queue leaked its activity marker")
	}
}

func TestSyncChatTurnDisconnectPreservesContextValuesAndPersistence(t *testing.T) {
	s := newChatTurnLifetimeTestServer(t)
	const id = "sync-detached-persistence"
	type valueKey struct{}
	requestCtx, disconnect := context.WithCancel(context.WithValue(context.Background(), valueKey{}, "request-value"))
	defer disconnect()
	ctx, release, err := s.beginChatSessionTurn(requestCtx, id, false, "")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	disconnect()
	if ctx.Err() != nil || ctx.Value(valueKey{}) != "request-value" {
		t.Fatalf("detached run lost its lifetime or request values: %v %v", ctx.Err(), ctx.Value(valueKey{}))
	}
	child, releaseChild, err := s.ShortTermMem.RegisterSessionRun(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseChild()
	if err := s.ShortTermMem.WithSessionWrite(child, id, func() error {
		_, err := s.ShortTermMem.InsertMessage(id, "assistant", "detached completion", false, false)
		return err
	}); err != nil {
		t.Fatalf("client disconnect prevented detached persistence: %v", err)
	}
	messages, err := s.ShortTermMem.GetRecentMessages(id, 10)
	if err != nil || len(messages) != 1 || messages[0].Content != "detached completion" {
		t.Fatalf("detached completion was not persisted: %+v %v", messages, err)
	}
}

func TestSyncChatTurnRetainsShutdownAndExplicitOwners(t *testing.T) {
	for _, kind := range []string{"drain", "lifetime", "mission", "background"} {
		t.Run(kind, func(t *testing.T) {
			s := newChatTurnLifetimeTestServer(t)
			owner, cancelOwner := context.WithCancel(context.Background())
			defer cancelOwner()
			requestCtx := context.Background()
			missionID := ""
			if kind == "lifetime" {
				s.integrationCtx = owner
			}
			if kind == "background" {
				requestCtx = context.WithValue(requestCtx, backgroundExecutionOwnerKey{}, owner)
			}
			if kind == "mission" {
				missionID = "sync-owned-mission"
			}
			ctx, release, err := s.beginChatSessionTurn(requestCtx, "sync-owner-"+kind, false, missionID)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			switch kind {
			case "drain":
				s.beginHTTPDrain()
			case "mission":
				if !s.missionRunTracker().cancel(missionID) {
					t.Fatal("mission owner was not registered before turn execution")
				}
			default:
				cancelOwner()
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("detached turn lost its shutdown/owner cancellation")
			}
		})
	}
}
