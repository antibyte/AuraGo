package memory

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// AutomaticMemoryWrite records only ownership established by this operation.
// A nil snapshot is deliberately insufficient authority for rollback.
type AutomaticMemoryWrite struct {
	PublicationID string
	DocID         string
	ContentHash   string
	Metadata      *MemoryMeta
	Sources       []MemoryExtractionSource
}

type AutomaticMemoryStoreResult struct {
	VectorStoreResult
	Writes []AutomaticMemoryWrite
}

func (s *SQLiteMemory) initializeAutomaticMemoryMeta(id string, details MemoryMetaUpdate) (*MemoryMeta, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("automatic memory metadata is unavailable")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin automatic metadata: %w", err)
	}
	defer tx.Rollback()
	inserted, err := insertMemoryMeta(tx, id, details)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, nil
	}
	var snapshot MemoryMeta
	if err := scanMemoryMeta(tx.QueryRow(`SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, id), &snapshot); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit automatic metadata: %w", err)
	}
	return &snapshot, nil
}

// StoreAutomaticMemoryDocument shares insert-only metadata and ownership
// receipts across analysis, consolidation, hierarchy and queued retries.
func StoreAutomaticMemoryDocument(s *SQLiteMemory, ltm VectorDB, concept, content string, details MemoryMetaUpdate) (AutomaticMemoryStoreResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return storeAutomaticMemoryDocument(ctx, s, ltm, concept, content, details, func(ctx context.Context, ltm VectorDB, concept, content string) (VectorStoreResult, error) {
		if _, ok := ltm.(*ChromemVectorDB); ok {
			return StoreDocumentWithOwnershipContext(ctx, ltm, concept, content)
		}
		if err := startMemoryPublication(ctx); err != nil {
			return VectorStoreResult{}, err
		}
		return StoreDocumentWithOwnership(ltm, concept, content)
	})
}

// StoreAutomaticMemoryDocumentWithContext propagates the run deadline through
// embedding and vector storage. Receipts and metadata still describe any writes.
func StoreAutomaticMemoryDocumentWithContext(ctx context.Context, s *SQLiteMemory, ltm VectorDB, concept, content string, details MemoryMetaUpdate) (AutomaticMemoryStoreResult, error) {
	return storeAutomaticMemoryDocument(ctx, s, ltm, concept, content, details, func(ctx context.Context, ltm VectorDB, concept, content string) (VectorStoreResult, error) {
		if _, ok := ltm.(*ChromemVectorDB); !ok {
			if err := startMemoryPublication(ctx); err != nil {
				return VectorStoreResult{}, err
			}
		}
		return StoreDocumentWithOwnershipContext(ctx, ltm, concept, content)
	})
}

func storeAutomaticMemoryDocument(ctx context.Context, s *SQLiteMemory, ltm VectorDB, concept, content string, details MemoryMetaUpdate, store func(context.Context, VectorDB, string, string) (VectorStoreResult, error)) (result AutomaticMemoryStoreResult, resultErr error) {
	if err := ctx.Err(); err != nil {
		return AutomaticMemoryStoreResult{}, err
	}
	if s != nil {
		if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
			return AutomaticMemoryStoreResult{}, err
		}
		defer s.automaticMemoryMu.Unlock()
		s.bindArchiveVector(ltm)
		if err := s.backfillArchiveIdentities(ctx, ltm); err != nil {
			return result, err
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if HasDurableCredential(concept + "\n" + content) {
		return result, fmt.Errorf("%w: credentials cannot enter durable memory", ErrAutomaticMemorySkipped)
	}
	session := strings.TrimSpace(details.SourceSessionID)
	if session == "" {
		session, _ = MemoryAnalysisSession(content)
	}
	if session == "" {
		session = "default"
	}
	validate := func() error {
		if s == nil {
			return nil
		}
		blockedArchive, err := s.archivedIdentityBlocked(ctx, memoryDocumentIdentity(buildContentString(concept, content), ""))
		if err != nil {
			return fmt.Errorf("check archived memory identity: %w", err)
		}
		if blockedArchive {
			return fmt.Errorf("%w: fact is archived", ErrAutomaticMemorySkipped)
		}
		deleted, err := s.IsSessionDeleted(session)
		if err != nil {
			return fmt.Errorf("check memory source session: %w", err)
		}
		if deleted {
			return fmt.Errorf("%w: source session deleted", ErrAutomaticMemorySkipped)
		}
		blocked, err := s.IsCorrectedMemoryFact(CorrectionFactText(buildContentString(concept, content)))
		if err != nil {
			return fmt.Errorf("check memory correction history: %w", err)
		}
		if blocked {
			return fmt.Errorf("%w: fact was corrected; automatic extraction cannot restore it", ErrAutomaticMemorySkipped)
		}
		return nil
	}
	if err := validate(); err != nil {
		return result, err
	}
	ctx, publication := prepareMemoryPublication(ctx, s, validate)
	complete := false
	defer func() { resultErr = errors.Join(resultErr, publication.finish(complete)) }()
	if _, ok := AnalysisMemoryIdentity(concept, content, ""); ok && details.SourceType == "memory_analysis" {
		content = MemoryAnalysisSourceMarker
		if details.SourceRole != "" {
			role := details.SourceRole
			if role != "user" && role != "assistant" && role != "user_and_assistant" {
				role = "unknown"
			}
			content += " role:" + role
		}
	}
	stored, storeErr := store(ctx, ltm, concept, content)
	result.VectorStoreResult = stored
	resultErr = storeErr
	publication.createdIDs = stored.CreatedIDs
	publication.rollbackAllowed = len(stored.ReusedIDs)+len(stored.UnknownIDs) == 0
	for _, id := range stored.CreatedIDs {
		write := AutomaticMemoryWrite{DocID: id, PublicationID: publication.key}
		if s != nil {
			var err error
			write.Metadata, err = s.initializeAutomaticMemoryMeta(id, details)
			resultErr = errors.Join(resultErr, err)
		}
		// Whole content must be provable against the issued write. Unknown
		// backend/chunk formats are retained rather than guessed during rollback.
		if len(buildContentString(concept, content)) <= 4000 {
			actual, err := ltm.GetByID(id)
			if err == nil && (actual == buildContentString(concept, content) || actual == content) {
				write.ContentHash = contentSHA256(actual)
			}
		}
		result.Writes = append(result.Writes, write)
	}
	if s != nil {
		for _, id := range stored.UnknownIDs {
			resultErr = errors.Join(resultErr, s.EnsureMemoryMetaWithDetails(id, details))
		}
		ids := append(append(append([]string{}, stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
		for _, id := range ids {
			resultErr = errors.Join(resultErr, s.recordDocumentIdentity(ctx, id, memoryDocumentIdentity(buildContentString(concept, content), "")))
			source := strings.TrimSpace(details.SourceType)
			if source == "" {
				source = "system"
			}
			if source == "memory_analysis" {
				if document, err := ltm.GetByID(id); err == nil {
					if _, body, ok := AnalysisDocumentParts(document); ok {
						if legacySession, _ := MemoryAnalysisSession(body); legacySession != "" && legacySession != session {
							resultErr = errors.Join(resultErr, s.RecordMemoryExtractionSource(id, source, legacySession))
						}
					}
				}
			}
			resultErr = errors.Join(resultErr, s.RecordMemoryExtractionSource(id, source, session))
		}
		for i := range result.Writes {
			sources, err := s.GetMemoryExtractionSources(result.Writes[i].DocID)
			resultErr = errors.Join(resultErr, err)
			if len(sources) == 1 && sources[0].FirstSeenAt == sources[0].LastSeenAt {
				result.Writes[i].Sources = sources
			}
		}
	}
	if resultErr == nil && len(stored.CreatedIDs)+len(stored.ReusedIDs)+len(stored.UnknownIDs) == 0 {
		resultErr = fmt.Errorf("automatic memory write returned no document IDs")
	}
	complete = resultErr == nil
	return result, resultErr
}

// RollbackAutomaticMemoryWrites excludes curated, accessed, referenced and
// reused documents. The SQLite write lock prevents curation between validation
// and the vector store's conditional content deletion.
func (s *SQLiteMemory) RollbackAutomaticMemoryWrites(ltm VectorDB, writes []AutomaticMemoryWrite) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.RollbackAutomaticMemoryWritesContext(ctx, ltm, writes)
}

func (s *SQLiteMemory) RollbackAutomaticMemoryWritesContext(ctx context.Context, ltm VectorDB, writes []AutomaticMemoryWrite) error {
	if s != nil {
		if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
			return err
		}
		defer s.automaticMemoryMu.Unlock()
	}
	if s == nil {
		return fmt.Errorf("memory metadata store is unavailable")
	}
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return err
	}
	defer release()
	var result error
	for _, write := range writes {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		result = errors.Join(result, s.rollbackAutomaticMemoryWriteContext(ctx, ltm, write))
	}
	if result == nil {
		result = s.completeRolledBackPublications(writes)
	}
	return result
}

func (s *SQLiteMemory) rollbackAutomaticMemoryWrite(ltm VectorDB, write AutomaticMemoryWrite) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.rollbackAutomaticMemoryWriteContext(ctx, ltm, write)
}

func (s *SQLiteMemory) rollbackAutomaticMemoryWriteContext(ctx context.Context, ltm VectorDB, write AutomaticMemoryWrite) error {
	owned, ok := ltm.(OwnershipAwareVectorDB)
	if s == nil || s.db == nil || !ok || write.Metadata == nil || write.ContentHash == "" || len(write.Sources) != 1 {
		return fmt.Errorf("retained automatic memory %s: rollback ownership is unproven", write.DocID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("retain automatic memory %s: %w", write.DocID, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE memory_meta SET doc_id=doc_id WHERE doc_id=?`, write.DocID); err != nil {
		return err
	}
	var current MemoryMeta
	if err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, write.DocID), &current); err != nil {
		return err
	}
	sources, err := readMemoryExtractionSources(tx, write.DocID)
	if err != nil {
		return err
	}
	if !memoryMetaEqual(current, *write.Metadata) || !reflect.DeepEqual(sources, write.Sources) ||
		current.LastReviewedAt != "" || current.Protected || current.KeepForever || IsMemoryArchived(current) ||
		current.VerificationStatus != "unverified" || current.AccessCount != 0 || current.UsefulCount != 0 || current.UselessCount != 0 {
		return fmt.Errorf("retained automatic memory %s: metadata or sources changed", write.DocID)
	}
	var referenced bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_curation_events WHERE doc_id=?) OR
		EXISTS(SELECT 1 FROM memory_usage_log WHERE memory_id=?) OR
		EXISTS(SELECT 1 FROM memory_conflicts WHERE doc_id_left=? OR doc_id_right=?) OR
		EXISTS(SELECT 1 FROM file_embedding_docs WHERE doc_id=?)`, write.DocID, write.DocID, write.DocID, write.DocID, write.DocID).Scan(&referenced); err != nil {
		return err
	}
	var episodesExist bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='episodic_memories')`).Scan(&episodesExist); err != nil {
		return err
	}
	if episodesExist {
		var linked bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM episodic_memories,json_each(related_doc_ids) WHERE json_each.value=?)`, write.DocID).Scan(&linked); err != nil {
			return err
		}
		referenced = referenced || linked
	}
	if referenced {
		return fmt.Errorf("retained automatic memory %s: references exist", write.DocID)
	}
	var deleted bool
	if native, ok := ltm.(*ChromemVectorDB); ok {
		deleted, err = native.DeleteDocumentIfContentMatchesContext(ctx, write.DocID, write.ContentHash)
	} else {
		deleted, err = owned.DeleteDocumentIfContentMatches(write.DocID, write.ContentHash)
	}
	if err != nil {
		return fmt.Errorf("rollback automatic vector %s: %w", write.DocID, err)
	}
	if !deleted {
		return fmt.Errorf("retained automatic memory %s: vector content changed", write.DocID)
	}
	for _, table := range []string{"memory_extraction_sources", "memory_meta"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE doc_id=?`, write.DocID); err != nil {
			return fmt.Errorf("rollback automatic %s: %w", table, err)
		}
	}
	return tx.Commit()
}
