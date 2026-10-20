package memory

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateMemoryArchives(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memory_document_identities (
			doc_id TEXT PRIMARY KEY, identity TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS memory_archive_identities (
			doc_id TEXT PRIMARY KEY, identity TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_memory_archive_identity ON memory_archive_identities(identity);
		CREATE TABLE IF NOT EXISTS memory_archive_cleanup (
			doc_id TEXT PRIMARY KEY, expected_hash TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`)
	if err != nil {
		return fmt.Errorf("create memory archive identity and cleanup schema: %w", err)
	}
	return nil
}
