package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const privacyDeletionFinalizeTimeout = 5 * time.Second

func boundedPrivacyDeletionFinalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), privacyDeletionFinalizeTimeout)
}

// ProcessDuePrivacyDeletionJobs applies already journaled chat-memory cleanup.
// It is safe to call at startup and from a periodic worker.
func (s *SQLiteMemory) ProcessDuePrivacyDeletionJobs(ctx context.Context, ltm VectorDB, kg *KnowledgeGraph, now time.Time, limit int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("privacy deletion store is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `SELECT deletion_job_id FROM privacy_deletion_jobs
		WHERE kind='chat' AND deletion_status IN (?,?,?,?,?) AND next_retry_at<=?
		ORDER BY next_retry_at,created_at LIMIT ?`, PrivacyDeletionPreparing, PrivacyDeletionPending, PrivacyDeletionRetryWait, PrivacyDeletionProcessing, PrivacyDeletionReconcilePending, stamp, limit)
	if err != nil {
		return fmt.Errorf("list due privacy deletion jobs: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	var joined error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return errors.Join(joined, err)
		}
		joined = errors.Join(joined, s.processChatPrivacyDeletion(ctx, id, ltm, kg, now))
	}
	return joined
}

func (s *SQLiteMemory) processChatPrivacyDeletion(ctx context.Context, id string, ltm VectorDB, kg *KnowledgeGraph, now time.Time) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	defer func() {
		if resultErr == nil || ctx.Err() != nil {
			return
		}
		var reconcile *privacyDeletionReconcileError
		finalizeCtx, cancel := boundedPrivacyDeletionFinalizationContext(ctx)
		defer cancel()
		if errors.As(resultErr, &reconcile) {
			resultErr = errors.Join(resultErr, s.markPrivacyDeletionReconcilePending(finalizeCtx, id, reconcile.code, now))
			return
		}
		resultErr = errors.Join(resultErr, s.markPrivacyDeletionFailure(finalizeCtx, id, "privacy_cleanup_failed", now))
	}()
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return err
	}
	defer s.sessionWriteMu.Unlock()
	if err := lockMemoryMutex(ctx, &s.correctionMu); err != nil {
		return err
	}
	defer s.correctionMu.Unlock()
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return err
	}
	defer s.automaticMemoryMu.Unlock()
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return err
	}
	defer release()

	job, err := s.GetPrivacyDeletionJob(ctx, id)
	if err != nil {
		return err
	}
	if job.DeletionStatus == PrivacyDeletionCompleted || job.DeletionStatus == PrivacyDeletionAborted || job.DeletionStatus == PrivacyDeletionRetainedReview {
		return nil
	}
	var sessionID string
	err = s.db.QueryRowContext(ctx, `SELECT session_id FROM deleted_chat_sessions WHERE privacy_job_id=? ORDER BY deleted_at DESC LIMIT 1`, id).Scan(&sessionID)
	if err != nil {
		return fmt.Errorf("resolve privacy scrub mapping: %w", err)
	}
	markerState, err := s.privacyPublicationMarkerState(ctx, id)
	if err != nil {
		return fmt.Errorf("read privacy publication marker: %w", err)
	}
	if job.DeletionStatus == PrivacyDeletionPreparing || markerState == "scrub" {
		if err := s.finishChatPrivacyScrub(ctx, id, sessionID, kg); err != nil {
			return err
		}
		job, err = s.GetPrivacyDeletionJob(ctx, id)
		if err != nil {
			return err
		}
	}
	if job.DeletionStatus == PrivacyDeletionRetryWait && job.NextRetryAt != "" {
		next, parseErr := time.Parse(time.RFC3339Nano, job.NextRetryAt)
		if parseErr == nil && next.After(now) {
			return nil
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, privacyDeletionSnapshotMarkerPrefix+id, "apply"); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,updated_at=? WHERE deletion_job_id=?`, PrivacyDeletionProcessing, now.UTC().Format(time.RFC3339Nano), id); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	targets, err := s.privacyDeletionTargets(ctx, id)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.State != "pending" {
			continue
		}
		var deleted bool
		switch target.Store {
		case "personality_profile", "personality_emotion", "personality_character":
			deleted, err = s.applyPersonalityPrivacyTarget(ctx, id, target)
		case "ltm":
			deleted, err = s.applySessionLongTermPrivacyTarget(ctx, id, ltm, target)
		case "kg_node", "kg_edge", "kg_claim":
			if kg == nil {
				err = fmt.Errorf("knowledge graph is unavailable for privacy deletion")
			} else {
				deleted, err = kg.applySessionGraphPrivacyTarget(ctx, target)
			}
		case privacyTargetCorrectionReceipt:
			if kg == nil {
				err = privacyNeedsReconciliation("correction_receipt_store_unavailable", fmt.Errorf("knowledge graph is unavailable for correction receipt cleanup"))
			} else {
				err = kg.deleteCorrectionReceipts(ctx, []string{target.TargetID})
				deleted = err == nil
			}
		case privacyTargetCorrectionVector:
			deleted, err = s.applyPrivacyCorrectionVectorTarget(ctx, ltm, id, target)
		case privacyTargetGraphTranscript:
			if kg == nil {
				err = privacyNeedsReconciliation("graph_transcript_store_unavailable", fmt.Errorf("knowledge graph is unavailable for transcript evidence scrub"))
			} else {
				err = kg.ScrubSessionTranscriptEvidence(ctx, sessionID)
				deleted = err == nil
			}
		default:
			err = fmt.Errorf("unknown privacy target store %q", target.Store)
		}
		if err != nil {
			return err
		}
		state, reason := "deleted", ""
		if !deleted {
			state, reason = PrivacyDeletionRetainedReview, "candidate_changed_or_shared"
		}
		if err := s.setPrivacyTargetState(ctx, id, target.Store, target.TargetID, state, reason); err != nil {
			return err
		}
	}
	if err := s.refreshPrivacyDeletionCounts(ctx, id, now); err != nil {
		return err
	}
	return nil
}

type privacyDeletionReconcileError struct {
	code  string
	cause error
}

func (e *privacyDeletionReconcileError) Error() string {
	if e == nil || e.cause == nil {
		return "privacy deletion requires reconciliation"
	}
	return e.cause.Error()
}

func (e *privacyDeletionReconcileError) Unwrap() error { return e.cause }

func privacyNeedsReconciliation(code string, cause error) error {
	return &privacyDeletionReconcileError{code: safePrivacyDeletionErrorCode(code), cause: cause}
}

func (s *SQLiteMemory) hasPrivacyPublicationMarker(ctx context.Context, id string) bool {
	var exists bool
	_ = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_maintenance_meta WHERE key=?)`, privacyDeletionSnapshotMarkerPrefix+id).Scan(&exists)
	return exists
}

func (s *SQLiteMemory) privacyPublicationMarkerState(ctx context.Context, id string) (string, error) {
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM memory_maintenance_meta WHERE key=?`, privacyDeletionSnapshotMarkerPrefix+id).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return state, err
}

func (s *SQLiteMemory) privacyDeletionTargets(ctx context.Context, id string) ([]privacyDeletionTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT store,target_id,expected_fingerprint,content_sha256,source_marker,state,reason_code FROM privacy_deletion_targets WHERE deletion_job_id=? ORDER BY CASE store WHEN 'kg_claim' THEN 0 WHEN 'kg_edge' THEN 1 WHEN 'kg_node' THEN 2 ELSE 3 END,target_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []privacyDeletionTarget
	for rows.Next() {
		var target privacyDeletionTarget
		if err := rows.Scan(&target.Store, &target.TargetID, &target.ExpectedFingerprint, &target.ContentSHA256, &target.SourceMarker, &target.State, &target.ReasonCode); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *SQLiteMemory) applySessionLongTermPrivacyTarget(ctx context.Context, jobID string, ltm VectorDB, target privacyDeletionTarget) (bool, error) {
	if ltm == nil || ltm.IsDisabled() {
		return false, fmt.Errorf("long-term memory is unavailable for privacy deletion")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	eligible, err := sessionLongTermTargetStillExclusiveTx(ctx, tx, target)
	if err != nil {
		return false, err
	}
	if !eligible {
		var metadataExists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta WHERE doc_id=?)`, target.TargetID).Scan(&metadataExists); err != nil {
			return false, err
		}
		if !metadataExists {
			if _, readErr := ltm.GetByID(target.TargetID); readErr != nil {
				if isKnownVectorDocumentMissing(readErr, target.TargetID) {
					return s.finishMissingVectorTarget(ctx, tx, jobID, target)
				}
				return false, readErr
			}
		}
		return false, nil
	}
	content, err := ltm.GetByID(target.TargetID)
	if err != nil {
		if !isKnownVectorDocumentMissing(err, target.TargetID) {
			return false, err
		}
		return s.finishMissingVectorTarget(ctx, tx, jobID, target)
	}
	if contentSHA256(content) != target.ContentSHA256 {
		return false, nil
	}
	if owned, ok := ltm.(OwnershipAwareVectorDB); ok {
		deleted, err := owned.DeleteDocumentIfContentMatches(target.TargetID, target.ContentSHA256)
		if err != nil {
			return false, err
		}
		if !deleted {
			_, readErr := ltm.GetByID(target.TargetID)
			if readErr != nil && isKnownVectorDocumentMissing(readErr, target.TargetID) {
				return s.finishMissingVectorTarget(ctx, tx, jobID, target)
			}
			if readErr != nil {
				return false, readErr
			}
			return false, nil
		}
	} else if err := ltm.DeleteDocument(target.TargetID); err != nil {
		return false, err
	}
	for _, statement := range []string{`DELETE FROM memory_meta WHERE doc_id=?`, `DELETE FROM memory_extraction_sources WHERE doc_id=?`, `DELETE FROM memory_usage_log WHERE memory_id=?`} {
		if _, err := tx.ExecContext(ctx, statement, target.TargetID); err != nil {
			return false, err
		}
	}
	if err := markPrivacyTargetStateTx(ctx, tx, jobID, target, "deleted", ""); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

type privacyMemoryMetaQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// privacyMemoryMetaFingerprint captures semantic state that can change a
// memory's authorization for deletion. Usage/access counters are omitted so
// ordinary retrieval does not invalidate an explicit deletion request.
func privacyMemoryMetaFingerprint(ctx context.Context, queryer privacyMemoryMetaQueryer, docID string) (string, error) {
	var values [12]string
	err := queryer.QueryRowContext(ctx, `SELECT doc_id,
		COALESCE(CAST(extraction_confidence AS TEXT),''),COALESCE(verification_status,''),COALESCE(source_type,''),
		COALESCE(CAST(source_reliability AS TEXT),''),COALESCE(CAST(protected AS TEXT),''),COALESCE(CAST(keep_forever AS TEXT),''),
		COALESCE(archived_at,''),COALESCE(archived_reason,''),COALESCE(last_reviewed_at,''),COALESCE(review_note,''),
		CAST(COALESCE((SELECT MAX(id) FROM memory_curation_events WHERE doc_id=memory_meta.doc_id AND dry_run=0),0) AS TEXT)
		FROM memory_meta WHERE doc_id=?`, docID).Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8], &values[9], &values[10], &values[11])
	if err != nil {
		return "", err
	}
	return privacyFingerprint(values[:]...), nil
}

func (s *SQLiteMemory) finishMissingVectorTarget(ctx context.Context, tx *sql.Tx, jobID string, target privacyDeletionTarget) (bool, error) {
	for _, statement := range []string{`DELETE FROM memory_meta WHERE doc_id=?`, `DELETE FROM memory_extraction_sources WHERE doc_id=?`, `DELETE FROM memory_usage_log WHERE memory_id=?`} {
		if _, err := tx.ExecContext(ctx, statement, target.TargetID); err != nil {
			return false, err
		}
	}
	if err := markPrivacyTargetStateTx(ctx, tx, jobID, target, "deleted", ""); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func isKnownVectorDocumentMissing(err error, id string) bool {
	return err != nil && strings.Contains(err.Error(), "document with ID '"+id+"' not found")
}

func (s *SQLiteMemory) sessionLongTermTargetStillExclusive(ctx context.Context, target privacyDeletionTarget) (bool, error) {
	var eligible bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta m WHERE m.doc_id=? AND m.protected=0 AND m.keep_forever=0
		AND m.source_type IN ('memory_analysis','consolidation','chat','hierarchy')
		AND NOT EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND (x.session_id<>? OR x.source_type<>'deleted_source'))
		AND EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND x.session_id=? AND x.source_type='deleted_source')
		AND NOT EXISTS(SELECT 1 FROM file_embedding_docs f WHERE f.doc_id=m.doc_id)
		AND NOT EXISTS(SELECT 1 FROM memory_curation_events c WHERE c.doc_id=m.doc_id AND c.dry_run=0))`, target.TargetID, target.SourceMarker, target.SourceMarker).Scan(&eligible)
	return eligible, err
}

func sessionLongTermTargetStillExclusiveTx(ctx context.Context, tx *sql.Tx, target privacyDeletionTarget) (bool, error) {
	var eligible bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta m WHERE m.doc_id=? AND m.protected=0 AND m.keep_forever=0
		AND m.source_type IN ('memory_analysis','consolidation','chat','hierarchy')
		AND NOT EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND (x.session_id<>? OR x.source_type<>'deleted_source'))
		AND EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND x.session_id=? AND x.source_type='deleted_source')
		AND NOT EXISTS(SELECT 1 FROM file_embedding_docs f WHERE f.doc_id=m.doc_id)
		AND NOT EXISTS(SELECT 1 FROM memory_curation_events c WHERE c.doc_id=m.doc_id AND c.dry_run=0))`, target.TargetID, target.SourceMarker, target.SourceMarker).Scan(&eligible)
	if err != nil || !eligible {
		return eligible, err
	}
	fingerprint, err := privacyMemoryMetaFingerprint(ctx, tx, target.TargetID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fingerprint == target.ExpectedFingerprint, nil
}

func markPrivacyTargetStateTx(ctx context.Context, tx *sql.Tx, jobID string, target privacyDeletionTarget, state, reason string) error {
	_, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_targets SET state=?,reason_code=? WHERE deletion_job_id=? AND store=? AND target_id=?`, state, reason, jobID, target.Store, target.TargetID)
	return err
}

func (s *SQLiteMemory) setPrivacyTargetState(ctx context.Context, jobID, store, targetID, state, reason string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE privacy_deletion_targets SET state=?,reason_code=? WHERE deletion_job_id=? AND store=? AND target_id=?`, state, reason, jobID, store, targetID)
	return err
}

func (s *SQLiteMemory) refreshPrivacyDeletionCounts(ctx context.Context, id string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existingStatus, kind string
	if err := tx.QueryRowContext(ctx, `SELECT deletion_status,kind FROM privacy_deletion_jobs WHERE deletion_job_id=?`, id).Scan(&existingStatus, &kind); err != nil {
		return err
	}
	if existingStatus == PrivacyDeletionCompleted || existingStatus == PrivacyDeletionAborted || existingStatus == PrivacyDeletionRetainedReview {
		return nil
	}
	var pending, deleted, retained int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN state='pending' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='deleted' AND (?<>'chat' OR store IN ('ltm','kg_node','kg_edge','kg_claim')) THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN state='retained_review' AND store NOT LIKE 'personality_%' THEN 1 ELSE 0 END),0) FROM privacy_deletion_targets WHERE deletion_job_id=?`, kind, id).Scan(&pending, &deleted, &retained); err != nil {
		return err
	}
	status := PrivacyDeletionCompleted
	if _, err = tx.ExecContext(ctx, `INSERT INTO personality_privacy_counts(job_id,deleted,retained) SELECT ?,COALESCE(SUM(state='deleted'),0),COALESCE(SUM(state='retained_review'),0) FROM privacy_deletion_targets WHERE deletion_job_id=? AND store LIKE 'personality_%' ON CONFLICT(job_id) DO UPDATE SET deleted=excluded.deleted,retained=excluded.retained`, id, id); err != nil {
		return err
	}
	var legacyUnknown, personalityRetained int
	if err = tx.QueryRowContext(ctx, `SELECT legacy_unknown,retained FROM personality_privacy_counts WHERE job_id=?`, id).Scan(&legacyUnknown, &personalityRetained); err != nil {
		return err
	}
	if pending > 0 {
		status = PrivacyDeletionPending
	} else if retained > 0 || legacyUnknown > 0 || personalityRetained > 0 {
		status = PrivacyDeletionRetainedReview
	}
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET deletion_status=?,pending_count=?,deleted_count=?,retained_count=?,error_code='',updated_at=? WHERE deletion_job_id=?`, status, pending, deleted, retained, now.UTC().Format(time.RFC3339Nano), id); err != nil {
		return err
	}
	if pending == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM privacy_deletion_targets WHERE deletion_job_id=?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET contact_source_id='',contact_operation_token='',session_hash='' WHERE deletion_job_id=?`, id); err != nil {
			return err
		}
		if err := s.clearPrivacyPublicationMarkerTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
