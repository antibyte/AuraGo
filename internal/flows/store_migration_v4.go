package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const libraryFoldersSchema = `CREATE TABLE flow_library_folders (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE COLLATE BINARY,
	revision INTEGER NOT NULL CHECK(revision > 0),
	updated_at TEXT NOT NULL
)`

const libraryMetadataSchema = `CREATE TABLE flow_library_metadata (
	flow_id TEXT PRIMARY KEY REFERENCES flows(id) ON DELETE CASCADE,
	folder_id TEXT REFERENCES flow_library_folders(id) ON DELETE RESTRICT,
	tags_json TEXT NOT NULL,
	revision INTEGER NOT NULL CHECK(revision > 0)
)`

const versionNotesSchema = `CREATE TABLE flow_version_notes (
	flow_id TEXT NOT NULL,
	revision INTEGER NOT NULL,
	note TEXT NOT NULL,
	note_revision INTEGER NOT NULL CHECK(note_revision > 0),
	PRIMARY KEY(flow_id,revision),
	FOREIGN KEY(flow_id,revision) REFERENCES flow_versions(flow_id,revision) ON DELETE CASCADE
)`

const libraryFolderIndex = `CREATE INDEX idx_flow_library_folder ON flow_library_metadata(folder_id,flow_id)`

func verifyFlowsV4(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 4 {
		return err
	}
	for _, item := range []struct{ kind, name, definition string }{
		{"table", "flow_library_folders", libraryFoldersSchema},
		{"table", "flow_library_metadata", libraryMetadataSchema},
		{"table", "flow_version_notes", versionNotesSchema},
		{"index", "idx_flow_library_folder", libraryFolderIndex},
	} {
		var definition string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type=? AND name=?`, item.kind, item.name).Scan(&definition); err != nil {
			return fmt.Errorf("verify flow library schema: %w", err)
		}
		canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
		if canonical(definition) != canonical(item.definition) {
			return errors.New("invalid flow library schema")
		}
	}
	return nil
}

func migrateFlowsV4(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("read flows schema: %w", err)
	}
	if version >= 4 {
		return verifyFlowsV4(db)
	}
	if version != 3 {
		return errors.New("flows v4 migration requires schema v3")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v3-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin flows v4 migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{libraryFoldersSchema, libraryMetadataSchema, versionNotesSchema, libraryFolderIndex} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate flows v4: %w", err)
		}
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='4' WHERE key='schema_version' AND value='3'`)
	if err != nil {
		return fmt.Errorf("record flows v4 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v4 schema: version changed")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit flows v4 schema: %w", err)
	}
	return nil
}
