package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// RunFilter narrows ListRuns. Limit defaults to 50 and is capped at 200.
type RunFilter struct {
	Mode   RunMode
	Status RunStatus
	Limit  int
	Offset int
}

const runColumns = `id, flow_id, revision, mode, trigger_node, trigger_type, trigger_data_json, status,
	error_code, error_message, started_at, finished_at, duration_ms, parent_run_id, parent_node_id`

func scanRun(row scanner) (*RunRecord, error) {
	var r RunRecord
	var mode, status, triggerJSON, startedAt, finishedAt string
	if err := row.Scan(&r.ID, &r.FlowID, &r.Revision, &mode, &r.TriggerNode, &r.TriggerType, &triggerJSON, &status,
		&r.ErrorCode, &r.ErrorMessage, &startedAt, &finishedAt, &r.DurationMS, &r.ParentRunID, &r.ParentNodeID); err != nil {
		return nil, err
	}
	r.Mode = RunMode(mode)
	r.Status = RunStatus(status)
	if triggerJSON != "" {
		_ = json.Unmarshal([]byte(triggerJSON), &r.TriggerData)
	}
	r.StartedAt = parseTime(startedAt)
	if finishedAt != "" {
		t := parseTime(finishedAt)
		r.FinishedAt = &t
	}
	return &r, nil
}

func marshalMap(m map[string]any) (string, error) {
	if m == nil {
		return "{}", nil
	}
	data, err := json.Marshal(m)
	return string(data), err
}

// marshalBounded is marshalMap with the run log's size rule: above
// MaxStoredOutputBytes it encodes {"_preview": "<start of the encoded JSON>"}
// instead, exactly like a step output (storedOutput, previewOf). m is not modified.
func marshalBounded(m map[string]any) (string, error) {
	if m == nil {
		return "{}", nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	stored, truncated := storedOutput(m, data)
	if !truncated {
		return string(data), nil
	}
	return marshalMap(stored)
}

// requireRow maps an UPDATE that matched no run to ErrRunNotFound. SQLite counts
// matched rows, so an update that leaves the values unchanged still succeeds.
func requireRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrRunNotFound
	}
	return nil
}

// CreateRun inserts a run header. For test runs the document is stored so the run view can show it later.
// The trigger data (a webhook or mail payload of up to MaxOutputBytes) is bounded like a step
// output: beyond MaxStoredOutputBytes the header keeps {"_preview": ...} instead.
func (s *Store) CreateRun(ctx context.Context, rec RunRecord, doc *Flow) error {
	trigger, err := marshalBounded(rec.TriggerData)
	if err != nil {
		return err
	}
	docJSON := ""
	if doc != nil && rec.Mode == ModeTest {
		data, err := doc.Marshal()
		if err != nil {
			return err
		}
		docJSON = string(data)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO flow_runs (id, flow_id, revision, mode, trigger_node, trigger_type,
		trigger_data_json, status, started_at, parent_run_id, parent_node_id, doc_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.FlowID, rec.Revision, string(rec.Mode), rec.TriggerNode, rec.TriggerType, trigger,
		string(rec.Status), formatTime(rec.StartedAt), rec.ParentRunID, rec.ParentNodeID, docJSON)
	return err
}

// SetRunStatus updates the status of a run. It returns ErrRunNotFound when no
// such run is stored, for example because its flow was deleted meanwhile.
func (s *Store) SetRunStatus(ctx context.Context, runID string, status RunStatus) error {
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status = ? WHERE id = ?`, string(status), runID)
	if err != nil {
		return err
	}
	return requireRow(res)
}

// SaveStep stores (or replaces) the step with sequence number seq.
func (s *Store) SaveStep(ctx context.Context, runID string, seq int, step StepRecord) error {
	params, err := marshalMap(step.Params)
	if err != nil {
		return err
	}
	output, err := marshalMap(step.Output)
	if err != nil {
		return err
	}
	ports, err := json.Marshal(step.Ports)
	if err != nil {
		return err
	}
	paramsTruncated, outputTruncated := 0, 0
	if step.ParamsTruncated {
		paramsTruncated = 1
	}
	if step.OutputTruncated {
		outputTruncated = 1
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO flow_run_steps (run_id, seq, node_id, node_key, attempt, status,
		started_at, finished_at, duration_ms, params_json, params_truncated, output_json, output_truncated, item_count,
		ports_json, error_code, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, seq, step.NodeID, step.NodeKey, step.Attempt, string(step.Status), formatTime(step.StartedAt),
		formatTime(step.FinishedAt), step.DurationMS, params, paramsTruncated, output, outputTruncated, step.ItemCount,
		string(ports), step.ErrorCode, step.ErrorMessage)
	return err
}

// FinishRun stores the final status of a run. It returns ErrRunNotFound when no
// such run is stored, for example because its flow was deleted meanwhile.
func (s *Store) FinishRun(ctx context.Context, runID string, res RunResult) error {
	r, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status = ?, error_code = ?, error_message = ?, finished_at = ?,
		duration_ms = ? WHERE id = ?`, string(res.Status), res.ErrorCode, res.ErrorMessage, formatTime(res.FinishedAt),
		res.DurationMS, runID)
	if err != nil {
		return err
	}
	return requireRow(r)
}

// GetRun returns the run header and its steps in sequence order.
func (s *Store) GetRun(ctx context.Context, runID string) (*RunRecord, []StepRecord, error) {
	rec, err := scanRun(s.db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM flow_runs WHERE id = ?`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrRunNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT node_id, node_key, attempt, status, started_at, finished_at, duration_ms,
		params_json, params_truncated, output_json, output_truncated, item_count, ports_json, error_code, error_message
		FROM flow_run_steps WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var steps []StepRecord
	for rows.Next() {
		var st StepRecord
		var status, startedAt, finishedAt, params, output, ports string
		var paramsTruncated, outputTruncated int
		if err := rows.Scan(&st.NodeID, &st.NodeKey, &st.Attempt, &status, &startedAt, &finishedAt, &st.DurationMS,
			&params, &paramsTruncated, &output, &outputTruncated, &st.ItemCount, &ports, &st.ErrorCode, &st.ErrorMessage); err != nil {
			return nil, nil, err
		}
		st.Status = StepStatus(status)
		st.StartedAt = parseTime(startedAt)
		st.FinishedAt = parseTime(finishedAt)
		st.ParamsTruncated = paramsTruncated == 1
		st.OutputTruncated = outputTruncated == 1
		_ = json.Unmarshal([]byte(params), &st.Params)
		_ = json.Unmarshal([]byte(output), &st.Output)
		_ = json.Unmarshal([]byte(ports), &st.Ports)
		steps = append(steps, st)
	}
	return rec, steps, rows.Err()
}

// GetRunDoc returns the flow document a run executed: the stored draft for test runs,
// otherwise the published version. Pruned versions yield ErrNotFound.
func (s *Store) GetRunDoc(ctx context.Context, runID string) (*Flow, error) {
	var flowID, docJSON string
	var revision int
	err := s.db.QueryRowContext(ctx, `SELECT flow_id, revision, doc_json FROM flow_runs WHERE id = ?`, runID).
		Scan(&flowID, &revision, &docJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if docJSON != "" {
		doc, err := ParseFlow([]byte(docJSON))
		if err != nil {
			return nil, fmt.Errorf("run %s document: %w", quoteForError(runID), err)
		}
		return doc, nil
	}
	return s.GetVersion(ctx, flowID, revision)
}

// ListRuns returns runs of a flow, newest first.
func (s *Store) ListRuns(ctx context.Context, flowID string, f RunFilter) ([]RunRecord, error) {
	query := `SELECT ` + runColumns + ` FROM flow_runs WHERE flow_id = ?`
	args := []any{flowID}
	if f.Mode != "" {
		query += ` AND mode = ?`
		args = append(args, string(f.Mode))
	}
	if f.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(f.Status))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	query += ` ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	return s.queryRuns(ctx, query, args...)
}

func (s *Store) queryRuns(ctx context.Context, query string, args ...any) ([]RunRecord, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunRecord
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// LastLiveRuns returns the newest non-test run of every flow, keyed by flow id.
// Runs of one flow can share a started_at; the ORDER BY makes the first row seen per
// flow the one with the highest id, the same tie-break ListRuns uses.
func (s *Store) LastLiveRuns(ctx context.Context) (map[string]RunRecord, error) {
	runs, err := s.queryRuns(ctx, `SELECT `+runColumns+` FROM flow_runs r WHERE mode != 'test' AND started_at =
		(SELECT MAX(started_at) FROM flow_runs r2 WHERE r2.flow_id = r.flow_id AND r2.mode != 'test')
		ORDER BY started_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	out := make(map[string]RunRecord, len(runs))
	for _, r := range runs {
		if _, seen := out[r.FlowID]; !seen {
			out[r.FlowID] = r
		}
	}
	return out, nil
}

// MarkInterruptedRuns cancels runs left queued, running or waiting by a previous process:
// only runs that started before `before` (the boot time of this process) are touched, so
// runs that Mission Control started during startup survive.
func (s *Store) MarkInterruptedRuns(ctx context.Context, before, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status = 'cancelled', error_code = 'FLOW_RESTARTED',
		error_message = 'AuraGo restarted while the run was active', finished_at = ?
		WHERE status IN ('queued', 'running', 'waiting') AND started_at < ?`, formatTime(now), formatTime(before))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneRuns deletes finished runs older than retentionDays and keeps at most maxPerFlow finished runs per flow.
func (s *Store) PruneRuns(ctx context.Context, retentionDays, maxPerFlow int, now time.Time) (int64, error) {
	var total int64
	if retentionDays > 0 {
		cutoff := formatTime(now.AddDate(0, 0, -retentionDays))
		res, err := s.db.ExecContext(ctx, `DELETE FROM flow_runs WHERE started_at < ?
			AND status IN ('success', 'error', 'cancelled')`, cutoff)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	if maxPerFlow > 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT flow_id FROM flow_runs`)
		if err != nil {
			return total, err
		}
		var flowIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return total, err
			}
			flowIDs = append(flowIDs, id)
		}
		rows.Close()
		for _, id := range flowIDs {
			res, err := s.db.ExecContext(ctx, `DELETE FROM flow_runs WHERE id IN (
				SELECT id FROM flow_runs WHERE flow_id = ? AND status IN ('success', 'error', 'cancelled')
				ORDER BY started_at DESC, id DESC LIMIT -1 OFFSET ?)`, id, maxPerFlow)
			if err != nil {
				return total, err
			}
			n, _ := res.RowsAffected()
			total += n
		}
	}
	return total, nil
}
