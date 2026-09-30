package memory

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// AutomaticMemoryWrite records only ownership established by this operation.
// A nil snapshot is deliberately insufficient authority for rollback.
type AutomaticMemoryWrite struct {
	DocID       string
	ContentHash string
	Metadata    *MemoryMeta
	Sources     []MemoryExtractionSource
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
	var result AutomaticMemoryStoreResult
	session := "default"
	if _, ok := AnalysisMemoryIdentity(concept, content, ""); ok && details.SourceType == "memory_analysis" {
		session, _ = MemoryAnalysisSession(content)
		content = MemoryAnalysisSourceMarker
	}
	stored, storeErr := StoreDocumentWithOwnership(ltm, concept, content)
	result.VectorStoreResult = stored
	resultErr := storeErr
	for _, id := range stored.CreatedIDs {
		write := AutomaticMemoryWrite{DocID: id}
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
	return result, resultErr
}

// RollbackAutomaticMemoryWrites excludes curated, accessed, referenced and
// reused documents. The SQLite write lock prevents curation between validation
// and the vector store's conditional content deletion.
func (s *SQLiteMemory) RollbackAutomaticMemoryWrites(ltm VectorDB, writes []AutomaticMemoryWrite) error {
	var result error
	for _, write := range writes {
		result = errors.Join(result, s.rollbackAutomaticMemoryWrite(ltm, write))
	}
	return result
}

func (s *SQLiteMemory) rollbackAutomaticMemoryWrite(ltm VectorDB, write AutomaticMemoryWrite) error {
	owned, ok := ltm.(OwnershipAwareVectorDB)
	if s == nil || s.db == nil || !ok || write.Metadata == nil || write.ContentHash == "" || len(write.Sources) != 1 {
		return fmt.Errorf("retained automatic memory %s: rollback ownership is unproven", write.DocID)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("retain automatic memory %s: %w", write.DocID, err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE memory_meta SET doc_id=doc_id WHERE doc_id=?`, write.DocID); err != nil {
		return err
	}
	var current MemoryMeta
	if err := scanMemoryMeta(tx.QueryRow(`SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, write.DocID), &current); err != nil {
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
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_curation_events WHERE doc_id=?) OR
		EXISTS(SELECT 1 FROM memory_usage_log WHERE memory_id=?) OR
		EXISTS(SELECT 1 FROM memory_conflicts WHERE doc_id_left=? OR doc_id_right=?) OR
		EXISTS(SELECT 1 FROM file_embedding_docs WHERE doc_id=?)`, write.DocID, write.DocID, write.DocID, write.DocID, write.DocID).Scan(&referenced); err != nil {
		return err
	}
	var episodesExist bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='episodic_memories')`).Scan(&episodesExist); err != nil {
		return err
	}
	if episodesExist {
		var linked bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM episodic_memories,json_each(related_doc_ids) WHERE json_each.value=?)`, write.DocID).Scan(&linked); err != nil {
			return err
		}
		referenced = referenced || linked
	}
	if referenced {
		return fmt.Errorf("retained automatic memory %s: references exist", write.DocID)
	}
	deleted, err := owned.DeleteDocumentIfContentMatches(write.DocID, write.ContentHash)
	if err != nil {
		return fmt.Errorf("rollback automatic vector %s: %w", write.DocID, err)
	}
	if !deleted {
		return fmt.Errorf("retained automatic memory %s: vector content changed", write.DocID)
	}
	for _, table := range []string{"memory_extraction_sources", "memory_meta"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE doc_id=?`, write.DocID); err != nil {
			return fmt.Errorf("rollback automatic %s: %w", table, err)
		}
	}
	return tx.Commit()
}
