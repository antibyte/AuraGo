package memory

import (
	"database/sql"
	"fmt"
	"time"
)

const ftsSchemaVersion = "1"

func (s *SQLiteMemory) ensureMemorySchemaMeta() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("memory schema metadata: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) rebuildFTS5IfNeeded(markerKey, indexTable, sourceTable string) error {
	if err := s.ensureMemorySchemaMeta(); err != nil {
		return err
	}

	var version string
	err := s.db.QueryRow(
		"SELECT value FROM memory_schema_meta WHERE key = ?", markerKey,
	).Scan(&version)
	if err == nil && version == ftsSchemaVersion {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read FTS marker %s: %w", markerKey, err)
	}

	started := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin FTS rebuild %s: %w", indexTable, err)
	}
	defer tx.Rollback()

	var sourceRows int64
	if err := tx.QueryRow("SELECT count(*) FROM " + sourceTable).Scan(&sourceRows); err != nil {
		return fmt.Errorf("count %s rows: %w", sourceTable, err)
	}
	if _, err := tx.Exec(
		"INSERT INTO " + indexTable + "(" + indexTable + ") VALUES('rebuild')",
	); err != nil {
		return fmt.Errorf("rebuild %s: %w", indexTable, err)
	}
	if _, err := tx.Exec(`
		INSERT INTO memory_schema_meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, markerKey, ftsSchemaVersion); err != nil {
		return fmt.Errorf("write FTS marker %s: %w", markerKey, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit FTS rebuild %s: %w", indexTable, err)
	}

	if s.logger != nil {
		s.logger.Info("Rebuilt FTS5 index",
			"index", indexTable,
			"source_rows", sourceRows,
			"duration", time.Since(started))
	}
	return nil
}
