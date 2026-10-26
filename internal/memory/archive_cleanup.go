package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type memoryArchiveCandidate struct {
	meta     MemoryMeta
	identity string
	hash     string
}

// ApplyMemoryBudgetEnforcementContext first commits the actual archive decisions
// and their cleanup receipts. No vector deletion is authorized by a stale preview.
func (s *SQLiteMemory) ApplyMemoryBudgetEnforcementContext(ctx context.Context, budget int, vector VectorDB) (int, error) {
	if vector == nil {
		return 0, nil
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return 0, err
	}
	defer s.automaticMemoryMu.Unlock()
	s.bindArchiveVector(vector)
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	if err := s.cleanupMemoryArchives(ctx, vector, 500); err != nil {
		return 0, err
	}
	if budget <= 0 {
		return 0, nil
	}
	ids, err := s.EnforceMemoryBudgetContext(ctx, budget)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	// Limit each transaction and vector publication even for a large overage.
	if len(ids) > 500 {
		ids = ids[:500]
	}
	var candidates []memoryArchiveCandidate
	for _, id := range ids {
		var candidate memoryArchiveCandidate
		if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, id), &candidate.meta); err != nil {
			return 0, err
		}
		candidate.identity, candidate.hash, err = s.archiveDocumentIdentity(ctx, vector, id)
		if err != nil {
			return 0, err
		}
		if candidate.identity == "" {
			// Missing source content cannot grant authority to delete a future
			// document that happens to reuse the same identifier.
			continue
		}
		candidates = append(candidates, candidate)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	applied := 0
	for _, candidate := range candidates {
		var current MemoryMeta
		err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, candidate.meta.DocID), &current)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if current.Protected || current.KeepForever || IsMemoryArchived(current) ||
			current.VerificationStatus == MemoryVerificationContradicted || !memoryMetaEqual(current, candidate.meta) {
			continue
		}
		if err := archiveMemoryCandidate(ctx, tx, candidate, "memory_meta budget enforcement", "system", true); err != nil {
			return 0, err
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit memory budget archive: %w", err)
	}
	if err := s.cleanupMemoryArchives(ctx, vector, 500); err != nil {
		return 0, err
	}
	return applied, nil
}

func archiveMemoryCandidate(ctx context.Context, tx *sql.Tx, candidate memoryArchiveCandidate, reason, actor string, cleanup bool) error {
	id := candidate.meta.DocID
	if candidate.identity != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO memory_archive_identities(doc_id,identity) VALUES(?,?)
			ON CONFLICT(doc_id) DO UPDATE SET identity=excluded.identity`, id, candidate.identity); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO memory_archive_identities(doc_id,identity)
		SELECT doc_id,identity FROM memory_document_identities WHERE doc_id=? ON CONFLICT(doc_id) DO NOTHING`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memory_meta SET verification_status='archived',
		archived_at=CURRENT_TIMESTAMP, archived_reason=?,last_reviewed_at=CURRENT_TIMESTAMP,
		review_note=?,last_event_at=CURRENT_TIMESTAMP WHERE doc_id=?`, reason, reason, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_curation_events(doc_id,action,actor,previous_status,new_status,reason,dry_run)
		VALUES(?,'archive',?,?,'archived',?,0)`, id, actor, normalizeMemoryVerificationStatus(candidate.meta.VerificationStatus), reason); err != nil {
		return err
	}
	if cleanup {
		if _, err := tx.ExecContext(ctx, `INSERT INTO memory_archive_cleanup(doc_id,expected_hash) VALUES(?,?)
			ON CONFLICT(doc_id) DO UPDATE SET expected_hash=excluded.expected_hash`, id, candidate.hash); err != nil {
			return err
		}
	}
	return nil
}

// cleanupMemoryArchives runs under automaticMemoryMu and the publication lease.
// A failed deletion leaves its receipt durable for the next run, including when
// the process exits after deleting the vector but before acknowledging cleanup.
func (s *SQLiteMemory) cleanupMemoryArchives(ctx context.Context, vector VectorDB, limit int) error {
	rows, err := s.db.QueryContext(ctx, `SELECT doc_id,expected_hash FROM memory_archive_cleanup ORDER BY doc_id LIMIT ?`, limit)
	if err != nil {
		return err
	}
	type cleanup struct{ id, hash string }
	var jobs []cleanup
	for rows.Next() {
		var job cleanup
		if err := rows.Scan(&job.id, &job.hash); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, job)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var completed, cancelled []string
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		var allowed bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_meta WHERE doc_id=?
			AND (verification_status='archived' OR COALESCE(archived_at,'')<>'') AND protected=0 AND keep_forever=0)`, job.id).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			cancelled = append(cancelled, job.id)
			continue
		}
		_, hash, err := archiveDocumentIdentity(ctx, vector, job.id)
		if err != nil {
			return err
		}
		if hash != "" {
			if hash != job.hash {
				return fmt.Errorf("retain changed archived vector %s for reconciliation", job.id)
			}
			var deleted bool
			if native, ok := vector.(*ChromemVectorDB); ok {
				deleted, err = native.DeleteDocumentIfContentMatchesContext(ctx, job.id, job.hash)
			} else if owned, ok := vector.(OwnershipAwareVectorDB); ok {
				deleted, err = owned.DeleteDocumentIfContentMatches(job.id, job.hash)
			} else {
				return fmt.Errorf("retain archived vector %s: conditional deletion is unavailable", job.id)
			}
			if err != nil {
				return fmt.Errorf("clean archived vector: %w", err)
			}
			if !deleted {
				return fmt.Errorf("archived vector changed during cleanup")
			}
		} else if native, ok := vector.(*ChromemVectorDB); ok {
			if _, err := native.DeleteDocumentIfContentMatchesContext(ctx, job.id, job.hash); err != nil {
				return fmt.Errorf("clean orphan archived index entry: %w", err)
			}
		}
		completed = append(completed, job.id)
	}
	if len(completed)+len(cancelled) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := cleanupDeletedVectorReferencesTx(ctx, tx, completed); err != nil {
		return err
	}
	// The shared writer/publication locks keep receipts unchanged until this
	// acknowledgement. Reference cleanup and queue removal commit together.
	if err := deleteMemoryIDsContext(ctx, tx, "memory_archive_cleanup", "doc_id", append(completed, cancelled...)); err != nil {
		return err
	}
	return tx.Commit()
}
