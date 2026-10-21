package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func formatKGTimestamp(value time.Time) string {
	return value.UTC().Format(kgTimestampLayout)
}

func normalizeKGTimestamp(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty timestamp")
	}
	for _, layout := range []string{
		kgTimestampLayout,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		time.RFC3339Nano,
		"2006-01-02",
	} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return formatKGTimestamp(parsed), nil
		}
	}
	return "", fmt.Errorf("unsupported timestamp format %q", raw)
}

// checkKGSchemaVersion is deliberately called before initTables performs any DDL.
// The KG database is separate from STM and owns its own SQLite user_version.
func (kg *KnowledgeGraph) checkKGSchemaVersion() error {
	if kg == nil || kg.db == nil {
		return fmt.Errorf("knowledge graph database is not initialized")
	}
	var version int
	if err := kg.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read KG user_version: %w", err)
	}
	if version > kgSchemaVersion {
		return fmt.Errorf("KG database schema version %d is newer than supported version %d", version, kgSchemaVersion)
	}

	var hasMeta bool
	if err := kg.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='kg_meta')`).Scan(&hasMeta); err != nil {
		return fmt.Errorf("inspect KG metadata schema version: %w", err)
	}
	if hasMeta {
		var raw sql.NullString
		err := kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read KG timestamp migration version: %w", err)
		}
		if raw.Valid && strings.TrimSpace(raw.String) != "" {
			markerVersion, parseErr := strconv.Atoi(raw.String)
			if parseErr != nil {
				return fmt.Errorf("parse KG timestamp migration version %q: %w", raw.String, parseErr)
			}
			if markerVersion > kgTimestampMigrationVersion {
				return fmt.Errorf("KG timestamp migration version %d is newer than supported version %d", markerVersion, kgTimestampMigrationVersion)
			}
		}
	}
	if version == kgSchemaVersion {
		if err := kg.validateCurrentKGSchema(); err != nil {
			return err
		}
	}
	return nil
}

func (kg *KnowledgeGraph) migrateKGTimestamps() error {
	var raw sql.NullString
	err := kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read KG timestamp migration marker: %w", err)
	}
	markerVersion := 0
	if raw.Valid && strings.TrimSpace(raw.String) != "" {
		markerVersion, err = strconv.Atoi(raw.String)
		if err != nil {
			return fmt.Errorf("parse KG timestamp migration marker %q: %w", raw.String, err)
		}
	}
	if markerVersion > kgTimestampMigrationVersion {
		return fmt.Errorf("KG timestamp migration version %d is newer than supported version %d", markerVersion, kgTimestampMigrationVersion)
	}
	var userVersion int
	if err := kg.db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return fmt.Errorf("read KG schema version before timestamp normalization: %w", err)
	}
	if markerVersion == kgTimestampMigrationVersion && userVersion == kgSchemaVersion {
		return nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin KG timestamp normalization: %w", err)
	}
	defer tx.Rollback()

	// Re-read the marker under the write transaction so concurrent startup cannot
	// normalize with a stale version decision.
	raw = sql.NullString{}
	err = tx.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("re-read KG timestamp migration marker: %w", err)
	}
	markerVersion = 0
	if raw.Valid && strings.TrimSpace(raw.String) != "" {
		markerVersion, err = strconv.Atoi(raw.String)
		if err != nil {
			return fmt.Errorf("parse KG timestamp migration marker %q: %w", raw.String, err)
		}
	}
	if markerVersion > kgTimestampMigrationVersion {
		return fmt.Errorf("KG timestamp migration version %d is newer than supported version %d", markerVersion, kgTimestampMigrationVersion)
	}
	if markerVersion < kgTimestampMigrationVersion {
		for _, table := range []string{"kg_nodes", "kg_edges"} {
			if err := normalizeKGTimestampColumns(tx, table); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO kg_meta(key,value) VALUES(?,?)`, kgTimestampMigrationKey, strconv.Itoa(kgTimestampMigrationVersion)); err != nil {
			return fmt.Errorf("write KG timestamp migration marker: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, kgSchemaVersion)); err != nil {
		return fmt.Errorf("write KG user_version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit KG timestamp normalization: %w", err)
	}
	return nil
}

func normalizeKGTimestampColumns(tx *sql.Tx, table string) error {
	if table != "kg_nodes" && table != "kg_edges" {
		return fmt.Errorf("refusing to normalize timestamps in unexpected KG table %q", table)
	}
	rows, err := tx.Query(`SELECT rowid, updated_at, COALESCE(NULLIF(updated_at,''),created_at,CURRENT_TIMESTAMP), semantic_indexed_at FROM ` + table)
	if err != nil {
		return fmt.Errorf("read %s timestamps for normalization: %w", table, err)
	}
	type timestampUpdate struct {
		rowid          int64
		updatedAt      string
		semanticAt     sql.NullString
		updatedChanged bool
		semanticValue  sql.NullString
		semanticChange bool
	}
	var updates []timestampUpdate
	for rows.Next() {
		var row timestampUpdate
		var oldUpdated sql.NullString
		var timestampSource string
		if err := rows.Scan(&row.rowid, &oldUpdated, &timestampSource, &row.semanticAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan %s timestamps for normalization: %w", table, err)
		}
		updated, err := normalizeKGTimestamp(timestampSource)
		if err != nil {
			rows.Close()
			return fmt.Errorf("normalize %s.updated_at at row %d: %w", table, row.rowid, err)
		}
		row.updatedAt = updated
		row.updatedChanged = !oldUpdated.Valid || strings.TrimSpace(oldUpdated.String) == "" || oldUpdated.String != updated
		row.semanticValue = row.semanticAt
		if row.semanticAt.Valid && strings.TrimSpace(row.semanticAt.String) != "" {
			semantic, err := normalizeKGTimestamp(row.semanticAt.String)
			if err != nil {
				rows.Close()
				return fmt.Errorf("normalize %s.semantic_indexed_at at row %d: %w", table, row.rowid, err)
			}
			row.semanticValue = sql.NullString{String: semantic, Valid: true}
			row.semanticChange = row.semanticAt.String != semantic
		} else if row.semanticAt.Valid {
			row.semanticValue = sql.NullString{}
			row.semanticChange = true
		}
		if row.updatedChanged || row.semanticChange {
			updates = append(updates, row)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate %s timestamps for normalization: %w", table, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close %s timestamp rows: %w", table, err)
	}
	stmt, err := tx.Prepare(`UPDATE ` + table + ` SET updated_at=?, semantic_indexed_at=? WHERE rowid=?`)
	if err != nil {
		return fmt.Errorf("prepare %s timestamp normalization: %w", table, err)
	}
	defer stmt.Close()
	for _, row := range updates {
		if _, err := stmt.Exec(row.updatedAt, nullableString(row.semanticValue.String), row.rowid); err != nil {
			return fmt.Errorf("write normalized %s timestamps at row %d: %w", table, row.rowid, err)
		}
	}
	return nil
}
