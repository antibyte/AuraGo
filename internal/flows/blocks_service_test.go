package flows

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBlockServiceLifecycleAndKindBoundary(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Reusable")
	if err != nil {
		t.Fatal(err)
	}
	if block.Kind != KindBlock || block.MissionID != "" || block.Published() {
		t.Fatalf("created block: %+v", block)
	}
	if len(bridge.startedRuns()) != 0 {
		t.Fatal("block creation started a mission")
	}
	if _, err := s.GetFlow(ctx, block.ID); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("flow get block: %v", err)
	}
	if _, err := s.PublishPreview(ctx, block.ID); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("flow preview block: %v", err)
	}
	if _, err := s.Simulate(ctx, block.ID, SimulationRequest{}); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("flow simulation block: %v", err)
	}
	if _, _, err := s.SaveDraft(ctx, block.ID, block.Draft, block.DraftRevision); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("flow save block: %v", err)
	}
	preview, err := s.BlockPublishPreview(ctx, block.ID)
	if err != nil || !preview.CanPublish || preview.DraftRevision != block.DraftRevision {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	pub, issues, err := s.PublishBlock(ctx, block.ID, block.DraftRevision)
	if err != nil {
		t.Fatalf("publish block: %v, %+v", err, issues)
	}
	if pub.LiveRevision != 1 || pub.MissionID != "" {
		t.Fatalf("published block: %+v", pub)
	}
	version, err := s.GetBlockVersion(ctx, block.ID, 1)
	if err != nil || version.Doc.ID != block.ID || len(version.Contract.Inputs) != 0 || len(version.Contract.Outputs) != 0 {
		t.Fatalf("version: %+v, %v", version, err)
	}
	page, err := s.ListBlocks(ctx, "", 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != block.ID || !page.Items[0].Published {
		t.Fatalf("list: %+v, %v", page, err)
	}
	flow, err := s.CreateFlow(ctx, CreateRequest{Name: "Regular"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBlock(ctx, flow.ID); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("block get flow: %v", err)
	}
	if _, _, err := s.SaveBlockDraft(ctx, flow.ID, flow.Draft, flow.DraftRevision); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("block save flow: %v", err)
	}
	if err := s.DeleteBlock(ctx, flow.ID, flow.DraftRevision, flow.LiveRevision); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("block delete flow: %v", err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision+1, pub.LiveRevision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, pub.LiveRevision); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestBlockSimulationTypedIsolatedAndRevisionBound(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
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
	doc.Nodes[1].Params["values"] = map[string]any{"greeting": "{{block_input.data.name}}"}
	step := Node{ID: NewNodeID(), Key: "middle", Type: TypeSet, TypeVersion: 1, Params: map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "safe"}}}}
	doc.Nodes = append(doc.Nodes, step)
	doc.Edges[0].Target.Node = step.ID
	doc.Edges = append(doc.Edges, Edge{ID: NewEdgeID(), Source: PortRef{Node: step.ID, Port: PortOut}, Target: PortRef{Node: doc.Nodes[1].ID, Port: PortIn}})
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v, %+v", err, issues)
	}
	var calls atomic.Int32
	def, _ := s.reg.Lookup(TypeSet)
	forged := *def
	forged.Execute = func(context.Context, ExecInput) (ExecResult, error) { calls.Add(1); return ExecResult{}, nil }
	s.reg.Replace(&forged)
	spy := &fakeTools{}
	s.services.Tools = spy
	at := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	res, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev, Inputs: map[string]any{"name": "Ada"}, At: at})
	if err != nil || res.Status != RunSuccess || res.Outputs["greeting"] != "Ada" || !res.StartedAt.Equal(at) {
		t.Fatalf("simulation: %+v, %v", res, err)
	}
	if calls.Load() != 0 || spy.count() != 0 || len(bridge.startedRuns()) != 0 {
		t.Fatalf("production callback executed: %d/%d", calls.Load(), spy.count())
	}
	if _, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev + 1}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("revision: %v", err)
	}
	if _, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev, Inputs: map[string]any{"name": 1}}); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("type: %v", err)
	}
	if _, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev, Inputs: map[string]any{"name": "Ada", "extra": true}}); !errors.Is(err, ErrBlockInvalid) {
		t.Fatalf("unknown input: %v", err)
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, block.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stored runs: %d, %v", count, err)
	}
}

func TestBlockSimulationEmptyOutputDoesNotRequireValues(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Empty")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := block.Draft.Clone()
	delete(doc.Nodes[1].Params, "values")
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	if _, issues, err := s.PublishBlock(ctx, block.ID, rev); err != nil {
		t.Fatalf("publish: %v %+v", err, issues)
	}
	result, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev})
	if err != nil || result.Status != RunSuccess || result.Outputs == nil || len(result.Outputs) != 0 {
		t.Fatalf("simulation: %+v, %v", result, err)
	}
}

func TestBlockServiceRejectsImpureAndCorruptDocuments(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Reusable")
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{TypeWait, TypeStop, TypeHTTPRequest, blockCallType, blockEachType} {
		doc, err := block.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: "forbidden", Type: typ, TypeVersion: 1, Settings: NodeSettings{Disabled: true}})
		if _, _, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision); err == nil {
			t.Fatalf("accepted disabled %s", typ)
		}
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE flows SET mission_id='foreign' WHERE id=?`, block.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBlock(ctx, block.ID); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("corrupt mission read: %v", err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("corrupt mission delete: %v", err)
	}
	if _, err := s.ListBlocks(ctx, "", 10); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("corrupt mission list: %v", err)
	}
}
