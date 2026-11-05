package flows

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
)

// ErrBlockUnavailable means a referenced published block cannot be safely read.
var ErrBlockUnavailable = errors.New("the referenced block revision is unavailable")

// ErrBatchWorkLimit rejects a batch whose declared worst-case work is too large.
var ErrBatchWorkLimit = errors.New("batch work exceeds the allowed limit")

const maxBatchStepAttempts = 10000

type batchWorkError struct{ NodeID string }

func (e *batchWorkError) Error() string {
	return "reduce max_items: batch work exceeds the allowed limit"
}
func (e *batchWorkError) Unwrap() []error { return []error{ErrBatchWorkLimit, ErrBlockInvalid} }

func batchWorkIssue(err error) (Issue, bool) {
	var budget *batchWorkError
	if !errors.As(err, &budget) {
		return Issue{}, false
	}
	return Issue{Code: IssueBatchWorkLimit, Severity: SeverityError, NodeID: budget.NodeID, Param: "max_items", Message: "reduce max_items: the batch exceeds the work limit"}, true
}

func blockWorstAttempts(doc *Flow) int {
	total := 0
	for _, node := range doc.Nodes {
		total += 1 + clampInt(node.Settings.Retry.Count, 0, MaxRetryCount)
	}
	return total
}

// blockPlan holds cloned, validated snapshots; runtime performs no version lookup.
type blockPlan struct {
	byNode map[string]resolvedBlock
	blocks []resolvedBlock
}

func prepareBlockPlan(ctx context.Context, store *Store, doc *Flow, services *Services) (*blockPlan, error) {
	if doc == nil {
		return nil, ErrBlockInvalid
	}
	hasCalls := false
	for _, n := range doc.Nodes {
		if n.Type == blockCallType || n.Type == blockEachType {
			hasCalls = true
		}
	}
	if !hasCalls {
		return nil, nil
	}
	var plan *blockPlan
	err := blockReadSnapshot(ctx, store.db, func(tx *sql.Tx) error {
		var err error
		plan, err = prepareBlockPlanTx(ctx, tx, doc, services)
		return err
	})
	return plan, err
}

// prepareDraftBlockPlan checks the parent and all child revisions in one read snapshot.
func prepareDraftBlockPlan(ctx context.Context, store *Store, doc *Flow, revision int, services *Services) (*blockPlan, error) {
	var plan *blockPlan
	err := blockReadSnapshot(ctx, store.db, func(tx *sql.Tx) error {
		var raw, kind string
		var current int
		if err := tx.QueryRowContext(ctx, `SELECT draft_json,draft_revision,kind FROM flows WHERE id=?`, doc.ID).Scan(&raw, &current, &kind); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if kind != string(KindFlow) {
			return ErrWrongFlowKind
		}
		if current != revision {
			return ErrRevisionConflict
		}
		stored, err := ParseFlow([]byte(raw))
		if err != nil {
			return err
		}
		storedJSON, err := stored.Marshal()
		if err != nil {
			return err
		}
		requested, err := doc.Marshal()
		if err != nil {
			return err
		}
		if stored.ID != doc.ID || stored.Kind != KindFlow || sha256.Sum256(storedJSON) != sha256.Sum256(requested) {
			return ErrRevisionConflict
		}
		plan, err = prepareBlockPlanTx(ctx, tx, doc, services)
		return err
	})
	return plan, err
}

func prepareBlockPlanTx(ctx context.Context, tx *sql.Tx, doc *Flow, services *Services) (*blockPlan, error) {
	if doc == nil || doc.Kind != KindFlow || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
		return nil, ErrBlockInvalid
	}
	data, err := doc.Marshal()
	if err != nil || len(data) > MaxDocumentBytes {
		return nil, ErrBlockInvalid
	}
	loader := func(ctx context.Context, id string, revision int) (*Flow, error) {
		var raw string
		query := `SELECT v.json FROM flow_versions v JOIN flows f ON f.id=v.flow_id
			WHERE v.flow_id=? AND v.revision=? AND f.kind='block' AND f.mission_id='' AND length(CAST(v.json AS BLOB))<=?
			AND (` + validStoredDocumentSQL("f.draft_json") + `) AND (f.live_json='' OR (` + validStoredDocumentSQL("f.live_json") + `))`
		err := tx.QueryRowContext(ctx, query, id, revision, MaxDocumentBytes).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBlockUnavailable
		}
		if err != nil {
			return nil, err
		}
		child, err := ParseFlow([]byte(raw))
		if err != nil || child.ID != id || !blockDocumentAllowed(child) {
			return nil, ErrBlockUnavailable
		}
		issues := Validate(child, blockRegistry(), ValidateContext{Mode: ModePublish, Now: services.Now(), Location: services.Loc()})
		if HasErrors(issues) {
			return nil, ErrBlockUnavailable
		}
		return child, nil
	}
	blocks, err := resolveBlockDependencies(ctx, doc, loader)
	if err != nil {
		return nil, fmt.Errorf("resolve block: %w", err)
	}
	byRef := make(map[blockRef]resolvedBlock, len(blocks))
	for _, block := range blocks {
		byRef[block.Ref] = block
	}
	plan := &blockPlan{byNode: make(map[string]resolvedBlock), blocks: blocks}
	nodes, edges, bytes := len(doc.Nodes), len(doc.Edges), len(data)
	work := blockWorstAttempts(doc)
	hasBatch := false
	firstBatchID := ""
	for _, n := range doc.Nodes {
		if n.Type == blockEachType {
			hasBatch = true
			firstBatchID = n.ID
			break
		}
	}
	seenIDs := make(map[string]bool, len(doc.Nodes))
	for _, n := range doc.Nodes {
		if n.ID == "" || seenIDs[n.ID] {
			return nil, ErrBlockInvalid
		}
		seenIDs[n.ID] = true
		if n.Type != blockCallType && n.Type != blockEachType {
			continue
		}
		if n.Settings.Retry.Count != 0 {
			return nil, ErrBlockInvalid
		}
		ref, err := blockReference(n)
		if err != nil {
			return nil, ErrBlockInvalid
		}
		child, ok := byRef[ref]
		if !ok {
			return nil, ErrBlockUnavailable
		}
		if err := checkBlockBindings(n, child.Contract); err != nil {
			return nil, ErrBlockInvalid
		}
		encoded, err := child.Document.Marshal()
		if err != nil {
			return nil, ErrBlockInvalid
		}
		nodes, edges, bytes = nodes+len(child.Document.Nodes), edges+len(child.Document.Edges), bytes+len(encoded)
		if nodes > MaxNodes || edges > MaxEdges || bytes > MaxDocumentBytes {
			return nil, ErrBlockInvalid
		}
		if hasBatch {
			multiplier := 1
			if n.Type == blockEachType {
				multiplier = int(n.Params["max_items"].(float64))
			}
			childWork := blockWorstAttempts(child.Document)
			if childWork > 0 && (work > maxBatchStepAttempts || multiplier > (maxBatchStepAttempts-work)/childWork) {
				id := firstBatchID
				if n.Type == blockEachType {
					id = n.ID
				}
				return nil, &batchWorkError{NodeID: id}
			}
			work += multiplier * childWork
		}
		plan.byNode[n.ID] = child
	}
	return plan, nil
}
