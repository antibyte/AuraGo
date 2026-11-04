package flows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type durableReservation struct {
	RunID      string
	FlowID     string
	Revision   int
	Generation int64
	State      string
	WakeAt     time.Time
	Deadline   time.Time
}

// durableReservations reads only non-secret scheduler metadata. A malformed
// row blocks recovery instead of freeing its flow's concurrency reservation.
func (s *Store) durableReservations(ctx context.Context) ([]durableReservation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.run_id,r.flow_id,r.revision,c.generation,c.state,c.wake_at,c.deadline,r.mode,r.status
		FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id ORDER BY c.wake_at,c.run_id`)
	if err != nil {
		return nil, fmt.Errorf("read durable reservations: %w", err)
	}
	defer rows.Close()
	var out []durableReservation
	for rows.Next() {
		var rec durableReservation
		var wake, deadline, mode, status string
		if err := rows.Scan(&rec.RunID, &rec.FlowID, &rec.Revision, &rec.Generation, &rec.State, &wake, &deadline, &mode, &status); err != nil {
			return nil, err
		}
		rec.WakeAt, rec.Deadline = parseTime(wake), parseTime(deadline)
		if rec.RunID == "" || rec.FlowID == "" || rec.Revision <= 0 || rec.Generation <= 0 || rec.WakeAt.IsZero() || rec.Deadline.IsZero() || !rec.WakeAt.Before(rec.Deadline) ||
			mode != string(ModeLive) || (rec.State != "waiting" && rec.State != "claimed") ||
			(rec.State == "waiting" && status != string(RunWaiting)) ||
			(rec.State == "claimed" && status != string(RunQueued) && status != string(RunRunning)) {
			return nil, errInvalidCheckpoint
		}
		out = append(out, rec)
		if len(out) > maxDurableCheckpoints {
			return nil, errCheckpointCapacity
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_checkpoints`).Scan(&total); err != nil {
		return nil, err
	}
	if total != len(out) {
		return nil, errInvalidCheckpoint
	}
	return out, nil
}

// claimedDurableState authenticates the ciphertext only for the current lease.
func (s *Store) claimedDurableState(ctx context.Context, runID string, generation int64, owner string, now time.Time, keys CheckpointKeyProvider) (durableCheckpointState, error) {
	if keys == nil || runID == "" || generation <= 0 || owner == "" || now.IsZero() {
		return durableCheckpointState{}, errInvalidCheckpoint
	}
	var sealed []byte
	var deadline, flowID, triggerNode, mode, status string
	var revision int
	err := s.db.QueryRowContext(ctx, `SELECT c.ciphertext,c.deadline,r.flow_id,r.revision,r.trigger_node,r.mode,r.status
		FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id
		WHERE c.run_id=? AND c.generation=? AND c.state='claimed' AND c.lease_owner=? AND c.lease_until>? AND length(c.ciphertext)<=?`,
		runID, generation, owner, formatTime(now), maxCheckpointBytes).Scan(&sealed, &deadline, &flowID, &revision, &triggerNode, &mode, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return durableCheckpointState{}, errCheckpointConflict
	}
	if err != nil {
		return durableCheckpointState{}, err
	}
	key, err := keys.CheckpointKey(ctx, false)
	if err != nil {
		return durableCheckpointState{}, fmt.Errorf("read checkpoint key: %w", err)
	}
	defer clear(key)
	plain, err := openCheckpoint(key, runID, uint64(generation), sealed)
	if err != nil {
		return durableCheckpointState{}, err
	}
	defer clear(plain)
	state, err := decodeDurableState(plain)
	if err != nil || state.RunID != runID || state.Flow.ID != flowID || state.Revision != revision || state.TriggerNode != triggerNode ||
		mode != string(ModeLive) || (status != string(RunQueued) && status != string(RunRunning)) || formatTime(state.Deadline) != deadline {
		return durableCheckpointState{}, errInvalidCheckpoint
	}
	return state, nil
}

// cancelDurableRunTx invalidates the lease and terminalizes the run together.
func (s *Store) cancelDurableRunTx(ctx context.Context, tx *sql.Tx, runID string, now time.Time, code, message string) (bool, error) {
	if runID == "" || now.IsZero() {
		return false, errInvalidCheckpoint
	}
	result, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status='cancelled',error_code=?,error_message=?,finished_at=?
		WHERE id=? AND status IN ('queued','running','waiting') AND EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=?)`,
		code, s.scrubText(message), formatTime(now), runID, runID)
	if err != nil {
		return false, fmt.Errorf("cancel durable run: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	result, err = tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=?`, runID)
	if err != nil {
		return false, err
	}
	if n, err = result.RowsAffected(); err != nil || n != 1 {
		return false, errCheckpointConflict
	}
	return true, nil
}
