package flows

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type retryDurableHistory struct {
	*fakeBridge
	calls  atomic.Int32
	signal chan string
}

func (b *retryDurableHistory) ProjectDurableRun(_ context.Context, id string) error {
	n := b.calls.Add(1)
	b.signal <- id
	if n == 1 {
		return errors.New("temporary history failure")
	}
	return nil
}

func TestDurableHistoryReconcilesRetryWhileProcessingOff(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	doc := sampleFlow("flow_history_retry")
	if _, err := store.CreateFlow(ctx, doc, "mission_history", time.Now()); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_history_retry", FlowID: doc.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, StartedAt: time.Now()}
	if err := store.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	markDurableTestRun(t, store, run.ID)
	if _, err := store.db.Exec(`UPDATE flow_runs SET status='success',finished_at=? WHERE id=?`, formatTime(time.Now()), run.ID); err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock(time.Now())
	bridge := &retryDurableHistory{fakeBridge: newFakeBridge(), signal: make(chan string, 4)}
	s := NewService(store, catalogRegistry(t, fullEnv()), &Services{Clock: clock}, bridge, ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-bridge.signal:
		if id != run.ID {
			t.Fatalf("wrong run %s", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("initial projection missing")
	}
	until := time.Now().Add(3 * time.Second)
	for bridge.calls.Load() < 2 && time.Now().Before(until) {
		clock.Advance(5 * time.Minute)
		time.Sleep(20 * time.Millisecond)
	}
	if bridge.calls.Load() < 2 {
		t.Fatal("failed history projection was not retried")
	}
	if len(bridge.startedRuns()) != 0 {
		t.Fatal("ordinary mission start hook was called")
	}
	header, err := store.GetRunHeader(ctx, run.ID)
	if err != nil || header.Status != RunSuccess {
		t.Fatalf("source changed: %+v %v", header, err)
	}
}
