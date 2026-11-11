package flows

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrVersionPurged is returned when a rollback or a copy would read a version that the credential
// purge marked.
var ErrVersionPurged = errors.New("this version was marked for credential cleanup")

// versionPinnedSQL is the SQL predicate "a run, a block pin or a pending gate still needs this
// version". flowExpr and revExpr are SQL expressions for the version's flow id and revision. It is
// the pin rule of pruneVersionsSQL plus pending durable gates (their run is waiting anyway).
func versionPinnedSQL(flowExpr, revExpr string) string {
	return `(EXISTS (SELECT 1 FROM flow_runs r WHERE r.flow_id = ` + flowExpr + ` AND r.revision = ` + revExpr + ` AND r.mode = 'live' AND r.status IN ('queued', 'running', 'waiting'))
	OR EXISTS (SELECT 1 FROM flow_block_version_pins p WHERE p.block_flow_id = ` + flowExpr + ` AND p.block_revision = ` + revExpr + `)
	OR EXISTS (SELECT 1 FROM flow_block_run_pins p WHERE p.block_flow_id = ` + flowExpr + ` AND p.block_revision = ` + revExpr + `)
	OR EXISTS (SELECT 1 FROM flow_durable_gates g WHERE g.flow_id = ` + flowExpr + ` AND g.revision = ` + revExpr + ` AND g.state = 'pending'))`
}

// MarkVersionsForPurge marks published versions of one flow. The live revision is never marked.
func (s *Store) MarkVersionsForPurge(ctx context.Context, flowID string, revisions []int, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, revision := range revisions {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO flow_version_purge (flow_id, revision, marked_at)
			SELECT v.flow_id, v.revision, ? FROM flow_versions v JOIN flows f ON f.id = v.flow_id
			WHERE v.flow_id = ? AND v.revision = ? AND v.revision <> f.live_revision`, formatTime(now), flowID, revision); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PrunePurgedVersions deletes the marked versions of flowID (every flow when it is empty) that
// nothing pins, with their notes (cascade) and marks (cascade). It deletes with secure_delete on
// and checkpoints the WAL afterwards, so a deleted version leaves no readable copy behind. It returns
// the number of deleted versions.
func (s *Store) PrunePurgedVersions(ctx context.Context, flowID string) (int64, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM flow_version_purge LIMIT 1`).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		return 0, err
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA secure_delete = OFF`) }()
	res, err := conn.ExecContext(ctx, `DELETE FROM flow_versions
		WHERE (? = '' OR flow_id = ?)
		AND EXISTS (SELECT 1 FROM flow_version_purge m WHERE m.flow_id = flow_versions.flow_id AND m.revision = flow_versions.revision)
		AND revision < COALESCE((SELECT live_revision FROM flows f WHERE f.id = flow_versions.flow_id), 0)
		AND NOT `+versionPinnedSQL("flow_versions.flow_id", "flow_versions.revision"), flowID, flowID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		// Best effort: a reader may hold the WAL; the next checkpoint finishes the job.
		_, _ = conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	return n, nil
}

// prunePurgedAfterRun is called when a run ends: the version it pinned may have been waiting for it.
func (s *Store) prunePurgedAfterRun(ctx context.Context, runID string) {
	var flowID string
	if err := s.db.QueryRowContext(ctx, `SELECT flow_id FROM flow_runs WHERE id = ?`, runID).Scan(&flowID); err != nil {
		return
	}
	if _, err := s.PrunePurgedVersions(ctx, flowID); err != nil {
		s.logger.Warn("marked flow versions could not be deleted", "flow_id", flowID, "error", err)
	}
}

// GetVersionForCopy is GetVersion for the callers that copy a version into a draft (rollback, clone).
// A version marked for purge is refused with ErrVersionPurged.
func (s *Store) GetVersionForCopy(ctx context.Context, id string, revision int) (*Flow, error) {
	var marked int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_version_purge WHERE flow_id = ? AND revision = ?`, id, revision).Scan(&marked); err != nil {
		return nil, err
	}
	if marked > 0 {
		return nil, ErrVersionPurged
	}
	return s.GetVersion(ctx, id, revision)
}
