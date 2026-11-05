package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func TestChildHistoryNestedRunAndSimulation(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	outer := nestedGreeting(t, s, leaf)
	parent, call := blockParent(t, s, outer, "Child evidence")
	blockSim, err := s.SimulateBlock(ctx, outer.ID, BlockSimulationRequest{BaseRevision: outer.DraftRevision, Inputs: map[string]any{"name": "Ada"}})
	if err != nil || len(blockSim.ChildTraces) != 1 || len(blockSim.ChildTraces[0].Frames) != 1 {
		t.Fatalf("block simulation history: %+v, %v", blockSim, err)
	}
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunSuccess || len(sim.ChildTraces) != 1 || len(sim.ChildDocuments) != 2 {
		t.Fatalf("simulation trace: %+v, %v", sim, err)
	}
	trace := sim.ChildTraces[0]
	if trace.RunID != "" || trace.ParentSeq <= 0 || len(trace.Frames) != 2 || trace.Frames[1].ParentInvocationID != trace.Frames[0].InvocationID {
		t.Fatalf("simulation frame identities: %+v", trace)
	}
	var summary *ChildHistorySummary
	for _, step := range sim.Steps {
		if step.NodeID == call {
			summary = step.ChildHistory
		}
	}
	if summary == nil || summary.ParentSeq != trace.ParentSeq {
		t.Fatalf("simulation parent sequence: %+v", summary)
	}
	start, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitRun(t, s, start.RunID); result.Status != RunSuccess {
		t.Fatalf("run: %+v", result)
	}
	detail, err := s.Run(ctx, start.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	summary = nil
	for _, step := range detail.Steps {
		if step.NodeID == call {
			summary = step.ChildHistory
		}
	}
	if summary == nil || summary.State != ChildHistoryComplete {
		t.Fatalf("stored summary: %+v", summary)
	}
	stored, err := s.ChildTrace(ctx, start.RunID, summary.ParentSeq)
	if err != nil || len(stored.Frames) != 2 || stored.RunID != start.RunID {
		t.Fatalf("stored trace: %+v, %v", stored, err)
	}
	doc, err := s.ChildTraceDocument(ctx, start.RunID, summary.ParentSeq, stored.Frames[1].InvocationID)
	if err != nil || doc.Doc == nil || doc.Doc.ID != leaf.ID || doc.Revision != leaf.LiveRevision {
		t.Fatalf("pinned doc: %+v, %v", doc, err)
	}
}

func TestChildHistoryRejectBeforeEntry(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	for _, batch := range []bool{false, true} {
		var parent *FlowRecord
		var call string
		if batch {
			parent, call = batchParent(t, s, block, []any{"Ada"}, 2, "collect")
		} else {
			parent, call = blockParent(t, s, block, "Pre-entry")
		}
		plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := parent.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		node := doc.NodeByID(call)
		if batch {
			node.Params["items"] = "{{missing.value}}"
		} else {
			node.Params["inputs"] = map[string]any{"name": "{{missing.value}}"}
		}
		var trigger string
		for _, n := range doc.Nodes {
			if n.Type == TypeTriggerManual {
				trigger = n.ID
				break
			}
		}
		engine := NewEngine(s.reg, s.services, discardLogger(), 1)
		result := engine.Execute(ctx, RunRequest{Flow: doc, Mode: ModeTest, TriggerNode: trigger, TriggerType: "manual", blocks: plan, scope: &blockRunScope{}}, nil)
		if result.Status == RunSuccess || len(result.childTraces) != 1 {
			t.Fatalf("batch=%v result %+v", batch, result)
		}
		trace := result.childTraces[0]
		if batch {
			if trace.History.State != ChildHistoryIncomplete || trace.History.TotalInvocations != nil {
				t.Fatalf("batch preflight: %+v", trace)
			}
		} else if len(trace.Frames) != 1 || trace.Frames[0].Entered || trace.Frames[0].ErrorCode == "" {
			t.Fatalf("call preflight: %+v", trace)
		}
	}
}

func TestChildHistoryOmissionAndFrozenCollector(t *testing.T) {
	value := map[string]any{"data": strings.Repeat("x", maxChildHistoryValueBytes)}
	stored, truncated := childBoundValue(value, false, Scrubber{})
	if !truncated || stored["_preview"] != "" || stored["_omitted"] == nil {
		t.Fatalf("oversized value: %+v", stored)
	}
	stored, truncated = childBoundValue(map[string]any{"_preview": "secret-prefix"}, true, Scrubber{})
	if !truncated || strings.Contains(stored["_preview"].(string), "secret") {
		t.Fatal("partial preview retained")
	}
	collector := newChildCollector(&blockRunScope{})
	block := &resolvedBlock{Ref: blockRef{FlowID: "block_id", Revision: 1}}
	id := collector.open("", "node", 1, block, nil)
	collector.step(id, StepRecord{NodeID: "child", Status: StepSuccess})
	trace := collector.freeze(true)
	before, _ := json.Marshal(trace)
	collector.step(id, StepRecord{NodeID: "late", Status: StepSuccess})
	after, _ := json.Marshal(trace)
	if string(before) != string(after) || trace.History.State != ChildHistoryIncomplete {
		t.Fatal("late write changed frozen evidence")
	}
}

func TestChildHistorySimulationResponseBudgets(t *testing.T) {
	const count = 26
	sim := SimulationResult{Mode: ModeSimulation, Status: RunSuccess}
	for i := 0; i < count; i++ {
		frame := ChildFrame{InvocationID: "i_1", CallNodeID: "n_aaaaaaaa", CallAttempt: 1, BlockID: "block_v1", Revision: 1, Hash: strings.Repeat("a", 64), Entered: true, Status: RunSuccess}
		for j := 0; j < 12; j++ {
			frame.Steps = append(frame.Steps, ChildStep{Ordinal: j + 1, Step: StepRecord{NodeID: "n_bbbbbbbb", NodeKey: "step", Attempt: 1, Status: StepSuccess, Output: map[string]any{"text": strings.Repeat("x", 15000)}}})
		}
		seq := i + 1
		sim.ChildTraces = append(sim.ChildTraces, ChildTrace{Schema: 1, ParentSeq: seq, History: ChildHistorySummary{ParentSeq: seq, State: ChildHistoryComplete}, Frames: []ChildFrame{frame}})
		sim.Steps = append(sim.Steps, StepRecord{NodeID: "n_aaaaaaaa", ChildHistory: &ChildHistorySummary{ParentSeq: seq, State: ChildHistoryComplete}})
	}
	for i := 0; i < 16; i++ {
		doc := sampleFlow("flow_doc")
		doc.Kind = KindBlock
		doc.Description = strings.Repeat("z", 150000)
		sim.ChildDocuments = append(sim.ChildDocuments, ChildTraceDocument{BlockID: doc.ID, Revision: i + 1, Hash: strings.Repeat("a", 64), Doc: doc})
	}
	data, err := EncodeChildHistoryResponse(&sim, Scrubber{})
	if err != nil {
		t.Fatal(err)
	}
	var got SimulationResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	traces, _ := json.Marshal(got.ChildTraces)
	docs, _ := json.Marshal(got.ChildDocuments)
	if len(traces) > maxChildHistoryBytes || len(docs) > MaxChildSimulationDocumentBytes {
		t.Fatalf("encoded budgets: traces=%d docs=%d", len(traces), len(docs))
	}
	var truncated, missing bool
	for _, trace := range got.ChildTraces {
		truncated = truncated || trace.History.Truncated
	}
	for i, trace := range got.ChildTraces {
		if got.Steps[i].ChildHistory == nil || got.Steps[i].ChildHistory.State != trace.History.State || got.Steps[i].ChildHistory.RetainedSteps != trace.History.RetainedSteps {
			t.Fatalf("summary differs at %d", i)
		}
	}
	for _, doc := range got.ChildDocuments {
		missing = missing || doc.DocMissing == ChildDocumentBudget
	}
	if !truncated || !missing {
		t.Fatalf("missing budget evidence: truncated=%v missing=%v", truncated, missing)
	}
}

func TestChildHistoryBudgetKeepsAtomicMarker(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, call := blockParent(t, s, block, "Budget marker")
	const runID = "run_child_marker"
	if err := s.store.CreateRun(ctx, RunRecord{ID: runID, FlowID: parent.ID, Mode: ModeTest, Status: RunRunning, StartedAt: storeNow}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_runs SET child_history_bytes=? WHERE id=?`, maxChildHistoryBytes-maxChildHistoryMetadataBytes, runID); err != nil {
		t.Fatal(err)
	}
	trace := &ChildTrace{Schema: 1, RunID: runID, ParentSeq: 7, History: ChildHistorySummary{ParentSeq: 7, State: ChildHistoryComplete}, Frames: []ChildFrame{}}
	step := StepRecord{NodeID: call, NodeKey: "greet", Status: StepSuccess}
	if err := s.store.saveChildStep(ctx, runID, 7, step, trace); err != nil {
		t.Fatal(err)
	}
	var marker sql.NullString
	var total, metadata int
	if err := s.store.db.QueryRow(`SELECT t.trace_json,r.child_history_bytes,r.child_history_metadata FROM flow_child_traces t JOIN flow_runs r ON r.id=t.run_id WHERE t.run_id=? AND t.parent_seq=7`, runID).Scan(&marker, &total, &metadata); err != nil {
		t.Fatal(err)
	}
	if marker.Valid || total > maxChildHistoryBytes || total <= maxChildHistoryBytes-maxChildHistoryMetadataBytes || metadata <= 0 {
		t.Fatalf("marker=%+v total=%d metadata=%d", marker, total, metadata)
	}
	got, err := s.ChildTrace(ctx, runID, 7)
	if err != nil || got.History.State != ChildHistoryTruncated || !got.History.Truncated || len(got.Frames) != 0 {
		t.Fatalf("marker read: %+v %v", got, err)
	}
}

func TestChildHistoryBatchItemsKeepInvocationIdentity(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, _ := batchParent(t, s, block, []any{"Ada", float64(42)}, 2, "collect")
	result, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || result.Status != RunSuccess || len(result.ChildTraces) != 1 {
		t.Fatalf("batch: %+v %v", result, err)
	}
	frames := result.ChildTraces[0].Frames
	if len(frames) != 2 || frames[0].ItemIndex == nil || *frames[0].ItemIndex != 0 || !frames[0].Entered || frames[1].ItemIndex == nil || *frames[1].ItemIndex != 1 || frames[1].Entered || frames[1].ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("batch frames: %+v", frames)
	}
}
