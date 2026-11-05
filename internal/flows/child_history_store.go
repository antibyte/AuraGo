package flows

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// saveChildStep commits a parent and its trace together. Never REPLACE: doing
// so would delete the old parent row and cascade its immutable evidence.
func (s *Store) saveChildStep(ctx context.Context, runID string, seq int, step StepRecord, trace *ChildTrace) error {
	if trace == nil || trace.RunID != runID || trace.ParentSeq != seq || !ValidChildParentSeq(seq) {
		return ErrChildHistoryUnavailable
	}
	if !childTextSafe(runID, s.childHistoryScrubber()) {
		return ErrChildHistoryUnavailable
	}
	data, err := json.Marshal(trace)
	if err != nil || len(data) > MaxChildTraceBytes {
		return ErrChildHistoryUnavailable
	}
	summary, err := json.Marshal(trace.History)
	if err != nil || len(summary) > 2048 {
		return ErrChildHistoryUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	metadata, err := childTraceMetadataSize(trace)
	if err != nil {
		return err
	}
	rowBytes := len(summary) + len(runID) + 256
	metadata += rowBytes
	chargedBytes := len(data) + rowBytes
	res, err := tx.ExecContext(ctx, `UPDATE flow_runs SET child_history_bytes=child_history_bytes+?,
		child_history_metadata=child_history_metadata+? WHERE id=? AND durable=0 AND child_history_state='pending'
		AND child_history_bytes+?<=? AND child_history_metadata+?<=?`, chargedBytes, metadata, runID,
		chargedBytes, maxChildHistoryBytes-maxChildHistoryMetadataBytes, metadata, maxChildHistoryMetadataBytes)
	if err != nil {
		return err
	}
	var traceValue any = string(data)
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		// The blob budget is full: retain a bounded marker and parent step in
		// the same transaction, never replace an existing journal row.
		trace.Frames = []ChildFrame{}
		trace.History.RetainedInvocations = 0
		trace.History.RetainedSteps = 0
		trace.History.TotalInvocations = nil
		trace.History.TotalSteps = nil
		trace.History.Truncated = true
		if trace.History.State != ChildHistoryIncomplete {
			trace.History.State = ChildHistoryTruncated
		}
		summary, err = json.Marshal(trace.History)
		if err != nil {
			return err
		}
		metadata = len(summary) + len(runID) + 256
		res, err = tx.ExecContext(ctx, `UPDATE flow_runs SET child_history_bytes=child_history_bytes+?,child_history_metadata=child_history_metadata+?
			WHERE id=? AND durable=0 AND child_history_state='pending' AND child_history_bytes+?<=? AND child_history_metadata+?<=?`,
			metadata, metadata, runID, metadata, maxChildHistoryBytes, metadata, maxChildHistoryMetadataBytes)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			return ErrChildHistoryUnavailable
		}
		traceValue = nil
	}
	if err := s.saveStepOn(ctx, tx, runID, seq, step, false); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO flow_child_traces(run_id,parent_seq,summary_json,trace_json) VALUES(?,?,?,?)`, runID, seq, string(summary), traceValue); err != nil {
		return err
	}
	return tx.Commit()
}

func childTraceMetadataSize(trace *ChildTrace) (int, error) {
	copyTrace := *trace
	copyTrace.Frames = make([]ChildFrame, len(trace.Frames))
	for i, frame := range trace.Frames {
		copyTrace.Frames[i] = frame
		copyTrace.Frames[i].Steps = make([]ChildStep, len(frame.Steps))
		for j, step := range frame.Steps {
			step.Step.Params = nil
			step.Step.Output = nil
			copyTrace.Frames[i].Steps[j] = step
		}
	}
	data, err := json.Marshal(copyTrace)
	return len(data), err
}

func (s *Store) finishRunWithChildHistory(ctx context.Context, runID string, result RunResult, state ChildHistoryState) error {
	if state != ChildHistoryComplete && state != ChildHistoryIncomplete && state != ChildHistoryTruncated {
		return ErrChildHistoryUnavailable
	}
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status=?,error_code=?,error_message=?,finished_at=?,duration_ms=?,
		child_history_state=CASE WHEN child_history_state='pending' THEN ? ELSE child_history_state END WHERE id=?`,
		string(result.Status), result.ErrorCode, s.scrubText(result.ErrorMessage), formatTime(result.FinishedAt), result.DurationMS, state, runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrRunNotFound)
}

// childTraceTx verifies the parent ownership and reads one bounded trace within
// the same snapshot used by its optional immutable child-document lookup.
func childTraceTx(ctx context.Context, tx *sql.Tx, runID string, seq int) (*ChildTrace, error) {
	if runID == "" || len(runID) > 128 || !ValidChildParentSeq(seq) {
		return nil, ErrChildHistoryNotFound
	}
	var data sql.NullString
	var state string
	var traceSize int
	var summary sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT CASE WHEN length(CAST(t.trace_json AS BLOB))<=? THEN t.trace_json END,
		CASE WHEN t.run_id IS NULL THEN 0 WHEN t.trace_json IS NULL THEN 3 WHEN length(CAST(t.trace_json AS BLOB))<=? THEN 1 ELSE 2 END,
		CASE WHEN length(CAST(t.summary_json AS BLOB))<=2048 THEN t.summary_json END,r.child_history_state FROM flow_runs r
		JOIN flow_run_steps s ON s.run_id=r.id AND s.seq=?
		LEFT JOIN flow_child_traces t ON t.run_id=s.run_id AND t.parent_seq=s.seq WHERE r.id=?
		AND EXISTS (SELECT 1 FROM flows f WHERE f.id=r.flow_id AND f.kind='flow' AND (`+validStoredDocumentSQL("f.draft_json")+`)
		AND (f.live_json='' OR (`+validStoredDocumentSQL("f.live_json")+`)))`, MaxChildTraceBytes, MaxChildTraceBytes, seq, runID).Scan(&data, &traceSize, &summary, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChildHistoryNotFound
	}
	if err != nil {
		return nil, err
	}
	if traceSize == 2 {
		return nil, ErrChildHistoryUnavailable
	}
	if traceSize == 3 {
		var child ChildHistorySummary
		if !summary.Valid || json.Unmarshal([]byte(summary.String), &child) != nil || child.ParentSeq != seq ||
			(child.State != ChildHistoryTruncated && child.State != ChildHistoryIncomplete) || !child.Truncated ||
			child.RetainedInvocations != 0 || child.RetainedSteps != 0 || child.TotalInvocations != nil || child.TotalSteps != nil {
			return nil, ErrChildHistoryUnavailable
		}
		return &ChildTrace{Schema: 1, RunID: runID, ParentSeq: seq, History: child, Frames: []ChildFrame{}}, nil
	}
	if !data.Valid {
		if ChildHistoryState(state) == ChildHistoryLegacyUnknown {
			return &ChildTrace{Schema: 1, RunID: runID, ParentSeq: seq, History: ChildHistorySummary{ParentSeq: seq, State: ChildHistoryLegacyUnknown}, Frames: []ChildFrame{}}, nil
		}
		return nil, ErrChildHistoryNotFound
	}
	if len(data.String) > MaxChildTraceBytes {
		return nil, ErrChildHistoryUnavailable
	}
	var trace ChildTrace
	if json.Unmarshal([]byte(data.String), &trace) != nil || trace.Schema != 1 || trace.RunID != runID || trace.ParentSeq != seq || trace.History.ParentSeq != seq {
		return nil, ErrChildHistoryUnavailable
	}
	var child ChildHistorySummary
	if !summary.Valid || json.Unmarshal([]byte(summary.String), &child) != nil || !reflect.DeepEqual(child, trace.History) {
		return nil, ErrChildHistoryUnavailable
	}
	return &trace, nil
}

func childHistoryReadError(err error) error {
	if err == nil || errors.Is(err, ErrChildHistoryNotFound) || errors.Is(err, ErrChildHistoryUnavailable) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrChildHistoryUnavailable, err)
}

func (s *Service) ChildTrace(ctx context.Context, runID string, parentSeq int) (*ChildTrace, error) {
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, childHistoryReadError(err)
	}
	defer tx.Rollback()
	trace, err := childTraceTx(ctx, tx, runID, parentSeq)
	if err != nil {
		return nil, childHistoryReadError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, childHistoryReadError(err)
	}
	return trace, nil
}

func (s *Service) ChildTraceDocument(ctx context.Context, runID string, parentSeq int, invocationID string) (*ChildTraceDocument, error) {
	if !ValidChildInvocationID(invocationID) {
		return nil, ErrChildHistoryNotFound
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, childHistoryReadError(err)
	}
	defer tx.Rollback()
	trace, err := childTraceTx(ctx, tx, runID, parentSeq)
	if err != nil {
		return nil, childHistoryReadError(err)
	}
	var frame *ChildFrame
	for i := range trace.Frames {
		if trace.Frames[i].InvocationID == invocationID {
			frame = &trace.Frames[i]
			break
		}
	}
	if frame == nil {
		return nil, ErrChildHistoryNotFound
	}
	result := &ChildTraceDocument{BlockID: frame.BlockID, Revision: frame.Revision, Hash: frame.Hash}
	var raw string
	var hash []byte
	err = tx.QueryRowContext(ctx, `SELECT v.json,p.block_hash FROM flow_block_run_pins p
		JOIN flow_versions v ON v.flow_id=p.block_flow_id AND v.revision=p.block_revision
		JOIN flows f ON f.id=v.flow_id AND f.kind='block' AND f.mission_id='' AND (`+validStoredDocumentSQL("f.draft_json")+`)
		AND (f.live_json='' OR (`+validStoredDocumentSQL("f.live_json")+`))
		WHERE p.run_id=? AND p.block_flow_id=? AND p.block_revision=? AND length(CAST(v.json AS BLOB))<=?`,
		runID, frame.BlockID, frame.Revision, MaxDocumentBytes).Scan(&raw, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, childHistoryReadError(err)
	}
	if err == nil {
		doc, parseErr := ParseFlow([]byte(raw))
		if parseErr == nil && doc.ID == frame.BlockID && doc.Kind == KindBlock {
			canonical, marshalErr := json.Marshal(doc)
			sum := sha256.Sum256(canonical)
			if marshalErr == nil && hex.EncodeToString(sum[:]) == frame.Hash && hex.EncodeToString(hash) == frame.Hash {
				result.Doc, err = RedactCredentials(doc)
				if err != nil {
					result.Doc = nil
				}
			}
		}
	}
	if result.Doc == nil {
		result.DocMissing = ChildDocumentUnavailable
	}
	if err := tx.Commit(); err != nil {
		return nil, childHistoryReadError(err)
	}
	return result, nil
}
