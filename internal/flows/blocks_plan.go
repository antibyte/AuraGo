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
		var raw, kind, mission string
		var current int
		var draftValid, liveValid bool
		query := `SELECT draft_json,draft_revision,kind,mission_id, (` + validStoredDocumentSQL("draft_json") + `),
			(live_json='' OR (` + validStoredDocumentSQL("live_json") + `)) FROM flows WHERE id=?`
		if err := tx.QueryRowContext(ctx, query, doc.ID).Scan(&raw, &current, &kind, &mission, &draftValid, &liveValid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if kind != string(doc.Kind) || (doc.Kind != KindFlow && doc.Kind != KindBlock) || (doc.Kind == KindBlock && mission != "") || !draftValid || !liveValid {
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
		if stored.ID != doc.ID || stored.Kind != doc.Kind || sha256.Sum256(storedJSON) != sha256.Sum256(requested) {
			return ErrRevisionConflict
		}
		plan, err = prepareBlockPlanTx(ctx, tx, doc, services)
		return err
	})
	return plan, err
}

func prepareBlockPlanTx(ctx context.Context, tx *sql.Tx, doc *Flow, services *Services) (*blockPlan, error) {
	if doc == nil || (doc.Kind != KindFlow && doc.Kind != KindBlock) || len(doc.Nodes) > MaxNodes || len(doc.Edges) > MaxEdges {
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
		if errors.Is(err, errBlockContract) {
			return nil, fmt.Errorf("%w: invalid block dependency: %w", ErrBlockInvalid, err)
		}
		return nil, fmt.Errorf("resolve block: %w", err)
	}
	byRef := make(map[blockRef]resolvedBlock, len(blocks))
	buildFrame := func(frame *Flow) (*blockPlan, error) {
		plan := &blockPlan{byNode: make(map[string]resolvedBlock)}
		seenIDs := make(map[string]bool, len(frame.Nodes))
		seenRefs := make(map[blockRef]bool)
		for _, node := range frame.Nodes {
			if node.ID == "" || seenIDs[node.ID] {
				return nil, ErrBlockInvalid
			}
			seenIDs[node.ID] = true
			if node.Type != blockCallType && node.Type != blockEachType {
				continue
			}
			if node.Settings.Retry.Count != 0 || (frame.Kind == KindBlock && node.Type == blockEachType) {
				return nil, ErrBlockInvalid
			}
			ref, err := blockReference(node)
			if err != nil {
				return nil, ErrBlockInvalid
			}
			child, ok := byRef[ref]
			if !ok {
				return nil, ErrBlockUnavailable
			}
			if checkBlockBindings(node, child.Contract) != nil {
				return nil, ErrBlockInvalid
			}
			plan.byNode[node.ID] = child
			if child.plan != nil {
				for _, descendant := range child.plan.blocks {
					if !seenRefs[descendant.Ref] {
						seenRefs[descendant.Ref] = true
						plan.blocks = append(plan.blocks, descendant)
					}
				}
			}
			if !seenRefs[ref] {
				seenRefs[ref] = true
				plan.blocks = append(plan.blocks, child)
			}
		}
		return plan, nil
	}
	for _, block := range blocks {
		frame, err := buildFrame(block.Document)
		if err != nil {
			return nil, err
		}
		if len(frame.byNode) > 0 {
			block.plan = frame
		}
		byRef[block.Ref] = block
	}
	plan, err := buildFrame(doc)
	if err != nil {
		return nil, err
	}
	if err := verifyBlockPinsTx(ctx, tx, plan.blocks); err != nil {
		return nil, err
	}
	nodes, edges, bytes, work := 0, 0, 0, 0
	hasBatch := false
	firstBatchID := ""
	for _, n := range doc.Nodes {
		if n.Type == blockEachType {
			hasBatch = true
			firstBatchID = n.ID
			break
		}
	}
	var measure func(*Flow, *blockPlan, int, string) error
	measure = func(frame *Flow, framePlan *blockPlan, multiplier int, culprit string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		encoded, err := frame.Marshal()
		if err != nil || len(frame.Nodes) > MaxNodes-nodes || len(frame.Edges) > MaxEdges-edges || len(encoded) > MaxDocumentBytes-bytes {
			return ErrBlockInvalid
		}
		nodes += len(frame.Nodes)
		edges += len(frame.Edges)
		bytes += len(encoded)
		if hasBatch {
			attempts := blockWorstAttempts(frame)
			if attempts > 0 && (work > maxBatchStepAttempts || multiplier > (maxBatchStepAttempts-work)/attempts) {
				if culprit == "" {
					culprit = firstBatchID
				}
				return &batchWorkError{NodeID: culprit}
			}
			work += multiplier * attempts
		}
		for _, node := range frame.Nodes {
			child, ok := framePlan.byNode[node.ID]
			if !ok {
				continue
			}
			next := multiplier
			nextCulprit := culprit
			if node.Type == blockEachType {
				raw, ok := node.Params["max_items"].(float64)
				if !ok || !(raw >= 1 && raw <= maxBatchItems) || raw != float64(int(raw)) {
					return ErrBlockInvalid
				}
				limit := int(raw)
				nextCulprit = node.ID
				if hasBatch && (next > maxBatchStepAttempts/limit) {
					return &batchWorkError{NodeID: node.ID}
				}
				next *= limit
			}
			childPlan := child.plan
			if childPlan == nil {
				childPlan = &blockPlan{}
			}
			if err := measure(child.Document, childPlan, next, nextCulprit); err != nil {
				return err
			}
		}
		return nil
	}
	if err := measure(doc, plan, 1, ""); err != nil {
		return nil, err
	}
	return plan, nil
}
