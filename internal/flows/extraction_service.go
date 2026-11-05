package flows

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
)

func extractionDigest(id string, req ExtractionRequest, ids ExtractionIDs, sourceHash, parentHash, blockHash string) (string, error) {
	return extractionHash(struct {
		Version    int               `json:"version"`
		FlowID     string            `json:"flow_id"`
		Request    ExtractionRequest `json:"request"`
		IDs        ExtractionIDs     `json:"ids"`
		SourceHash string            `json:"source_hash"`
		ParentHash string            `json:"parent_hash"`
		BlockHash  string            `json:"block_hash"`
	}{1, id, req, ids, sourceHash, parentHash, blockHash})
}

func validExtractionHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(hash)
	return err == nil && hex.EncodeToString(decoded) == hash
}

func (s *Service) PreviewExtraction(ctx context.Context, flowID string, req ExtractionRequest) (*ExtractionPreview, error) {
	if !validExtractionRequest(req) {
		return nil, ErrExtractionInvalid
	}
	rec, err := s.requireFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	if rec.DraftRevision != req.BaseRevision {
		return nil, ErrRevisionConflict
	}
	ids := extractionIDs()
	var occupied int
	if err := s.store.db.QueryRowContext(ctx, `SELECT count(*) FROM flows WHERE id=?`, ids.BlockID).Scan(&occupied); err != nil {
		return nil, err
	}
	if occupied != 0 {
		return nil, ErrExtractionConflict
	}
	proposal, err := extractionProposalFor(rec.Draft, req, ids, s.reg)
	if err != nil {
		return nil, err
	}
	// Resolve existing calls in the same revision-checked read snapshot, then
	// measure the proposed new block without inserting it or mutating the plan.
	existing, err := prepareDraftBlockPlan(ctx, s.store, rec.Draft, req.BaseRevision, s.services)
	if err != nil {
		return nil, err
	}
	if len(existing.blocks) >= maxBlockVersions {
		return nil, extractionFailure("budget")
	}
	proposedPlan := &blockPlan{byNode: make(map[string]resolvedBlock, len(existing.byNode)+1)}
	for id, child := range existing.byNode {
		proposedPlan.byNode[id] = child
	}
	proposedPlan.byNode[ids.CallID] = resolvedBlock{Document: proposal.block}
	if err := measureBlockPlan(ctx, proposal.parent, proposedPlan); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, extractionFailure("budget")
	}
	sourceHash, err := extractionHash(rec.Draft)
	if err != nil {
		return nil, err
	}
	parentHash, err := extractionHash(proposal.parent)
	if err != nil {
		return nil, err
	}
	blockHash, err := extractionHash(proposal.block)
	if err != nil {
		return nil, err
	}
	digest, err := extractionDigest(flowID, req, ids, sourceHash, parentHash, blockHash)
	if err != nil {
		return nil, err
	}
	return &ExtractionPreview{BaseRevision: req.BaseRevision, IDs: ids, Digest: digest, ParentHash: parentHash, BlockHash: blockHash, BlockRevision: 1, Review: proposal.review}, nil
}

func (s *Service) ApplyExtraction(ctx context.Context, flowID string, req ExtractionApplyRequest) (*ExtractionApplyResult, error) {
	if !validExtractionRequest(req.ExtractionRequest) || !validExtractionHash(req.Digest) {
		return nil, ErrExtractionInvalid
	}
	rec, err := s.requireFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	if rec.DraftRevision != req.BaseRevision {
		return nil, ErrRevisionConflict
	}
	proposal, err := extractionProposalFor(rec.Draft, req.ExtractionRequest, req.IDs, s.reg)
	if err != nil {
		return nil, err
	}
	sourceHash, err := extractionHash(rec.Draft)
	if err != nil {
		return nil, err
	}
	parentHash, err := extractionHash(proposal.parent)
	if err != nil {
		return nil, err
	}
	blockHash, err := extractionHash(proposal.block)
	if err != nil {
		return nil, err
	}
	want, err := extractionDigest(flowID, req.ExtractionRequest, req.IDs, sourceHash, parentHash, blockHash)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(req.Digest)) != 1 {
		return nil, ErrExtractionConflict
	}
	unlock, err := s.draftLocks.lock(ctx, flowID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	tx, release, err := beginBlockWrite(ctx, s.store.db, flowID)
	if err != nil {
		return nil, err
	}
	defer release()
	defer tx.Rollback()
	current, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, flowID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if current.Kind != KindFlow {
		return nil, ErrWrongFlowKind
	}
	if current.DraftRevision != req.BaseRevision {
		return nil, ErrRevisionConflict
	}
	actual, err := extractionHash(current.Draft)
	if err != nil {
		return nil, err
	}
	if actual != sourceHash {
		return nil, ErrRevisionConflict
	}
	blockBytes, err := proposal.block.Marshal()
	if err != nil {
		return nil, err
	}
	parentBytes, err := proposal.parent.Marshal()
	if err != nil {
		return nil, err
	}
	now := formatTime(s.now())
	insert, err := tx.ExecContext(ctx, `INSERT INTO flows(id,kind,name,description,mission_id,draft_json,draft_revision,live_json,live_revision,published_draft_revision,published_at,created_at,updated_at) VALUES(?, 'block', ?, ?, '', ?, 1, ?, 1, 1, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, proposal.block.ID, proposal.block.Name, proposal.block.Description, string(blockBytes), string(blockBytes), now, now, now)
	if err != nil {
		return nil, err
	}
	n, err := insert.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrExtractionConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO flow_versions(flow_id,revision,json,published_at) VALUES(?,1,?,?)`, proposal.block.ID, string(blockBytes), now); err != nil {
		return nil, err
	}
	if _, err := prepareBlockPlanTx(ctx, tx, proposal.parent, s.services); err != nil {
		return nil, err
	}
	update, err := tx.ExecContext(ctx, `UPDATE flows SET draft_json=?,name=?,description=?,draft_revision=draft_revision+1,updated_at=? WHERE id=? AND kind='flow' AND draft_revision=?`, string(parentBytes), proposal.parent.Name, proposal.parent.Description, now, flowID, req.BaseRevision)
	if err != nil {
		return nil, err
	}
	n, err = update.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrRevisionConflict
	}
	flow, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, flowID))
	if err != nil {
		return nil, err
	}
	block, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, proposal.block.ID))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ExtractionApplyResult{Flow: flow, Block: block}, nil
}

func (s *Service) ReconcileExtraction(ctx context.Context, flowID string, req ExtractionReconcileRequest) (*ExtractionReconcileResult, error) {
	if req.BaseRevision < 1 || !blockFlowID.MatchString(req.BlockID) || !validExtractionHash(req.ParentHash) || !validExtractionHash(req.BlockHash) {
		return nil, ErrExtractionInvalid
	}
	result := &ExtractionReconcileResult{Status: "unknown_or_conflict"}
	err := blockReadSnapshot(ctx, s.store.db, func(tx *sql.Tx) error {
		flow, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, flowID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		block, err := scanFlow(tx.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id=?`, req.BlockID))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if flow.Kind != KindFlow || block.Kind != KindBlock || block.MissionID != "" || flow.DraftRevision != req.BaseRevision+1 || block.DraftRevision != 1 || block.PublishedDraftRevision != 1 || block.LiveRevision != 1 || block.Live == nil {
			return nil
		}
		parentHash, err := extractionHash(flow.Draft)
		if err != nil {
			return err
		}
		blockHash, err := extractionHash(block.Live)
		if err != nil {
			return err
		}
		draftHash, err := extractionHash(block.Draft)
		if err != nil {
			return err
		}
		if parentHash != req.ParentHash || blockHash != req.BlockHash || draftHash != req.BlockHash {
			return nil
		}
		var version string
		if err := tx.QueryRowContext(ctx, `SELECT json FROM flow_versions WHERE flow_id=? AND revision=1`, req.BlockID).Scan(&version); errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		doc, err := ParseFlow([]byte(version))
		if err != nil {
			return nil
		}
		versionHash, err := extractionHash(doc)
		if err != nil {
			return err
		}
		if versionHash != req.BlockHash {
			return nil
		}
		result.Status = "applied_state_confirmed"
		result.Flow = flow
		result.Block = block
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
