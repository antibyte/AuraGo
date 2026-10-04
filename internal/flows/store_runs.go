package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RunFilter narrows ListRuns. Limit defaults to 50 and is capped at 200.
type RunFilter struct {
	Mode   RunMode
	Status RunStatus
	Limit  int
	Offset int
}

// runColumnNames are the flow_runs columns scanRun reads, in scan order.
var runColumnNames = []string{"id", "flow_id", "revision", "mode", "trigger_node", "trigger_type", "trigger_data_json",
	"status", "error_code", "error_message", "started_at", "finished_at", "duration_ms", "parent_run_id", "parent_node_id"}

// runSelect builds the column list scanRun reads. alias qualifies the columns when
// the query joins another table. Without withTrigger the potentially large
// trigger_data_json (up to MaxStoredOutputBytes per run) is not read and an empty
// object stands in for it.
func runSelect(alias string, withTrigger bool) string {
	cols := make([]string, len(runColumnNames))
	for i, name := range runColumnNames {
		switch {
		case name == "trigger_data_json" && !withTrigger:
			cols[i] = "'{}'"
		case alias != "":
			cols[i] = alias + "." + name
		default:
			cols[i] = name
		}
	}
	return strings.Join(cols, ", ")
}

var (
	// runColumns is for GetRun, the only reader of the trigger data.
	runColumns = runSelect("", true)
	// runListColumns is for the list queries, which leave TriggerData empty.
	runListColumns = runSelect("", false)
	// lastLiveColumns is runListColumns for LastLiveRuns, which aliases flow_runs as r.
	lastLiveColumns = runSelect("r", false)
)

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

// requireRow maps a statement that touched no row to notFound. The writes of this
// store are conditional (an UPDATE by id, or an INSERT ... SELECT ... WHERE EXISTS
// on the parent row), so "no row" means the target or its parent is missing. SQLite
// counts matched rows, so an update that leaves the values unchanged still succeeds.
func requireRow(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}

// CreateRun inserts a run header. It returns ErrNotFound when the flow does not exist
// (the foreign key would fail) and rejects a zero StartedAt.
//
// For test runs the flow document is stored so the run view can show it later; one
// above MaxDocumentBytes could not be read back by ParseFlow, so it is left out (with
// a warning) instead of failing the run, and GetRunDoc then reports ErrNotFound.
//
// The trigger data (a webhook or mail payload of up to MaxOutputBytes) is bounded like a step
// output: beyond MaxStoredOutputBytes the header keeps {"_preview": ...} instead.
func (s *Store) CreateRun(ctx context.Context, rec RunRecord, doc *Flow) error {
	if rec.StartedAt.IsZero() {
		return errors.New("a run needs a started_at time")
	}
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
		if len(data) > MaxDocumentBytes {
			s.logger.Warn("flow document too large to store with its test run", "run", truncateForError(rec.ID), "bytes", len(data))
		} else {
			docJSON = string(data)
		}
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO flow_runs (id, flow_id, revision, mode, trigger_node, trigger_type,
		trigger_data_json, status, started_at, parent_run_id, parent_node_id, doc_json)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM flows WHERE id = ?)`,
		rec.ID, rec.FlowID, rec.Revision, string(rec.Mode), rec.TriggerNode, rec.TriggerType, trigger,
		string(rec.Status), formatTime(rec.StartedAt), rec.ParentRunID, rec.ParentNodeID, docJSON, rec.FlowID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrNotFound)
}

// SetRunStatus updates the status of a run. It returns ErrRunNotFound when no
// such run is stored, for example because its flow was deleted meanwhile.
func (s *Store) SetRunStatus(ctx context.Context, runID string, status RunStatus) error {
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status = ? WHERE id = ?`, string(status), runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrRunNotFound)
}

// SaveStep stores (or replaces) the step with sequence number seq. It returns
// ErrRunNotFound when the run does not exist, for example because its flow was
// deleted meanwhile.
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
	res, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO flow_run_steps (run_id, seq, node_id, node_key, attempt, status,
		started_at, finished_at, duration_ms, params_json, params_truncated, output_json, output_truncated, item_count,
		ports_json, error_code, error_message)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM flow_runs WHERE id = ?)`,
		runID, seq, step.NodeID, step.NodeKey, step.Attempt, string(step.Status), formatTime(step.StartedAt),
		formatTime(step.FinishedAt), step.DurationMS, params, paramsTruncated, output, outputTruncated, step.ItemCount,
		string(ports), step.ErrorCode, step.ErrorMessage, runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrRunNotFound)
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
	return requireRow(r, ErrRunNotFound)
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
// otherwise the published version. Pruned versions yield ErrNotFound, and so does a
// test run whose document was not stored (see CreateRun): its revision is a draft
// revision, which says nothing about the published versions.
func (s *Store) GetRunDoc(ctx context.Context, runID string) (*Flow, error) {
	var flowID, mode, docJSON string
	var revision int
	err := s.db.QueryRowContext(ctx, `SELECT flow_id, revision, mode, doc_json FROM flow_runs WHERE id = ?`, runID).
		Scan(&flowID, &revision, &mode, &docJSON)
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
	if RunMode(mode) == ModeTest {
		return nil, ErrNotFound
	}
	return s.GetVersion(ctx, flowID, revision)
}

// ListRuns returns runs of a flow, newest first. Like LastLiveRuns it does not read
// the trigger data, so TriggerData of the returned runs is empty; GetRun returns it.
func (s *Store) ListRuns(ctx context.Context, flowID string, f RunFilter) ([]RunRecord, error) {
	query := `SELECT ` + runListColumns + ` FROM flow_runs WHERE flow_id = ?`
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

// LastLiveRuns returns the newest non-test run of every flow that has one, keyed by
// flow id. It does not read the trigger data, so TriggerData is empty; GetRun
// returns it.
//
// The query drives from flows and looks the run up per flow, so it is one index
// seek per flow instead of a scan of every run (it backs the start page). Runs of
// one flow can share a started_at; the highest id wins, the same tie-break ListRuns
// uses.
func (s *Store) LastLiveRuns(ctx context.Context) (map[string]RunRecord, error) {
	runs, err := s.queryRuns(ctx, `SELECT `+lastLiveColumns+` FROM flows f JOIN flow_runs r ON r.id = (
		SELECT r2.id FROM flow_runs r2 WHERE r2.flow_id = f.id AND r2.mode != 'test'
		ORDER BY r2.started_at DESC, r2.id DESC LIMIT 1)`)
	if err != nil {
		return nil, err
	}
	out := make(map[string]RunRecord, len(runs))
	for _, r := range runs {
		out[r.FlowID] = r
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

// PruneRuns deletes finished (success, error or cancelled) runs older than
// retentionDays and keeps at most maxPerFlow finished runs per flow, the newest ones.
// A value of zero or less disables that rule. Queued, running and waiting runs are
// never deleted and do not count toward maxPerFlow. The steps of a deleted run go
// with it (ON DELETE CASCADE). It returns the number of deleted runs.
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
		rows, err := s.db.QueryContext(ctx, `SELECT id FROM flows`)
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
		// The rows are closed before the deletes below: they need the connection pool.
		err = rows.Err()
		rows.Close()
		if err != nil {
			return total, err
		}
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
