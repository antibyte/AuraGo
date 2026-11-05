package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type checkpointTestKeys struct {
	requests []bool
	last     []byte
}

func (p *checkpointTestKeys) CheckpointKey(_ context.Context, create bool) ([]byte, error) {
	p.requests = append(p.requests, create)
	p.last = bytes.Repeat([]byte{7}, 32)
	return p.last, nil
}

func TestCheckpointKeyAdmissionUsesEmptyProof(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaac")
	if _, err := s.CreateFlow(ctx, f, "mission_3", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_c", FlowID: f.ID, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	keys := &checkpointTestKeys{}
	err := withCheckpointKey(ctx, s.db, keys, func(tx *sql.Tx, key []byte) error {
		sealed, err := sealCheckpoint(key, run.ID, 1, []byte("private"))
		if err != nil {
			return err
		}
		return saveDurableCheckpointTx(ctx, tx, durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: storeNow.Add(time.Second), Deadline: storeNow.Add(time.Hour), Ciphertext: sealed}, 0, "", storeNow)
	})
	if err != nil || len(keys.requests) != 1 || !keys.requests[0] || !bytes.Equal(keys.last, make([]byte, 32)) {
		t.Fatalf("initial admission: %v, requests=%v", err, keys.requests)
	}
	err = withCheckpointKey(ctx, s.db, keys, func(*sql.Tx, []byte) error { return nil })
	if err != nil || len(keys.requests) != 2 || keys.requests[1] {
		t.Fatalf("existing checkpoint allowed key creation: %v %v", err, keys.requests)
	}
}

func TestDurableCheckpointCASAndLease(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaaa")
	if _, err := s.CreateFlow(ctx, f, "mission_1", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_a", FlowID: f.ID, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	markDurableTestRun(t, s, run.ID)
	wake, deadline := storeNow.Add(time.Second), storeNow.Add(time.Hour)
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: wake, Deadline: deadline, Ciphertext: []byte("sealed")}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); !errors.Is(err, errCheckpointConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if got, err := claimDurableCheckpoint(ctx, s.db, run.ID, 1, "worker_a", storeNow, deadline); err != nil || got {
		t.Fatalf("early claim: %v %v", got, err)
	}
	if got, err := claimDurableCheckpoint(ctx, s.db, run.ID, 1, "worker_a", wake, deadline); err != nil || !got {
		t.Fatalf("claim: %v %v", got, err)
	}
	if got, err := claimDurableCheckpoint(ctx, s.db, run.ID, 1, "worker_b", wake, deadline); err != nil || got {
		t.Fatalf("duplicate claim: %v %v", got, err)
	}
	if err := s.SetRunStatus(ctx, run.ID, RunRunning); err != nil {
		t.Fatal(err)
	}
	rec.Generation = 2
	if err := saveDurableCheckpoint(ctx, s.db, rec, 1, "worker_b", wake); !errors.Is(err, errCheckpointConflict) {
		t.Fatalf("foreign lease: %v", err)
	}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 1, "worker_a", wake); err != nil {
		t.Fatal(err)
	}
	header, err := s.GetRunHeader(ctx, run.ID)
	if err != nil || header.Status != RunWaiting {
		t.Fatalf("run state: %+v %v", header, err)
	}
}

func TestDurableCheckpointSaveRollsBackWhenRunNotRunning(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaab")
	if _, err := s.CreateFlow(ctx, f, "mission_2", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_b", FlowID: f.ID, Mode: ModeLive, Status: RunCancelled, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: storeNow.Add(time.Second), Deadline: storeNow.Add(time.Hour), Ciphertext: []byte("sealed")}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); !errors.Is(err, errCheckpointConflict) {
		t.Fatalf("nonrunning save: %v", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphan checkpoint: %d %v", count, err)
	}
}

func TestDurableActiveCheckpointFencesStepAndState(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	f := sampleFlow("flow_aaaaaaaaad")
	if _, err := s.CreateFlow(ctx, f, "mission_4", storeNow); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_d", FlowID: f.ID, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}
	if err := s.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	markDurableTestRun(t, s, run.ID)
	wake, deadline := storeNow.Add(time.Second), storeNow.Add(time.Hour)
	rec := durableCheckpointRecord{RunID: run.ID, Generation: 1, WakeAt: wake, Deadline: deadline, Ciphertext: []byte("first")}
	if err := saveDurableCheckpoint(ctx, s.db, rec, 0, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if ok, err := claimDurableCheckpoint(ctx, s.db, run.ID, 1, "owner", wake, deadline); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	if err := s.SetRunStatus(ctx, run.ID, RunRunning); err != nil {
		t.Fatal(err)
	}
	rec.Generation, rec.Ciphertext = 2, []byte("second")
	step := StepRecord{NodeID: f.Nodes[1].ID, NodeKey: "echo", Attempt: 1, Status: StepSuccess, StartedAt: wake, FinishedAt: wake, Output: map[string]any{"x": 1}}
	for _, owner := range []string{"wrong", "owner"} {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = s.saveActiveCheckpointTx(ctx, tx, rec, 1, owner, wake, deadline, 4, step)
		if owner == "wrong" {
			_ = tx.Rollback()
			if !errors.Is(err, errCheckpointConflict) {
				t.Fatalf("foreign lease: %v", err)
			}
		} else {
			if err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
	}
	var generation int
	if err := s.db.QueryRow(`SELECT generation FROM flow_checkpoints WHERE run_id=?`, run.ID).Scan(&generation); err != nil || generation != 2 {
		t.Fatalf("generation %d %v", generation, err)
	}
	_, steps, err := s.GetRun(ctx, run.ID)
	if err != nil || len(steps) != 1 || steps[0].NodeID != step.NodeID {
		t.Fatalf("stored steps %+v %v", steps, err)
	}
	expired := wake.Add(time.Second)
	if _, err := s.db.Exec(`UPDATE flow_checkpoints SET lease_until=? WHERE run_id=?`, formatTime(expired), run.ID); err != nil {
		t.Fatal(err)
	}
	recoveredAt := expired.Add(time.Second)
	if ok, err := claimDurableCheckpoint(ctx, s.db, run.ID, 2, "owner2", recoveredAt, deadline); err != nil || !ok {
		t.Fatalf("reclaim running pure run: %v %v", ok, err)
	}
	if err := s.SetRunStatus(ctx, run.ID, RunRunning); err != nil {
		t.Fatal(err)
	}
	rec.Generation = 3
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = s.saveActiveCheckpointTx(ctx, tx, rec, 2, "owner", recoveredAt, deadline, 5, step)
	_ = tx.Rollback()
	if !errors.Is(err, errCheckpointConflict) {
		t.Fatalf("old worker advanced after reclaim: %v", err)
	}
	result := RunResult{Status: RunSuccess, FinishedAt: recoveredAt}
	for _, owner := range []string{"owner", "owner2"} {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = s.finishDurableCheckpointTx(ctx, tx, run.ID, 2, owner, recoveredAt, result)
		if owner == "owner" {
			_ = tx.Rollback()
			if !errors.Is(err, errCheckpointConflict) {
				t.Fatalf("foreign finish: %v", err)
			}
		} else {
			if err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
	}
	header, err := s.GetRunHeader(ctx, run.ID)
	if err != nil || header.Status != RunSuccess {
		t.Fatalf("finished run %+v %v", header, err)
	}
	var remaining int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, run.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("finished checkpoint %d %v", remaining, err)
	}
}
