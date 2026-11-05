package flows

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

type durableStatusBridge struct {
	*fakeBridge
	durableTestAuthority
	denied bool
}

func (*durableStatusBridge) ProjectDurableRun(context.Context, string) error { return nil }

func (b *durableStatusBridge) AuthorizeDurable(context.Context, string, string, int) error {
	if b.denied {
		return errors.New("revoked")
	}
	return nil
}

type durableStatusKeys struct {
	value       byte
	unavailable bool
}

func (k *durableStatusKeys) CheckpointKey(context.Context, bool) ([]byte, error) {
	if k.unavailable {
		return nil, errors.New("unavailable")
	}
	return bytes.Repeat([]byte{k.value}, 32), nil
}

func TestDurableStatusReportsAuthenticatedPerRunBlocks(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_status_review", 60)
	keys := &durableStatusKeys{value: 7}
	services := &Services{CheckpointKeys: keys, Location: time.UTC}
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{}, RunnerConfig{DurableEnabled: true, DurableAuthority: bridge, DurableHistory: &durableTestProjection{}}, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(ctx) })
	r.mu.Lock()
	r.durableStarted = true
	r.mu.Unlock()
	started, err := r.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, r, store, started.RunID)
	s := &Service{store: store, bridge: bridge, services: services, runner: r, cfg: ServiceConfig{DurableEnabled: true}}
	assert := func(wantState, wantCode string, waiting, blocked int) {
		t.Helper()
		status, err := s.DurableStatus(ctx)
		if err != nil || status.WaitingCount != waiting || status.BlockedCount != blocked || status.ClaimedCount != 0 {
			t.Fatalf("global status=%+v err=%v", status, err)
		}
		run, err := s.DurableRunStatus(ctx, started.RunID)
		if err != nil || run == nil || run.State != wantState || run.BlockedCode != wantCode || !run.CancelAvailable {
			t.Fatalf("run status=%+v err=%v", run, err)
		}
	}
	assert("waiting", "", 1, 0)
	keys.unavailable = true
	assert("blocked", "key_unavailable", 0, 1)
	keys.unavailable = false
	keys.value = 8
	assert("blocked", "checkpoint_unreadable", 0, 1)
	keys.value = 7
	bridge.denied = true
	assert("blocked", "authority", 0, 1)
	bridge.denied = false
	assert("waiting", "", 1, 0)
	var originalWake string
	if err := store.db.QueryRow(`SELECT wake_at FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&originalWake); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE flow_checkpoints SET wake_at=? WHERE run_id=?`, formatTime(time.Now()), started.RunID); err != nil {
		t.Fatal(err)
	}
	assert("blocked", "checkpoint_unreadable", 0, 1)
	if _, err := store.db.Exec(`UPDATE flow_checkpoints SET wake_at=? WHERE run_id=?`, originalWake, started.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE flow_runs SET trigger_type='api' WHERE id=?`, started.RunID); err != nil {
		t.Fatal(err)
	}
	assert("blocked", "checkpoint_unreadable", 0, 1)
	if _, err := store.db.Exec(`UPDATE flow_runs SET trigger_type='manual' WHERE id=?`, started.RunID); err != nil {
		t.Fatal(err)
	}

	// A run-detail read must not decode or classify another run's payload.
	other := RunRecord{ID: "run_status_oversize", FlowID: req.Flow.ID, Revision: req.Revision, Mode: ModeLive, Status: RunRunning, StartedAt: time.Now()}
	if err := store.CreateRun(ctx, other, nil); err != nil {
		t.Fatal(err)
	}
	markDurableTestRun(t, store, other.ID)
	checkpoint := durableCheckpointRecord{RunID: other.ID, Generation: 1, WakeAt: time.Now().Add(time.Minute), Deadline: time.Now().Add(time.Hour), Ciphertext: bytes.Repeat([]byte{3}, 1<<20)}
	if err := saveDurableCheckpoint(ctx, store.db, checkpoint, 0, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	run, err := s.DurableRunStatus(ctx, started.RunID)
	if err != nil || run == nil || run.State != "waiting" {
		t.Fatalf("unrelated checkpoint affected target: %+v %v", run, err)
	}
	status, err := s.DurableStatus(ctx)
	if err != nil || status.WaitingCount != 1 || status.BlockedCount != 1 {
		t.Fatalf("aggregate %+v %v", status, err)
	}
	// An already busy diagnostic never starts a second decode and is cancellable.
	r.durableDiagnostics <- struct{}{}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.DurableRunStatus(cancelled, started.RunID)
	<-r.durableDiagnostics
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting for diagnostics ignored cancellation: %v", err)
	}
}

func TestDurableHistoryAndRecoveryBindAdmissionMission(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_owner_review", 60)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	s.runner.mu.Lock()
	s.runner.durableStarted = true
	s.runner.mu.Unlock()
	started, err := s.runner.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, s.runner, store, started.RunID)
	if err := store.SetMissionID(ctx, req.Flow.ID, "mission_reassigned"); err != nil {
		t.Fatal(err)
	}
	projection, err := s.DurableProjection(ctx, started.RunID)
	if err != nil || projection.MissionID != req.missionID {
		t.Fatalf("projection owner %+v %v", projection, err)
	}
	status, err := s.DurableRunStatus(ctx, started.RunID)
	if err != nil || status == nil || status.State != "blocked" || status.BlockedCode != "authority" {
		t.Fatalf("reassigned owner should block recovery: %+v %v", status, err)
	}
}

func TestDurableUnownedRunWithoutCheckpointIsBlocked(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	doc := sampleFlow("flow_unowned_durable")
	if _, err := store.CreateFlow(ctx, doc, "mission_unowned", time.Now()); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_unowned_durable", FlowID: doc.ID, Revision: 1, Mode: ModeLive, Status: RunQueued, StartedAt: time.Now()}
	if err := store.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	markDurableTestRun(t, store, run.ID)
	s := NewService(store, catalogRegistry(t, fullEnv()), &Services{}, newFakeBridge(), ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	status, err := s.DurableRunStatus(ctx, run.ID)
	if err != nil || status == nil || status.State != "blocked" || status.BlockedCode != "storage_unavailable" || !status.CancelAvailable || status.Generation != 0 {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	global, err := s.DurableStatus(ctx)
	if err != nil || global.BlockedCount != 1 {
		t.Fatalf("global status=%+v err=%v", global, err)
	}
	if ok, err := s.CancelDurableRun(ctx, run.ID, 0); !ok || err != nil {
		t.Fatalf("cancel=%v err=%v", ok, err)
	}
}
