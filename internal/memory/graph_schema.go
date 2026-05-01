package memory

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

const kgFTSSchemaVersion = "v1"

func (kg *KnowledgeGraph) initTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS kg_nodes (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT NOT NULL UNIQUE,
		label TEXT NOT NULL DEFAULT '',
		properties TEXT NOT NULL DEFAULT '{}',
		access_count INTEGER NOT NULL DEFAULT 0,
		protected INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		semantic_indexed_at DATETIME,
		node_type TEXT GENERATED ALWAYS AS (json_extract(properties, '$.type')) STORED,
		source_type TEXT GENERATED ALWAYS AS (json_extract(properties, '$.source')) STORED
	);

	CREATE TABLE IF NOT EXISTS kg_edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		relation TEXT NOT NULL,
		properties TEXT NOT NULL DEFAULT '{}',
		access_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		semantic_indexed_at DATETIME,
		UNIQUE(source, target, relation)
	);

	CREATE INDEX IF NOT EXISTS idx_kg_edges_source ON kg_edges(source);
	CREATE INDEX IF NOT EXISTS idx_kg_edges_target ON kg_edges(target);
	CREATE TABLE IF NOT EXISTS kg_meta (key TEXT PRIMARY KEY, value TEXT);
	`
	if _, err := kg.db.Exec(schema); err != nil {
		return fmt.Errorf("create kg tables: %w", err)
	}

	type colMigration struct {
		table, column, def string
	}
	colMigrations := []colMigration{
		{"kg_nodes", "semantic_indexed_at", "DATETIME"},
		{"kg_edges", "semantic_indexed_at", "DATETIME"},
		{"kg_edges", "updated_at", "DATETIME"},
	}
	for _, cm := range colMigrations {
		if !validIdentifier(cm.table) || !validIdentifier(cm.column) {
			kg.logger.Warn("KG migration: skipping unsafe identifier", "table", cm.table, "column", cm.column)
			continue
		}
		var exists bool
		kg.db.QueryRow(fmt.Sprintf("SELECT count(*)>0 FROM pragma_table_info('%s') WHERE name=?", cm.table), cm.column).Scan(&exists)
		if exists {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", cm.table, cm.column, cm.def)
		if _, err := kg.db.Exec(stmt); err != nil {
			kg.logger.Warn("KG migration: add column failed", "table", cm.table, "column", cm.column, "error", err)
		} else {
			kg.logger.Info("KG migration: added column", "table", cm.table, "column", cm.column)
		}
	}
	if _, err := kg.db.Exec("UPDATE kg_edges SET updated_at = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP) WHERE updated_at IS NULL OR updated_at = ''"); err != nil {
		kg.logger.Warn("KG migration: backfill kg_edges.updated_at failed", "error", err)
	}

	var hasNodeType bool
	kg.db.QueryRow("SELECT count(*)>0 FROM pragma_table_info('kg_nodes') WHERE name='node_type'").Scan(&hasNodeType)
	if !hasNodeType {
		kg.logger.Info("KG migration: rebuilding kg_nodes to add generated columns")
		rebuild := []string{
			`ALTER TABLE kg_nodes RENAME TO kg_nodes_old`,
			`CREATE TABLE kg_nodes (
				rowid INTEGER PRIMARY KEY AUTOINCREMENT,
				id TEXT NOT NULL UNIQUE,
				label TEXT NOT NULL DEFAULT '',
				properties TEXT NOT NULL DEFAULT '{}',
				access_count INTEGER NOT NULL DEFAULT 0,
				protected INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				semantic_indexed_at DATETIME,
				node_type TEXT GENERATED ALWAYS AS (json_extract(properties, '$.type')) STORED,
				source_type TEXT GENERATED ALWAYS AS (json_extract(properties, '$.source')) STORED
			)`,
			`INSERT INTO kg_nodes (rowid, id, label, properties, access_count, protected, created_at, updated_at, semantic_indexed_at)
				SELECT rowid, id, label, properties, access_count, protected, created_at, updated_at, semantic_indexed_at
				FROM kg_nodes_old`,
			`DROP TABLE kg_nodes_old`,
		}
		for _, stmt := range rebuild {
			if _, err := kg.db.Exec(stmt); err != nil {
				return fmt.Errorf("KG migration rebuild kg_nodes: %w", err)
			}
		}
		kg.logger.Info("KG migration: kg_nodes rebuilt with generated columns")
	}

	idxStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_kg_nodes_type ON kg_nodes(node_type)`,
		`CREATE INDEX IF NOT EXISTS idx_kg_nodes_source ON kg_nodes(source_type)`,
	}
	for _, stmt := range idxStmts {
		if _, err := kg.db.Exec(stmt); err != nil {
			kg.logger.Warn("KG migration: index creation failed", "error", err, "stmt", stmt)
		}
	}

	var storedFTSVersion string
	kg.db.QueryRow(`SELECT value FROM kg_meta WHERE key = 'fts_schema_version'`).Scan(&storedFTSVersion)
	if storedFTSVersion != kgFTSSchemaVersion {
		if err := kg.rebuildFTSIndexes(); err != nil {
			return err
		}
		kg.db.Exec(`INSERT OR REPLACE INTO kg_meta (key, value) VALUES ('fts_schema_version', ?)`, kgFTSSchemaVersion)
	}

	return nil
}

func (kg *KnowledgeGraph) rebuildFTSIndexes() error {
	dropStatements := []string{
		`DROP TRIGGER IF EXISTS kg_nodes_ai;`,
		`DROP TRIGGER IF EXISTS kg_nodes_ad;`,
		`DROP TRIGGER IF EXISTS kg_nodes_au;`,
		`DROP TRIGGER IF EXISTS kg_edges_ai;`,
		`DROP TRIGGER IF EXISTS kg_edges_ad;`,
		`DROP TRIGGER IF EXISTS kg_edges_au;`,
		`DROP TABLE IF EXISTS kg_nodes_fts;`,
		`DROP TABLE IF EXISTS kg_edges_fts;`,
	}
	for _, stmt := range dropStatements {
		if _, err := kg.db.Exec(stmt); err != nil {
			return fmt.Errorf("drop kg FTS artifact: %w", err)
		}
	}

	createStatements := []string{
		`CREATE VIRTUAL TABLE kg_nodes_fts USING fts5(
			id, label, properties, content=kg_nodes, content_rowid=rowid
		);`,
		`CREATE VIRTUAL TABLE kg_edges_fts USING fts5(
			source, target, relation, properties, content=kg_edges, content_rowid=id
		);`,
		`CREATE TRIGGER kg_nodes_ai AFTER INSERT ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(rowid, id, label, properties)
			VALUES (new.rowid, new.id, new.label, new.properties);
		END;`,
		`CREATE TRIGGER kg_nodes_ad AFTER DELETE ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(kg_nodes_fts, rowid, id, label, properties)
			VALUES ('delete', old.rowid, old.id, old.label, old.properties);
		END;`,
		`CREATE TRIGGER kg_nodes_au AFTER UPDATE ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(kg_nodes_fts, rowid, id, label, properties)
			VALUES ('delete', old.rowid, old.id, old.label, old.properties);
			INSERT INTO kg_nodes_fts(rowid, id, label, properties)
			VALUES (new.rowid, new.id, new.label, new.properties);
		END;`,
		`CREATE TRIGGER kg_edges_ai AFTER INSERT ON kg_edges BEGIN
			INSERT INTO kg_edges_fts(rowid, source, target, relation, properties)
			VALUES (new.id, new.source, new.target, new.relation, new.properties);
		END;`,
		`CREATE TRIGGER kg_edges_ad AFTER DELETE ON kg_edges BEGIN
			INSERT INTO kg_edges_fts(kg_edges_fts, rowid, source, target, relation, properties)
			VALUES ('delete', old.id, old.source, old.target, old.relation, old.properties);
		END;`,
		`CREATE TRIGGER kg_edges_au AFTER UPDATE ON kg_edges BEGIN
			INSERT INTO kg_edges_fts(kg_edges_fts, rowid, source, target, relation, properties)
			VALUES ('delete', old.id, old.source, old.target, old.relation, old.properties);
			INSERT INTO kg_edges_fts(rowid, source, target, relation, properties)
			VALUES (new.id, new.source, new.target, new.relation, new.properties);
		END;`,
	}
	for _, stmt := range createStatements {
		if _, err := kg.db.Exec(stmt); err != nil {
			return fmt.Errorf("create kg FTS artifact: %w", err)
		}
	}

	rebuildStatements := []string{
		`INSERT INTO kg_nodes_fts(kg_nodes_fts) VALUES ('rebuild');`,
		`INSERT INTO kg_edges_fts(kg_edges_fts) VALUES ('rebuild');`,
	}
	for _, stmt := range rebuildStatements {
		if _, err := kg.db.Exec(stmt); err != nil {
			return fmt.Errorf("rebuild kg FTS index: %w", err)
		}
	}

	return nil
}

func (kg *KnowledgeGraph) migrateFromJSON(jsonPath string) {
	f, err := os.Open(jsonPath)
	if err != nil {
		return
	}

	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		f.Close()
		return
	}
	f.Close()

	var state struct {
		Nodes map[string]Node `json:"nodes"`
		Edges []Edge          `json:"edges"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		kg.logger.Warn("[KG] Failed to parse graph.json for migration", "error", err)
		return
	}

	tx, err := kg.db.Begin()
	if err != nil {
		kg.logger.Error("[KG] Failed to begin migration transaction", "error", err)
		return
	}
	defer tx.Rollback()

	migrated := 0
	for _, n := range state.Nodes {
		if n.ID == "" {
			continue
		}
		propsJSON, err := json.Marshal(n.Properties)
		if err != nil {
			kg.logger.Warn("[KG] Failed to marshal node properties during migration", "node_id", n.ID, "error", err)
			continue
		}
		accessCount := 0
		if countStr, ok := n.Properties["access_count"]; ok {
			if parsed, err := strconv.Atoi(countStr); err == nil {
				accessCount = parsed
			}
		}
		isProtected := 0
		if n.Properties["protected"] == "true" {
			isProtected = 1
		}
		_, err = tx.Exec(
			"INSERT OR IGNORE INTO kg_nodes (id, label, properties, access_count, protected) VALUES (?, ?, ?, ?, ?)",
			n.ID, n.Label, string(propsJSON), accessCount, isProtected,
		)
		if err == nil {
			migrated++
		}
	}
	edgeMigrated := 0
	for _, e := range state.Edges {
		propsJSON, err := json.Marshal(e.Properties)
		if err != nil {
			kg.logger.Warn("[KG] Failed to marshal edge properties during migration", "source", e.Source, "target", e.Target, "error", err)
			continue
		}
		if _, err := tx.Exec(
			"INSERT OR IGNORE INTO kg_edges (source, target, relation, properties) VALUES (?, ?, ?, ?)",
			e.Source, e.Target, e.Relation, string(propsJSON),
		); err != nil {
			kg.logger.Warn("[KG] Failed to migrate edge", "source", e.Source, "target", e.Target, "error", err)
		} else {
			edgeMigrated++
		}
	}

	if err := tx.Commit(); err != nil {
		kg.logger.Error("[KG] Migration commit failed", "error", err)
		return
	}

	if renameErr := os.Rename(jsonPath, jsonPath+".migrated"); renameErr != nil {
		kg.logger.Warn("[KG] Could not rename migrated JSON file", "error", renameErr)
	}
	kg.logger.Info("[KG] Migrated graph.json to SQLite", "nodes", migrated, "edges", edgeMigrated)
}
