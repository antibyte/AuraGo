package flows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	maxDurableCheckpoints = 64
	maxDurableBytes       = 512 << 20
)

var (
	errCheckpointConflict = errors.New("checkpoint changed elsewhere")
	errCheckpointCapacity = errors.New("checkpoint capacity exhausted")
)

// This schema is installed only by the reviewed, backed-up version migration.
const durableCheckpointSchema = `CREATE TABLE flow_checkpoints (
	run_id TEXT PRIMARY KEY REFERENCES flow_runs(id) ON DELETE CASCADE,
	generation INTEGER NOT NULL CHECK (generation > 0),
	state TEXT NOT NULL CHECK (state IN ('waiting', 'claimed')),
	wake_at TEXT NOT NULL,
	deadline TEXT NOT NULL,
	ciphertext BLOB NOT NULL,
	lease_owner TEXT NOT NULL DEFAULT '',
	lease_until TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
)`

type durableCheckpointRecord struct {
	RunID      string
	Generation int64
	WakeAt     time.Time
	Deadline   time.Time
	Ciphertext []byte
}

// CheckpointKeyProvider returns a fresh 32-byte copy; create is allowed only
// under the checkpoint admission lock when no ciphertext exists.
type CheckpointKeyProvider interface {
	CheckpointKey(context.Context, bool) ([]byte, error)
}

func withCheckpointKey(ctx context.Context, db *sql.DB, provider CheckpointKeyProvider, fn func(*sql.Tx, []byte) error) error {
	if provider == nil {
		return errInvalidCheckpoint
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin checkpoint admission: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`)
	if err != nil {
		return fmt.Errorf("lock checkpoint admission: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errInvalidCheckpoint
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_checkpoints`).Scan(&count); err != nil {
		return fmt.Errorf("read checkpoints: %w", err)
	}
	key, err := provider.CheckpointKey(ctx, count == 0)
	if err != nil {
		return fmt.Errorf("read checkpoint key: %w", err)
	}
	defer clear(key)
	if len(key) != 32 {
		return errInvalidCheckpoint
	}
	if err := fn(tx, key); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit checkpoint admission: %w", err)
	}
	return nil
}

// saveDurableCheckpoint commits full ciphertext and running→waiting together.
// expected is zero for a new run, otherwise its current checkpoint generation.
func saveDurableCheckpoint(ctx context.Context, db *sql.DB, rec durableCheckpointRecord, expected int64, owner string, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin checkpoint: %w", err)
	}
	defer tx.Rollback()
	if err := saveDurableCheckpointTx(ctx, tx, rec, expected, owner, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit checkpoint: %w", err)
	}
	return nil
}

// saveDurableCheckpointTx is called inside withCheckpointKey in production so
// the empty-store proof and first encrypted insert hold one write lock.
func saveDurableCheckpointTx(ctx context.Context, tx *sql.Tx, rec durableCheckpointRecord, expected int64, owner string, now time.Time) error {
	if rec.RunID == "" || len(rec.RunID) > 128 || expected < 0 || rec.Generation <= 0 || rec.Generation != expected+1 || (expected > 0 && (owner == "" || len(owner) > 128)) || len(rec.Ciphertext) == 0 || len(rec.Ciphertext) > maxCheckpointBytes ||
		wakeOrDeadlineInvalid(rec.WakeAt, rec.Deadline, now) {
		return errInvalidCheckpoint
	}
	var count, used int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(length(ciphertext)), 0) FROM flow_checkpoints WHERE run_id <> ?`, rec.RunID).Scan(&count, &used); err != nil {
		return fmt.Errorf("read checkpoint capacity: %w", err)
	}
	if count >= maxDurableCheckpoints || used+int64(len(rec.Ciphertext)) > maxDurableBytes {
		return errCheckpointCapacity
	}
	wake := rec.WakeAt.UTC().Truncate(time.Microsecond)
	if wake.Before(rec.WakeAt) {
		wake = wake.Add(time.Microsecond)
	}
	if !wake.Before(rec.Deadline.UTC().Truncate(time.Microsecond)) {
		return errInvalidCheckpoint
	}
	var result sql.Result
	var err error
	if expected == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at)
			SELECT ?,?,'waiting',?,?,?,? WHERE NOT EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=?)`,
			rec.RunID, rec.Generation, formatTime(wake), formatTime(rec.Deadline), rec.Ciphertext, formatTime(now), rec.RunID)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE flow_checkpoints SET generation=?,state='waiting',wake_at=?,deadline=?,ciphertext=?,
			lease_owner='',lease_until='',updated_at=? WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=? AND lease_until>? AND deadline=?`,
			rec.Generation, formatTime(wake), formatTime(rec.Deadline), rec.Ciphertext, formatTime(now), rec.RunID, expected, owner, formatTime(now), formatTime(rec.Deadline))
	}
	if err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errCheckpointConflict
	}
	result, err = tx.ExecContext(ctx, `UPDATE flow_runs SET status='waiting' WHERE id=? AND status='running'`, rec.RunID)
	if err != nil {
		return fmt.Errorf("pause checkpoint run: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errCheckpointConflict
	}
	return nil
}

// saveWaitingCheckpointTx records the waiting step with the checkpoint; the
// caller commits the transaction acquired by withCheckpointKey.
func (s *Store) saveWaitingCheckpointTx(ctx context.Context, tx *sql.Tx, rec durableCheckpointRecord, expected int64,
	owner string, now time.Time, seq int, step StepRecord) error {
	if seq <= 0 || step.Status != StepWaiting {
		return errInvalidCheckpoint
	}
	if err := saveDurableCheckpointTx(ctx, tx, rec, expected, owner, now); err != nil {
		return err
	}
	if err := s.saveStepOn(ctx, tx, rec.RunID, seq, step, false); err != nil {
		return fmt.Errorf("save waiting step: %w", err)
	}
	return nil
}

func wakeOrDeadlineInvalid(wake, deadline, now time.Time) bool {
	return wake.IsZero() || deadline.IsZero() || now.IsZero() || wake.After(deadline) || !deadline.After(now)
}

// saveActiveCheckpointTx fences a resumed worker and persists its completed
// step with its full coordinator state before successors may be dispatched.
func (s *Store) saveActiveCheckpointTx(ctx context.Context, tx *sql.Tx, rec durableCheckpointRecord, expected int64,
	owner string, now, leaseUntil time.Time, seq int, step StepRecord) error {
	if rec.RunID == "" || len(rec.RunID) > 128 || expected <= 0 || rec.Generation <= 0 || rec.Generation != expected+1 ||
		owner == "" || len(owner) > 128 || now.IsZero() || rec.Deadline.IsZero() || !leaseUntil.After(now) || seq <= 0 || len(rec.Ciphertext) == 0 || len(rec.Ciphertext) > maxCheckpointBytes {
		return errInvalidCheckpoint
	}
	var used int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(ciphertext)),0) FROM flow_checkpoints WHERE run_id<>?`, rec.RunID).Scan(&used); err != nil {
		return fmt.Errorf("read checkpoint capacity: %w", err)
	}
	if used+int64(len(rec.Ciphertext)) > maxDurableBytes {
		return errCheckpointCapacity
	}
	result, err := tx.ExecContext(ctx, `UPDATE flow_checkpoints SET generation=?,ciphertext=?,lease_until=?,updated_at=?
		WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=? AND lease_until>? AND deadline>? AND deadline=?
		AND EXISTS (SELECT 1 FROM flow_runs WHERE id=? AND status='running')`, rec.Generation, rec.Ciphertext, formatTime(leaseUntil),
		formatTime(now), rec.RunID, expected, owner, formatTime(now), formatTime(now), formatTime(rec.Deadline), rec.RunID)
	if err != nil {
		return fmt.Errorf("save active checkpoint: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errCheckpointConflict
	}
	if err := s.saveStepOn(ctx, tx, rec.RunID, seq, step, false); err != nil {
		return fmt.Errorf("save checkpoint step: %w", err)
	}
	return nil
}

// finishDurableCheckpointTx fences completion and removes its resumable state
// in the same transaction. A stale lease cannot finish or delete the run.
func (s *Store) finishDurableCheckpointTx(ctx context.Context, tx *sql.Tx, runID string, generation int64,
	owner string, now time.Time, result RunResult) error {
	if runID == "" || len(runID) > 128 || generation <= 0 || owner == "" || len(owner) > 128 || now.IsZero() ||
		!result.Status.Terminal() || result.FinishedAt.IsZero() {
		return errInvalidCheckpoint
	}
	updated, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status=?,error_code=?,error_message=?,finished_at=?,duration_ms=?
		WHERE id=? AND status='running' AND EXISTS (SELECT 1 FROM flow_checkpoints
		WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=? AND lease_until>?)`,
		string(result.Status), result.ErrorCode, s.scrubText(result.ErrorMessage), formatTime(result.FinishedAt), result.DurationMS,
		runID, runID, generation, owner, formatTime(now))
	if err != nil {
		return fmt.Errorf("finish durable run: %w", err)
	}
	if n, err := updated.RowsAffected(); err != nil || n != 1 {
		return errCheckpointConflict
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=?`, runID, generation, owner)
	if err != nil {
		return fmt.Errorf("remove completed checkpoint: %w", err)
	}
	if n, err := deleted.RowsAffected(); err != nil || n != 1 {
		return errCheckpointConflict
	}
	return nil
}

// claimDurableCheckpoint permits exactly one lease for a due pure-run generation.
// Effectful recovery must never reuse this reclaim path without receipts.
func claimDurableCheckpoint(ctx context.Context, db *sql.DB, runID string, generation int64, owner string, now, leaseUntil time.Time) (bool, error) {
	if runID == "" || len(runID) > 128 || generation <= 0 || owner == "" || len(owner) > 128 || now.IsZero() || !leaseUntil.After(now) {
		return false, errInvalidCheckpoint
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin checkpoint claim: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE flow_checkpoints SET state='claimed',lease_owner=?,lease_until=?,updated_at=?
		WHERE run_id=? AND generation=? AND wake_at<=? AND deadline>? AND (
		(state='waiting' AND EXISTS (SELECT 1 FROM flow_runs WHERE id=? AND status='waiting')) OR
		(state='claimed' AND lease_until<=? AND EXISTS (SELECT 1 FROM flow_runs WHERE id=? AND status IN ('queued','running'))))`,
		owner, formatTime(leaseUntil), formatTime(now), runID, generation, formatTime(now), formatTime(now), runID, formatTime(now), runID)
	if err != nil {
		return false, fmt.Errorf("claim checkpoint: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return false, nil
	}
	result, err = tx.ExecContext(ctx, `UPDATE flow_runs SET status='queued' WHERE id=? AND status IN ('waiting','queued','running')`, runID)
	if err != nil {
		return false, fmt.Errorf("queue checkpoint run: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return false, errCheckpointConflict
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit checkpoint claim: %w", err)
	}
	return true, nil
}
