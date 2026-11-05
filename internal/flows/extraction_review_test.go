package flows

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestExtractionRollsBackBlockWhenParentWriteFails(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, node := extractionTestFlow(t, s)
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Atomic block"}
	preview, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	// Fail after the new block and version have been inserted, at the parent write.
	if _, err := s.store.db.ExecContext(ctx, `CREATE TRIGGER extraction_parent_failure BEFORE UPDATE OF draft_json ON flows WHEN OLD.kind='flow' BEGIN SELECT RAISE(ABORT, 'injected parent write failure'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyExtraction(ctx, rec.ID, ExtractionApplyRequest{ExtractionRequest: req, IDs: preview.IDs, Digest: preview.Digest})
	if err == nil || result != nil {
		t.Fatalf("failed transaction acknowledged: %+v %v", result, err)
	}
	var blocks, versions int
	if err := s.store.db.QueryRowContext(ctx, `SELECT count(*) FROM flows WHERE id=?`, preview.IDs.BlockID).Scan(&blocks); err != nil {
		t.Fatal(err)
	}
	if err := s.store.db.QueryRowContext(ctx, `SELECT count(*) FROM flow_versions WHERE flow_id=?`, preview.IDs.BlockID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	current, err := s.GetFlow(ctx, rec.ID)
	if err != nil || blocks != 0 || versions != 0 || current.DraftRevision != rec.DraftRevision || !reflect.DeepEqual(current.Draft, rec.Draft) {
		t.Fatalf("partial commit: blocks=%d versions=%d parent=%+v err=%v", blocks, versions, current, err)
	}
	check, err := s.ReconcileExtraction(ctx, rec.ID, ExtractionReconcileRequest{BaseRevision: req.BaseRevision, BlockID: preview.IDs.BlockID, ParentHash: preview.ParentHash, BlockHash: preview.BlockHash})
	if err != nil || check.Status != "unknown_or_conflict" || check.Flow != nil || check.Block != nil {
		t.Fatalf("missing data cannot prove failure or success: %+v %v", check, err)
	}
}

func TestExtractionOccupiedIDNeverOverwritesExistingBlock(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	first, firstNode := extractionTestFlow(t, s)
	firstReq := ExtractionRequest{BaseRevision: first.DraftRevision, NodeIDs: []string{firstNode}, Name: "Existing"}
	p, err := s.PreviewExtraction(ctx, first.ID, firstReq)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := s.ApplyExtraction(ctx, first.ID, ExtractionApplyRequest{ExtractionRequest: firstReq, IDs: p.IDs, Digest: p.Digest})
	if err != nil {
		t.Fatal(err)
	}
	second, secondNode := extractionTestFlow(t, s)
	req := ExtractionRequest{BaseRevision: second.DraftRevision, NodeIDs: []string{secondNode}, Name: "Must not replace"}
	ids := extractionIDs()
	ids.BlockID = existing.Block.ID
	proposal, err := extractionProposalFor(second.Draft, req, ids, s.reg)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash, err := extractionHash(second.Draft)
	if err != nil {
		t.Fatal(err)
	}
	parentHash, err := extractionHash(proposal.parent)
	if err != nil {
		t.Fatal(err)
	}
	blockHash, err := extractionHash(proposal.block)
	if err != nil {
		t.Fatal(err)
	}
	// A digest is a consistency check, not a secret or an authorization token.
	digest, err := extractionDigest(second.ID, req, ids, sourceHash, parentHash, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyExtraction(ctx, second.ID, ExtractionApplyRequest{ExtractionRequest: req, IDs: ids, Digest: digest})
	if !errors.Is(err, ErrExtractionConflict) || result != nil {
		t.Fatalf("occupied ID was accepted: %+v %v", result, err)
	}
	block, err := s.GetBlock(ctx, existing.Block.ID)
	if err != nil || !reflect.DeepEqual(block, existing.Block) {
		t.Fatalf("existing block changed: %+v %v", block, err)
	}
	current, err := s.GetFlow(ctx, second.ID)
	if err != nil || !reflect.DeepEqual(current, second) {
		t.Fatalf("losing parent changed: %+v %v", current, err)
	}
}

func TestExtractionReconcileRefusesLaterBlockDraft(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, node := extractionTestFlow(t, s)
	req := ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{node}, Name: "Original"}
	p, err := s.PreviewExtraction(ctx, rec.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyExtraction(ctx, rec.ID, ExtractionApplyRequest{ExtractionRequest: req, IDs: p.IDs, Digest: p.Digest})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := result.Block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Name = "Later edit"
	if _, _, err := s.SaveBlockDraft(ctx, result.Block.ID, doc, result.Block.DraftRevision); err != nil {
		t.Fatal(err)
	}
	check, err := s.ReconcileExtraction(ctx, rec.ID, ExtractionReconcileRequest{BaseRevision: req.BaseRevision, BlockID: p.IDs.BlockID, ParentHash: p.ParentHash, BlockHash: p.BlockHash})
	if err != nil || check.Status != "unknown_or_conflict" || check.Flow != nil || check.Block != nil {
		t.Fatalf("later block draft claimed as exact acknowledgment: %+v %v", check, err)
	}
}

func TestExtractionRewritePreservesSourceLiteralsAndFilterArguments(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`literal \{{source.value}}`, `literal \{{source.value}}`},
		{`{{ source.value | default("{{fallback}}") }}`, `{{ block_input.data.source.value | default("{{fallback}}") }}`},
		{`prefix \\{{source.value}} / {{source.value}}`, `prefix \\{{source.value}} / {{block_input.data.source.value}}`},
		{`{{inside.value}} / {{source["odd.key"] | default("a\\b")}}`, `{{inside.value}} / {{block_input.data.source["odd.key"] | default("a\\b")}}`},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := rewriteExtractionValue(tt.input, map[string]bool{"inside": true}, map[string]bool{"source": true}, 0)
			if err != nil || got != tt.want {
				t.Fatalf("rewrite: %#v, %v; want %#v", got, err, tt.want)
			}
		})
	}
}
