package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
	"time"
)

// nestedGreeting deliberately reuses a call-node ID across distinct documents.
func nestedGreeting(t *testing.T, s *Service, child *FlowRecord) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Nested greeting")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Params["fields"] = []any{map[string]any{"name": "name", "type": "string", "required": true}}
	doc.Nodes[1].Params["fields"] = []any{map[string]any{"name": "greeting", "type": "string", "required": true}}
	doc.Nodes[1].Params["values"] = map[string]any{"greeting": "{{nested.greeting}}"}
	call := Node{ID: "n_zzzzzzzz", Key: "nested", Type: blockCallType, TypeVersion: 1, Params: map[string]any{"flow_id": child.ID, "revision": float64(child.LiveRevision), "inputs": map[string]any{"name": "{{block_input.data.name}}"}}}
	doc.Nodes = append(doc.Nodes, call)
	doc.Edges = []Edge{
		{ID: NewEdgeID(), Source: PortRef{Node: doc.Nodes[0].ID, Port: PortOut}, Target: PortRef{Node: call.ID, Port: PortIn}},
		{ID: NewEdgeID(), Source: PortRef{Node: call.ID, Port: PortOut}, Target: PortRef{Node: doc.Nodes[1].ID, Port: PortIn}},
	}
	revision, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save nested block: %v %+v", err, issues)
	}
	block, issues, err = s.PublishBlock(ctx, block.ID, revision)
	if err != nil {
		t.Fatalf("publish nested block: %v %+v", err, issues)
	}
	return block
}

func TestBlockNestedPinsSimulationAndRealRun(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	middle := nestedGreeting(t, s, leaf)
	outer := nestedGreeting(t, s, middle)
	preview, err := s.BlockPublishPreview(ctx, outer.ID)
	if err != nil || !preview.CanPublish {
		t.Fatalf("block preview: %+v %v", preview, err)
	}
	sim, err := s.SimulateBlock(ctx, outer.ID, BlockSimulationRequest{BaseRevision: outer.DraftRevision, Inputs: map[string]any{"name": "Ada"}})
	if err != nil || sim.Status != RunSuccess || sim.Outputs["greeting"] != "Hello Ada" {
		t.Fatalf("nested block simulation: %+v %v", sim, err)
	}
	batch, batchID := batchParent(t, s, outer, []any{"Ada", "Grace"}, 2, "collect")
	batchResult, err := s.Simulate(ctx, batch.ID, SimulationRequest{BaseRevision: batch.DraftRevision})
	if err != nil || batchResult.Status != RunSuccess {
		t.Fatalf("nested batch simulation: %+v %v", batchResult, err)
	}
	rows := batchStep(t, batchResult, batchID).Output["items"].([]any)
	if rows[1].(map[string]any)["output"].(map[string]any)["greeting"] != "Hello Grace" {
		t.Fatalf("nested batch output: %+v", rows)
	}
	parent, call := blockParent(t, s, outer, "Nested parent")
	parent, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision)
	if err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	def, _ := s.reg.Lookup(blockCallType)
	fake := *def
	var invoked atomic.Bool
	fake.Execute = func(context.Context, ExecInput) (ExecResult, error) { invoked.Store(true); return ExecResult{}, nil }
	s.reg.Replace(&fake)
	for _, tc := range []struct {
		id              string
		revision, count int
	}{{middle.ID, middle.LiveRevision, 1}, {outer.ID, outer.LiveRevision, 2}, {parent.ID, parent.LiveRevision, 3}} {
		var count int
		if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_version_pins WHERE parent_flow_id=? AND parent_revision=?`, tc.id, tc.revision).Scan(&count); err != nil || count != tc.count {
			t.Fatalf("pins %s: %d %v", tc.id, count, err)
		}
	}
	start, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitRun(t, s, start.RunID); result.Status != RunSuccess {
		t.Fatalf("nested test run: %+v", result)
	}
	if invoked.Load() {
		t.Fatal("production registry override reached nested pure execution")
	}
	detail, err := s.Run(ctx, start.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	got := false
	for _, step := range detail.Steps {
		if step.NodeID == call && step.Output["greeting"] == "Hello Ada" {
			got = true
		}
	}
	if !got {
		t.Fatalf("nested output missing: %+v", detail.Steps)
	}
	changed, err := leaf.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	changed.Nodes[1].Params["values"] = map[string]any{"greeting": "Changed {{block_input.data.name}}"}
	rev, issues, err := s.SaveBlockDraft(ctx, leaf.ID, changed, leaf.DraftRevision)
	if err != nil {
		t.Fatalf("save leaf v2: %v %+v", err, issues)
	}
	if _, issues, err := s.PublishBlock(ctx, leaf.ID, rev); err != nil {
		t.Fatalf("publish leaf v2: %v %+v", err, issues)
	}
	flowSim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || flowSim.Status != RunSuccess || flowSim.Steps[len(flowSim.Steps)-1].Output["greeting"] != "Hello Ada" {
		t.Fatalf("pinned nested version changed: %+v %v", flowSim, err)
	}
	if err := s.SetEnabled(ctx, parent.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_block_version_pins SET block_hash=zeroblob(32) WHERE parent_flow_id=? AND parent_revision=?`, middle.ID, middle.LiveRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision}); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("changed intermediate pin accepted: %v", err)
	}
	var before, after int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunNow(ctx, parent.ID); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("changed pin admitted a live run: %v", err)
	}
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&after); err != nil || after != before {
		t.Fatalf("changed pin created a run: %d→%d, %v", before, after, err)
	}
}

func TestBlockNestedSharesCancellationAndOutputBudget(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	middle := nestedGreeting(t, s, leaf)
	outer := nestedGreeting(t, s, middle)
	plan, err := prepareDraftBlockPlan(ctx, s.store, outer.Draft, outer.DraftRevision, s.services)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	result := executePureBlockPlanned(cancelled, outer.Draft, map[string]any{"name": "Ada"}, &Services{}, discardLogger(), ModeSimulation, outer.DraftRevision, "", &blockRunScope{}, plan)
	if result.Status == RunSuccess {
		t.Fatal("cancelled nested frame succeeded")
	}
	expired, stop := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stop()
	result = executePureBlockPlanned(expired, outer.Draft, map[string]any{"name": "Ada"}, &Services{}, discardLogger(), ModeSimulation, outer.DraftRevision, "", &blockRunScope{}, plan)
	if result.Status == RunSuccess {
		t.Fatal("expired nested frame succeeded")
	}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	input := map[string]any{"name": "Ada"}
	encoded, err := json.Marshal(map[string]any{"type": blockInputType, "node": outer.Draft.Nodes[0].Key, "fired_at": at.Format(time.RFC3339), "data": input})
	if err != nil {
		t.Fatal(err)
	}
	initial := MaxRunOutputBytes - len(encoded) - 1
	scope := &blockRunScope{outputBytes: initial}
	services := &Services{Clock: simulationClock{at: at}}
	result = executePureBlockPlanned(ctx, outer.Draft, input, services, discardLogger(), ModeSimulation, outer.DraftRevision, "", scope, plan)
	if result.Status == RunSuccess || scope.outputBytes != initial+len(encoded) {
		t.Fatalf("nested frame did not honor shared output limit: %+v, charged=%d", result, scope.outputBytes-initial)
	}
}

func TestBlockNestedExpandedLimitsAndDepth(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	current := leaf
	for i := 1; i < maxBlockDepth; i++ {
		current = nestedGreeting(t, s, current)
	}
	parent, _ := blockParent(t, s, current, "Depth eight")
	if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); err != nil {
		t.Fatalf("depth eight rejected: %v", err)
	}
	tooDeep := nestedGreeting(t, s, current)
	parent, callID := blockParent(t, s, tooDeep, "Depth nine")
	if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("depth nine accepted: %v", err)
	}
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil || !hasBlockDependencyIssue(preview.Issues, callID) {
		t.Fatalf("depth preview does not identify root call %s: %+v %v", callID, preview, err)
	}
	wide := parent.Draft
	wide.Nodes = wide.Nodes[:1]
	for i := 0; i < 75; i++ {
		wide.Nodes = append(wide.Nodes, Node{ID: NewNodeID(), Key: fmt.Sprintf("call_%d", i), Type: blockCallType, TypeVersion: 1, Params: map[string]any{"flow_id": current.ID, "revision": float64(current.LiveRevision), "inputs": map[string]any{"name": "Ada"}}})
	}
	wide.Edges = nil
	if _, err := prepareBlockPlan(ctx, s.store, wide, s.services); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("expanded graph accepted: %v", err)
	}
}

func TestBlockNestedCycleRejectedBeforePublish(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	middle := nestedGreeting(t, s, leaf)
	draft, err := middle.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	draft.ID = leaf.ID
	draft.Nodes[2].Params["flow_id"] = middle.ID
	draft.Nodes[2].Params["revision"] = float64(middle.LiveRevision)
	rev, issues, err := s.SaveBlockDraft(ctx, leaf.ID, draft, leaf.DraftRevision)
	if err != nil {
		t.Fatalf("save draft: %v %+v", err, issues)
	}
	preview, err := s.BlockPublishPreview(ctx, leaf.ID)
	if err != nil || preview.CanPublish {
		t.Fatalf("cycle preview accepted: %+v %v", preview, err)
	}
	if !hasBlockDependencyIssue(preview.Issues, draft.Nodes[2].ID) {
		t.Fatalf("cycle preview does not identify root call: %+v", preview.Issues)
	}
	if _, _, err := s.PublishBlock(ctx, leaf.ID, rev); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("cycle publish accepted: %v", err)
	}
	current, err := s.GetBlock(ctx, leaf.ID)
	if err != nil || current.LiveRevision != leaf.LiveRevision {
		t.Fatalf("cycle changed live version: %+v %v", current, err)
	}
}

func hasBlockDependencyIssue(issues []Issue, nodeID string) bool {
	for _, issue := range issues {
		if issue.Code == IssueParamInvalid && issue.NodeID == nodeID && issue.Param == "flow_id" {
			return true
		}
	}
	return false
}

func TestBlockNestedMissingDependencyIdentifiesRootCall(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	outer := nestedGreeting(t, s, nestedGreeting(t, s, publishedGreetingBlock(t, s)))
	parent, callID := blockParent(t, s, outer, "Missing dependency")
	doc, err := parent.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.NodeByID(callID).Params["revision"] = float64(9999)
	if _, issues, err := s.SaveDraft(ctx, parent.ID, doc, parent.DraftRevision); err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil || !hasBlockDependencyIssue(preview.Issues, callID) {
		t.Fatalf("missing revision does not identify root call %s: %+v %v", callID, preview, err)
	}
	_, err = prepareBlockPlan(ctx, s.store, doc, s.services)
	if !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("missing revision lost error cause: %v", err)
	}
}

func TestBlockNestedBatchCountsDescendantAttempts(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	doc, _ := leaf.Draft.Clone()
	doc.Nodes[1].Settings.Retry.Count = MaxRetryCount
	rev, issues, err := s.SaveBlockDraft(ctx, leaf.ID, doc, leaf.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	leaf, issues, err = s.PublishBlock(ctx, leaf.ID, rev)
	if err != nil {
		t.Fatalf("publish: %v %+v", err, issues)
	}
	middle := nestedGreeting(t, s, leaf)
	outer := nestedGreeting(t, s, middle)
	parent, id := batchParent(t, s, outer, []any{}, 1000, "collect")
	if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); !errors.Is(err, ErrBatchWorkLimit) {
		t.Fatalf("nested batch work accepted: %v", err)
	}
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range preview.Issues {
		if issue.Code == IssueBatchWorkLimit && issue.NodeID == id && issue.Param == "max_items" {
			return
		}
	}
	t.Fatalf("nested work missing actionable issue: %+v", preview.Issues)
}

func TestBlockNestedBatchBudgetIdentifiesCrossingBatch(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	outer := nestedGreeting(t, s, nestedGreeting(t, s, publishedGreetingBlock(t, s)))
	parent, firstID := batchParent(t, s, outer, []any{}, 900, "collect")
	doc, err := parent.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	second := *doc.NodeByID(firstID)
	second.ID, second.Key = NewNodeID(), "second_batch"
	doc.Nodes = append(doc.Nodes, second)
	revision, issues, err := s.SaveDraft(ctx, parent.ID, doc, parent.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range preview.Issues {
		if issue.Code == IssueBatchWorkLimit && issue.NodeID == second.ID && issue.Param == "max_items" {
			return
		}
	}
	t.Fatalf("work limit did not identify second batch at revision %d: %+v", revision, preview.Issues)
}

func TestBlockNestedRejectsDisabledEffectsAndNestedBatches(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	for _, typ := range []string{"ai.step", "web.read", blockEachType} {
		doc, err := leaf.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: "forbidden", Type: typ, TypeVersion: 1, Settings: NodeSettings{Disabled: true}, Params: map[string]any{}})
		if blockDocumentAllowed(doc) || !HasErrors(s.ValidateBlock(doc, ModePublish)) {
			t.Fatalf("disabled %s was accepted", typ)
		}
	}
}

func TestBlockNestedRejectsInvalidBatchLimitsBeforeMeasurement(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	outer := nestedGreeting(t, s, publishedGreetingBlock(t, s))
	parent, id := batchParent(t, s, outer, []any{}, 2, "collect")
	for _, invalid := range []any{float64(0), "2", float64(-1), float64(1.5), math.NaN(), math.Inf(1)} {
		doc, err := parent.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		node := doc.NodeByID(id)
		node.Params["max_items"] = invalid
		node.Settings.Disabled = true
		if _, err := prepareBlockPlan(ctx, s.store, doc, s.services); !errors.Is(err, ErrBlockInvalid) {
			t.Fatalf("invalid disabled max_items %v was accepted: %v", invalid, err)
		}
	}
}

func TestBlockNestedPinVerificationRejectsMutatedSnapshot(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	outer := nestedGreeting(t, s, publishedGreetingBlock(t, s))
	parent, _ := blockParent(t, s, outer, "Snapshot")
	plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
	if err != nil {
		t.Fatal(err)
	}
	plan.blocks[0].Document.Name = "mutated snapshot"
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := verifyBlockPinsTx(ctx, tx, plan.blocks); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("mutated snapshot accepted: %v", err)
	}
}

func TestBlockNestedDraftSnapshotRejectsCorruptLiveOwner(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	outer := nestedGreeting(t, s, publishedGreetingBlock(t, s))
	if _, err := s.store.db.Exec(`UPDATE flows SET live_json=json_set(live_json,'$.id','other') WHERE id=?`, outer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDraftBlockPlan(ctx, s.store, outer.Draft, outer.DraftRevision, s.services); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("corrupt live owner accepted in snapshot: %v", err)
	}
}

func TestBlockNestedChangedStoredRevisionFailsClosed(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	middle := nestedGreeting(t, s, leaf)
	outer := nestedGreeting(t, s, middle)
	parent, _ := blockParent(t, s, outer, "Pinned parent")
	if _, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision); err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	changed, err := leaf.Live.Clone()
	if err != nil {
		t.Fatal(err)
	}
	changed.Name = "unexpected replacement"
	wire, err := changed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=?`, string(wire), leaf.ID, leaf.LiveRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision}); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("replacement under same leaf revision was accepted: %v", err)
	}
}

func TestBlockNestedScenarioFingerprintsIncludeTransitivePins(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	leaf := publishedGreetingBlock(t, s)
	middle := nestedGreeting(t, s, leaf)
	outer := nestedGreeting(t, s, middle)
	parent, _ := blockParent(t, s, outer, "Scenario parent")
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	fingerprints, err := scenarioFingerprintsWithBlocks(ctx, tx, parent.Draft, Scenario{TriggerNode: parent.Draft.Nodes[0].ID}, s.services)
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range []*FlowRecord{leaf, middle, outer} {
		key := fmt.Sprintf("__block__:%s:%d", block.ID, block.LiveRevision)
		if fingerprints[key] == "" {
			t.Fatalf("scenario missing transitive fingerprint %s", key)
		}
	}
}
