package flows

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func extractionTestFlow(t *testing.T, s *Service) (*FlowRecord, string) {
	t.Helper()
	b := newFlow("Extractable")
	start := b.node("start", TypeTriggerManual, nil)
	value := b.node("result", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "greeting", "type": "text", "value": "Hello {{trigger.data.name | upper}}"}}})
	b.edge(start, PortOut, value)
	rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	return rec, value
}

func TestExtractionPreviewApplyAndReconcile(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, node := extractionTestFlow(t, s)
	before, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || before.Status != RunSuccess {
		t.Fatalf("before: %+v %v", before, err)
	}
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Greeting"}
	preview, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	apply := ExtractionApplyRequest{ExtractionRequest: req, IDs: preview.IDs, Digest: preview.Digest}
	result, err := s.ApplyExtraction(ctx, rec.ID, apply)
	if err != nil {
		t.Fatal(err)
	}
	if result.Flow.DraftRevision != rec.DraftRevision+1 || result.Block.LiveRevision != 1 {
		t.Fatalf("wrong revisions: %+v", result)
	}
	input, extracted, output := result.Block.Draft.Nodes[0], result.Block.Draft.Nodes[1], result.Block.Draft.Nodes[2]
	if !(input.Position.X < extracted.Position.X && output.Position.X > extracted.Position.X) {
		t.Fatalf("overlapping block boundary positions: %+v %+v %+v", input.Position, extracted.Position, output.Position)
	}
	after, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: result.Flow.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || after.Status != RunSuccess {
		t.Fatalf("after: %+v %v", after, err)
	}
	if before.Steps[len(before.Steps)-1].Output["greeting"] != after.Steps[len(after.Steps)-1].Output["greeting"] {
		t.Fatalf("result changed: %+v %+v", before.Steps, after.Steps)
	}
	check := ExtractionReconcileRequest{BaseRevision: req.BaseRevision, BlockID: preview.IDs.BlockID, ParentHash: preview.ParentHash, BlockHash: preview.BlockHash}
	confirmed, err := s.ReconcileExtraction(ctx, rec.ID, check)
	if err != nil || confirmed.Status != "applied_state_confirmed" || confirmed.Flow == nil || confirmed.Block == nil {
		t.Fatalf("reconcile: %+v %v", confirmed, err)
	}
	if _, err := s.ApplyExtraction(ctx, rec.ID, apply); !errors.Is(err, ErrRevisionConflict) && !errors.Is(err, ErrExtractionConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	doc, _ := result.Flow.Draft.Clone()
	doc.Name = "Changed"
	if _, _, err := s.SaveDraft(ctx, rec.ID, doc, result.Flow.DraftRevision); err != nil {
		t.Fatal(err)
	}
	unknown, err := s.ReconcileExtraction(ctx, rec.ID, check)
	if err != nil || unknown.Status != "unknown_or_conflict" || unknown.Flow != nil || unknown.Block != nil {
		t.Fatalf("changed parent: %+v %v", unknown, err)
	}
}

func TestExtractionRejectsUnsupportedAndTampering(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, node := extractionTestFlow(t, s)
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Block"}
	preview, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	bad := ExtractionApplyRequest{ExtractionRequest: req, IDs: preview.IDs, Digest: preview.Digest}
	bad.Name = "Another"
	if _, err := s.ApplyExtraction(ctx, rec.ID, bad); !errors.Is(err, ErrExtractionConflict) {
		t.Fatalf("tamper: %v", err)
	}
	current, err := s.GetFlow(ctx, rec.ID)
	if err != nil || current.DraftRevision != rec.DraftRevision {
		t.Fatalf("tampering changed draft: %+v %v", current, err)
	}
	doc, _ := rec.Draft.Clone()
	doc.NodeByID(node).Params["keep_input"] = true
	rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	req.BaseRevision = rev
	_, err = s.PreviewExtraction(ctx, rec.ID, req)
	var why *ExtractionError
	if !errors.As(err, &why) || why.Reason != "node" {
		t.Fatalf("keep_input accepted: %v", err)
	}
}

func TestExtractionNameControlCharacters(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	rec, node := extractionTestFlow(t, s)
	for _, name := range []string{"ok\x00bad", "ok\nbad", "ok\x7fbad", "ok\u0085bad"} {
		_, err := s.PreviewExtraction(context.Background(), rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: name})
		if !errors.Is(err, ErrExtractionInvalid) {
			t.Errorf("control name accepted: %q: %v", name, err)
		}
	}
	if _, err := s.PreviewExtraction(context.Background(), rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Begrüßung 東京"}); err != nil {
		t.Fatalf("valid Unicode name rejected: %v", err)
	}
}

func TestExtractionLinearChainWithSuccessor(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Chain")
	start := b.node("start", TypeTriggerManual, nil)
	field := func(value string) map[string]any {
		return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": value}}}
	}
	first := b.node("first", TypeSet, field("{{trigger.data.name}}"))
	second := b.node("second", TypeSet, field("{{first.text | upper}}"))
	after := b.node("after", TypeSet, field("{{second.text}}"))
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, second)
	b.edge(second, PortOut, after)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || before.Status != RunSuccess {
		t.Fatalf("before: %+v %v", before, err)
	}
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{first, second}, Name: "Chain"}
	p, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyExtraction(ctx, rec.ID, ExtractionApplyRequest{ExtractionRequest: req, IDs: p.IDs, Digest: p.Digest})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: result.Flow.DraftRevision, TriggerData: map[string]any{"name": "Ada"}})
	if err != nil || run.Status != RunSuccess {
		t.Fatalf("after: %+v %v", run, err)
	}
	if got := run.Steps[len(run.Steps)-1].Output["text"]; got != "ADA" {
		t.Fatalf("wrong output %v", got)
	}
}

func TestExtractionMovesCompleteGroupsAndShrinksPartialGroups(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Groups")
	start := b.node("start", TypeTriggerManual, nil)
	params := func(value string) map[string]any {
		return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": value}}}
	}
	first := b.node("first", TypeSet, params("{{trigger.data.name}}"))
	second := b.node("second", TypeSet, params("{{first.text}}"))
	after := b.node("after", TypeSet, params("{{second.text}}"))
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, second)
	b.edge(second, PortOut, after)
	doc := b.build()
	groups := []VisualGroup{{ID: "g_aaaaaaaa", Title: "All", Color: "blue", Members: []string{first}}, {ID: "g_bbbbbbbb", Title: "Part", Color: "green", Members: []string{second, after}}}
	doc.Groups = &groups
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{first, second}, Name: "Grouped"}
	p, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Review.GroupsMoved != 1 || p.Review.GroupsShrunk != 1 {
		t.Fatalf("groups review: %+v", p.Review)
	}
	out, err := s.ApplyExtraction(ctx, rec.ID, ExtractionApplyRequest{ExtractionRequest: req, IDs: p.IDs, Digest: p.Digest})
	if err != nil {
		t.Fatal(err)
	}
	if out.Flow.Draft.Groups == nil || len(*out.Flow.Draft.Groups) != 1 || len((*out.Flow.Draft.Groups)[0].Members) != 1 || (*out.Flow.Draft.Groups)[0].Members[0] != after {
		t.Fatalf("parent groups: %+v", out.Flow.Draft.Groups)
	}
	if out.Block.Draft.Groups == nil || len(*out.Block.Draft.Groups) != 1 || (*out.Block.Draft.Groups)[0].ID != "g_aaaaaaaa" {
		t.Fatalf("block groups: %+v", out.Block.Draft.Groups)
	}
}

func TestExtractionRejectsUnsafeControlPort(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, node := extractionTestFlow(t, s)
	doc, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	// A draft can carry an invalid control port; extraction must never rewrite it.
	doc.Edges[0].Target.Port = "other"
	_, err = extractionProposalFor(doc, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Unsafe"}, extractionIDs(), s.reg)
	var why *ExtractionError
	if !errors.As(err, &why) || why.Reason != "boundary" {
		t.Fatalf("invalid port accepted: %v", err)
	}
	_ = ctx
}

func TestExtractionPreviewExpandedBudget(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Existing")
	if err != nil {
		t.Fatal(err)
	}
	block, _, err = s.PublishBlock(ctx, block.ID, block.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		nodeCount int
		wantError bool
	}{{495, false}, {496, true}} {
		t.Run(fmt.Sprint(tc.nodeCount), func(t *testing.T) {
			b := newFlow("Budget")
			start := b.node("start", TypeTriggerManual, nil)
			selected := b.node("result", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "text", "type": "text", "value": "{{trigger.data.name}}"}}})
			b.edge(start, PortOut, selected)
			b.node("existing", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{}})
			for i := 3; i < tc.nodeCount; i++ {
				b.node(fmt.Sprintf("unused_%d", i), TypeSet, map[string]any{"fields": []any{map[string]any{"name": "text", "type": "text", "value": "x"}}})
			}
			rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.PreviewExtraction(ctx, rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{selected}, Name: "Extracted"})
			if tc.wantError {
				var reason *ExtractionError
				if !errors.As(err, &reason) || reason.Reason != "budget" {
					t.Fatalf("expected budget refusal, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("boundary rejected: %v", err)
			}
		})
	}
}

func TestExtractionPreviewExpandedEdgeBudget(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Existing")
	if err != nil {
		t.Fatal(err)
	}
	block, _, err = s.PublishBlock(ctx, block.ID, block.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		edges     int
		wantError bool
	}{{1997, false}, {1998, true}} {
		t.Run(fmt.Sprint(tc.edges), func(t *testing.T) {
			b := newFlow("Edge budget")
			start := b.node("start", TypeTriggerManual, nil)
			selected := b.node("result", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "text", "type": "text", "value": "{{trigger.data.name}}"}}})
			b.edge(start, PortOut, selected)
			b.node("existing", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{}})
			var other []string
			for i := 0; i < 64; i++ {
				other = append(other, b.node(fmt.Sprintf("other_%d", i), TypeSet, map[string]any{"fields": []any{map[string]any{"name": "text", "type": "text", "value": "x"}}}))
			}
			count := 1
			for i := 0; i < len(other) && count < tc.edges; i++ {
				for j := i + 1; j < len(other) && count < tc.edges; j++ {
					b.edge(other[i], PortOut, other[j])
					count++
				}
			}
			if count != tc.edges {
				t.Fatal("insufficient DAG edges")
			}
			rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.PreviewExtraction(ctx, rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{selected}, Name: "Extracted"})
			if tc.wantError {
				var reason *ExtractionError
				if !errors.As(err, &reason) || reason.Reason != "budget" {
					t.Fatalf("expected edge budget refusal: %v", err)
				}
			} else if err != nil {
				t.Fatalf("edge boundary rejected: %v", err)
			}
		})
	}
}

func TestExtractionRefusesTransitiveTaintChangeAtFencedSink(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Fenced sink")
	start := b.node("start", TypeTriggerManual, nil)
	field := func(value string) map[string]any {
		return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": value}}}
	}
	first := b.node("first", TypeSet, field("{{trigger.data.name}}"))
	after := b.node("after", TypeSet, field("{{first.text}}"))
	ask := b.node("ask", TypeAIStep, map[string]any{"prompt": "{{after.text}}"})
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, after)
	b.edge(after, PortOut, ask)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PreviewExtraction(ctx, rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{first}, Name: "Unsafe"})
	var reason *ExtractionError
	if !errors.As(err, &reason) || reason.Reason != "taint" {
		t.Fatalf("changed downstream fence accepted: %v", err)
	}
}

func TestExtractionRefusesDistantInputWithoutDeliveryProof(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Distant source")
	field := func(v string) map[string]any {
		return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": v}}}
	}
	start := b.node("start", TypeTriggerManual, nil)
	first := b.node("first", TypeSet, field("x"))
	next := b.node("next", TypeSet, field("y"))
	last := b.node("last", TypeSet, field("{{first.text}}"))
	b.edge(start, PortOut, first)
	b.edge(first, PortOut, next)
	b.edge(next, PortOut, last)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PreviewExtraction(ctx, rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{last}, Name: "Safe"})
	var reason *ExtractionError
	if !errors.As(err, &reason) || reason.Reason != "reference" {
		t.Fatalf("unproved ancestor accepted: %v", err)
	}
}

func TestExtractionRefusesSkippedAncestorAcrossBranch(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Skipped ancestor")
	start := b.node("start", TypeTriggerManual, nil)
	branch := b.node("branch", TypeIf, map[string]any{"condition": cond(1.0, "eq", 2.0)})
	field := func(v string) map[string]any {
		return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": v}}}
	}
	skipped := b.node("skipped", TypeSet, field("should not run"))
	join := b.node("join", TypeMerge, nil)
	selected := b.node("result", TypeSet, field("{{skipped.text}}"))
	b.edge(start, PortOut, branch)
	b.edge(branch, PortTrue, skipped)
	b.edge(skipped, PortOut, join)
	b.edge(branch, PortFalse, join)
	b.edge(join, PortOut, selected)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision})
	if err != nil || run.Status != RunSuccess {
		t.Fatalf("original run: %+v %v", run, err)
	}
	var skippedSeen bool
	for _, step := range run.Steps {
		if step.NodeID == skipped && step.Status == StepSkipped {
			skippedSeen = true
		}
	}
	if !skippedSeen {
		t.Fatalf("original predecessor was not skipped: %+v", run.Steps)
	}
	_, err = s.PreviewExtraction(ctx, rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{selected}, Name: "Refuse"})
	var reason *ExtractionError
	if !errors.As(err, &reason) || reason.Reason != "reference" {
		t.Fatalf("skipped ancestor accepted: %v", err)
	}
}
