package flows

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type auditFaultProjector struct{ fail atomic.Bool }

func (p *auditFaultProjector) ProjectDurableRun(context.Context, string) error {
	if p.fail.Load() {
		panic("private review must not reach the log")
	}
	return nil
}

func TestDurableRuntimeFaultRestartResumesSameRunOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "flows.db")
	store, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	f := newDurableApprovalFixture(t, store, "flow_fault_restart", nil, 0)
	projector := &auditFaultProjector{}
	f.service.runner.cfg.DurableHistory = projector
	f.start(t)
	var before []byte
	var deadline string
	if err := store.db.QueryRow(`SELECT ciphertext,deadline FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&before, &deadline); err != nil {
		t.Fatal(err)
	}
	projector.fail.Store(true)
	f.service.runner.projectDurable(f.started.RunID)
	if !f.service.runner.durableRuntimeFaulted() {
		t.Fatal("projection panic did not latch the fault")
	}
	if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); !errors.Is(err, ErrDurableApprovalUnavailable) {
		t.Fatalf("fault allowed a decision: %v", err)
	}
	if err := f.service.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var after []byte
	var retainedDeadline string
	if err := reopened.db.QueryRow(`SELECT ciphertext,deadline FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&after, &retainedDeadline); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || retainedDeadline != deadline {
		t.Fatal("fault/restart changed the checkpoint or extended its deadline")
	}
	restarted, _, _ := newDurableApprovalService(t, reopened, f.request.Flow.ID, f.clock, 0)
	if err := restarted.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if restarted.runner.durableRuntimeFaulted() {
		t.Fatal("runtime fault survived restart")
	}
	if _, err := restarted.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); err != nil {
		t.Fatal(err)
	}
	restarted.recoverDurableDue(ctx)
	if result := waitRun(t, restarted, f.started.RunID); result.Status != RunSuccess {
		t.Fatalf("restarted run did not finish: %+v", result)
	}
	for range 3 {
		restarted.recoverDurableDue(ctx)
	}
	_, steps, err := reopened.GetRun(ctx, f.started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	continuations := 0
	for _, step := range steps {
		if step.NodeKey == "after" && step.Status == StepSuccess {
			continuations++
		}
	}
	runs, err := restarted.Runs(ctx, f.request.Flow.ID, RunFilter{})
	if err != nil || len(runs) != 1 || continuations != 1 {
		t.Fatalf("duplicate continuation: runs=%d completed steps=%d err=%v", len(runs), continuations, err)
	}
}
