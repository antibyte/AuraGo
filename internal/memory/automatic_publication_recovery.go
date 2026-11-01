package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RecoverAutomaticMemoryPublicationsContext resumes only the states that can
// be proven from the version-1 source-free journal. Ready entries require the
// same current permission validator used by guarded writes.
func (s *SQLiteMemory) RecoverAutomaticMemoryPublicationsContext(ctx context.Context, vector VectorDB, validate func() error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("automatic memory metadata store is unavailable")
	}
	backend, ok := vector.(PreparedAutomaticMemoryVectorDB)
	if !ok {
		return fmt.Errorf("automatic memory publication recovery requires a prepared vector backend")
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return err
	}
	defer s.automaticMemoryMu.Unlock()
	s.bindArchiveVector(vector)
	pubCtx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return err
	}
	defer release()
	rows, err := s.db.QueryContext(pubCtx, `SELECT key,value FROM memory_maintenance_meta WHERE key LIKE ? ORDER BY key`, automaticMemoryPublicationPrefix+"%")
	if err != nil {
		return fmt.Errorf("read automatic memory publication journals: %w", err)
	}
	type entry struct{ key, value string }
	var entries []entry
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.key, &item.value); err != nil {
			rows.Close()
			return fmt.Errorf("scan automatic memory publication journal: %w", err)
		}
		entries = append(entries, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("finish automatic memory publication journal scan: %w", err)
	}
	native, isNative := vector.(*ChromemVectorDB)
	if len(entries) == 0 {
		if !isNative {
			return nil
		}
		completedTokens, err := native.hasCompletedAutomaticMemoryPublicationTokens(pubCtx)
		if err != nil {
			return fmt.Errorf("scan completed automatic memory publication tokens: %w", err)
		}
		// Pending orphan tokens are intentionally left hidden and fenced. Only
		// completed rows need the vector backend to retire the token and, after
		// a model change, rebuild their index from canonical content.
		if !completedTokens {
			return nil
		}
		if err := native.waitMemoryPublicationReady(pubCtx); err != nil {
			return err
		}
	} else if isNative {
		if err := native.waitMemoryPublicationReady(pubCtx); err != nil {
			return err
		}
	}
	var allErr error
	activeKeys := make(map[string]struct{}, len(entries))
	for _, item := range entries {
		activeKeys[item.key] = struct{}{}
	}
	for _, item := range entries {
		var journal automaticMemoryPublicationJournal
		if err := json.Unmarshal([]byte(item.value), &journal); err != nil || journal.Version != 1 || (journal.Status != automaticMemoryPublicationStaged && journal.Status != automaticMemoryPublicationReady) {
			if err == nil {
				err = fmt.Errorf("unsupported version or state")
			}
			allErr = errors.Join(allErr, fmt.Errorf("invalid automatic memory publication journal %s: %w", item.key, err))
			continue
		}
		if journal.Status == automaticMemoryPublicationStaged {
			if err := ensureAutomaticMemoryPublicationUnreferenced(pubCtx, s, journal.Documents); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("retain staged automatic memory publication %s: %w", item.key, err))
				continue
			}
			if err := backend.CleanupAutomaticMemoryWriteContext(pubCtx, item.key, journal.Documents); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("clean staged automatic memory publication %s: %w", item.key, err))
				continue
			}
			if err := deleteAutomaticMemoryPublicationJournal(pubCtx, s.db, item.key, item.value); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("clear staged automatic memory publication %s: %w", item.key, err))
			} else {
				delete(activeKeys, item.key)
			}
			continue
		}
		if journal.RequiresValidation && validate == nil {
			allErr = errors.Join(allErr, fmt.Errorf("ready automatic memory publication %s requires current validation", item.key))
			continue
		}
		if validate != nil {
			if err := validate(); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("validate ready automatic memory publication %s: %w", item.key, err))
				continue
			}
		}
		archived, err := allAutomaticMemoryPublicationDocumentsArchived(pubCtx, s, journal.Documents)
		if err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("check archived ready automatic memory publication %s: %w", item.key, err))
			continue
		}
		if archived {
			if err := backend.CleanupAutomaticMemoryWriteContext(pubCtx, item.key, journal.Documents); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("clean archived ready automatic memory publication %s: %w", item.key, err))
				continue
			}
			if err := deleteAutomaticMemoryPublicationJournal(pubCtx, s.db, item.key, item.value); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("complete archived automatic memory publication %s: %w", item.key, err))
			} else {
				delete(activeKeys, item.key)
			}
			continue
		}
		if err := validateReadyAutomaticMemoryPublication(pubCtx, s, vector, journal.Documents); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("recheck ready automatic memory publication %s: %w", item.key, err))
			continue
		}
		if err := backend.ActivateAutomaticMemoryWriteContext(pubCtx, item.key, journal.Documents); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("activate ready automatic memory publication %s: %w", item.key, err))
			continue
		}
		if validate != nil {
			if err := validate(); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("revalidate ready automatic memory publication %s after activation: %w", item.key, err))
				continue
			}
		}
		if err := validateReadyAutomaticMemoryPublication(pubCtx, s, vector, journal.Documents); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("recheck ready automatic memory publication %s after activation: %w", item.key, err))
			continue
		}
		if err := deleteAutomaticMemoryPublicationJournal(pubCtx, s.db, item.key, item.value); err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("complete ready automatic memory publication %s: %w", item.key, err))
		} else {
			delete(activeKeys, item.key)
		}
	}
	if isNative {
		needsReconcile, err := retireCompletedAutomaticMemoryPublicationTokens(pubCtx, s, native, activeKeys)
		if err != nil {
			allErr = errors.Join(allErr, fmt.Errorf("retire completed automatic memory publication tokens: %w", err))
		} else if needsReconcile {
			if err := native.reconcileCorpus(pubCtx); err != nil {
				allErr = errors.Join(allErr, fmt.Errorf("reconcile completed memories after model change: %w", err))
			}
		}
	}
	return allErr
}

func deleteAutomaticMemoryPublicationJournal(ctx context.Context, db *sql.DB, key, value string) error {
	result, err := db.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, key, value)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return errors.Join(fmt.Errorf("memory publication journal changed before completion"), err)
	}
	return nil
}

func retireCompletedAutomaticMemoryPublicationTokens(ctx context.Context, s *SQLiteMemory, vector *ChromemVectorDB, activeKeys map[string]struct{}) (bool, error) {
	if vector.corpus == nil {
		return false, nil
	}
	needsReconcile := false
	afterID := ""
	for {
		page, err := vector.corpus.documentPage(ctx, "aurago_memories", afterID, false)
		if err != nil {
			return needsReconcile, err
		}
		if len(page) == 0 {
			break
		}
		for _, snapshot := range page {
			doc := snapshot.document
			publicationID := doc.Metadata["memory_publication_id"]
			if publicationID == "" || snapshot.pending != 0 {
				continue
			}
			if _, active := activeKeys[publicationID]; active {
				continue
			}
			var meta MemoryMeta
			if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, doc.ID), &meta); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					err = fmt.Errorf("completed token has no memory metadata")
				}
				quarantineErr := vector.BlockCompletedAutomaticMemoryPublicationTokenContext(ctx, publicationID, MemoryPublicationDocument{ID: doc.ID, Hash: contentSHA256(doc.Content)})
				return needsReconcile, errors.Join(fmt.Errorf("read completed memory metadata %s: %w", doc.ID, err), quarantineErr)
			}
			var identity string
			if err := s.db.QueryRowContext(ctx, `SELECT identity FROM memory_document_identities WHERE doc_id=?`, doc.ID).Scan(&identity); err != nil || identity == "" {
				if err == nil {
					err = fmt.Errorf("completed token has no durable identity")
				}
				quarantineErr := vector.BlockCompletedAutomaticMemoryPublicationTokenContext(ctx, publicationID, MemoryPublicationDocument{ID: doc.ID, Hash: contentSHA256(doc.Content)})
				return needsReconcile, errors.Join(fmt.Errorf("read completed memory identity %s: %w", doc.ID, err), quarantineErr)
			}
			var hasSource bool
			if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_extraction_sources WHERE doc_id=?)`, doc.ID).Scan(&hasSource); err != nil || !hasSource {
				if err == nil {
					err = fmt.Errorf("completed token has no source provenance")
				}
				quarantineErr := vector.BlockCompletedAutomaticMemoryPublicationTokenContext(ctx, publicationID, MemoryPublicationDocument{ID: doc.ID, Hash: contentSHA256(doc.Content)})
				return needsReconcile, errors.Join(fmt.Errorf("read completed memory provenance %s: %w", doc.ID, err), quarantineErr)
			}
			receipt := MemoryPublicationDocument{ID: doc.ID, Hash: contentSHA256(doc.Content)}
			// This is an already-published row. A later chat deletion or policy
			// change does not revoke an existing fact when the user kept memories;
			// only prove completion, durable identity and provenance here. The
			// ready-journal path above retains its stricter new-publication checks.
			reindex, err := vector.RetireCompletedAutomaticMemoryPublicationTokenContext(ctx, publicationID, receipt, !IsMemoryArchived(meta))
			if err != nil {
				return needsReconcile, err
			}
			needsReconcile = needsReconcile || reindex
		}
		afterID = page[len(page)-1].document.ID
		if len(page) < corpusPageSize {
			break
		}
	}
	return needsReconcile, nil
}

func (cv *ChromemVectorDB) hasCompletedAutomaticMemoryPublicationTokens(ctx context.Context) (bool, error) {
	if cv == nil || cv.corpus == nil {
		return false, nil
	}
	var exists bool
	err := cv.corpus.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM documents WHERE collection='aurago_memories' AND pending=0
		AND json_valid(metadata) AND COALESCE(json_extract(metadata,'$.memory_publication_id'),'')<>'')`).Scan(&exists)
	return exists, err
}

func allAutomaticMemoryPublicationDocumentsArchived(ctx context.Context, s *SQLiteMemory, docs []MemoryPublicationDocument) (bool, error) {
	if len(docs) == 0 {
		return false, fmt.Errorf("ready memory publication has no documents")
	}
	for _, doc := range docs {
		var meta MemoryMeta
		if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, doc.ID), &meta); err != nil {
			return false, err
		}
		if !IsMemoryArchived(meta) {
			return false, nil
		}
		var identity string
		if err := s.db.QueryRowContext(ctx, `SELECT identity FROM memory_document_identities WHERE doc_id=?`, doc.ID).Scan(&identity); err != nil {
			return false, err
		}
		if identity == "" {
			return false, fmt.Errorf("archived ready memory %s has no durable identity", doc.ID)
		}
		var hasSource bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_extraction_sources WHERE doc_id=?)`, doc.ID).Scan(&hasSource); err != nil {
			return false, err
		}
		if !hasSource {
			return false, fmt.Errorf("archived ready memory %s has no source provenance", doc.ID)
		}
	}
	return true, nil
}

func validateReadyAutomaticMemoryPublication(ctx context.Context, s *SQLiteMemory, vector VectorDB, docs []MemoryPublicationDocument) error {
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return err
		}
		var meta MemoryMeta
		if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, doc.ID), &meta); err != nil {
			return fmt.Errorf("read ready memory metadata: %w", err)
		}
		archived := IsMemoryArchived(meta)
		var identity string
		if err := s.db.QueryRowContext(ctx, `SELECT identity FROM memory_document_identities WHERE doc_id=?`, doc.ID).Scan(&identity); err != nil {
			return fmt.Errorf("read ready memory identity: %w", err)
		}
		if identity == "" {
			return fmt.Errorf("ready memory %s has no durable identity", doc.ID)
		}
		if identity != "" && !archived {
			var archived bool
			if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_archive_identities WHERE identity=?)`, identity).Scan(&archived); err != nil {
				return fmt.Errorf("recheck ready archived identity: %w", err)
			}
			if archived {
				return fmt.Errorf("ready memory %s matches an archived identity", doc.ID)
			}
		}
		var sessions []string
		rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT session_id FROM memory_extraction_sources WHERE doc_id=?`, doc.ID)
		if err != nil {
			return fmt.Errorf("read ready memory source fences: %w", err)
		}
		for rows.Next() {
			var session string
			if err := rows.Scan(&session); err != nil {
				rows.Close()
				return err
			}
			sessions = append(sessions, session)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return fmt.Errorf("finish ready memory source fences: %w", err)
		}
		if len(sessions) == 0 {
			return fmt.Errorf("ready memory %s has no source provenance", doc.ID)
		}
		for _, session := range sessions {
			deleted, err := s.IsSessionDeleted(session)
			if err != nil {
				return fmt.Errorf("check ready memory source fence: %w", err)
			}
			if deleted {
				return fmt.Errorf("ready memory %s has a deleted source session", doc.ID)
			}
			current, err := memorySourceSessionGenerationCurrent(ctx, s.db, session)
			if err != nil {
				return fmt.Errorf("check ready memory session generation: %w", err)
			}
			if !current {
				return fmt.Errorf("ready memory %s belongs to a superseded source session", doc.ID)
			}
		}
		var content string
		var readErr error
		if reader, ok := vector.(automaticMemoryPublicationDocumentReader); ok {
			content, readErr = reader.ReadAutomaticMemoryPublicationDocumentContext(ctx, doc.ID)
		} else {
			content, readErr = vector.GetByID(doc.ID)
		}
		if readErr != nil {
			return fmt.Errorf("read ready memory source: %w", readErr)
		}
		if contentSHA256(content) != doc.Hash {
			return fmt.Errorf("ready memory %s content changed", doc.ID)
		}
		if HasDurableCredential(content) {
			return fmt.Errorf("ready memory %s violates durable credential policy", doc.ID)
		}
		if corrected, err := s.IsCorrectedMemoryFact(CorrectionFactText(content)); err != nil {
			return fmt.Errorf("recheck ready memory correction: %w", err)
		} else if corrected {
			return fmt.Errorf("ready memory %s was corrected before activation", doc.ID)
		}
	}
	return nil
}

type memoryPublicationRowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func memorySourceSessionGenerationCurrent(ctx context.Context, db memoryPublicationRowQueryer, session string) (bool, error) {
	var hasGenerations bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='chat_session_generations')`).Scan(&hasGenerations); err != nil {
		return false, err
	}
	if !hasGenerations {
		return true, nil
	}
	baseHashBytes := sha256.Sum256([]byte(session))
	baseHash := hex.EncodeToString(baseHashBytes[:])
	var current string
	err := db.QueryRowContext(ctx, `SELECT session_id FROM chat_session_generations WHERE base_hash=?`, baseHash).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil && current != session {
		return false, nil
	}
	if strings.HasPrefix(session, "sess_") {
		// ResolveUserSession uses `sess_` only for renewable fixed-channel
		// generations. Ordinary user-created chats use `sess-` IDs.
		var active bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM chat_session_generations WHERE session_id=?)`, session).Scan(&active); err != nil {
			return false, err
		}
		if !active {
			return false, nil
		}
	}
	return true, nil
}
