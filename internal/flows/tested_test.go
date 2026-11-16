package flows

import (
	"context"
	"testing"
	"time"
)

// testedFlow is a manual trigger feeding a Stop step that ends the run with the given status.
func testedFlow(name, status string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	halt := b.node("halt", TypeStop, map[string]any{"status": status, "message": "Ende"})
	b.edge(start, PortOut, halt)
	return b.build()
}

func mustCreate(t *testing.T, s *Service, doc *Flow) *FlowRecord {
	t.Helper()
	rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: doc})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return rec
}

func TestTestedAfterASuccessfulSimulation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, testedFlow("Ok", "success"))
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a new flow is tested")
	}
	res, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision})
	if err != nil || res.Status != RunSuccess {
		t.Fatalf("Simulate = %+v, %v", res, err)
	}
	if !s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a successful simulation did not mark the draft revision")
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision+1) {
		t.Fatal("the next revision is tested before anything ran")
	}
	// A save makes a new revision, which is untested.
	rev, _, err := s.SaveDraft(ctx, rec.ID, testedFlow("Ok", "success"), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsTested(ctx, rec.ID, rev) {
		t.Fatal("a save left the flow tested")
	}
	// An older simulation finishing late does not erase the newer mark.
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rev}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.markSimulated(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	if !s.IsTested(ctx, rec.ID, rev) {
		t.Fatal("an older revision's mark replaced the newer one")
	}
}

func TestTestedNotAfterAFailedSimulation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, testedFlow("Fehler", "error"))
	res, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision})
	if err != nil || res.Status != RunError {
		t.Fatalf("Simulate = %+v, %v; want a failed simulation", res, err)
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a failed simulation marked the draft revision")
	}
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision + 7}); err == nil {
		t.Fatal("a simulation of a wrong revision answered")
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) || s.IsTested(ctx, rec.ID, rec.DraftRevision+7) {
		t.Fatal("a refused simulation marked a revision")
	}
}

func TestTestedFollowsTheStoredTestRuns(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	bad := mustCreate(t, s, testedFlow("Fehler", "error"))
	started, err := s.StartTestRun(ctx, bad.ID, TestRunRequest{BaseRevision: bad.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunError {
		t.Fatalf("test run status = %s, want error", run.Status)
	}
	if s.IsTested(ctx, bad.ID, bad.DraftRevision) {
		t.Fatal("a failed test run marked the draft revision")
	}
	good := mustCreate(t, s, testedFlow("Ok", "success"))
	started, err = s.StartTestRun(ctx, good.ID, TestRunRequest{BaseRevision: good.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunSuccess {
		t.Fatalf("test run status = %s, want success", run.Status)
	}
	if !s.IsTested(ctx, good.ID, good.DraftRevision) {
		t.Fatal("a successful test run did not mark the draft revision")
	}
	rev, _, err := s.SaveDraft(ctx, good.ID, testedFlow("Ok", "success"), good.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsTested(ctx, good.ID, rev) {
		t.Fatal("a save left the flow tested")
	}
}

func TestTestedAfterABlockSimulationAndItsDelete(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Leer")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := block.Draft.Clone()
	delete(doc.Nodes[1].Params, "values")
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	if s.IsTested(ctx, block.ID, rev) {
		t.Fatal("a new block draft is tested")
	}
	if res, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev}); err != nil || res.Status != RunSuccess {
		t.Fatalf("SimulateBlock = %+v, %v", res, err)
	}
	if !s.IsTested(ctx, block.ID, rev) {
		t.Fatal("a successful block simulation did not mark the draft revision")
	}
	if err := s.DeleteBlock(ctx, block.ID, rev, 0); err != nil {
		t.Fatalf("DeleteBlock: %v", err)
	}
	if _, ok, err := s.store.simulatedRevision(ctx, block.ID); err != nil || ok {
		t.Fatalf("the marker of a deleted block is still stored: %v %v", ok, err)
	}
}

// A test run that ended with success keeps its flow tested after retention prunes the run's
// row; a failed or cancelled test run writes no marker.
func TestTestedSurvivesPruningOfTheTestRun(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	good := mustCreate(t, s, testedFlow("Ok", "success"))
	started, err := s.StartTestRun(ctx, good.ID, TestRunRequest{BaseRevision: good.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunSuccess {
		t.Fatalf("test run status = %s, want success", run.Status)
	}
	// The hook runs just after the row is saved: wait for the marker.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if revision, ok, err := s.store.simulatedRevision(ctx, good.ID); err == nil && ok && revision == good.DraftRevision {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a successful test run wrote no marker")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := s.store.db.ExecContext(ctx, `DELETE FROM flow_runs WHERE flow_id = ?`, good.ID); err != nil {
		t.Fatal(err)
	}
	if !s.IsTested(ctx, good.ID, good.DraftRevision) {
		t.Fatal("pruning the test run's row made the flow untested")
	}

	// The hook itself: only success writes.
	bad := mustCreate(t, s, testedFlow("Fehler", "error"))
	for _, status := range []RunStatus{RunError, RunCancelled} {
		s.onRunFinished(RunRecord{FlowID: bad.ID, Mode: ModeTest, Revision: bad.DraftRevision}, RunResult{Status: status})
		if _, ok, err := s.store.simulatedRevision(ctx, bad.ID); err != nil || ok {
			t.Fatalf("a test run that ended with %s wrote a marker (%v, %v)", status, ok, err)
		}
	}
	s.onRunFinished(RunRecord{FlowID: bad.ID, Mode: ModeTest, Revision: bad.DraftRevision}, RunResult{Status: RunSuccess})
	if !s.IsTested(ctx, bad.ID, bad.DraftRevision) {
		t.Fatal("a successful test run result did not mark the revision")
	}
}
