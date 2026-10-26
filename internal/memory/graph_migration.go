package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

var currentKGRequiredTables = []string{
	"kg_nodes",
	"kg_edges",
	"kg_evidence",
	"kg_claims",
	"kg_conflicts",
	"kg_meta",
	"kg_correction_receipts",
	"kg_correction_guards",
	"kg_correction_field_revisions",
}

func (kg *KnowledgeGraph) validateCurrentKGSchema() error {
	for _, table := range currentKGRequiredTables {
		var exists bool
		if err := kg.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect current KG table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("KG database claims current schema version %d but required table %s is missing", kgSchemaVersion, table)
		}
	}
	for _, column := range []struct{ table, name string }{
		{"kg_nodes", "node_type"}, {"kg_nodes", "source_type"},
		{"kg_nodes", "semantic_indexed_at"}, {"kg_edges", "status"},
		{"kg_edges", "status_reason"}, {"kg_edges", "superseded_by_claim_id"},
		{"kg_edges", "retracted_at"}, {"kg_edges", "semantic_indexed_at"},
	} {
		exists, err := kg.hasKGColumn(column.table, column.name)
		if err != nil {
			return fmt.Errorf("inspect current KG column %s.%s: %w", column.table, column.name, err)
		}
		if !exists {
			return fmt.Errorf("KG database claims current schema version %d but required column %s.%s is missing", kgSchemaVersion, column.table, column.name)
		}
	}
	hasCascade, err := kg.kgEdgesHaveCascadeFKs()
	if err != nil {
		return fmt.Errorf("inspect current KG cascade foreign keys: %w", err)
	}
	if !hasCascade {
		return fmt.Errorf("KG database claims current schema version %d but kg_edges cascade foreign keys are missing", kgSchemaVersion)
	}
	return nil
}

func (kg *KnowledgeGraph) backupBeforeKGMigration() error {
	var tableCount int
	if err := kg.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name GLOB 'kg_*'`).Scan(&tableCount); err != nil {
		return fmt.Errorf("inspect KG tables before migration: %w", err)
	}
	if tableCount == 0 {
		return nil
	}
	var userVersion int
	if err := kg.db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return fmt.Errorf("read KG version before migration backup: %w", err)
	}
	needsMigration := userVersion < kgSchemaVersion
	var hasMeta bool
	if err := kg.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='kg_meta')`).Scan(&hasMeta); err != nil {
		return fmt.Errorf("inspect KG metadata before migration backup: %w", err)
	}
	timestampMigration := !hasMeta
	if hasMeta {
		var raw sql.NullString
		err := kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key=?`, kgTimestampMigrationKey).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read KG timestamp marker before migration backup: %w", err)
		}
		version := 0
		if raw.Valid && raw.String != "" {
			version, err = strconv.Atoi(raw.String)
			if err != nil {
				return fmt.Errorf("parse KG timestamp marker before migration backup %q: %w", raw.String, err)
			}
		}
		timestampMigration = version < kgTimestampMigrationVersion
	}
	needsMigration = needsMigration || timestampMigration
	backupLabel := "graph-schema-v2"
	if timestampMigration {
		backupLabel = "kg-utc-timestamps-v1"
	}
	var storedFTS sql.NullString
	if hasMeta {
		ftsErr := kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key='fts_schema_version'`).Scan(&storedFTS)
		if ftsErr != nil && !errors.Is(ftsErr, sql.ErrNoRows) {
			return fmt.Errorf("read KG FTS version before migration backup: %w", ftsErr)
		}
	}
	needsMigration = needsMigration || storedFTS.String != kgFTSSchemaVersion

	for _, table := range currentKGRequiredTables {
		var exists bool
		if err := kg.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect KG table %s before migration backup: %w", table, err)
		}
		if !exists {
			needsMigration = true
		}
	}
	for _, column := range []struct{ table, name string }{
		{"kg_nodes", "node_type"}, {"kg_nodes", "source_type"},
		{"kg_nodes", "semantic_indexed_at"}, {"kg_edges", "updated_at"},
		{"kg_edges", "status"}, {"kg_edges", "status_reason"},
		{"kg_edges", "superseded_by_claim_id"}, {"kg_edges", "retracted_at"},
		{"kg_edges", "semantic_indexed_at"},
	} {
		exists, err := kg.hasKGColumn(column.table, column.name)
		if err != nil {
			return fmt.Errorf("inspect KG column %s.%s before migration backup: %w", column.table, column.name, err)
		}
		if !exists {
			needsMigration = true
		}
	}
	hasCascade, err := kg.kgEdgesHaveCascadeFKs()
	if err != nil {
		return fmt.Errorf("inspect KG cascade keys before migration backup: %w", err)
	}
	if !hasCascade {
		needsMigration = true
	}
	if !needsMigration {
		return nil
	}
	if _, err := backupMainDatabase(kg.db, backupLabel); err != nil {
		return fmt.Errorf("create consistent KG pre-migration backup: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) hasKGColumn(table, column string) (bool, error) {
	if !validIdentifier(table) || !validIdentifier(column) {
		return false, fmt.Errorf("unsafe KG schema identifier")
	}
	var exists bool
	err := kg.db.QueryRow(fmt.Sprintf("SELECT count(*)>0 FROM pragma_table_xinfo('%s') WHERE name=?", table), column).Scan(&exists)
	return exists, err
}
