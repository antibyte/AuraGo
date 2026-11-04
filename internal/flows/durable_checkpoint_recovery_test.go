package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestClaimedDurableStateMatchesRunIdentity(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := simpleFlow("Identity")
	f.ID = "flow_aaaaaaaaag"
	wait := Node{ID: testNodeID(3), Key: "wait", Type: durableWaitType, TypeVersion: 1}
	f.Nodes = append(f.Nodes, wait)
	f.Edges = append(f.Edges, Edge{ID: "e_2", Source: PortRef{Node: f.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: wait.ID, Port: PortIn}})
	if _, err := s.CreateFlow(ctx, f, "mission_7", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_g", FlowID: f.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, TriggerNode: f.Nodes[0].ID, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	trigger := map[string]any{"type": "manual", "node": "start", "fired_at": storeNow.Format(time.RFC3339), "data": map[string]any{}}
	triggerJSON, _ := json.Marshal(trigger)
	state := durableCheckpointState{Version: 1, RunID: run.ID, Flow: f, Revision: 1, TriggerNode: run.TriggerNode, Trigger: trigger,
		StartedAt: storeNow, WaitStartedAt: storeNow, WakeAt: storeNow.Add(time.Second), Deadline: storeNow.Add(time.Hour), WaitingNode: wait.ID,
		Statuses: map[string]StepStatus{f.Nodes[0].ID: StepSuccess, f.Nodes[1].ID: StepSuccess, wait.ID: StepWaiting},
		Edges:    map[string]edgeState{"e_1": edgeDelivered, "e_2": edgeDelivered}, Outputs: map[string]map[string]any{"start": trigger, "greet": {"greeting": "hello"}}, OutputBytes: len(triggerJSON) + 20}
	plain, err := encodeDurableState(state)
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	sealed, err := sealCheckpoint(key, run.ID, 1, plain)
	if err != nil {
		t.Fatal(err)
	}
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: state.WakeAt, Deadline: state.Deadline, Ciphertext: sealed}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if ok, err := claimDurableCheckpoint(ctx, s.db, run.ID, 1, "owner", state.WakeAt, state.Deadline); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	keys := &checkpointTestKeys{}
	if _, err := s.claimedDurableState(ctx, run.ID, 1, "owner", state.WakeAt, keys); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE flow_runs SET revision=2 WHERE id=?`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.claimedDurableState(ctx, run.ID, 1, "owner", state.WakeAt, keys); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("mismatched identity accepted: %v", err)
	}
	other, _ := f.Clone()
	other.ID = "flow_aaaaaaaaah"
	if _, err := s.CreateFlow(ctx, other, "mission_8", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE flow_runs SET revision=1,flow_id=? WHERE id=?`, other.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.claimedDurableState(ctx, run.ID, 1, "owner", state.WakeAt, keys); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("mismatched flow accepted: %v", err)
	}
}

func TestDurableReservationsAndAtomicCancel(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaae")
	if _, err := s.CreateFlow(ctx, f, "mission_5", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_e", FlowID: f.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: storeNow.Add(time.Second), Deadline: storeNow.Add(time.Hour), Ciphertext: []byte("sealed")}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); err != nil {
		t.Fatal(err)
	}
	reservations, err := s.durableReservations(ctx)
	if err != nil || len(reservations) != 1 || reservations[0].FlowID != f.ID {
		t.Fatalf("reservations %+v %v", reservations, err)
	}
	if _, err := s.db.Exec(`UPDATE flow_checkpoints SET state='claimed' WHERE run_id=?`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.durableReservations(ctx); err == nil {
		t.Fatal("inconsistent checkpoint released its reservation")
	}
	if _, err := s.db.Exec(`UPDATE flow_checkpoints SET state='waiting' WHERE run_id=?`, run.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := s.cancelDurableRunTx(ctx, tx, run.ID, storeNow.Add(2*time.Second), "FLOW_CANCELLED", "cancelled")
	if err != nil || !ok {
		_ = tx.Rollback()
		t.Fatalf("cancel %v %v", ok, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	header, err := s.GetRunHeader(ctx, run.ID)
	if err != nil || header.Status != RunCancelled {
		t.Fatalf("cancelled run %+v %v", header, err)
	}
	reservations, err = s.durableReservations(ctx)
	if err != nil || len(reservations) != 0 {
		t.Fatalf("cancelled reservation %+v %v", reservations, err)
	}
}

func TestDurableReservationsSurviveRestartAdmission(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaaf")
	f.Settings.Concurrency = ConcurrencySkip
	if _, err := s.CreateFlow(ctx, f, "mission_6", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_f", FlowID: f.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: storeNow.Add(time.Minute), Deadline: storeNow.Add(time.Hour), Ciphertext: []byte("sealed")}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if count, err := s.MarkInterruptedRuns(ctx, storeNow.Add(time.Minute), storeNow.Add(time.Minute)); err != nil || count != 0 {
		t.Fatalf("checkpointed run interrupted: %d %v", count, err)
	}
	r := NewRunner(newTestEngine(newTestRegistry(t), nil, 1), s, RunnerHooks{}, RunnerConfig{}, discardLogger())
	defer r.Shutdown(ctx)
	if !r.IsBusy(f.ID) {
		t.Fatal("checkpoint did not reserve its flow")
	}
	start, err := r.Start(StartRequest{Flow: f, Revision: 1, Mode: ModeLive, TriggerNode: f.Nodes[0].ID})
	if err != nil || start.Status != StartSkipped {
		t.Fatalf("exclusive admission = %+v %v", start, err)
	}
}
