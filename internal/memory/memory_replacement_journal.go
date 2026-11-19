package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const memoryReplacementPublicationPrefix = automaticMemoryPublicationPrefix + "replacement."

type memoryReplacementJournal struct {
	automaticMemoryPublicationJournal
	OldID           string `json:"old_id"`
	OldHash         string `json:"old_hash"`
	ReplacementHash string `json:"replacement_hash"`
}

type memoryReplacementTransition struct {
	key, staged, ready string
}

func memoryReplacementKey(oldID string) string {
	return memoryReplacementPublicationPrefix + contentSHA256(oldID)
}

func (j memoryReplacementJournal) documentIDs() []string {
	ids := make([]string, 0, len(j.Documents))
	for _, document := range j.Documents {
		ids = append(ids, document.ID)
	}
	return ids
}

// replaceMemoryWithJournal holds automaticMemoryMu. Preparation can wait for
// embeddings before taking the snapshot lease.
func (s *SQLiteMemory) replaceMemoryWithJournal(ctx context.Context, vector *ChromemVectorDB, oldHash, concept, replacement string, expected MemoryMeta, reason, actor string, validate func() error) ([]string, error) {
	prepared, err := vector.prepareMemoryWriteContext(ctx, concept, replacement, VectorStoreForceCreate)
	if err != nil {
		return nil, err
	}
	defer prepared.Release()
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := validate(); err != nil {
		return nil, err
	}

	journal := memoryReplacementJournal{
		automaticMemoryPublicationJournal: automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationStaged},
		OldID:                             expected.DocID, OldHash: oldHash, ReplacementHash: contentSHA256(buildContentString(concept, replacement)),
	}
	for _, doc := range prepared.Documents {
		journal.Documents = append(journal.Documents, MemoryPublicationDocument{ID: doc.ID, Hash: contentSHA256(doc.Content)})
	}
	staged, err := jsonString(journal)
	if err != nil {
		return nil, err
	}
	journal.Status = automaticMemoryPublicationReady
	ready, err := jsonString(journal)
	if err != nil {
		return nil, err
	}
	key := memoryReplacementKey(expected.DocID)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, key, staged); err != nil {
		return nil, fmt.Errorf("reserve replacement publication: %w", err)
	}
	rollback := func(cause error) error {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		err := cleanupStagedAutomaticMemoryPublication(cleanupCtx, s, vector, key, staged, journal.Documents, nil)
		if err != nil {
			return errors.Join(cause, ErrAutomaticMemoryPublicationPending, err)
		}
		return cause
	}
	if err := vector.StageAutomaticMemoryWriteContext(ctx, key, prepared.Documents); err != nil {
		return nil, rollback(err)
	}
	if err := validate(); err != nil {
		return nil, rollback(err)
	}
	ids := prepared.Result.CreatedIDs
	if err := s.archiveAndCopyMemoryMeta(ctx, expected, ids, reason, actor, &memoryReplacementTransition{key: key, staged: staged, ready: ready}); err != nil {
		var commitErr *memoryReplacementMetadataCommitError
		if errors.As(err, &commitErr) {
			return ids, err
		}
		return nil, rollback(err)
	}
	if err := s.finishMemoryReplacement(ctx, vector, key, ready, journal); err != nil {
		return ids, errors.Join(ErrAutomaticMemoryPublicationPending, err)
	}
	return ids, nil
}

func (s *SQLiteMemory) resumeMemoryReplacement(ctx context.Context, vector *ChromemVectorDB, key, raw string, validate func() error) (memoryReplacementJournal, error) {
	var journal memoryReplacementJournal
	if err := json.Unmarshal([]byte(raw), &journal); err != nil {
		return journal, fmt.Errorf("decode memory replacement journal: %w", err)
	}
	if journal.Version != 1 || journal.OldID == "" || key != memoryReplacementKey(journal.OldID) || journal.OldHash == "" || journal.ReplacementHash == "" || len(journal.Documents) == 0 {
		return journal, fmt.Errorf("invalid memory replacement journal %s", key)
	}
	for _, doc := range journal.Documents {
		if doc.ID == "" || doc.ID == journal.OldID || doc.Hash == "" {
			return journal, fmt.Errorf("invalid memory replacement document receipt")
		}
	}
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return journal, err
	}
	defer release()
	switch journal.Status {
	case automaticMemoryPublicationStaged:
		err = cleanupStagedAutomaticMemoryPublication(ctx, s, vector, key, raw, journal.Documents, nil)
	case automaticMemoryPublicationReady:
		if validate != nil {
			if err := validate(); err != nil {
				return journal, err
			}
		}
		err = s.finishMemoryReplacement(ctx, vector, key, raw, journal)
	default:
		err = fmt.Errorf("unknown memory replacement publication state")
	}
	return journal, err
}

func (s *SQLiteMemory) finishMemoryReplacement(ctx context.Context, vector *ChromemVectorDB, key, raw string, journal memoryReplacementJournal) error {
	if err := vector.ActivateAutomaticMemoryWriteContext(ctx, key, journal.Documents); err != nil {
		return fmt.Errorf("activate committed memory replacement: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE memory_meta SET doc_id=doc_id WHERE doc_id=?`, journal.OldID); err != nil {
		return err
	}
	var old MemoryMeta
	if err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, journal.OldID), &old); err != nil {
		return fmt.Errorf("read committed memory replacement source: %w", err)
	}
	if !IsMemoryArchived(old) || old.Protected || old.KeepForever {
		return fmt.Errorf("retain changed replacement source %s", journal.OldID)
	}
	for _, document := range journal.Documents {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta WHERE doc_id=?)`, document.ID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("replacement metadata %s is missing", document.ID)
		}
	}
	deleted, err := vector.DeleteDocumentIfContentMatchesContext(ctx, journal.OldID, journal.OldHash)
	if err != nil {
		return fmt.Errorf("retire committed memory replacement source: %w", err)
	}
	if !deleted {
		return fmt.Errorf("retain changed replacement source %s", journal.OldID)
	}
	if err := cleanupDeletedVectorReferencesTx(ctx, tx, []string{journal.OldID}); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, key, raw)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return errors.Join(fmt.Errorf("memory replacement journal changed before completion"), err)
	}
	return tx.Commit()
}
