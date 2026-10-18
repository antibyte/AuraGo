package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"aurago/internal/contacts"
)

// ContactPrivacyDeletionResult distinguishes a confirmed contact outcome from
// an accepted, durable reconciliation. Its job contains no contact identity.
type ContactPrivacyDeletionResult struct {
	Job            PrivacyDeletionJob
	ContactDeleted bool
	OutcomeKnown   bool
}

// DeleteContactWithPrivacy is the only privacy-job path that calls DELETE.
// Startup and manual retries exclusively reconcile existence and memory targets.
func (s *SQLiteMemory) DeleteContactWithPrivacy(ctx context.Context, db *sql.DB, kg *KnowledgeGraph, id string, deleteMemories bool) (result ContactPrivacyDeletionResult, outErr error) {
	if s == nil || s.db == nil || db == nil || strings.TrimSpace(id) == "" {
		return result, fmt.Errorf("contact deletion stores and identity are required")
	}
	unlock, err := contacts.LockIDContext(ctx, id)
	if err != nil {
		return result, err
	}
	defer unlock()
	if _, err := contacts.GetByIDContext(ctx, db, id); err != nil {
		if errors.Is(err, contacts.ErrNotFound) {
			result.ContactDeleted, result.OutcomeKnown = true, true
		}
		return result, err
	}
	result.OutcomeKnown = true
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	source, token, jobID := "contact_"+id, newPrivacyDeletionID(), newPrivacyDeletionID()
	var targets []privacyDeletionTarget
	if kg != nil {
		targets, err = kg.prepareContactPrivacyTargets(ctx, source, deleteMemories)
		if err != nil {
			return result, err
		}
	} else if deleteMemories {
		return result, fmt.Errorf("knowledge graph is unavailable for contact deletion preparation")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err := s.createPrivacyDeletionJobTx(ctx, tx, jobID, "contact", PrivacyDeletionAwaitingContactDelete, deleteMemories, source, token, time.Now().UTC(), targets); err != nil {
		return result, err
	}
	if err := s.setPrivacyPublicationMarkerTx(ctx, tx, jobID); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	// Once journaled, a failed request must still return the durable job identity.
	defer func() {
		readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		job, err := s.GetPrivacyDeletionJob(readCtx, jobID)
		if err == nil {
			result.Job = job
		} else {
			result.Job.DeletionJobID = jobID
			result.Job.DeletionStatus = PrivacyDeletionReconcilePending
			outErr = errors.Join(outErr, err)
		}
	}()
	if kg != nil {
		if err := kg.BeginContactSourceDeletion(ctx, source, token); err != nil {
			// This operation never deletes a pre-existing or legacy fence.
			ownedToken, _, exists, readErr := kg.ContactSourceFence(ctx, source)
			if readErr == nil && (!exists || ownedToken != token) {
				result.OutcomeKnown = true
				return result, errors.Join(err, s.finishContactPrivacyAbort(ctx, jobID))
			}
			_ = s.deferContactReconciliation(ctx, jobID, "contact_fence_uncertain", time.Now().UTC())
			return result, err
		}
	}
	deleteErr := contacts.DeleteLockedContext(ctx, db, id)
	if deleteErr == nil {
		result.ContactDeleted, result.OutcomeKnown = true, true
		if err := s.confirmContactPrivacyDeletion(ctx, kg, jobID, source, token); err != nil {
			_ = s.deferContactReconciliation(ctx, jobID, "contact_confirmation_pending", time.Now().UTC())
			return result, err
		}
		return result, s.processContactPrivacyTargets(ctx, kg, jobID, time.Now().UTC())
	}
	// A cancellation or SQLite error is not evidence the DELETE rolled back.
	checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	deleted, known, reconcileErr := s.reconcileContactPrivacyDeletion(checkCtx, db, kg, jobID, source, token, time.Now().UTC())
	result.ContactDeleted, result.OutcomeKnown = deleted, known
	if known && deleted {
		return result, reconcileErr
	}
	return result, errors.Join(deleteErr, reconcileErr)
}

// ProcessDueContactPrivacyDeletionJobs NEVER calls contacts.Delete. A pending
// contact operation is resolved by observing the existing row under the same
// per-contact lock used by updates, synchronization, and the initial DELETE.
func (s *SQLiteMemory) ProcessDueContactPrivacyDeletionJobs(ctx context.Context, db *sql.DB, kg *KnowledgeGraph, now time.Time, limit int) error {
	if s == nil || s.db == nil || db == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `SELECT deletion_job_id,contact_source_id,contact_operation_token FROM privacy_deletion_jobs
		WHERE kind='contact' AND deletion_status IN (?,?,?,?,?) AND next_retry_at<=? ORDER BY next_retry_at,created_at LIMIT ?`, PrivacyDeletionAwaitingContactDelete, PrivacyDeletionReconcilePending, PrivacyDeletionPending, PrivacyDeletionProcessing, PrivacyDeletionRetryWait, now.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return err
	}
	type operation struct{ job, source, token string }
	var operations []operation
	for rows.Next() {
		var op operation
		if err := rows.Scan(&op.job, &op.source, &op.token); err != nil {
			rows.Close()
			return err
		}
		operations = append(operations, op)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var combined error
	for _, op := range operations {
		if err := ctx.Err(); err != nil {
			return errors.Join(combined, err)
		}
		if op.token == "" || !strings.HasPrefix(op.source, "contact_") {
			// Legacy fences and anonymous markers are never invented into jobs.
			continue
		}
		err := func() error {
			unlock, err := contacts.LockIDContext(ctx, strings.TrimPrefix(op.source, "contact_"))
			if err != nil {
				return err
			}
			defer unlock()
			ctx, release, err := s.BeginMemoryPublication(ctx)
			if err != nil {
				return err
			}
			defer release()
			job, err := s.GetPrivacyDeletionJob(ctx, op.job)
			if err != nil {
				return err
			}
			switch job.DeletionStatus {
			case PrivacyDeletionAwaitingContactDelete, PrivacyDeletionReconcilePending:
				_, _, err = s.reconcileContactPrivacyDeletion(ctx, db, kg, op.job, op.source, op.token, now)
				return err
			case PrivacyDeletionPending, PrivacyDeletionProcessing, PrivacyDeletionRetryWait:
				return s.processContactPrivacyTargets(ctx, kg, op.job, now)
			default:
				return nil
			}
		}()
		combined = errors.Join(combined, err)
	}
	return combined
}

func (s *SQLiteMemory) reconcileContactPrivacyDeletion(ctx context.Context, db *sql.DB, kg *KnowledgeGraph, jobID, source, token string, now time.Time) (deleted, known bool, err error) {
	_, readErr := contacts.GetByIDContext(ctx, db, strings.TrimPrefix(source, "contact_"))
	if readErr == nil {
		if kg != nil {
			fenceToken, _, exists, err := kg.ContactSourceFence(ctx, source)
			if err != nil {
				_ = s.deferContactReconciliation(ctx, jobID, "contact_fence_read_failed", now)
				return false, true, err
			}
			if exists && fenceToken == token {
				if err := kg.ReleaseContactSourceDeletion(ctx, source, token); err != nil {
					_ = s.deferContactReconciliation(ctx, jobID, "contact_fence_release_failed", now)
					return false, true, err
				}
			}
		}
		return false, true, s.finishContactPrivacyAbort(ctx, jobID)
	}
	if !errors.Is(readErr, contacts.ErrNotFound) {
		return false, false, errors.Join(readErr, s.deferContactReconciliation(ctx, jobID, "contact_outcome_unknown", now))
	}
	if err := s.confirmContactPrivacyDeletion(ctx, kg, jobID, source, token); err != nil {
		_ = s.deferContactReconciliation(ctx, jobID, "contact_confirmation_pending", now)
		return true, true, err
	}
	return true, true, s.processContactPrivacyTargets(ctx, kg, jobID, now)
}

func (s *SQLiteMemory) confirmContactPrivacyDeletion(ctx context.Context, kg *KnowledgeGraph, jobID, source, token string) error {
	if kg != nil {
		if err := kg.CommitContactSourceDeletion(ctx, source, token); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,error_code='',updated_at=? WHERE deletion_job_id=?`, PrivacyDeletionPending, time.Now().UTC().Format(time.RFC3339Nano), jobID)
	return err
}

func (s *SQLiteMemory) deferContactReconciliation(ctx context.Context, id, code string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var attempts int
	if err := tx.QueryRowContext(ctx, `SELECT attempt_count FROM privacy_deletion_jobs WHERE deletion_job_id=?`, id).Scan(&attempts); err != nil {
		return err
	}
	attempts++
	delay := privacyDeletionRetryInitial
	for step := 1; step < attempts && delay < privacyDeletionRetryMaximum; step++ {
		delay *= 2
	}
	if delay > privacyDeletionRetryMaximum {
		delay = privacyDeletionRetryMaximum
	}
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,attempt_count=?,error_code=?,next_retry_at=?,updated_at=? WHERE deletion_job_id=?`, PrivacyDeletionReconcilePending, attempts, safePrivacyDeletionErrorCode(code), now.UTC().Add(delay).Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteMemory) finishContactPrivacyAbort(ctx context.Context, jobID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,retained_count=retained_count+pending_count,pending_count=0,contact_source_id='',contact_operation_token='',session_hash='',error_code='contact_delete_aborted',updated_at=? WHERE deletion_job_id=?`, PrivacyDeletionAborted, time.Now().UTC().Format(time.RFC3339Nano), jobID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM privacy_deletion_targets WHERE deletion_job_id=?`, jobID); err != nil {
		return err
	}
	if err := s.clearPrivacyPublicationMarkerTx(ctx, tx, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteMemory) processContactPrivacyTargets(ctx context.Context, kg *KnowledgeGraph, jobID string, now time.Time) (outErr error) {
	defer func() {
		if outErr != nil && ctx.Err() == nil {
			outErr = errors.Join(outErr, s.markPrivacyDeletionFailure(ctx, jobID, "contact_memory_cleanup_failed", now))
		}
	}()
	targets, err := s.privacyDeletionTargets(ctx, jobID)
	if err != nil {
		return err
	}
	order := map[string]int{"kg_contact_claim": 0, "kg_contact_edge": 1, "kg_contact_node": 2}
	sort.SliceStable(targets, func(i, j int) bool { return order[targets[i].Store] < order[targets[j].Store] })
	for _, target := range targets {
		if target.State != "pending" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if kg == nil {
			return fmt.Errorf("contact memory store is unavailable")
		}
		deleted, err := kg.applyContactPrivacyTarget(ctx, target)
		if err != nil {
			return err
		}
		state, reason := "deleted", ""
		if !deleted {
			state, reason = PrivacyDeletionRetainedReview, "candidate_changed_or_shared"
		}
		if err := s.setPrivacyTargetState(ctx, jobID, target.Store, target.TargetID, state, reason); err != nil {
			return err
		}
	}
	return s.refreshPrivacyDeletionCounts(ctx, jobID, now)
}
