package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SessionDeletionReport struct {
	PersonalityDeleted       int  `json:"personality_deleted"`
	PersonalityRetained      int  `json:"personality_retained"`
	PersonalityLegacyUnknown int  `json:"personality_legacy_unknown"`
	TranscriptDeleted        bool `json:"transcript_deleted"`
	DeletedMemories          int  `json:"deleted_memories"`
	RetainedMemories         int  `json:"retained_memories"`
	DeletedGraphItems        int  `json:"deleted_graph_items"`
	RetainedGraphItems       int  `json:"retained_graph_items"`
}

// DeleteChatSessionWithMemories removes only memories with exclusive, automatic provenance.
// Unknown ownership, curated documents, protected records and shared observations survive.
func (s *SQLiteMemory) DeleteChatSessionWithMemories(ctx context.Context, sessionID string, ltm VectorDB, kg *KnowledgeGraph, deleteMemories bool) (SessionDeletionReport, error) {
	s.cancelSessionRuns(sessionID)
	defer s.finishSessionDeletion(sessionID)
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return SessionDeletionReport{}, err
	}
	defer s.sessionWriteMu.Unlock()
	s.correctionMu.Lock()
	defer s.correctionMu.Unlock()
	s.automaticMemoryMu.Lock()
	defer s.automaticMemoryMu.Unlock()
	// Preserve the compatibility helper's separate vector/graph counters. The
	// HTTP path uses PrepareChatPrivacyDeletion and its full durable manifest.
	if err := s.ensureDeletedSessionPrivacyMapping(ctx); err != nil {
		return SessionDeletionReport{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_chat_sessions(session_id) VALUES(?)`, sessionID); err != nil {
		return SessionDeletionReport{}, err
	}
	personalityTargets, err := s.preparePersonalityPrivacyTargets(ctx, sessionID, deleteMemories)
	if err != nil {
		return SessionDeletionReport{}, err
	}
	var report SessionDeletionReport
	var cleanupErr error
	if deleteMemories {
		rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT doc_id FROM memory_extraction_sources WHERE session_id=?`, sessionID)
		var ids []string
		if err == nil {
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					break
				}
				ids = append(ids, id)
			}
			err = errors.Join(err, rows.Err())
			rows.Close()
		}
		cleanupErr = errors.Join(cleanupErr, err)
		for _, id := range ids {
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
				report.RetainedMemories++
				continue
			}
			if _, err = tx.ExecContext(ctx, `UPDATE memory_meta SET doc_id=doc_id WHERE doc_id=?`, id); err != nil {
				tx.Rollback()
				cleanupErr = errors.Join(cleanupErr, err)
				report.RetainedMemories++
				continue
			}
			var exclusive bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta m WHERE m.doc_id=? AND m.protected=0 AND m.keep_forever=0
			 AND m.source_type IN ('memory_analysis','consolidation','chat','hierarchy')
			 AND NOT EXISTS(SELECT 1 FROM memory_extraction_sources x WHERE x.doc_id=m.doc_id AND (x.session_id<>? OR x.source_type NOT IN ('memory_analysis','consolidation','chat','hierarchy')))
			 AND NOT EXISTS(SELECT 1 FROM file_embedding_docs f WHERE f.doc_id=m.doc_id)
			 AND NOT EXISTS(SELECT 1 FROM memory_curation_events c WHERE c.doc_id=m.doc_id AND c.dry_run=0))`, id, sessionID).Scan(&exclusive)
			sourceDeletion := false
			if durable, ok := ltm.(interface{ CanDeleteWithoutEmbeddings() bool }); ok {
				sourceDeletion = durable.CanDeleteWithoutEmbeddings()
			}
			if err != nil || !exclusive || ltm == nil || (ltm.IsDisabled() && !sourceDeletion) {
				tx.Rollback()
				report.RetainedMemories++
				cleanupErr = errors.Join(cleanupErr, err)
				continue
			}
			if err := ltm.DeleteDocument(id); err != nil {
				tx.Rollback()
				report.RetainedMemories++
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete session memory: %w", err))
				continue
			}
			for _, statement := range []string{`DELETE FROM memory_meta WHERE doc_id=?`, `DELETE FROM memory_extraction_sources WHERE doc_id=?`, `DELETE FROM memory_usage_log WHERE memory_id=?`} {
				if _, err = tx.ExecContext(ctx, statement, id); err != nil {
					break
				}
			}
			if err != nil {
				tx.Rollback()
				cleanupErr = errors.Join(cleanupErr, err)
			} else {
				if commitErr := tx.Commit(); commitErr != nil {
					cleanupErr = errors.Join(cleanupErr, commitErr)
					continue
				}
				report.DeletedMemories++
			}
		}
		if kg != nil {
			report.DeletedGraphItems, report.RetainedGraphItems, err = kg.deleteExclusiveSessionGraph(ctx, sessionID)
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	purgeErr := s.purgeChatSessionWithGraphPrivacyJobLocked(sessionID, "", kg, personalityTargets)
	report.TranscriptDeleted = purgeErr == nil
	var personalityJobID string
	if err := s.db.QueryRowContext(ctx, `SELECT privacy_job_id FROM deleted_chat_sessions WHERE session_id=?`, sessionID).Scan(&personalityJobID); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if personalityJobID != "" && purgeErr == nil {
		targets, err := s.privacyDeletionTargets(ctx, personalityJobID)
		cleanupErr = errors.Join(cleanupErr, err)
		for _, target := range targets {
			if strings.HasPrefix(target.Store, "personality_") && target.State == "pending" {
				_, err := s.applyPersonalityPrivacyTarget(ctx, personalityJobID, target)
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		cleanupErr = errors.Join(cleanupErr, s.refreshPrivacyDeletionCounts(ctx, personalityJobID, time.Now().UTC()))
	}
	if kg != nil {
		graphErr := kg.ScrubSessionTranscriptEvidence(ctx, sessionID)
		cleanupErr = errors.Join(cleanupErr, graphErr)
		if graphErr == nil {
			var jobID string
			if err := s.db.QueryRowContext(ctx, `SELECT privacy_job_id FROM deleted_chat_sessions WHERE session_id=?`, sessionID).Scan(&jobID); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
			} else if jobID != "" {
				tx, err := s.db.BeginTx(ctx, nil)
				if err != nil {
					cleanupErr = errors.Join(cleanupErr, err)
				} else {
					target := privacyGraphTranscriptTarget(sessionID)
					if err = markPrivacyTargetStateTx(ctx, tx, jobID, target, "deleted", ""); err == nil {
						err = tx.Commit()
					} else {
						_ = tx.Rollback()
					}
					cleanupErr = errors.Join(cleanupErr, err)
					if err == nil {
						cleanupErr = errors.Join(cleanupErr, s.refreshPrivacyDeletionCounts(ctx, jobID, time.Now().UTC()))
					}
				}
			}
		}
	}
	if personalityJobID != "" {
		var err error
		report.PersonalityDeleted, report.PersonalityRetained, report.PersonalityLegacyUnknown, err = s.personalityPrivacyCounts(ctx, personalityJobID)
		cleanupErr = errors.Join(cleanupErr, err)
	}
	return report, errors.Join(cleanupErr, purgeErr)
}
