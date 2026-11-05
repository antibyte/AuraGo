package flows

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Observe the real Runner without waiting for a terminal database write that the
// test deliberately rejects. Shutdown joins the worker before inspecting storage.
func childReviewRun(t *testing.T, s *Service, parent *FlowRecord) (string, []RunEvent) {
	t.Helper()
	ctx := context.Background()
	events := make(chan RunEvent, 16)
	s.runner.testOnEvent = func(event RunEvent) { events <- event }
	start, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	var observed []RunEvent
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			observed = append(observed, event)
			if event.Type == EventRunFinished {
				joinCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				if err := s.Shutdown(joinCtx); err != nil {
					t.Fatal(err)
				}
				return start.RunID, observed
			}
		case <-timer.C:
			t.Fatal("real Runner did not finish")
		}
	}
}

func TestChildHistoryReviewAtomicTraceFailure(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	parent, call := blockParent(t, s, publishedGreetingBlock(t, s), "Atomic trace failure")
	if _, err := s.store.db.Exec(`CREATE TRIGGER child_review_reject_trace BEFORE INSERT ON flow_child_traces
		BEGIN SELECT RAISE(ABORT, 'injected child trace failure'); END`); err != nil {
		t.Fatal(err)
	}
	runID, events := childReviewRun(t, s, parent)
	finished, parentSeq := 0, 0
	for _, event := range events {
		if event.Type == EventStepFinished && event.NodeID == call {
			finished++
			parentSeq = event.Seq
			if event.Step == nil || event.Step.Status != StepSuccess || event.Step.Output["greeting"] != "Hello Ada" || event.Step.ChildHistory != nil {
				t.Fatalf("execution changed or uncommitted history advertised: %+v", event.Step)
			}
		}
		if event.NodeID != "" && event.NodeID != call && event.NodeID != parent.Draft.Nodes[0].ID {
			t.Fatal("child step leaked into parent event stream")
		}
	}
	if finished != 1 {
		t.Fatalf("parent executed %d times", finished)
	}
	var traces, parentRows, bytes, metadata int
	if err := s.store.db.QueryRow(`SELECT (SELECT COUNT(*) FROM flow_child_traces WHERE run_id=?),
		(SELECT COUNT(*) FROM flow_run_steps WHERE run_id=? AND seq=?),child_history_bytes,child_history_metadata
		FROM flow_runs WHERE id=?`, runID, runID, parentSeq, runID).Scan(&traces, &parentRows, &bytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if traces != 0 || parentRows != 0 || bytes != 0 || metadata != 0 {
		t.Fatalf("partial journal commit: traces=%d parents=%d bytes=%d metadata=%d", traces, parentRows, bytes, metadata)
	}
	rec, _, err := s.store.GetRun(ctx, runID)
	if err != nil || rec.Status != RunSuccess || rec.ChildHistoryState != ChildHistoryIncomplete {
		t.Fatalf("trace failure changed execution or claimed complete history: %+v, %v", rec, err)
	}
}

func TestChildHistoryReviewTerminalCommitFailure(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	parent, _ := blockParent(t, s, publishedGreetingBlock(t, s), "Terminal trace failure")
	if _, err := s.store.db.Exec(`CREATE TRIGGER child_review_reject_terminal BEFORE UPDATE ON flow_runs
		WHEN NEW.child_history_state != OLD.child_history_state AND NEW.child_history_state != 'pending'
		BEGIN SELECT RAISE(ABORT, 'injected terminal history failure'); END`); err != nil {
		t.Fatal(err)
	}
	runID, _ := childReviewRun(t, s, parent)
	var status, state string
	var traces int
	if err := s.store.db.QueryRow(`SELECT status,child_history_state,
		(SELECT COUNT(*) FROM flow_child_traces WHERE run_id=flow_runs.id) FROM flow_runs WHERE id=?`, runID).Scan(&status, &state, &traces); err != nil {
		t.Fatal(err)
	}
	if status != string(RunRunning) || state != string(ChildHistoryPending) || traces != 1 {
		t.Fatalf("terminal write partially committed: status=%s history=%s traces=%d", status, state, traces)
	}
	if _, err := s.store.db.Exec(`DROP TRIGGER child_review_reject_terminal`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if n, err := s.store.MarkInterruptedRuns(ctx, now, now); err != nil || n != 1 {
		t.Fatalf("restart reconciliation: %d, %v", n, err)
	}
	rec, steps, err := s.store.GetRun(ctx, runID)
	if err != nil || rec.Status != RunCancelled || rec.ChildHistoryState != ChildHistoryIncomplete {
		t.Fatalf("interrupted history must remain incomplete: %+v, %v", rec, err)
	}
	header, err := s.store.GetRunHeader(ctx, runID)
	if err != nil || header.ChildHistoryState != ChildHistoryIncomplete {
		t.Fatalf("header disagrees with interrupted run: %+v, %v", header, err)
	}
	for _, step := range steps {
		if step.ChildHistory != nil {
			trace, err := s.ChildTrace(ctx, runID, step.ChildHistory.ParentSeq)
			if err != nil || trace.History.State != ChildHistoryComplete || len(trace.Frames) != 1 {
				t.Fatalf("restart lost already committed invocation evidence: %+v, %v", trace, err)
			}
		}
	}
}

func TestChildHistoryReviewDuplicateInsertPreservesEvidence(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	parent, call := blockParent(t, s, publishedGreetingBlock(t, s), "Immutable child evidence")
	runID, _ := childReviewRun(t, s, parent)
	_, steps, err := s.store.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	var step StepRecord
	for _, candidate := range steps {
		if candidate.NodeID == call {
			step = candidate
		}
	}
	if step.ChildHistory == nil {
		t.Fatal("missing committed parent summary")
	}
	seq := step.ChildHistory.ParentSeq
	trace, err := s.ChildTrace(ctx, runID, seq)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(trace)
	var bytesBefore, metadataBefore int
	if err := s.store.db.QueryRow(`SELECT child_history_bytes,child_history_metadata FROM flow_runs WHERE id=?`, runID).Scan(&bytesBefore, &metadataBefore); err != nil {
		t.Fatal(err)
	}
	// Exercise a duplicate while admission is still pending, rather than letting
	// the terminal-state guard hide an unsafe parent-row replacement.
	if _, err := s.store.db.Exec(`UPDATE flow_runs SET child_history_state='pending' WHERE id=?`, runID); err != nil {
		t.Fatal(err)
	}
	step.Output = map[string]any{"greeting": "replacement must never persist"}
	if err := s.store.saveChildStep(ctx, runID, seq, step, trace); err == nil {
		t.Fatal("duplicate child journal insert accepted")
	}
	afterTrace, err := s.ChildTrace(ctx, runID, seq)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterTrace)
	var bytesAfter, metadataAfter int
	if err := s.store.db.QueryRow(`SELECT child_history_bytes,child_history_metadata FROM flow_runs WHERE id=?`, runID).Scan(&bytesAfter, &metadataAfter); err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) || bytesBefore != bytesAfter || metadataBefore != metadataAfter {
		t.Fatal("duplicate changed immutable evidence or consumed journal quota")
	}
	_, steps, err = s.store.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range steps {
		if candidate.NodeID == call && candidate.Output["greeting"] != "Hello Ada" {
			t.Fatal("duplicate replaced parent record")
		}
	}
	if _, err := s.store.db.Exec(`DELETE FROM flow_run_steps WHERE run_id=? AND seq=?`, runID, seq); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_child_traces WHERE run_id=?`, runID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("parent deletion did not cascade evidence: %d, %v", remaining, err)
	}
}
