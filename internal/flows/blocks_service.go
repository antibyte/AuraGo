package flows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

// ErrBlockInUse means a published parent or retained run pins this block.
var ErrBlockInUse = errors.New("the block is used by a published flow or retained run")

// ErrBlockInvalid denotes a malformed or out-of-bounds block request.
var ErrBlockInvalid = errors.New("invalid block request")

// BlockSummary contains bounded listing metadata, never a block document.
type BlockSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	DraftRevision int    `json:"draft_revision"`
	LiveRevision  int    `json:"live_revision"`
	Published     bool   `json:"published"`
	UpdatedAt     string `json:"updated_at"`
}

// BlockList is a bounded keyset page ordered by block ID.
type BlockList struct {
	Items      []BlockSummary `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// CreateBlock creates a mission-free, unpublished block with empty boundaries.
func (s *Service) CreateBlock(ctx context.Context, name string) (*FlowRecord, error) {
	in, out := NewNodeID(), NewNodeID()
	f := &Flow{Schema: SchemaVersion, ID: NewFlowID(), Kind: KindBlock, Name: strings.TrimSpace(name),
		Nodes: []Node{
			{ID: in, Key: "block_input", Type: blockInputType, TypeVersion: 1, Params: map[string]any{"fields": []any{}}},
			{ID: out, Key: "block_output", Type: blockOutputType, TypeVersion: 1, Position: Point{X: 320}, Params: map[string]any{"fields": []any{}, "values": map[string]any{}}},
		}, Edges: []Edge{{ID: NewEdgeID(), Source: PortRef{Node: in, Port: PortOut}, Target: PortRef{Node: out, Port: PortIn}}}}
	f.Normalize()
	if issues := s.ValidateBlock(f, ModeDraft); HasErrors(issues) {
		return nil, &ValidationError{Issues: issues}
	}
	return s.store.CreateFlow(ctx, f, "", s.now())
}

// ListBlocks returns reusable blocks, separate from ordinary flow listings.
func (s *Service) ListBlocks(ctx context.Context, cursor string, limit int) (*BlockList, error) {
	if limit < 1 || limit > 100 || (cursor != "" && !blockFlowID.MatchString(cursor)) {
		return nil, ErrBlockInvalid
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT substr(id,1,65),substr(name,1,201),substr(description,1,501),draft_revision,live_revision,live_json<>'',updated_at,
		mission_id='',
		CASE WHEN length(CAST(draft_json AS BLOB))<=? AND json_valid(draft_json)
			THEN COALESCE(json_type(draft_json,'$.id')='text' AND json_extract(draft_json,'$.id')=id AND json_extract(draft_json,'$.kind')='block' AND json_type(draft_json,'$.schema')='integer' AND json_extract(draft_json,'$.schema')=?,0) ELSE 0 END,
		CASE WHEN live_json='' THEN 1 WHEN length(CAST(live_json AS BLOB))<=? AND json_valid(live_json)
			THEN COALESCE(json_type(live_json,'$.id')='text' AND json_extract(live_json,'$.id')=id AND json_extract(live_json,'$.kind')='block' AND json_type(live_json,'$.schema')='integer' AND json_extract(live_json,'$.schema')=?,0) ELSE 0 END
		FROM flows WHERE kind=? AND id>? ORDER BY id LIMIT ?`, MaxDocumentBytes, SchemaVersion, MaxDocumentBytes, SchemaVersion, string(KindBlock), cursor, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &BlockList{Items: []BlockSummary{}}
	for rows.Next() {
		var item BlockSummary
		var noMission, draftValid, liveValid bool
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.DraftRevision, &item.LiveRevision, &item.Published, &item.UpdatedAt, &noMission, &draftValid, &liveValid); err != nil {
			return nil, err
		}
		if !noMission || !draftValid || !liveValid {
			return nil, ErrWrongFlowKind
		}
		if !blockFlowID.MatchString(item.ID) {
			return nil, ErrBlockInvalid
		}
		if len(page.Items) == limit {
			page.NextCursor = page.Items[len(page.Items)-1].ID
			break
		}
		item.Name, item.Description = truncateRunes(item.Name, 200), truncateRunes(item.Description, 500)
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return page, nil
}

// GetBlock reads only a reusable-block record.
func (s *Service) GetBlock(ctx context.Context, id string) (*FlowRecord, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.Kind != KindBlock || rec.MissionID != "" {
		return nil, ErrWrongFlowKind
	}
	return rec, nil
}

// SaveBlockDraft changes a block only when its kind and draft revision match.
func (s *Service) SaveBlockDraft(ctx context.Context, id string, doc *Flow, baseRevision int) (int, []Issue, error) {
	if doc == nil {
		return 0, nil, errors.New("a block document is required")
	}
	if doc.Kind != KindBlock {
		return 0, nil, ErrWrongFlowKind
	}
	f, err := doc.Clone()
	if err != nil {
		return 0, nil, err
	}
	f.ID, f.Kind = id, KindBlock
	f.Normalize()
	issues := s.ValidateBlock(f, ModeDraft)
	if HasErrors(issues) {
		return 0, issues, &ValidationError{Issues: issues}
	}
	unlock, err := s.draftLocks.lock(ctx, id)
	if err != nil {
		return 0, issues, err
	}
	defer unlock()
	rev, err := s.store.SaveDraft(ctx, id, f, baseRevision, s.now())
	return rev, issues, err
}

// BlockPreview pairs publish checks and the optional contract of the same draft.
type BlockPreview struct {
	PublishPreview
	Contract *BlockContract `json:"contract,omitempty"`
}

// BlockPublishPreview describes the exact draft revision, without publishing it.
func (s *Service) BlockPublishPreview(ctx context.Context, id string) (*BlockPreview, error) {
	rec, err := s.GetBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	issues := s.ValidateBlock(rec.Draft, ModePublish)
	if issues == nil {
		issues = []Issue{}
	}
	contract, _ := BlockDraftContract(rec.Draft)
	return &BlockPreview{PublishPreview: PublishPreview{DraftRevision: rec.DraftRevision, Issues: issues, Effects: []EffectSummary{}, Diff: DiffFlows(rec.Live, rec.Draft), CanPublish: !HasErrors(issues)}, Contract: contract}, nil
}

// PublishBlock publishes a validated block without creating a mission or timer.
func (s *Service) PublishBlock(ctx context.Context, id string, baseRevision int) (*FlowRecord, []Issue, error) {
	unlock, err := s.locks.lock(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	defer unlock()
	rec, err := s.GetBlock(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if rec.DraftRevision != baseRevision {
		return nil, nil, ErrRevisionConflict
	}
	issues := s.ValidateBlock(rec.Draft, ModePublish)
	if HasErrors(issues) {
		return nil, issues, &ValidationError{Issues: issues}
	}
	result, err := s.store.publishKind(ctx, id, baseRevision, s.now(), KindBlock)
	return result, issues, err
}

// BlockVersionPage contains a bounded page of published version metadata.
type BlockVersionPage struct {
	Items      []FlowVersionSummary `json:"items"`
	NextCursor int                  `json:"next_cursor,omitempty"`
}

// BlockVersions lists immutable revisions newest first, including pinned old versions.
func (s *Service) BlockVersions(ctx context.Context, id string, beforeRevision, limit int) (*BlockVersionPage, error) {
	if beforeRevision < 0 || beforeRevision > math.MaxInt32 || limit < 1 || limit > 100 {
		return nil, ErrBlockInvalid
	}
	if _, err := s.GetBlock(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT revision,published_at,
		CASE WHEN length(CAST(json AS BLOB))<=? AND json_valid(json) THEN
			COALESCE(json_type(json,'$.id')='text' AND json_extract(json,'$.id')=flow_id AND json_extract(json,'$.kind')='block'
			AND json_type(json,'$.schema')='integer' AND json_extract(json,'$.schema')=?,0) ELSE 0 END,
		CASE WHEN length(CAST(json AS BLOB))<=? AND json_valid(json) THEN COALESCE(json_array_length(json,'$.nodes'),0) ELSE 0 END,
		CASE WHEN length(CAST(json AS BLOB))<=? AND json_valid(json) THEN COALESCE(json_array_length(json,'$.edges'),0) ELSE 0 END
		FROM flow_versions WHERE flow_id=? AND (?=0 OR revision<?) ORDER BY revision DESC LIMIT ?`,
		MaxDocumentBytes, SchemaVersion, MaxDocumentBytes, MaxDocumentBytes, id, beforeRevision, beforeRevision, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &BlockVersionPage{Items: []FlowVersionSummary{}}
	for rows.Next() {
		var item FlowVersionSummary
		var valid bool
		var published string
		if err := rows.Scan(&item.Revision, &published, &valid, &item.NodeCount, &item.EdgeCount); err != nil {
			return nil, err
		}
		if !valid || item.Revision <= 0 || item.Revision > math.MaxInt32 || item.NodeCount > MaxNodes || item.EdgeCount > MaxEdges {
			return nil, ErrBlockInvalid
		}
		if len(page.Items) == limit {
			page.NextCursor = page.Items[len(page.Items)-1].Revision
			break
		}
		item.PublishedAt = parseTime(published)
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return page, nil
}

// GetBlockVersion returns the exact published document and checked boundary.
func (s *Service) GetBlockVersion(ctx context.Context, id string, revision int) (*BlockVersion, error) {
	if _, err := s.GetBlock(ctx, id); err != nil {
		return nil, err
	}
	doc, err := s.store.GetVersion(ctx, id, revision)
	if err != nil {
		return nil, err
	}
	if doc.ID != id || !blockDocumentAllowed(doc) {
		return nil, ErrWrongFlowKind
	}
	contract, err := readBlockContract(doc)
	if err != nil {
		return nil, err
	}
	if issues := s.ValidateBlock(doc, ModePublish); HasErrors(issues) {
		return nil, &ValidationError{Issues: issues}
	}
	return &BlockVersion{Doc: doc, Contract: contract}, nil
}

// DeleteBlock refuses any referenced block while holding SQLite's write lock.
func (s *Service) DeleteBlock(ctx context.Context, id string, draftRevision, liveRevision int) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE flows SET kind=kind WHERE id=? AND kind=?`, id, string(KindBlock))
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		var kind string
		if err := tx.QueryRowContext(ctx, `SELECT kind FROM flows WHERE id=?`, id).Scan(&kind); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return ErrWrongFlowKind
	}
	var draft, live int
	var draftJSON, liveJSON, missionID string
	if err := tx.QueryRowContext(ctx, `SELECT draft_revision,live_revision,draft_json,live_json,mission_id FROM flows WHERE id=?`, id).Scan(&draft, &live, &draftJSON, &liveJSON, &missionID); err != nil {
		return err
	}
	if missionID != "" {
		return ErrWrongFlowKind
	}
	for _, raw := range []string{draftJSON, liveJSON} {
		if raw == "" {
			continue
		}
		doc, err := ParseFlow([]byte(raw))
		if err != nil || doc.ID != id || doc.Kind != KindBlock {
			return ErrWrongFlowKind
		}
	}
	if draft != draftRevision || live != liveRevision {
		return ErrRevisionConflict
	}
	var used int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flow_block_version_pins WHERE block_flow_id=? UNION ALL SELECT 1 FROM flow_block_run_pins WHERE block_flow_id=?)`, id, id).Scan(&used); err != nil {
		return fmt.Errorf("check block dependencies: %w", err)
	}
	if used != 0 {
		return ErrBlockInUse
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM flows WHERE id=? AND kind=?`, id, string(KindBlock)); err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	return tx.Commit()
}
