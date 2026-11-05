package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitBlockAdmission(t *testing.T, r *Runner) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r.blockAdmissionTokens.mu.Lock()
		n := len(r.blockAdmissionTokens.active)
		r.blockAdmissionTokens.mu.Unlock()
		if n != 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("block admission did not register")
}

func TestBlockAdmissionRevokedAcrossDisableAndReenable(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	pub, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision)
	if err != nil {
		t.Fatalf("publish: %v %+v", err, issues)
	}
	if err := s.SetEnabled(ctx, parent.ID, true); err != nil {
		t.Fatal(err)
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := s.RunNow(ctx, parent.ID); result <- err }()
	waitBlockAdmission(t, s.runner)
	if err := bridge.SetFlowMissionEnabled(pub.MissionID, false); err != nil {
		t.Fatal(err)
	}
	s.InvalidateFlowAdmissions(pub.MissionID)
	if err := bridge.SetFlowMissionEnabled(pub.MissionID, true); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("revoked admission: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("revoked admission remained blocked")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var runs, pins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_run_pins`).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if runs != 0 || pins != 0 {
		t.Fatalf("revoked admission created runs=%d pins=%d", runs, pins)
	}
}

func TestBlockAdmissionCancelFlowModeInterruptsDatabaseWait(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
		result <- err
	}()
	waitBlockAdmission(t, s.runner)
	done := make(chan struct{})
	go func() { s.runner.CancelFlowMode(parent.ID, ModeTest); close(done) }()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled admission: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled admission remained blocked")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("CancelFlowMode remained blocked")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var runs int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("cancelled admission created %d runs: %v", runs, err)
	}
}

func TestBlockAdmissionRevokedAfterCommitBeforeTransfer(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
		result <- err
	}()
	waitBlockAdmission(t, s.runner)
	tracker := &s.runner.blockAdmissionTokens
	tracker.mu.Lock()
	locked := true
	defer func() {
		if locked {
			tracker.mu.Unlock()
		}
	}()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var runID string
	for time.Now().Before(deadline) {
		err = s.store.db.QueryRow(`SELECT id FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runID)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if runID == "" {
		t.Fatal("admission did not commit its header")
	}
	for a := range tracker.active {
		a.cancel()
	}
	tracker.mu.Unlock()
	locked = false
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("revoked transfer: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("revoked transfer remained blocked")
	}
	rec, steps, err := s.store.GetRun(ctx, runID)
	if err != nil || rec.Status != RunCancelled || rec.ErrorCode != "FLOW_CANCELLED" {
		t.Fatalf("committed run was not closed without execution: %+v, %v", rec, err)
	}
	if len(steps) != 0 {
		t.Fatalf("revoked run executed %d steps", len(steps))
	}
}
