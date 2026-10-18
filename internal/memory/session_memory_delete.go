package memory

import (
	"context"
	"errors"
	"fmt"
)

type SessionDeletionReport struct {
	TranscriptDeleted  bool `json:"transcript_deleted"`
	DeletedMemories    int  `json:"deleted_memories"`
	RetainedMemories   int  `json:"retained_memories"`
	DeletedGraphItems  int  `json:"deleted_graph_items"`
	RetainedGraphItems int  `json:"retained_graph_items"`
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
	s.automaticMemoryMu.Lock()
	defer s.automaticMemoryMu.Unlock()
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
				cleanupErr = errors.Join(cleanupErr, tx.Commit())
			}
			report.DeletedMemories++
		}
		if kg != nil {
			report.DeletedGraphItems, report.RetainedGraphItems, err = kg.deleteExclusiveSessionGraph(ctx, sessionID)
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	purgeErr := s.purgeChatSessionLocked(sessionID)
	report.TranscriptDeleted = purgeErr == nil
	if kg != nil {
		cleanupErr = errors.Join(cleanupErr, kg.ScrubSessionTranscriptEvidence(ctx, sessionID))
	}
	return report, errors.Join(cleanupErr, purgeErr)
}
