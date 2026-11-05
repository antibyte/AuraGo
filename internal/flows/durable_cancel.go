package flows

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CancelDurableRun acknowledges only a fenced checkpoint deletion, or an
// accepted cancellation of a still-uncheckpointed worker. Terminal legacy
// rows retain their status when their incompatible checkpoint is removed.
func (s *Service) CancelDurableRun(ctx context.Context, runID string, expectedGeneration int64) (bool, error) {
	return s.cancelDurableRun(ctx, runID, expectedGeneration, "")
}

// CancelDurableRunAs records the authenticated local administrator when the
// caller has one. Stop remains key-independent and generation fenced.
func (s *Service) CancelDurableRunAs(ctx context.Context, runID string, expectedGeneration int64, actor string) (bool, error) {
	if actor != "local_admin" {
		actor = ""
	}
	return s.cancelDurableRun(ctx, runID, expectedGeneration, actor)
}

func (s *Service) cancelDurableRun(ctx context.Context, runID string, expectedGeneration int64, actor string) (bool, error) {
	if runID == "" || len(runID) > 128 || expectedGeneration < 0 {
		return false, ErrDurableGenerationConflict
	}
	header, err := s.store.GetRunHeader(ctx, runID)
	if err != nil {
		return false, err
	}
	if _, err := s.requireFlow(ctx, header.FlowID); err != nil {
		return false, err
	}
	tx, release, err := beginBlockWrite(ctx, s.store.db, header.FlowID)
	if err != nil {
		return false, err
	}
	released := false
	defer func() {
		if !released {
			release()
		}
	}()
	defer tx.Rollback()
	var marker int
	var status string
	var generation sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT r.durable,r.status,c.generation FROM flow_runs r LEFT JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.id=?`, runID).Scan(&marker, &status, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrRunNotFound
	}
	if err != nil {
		return false, err
	}
	if !generation.Valid {
		if expectedGeneration != 0 {
			return false, ErrDurableGenerationConflict
		}
		if marker != 1 || RunStatus(status).Terminal() {
			return false, nil
		}
		updated, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status='cancelled',error_code='FLOW_CANCELLED',error_message='the run was cancelled',finished_at=? WHERE id=? AND durable=1 AND status IN ('queued','running') AND NOT EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=?)`, formatTime(s.now()), runID, runID)
		if err != nil {
			return false, err
		}
		if n, err := updated.RowsAffected(); err != nil || n != 1 {
			return false, ErrDurableGenerationConflict
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		release()
		released = true
		s.runner.mu.Lock()
		run, active := s.runner.cancels[runID]
		if active {
			run.cancelled = true
			s.runner.cancels[runID] = run
		}
		pending := s.runner.removeQueuedLocked(runID)
		s.runner.mu.Unlock()
		if active {
			run.cancel()
		}
		if pending != nil {
			s.runner.releaseDurableToken(pending)
		}
		s.runner.bus.Finish(runID)
		s.runner.projectDurable(runID)
		return true, nil
	}
	if expectedGeneration != generation.Int64 {
		return false, ErrDurableGenerationConflict
	}
	if !RunStatus(status).Terminal() {
		_, err := tx.ExecContext(ctx, `UPDATE flow_durable_gates SET state='cancelled',decision='',actor=?,decided_at=? WHERE run_id=? AND state='pending'`, actor, formatTime(s.now()), runID)
		if err != nil {
			return false, err
		}
	}
	if !RunStatus(status).Terminal() {
		res, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status='cancelled',error_code='FLOW_CANCELLED',error_message='the run was cancelled',finished_at=? WHERE id=? AND status IN ('queued','running','waiting')`, formatTime(s.now()), runID)
		if err != nil {
			return false, err
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			return false, ErrDurableGenerationConflict
		}
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=? AND generation=?`, runID, expectedGeneration)
	if err != nil {
		return false, err
	}
	if n, err := deleted.RowsAffected(); err != nil || n != 1 {
		return false, ErrDurableGenerationConflict
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	release()
	released = true
	s.runner.mu.Lock()
	run, active := s.runner.cancels[runID]
	if active {
		run.cancelled = true
		s.runner.cancels[runID] = run
	}
	pending := s.runner.removeQueuedLocked(runID)
	_, reserved := s.runner.durable[runID]
	delete(s.runner.durable, runID)
	if reserved && !active && (pending == nil || !pending.counted) && s.runner.live[header.FlowID] > 0 {
		s.runner.releaseFlowLocked(header.FlowID)
	}
	s.runner.mu.Unlock()
	if active {
		run.cancel()
	}
	if pending != nil {
		s.runner.releaseDurableToken(pending)
	}
	s.runner.bus.Finish(runID)
	if marker == 1 {
		s.runner.projectDurable(runID)
	}
	return true, nil
}

func (s *Store) startDurableRun(ctx context.Context, runID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status='running' WHERE id=? AND durable=1 AND status='queued'`, runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrDurableGenerationConflict)
}

func (s *Store) finishQueuedDurable(ctx context.Context, runID, code, message string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status='cancelled',error_code=?,error_message=?,finished_at=? WHERE id=? AND durable=1 AND status='queued' AND NOT EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=?)`, code, s.scrubText(message), formatTime(now), runID, runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrDurableGenerationConflict)
}

func (s *Store) startClaimedDurableRun(ctx context.Context, runID string, generation int64, owner string, guard durableClaimGuard, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockDurableGuardTx(ctx, tx); err != nil {
		return err
	}
	if err := verifyDurableClaimGuard(ctx, tx, runID, guard); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status='running' WHERE id=? AND durable=1 AND status='queued' AND flow_id=? AND revision=? AND EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=? AND lease_until>?)`, runID, guard.flowID, guard.revision, runID, generation, owner, formatTime(now))
	if err != nil {
		return err
	}
	if err := requireRow(res, errCheckpointConflict); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) finishUncheckpointedDurable(ctx context.Context, runID string, result RunResult) error {
	if !result.Status.Terminal() || result.FinishedAt.IsZero() {
		return ErrDurableUnavailable
	}
	res, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET status=?,error_code=?,error_message=?,finished_at=?,duration_ms=? WHERE id=? AND durable=1 AND status='running' AND NOT EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=?)`, string(result.Status), result.ErrorCode, s.scrubText(result.ErrorMessage), formatTime(result.FinishedAt), result.DurationMS, runID, runID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrDurableGenerationConflict)
}
