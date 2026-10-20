package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/uid"
)

const (
	PrivacyDeletionPreparing             = "preparing"
	PrivacyDeletionAwaitingContactDelete = "awaiting_contact_delete"
	PrivacyDeletionPending               = "pending"
	PrivacyDeletionProcessing            = "processing"
	PrivacyDeletionRetryWait             = "retry_wait"
	PrivacyDeletionRetainedReview        = "retained_review"
	PrivacyDeletionReconcilePending      = "reconcile_pending"
	PrivacyDeletionCompleted             = "completed"
	PrivacyDeletionAborted               = "aborted"
	privacyDeletionRetryInitial          = 5 * time.Minute
	privacyDeletionRetryMaximum          = 24 * time.Hour
	privacyDeletionTerminalRetention     = 30 * 24 * time.Hour
	privacyDeletionSnapshotMarkerPrefix  = "snapshot.publication.privacy."
	privacyDeletionDefaultPageSize       = 50
	privacyDeletionMaximumPageSize       = 100
)

type PrivacyDeletionJob struct {
	DeletionJobID     string `json:"deletion_job_id"`
	Kind              string `json:"kind"`
	DeletionStatus    string `json:"deletion_status"`
	DeleteMemories    bool   `json:"delete_memories"`
	TranscriptDeleted bool   `json:"transcript_deleted"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	NextRetryAt       string `json:"next_retry_at,omitempty"`
	AttemptCount      int    `json:"attempt_count"`
	PendingCount      int    `json:"pending_count"`
	DeletedCount      int    `json:"deleted_count"`
	RetainedCount     int    `json:"retained_count"`
	ErrorCode         string `json:"error_code,omitempty"`
}

type privacyDeletionTarget struct {
	Store               string
	TargetID            string
	ExpectedFingerprint string
	ContentSHA256       string
	SourceMarker        string
	State               string
	ReasonCode          string
}

const privacyDeletionSchema = `
CREATE TABLE IF NOT EXISTS privacy_deletion_jobs (
	deletion_job_id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	deletion_status TEXT NOT NULL,
	delete_memories INTEGER NOT NULL DEFAULT 0,
	contact_source_id TEXT NOT NULL DEFAULT '',
	contact_operation_token TEXT NOT NULL DEFAULT '',
	session_hash TEXT NOT NULL DEFAULT '',
	transcript_deleted INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	next_retry_at TEXT NOT NULL,
	attempt_count INTEGER NOT NULL DEFAULT 0,
	pending_count INTEGER NOT NULL DEFAULT 0,
	deleted_count INTEGER NOT NULL DEFAULT 0,
	retained_count INTEGER NOT NULL DEFAULT 0,
	error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_privacy_deletion_due ON privacy_deletion_jobs(deletion_status,next_retry_at,created_at);
CREATE TABLE IF NOT EXISTS privacy_deletion_targets (
	deletion_job_id TEXT NOT NULL,
	store TEXT NOT NULL,
	target_id TEXT NOT NULL,
	expected_fingerprint TEXT NOT NULL DEFAULT '',
	content_sha256 TEXT NOT NULL DEFAULT '',
	source_marker TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'pending',
	reason_code TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(deletion_job_id,store,target_id),
	FOREIGN KEY(deletion_job_id) REFERENCES privacy_deletion_jobs(deletion_job_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_privacy_deletion_targets_state ON privacy_deletion_targets(deletion_job_id,state);
`

func (s *SQLiteMemory) initPrivacyDeletionJobs() error {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='privacy_deletion_jobs')`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect privacy deletion migration: %w", err)
	}
	if !exists {
		var populated bool
		if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM messages) OR EXISTS(SELECT 1 FROM memory_meta)`).Scan(&populated); err != nil {
			return fmt.Errorf("inspect memory before privacy deletion migration: %w", err)
		}
		if populated {
			if _, err := backupMainDatabase(s.db, "privacy-deletions-v1"); err != nil {
				return fmt.Errorf("back up memory before privacy deletion migration: %w", err)
			}
		}
	}
	if _, err := s.db.Exec(privacyDeletionSchema); err != nil {
		return fmt.Errorf("create privacy deletion queue: %w", err)
	}
	var hasSessionHash, hasTranscriptDeleted bool
	if err := s.db.QueryRow(`SELECT
		EXISTS(SELECT 1 FROM pragma_table_info('privacy_deletion_jobs') WHERE name='session_hash'),
		EXISTS(SELECT 1 FROM pragma_table_info('privacy_deletion_jobs') WHERE name='transcript_deleted')`).Scan(&hasSessionHash, &hasTranscriptDeleted); err != nil {
		return fmt.Errorf("inspect privacy deletion metadata migration: %w", err)
	}
	if !hasSessionHash || !hasTranscriptDeleted {
		if _, err := backupMainDatabase(s.db, "privacy-deletions-v2"); err != nil {
			return fmt.Errorf("back up database before privacy deletion metadata migration: %w", err)
		}
	}
	if !hasSessionHash {
		if _, err := s.db.Exec(`ALTER TABLE privacy_deletion_jobs ADD COLUMN session_hash TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate privacy deletion session hash: %w", err)
		}
	}
	if !hasTranscriptDeleted {
		if _, err := s.db.Exec(`ALTER TABLE privacy_deletion_jobs ADD COLUMN transcript_deleted INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate privacy deletion transcript state: %w", err)
		}
	}
	return nil
}

func (s *SQLiteMemory) ListPrivacyDeletionJobs(ctx context.Context, limit int) ([]PrivacyDeletionJob, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("privacy deletion store is unavailable")
	}
	if limit <= 0 {
		limit = privacyDeletionDefaultPageSize
	}
	if limit > privacyDeletionMaximumPageSize {
		limit = privacyDeletionMaximumPageSize
	}
	cutoff := time.Now().UTC().Add(-privacyDeletionTerminalRetention).Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `SELECT deletion_job_id,kind,deletion_status,delete_memories,transcript_deleted,created_at,updated_at,next_retry_at,attempt_count,pending_count,deleted_count,retained_count,error_code
		FROM privacy_deletion_jobs WHERE deletion_status NOT IN (?,?,?) OR updated_at>=?
		ORDER BY CASE WHEN deletion_status IN ('completed','aborted','retained_review') THEN 1 ELSE 0 END,updated_at DESC,deletion_job_id LIMIT ?`, PrivacyDeletionCompleted, PrivacyDeletionAborted, PrivacyDeletionRetainedReview, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list privacy deletion jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]PrivacyDeletionJob, 0)
	for rows.Next() {
		job, err := scanPrivacyDeletionJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan privacy deletion job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate privacy deletion jobs: %w", err)
	}
	return jobs, nil
}

func (s *SQLiteMemory) GetPrivacyDeletionJob(ctx context.Context, id string) (PrivacyDeletionJob, error) {
	if s == nil || s.db == nil {
		return PrivacyDeletionJob{}, fmt.Errorf("privacy deletion store is unavailable")
	}
	job, err := scanPrivacyDeletionJob(s.db.QueryRowContext(ctx, `SELECT deletion_job_id,kind,deletion_status,delete_memories,transcript_deleted,created_at,updated_at,next_retry_at,attempt_count,pending_count,deleted_count,retained_count,error_code FROM privacy_deletion_jobs WHERE deletion_job_id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return PrivacyDeletionJob{}, ErrPrivacyDeletionJobNotFound
	}
	if err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("get privacy deletion job: %w", err)
	}
	return job, nil
}

var ErrPrivacyDeletionJobNotFound = errors.New("privacy deletion job not found")
var ErrPrivacyDeletionJobNotRetryable = errors.New("privacy deletion job is not retryable")

// PrivacyDeletionScopeConflictError means an idempotent session-deletion retry
// requested a different memory-deletion scope than the durable job selected.
type PrivacyDeletionScopeConflictError struct {
	ExistingDeleteMemories  bool
	RequestedDeleteMemories bool
}

func (e *PrivacyDeletionScopeConflictError) Error() string {
	return "chat deletion already exists with a different memory-deletion scope"
}

func (s *SQLiteMemory) RetryPrivacyDeletionJob(ctx context.Context, id string, now time.Time) (PrivacyDeletionJob, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("begin privacy deletion retry: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=CASE WHEN deletion_status=? THEN ? ELSE deletion_status END,next_retry_at=?,updated_at=?,error_code=''
		WHERE deletion_job_id=? AND deletion_status IN (?,?)`, PrivacyDeletionRetryWait, PrivacyDeletionPending, now.UTC().Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano), id, PrivacyDeletionRetryWait, PrivacyDeletionReconcilePending)
	if err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("schedule privacy deletion retry: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return PrivacyDeletionJob{}, err
	}
	if changed == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM privacy_deletion_jobs WHERE deletion_job_id=?)`, id).Scan(&exists); err != nil {
			return PrivacyDeletionJob{}, err
		}
		if !exists {
			return PrivacyDeletionJob{}, ErrPrivacyDeletionJobNotFound
		}
		return PrivacyDeletionJob{}, ErrPrivacyDeletionJobNotRetryable
	}
	if err := tx.Commit(); err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("commit privacy deletion retry: %w", err)
	}
	return s.GetPrivacyDeletionJob(ctx, id)
}

func (s *SQLiteMemory) createPrivacyDeletionJobTx(ctx context.Context, tx *sql.Tx, jobID, kind, status string, deleteMemories bool, contactSourceID, contactToken string, now time.Time, targets []privacyDeletionTarget) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	deleteFlag := 0
	if deleteMemories {
		deleteFlag = 1
	}
	pending, retained := 0, 0
	for _, target := range targets {
		switch target.State {
		case "pending":
			pending++
		case "retained_review":
			retained++
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO privacy_deletion_jobs(deletion_job_id,kind,deletion_status,delete_memories,contact_source_id,contact_operation_token,created_at,updated_at,next_retry_at,pending_count,deleted_count,retained_count)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, jobID, kind, status, deleteFlag, contactSourceID, contactToken, stamp, stamp, stamp, pending, 0, retained); err != nil {
		return fmt.Errorf("persist privacy deletion job: %w", err)
	}
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `INSERT INTO privacy_deletion_targets(deletion_job_id,store,target_id,expected_fingerprint,content_sha256,source_marker,state,reason_code)
			VALUES(?,?,?,?,?,?,?,?)`, jobID, target.Store, target.TargetID, target.ExpectedFingerprint, target.ContentSHA256, target.SourceMarker, target.State, target.ReasonCode); err != nil {
			return fmt.Errorf("persist privacy deletion target: %w", err)
		}
	}
	return nil
}

func (s *SQLiteMemory) setPrivacyPublicationMarkerTx(ctx context.Context, tx *sql.Tx, jobID string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, privacyDeletionSnapshotMarkerPrefix+jobID, "scrub")
	if err != nil {
		return fmt.Errorf("record privacy snapshot marker: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) clearPrivacyPublicationMarkerTx(ctx context.Context, tx *sql.Tx, jobID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=?`, privacyDeletionSnapshotMarkerPrefix+jobID); err != nil {
		return fmt.Errorf("clear privacy snapshot marker: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) setPrivacyJobStatusTx(ctx context.Context, tx *sql.Tx, id, status, errorCode string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,error_code=?,updated_at=? WHERE deletion_job_id=?`, status, errorCode, now.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update privacy deletion status: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) markPrivacyDeletionFailure(ctx context.Context, id, code string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var attempts int
	var status, kind string
	if err := tx.QueryRowContext(ctx, `SELECT attempt_count,deletion_status,kind FROM privacy_deletion_jobs WHERE deletion_job_id=?`, id).Scan(&attempts, &status, &kind); err != nil {
		return err
	}
	if status == PrivacyDeletionCompleted || status == PrivacyDeletionAborted || status == PrivacyDeletionRetainedReview {
		return nil
	}
	var pending, deleted, retained int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='pending' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='deleted' AND (?<>'chat' OR store IN ('ltm','kg_node','kg_edge','kg_claim')) THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='retained_review' THEN 1 ELSE 0 END),0) FROM privacy_deletion_targets WHERE deletion_job_id=?`, kind, id).Scan(&pending, &deleted, &retained); err != nil {
		return err
	}
	attempts++
	delay := privacyDeletionBackoff(attempts)
	stamp := now.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,attempt_count=?,next_retry_at=?,updated_at=?,error_code=?,pending_count=?,deleted_count=?,retained_count=? WHERE deletion_job_id=?`, PrivacyDeletionRetryWait, attempts, now.UTC().Add(delay).Format(time.RFC3339Nano), stamp, safePrivacyDeletionErrorCode(code), pending, deleted, retained, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteMemory) markPrivacyDeletionReconcilePending(ctx context.Context, id, code string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var attempts int
	var status, kind string
	if err := tx.QueryRowContext(ctx, `SELECT attempt_count,deletion_status,kind FROM privacy_deletion_jobs WHERE deletion_job_id=?`, id).Scan(&attempts, &status, &kind); err != nil {
		return err
	}
	if status == PrivacyDeletionCompleted || status == PrivacyDeletionAborted || status == PrivacyDeletionRetainedReview {
		return nil
	}
	var pending, deleted, retained int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='pending' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='deleted' AND (?<>'chat' OR store IN ('ltm','kg_node','kg_edge','kg_claim')) THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='retained_review' THEN 1 ELSE 0 END),0) FROM privacy_deletion_targets WHERE deletion_job_id=?`, kind, id).Scan(&pending, &deleted, &retained); err != nil {
		return err
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	attempts++
	nextRetry := now.UTC().Add(privacyDeletionBackoff(attempts)).Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,attempt_count=?,next_retry_at=?,updated_at=?,error_code=?,pending_count=?,deleted_count=?,retained_count=? WHERE deletion_job_id=?`, PrivacyDeletionReconcilePending, attempts, nextRetry, stamp, safePrivacyDeletionErrorCode(code), pending, deleted, retained, id); err != nil {
		return err
	}
	return tx.Commit()
}

func privacyDeletionBackoff(attempt int) time.Duration {
	delay := privacyDeletionRetryInitial
	for step := 1; step < attempt && delay < privacyDeletionRetryMaximum; step++ {
		if delay > privacyDeletionRetryMaximum/2 {
			return privacyDeletionRetryMaximum
		}
		delay *= 2
	}
	if delay > privacyDeletionRetryMaximum {
		return privacyDeletionRetryMaximum
	}
	return delay
}

func safePrivacyDeletionErrorCode(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	if code == "" {
		return "privacy_cleanup_failed"
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return "privacy_cleanup_failed"
		}
	}
	if len(code) > 64 {
		return "privacy_cleanup_failed"
	}
	return code
}

func scanPrivacyDeletionJob(row interface{ Scan(...any) error }) (PrivacyDeletionJob, error) {
	var job PrivacyDeletionJob
	var next string
	err := row.Scan(&job.DeletionJobID, &job.Kind, &job.DeletionStatus, &job.DeleteMemories, &job.TranscriptDeleted, &job.CreatedAt, &job.UpdatedAt, &next, &job.AttemptCount, &job.PendingCount, &job.DeletedCount, &job.RetainedCount, &job.ErrorCode)
	if err != nil {
		return PrivacyDeletionJob{}, err
	}
	if next != "" && next != job.UpdatedAt {
		job.NextRetryAt = next
	}
	return job, nil
}

func newPrivacyDeletionID() string { return uid.New() }

func privacyFingerprint(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(hash, "%d:", len(part))
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func privacyDeletionRetryable(status string) bool {
	return status == PrivacyDeletionRetryWait || status == PrivacyDeletionReconcilePending
}

func (s *SQLiteMemory) cleanupPrivacyDeletionHistory(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Add(-privacyDeletionTerminalRetention).Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `DELETE FROM privacy_deletion_jobs WHERE deletion_status IN (?,?,?) AND updated_at<?`, PrivacyDeletionCompleted, PrivacyDeletionAborted, PrivacyDeletionRetainedReview, cutoff)
	if err != nil {
		return fmt.Errorf("clean retained privacy deletion history: %w", err)
	}
	return nil
}

// CleanupExpiredPrivacyDeletionJobs removes content-free terminal job headers
// after the retention period. Target identities are removed at finalization.
func (s *SQLiteMemory) CleanupExpiredPrivacyDeletionJobs(ctx context.Context, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("privacy deletion store is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.cleanupPrivacyDeletionHistory(ctx, now)
}
