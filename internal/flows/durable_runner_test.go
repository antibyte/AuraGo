package flows

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type durableTestAuthority struct{}

func (durableTestAuthority) DurableEnabled(context.Context) (bool, error)                { return true, nil }
func (durableTestAuthority) DurableAllowed(context.Context) error                        { return nil }
func (durableTestAuthority) AuthorizeDurable(context.Context, string, string, int) error { return nil }

type durableTestProjection struct{ calls atomic.Int32 }

func (p *durableTestProjection) ProjectDurableRun(context.Context, string) error {
	p.calls.Add(1)
	return nil
}

func TestDurableMissionTriggerRejectsNonManualProvenance(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_trigger_review", 60)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	s.runner.mu.Lock()
	s.runner.durableStarted = true
	s.runner.mu.Unlock()
	for _, trigger := range []string{"api", "webhook", "mission_completed"} {
		if _, err := s.TriggerFromMission(req.missionID, "", trigger, map[string]any{"test": "value"}); !errors.Is(err, ErrDurableInvalidGraph) {
			t.Fatalf("trigger %q: %v", trigger, err)
		}
	}
	var runs, checkpoints int
	if err := store.db.QueryRow(`SELECT (SELECT COUNT(*) FROM flow_runs), (SELECT COUNT(*) FROM flow_checkpoints)`).Scan(&runs, &checkpoints); err != nil || runs != 0 || checkpoints != 0 {
		t.Fatalf("runs=%d checkpoints=%d err=%v", runs, checkpoints, err)
	}
}

func TestDurableRunnerSuspendsWithoutReleasingReservation(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	b := newFlow("Runner durable")
	start := b.node("start", TypeTriggerManual, nil)
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 60.0})
	b.edge(start, PortOut, wait)
	doc := b.build()
	doc.ID = "flow_durablerunner"
	if _, err := store.CreateFlow(ctx, doc, "mission_runner", time.Now()); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Publish(ctx, doc.ID, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	services := &Services{CheckpointKeys: &checkpointTestKeys{}, Location: time.UTC}
	projector := &durableTestProjection{}
	r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{}, RunnerConfig{DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: projector}, discardLogger())
	defer r.Shutdown(ctx)
	r.mu.Lock()
	r.durableStarted = true
	r.mu.Unlock()
	started, err := r.StartContext(ctx, StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", missionID: "mission_runner"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		header, err := store.GetRunHeader(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status == RunWaiting {
			if !header.Durable {
				t.Fatal("durable marker missing")
			}
			r.mu.Lock()
			slots, live := r.slots, r.live[doc.ID]
			r.mu.Unlock()
			if slots == 0 && live == 1 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not suspend and retain its flow reservation")
}

func markDurableTestRun(t *testing.T, store *Store, id string) {
	t.Helper()
	if _, err := store.db.Exec(`UPDATE flow_runs SET durable=1,durable_mission_id=(SELECT mission_id FROM flows WHERE id=flow_runs.flow_id) WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
}

func TestDurableAdmissionRevocationSurvivesReenableBeforeDispatch(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_revoked_queue", 60)
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{}, RunnerConfig{MaxParallelRuns: 1, DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: &durableTestProjection{}}, discardLogger())
	t.Cleanup(func() { _ = r.Shutdown(ctx) })
	r.mu.Lock()
	r.durableStarted = true
	r.slots = 1
	r.mu.Unlock()
	started, err := r.StartContext(ctx, req)
	if err != nil || started.Status != StartQueued {
		t.Fatalf("start=%+v err=%v", started, err)
	}
	if known, first := r.CancelRun(started.RunID); known || first {
		t.Fatal("generation-less cancel reached a durable run")
	}
	(&Service{runner: r}).InvalidateDurableProcessing()
	// A subsequent enable cannot undo the already delivered cancellation.
	r.mu.Lock()
	r.slots = 0
	r.dispatchLocked()
	r.mu.Unlock()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := store.GetRun(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunCancelled || len(steps) != 0 {
				t.Fatalf("run=%+v steps=%+v", header, steps)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("revoked queued run did not stop")
}

func TestDurableRunnerWakeCompletesSameRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_wake_review", 1)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	started, err := s.runner.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, s.runner, store, started.RunID)
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := store.GetRun(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess {
				t.Fatalf("run=%+v steps=%+v", header, steps)
			}
			var before, waitSuccess, after int
			for _, step := range steps {
				switch step.NodeKey {
				case "before":
					before++
				case "wait":
					if step.Status == StepSuccess {
						waitSuccess++
					}
				case "after":
					after++
				}
			}
			if before != 1 || waitSuccess != 1 || after != 1 {
				t.Fatalf("duplicate or missing steps: %+v", steps)
			}
			var remaining int
			if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatalf("checkpoint count=%d err=%v", remaining, err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("durable run did not resume")
}

func TestDurableDueRunKeepsTicketOrderWithOrdinaryQueue(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_fairness_review", 1)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true, MaxParallelRuns: 1}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	r := s.runner
	r.mu.Lock()
	r.durableStarted = true
	r.mu.Unlock()
	started, err := r.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, r, store, started.RunID)
	r.mu.Lock()
	r.slots = 1
	r.durableStarted = false
	r.mu.Unlock()
	s.startDurableScheduler()
	until := time.Now().Add(5 * time.Second)
	for {
		r.mu.Lock()
		queued := len(r.waiting) > 0 && r.waiting[0].rec.ID == started.RunID
		r.mu.Unlock()
		if queued {
			break
		}
		if time.Now().After(until) {
			t.Fatal("due durable run did not join queue")
		}
		time.Sleep(20 * time.Millisecond)
	}
	ordinary := simpleFlow("ordinary queue")
	ordinary.ID = "flow_fair_ordinary"
	if _, err := store.CreateFlow(ctx, ordinary, "mission_fair", time.Now()); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Publish(ctx, ordinary.ID, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var trigger string
	for _, n := range pub.Live.Nodes {
		if n.Type == TypeTriggerManual {
			trigger = n.ID
		}
	}
	other, err := r.StartContext(ctx, StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: trigger, TriggerType: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan string, 16)
	r.testOnEvent = func(ev RunEvent) {
		if ev.Type == EventStepFinished {
			select {
			case events <- ev.RunID:
			default:
			}
		}
	}
	r.mu.Lock()
	r.slots = 0
	r.dispatchLocked()
	r.mu.Unlock()
	select {
	case id := <-events:
		if id != started.RunID {
			t.Fatalf("later run %s overtook due durable run %s (ordinary %s)", id, started.RunID, other.RunID)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("queued runs did not start")
	}
}

func TestDurableExpiredRunRequiresAuthenticKeyBeforeTimeout(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_timeout_review", 1)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	clock := newFakeClock(time.Now().UTC())
	keys := &durableStatusKeys{value: 7}
	services := &Services{CheckpointKeys: keys, Clock: clock, Location: time.UTC}
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
	clock.Advance(121 * time.Second)
	keys.value = 8
	s.recoverDurableDue(ctx)
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		s.runner.mu.Lock()
		active := s.runner.slots
		s.runner.mu.Unlock()
		if active == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	header, err := store.GetRunHeader(ctx, started.RunID)
	if err != nil || header.Status != RunWaiting {
		t.Fatalf("wrong key changed run: %+v %v", header, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("checkpoint %d %v", count, err)
	}
	keys.value = 7
	s.recoverDurableDue(ctx)
	until = time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		header, err = store.GetRunHeader(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if header.Status != RunError || header.ErrorCode != "FLOW_RUN_TIMEOUT" {
		t.Fatalf("timeout %+v", header)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("checkpoint %d %v", count, err)
	}
}

func TestDurableCancelRequiresCurrentGeneration(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_cancel_review", 60)
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
	if n, err := s.CancelMissionRuns(ctx, req.missionID); n != 0 || !errors.Is(err, ErrDurableMissionCancel) {
		t.Fatalf("mission cancel %d %v", n, err)
	}
	if _, err := store.db.Exec(`UPDATE flow_runs SET durable=0 WHERE id=?`, started.RunID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CancelMissionRuns(ctx, req.missionID); n != 0 || !errors.Is(err, ErrDurableMissionCancel) {
		t.Fatalf("legacy checkpoint mission cancel %d %v", n, err)
	}
	markDurableTestRun(t, store, started.RunID)
	if ok, err := s.CancelDurableRun(ctx, started.RunID, 0); ok || !errors.Is(err, ErrDurableGenerationConflict) {
		t.Fatalf("stale cancel %v %v", ok, err)
	}
	header, err := store.GetRunHeader(ctx, started.RunID)
	if err != nil || header.Status != RunWaiting {
		t.Fatalf("stale cancel changed run %+v %v", header, err)
	}
	if ok, err := s.CancelDurableRun(ctx, started.RunID, 1); !ok || err != nil {
		t.Fatalf("current cancel %v %v", ok, err)
	}
	header, err = store.GetRunHeader(ctx, started.RunID)
	if err != nil || header.Status != RunCancelled {
		t.Fatalf("cancelled run %+v %v", header, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("checkpoint=%d err=%v", count, err)
	}
	if s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("cancel left flow reserved")
	}
}

func TestDeleteFlowReleasesSleepingDurableReservation(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_delete_durable", 60)
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
	if err := s.DeleteFlow(ctx, req.Flow.ID); err != nil {
		t.Fatal(err)
	}
	if s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("deleted flow retained a durable reservation")
	}
	var runs, checkpoints int
	if err := store.db.QueryRow(`SELECT (SELECT COUNT(*) FROM flow_runs),(SELECT COUNT(*) FROM flow_checkpoints)`).Scan(&runs, &checkpoints); err != nil || runs != 0 || checkpoints != 0 {
		t.Fatalf("runs=%d checkpoints=%d err=%v", runs, checkpoints, err)
	}
}

func TestDurableWakeKeepsAdmittedPublishedRevision(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	updated, err := req.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	updated.Name = "newer published revision"
	if _, err := s.store.SaveDraft(ctx, req.Flow.ID, updated, 1, s.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.Publish(ctx, req.Flow.ID, 2, s.now()); err != nil {
		t.Fatal(err)
	}
	s.recoverDurableDue(ctx)
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := s.store.GetRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess || header.Revision != 1 {
				t.Fatalf("run=%+v", header)
			}
			for _, step := range steps {
				if step.NodeKey == "after" && step.Status == StepSuccess {
					return
				}
			}
			t.Fatalf("v1 did not complete: %+v", steps)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("v1 did not resume after publishing v2")
}

func TestMalformedDurableCheckpointDoesNotBlockOtherFlows(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	if _, err := s.store.db.Exec(`UPDATE flow_checkpoints SET wake_at='damaged' WHERE run_id=?`, runID); err != nil {
		t.Fatal(err)
	}
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	other := NewService(s.store, catalogRegistry(t, fullEnv()), &Services{Location: time.UTC}, newFakeBridge(), ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = other.Shutdown(ctx) })
	if err := other.Start(ctx); err != nil {
		t.Fatalf("unrelated service failed to start: %v", err)
	}
	if !other.runner.IsBusy(req.Flow.ID) {
		t.Fatal("damaged checkpoint lost its reservation")
	}
	doc := simpleFlow("unrelated")
	doc.ID = "flow_unrelated_ok"
	if _, err := s.store.CreateFlow(ctx, doc, "mission_unrelated", time.Now()); err != nil {
		t.Fatal(err)
	}
	pub, err := s.store.Publish(ctx, doc.ID, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var trigger string
	for _, node := range pub.Live.Nodes {
		if node.Type == TypeTriggerManual {
			trigger = node.ID
		}
	}
	started, err := other.runner.StartContext(ctx, StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: trigger, TriggerType: "manual"})
	if err != nil || started.RunID == "" {
		t.Fatalf("unrelated start %+v %v", started, err)
	}
}

func TestDurableTerminalCommitFailureKeepsCheckpointUntilReclaim(t *testing.T) {
	s, _, runID := durableQueuedFixture(t)
	ctx := context.Background()
	if _, err := s.store.db.Exec(`CREATE TRIGGER test_terminal_failure BEFORE DELETE ON flow_checkpoints BEGIN SELECT RAISE(ABORT,'injected terminal failure'); END`); err != nil {
		t.Fatal(err)
	}
	s.recoverDurableDue(ctx)
	until := time.Now().Add(5 * time.Second)
	var generation int
	var state, status string
	for time.Now().Before(until) {
		err := s.store.db.QueryRow(`SELECT c.generation,c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, runID).Scan(&generation, &state, &status)
		s.runner.mu.Lock()
		active := s.runner.slots
		s.runner.mu.Unlock()
		if err == nil && generation > 1 && state == "claimed" && status == "running" && active == 0 && len(s.runner.durableSlots) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if generation <= 1 || state != "claimed" || status != "running" {
		t.Fatalf("failed commit lost checkpoint: gen=%d state=%s status=%s", generation, state, status)
	}
	if _, err := s.store.db.Exec(`DROP TRIGGER test_terminal_failure`); err != nil {
		t.Fatal(err)
	}
	s.services.Clock.(*fakeClock).Advance(durableLease + time.Second)
	s.recoverDurableDue(ctx)
	until = time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := s.store.GetRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess {
				t.Fatalf("terminal status %+v", header)
			}
			var after int
			for _, step := range steps {
				if step.NodeKey == "after" {
					after++
				}
			}
			if after != 1 {
				t.Fatalf("committed step repeated %d: %+v", after, steps)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("failed terminal commit was not safely reclaimed")
}
