package flows

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestDurableExistingGenerationNeverCreatesAReplacementKey(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	keys := &checkpointTestKeys{}
	err := withCheckpointKeyMode(ctx, store.db, keys, false, func(tx *sql.Tx, key []byte) error {
		return authenticateOwnedCheckpointTx(ctx, tx, "missing_run", 1, "old_owner", time.Now(), key)
	})
	if err == nil || len(keys.requests) != 1 || keys.requests[0] {
		t.Fatalf("missing generation: err=%v key requests=%v", err, keys.requests)
	}
}

func TestDurableSecondWaitRejectsReplacedKey(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now().UTC()
	clock := newFakeClock(now)
	b := newFlow("Two durable waits")
	start := b.node("start", TypeTriggerManual, nil)
	first := b.node("first_wait", durableWaitType, map[string]any{"seconds": 1.0})
	between := b.node("between", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "preserved"}}, "keep_input": false})
	second := b.node("second_wait", durableWaitType, map[string]any{"seconds": 1.0})
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, between)
	b.edge(between, PortOut, second)
	doc := b.build()
	doc.ID = "flow_two_durable_waits"
	doc.Settings.MaxRunSeconds = 120
	if _, err := store.CreateFlow(ctx, doc, "mission_two_waits", now); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Publish(ctx, doc.ID, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions["mission_two_waits"] = &fakeMission{flowID: doc.ID, enabled: true}
	keys := &switchableCheckpointKeys{}
	services := &Services{CheckpointKeys: keys, Clock: clock, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	s.runner.mu.Lock()
	s.runner.durableStarted = true
	s.runner.mu.Unlock()
	started, err := s.runner.StartContext(ctx, StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", missionID: "mission_two_waits"})
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, s.runner, store, started.RunID)
	type saved struct {
		cipher     []byte
		generation int
		err        error
	}
	captured := make(chan saved, 1)
	s.runner.testOnEvent = func(ev RunEvent) {
		if ev.Type == EventStepFinished && ev.Step != nil && ev.Step.NodeKey == "between" {
			var v saved
			v.err = store.db.QueryRow(`SELECT ciphertext,generation FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&v.cipher, &v.generation)
			keys.wrong.Store(true)
			captured <- v
		}
	}
	clock.Advance(2 * time.Second)
	s.recoverDurableDue(ctx)
	var before saved
	select {
	case before = <-captured:
	case <-time.After(3 * time.Second):
		t.Fatal("between step not checkpointed")
	}
	if before.err != nil {
		t.Fatal(before.err)
	}
	until := time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	var after []byte
	var generation int
	var state string
	if err := store.db.QueryRow(`SELECT ciphertext,generation,state FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&after, &generation, &state); err != nil || generation != before.generation || state != "claimed" || !bytes.Equal(before.cipher, after) {
		t.Fatalf("replaced key overwrote checkpoint: generation=%d state=%s err=%v", generation, state, err)
	}
	header, steps, err := store.GetRun(ctx, started.RunID)
	if err != nil || header.Status.Terminal() {
		t.Fatalf("unexpected terminal run %+v %v", header, err)
	}
	for _, step := range steps {
		if step.NodeKey == "second_wait" {
			t.Fatalf("second wait persisted under replaced key: %+v", step)
		}
	}
	keys.wrong.Store(false)
	s.runner.testOnEvent = nil
	clock.Advance(durableLease + time.Second)
	s.recoverDurableDue(ctx)
	until = time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		header, steps, err = store.GetRun(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status == RunWaiting {
			for _, step := range steps {
				if step.NodeKey == "second_wait" && step.Status == StepWaiting {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("second wait did not recover with original key")
}

func TestDurableTimeoutReauthenticatesKeyInsideTerminalTransaction(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	doc, err := req.Flow.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	guard := durableClaimGuard{flowID: req.Flow.ID, missionID: req.missionID, revision: req.Revision, document: doc}
	keys := &switchableCheckpointKeys{}
	keys.wrong.Store(true)
	s.services.Clock.(*fakeClock).Advance(121 * time.Second)
	if err := s.store.expireDurableCheckpoint(ctx, runID, 1, s.now(), guard, keys); err == nil {
		t.Fatal("timeout accepted replaced key")
	}
	header, err := s.store.GetRunHeader(ctx, runID)
	if err != nil || header.Status != RunWaiting {
		t.Fatalf("run changed on wrong key: %+v %v", header, err)
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("checkpoint=%d err=%v", count, err)
	}
	keys.wrong.Store(false)
	if err := s.store.expireDurableCheckpoint(ctx, runID, 1, s.now(), guard, keys); err != nil {
		t.Fatal(err)
	}
	header, err = s.store.GetRunHeader(ctx, runID)
	if err != nil || header.Status != RunError || header.ErrorCode != "FLOW_RUN_TIMEOUT" {
		t.Fatalf("timeout result %+v %v", header, err)
	}
}
