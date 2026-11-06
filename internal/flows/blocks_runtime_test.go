package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func publishedGreetingBlock(t *testing.T, s *Service) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Greeting")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Params["fields"] = []any{map[string]any{"name": "name", "type": "string", "required": true}}
	doc.Nodes[1].Params["fields"] = []any{map[string]any{"name": "greeting", "type": "string", "required": true}}
	doc.Nodes[1].Params["values"] = map[string]any{"greeting": "Hello {{block_input.data.name}}"}
	revision, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save block: %v %+v", err, issues)
	}
	pub, issues, err := s.PublishBlock(ctx, block.ID, revision)
	if err != nil {
		t.Fatalf("publish block: %v %+v", err, issues)
	}
	return pub
}

func blockParent(t *testing.T, s *Service, block *FlowRecord, name string) (*FlowRecord, string) {
	t.Helper()
	ctx := context.Background()
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	call := b.node("greet", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
	b.edge(start, PortOut, call)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	return rec, call
}

func TestBlockCallSimulationAndPinnedRun(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, call := blockParent(t, s, block, "Parent")
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunSuccess {
		t.Fatalf("simulation: %+v, %v", sim, err)
	}
	var found bool
	for _, step := range sim.Steps {
		if step.NodeID == call && step.Output["greeting"] == "Hello Ada" {
			found = true
		}
	}
	if !found {
		t.Fatalf("call result not found: %+v", sim.Steps)
	}
	if _, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision, Fixtures: map[string]SimulationFixture{call: {Output: map[string]any{}}}}); !errors.Is(err, ErrInvalidSimulationFixture) {
		t.Fatalf("call fixture: %v", err)
	}
	start, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if result := waitRun(t, s, start.RunID); result.Status != RunSuccess {
		t.Fatalf("test run: %+v", result)
	}
	detail, err := s.Run(ctx, start.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, step := range detail.Steps {
		if step.NodeID == call && step.Output["greeting"] == "Hello Ada" {
			found = true
		}
	}
	if !found {
		t.Fatalf("persisted call result not found: %+v", detail.Steps)
	}
	var runPins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_run_pins WHERE run_id=?`, start.RunID).Scan(&runPins); err != nil || runPins != 1 {
		t.Fatalf("run pins %d, %v", runPins, err)
	}
	pub, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision)
	if err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	var versionPins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_version_pins WHERE parent_flow_id=? AND parent_revision=?`, parent.ID, pub.LiveRevision).Scan(&versionPins); err != nil || versionPins != 1 {
		t.Fatalf("version pins %d, %v", versionPins, err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); !errors.Is(err, ErrBlockInUse) {
		t.Fatalf("delete referenced block: %v", err)
	}
}

func TestBlockInvalidTypedOutputStopsSimulationAndParent(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Invalid output")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[1].Params["fields"] = []any{map[string]any{"name": "value", "type": "string", "required": true}}
	doc.Nodes[1].Params["values"] = map[string]any{"value": float64(42)}
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	preview, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev})
	if err != nil || preview.Status != RunError || preview.ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("block simulation accepted invalid output: %+v, %v", preview, err)
	}
	outputFailed := false
	for _, step := range preview.Steps {
		if step.NodeID == doc.Nodes[1].ID {
			outputFailed = step.Status == StepError && step.ErrorCode == "FLOW_PARAM_INVALID"
		}
	}
	if !outputFailed {
		t.Fatalf("output step did not report the type error: %+v", preview.Steps)
	}
	block, issues, err = s.PublishBlock(ctx, block.ID, rev)
	if err != nil {
		t.Fatalf("publish: %v %+v", err, issues)
	}
	b := newFlow("Parent")
	start := b.node("start", TypeTriggerManual, nil)
	call := b.node("call", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{}})
	after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "result", "type": "text", "value": "must not run"}}})
	b.edge(start, PortOut, call)
	b.edge(call, PortOut, after)
	parent, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunError || sim.ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("parent simulation accepted invalid output: %+v, %v", sim, err)
	}
	callFailed := false
	for _, step := range sim.Steps {
		if step.NodeID == call {
			callFailed = step.Status == StepError && step.ErrorCode == "FLOW_PARAM_INVALID"
		}
		if step.NodeID == after && step.Status != StepSkipped {
			t.Fatalf("downstream step ran: %+v", step)
		}
	}
	if !callFailed {
		t.Fatalf("parent call did not report the type error: %+v", sim.Steps)
	}
}

func TestBlockScenarioDetectsChangedVersionBeforeExecution(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	keys := &scenarioTestKeys{}
	saved, err := s.SaveScenario(ctx, parent.ID, parent.DraftRevision, 0, Scenario{Name: "Example", TriggerNode: parent.Draft.Nodes[0].ID}, keys)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := s.SimulateScenario(ctx, parent.ID, saved.ID, parent.DraftRevision, saved.Version, keys); err != nil || result.Status != RunSuccess {
		t.Fatalf("simulation: %+v, %v", result, err)
	}
	loaded, observed, err := s.getScenarioAtDraftRevision(ctx, parent.ID, saved.ID, keys)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := block.Live.Clone()
	if err != nil {
		t.Fatal(err)
	}
	changed.Nodes[1].Params["values"] = map[string]any{"greeting": "Different"}
	raw, err := changed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=?`, string(raw), block.ID, block.LiveRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.simulateLoadedScenario(ctx, parent.ID, parent.DraftRevision, saved.Version, observed, loaded); !errors.Is(err, ErrScenarioStale) {
		t.Fatalf("changed snapshot executed: %v", err)
	}
	got, err := s.GetScenario(ctx, parent.ID, saved.ID, keys)
	if err != nil || !got.Stale {
		t.Fatalf("stale scenario: %+v, %v", got, err)
	}
	list, err := s.ListScenarios(ctx, parent.ID, keys)
	if err != nil || len(list) != 1 || !list[0].Stale {
		t.Fatalf("stale listing: %+v, %v", list, err)
	}
}

func TestBlockTwoParentsKeepPinnedRevisionUntilExplicitUpgrade(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	first, _ := blockParent(t, s, block, "First")
	second, _ := blockParent(t, s, block, "Second")
	for _, parent := range []*FlowRecord{first, second} {
		if _, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision); err != nil {
			t.Fatalf("publish parent: %v %+v", err, issues)
		}
	}
	updated, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	updated.Nodes[1].Params["values"] = map[string]any{"greeting": "Hi {{block_input.data.name}}"}
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, updated, block.DraftRevision)
	if err != nil {
		t.Fatalf("save block: %v %+v", err, issues)
	}
	newBlock, issues, err := s.PublishBlock(ctx, block.ID, rev)
	if err != nil {
		t.Fatalf("publish block: %v %+v", err, issues)
	}
	for _, parent := range []*FlowRecord{first, second} {
		result, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
		if err != nil || result.Status != RunSuccess || result.Steps[len(result.Steps)-1].Output["greeting"] != "Hello Ada" {
			t.Fatalf("old parent: %+v, %v", result, err)
		}
	}
	upgraded, _ := second.Draft.Clone()
	upgraded.Nodes[1].Params["revision"] = float64(newBlock.LiveRevision)
	parentRev, issues, err := s.SaveDraft(ctx, second.ID, upgraded, second.DraftRevision)
	if err != nil {
		t.Fatalf("upgrade: %v %+v", err, issues)
	}
	if _, issues, err := s.Publish(ctx, second.ID, parentRev); err != nil {
		t.Fatalf("publish upgrade: %v %+v", err, issues)
	}
	result, err := s.Simulate(ctx, second.ID, SimulationRequest{BaseRevision: parentRev})
	if err != nil || result.Status != RunSuccess || result.Steps[len(result.Steps)-1].Output["greeting"] != "Hi Ada" {
		t.Fatalf("upgraded parent: %+v, %v", result, err)
	}
	if err := s.DeleteBlock(ctx, block.ID, rev, newBlock.LiveRevision); !errors.Is(err, ErrBlockInUse) {
		t.Fatalf("delete pinned: %v", err)
	}
}

func TestBlockAdmissionCancellationAndAcceptedRunLifetime(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, call := blockParent(t, s, block, "Parent")
	req := StartRequest{Flow: parent.Draft, Revision: parent.DraftRevision, Mode: ModeTest, TriggerNode: parent.Draft.Nodes[0].ID}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.runner.StartContext(canceled, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission: %v", err)
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unwanted run: %d, %v", count, err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	s.runner.testOnEvent = func(ev RunEvent) {
		if ev.Type == EventStepStarted && ev.NodeID == call && once.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
	}
	request, cancelRequest := context.WithCancel(context.Background())
	result, err := s.runner.StartContext(request, req)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("call did not start")
	}
	cancelRequest()
	close(release)
	if got := waitRun(t, s, result.RunID); got.Status != RunSuccess {
		t.Fatalf("accepted run interrupted by request: %+v", got)
	}
}

func TestBlockSharedOutputBudgetAcrossParallelCalls(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	b := newFlow("Budget")
	start := b.node("start", TypeTriggerManual, nil)
	for _, key := range []string{"call_a", "call_b", "call_c", "call_d"} {
		call := b.node(key, blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "{{trigger.data.name}}"}})
		b.edge(start, PortOut, call)
	}
	parent, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision, TriggerData: map[string]any{"name": strings.Repeat("x", 3<<20)}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		detail, err := s.Run(ctx, result.RunID, false)
		if err == nil && detail.Run.Status.Terminal() {
			if detail.Run.Status != RunError || detail.Run.ErrorCode != "FLOW_RUN_OUTPUT_TOO_LARGE" {
				t.Fatalf("shared output budget: %+v", detail.Run)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("budget run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBlockCallPreflightAndRegistryIsolation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, call := blockParent(t, s, block, "Parent")
	var invoked atomic.Int32
	def, _ := s.reg.Lookup(blockCallType)
	fake := *def
	fake.Execute = func(context.Context, ExecInput) (ExecResult, error) { invoked.Add(1); return ExecResult{}, nil }
	s.reg.Replace(&fake)
	result, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitRun(t, s, result.RunID); got.Status != RunSuccess || invoked.Load() != 0 {
		t.Fatalf("untrusted call definition executed: %+v, %d", got, invoked.Load())
	}
	bad, _ := parent.Draft.Clone()
	bad.NodeByID(call).Params["revision"] = float64(999)
	rev, issues, err := s.SaveDraft(ctx, parent.ID, bad, parent.DraftRevision)
	if err != nil {
		t.Fatalf("save draft: %v %+v", err, issues)
	}
	var before int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: rev}); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("missing version: %v", err)
	}
	var after int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&after); err != nil || before != after {
		t.Fatalf("preflight created run: %d -> %d, %v", before, after, err)
	}
	bad.NodeByID(call).Params["revision"] = float64(block.LiveRevision)
	bad.NodeByID(call).Settings.Retry.Count = 1
	rev, issues, err = s.SaveDraft(ctx, parent.ID, bad, rev)
	if err != nil {
		t.Fatalf("save retry draft: %v %+v", err, issues)
	}
	if _, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: rev}); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("call retry accepted: %v", err)
	}
}

func TestBlockOwnerCorruptionFailsBeforeConsumption(t *testing.T) {
	for _, column := range []string{"draft_json", "live_json"} {
		t.Run(column, func(t *testing.T) {
			s, _ := newServiceFixture(t, nil)
			ctx := context.Background()
			block := publishedGreetingBlock(t, s)
			parent, _ := blockParent(t, s, block, "Parent")
			plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
			if err != nil {
				t.Fatal(err)
			}
			query := `UPDATE flows SET ` + column + `=json_set(` + column + `,'$.kind','flow') WHERE id=?`
			if _, err := s.store.db.ExecContext(ctx, query, block.ID); err != nil {
				t.Fatal(err)
			}
			spy := &fakeTools{}
			s.services.Tools = spy
			if _, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision}); !errors.Is(err, ErrBlockUnavailable) {
				t.Fatalf("simulation accepted corrupt owner: %v", err)
			}
			if _, _, err := s.Publish(ctx, parent.ID, parent.DraftRevision); !errors.Is(err, ErrBlockUnavailable) {
				t.Fatalf("publish accepted corrupt owner: %v", err)
			}
			if _, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision}); !errors.Is(err, ErrBlockUnavailable) {
				t.Fatalf("start accepted corrupt owner: %v", err)
			}
			tx, err := s.store.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
				tx.Rollback()
				t.Fatal(err)
			}
			if err := verifyBlockPinsTx(ctx, tx, plan.blocks); !errors.Is(err, ErrBlockUnavailable) {
				tx.Rollback()
				t.Fatalf("pin accepted corrupt owner: %v", err)
			}
			tx.Rollback()
			var runs, pins int
			if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil {
				t.Fatal(err)
			}
			if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_version_pins WHERE parent_flow_id=?`, parent.ID).Scan(&pins); err != nil {
				t.Fatal(err)
			}
			if runs != 0 || pins != 0 || spy.count() != 0 {
				t.Fatalf("side effects: runs=%d pins=%d tools=%d", runs, pins, spy.count())
			}
		})
	}
}

func TestBlockLiveAdmissionRejectsChangedPinnedContent(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
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
	changed, _ := block.Live.Clone()
	changed.Nodes[1].Params["values"] = map[string]any{"greeting": "Different"}
	raw, err := changed.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=?`, string(raw), block.ID, block.LiveRevision); err != nil {
		t.Fatal(err)
	}
	spy := &fakeTools{}
	s.services.Tools = spy
	if _, err := s.RunNow(ctx, parent.ID); !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("changed pinned content accepted: %v", err)
	}
	var runs, pins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_run_pins`).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if runs != 0 || pins != 0 || spy.count() != 0 || pub.LiveRevision != 1 {
		t.Fatalf("side effects: runs=%d pins=%d tools=%d", runs, pins, spy.count())
	}
}

func TestBlockExpandedNodeLimitCountsRepeatedCalls(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	b := newFlow("Repeated")
	start := b.node("start", TypeTriggerManual, nil)
	for _, key := range []string{"first", "second"} {
		call := b.node(key, blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
		b.edge(start, PortOut, call)
	}
	doc := b.build()
	for i := 0; i < 493; i++ {
		doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: fmt.Sprintf("extra_%d", i), Type: TypeSet, TypeVersion: 1})
	}
	if _, err := prepareBlockPlan(context.Background(), s.store, doc, s.services); err != nil {
		t.Fatalf("500 expanded nodes should fit: %v", err)
	}
	doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: "overflow", Type: TypeSet, TypeVersion: 1})
	if _, err := prepareBlockPlan(context.Background(), s.store, doc, s.services); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("501 expanded nodes: %v", err)
	}
}

func TestBlockExpandedEdgeAndDocumentLimits(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	b := newFlow("Limits")
	start := b.node("start", TypeTriggerManual, nil)
	call := b.node("call", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
	b.edge(start, PortOut, call)
	doc := b.build()
	for i := 0; i < 63; i++ {
		doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: fmt.Sprintf("extra_%d", i), Type: TypeSet, TypeVersion: 1, Params: map[string]any{"fields": []any{map[string]any{"name": "x", "value": "x"}}}})
	}
	doc.Edges = nil
	for i := 0; i < len(doc.Nodes) && len(doc.Edges) < MaxEdges; i++ {
		for j := i + 1; j < len(doc.Nodes) && len(doc.Edges) < MaxEdges; j++ {
			doc.Edges = append(doc.Edges, Edge{ID: NewEdgeID(), Source: PortRef{Node: doc.Nodes[i].ID, Port: PortOut}, Target: PortRef{Node: doc.Nodes[j].ID, Port: PortIn}})
		}
	}
	if len(doc.Edges) != MaxEdges {
		t.Fatalf("fixture edges=%d", len(doc.Edges))
	}
	last := doc.Edges[len(doc.Edges)-1]
	doc.Edges = doc.Edges[:len(doc.Edges)-1]
	if _, err := prepareBlockPlan(ctx, s.store, doc, s.services); err != nil {
		t.Fatalf("2000 expanded edges should fit: %v", err)
	}
	doc.Edges = append(doc.Edges, last)
	if _, err := prepareBlockPlan(ctx, s.store, doc, s.services); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("2001 expanded edges: %v", err)
	}

	b = newFlow("Bytes")
	start = b.node("start", TypeTriggerManual, nil)
	for _, key := range []string{"first", "second"} {
		id := b.node(key, blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "Ada"}})
		b.edge(start, PortOut, id)
	}
	doc = b.build()
	doc.Description = "x"
	parentJSON, _ := doc.Marshal()
	childJSON, _ := block.Live.Marshal()
	length := MaxDocumentBytes - 2*len(childJSON) - len(parentJSON) + 2
	if length <= 0 {
		t.Fatal("invalid size fixture")
	}
	doc.Description = strings.Repeat("x", length)
	parentJSON, _ = doc.Marshal()
	if len(parentJSON) > MaxDocumentBytes {
		t.Fatal("parent alone exceeds document limit")
	}
	single, _ := doc.Clone()
	single.Nodes = single.Nodes[:2]
	single.Edges = single.Edges[:1]
	if _, err := prepareBlockPlan(ctx, s.store, single, s.services); err != nil {
		t.Fatalf("single bounded call: %v", err)
	}
	if _, err := prepareBlockPlan(ctx, s.store, doc, s.services); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("repeated document bytes: %v", err)
	}
}

func TestBlockAdmissionBoundAndDatabaseCancellation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	req := StartRequest{Flow: parent.Draft, Revision: parent.DraftRevision, Mode: ModeTest, TriggerNode: parent.Draft.Nodes[0].ID}
	for i := 0; i < cap(s.runner.blockAdmissions); i++ {
		s.runner.blockAdmissions <- struct{}{}
	}
	if _, err := s.runner.StartContext(ctx, req); !errors.Is(err, ErrRunnerQueueFull) {
		t.Fatalf("admission cap: %v", err)
	}
	for i := 0; i < cap(s.runner.blockAdmissions); i++ {
		<-s.runner.blockAdmissions
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	request, cancel := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { _, err := s.runner.StartContext(request, req); result <- err }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			tx.Rollback()
			t.Fatalf("cancel while database busy: %v", err)
		}
	case <-time.After(3 * time.Second):
		tx.Rollback()
		<-result
		t.Fatal("cancelled admission did not return while database was busy")
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
		t.Fatalf("cancelled admission wrote runs=%d pins=%d", runs, pins)
	}
}

func TestBlockCannotBeStartedAsIndependentFlow(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Block")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.runner.Start(StartRequest{Flow: block.Draft, Mode: ModeTest, TriggerNode: block.Draft.Nodes[0].ID}); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("block runner start: %v", err)
	}
	forged, _ := block.Draft.Clone()
	forged.Kind = KindFlow
	if _, err := s.runner.Start(StartRequest{Flow: forged, Mode: ModeTest, TriggerNode: forged.Nodes[0].ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("forged flow start: %v", err)
	}
	err = s.store.CreateRun(ctx, RunRecord{ID: NewRunID(), FlowID: block.ID, Mode: ModeTest, Status: RunQueued, StartedAt: time.Now()}, forged)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("block run insert: %v", err)
	}
	var runs int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, block.ID).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("block runs=%d, %v", runs, err)
	}
}

type observedBlockContext struct {
	context.Context
	seen chan struct{}
	once sync.Once
}

func (c *observedBlockContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.seen) })
	return c.Context.Done()
}

func TestBlockChildHonorsParentCancellationAndDeadline(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, cancel := context.WithCancel(context.Background())
	observed := &observedBlockContext{Context: parent, seen: make(chan struct{})}
	result := make(chan RunResult, 1)
	go func() {
		result <- executePureBlock(observed, block.Live, map[string]any{"name": strings.Repeat("x", 3<<20)}, &Services{}, discardLogger(), ModeTest, block.LiveRevision, "", &blockRunScope{})
	}()
	select {
	case <-observed.seen:
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("child did not start")
	}
	select {
	case got := <-result:
		if got.Status == RunSuccess {
			t.Fatalf("cancelled child succeeded: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child ignored cancellation")
	}
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	got := executePureBlock(expired, block.Live, map[string]any{"name": "Ada"}, &Services{}, discardLogger(), ModeTest, block.LiveRevision, "", &blockRunScope{})
	if got.Status == RunSuccess {
		t.Fatal("child ignored expired parent deadline")
	}
}

func TestBlockShutdownCancelsStartedCall(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, call := blockParent(t, s, block, "Parent")
	started, release := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	s.runner.testOnEvent = func(ev RunEvent) {
		if ev.Type == EventStepStarted && ev.NodeID == call && once.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
	}
	run, err := s.StartTestRun(context.Background(), parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("call did not start")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.runner.Shutdown(shutdownCtx) }()
	select {
	case <-s.runner.baseCtx.Done():
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("runner did not cancel")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if got := waitRun(t, s, run.RunID); got.Status != RunCancelled {
		t.Fatalf("shutdown run: %+v", got)
	}
}

func TestBlockShutdownCancelsAdmissionWaitingForDatabase(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	req := StartRequest{Flow: parent.Draft, Revision: parent.DraftRevision, Mode: ModeTest, TriggerNode: parent.Draft.Nodes[0].ID}
	admission := make(chan error, 1)
	go func() { _, err := s.runner.StartContext(ctx, req); admission <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for len(s.runner.blockAdmissions) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(s.runner.blockAdmissions) == 0 {
		tx.Rollback()
		t.Fatal("admission did not begin")
	}
	time.Sleep(100 * time.Millisecond)
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.runner.Shutdown(shutdownCtx); err != nil {
		tx.Rollback()
		t.Fatalf("shutdown blocked by admission: %v", err)
	}
	if err := <-admission; !errors.Is(err, context.Canceled) && !errors.Is(err, ErrRunnerClosed) {
		tx.Rollback()
		t.Fatalf("admission after shutdown: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runner.Start(req); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("start after shutdown: %v", err)
	}
	if err := s.runner.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
	var runs, pins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_run_pins`).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if runs != 0 || pins != 0 {
		t.Fatalf("shutdown admitted run/pins: %d/%d", runs, pins)
	}
}

func TestBlockSharedStoredLogBudgetAcrossCalls(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	b := newFlow("Log budget")
	start := b.node("start", TypeTriggerManual, nil)
	for i := 0; i < 8; i++ {
		call := b.node(fmt.Sprintf("call_%d", i), blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "{{trigger.data.name}}"}})
		b.edge(start, PortOut, call)
	}
	parent, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.StartTestRun(ctx, parent.ID, TestRunRequest{BaseRevision: parent.DraftRevision, TriggerData: map[string]any{"name": strings.Repeat("x", 200<<10)}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		detail, err := s.Run(ctx, run.RunID, false)
		if err == nil && detail.Run.Status.Terminal() {
			if detail.Run.Status != RunSuccess {
				t.Fatalf("run: %+v", detail.Run)
			}
			omitted := false
			for _, step := range detail.Steps {
				if step.Output["_omitted"] == storedOmittedReason {
					omitted = true
				}
			}
			if !omitted {
				t.Fatal("shared child/parent log budget did not omit any parent output")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("log-budget run did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
