package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"crypto/rand"
)

const (
	automaticMemoryPublicationPrefix  = "memory.publication."
	automaticMemoryPublicationStaged  = "staged"
	automaticMemoryPublicationReady   = "ready"
	automaticMemoryPublicationPending = "memory_publication_pending"
)

// ErrAutomaticMemoryPublicationPending defers a repeated fact until its own
// earlier publication is resolved, without blocking unrelated memory writes.
var ErrAutomaticMemoryPublicationPending = errors.New("automatic memory publication is pending")

// AutomaticMemoryWriteOptions carries optional publication-time checks and a
// fact expiry. A nil ExpiresAt leaves existing expiry unchanged; a non-nil
// zero value clears it.
type AutomaticMemoryWriteOptions struct {
	ExpiresAt *time.Time
	Validate  func() error
}

// PreparedMemoryDocument holds a vectorized document without publishing it.
// Prepared backends return these only after all embeddings and identity checks
// succeed, before writing either canonical storage or the live index.
type PreparedMemoryDocument struct {
	ID        string
	Content   string
	Embedding []float32
	Metadata  map[string]string
}

// MemoryPublicationDocument is the source-free receipt used by the durable
// publication journal and by conditional recovery cleanup.
type MemoryPublicationDocument struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

// PreparedAutomaticMemoryWrite is returned by a prepared vector backend. Its
// private release hook keeps the backend's concept stripe and lifecycle lease
// held until publication has finished.
type PreparedAutomaticMemoryWrite struct {
	Result    VectorStoreResult
	Documents []PreparedMemoryDocument
	release   func()
}

// Release drops resources retained by a prepared write. It is safe to call
// more than once.
func (p *PreparedAutomaticMemoryWrite) Release() {
	if p == nil || p.release == nil {
		return
	}
	release := p.release
	p.release = nil
	release()
}

// PreparedAutomaticMemoryVectorDB separates preparation from durable corpus
// and in-memory index publication. Legacy VectorDB adapters need not implement
// it; guarded automatic writes fail before mutation when it is unavailable.
type PreparedAutomaticMemoryVectorDB interface {
	PrepareAutomaticMemoryWriteContext(context.Context, string, string) (*PreparedAutomaticMemoryWrite, error)
	StageAutomaticMemoryWriteContext(context.Context, string, []PreparedMemoryDocument) error
	ActivateAutomaticMemoryWriteContext(context.Context, string, []MemoryPublicationDocument) error
	CleanupAutomaticMemoryWriteContext(context.Context, string, []MemoryPublicationDocument) error
}

type automaticMemoryPublicationDocumentReader interface {
	ReadAutomaticMemoryPublicationDocumentContext(context.Context, string) (string, error)
}

type canonicalMemoryCorpusAvailability interface {
	hasCanonicalMemoryCorpus() bool
}

type automaticMemoryPublicationJournal struct {
	Version            int                         `json:"version"`
	Status             string                      `json:"status"`
	RequiresValidation bool                        `json:"requires_validation,omitempty"`
	Fingerprint        string                      `json:"fingerprint,omitempty"`
	Documents          []MemoryPublicationDocument `json:"documents"`
}

// StoreAutomaticMemoryDocumentWithOptionsContext publishes a memory fact with
// optional expiry and a caller-owned validation gate. The native prepared path
// journals source-free receipts before writing the corpus; unguarded legacy
// adapters keep the compatibility path.
func StoreAutomaticMemoryDocumentWithOptionsContext(ctx context.Context, s *SQLiteMemory, ltm VectorDB, concept, content string, details MemoryMetaUpdate, options AutomaticMemoryWriteOptions) (AutomaticMemoryStoreResult, error) {
	prepared, ok := ltm.(PreparedAutomaticMemoryVectorDB)
	guarded := options.Validate != nil || options.ExpiresAt != nil
	if ok && s != nil {
		// Older in-memory Chromem fixtures have no canonical corpus. Preserve
		// their empty-option compatibility path; guarded production writes still
		// require the durable staged protocol.
		hasCorpus := true
		if availability, knows := ltm.(canonicalMemoryCorpusAvailability); knows {
			hasCorpus = availability.hasCanonicalMemoryCorpus()
		}
		if hasCorpus || guarded {
			return storePreparedAutomaticMemoryDocument(ctx, s, ltm, prepared, concept, content, details, options)
		}
	}
	if guarded {
		return AutomaticMemoryStoreResult{}, fmt.Errorf("guarded automatic memory writes require native prepared publication")
	}
	return storeAutomaticMemoryDocument(ctx, s, ltm, concept, content, details, func(ctx context.Context, ltm VectorDB, concept, content string) (VectorStoreResult, error) {
		if err := startMemoryPublication(ctx); err != nil {
			return VectorStoreResult{}, err
		}
		return StoreDocumentWithOwnershipContext(ctx, ltm, concept, content)
	})
}

func storePreparedAutomaticMemoryDocument(ctx context.Context, s *SQLiteMemory, ltm VectorDB, backend PreparedAutomaticMemoryVectorDB, concept, content string, details MemoryMetaUpdate, options AutomaticMemoryWriteOptions) (result AutomaticMemoryStoreResult, resultErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := validateMemoryDocument(concept, content); err != nil {
		return result, err
	}
	if HasDurableCredential(concept + "\n" + content) {
		return result, fmt.Errorf("%w: credentials cannot enter durable memory", ErrAutomaticMemorySkipped)
	}
	if s == nil || s.db == nil {
		return result, fmt.Errorf("automatic memory metadata store is unavailable")
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return result, err
	}
	defer s.automaticMemoryMu.Unlock()
	s.bindArchiveVector(ltm)
	if err := s.backfillArchiveIdentities(ctx, ltm); err != nil {
		return result, err
	}
	session := strings.TrimSpace(details.SourceSessionID)
	if session == "" {
		session, _ = MemoryAnalysisSession(content)
	}
	if session == "" {
		session = "default"
	}
	identity := memoryDocumentIdentity(buildContentString(concept, content), "")
	correctionFact := CorrectionFactText(buildContentString(concept, content))
	validate := func() error {
		if options.Validate != nil {
			if err := options.Validate(); err != nil {
				return err
			}
		}
		blockedArchive, err := s.archivedIdentityBlocked(ctx, identity)
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
		blocked, err := s.IsCorrectedMemoryFact(correctionFact)
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
	storedContent := content
	if _, ok := AnalysisMemoryIdentity(concept, content, ""); ok && details.SourceType == "memory_analysis" {
		storedContent = MemoryAnalysisSourceMarker
		if details.SourceRole != "" {
			role := details.SourceRole
			if role != "user" && role != "assistant" && role != "user_and_assistant" {
				role = "unknown"
			}
			storedContent += " role:" + role
		}
	}
	storedIdentity := memoryDocumentIdentity(buildContentString(concept, storedContent), "")
	if err := ensureAutomaticMemoryPublicationNotPending(ctx, s, ltm, storedIdentity, contentSHA256(buildContentString(concept, storedContent))); err != nil {
		return result, err
	}
	prepared, err := backend.PrepareAutomaticMemoryWriteContext(ctx, concept, storedContent)
	if err != nil {
		return result, err
	}
	if prepared == nil {
		return result, fmt.Errorf("prepared vector backend returned no write")
	}
	defer prepared.Release()
	result.VectorStoreResult = prepared.Result
	if len(prepared.Result.CreatedIDs)+len(prepared.Result.ReusedIDs)+len(prepared.Result.UnknownIDs) == 0 {
		return result, fmt.Errorf("automatic memory write returned no document IDs")
	}
	if len(prepared.Result.UnknownIDs) != 0 {
		return result, fmt.Errorf("prepared automatic memory write returned unknown ownership IDs")
	}
	if len(prepared.Documents) != len(prepared.Result.CreatedIDs) {
		return result, fmt.Errorf("prepared automatic memory write has incomplete document receipts")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := validate(); err != nil {
		return result, err
	}
	pubCtx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	if err := validate(); err != nil {
		return result, err
	}
	var key, stagedValue string
	var receipts []MemoryPublicationDocument
	if len(prepared.Documents) > 0 {
		key = automaticMemoryPublicationPrefix + rand.Text()
		receipts = make([]MemoryPublicationDocument, 0, len(prepared.Documents))
		for _, document := range prepared.Documents {
			if document.ID == "" || document.Content == "" {
				return result, fmt.Errorf("prepared automatic memory document is incomplete")
			}
			receipts = append(receipts, MemoryPublicationDocument{ID: document.ID, Hash: contentSHA256(document.Content)})
		}
		journal := automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationStaged, RequiresValidation: options.Validate != nil, Fingerprint: prepared.Documents[0].Metadata["embedding_fingerprint"], Documents: receipts}
		stagedValue, err = jsonString(journal)
		if err != nil {
			return result, err
		}
		if _, err := s.db.ExecContext(pubCtx, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, key, stagedValue); err != nil {
			return result, fmt.Errorf("persist automatic memory publication: %w", err)
		}
		if err := backend.StageAutomaticMemoryWriteContext(pubCtx, key, prepared.Documents); err != nil {
			return result, cleanupStagedAutomaticMemoryPublication(pubCtx, s, backend, key, stagedValue, receipts, err)
		}
	}
	ids := append(append([]string(nil), prepared.Result.CreatedIDs...), prepared.Result.ReusedIDs...)
	if err := validate(); err != nil {
		if key != "" {
			return result, cleanupStagedAutomaticMemoryPublication(pubCtx, s, backend, key, stagedValue, receipts, err)
		}
		return result, err
	}
	var readyValue string
	if key != "" {
		journal := automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationReady, RequiresValidation: options.Validate != nil, Fingerprint: prepared.Documents[0].Metadata["embedding_fingerprint"], Documents: receipts}
		readyValue, err = jsonString(journal)
		if err != nil {
			return result, cleanupStagedAutomaticMemoryPublication(pubCtx, s, backend, key, stagedValue, receipts, err)
		}
	}
	if err := publishPreparedAutomaticMemoryMetadata(pubCtx, s, ids, details, session, storedIdentity, identity, correctionFact, options.ExpiresAt, key, stagedValue, readyValue); err != nil {
		if key != "" {
			return result, cleanupStagedAutomaticMemoryPublication(pubCtx, s, backend, key, stagedValue, receipts, err)
		}
		return result, err
	}
	if err := validate(); err != nil {
		return result, fmt.Errorf("automatic memory publication remains hidden: %w", err)
	}
	if key != "" {
		if err := validateReadyAutomaticMemoryPublication(pubCtx, s, ltm, receipts); err != nil {
			return result, fmt.Errorf("automatic memory publication remains hidden: %w", err)
		}
		if err := backend.ActivateAutomaticMemoryWriteContext(pubCtx, key, receipts); err != nil {
			return result, fmt.Errorf("activate automatic memory publication: %w", err)
		}
		if err := validate(); err != nil {
			return result, fmt.Errorf("automatic memory publication remains hidden after activation: %w", err)
		}
		if err := validateReadyAutomaticMemoryPublication(pubCtx, s, ltm, receipts); err != nil {
			return result, fmt.Errorf("automatic memory publication remains hidden after activation: %w", err)
		}
		if err := deleteAutomaticMemoryPublicationJournal(pubCtx, s.db, key, readyValue); err != nil {
			return result, fmt.Errorf("complete automatic memory publication: %w", err)
		}
		if native, ok := ltm.(*ChromemVectorDB); ok {
			for _, document := range prepared.Documents {
				receipt := MemoryPublicationDocument{ID: document.ID, Hash: contentSHA256(document.Content)}
				reindex, retireErr := native.RetireCompletedAutomaticMemoryPublicationTokenContext(pubCtx, key, receipt, true)
				if retireErr != nil {
					if native.logger != nil {
						native.logger.Warn("Completed automatic memory publication token retirement failed", "doc_id", document.ID, "error", retireErr)
					}
					continue
				}
				if reindex {
					if err := native.reconcileCorpus(pubCtx); err != nil && native.logger != nil {
						native.logger.Warn("Reindex completed automatic memory after model change failed", "doc_id", document.ID, "error", err)
					}
				}
			}
		}
	}
	for _, document := range prepared.Documents {
		meta, err := s.GetMemoryMeta(document.ID)
		if err != nil {
			return result, err
		}
		sources, err := s.GetMemoryExtractionSources(document.ID)
		if err != nil {
			return result, err
		}
		result.Writes = append(result.Writes, AutomaticMemoryWrite{DocID: document.ID, ContentHash: contentSHA256(document.Content), Metadata: &meta, Sources: sources})
	}
	return result, nil
}

func ensureAutomaticMemoryPublicationNotPending(ctx context.Context, s *SQLiteMemory, vector VectorDB, identity, contentHash string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT p.key,json_extract(d.value,'$.id'),json_extract(d.value,'$.hash')
		FROM memory_maintenance_meta p,json_each(p.value,'$.documents') d
		LEFT JOIN memory_document_identities i ON i.doc_id=json_extract(d.value,'$.id')
		WHERE p.key LIKE ? AND (i.identity=? OR json_extract(d.value,'$.hash')=?)`, automaticMemoryPublicationPrefix+"%", identity, contentHash)
	if err != nil {
		return fmt.Errorf("check pending automatic memory identity: %w", err)
	}
	type candidate struct {
		key      string
		document MemoryPublicationDocument
	}
	var candidates []candidate
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.key, &item.document.ID, &item.document.Hash); err != nil {
			rows.Close()
			return fmt.Errorf("read pending automatic memory identity: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("finish pending automatic memory identity check: %w", err)
	}
	corpus, canReadCorpus := vector.(memoryPublicationCorpusEligibility)
	reader, canReadContent := vector.(automaticMemoryPublicationDocumentReader)
	for _, item := range candidates {
		if !canReadCorpus || !canReadContent {
			return ErrAutomaticMemoryPublicationPending
		}
		states, err := corpus.ReadAutomaticMemoryPublicationCorpusStateContext(ctx, []string{item.document.ID})
		if err != nil {
			return err
		}
		// A stale receipt must not fence a foreign or recycled document ID.
		if states[item.document.ID].publicationID != item.key {
			continue
		}
		content, err := reader.ReadAutomaticMemoryPublicationDocumentContext(ctx, item.document.ID)
		if err != nil {
			return err
		}
		if contentSHA256(content) == item.document.Hash {
			return ErrAutomaticMemoryPublicationPending
		}
	}
	return nil
}

func publishPreparedAutomaticMemoryMetadata(ctx context.Context, s *SQLiteMemory, ids []string, details MemoryMetaUpdate, session, storedIdentity, incomingIdentity, correctionFact string, expiresAt *time.Time, publicationKey, stagedValue, readyValue string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateAutomaticMemoryPublicationTx(ctx, tx, session, incomingIdentity, correctionFact); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := insertMemoryMeta(tx, id, details); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO memory_document_identities(doc_id,identity) VALUES(?,?) ON CONFLICT(doc_id) DO NOTHING`, id, storedIdentity); err != nil {
			return err
		}
		if err := recordMemoryExtractionSourceTx(ctx, tx, id, details.SourceType, session); err != nil {
			return err
		}
		if expiresAt != nil {
			if err := setMemoryExpiryTx(ctx, tx, id, *expiresAt); err != nil {
				return fmt.Errorf("record automatic memory expiry: %w", err)
			}
		}
	}
	if publicationKey != "" {
		result, err := tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=? WHERE key=? AND value=?`, readyValue, publicationKey, stagedValue)
		if err != nil {
			return err
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return errors.Join(fmt.Errorf("mark automatic memory publication ready"), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit automatic memory publication metadata: %w", err)
	}
	return nil
}

func cleanupStagedAutomaticMemoryPublication(ctx context.Context, s *SQLiteMemory, backend PreparedAutomaticMemoryVectorDB, key, stagedValue string, docs []MemoryPublicationDocument, cause error) error {
	if err := ensureAutomaticMemoryPublicationUnreferenced(ctx, s, docs); err != nil {
		return errors.Join(cause, fmt.Errorf("retain staged automatic memory publication %s: %w", key, err))
	}
	if err := backend.CleanupAutomaticMemoryWriteContext(ctx, key, docs); err != nil {
		return errors.Join(cause, fmt.Errorf("retain staged automatic memory publication %s: %w", key, err))
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, key, stagedValue); err != nil {
		return errors.Join(cause, fmt.Errorf("clear staged automatic memory publication %s: %w", key, err))
	}
	return cause
}

func ensureAutomaticMemoryPublicationUnreferenced(ctx context.Context, s *SQLiteMemory, docs []MemoryPublicationDocument) error {
	ids := make([]string, 0, len(docs))
	for _, document := range docs {
		ids = append(ids, document.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	query := `SELECT EXISTS(SELECT 1 FROM memory_meta WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_extraction_sources WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_document_identities WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_expiry WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_curation_events WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_usage_log WHERE memory_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM file_embedding_docs WHERE doc_id IN (` + corpusPlaceholders(len(ids)) + `)) OR
		EXISTS(SELECT 1 FROM memory_conflicts WHERE doc_id_left IN (` + corpusPlaceholders(len(ids)) + `) OR doc_id_right IN (` + corpusPlaceholders(len(ids)) + `))`
	args := make([]any, 0, len(ids)*9)
	for i := 0; i < 9; i++ {
		args = append(args, stringsToAny(ids)...)
	}
	var referenced bool
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&referenced); err != nil {
		return fmt.Errorf("check staged memory references: %w", err)
	}
	if referenced {
		return fmt.Errorf("staged document IDs already have memory references")
	}
	return nil
}

func validateAutomaticMemoryPublicationTx(ctx context.Context, tx *sql.Tx, session, identity, correctionFact string) error {
	if session != "" {
		var hasFence bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='deleted_chat_sessions')`).Scan(&hasFence); err != nil {
			return fmt.Errorf("inspect memory source privacy fence: %w", err)
		}
		var deleted bool
		if hasFence {
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, session).Scan(&deleted); err != nil {
				return fmt.Errorf("check memory source privacy fence: %w", err)
			}
		}
		if deleted {
			return fmt.Errorf("%w: source session deleted", ErrAutomaticMemorySkipped)
		}
		current, err := memorySourceSessionGenerationCurrent(ctx, tx, session)
		if err != nil {
			return fmt.Errorf("check memory source session generation: %w", err)
		}
		if !current {
			return fmt.Errorf("%w: source session generation changed", ErrAutomaticMemorySkipped)
		}
	}
	var blocked bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_archive_identities WHERE identity=?)`, identity).Scan(&blocked); err != nil {
		return fmt.Errorf("check archived memory identity: %w", err)
	}
	if blocked {
		return fmt.Errorf("%w: fact is archived", ErrAutomaticMemorySkipped)
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_correction_guards WHERE identity=?)`, correctionIdentity(correctionFact)).Scan(&blocked); err != nil {
		return fmt.Errorf("check corrected memory fact: %w", err)
	}
	if blocked {
		return fmt.Errorf("%w: fact was corrected; automatic extraction cannot restore it", ErrAutomaticMemorySkipped)
	}
	return nil
}

func recordMemoryExtractionSourceTx(ctx context.Context, tx *sql.Tx, docID, source, session string) error {
	if source = strings.TrimSpace(source); source == "" {
		source = "system"
	}
	if session = strings.TrimSpace(session); session == "" {
		session = "default"
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	_, err := tx.ExecContext(ctx, `INSERT INTO memory_extraction_sources VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(doc_id,source_type,session_id) DO UPDATE SET
		first_seen_at=MIN(first_seen_at,excluded.first_seen_at),last_seen_at=MAX(last_seen_at,excluded.last_seen_at)`, docID, source, session, now, now)
	if err != nil {
		return fmt.Errorf("record automatic memory source: %w", err)
	}
	return nil
}

func setMemoryExpiryTx(ctx context.Context, tx *sql.Tx, docID string, expiresAt time.Time) error {
	if expiresAt.IsZero() {
		_, err := tx.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id=?`, docID)
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO memory_expiry(doc_id,expires_at) VALUES(?,?)
		ON CONFLICT(doc_id) DO UPDATE SET expires_at=excluded.expires_at`, docID, expiresAt.UTC().Format(memoryExpiryLayout))
	return err
}

func jsonString(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
