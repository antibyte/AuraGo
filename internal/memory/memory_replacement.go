package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const memoryMetaSelectColumns = `
	doc_id, access_count, last_accessed, last_event_at,
	extraction_confidence, verification_status, source_type, source_reliability,
	useful_count, useless_count, COALESCE(last_effectiveness_at, ''),
	protected, keep_forever, COALESCE(archived_at, ''),
	COALESCE(archived_reason, ''), COALESCE(last_reviewed_at, ''),
	COALESCE(review_note, '')`

// memoryReplacementMetadataCommitError means SQLite could not tell the
// caller whether the metadata transaction committed. Replacement callers must
// retain the newly created vector in this case; deleting it could leave an
// archived source without a replacement if the commit did succeed.
type memoryReplacementMetadataCommitError struct {
	err error
}

func (e *memoryReplacementMetadataCommitError) Error() string {
	return e.err.Error()
}

func (e *memoryReplacementMetadataCommitError) Unwrap() error {
	return e.err
}

// EnsureMemoryMeta creates a tracking row without touching any existing
// metadata. It is the safe fallback for a VectorDB that cannot identify
// whether a returned ID was newly created or reused.
func (s *SQLiteMemory) EnsureMemoryMeta(docID string) error {
	return s.ensureMemoryMeta(docID, MemoryMetaUpdate{})
}

// EnsureMemoryMetaWithDetails inserts a metadata row with provenance defaults
// while preserving any existing row byte-for-byte. It is safe for ownership
// results whose vector ID may be either newly created or reused.
func (s *SQLiteMemory) EnsureMemoryMetaWithDetails(docID string, details MemoryMetaUpdate) error {
	return s.ensureMemoryMeta(docID, details)
}

func (s *SQLiteMemory) ensureMemoryMeta(docID string, details MemoryMetaUpdate) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory metadata store is unavailable")
	}
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return nil
	}
	extractionConfidence := details.ExtractionConfidence
	if extractionConfidence <= 0 {
		extractionConfidence = 0.75
	}
	if extractionConfidence > 1 {
		extractionConfidence = 1
	}
	verificationStatus := strings.TrimSpace(details.VerificationStatus)
	if verificationStatus == "" {
		verificationStatus = "unverified"
	}
	sourceType := strings.TrimSpace(details.SourceType)
	if sourceType == "" {
		sourceType = "system"
	}
	sourceReliability := details.SourceReliability
	if sourceReliability <= 0 {
		sourceReliability = 0.70
	}
	if sourceReliability > 1 {
		sourceReliability = 1
	}
	if _, err := s.db.Exec(`
		INSERT INTO memory_meta (
			doc_id, access_count, last_accessed, last_event_at,
			extraction_confidence, verification_status, source_type, source_reliability
		)
		VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?, ?, ?)
		ON CONFLICT(doc_id) DO NOTHING
	`, docID, extractionConfidence, verificationStatus, sourceType, sourceReliability); err != nil {
		return fmt.Errorf("ensure memory metadata %s: %w", docID, err)
	}
	return nil
}

// GetMemoryMeta returns one complete memory_meta snapshot for replacement
// compare-and-commit operations.
func (s *SQLiteMemory) GetMemoryMeta(docID string) (MemoryMeta, error) {
	var meta MemoryMeta
	if s == nil || s.db == nil {
		return meta, fmt.Errorf("memory metadata store is unavailable")
	}
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return meta, fmt.Errorf("memory metadata document ID is required")
	}
	row := s.db.QueryRow(`SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id = ?`, docID)
	if err := scanMemoryMeta(row, &meta); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return meta, fmt.Errorf("memory metadata %s not found: %w", docID, err)
		}
		return meta, fmt.Errorf("get memory metadata %s: %w", docID, err)
	}
	return meta, nil
}

type memoryMetaScanner interface {
	Scan(dest ...any) error
}

func scanMemoryMeta(scanner memoryMetaScanner, meta *MemoryMeta) error {
	return scanner.Scan(
		&meta.DocID,
		&meta.AccessCount,
		&meta.LastAccessed,
		&meta.LastEventAt,
		&meta.ExtractionConfidence,
		&meta.VerificationStatus,
		&meta.SourceType,
		&meta.SourceReliability,
		&meta.UsefulCount,
		&meta.UselessCount,
		&meta.LastEffectivenessAt,
		&meta.Protected,
		&meta.KeepForever,
		&meta.ArchivedAt,
		&meta.ArchivedReason,
		&meta.LastReviewedAt,
		&meta.ReviewNote,
	)
}

func memoryMetaEqual(left, right MemoryMeta) bool {
	return left.DocID == right.DocID &&
		left.AccessCount == right.AccessCount &&
		left.LastAccessed == right.LastAccessed &&
		left.LastEventAt == right.LastEventAt &&
		left.ExtractionConfidence == right.ExtractionConfidence &&
		left.VerificationStatus == right.VerificationStatus &&
		left.SourceType == right.SourceType &&
		left.SourceReliability == right.SourceReliability &&
		left.UsefulCount == right.UsefulCount &&
		left.UselessCount == right.UselessCount &&
		left.LastEffectivenessAt == right.LastEffectivenessAt &&
		left.Protected == right.Protected &&
		left.KeepForever == right.KeepForever &&
		left.ArchivedAt == right.ArchivedAt &&
		left.ArchivedReason == right.ArchivedReason &&
		left.LastReviewedAt == right.LastReviewedAt &&
		left.ReviewNote == right.ReviewNote
}

// archiveAndCopyMemoryMeta archives the old tracking row and copies its full
// snapshot to every replacement ID in one SQLite transaction. The transaction
// first compares every field exactly; a concurrent metadata write after that
// read causes SQLite's snapshot upgrade to fail instead of overwriting it.
func (s *SQLiteMemory) archiveAndCopyMemoryMeta(oldID string, expected MemoryMeta, newIDs []string, reason, actor string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory metadata store is unavailable")
	}
	oldID = strings.TrimSpace(oldID)
	if oldID == "" || expected.DocID != oldID {
		return fmt.Errorf("memory replacement metadata snapshot does not match old document")
	}
	if len(newIDs) == 0 {
		return fmt.Errorf("memory replacement requires at least one new document ID")
	}
	if strings.TrimSpace(expected.ArchivedAt) != "" || IsMemoryArchived(expected) {
		return fmt.Errorf("memory replacement source %s is already archived", oldID)
	}
	if strings.TrimSpace(reason) == "" {
		reason = "memory replacement"
	}
	if strings.TrimSpace(actor) == "" {
		actor = "system"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin memory replacement metadata transaction: %w", err)
	}
	defer tx.Rollback()
	var transactionMeta MemoryMeta
	if err := scanMemoryMeta(tx.QueryRow(`SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id = ?`, oldID), &transactionMeta); err != nil {
		return fmt.Errorf("read memory replacement source %s in transaction: %w", oldID, err)
	}
	if !memoryMetaEqual(transactionMeta, expected) {
		return fmt.Errorf("memory replacement source %s changed before metadata commit", oldID)
	}

	archiveResult, err := tx.Exec(`
		UPDATE memory_meta
		SET verification_status = ?,
		    archived_at = CURRENT_TIMESTAMP,
		    archived_reason = ?,
		    last_reviewed_at = CURRENT_TIMESTAMP,
		    review_note = ?,
		    last_event_at = CURRENT_TIMESTAMP
		WHERE doc_id = ?
	`, MemoryVerificationArchived, reason, reason, oldID)
	if err != nil {
		return fmt.Errorf("archive memory replacement source %s: %w", oldID, err)
	}
	rows, err := archiveResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("check archived memory replacement source %s: %w", oldID, err)
	}
	if rows != 1 {
		return fmt.Errorf("memory replacement source %s changed before metadata commit", oldID)
	}

	for _, newID := range newIDs {
		newID = strings.TrimSpace(newID)
		if newID == "" || newID == oldID {
			return fmt.Errorf("invalid memory replacement document ID %q", newID)
		}
		var lastEffectiveness any
		if expected.LastEffectivenessAt != "" {
			lastEffectiveness = expected.LastEffectivenessAt
		}
		var lastReviewed any
		if expected.LastReviewedAt != "" {
			lastReviewed = expected.LastReviewedAt
		}
		if _, err := tx.Exec(`
			INSERT INTO memory_meta (
				doc_id, access_count, last_accessed, last_event_at,
				extraction_confidence, verification_status, source_type, source_reliability,
				useful_count, useless_count, last_effectiveness_at, protected, keep_forever,
				archived_at, archived_reason, last_reviewed_at, review_note
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, '', ?, ?)
		`, newID, expected.AccessCount, expected.LastAccessed, expected.LastEventAt,
			expected.ExtractionConfidence, expected.VerificationStatus, expected.SourceType,
			expected.SourceReliability, expected.UsefulCount, expected.UselessCount,
			lastEffectiveness, expected.Protected, expected.KeepForever,
			lastReviewed, expected.ReviewNote); err != nil {
			return fmt.Errorf("copy memory replacement metadata to %s: %w", newID, err)
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO memory_curation_events
			(doc_id, action, actor, previous_status, new_status, reason, dry_run)
		VALUES (?, 'replace', ?, ?, ?, ?, 0)
	`, oldID, actor, expected.VerificationStatus, MemoryVerificationArchived, reason); err != nil {
		return fmt.Errorf("record memory replacement event %s: %w", oldID, err)
	}
	if err := tx.Commit(); err != nil {
		return &memoryReplacementMetadataCommitError{
			err: fmt.Errorf("commit memory replacement metadata %s: %w", oldID, err),
		}
	}
	return nil
}

func contentSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func cleanupCreatedReplacementArtifacts(ltm VectorDB, s *SQLiteMemory, ids []string) error {
	var joinedErr error
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := ltm.DeleteDocument(id); err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("delete replacement vector %s: %w", id, err))
			continue
		}
		if s != nil {
			if err := s.DeleteMemoryMeta(id); err != nil {
				joinedErr = errors.Join(joinedErr, fmt.Errorf("delete replacement metadata %s: %w", id, err))
			}
		}
	}
	return joinedErr
}

// ReplaceMemoryDocument stages a force-created replacement, copies complete
// metadata, archives the old row, and conditionally retires the old vector.
// It fails closed when the backend cannot report ownership. Once metadata is
// committed, a failed old-vector retirement is returned as an error while the
// new replacement remains active; deleting it could lose the only valid copy.
func (s *SQLiteMemory) ReplaceMemoryDocument(ltm VectorDB, oldID, concept, expectedContent, replacement string, expected MemoryMeta, reason, actor string) ([]string, error) {
	oldID = strings.TrimSpace(oldID)
	if s == nil || ltm == nil || oldID == "" || strings.TrimSpace(expectedContent) == "" || strings.TrimSpace(replacement) == "" {
		return nil, fmt.Errorf("memory replacement requires store, old document ID, expected content, and replacement content")
	}
	owned, ok := ltm.(OwnershipAwareVectorDB)
	if !ok {
		return nil, fmt.Errorf("vector store does not support ownership-aware replacement")
	}
	if expected.DocID == "" {
		expected.DocID = oldID
	}
	if expected.DocID != oldID {
		return nil, fmt.Errorf("memory replacement snapshot document ID %q does not match %q", expected.DocID, oldID)
	}

	oldContent, err := ltm.GetByID(oldID)
	if err != nil {
		return nil, fmt.Errorf("read memory replacement source %s: %w", oldID, err)
	}
	if oldContent != expectedContent {
		return nil, fmt.Errorf("memory replacement source %s changed before staging", oldID)
	}
	oldHash := contentSHA256(oldContent)
	currentMeta, err := s.GetMemoryMeta(oldID)
	if err != nil {
		return nil, err
	}
	if currentMeta.Protected || currentMeta.KeepForever {
		return nil, fmt.Errorf("memory replacement source %s is protected", oldID)
	}
	if IsMemoryArchived(currentMeta) {
		return nil, fmt.Errorf("memory replacement source %s is already archived", oldID)
	}
	if !memoryMetaEqual(currentMeta, expected) {
		return nil, fmt.Errorf("memory replacement source %s metadata changed before staging", oldID)
	}

	stored, err := owned.StoreDocumentOwned(concept, replacement, VectorStoreForceCreate)
	if err != nil {
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		if rollbackErr != nil {
			return nil, errors.Join(fmt.Errorf("store memory replacement %s: %w", oldID, err), rollbackErr)
		}
		return nil, fmt.Errorf("store memory replacement %s: %w", oldID, err)
	}
	if len(stored.ReusedIDs) != 0 || len(stored.UnknownIDs) != 0 || len(stored.CreatedIDs) == 0 {
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		base := fmt.Errorf("replacement store returned reused, unknown, or no IDs for %s", oldID)
		if rollbackErr != nil {
			return nil, errors.Join(base, rollbackErr)
		}
		return nil, base
	}

	latestContent, err := ltm.GetByID(oldID)
	if err != nil || contentSHA256(latestContent) != oldHash {
		if err == nil {
			err = fmt.Errorf("source content changed before replacement commit")
		}
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		if rollbackErr != nil {
			return nil, errors.Join(fmt.Errorf("memory replacement source %s changed: %w", oldID, err), rollbackErr)
		}
		return nil, fmt.Errorf("memory replacement source %s changed: %w", oldID, err)
	}
	latestMeta, err := s.GetMemoryMeta(oldID)
	if err != nil {
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	if latestMeta.Protected || latestMeta.KeepForever {
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		base := fmt.Errorf("memory replacement source %s is protected or keep-forever before commit", oldID)
		if rollbackErr != nil {
			return nil, errors.Join(base, rollbackErr)
		}
		return nil, base
	}
	if IsMemoryArchived(latestMeta) || !memoryMetaEqual(latestMeta, expected) {
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		base := fmt.Errorf("memory replacement source %s metadata changed before commit", oldID)
		if rollbackErr != nil {
			return nil, errors.Join(base, rollbackErr)
		}
		return nil, base
	}
	if err := s.archiveAndCopyMemoryMeta(oldID, expected, stored.CreatedIDs, reason, actor); err != nil {
		var commitErr *memoryReplacementMetadataCommitError
		if errors.As(err, &commitErr) {
			return stored.CreatedIDs, err
		}
		rollbackErr := cleanupCreatedReplacementArtifacts(ltm, s, stored.CreatedIDs)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}

	retired, err := owned.DeleteDocumentIfContentMatches(oldID, oldHash)
	if err != nil {
		return stored.CreatedIDs, fmt.Errorf("replacement committed but old vector %s could not be retired: %w", oldID, err)
	}
	if !retired {
		return stored.CreatedIDs, fmt.Errorf("replacement committed but old vector %s changed before retirement", oldID)
	}
	if err := s.CleanupDeletedVectorDocumentReferences(oldID); err != nil {
		return stored.CreatedIDs, fmt.Errorf("cleanup old vector doc references %s: %w", oldID, err)
	}
	return stored.CreatedIDs, nil
}
