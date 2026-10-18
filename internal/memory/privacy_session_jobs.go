package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PrepareChatPrivacyDeletion durably journals the selected, content-free
// manifest before any transcript or graph evidence is scrubbed.
func (s *SQLiteMemory) PrepareChatPrivacyDeletion(ctx context.Context, sessionID string, ltm VectorDB, kg *KnowledgeGraph, deleteMemories bool) (PrivacyDeletionJob, error) {
	if s == nil || s.db == nil || sessionID == "" {
		return PrivacyDeletionJob{}, fmt.Errorf("memory store and session ID are required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.cancelSessionRuns(sessionID)
	defer s.finishSessionDeletion(sessionID)
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return PrivacyDeletionJob{}, err
	}
	defer s.sessionWriteMu.Unlock()
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return PrivacyDeletionJob{}, err
	}
	defer s.automaticMemoryMu.Unlock()
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("begin privacy deletion publication: %w", err)
	}
	defer release()
	if err := s.ensureDeletedSessionPrivacyMapping(ctx); err != nil {
		return PrivacyDeletionJob{}, err
	}

	// A retry after a lost HTTP response reuses the existing journal.
	if job, err := s.privacyJobForSession(ctx, sessionID); err == nil {
		return job, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PrivacyDeletionJob{}, err
	}

	jobID := newPrivacyDeletionID()
	marker := sessionPrivacyMarker(sessionID)
	targets := make([]privacyDeletionTarget, 0)
	if deleteMemories {
		ltmTargets, err := s.prepareSessionLongTermTargets(ctx, sessionID, marker, ltm)
		if err != nil {
			return PrivacyDeletionJob{}, err
		}
		targets = append(targets, ltmTargets...)
		if kg != nil {
			graphTargets, err := kg.prepareSessionGraphDeletionTargets(ctx, sessionID, marker)
			if err != nil {
				return PrivacyDeletionJob{}, err
			}
			targets = append(targets, graphTargets...)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("begin privacy deletion preparation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO deleted_chat_sessions(session_id,privacy_job_id) VALUES(?,?)`, sessionID, jobID); err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("persist chat privacy fence: %w", err)
	}
	if err := s.createPrivacyDeletionJobTx(ctx, tx, jobID, "chat", PrivacyDeletionPreparing, deleteMemories, "", "", time.Now().UTC(), targets); err != nil {
		return PrivacyDeletionJob{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE privacy_deletion_jobs SET session_hash=? WHERE deletion_job_id=?`, strings.TrimPrefix(marker, "deleted:"), jobID); err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("persist chat session fingerprint: %w", err)
	}
	if err := s.setPrivacyPublicationMarkerTx(ctx, tx, jobID); err != nil {
		return PrivacyDeletionJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return PrivacyDeletionJob{}, fmt.Errorf("commit privacy deletion manifest: %w", err)
	}

	if err := s.finishChatPrivacyScrub(ctx, jobID, sessionID, kg); err != nil {
		_ = s.markPrivacyDeletionFailure(context.Background(), jobID, "privacy_scrub_failed", time.Now().UTC())
		job, readErr := s.GetPrivacyDeletionJob(context.Background(), jobID)
		return job, errors.Join(err, readErr)
	}
	return s.GetPrivacyDeletionJob(ctx, jobID)
}

func (s *SQLiteMemory) privacyJobForSession(ctx context.Context, sessionID string) (PrivacyDeletionJob, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT privacy_job_id FROM deleted_chat_sessions WHERE session_id=? AND privacy_job_id<>''`, sessionID).Scan(&id)
	if err != nil {
		return PrivacyDeletionJob{}, err
	}
	return s.GetPrivacyDeletionJob(ctx, id)
}

func (s *SQLiteMemory) ensureDeletedSessionPrivacyMapping(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deleted_chat_sessions (session_id TEXT PRIMARY KEY, deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, privacy_job_id TEXT NOT NULL DEFAULT '')`); err != nil {
		return fmt.Errorf("prepare deleted-session mapping: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE deleted_chat_sessions ADD COLUMN privacy_job_id TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return fmt.Errorf("migrate deleted-session mapping: %w", err)
	}
	return nil
}

func sessionPrivacyMarker(sessionID string) string {
	return fmt.Sprintf("deleted:%x", sha256Bytes([]byte(sessionID)))
}

func (s *SQLiteMemory) prepareSessionLongTermTargets(ctx context.Context, sessionID, marker string, ltm VectorDB) ([]privacyDeletionTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT doc_id FROM memory_extraction_sources WHERE session_id=? ORDER BY doc_id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read chat-owned memory candidates: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	targets := make([]privacyDeletionTarget, 0, len(ids))
	for _, id := range ids {
		target := privacyDeletionTarget{Store: "ltm", TargetID: id, SourceMarker: marker, State: "retained_review", ReasonCode: "ownership_uncertain"}
		var eligible bool
		err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta m WHERE m.doc_id=? AND m.protected=0 AND m.keep_forever=0
			AND m.source_type IN ('memory_analysis','consolidation','chat','hierarchy')
			AND NOT EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND (x.session_id<>? OR x.source_type NOT IN ('memory_analysis','consolidation','chat','hierarchy')))
			AND NOT EXISTS(SELECT 1 FROM file_embedding_docs f WHERE f.doc_id=m.doc_id)
			AND NOT EXISTS(SELECT 1 FROM memory_curation_events c WHERE c.doc_id=m.doc_id AND c.dry_run=0))`, id, sessionID).Scan(&eligible)
		if err != nil {
			return nil, fmt.Errorf("check chat memory ownership: %w", err)
		}
		if eligible {
			if ltm == nil || ltm.IsDisabled() {
				return nil, fmt.Errorf("cannot inspect chat-owned memory candidate")
			}
			metaFingerprint, fingerprintErr := privacyMemoryMetaFingerprint(ctx, s.db, id)
			if fingerprintErr != nil {
				return nil, fmt.Errorf("fingerprint chat-owned memory metadata: %w", fingerprintErr)
			}
			target.ExpectedFingerprint = metaFingerprint
			content, readErr := ltm.GetByID(id)
			if readErr != nil {
				if !isKnownVectorDocumentMissing(readErr, id) {
					return nil, fmt.Errorf("inspect chat-owned memory candidate: %w", readErr)
				}
				target.State = "pending"
				target.ReasonCode = "vector_missing"
				targets = append(targets, target)
				continue
			}
			target.ContentSHA256 = contentSHA256(content)
			target.State = "pending"
			target.ReasonCode = ""
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *SQLiteMemory) finishChatPrivacyScrub(ctx context.Context, jobID, sessionID string, kg *KnowledgeGraph) error {
	if err := s.purgeChatSessionWithPrivacyJobLocked(sessionID, jobID); err != nil {
		return fmt.Errorf("purge chat transcript: %w", err)
	}
	if kg != nil {
		if err := kg.ScrubSessionTranscriptEvidence(ctx, sessionID); err != nil {
			return fmt.Errorf("scrub graph transcript evidence: %w", err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.clearPrivacyPublicationMarkerTx(ctx, tx, jobID); err != nil {
		return err
	}
	if err := s.setPrivacyJobStatusTx(ctx, tx, jobID, PrivacyDeletionPending, "", time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func sha256Bytes(value []byte) []byte {
	h := sha256.Sum256(value)
	return h[:]
}
