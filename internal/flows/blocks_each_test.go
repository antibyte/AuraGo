package flows

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBlockEachItemTraversalBounds(t *testing.T) {
	ctx := context.Background()
	var nested any = "value"
	for i := 0; i < 63; i++ {
		nested = []any{nested}
	}
	if err := checkBatchItems(ctx, []any{nested}); err != nil {
		t.Fatalf("depth 64 must pass: %v", err)
	}
	if err := checkBatchItems(ctx, []any{[]any{nested}}); err == nil {
		t.Fatal("depth 65 must fail")
	}
	if err := checkBatchItems(ctx, []any{make([]any, 199998)}); err != nil {
		t.Fatalf("200000 values must pass: %v", err)
	}
	if err := checkBatchItems(ctx, []any{make([]any, 199999)}); err == nil {
		t.Fatal("200001 values must fail")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if err := checkBatchItems(ctx, []any{cycle}); err == nil {
		t.Fatal("cyclic input must fail")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := checkBatchItems(cancelled, []any{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled traversal: %v", err)
	}
}

func TestBlockEachRequiredConfigurationIsExplicit(t *testing.T) {
	def := blockEachDef()
	for _, name := range []string{"items", "max_items", "concurrency", "on_item_error"} {
		found := false
		for _, param := range def.Params {
			if param.Name == name {
				found = param.Required && param.Default == nil
			}
		}
		if !found {
			t.Errorf("%s must be required without an implicit default", name)
		}
	}
}

func batchParent(t *testing.T, s *Service, block *FlowRecord, items any, limit int, policy string) (*FlowRecord, string) {
	t.Helper()
	b := newFlow("Batch parent")
	start := b.node("start", TypeTriggerManual, nil)
	each := b.node("batch", blockEachType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "items": items, "item_field": "name", "inputs": map[string]any{}, "max_items": float64(limit), "concurrency": float64(1), "on_item_error": policy})
	b.edge(start, PortOut, each)
	rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	return rec, each
}

func batchStep(t *testing.T, sim *SimulationResult, id string) StepRecord {
	t.Helper()
	for _, step := range sim.Steps {
		if step.NodeID == id {
			return step
		}
	}
	t.Fatalf("no batch step: %+v", sim.Steps)
	return StepRecord{}
}

func TestBlockEachCollectAndLiteralData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	items := []any{"Ada", "{{run.started_at}}", float64(42)}
	parent, id := batchParent(t, s, block, items, 4, "collect")
	before, err := parent.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunSuccess {
		t.Fatalf("simulate: %+v %v", sim, err)
	}
	step := batchStep(t, sim, id)
	rows, ok := step.Output["items"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("rows: %+v", step.Output)
	}
	first := rows[0].(map[string]any)
	second := rows[1].(map[string]any)
	third := rows[2].(map[string]any)
	if first["index"] != float64(0) || first["status"] != "success" || first["output"].(map[string]any)["greeting"] != "Hello Ada" {
		t.Fatalf("first: %+v", first)
	}
	if second["output"].(map[string]any)["greeting"] != "Hello {{run.started_at}}" {
		t.Fatalf("literal was evaluated: %+v", second)
	}
	if third["index"] != float64(2) || third["status"] != "error" || third["error_code"] != "FLOW_PARAM_INVALID" {
		t.Fatalf("third: %+v", third)
	}
	if step.Output["count"] != float64(3) || step.Output["succeeded"] != float64(2) || step.Output["failed"] != float64(1) {
		t.Fatalf("counts: %+v", step.Output)
	}
	if !reflect.DeepEqual(parent.Draft, before) {
		t.Fatal("batch mutated parent document")
	}
	if _, err = s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision, Fixtures: map[string]SimulationFixture{id: {Output: map[string]any{}}}}); !errors.Is(err, ErrInvalidSimulationFixture) {
		t.Fatalf("fixture replaced batch: %v", err)
	}
}

func TestBlockEachReferencedListAndRegistryIsolation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, id := batchParent(t, s, block, "{{trigger.data.items}}", 2, "collect")
	def, _ := s.reg.Lookup(blockEachType)
	fake := *def
	called := false
	fake.Execute = func(context.Context, ExecInput) (ExecResult, error) { called = true; return ExecResult{}, nil }
	s.reg.Replace(&fake)
	input := []any{"{{run.started_at}}", "Ada"}
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision, TriggerData: map[string]any{"items": input}})
	if err != nil || sim.Status != RunSuccess || called {
		t.Fatalf("isolated simulation: %+v, %v, fake=%v", sim, err, called)
	}
	rows := batchStep(t, sim, id).Output["items"].([]any)
	if rows[0].(map[string]any)["output"].(map[string]any)["greeting"] != "Hello {{run.started_at}}" || input[0] != "{{run.started_at}}" {
		t.Fatalf("referenced item was re-evaluated or modified: %+v, %+v", rows, input)
	}
}

func TestBlockEachTemplateValidationTreatsLiteralItemsAsData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, id := batchParent(t, s, block, []any{"{{missing.field}}", "{{bad"}, 2, "collect")
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range preview.Issues {
		if issue.NodeID == id && (issue.Code == IssueTemplateUnknownRoot || issue.Code == IssueTemplateSyntax) {
			t.Fatalf("literal item treated as template: %+v", issue)
		}
	}
	if !preview.CanPublish {
		t.Fatalf("literal items refused: %+v", preview.Issues)
	}
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunSuccess {
		t.Fatalf("literal simulation: %+v %v", sim, err)
	}
	rows := batchStep(t, sim, id).Output["items"].([]any)
	if rows[0].(map[string]any)["output"].(map[string]any)["greeting"] != "Hello {{missing.field}}" {
		t.Fatalf("literal changed: %+v", rows)
	}
	changed, _ := parent.Draft.Clone()
	changed.NodeByID(id).Params["items"] = "{{missing.field}}"
	rev, issues, err := s.SaveDraft(ctx, parent.ID, changed, parent.DraftRevision)
	if err != nil {
		t.Fatalf("save expression: %v %+v", err, issues)
	}
	preview, err = s.PublishPreview(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range preview.Issues {
		if issue.NodeID == id && issue.Code == IssueTemplateUnknownRoot {
			found = true
		}
	}
	if !found || preview.CanPublish {
		t.Fatalf("top-level expression not validated, rev=%d: %+v", rev, preview.Issues)
	}
}

func TestBlockEachEmptyListStillChecksSharedInputs(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	doc, _ := block.Live.Clone()
	doc.Nodes[0].Params["fields"] = []any{map[string]any{"name": "name", "type": "string", "required": true}, map[string]any{"name": "prefix", "type": "string", "required": true}}
	contract, err := readBlockContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	node := &Node{Type: blockEachType, TypeVersion: 1, Params: map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "items": []any{}, "item_field": "name", "inputs": map[string]any{"prefix": 42}, "max_items": float64(1), "concurrency": float64(1), "on_item_error": "collect"}}
	ref := resolvedBlock{Document: doc, Contract: contract, Ref: blockRef{FlowID: block.ID, Revision: block.LiveRevision}}
	_, err = executeBlockEach(context.Background(), ExecInput{Node: node, Params: map[string]any{"items": []any{}, "inputs": map[string]any{"prefix": float64(42)}}, Services: &Services{}, block: &ref, scope: &blockRunScope{}})
	var failure *NodeError
	if !errors.As(err, &failure) || failure.Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("invalid shared value accepted: %v", err)
	}
}

func TestBlockEachListLimitsAndStop(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	for _, n := range []int{0, 1, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			items := make([]any, n)
			for i := range items {
				items[i] = "x"
			}
			parent, id := batchParent(t, s, block, items, 1000, "collect")
			if n == 0 {
				preview, err := s.PublishPreview(ctx, parent.ID)
				if err != nil || !preview.CanPublish {
					t.Fatalf("empty list must be publishable: %+v %v", preview, err)
				}
			}
			sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
			if err != nil || sim.Status != RunSuccess {
				t.Fatalf("simulate %d: %+v %v", n, sim, err)
			}
			step := batchStep(t, sim, id)
			if step.Output["count"] != float64(n) {
				t.Fatalf("count %d: %+v", n, step.Output)
			}
		})
	}
	parent, id := batchParent(t, s, block, []any{"ok", float64(9), "never"}, 3, "stop")
	sim, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.Status != RunError || sim.ErrorCode != "FLOW_BATCH_ITEM_FAILED" {
		t.Fatalf("stop: %+v %v", sim, err)
	}
	step := batchStep(t, sim, id)
	if step.ErrorMessage != "batch item 1 failed (FLOW_PARAM_INVALID)" || step.Output != nil {
		t.Fatalf("stop step: %+v", step)
	}
	tooMany := make([]any, 1001)
	parent, id = batchParent(t, s, block, tooMany, 1000, "collect")
	sim, err = s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || sim.ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("over limit: %+v %v", sim, err)
	}
	if step = batchStep(t, sim, id); step.ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("over limit step: %+v", step)
	}
}

func TestBlockEachResourceFailureNotCollected(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	items := make([]any, 20)
	large := strings.Repeat("x", 3<<20)
	for i := range items {
		items[i] = large
	}
	// The document stays small: inputs come from an upstream expression instead.
	parent, id := batchParent(t, s, block, "{{trigger.data.items}}", 20, "collect")
	plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
	if err != nil {
		t.Fatal(err)
	}
	scope := &blockRunScope{outputBytes: MaxRunOutputBytes - 64}
	node := parent.Draft.NodeByID(id)
	snapshot := plan.byNode[id]
	_, err = executeBlockEach(ctx, ExecInput{Node: node, Params: map[string]any{"items": items, "inputs": map[string]any{}}, Services: &Services{}, block: &snapshot, scope: scope})
	var failure *NodeError
	if !errors.As(err, &failure) || failure.Code != "FLOW_RUN_OUTPUT_TOO_LARGE" {
		t.Fatalf("resource error collected: %v", err)
	}
}

func TestBlockEachAggregateOutputIsBoundedBeforeReturn(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, id := batchParent(t, s, block, "{{trigger.data.items}}", 3, "collect")
	plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
	if err != nil {
		t.Fatal(err)
	}
	item := strings.Repeat("x", 2<<20)
	snapshot := plan.byNode[id]
	_, err = executeBlockEach(ctx, ExecInput{Node: parent.Draft.NodeByID(id), Params: map[string]any{"items": []any{item, item, item}, "inputs": map[string]any{}}, Services: &Services{}, block: &snapshot, scope: &blockRunScope{}})
	var failure *NodeError
	if !errors.As(err, &failure) || failure.Code != "FLOW_OUTPUT_TOO_LARGE" {
		t.Fatalf("aggregate limit: %v", err)
	}
}

func TestBlockEachCancelledBeforeFirstItem(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, id := batchParent(t, s, block, []any{"x"}, 1, "collect")
	plan, err := prepareBlockPlan(context.Background(), s.store, parent.Draft, s.services)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot := plan.byNode[id]
	scope := &blockRunScope{}
	_, err = executeBlockEach(ctx, ExecInput{Node: parent.Draft.NodeByID(id), Params: map[string]any{"items": []any{"x"}, "inputs": map[string]any{}}, Services: &Services{}, block: &snapshot, scope: scope})
	if !errors.Is(err, context.Canceled) || scope.outputBytes != 0 {
		t.Fatalf("cancel was collected or ran child: %v, %d", err, scope.outputBytes)
	}
}

func TestBlockInputCombinedValuesNeverSilentlyReplaceData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	doc, err := block.Live.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[0].Params["fields"] = []any{
		map[string]any{"name": "name", "type": "string", "required": true},
		map[string]any{"name": "shared", "type": "string", "required": true},
	}
	contract, err := readBlockContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	ref := resolvedBlock{Document: doc, Contract: contract, Ref: blockRef{FlowID: block.ID, Revision: block.LiveRevision}}
	large := strings.Repeat("x", 3<<20)
	inputs := map[string]any{"name": large, "shared": large}
	child := executePureBlock(context.Background(), doc, inputs, &Services{}, discardLogger(), ModeSimulation, block.LiveRevision, "", &blockRunScope{})
	if child.Status != RunError || child.ErrorCode != "FLOW_OUTPUT_TOO_LARGE" || len(child.Steps) != 0 {
		t.Fatalf("direct block silently replaced data: %+v", child)
	}
	scope := &blockRunScope{}
	_, err = blockCallDef().Execute(context.Background(), ExecInput{Params: map[string]any{"inputs": inputs}, Services: &Services{}, block: &ref, scope: scope})
	var failure *NodeError
	if !errors.As(err, &failure) || failure.Code != "FLOW_OUTPUT_TOO_LARGE" || scope.outputBytes != 0 {
		t.Fatalf("call silently replaced data: %v, bytes=%d", err, scope.outputBytes)
	}
	node := &Node{Type: blockEachType, TypeVersion: 1, Params: map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "items": []any{large}, "item_field": "name", "inputs": map[string]any{"shared": large}, "max_items": float64(1), "concurrency": float64(1), "on_item_error": "collect"}}
	scope = &blockRunScope{}
	_, err = executeBlockEach(context.Background(), ExecInput{Node: node, Params: map[string]any{"items": []any{large}, "inputs": map[string]any{"shared": large}}, Services: &Services{}, block: &ref, scope: scope})
	if !errors.As(err, &failure) || failure.Code != "FLOW_OUTPUT_TOO_LARGE" || scope.outputBytes != 0 {
		t.Fatalf("batch collected oversized input: %v, bytes=%d", err, scope.outputBytes)
	}
}

func TestBlockEachWorkBudgetAndPreviewIssue(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	child, _ := block.Draft.Clone()
	child.Nodes[1].Settings.Retry.Count = MaxRetryCount
	child.Nodes = append(child.Nodes, Node{ID: NewNodeID(), Key: "unused", Type: TypeSet, TypeVersion: 1, Settings: NodeSettings{Disabled: true, Retry: RetryPolicy{Count: MaxRetryCount}}, Params: map[string]any{"fields": []any{}}})
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, child, block.DraftRevision)
	if err != nil {
		t.Fatalf("save %v %+v", err, issues)
	}
	block, issues, err = s.PublishBlock(ctx, block.ID, rev)
	if err != nil {
		t.Fatalf("publish %v %+v", err, issues)
	}
	parent, id := batchParent(t, s, block, []any{}, 1000, "collect")
	if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); !errors.Is(err, ErrBatchWorkLimit) {
		t.Fatalf("missing work refusal: %v", err)
	}
	preview, err := s.PublishPreview(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, issue := range preview.Issues {
		if issue.Code == IssueBatchWorkLimit && issue.NodeID == id && issue.Param == "max_items" {
			found = true
		}
	}
	if !found || preview.CanPublish {
		t.Fatalf("missing actionable issue: %+v", preview)
	}
	_, publishIssues, err := s.Publish(ctx, parent.ID, parent.DraftRevision)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("publish must return validation issues: %v", err)
	}
	found = false
	for _, issue := range publishIssues {
		if issue.Code == IssueBatchWorkLimit && issue.NodeID == id && issue.Param == "max_items" {
			found = true
			for _, previewIssue := range preview.Issues {
				if previewIssue.Code == IssueBatchWorkLimit && !reflect.DeepEqual(previewIssue, issue) {
					t.Fatalf("publish issue differs from preview: %+v / %+v", issue, previewIssue)
				}
			}
		}
	}
	if !found || !reflect.DeepEqual(publishIssues, validation.Issues) {
		t.Fatalf("publish missing actionable issue: %+v / %+v", publishIssues, validation)
	}
}
