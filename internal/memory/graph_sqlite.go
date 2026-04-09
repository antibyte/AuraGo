package memory

import (
	"aurago/internal/dbutil"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// validIdentifierRE matches valid SQLite identifiers: alphanumeric and underscore, non-empty.
var validIdentifierRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validIdentifier reports whether name is a safe SQLite identifier (table or column name).
// SQLite identifiers must start with a letter or underscore and contain only
// alphanumeric characters and underscores. This prevents SQL injection via malicious
// identifier names in the migration code.
func validIdentifier(name string) bool {
	return name != "" && validIdentifierRE.MatchString(name)
}

// quoteIdentifier returns a safely quoted SQLite identifier using double-quotes.
// Embedded double-quotes are escaped by doubling them per SQLite rules.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Node represents an entity in the knowledge graph.
type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Properties map[string]string `json:"properties"`
	Protected  bool              `json:"protected,omitempty"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Relation   string            `json:"relation"`
	Properties map[string]string `json:"properties"`
}

// KnowledgeGraphDuplicateCandidate captures nodes that likely describe the same entity.
type KnowledgeGraphDuplicateCandidate struct {
	Label           string   `json:"label"`
	NormalizedLabel string   `json:"normalized_label"`
	Count           int      `json:"count"`
	IDs             []string `json:"ids"`
}

// KnowledgeGraphQualityReport summarizes curation-relevant graph quality signals.
type KnowledgeGraphQualityReport struct {
	Nodes               int                                `json:"nodes"`
	Edges               int                                `json:"edges"`
	ProtectedNodes      int                                `json:"protected_nodes"`
	IsolatedNodes       int                                `json:"isolated_nodes"`
	UntypedNodes        int                                `json:"untyped_nodes"`
	DuplicateGroups     int                                `json:"duplicate_groups"`
	DuplicateNodes      int                                `json:"duplicate_nodes"`
	IsolatedSample      []Node                             `json:"isolated_sample"`
	UntypedSample       []Node                             `json:"untyped_sample"`
	DuplicateCandidates []KnowledgeGraphDuplicateCandidate `json:"duplicate_candidates"`
}

// KnowledgeGraph implements the same interface as the old JSON-backed graph
// but stores all data in SQLite with FTS5 for full-text search.
type KnowledgeGraph struct {
	db          *sql.DB
	logger      *slog.Logger
	accessQueue chan knowledgeGraphAccessHit // buffered channel for async access-count updates
	doneChan    chan struct{}                // signals worker to exit
	closeOnce   sync.Once                    // ensures Close is only called once
	wg          sync.WaitGroup               // tracks accessCountWorker goroutine lifetime
	semantic    *knowledgeGraphSemanticIndex
}

const knowledgeGraphWriteTimeout = 5 * time.Second
const knowledgeGraphPropertyValueLimit = 500

var ErrKnowledgeGraphProtectedNode = errors.New("knowledge graph node is protected")

type knowledgeGraphAccessHit struct {
	nodeID   string
	source   string
	target   string
	relation string
}

// NewKnowledgeGraph creates a new SQLite-backed knowledge graph.
// If jsonMigratePath points to an existing JSON file, its data is imported
// and the file is renamed to .migrated.
func NewKnowledgeGraph(dbPath string, jsonMigratePath string, logger *slog.Logger) (*KnowledgeGraph, error) {
	db, err := dbutil.Open(dbPath, dbutil.WithMaxOpenConns(2))
	if err != nil {
		return nil, fmt.Errorf("open knowledge graph db: %w", err)
	}

	kg := &KnowledgeGraph{
		db:          db,
		logger:      logger,
		accessQueue: make(chan knowledgeGraphAccessHit, 1000), // buffer for 1000 access hits
		doneChan:    make(chan struct{}),
	}
	kg.wg.Add(1)
	go kg.accessCountWorker()
	if err := kg.initTables(); err != nil {
		close(kg.doneChan) // signal worker to exit
		kg.drainAccessQueue()
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("KG db close failed after init error", "error", closeErr)
		}
		return nil, fmt.Errorf("init knowledge graph tables: %w", err)
	}

	// One-time migration from old JSON file
	if jsonMigratePath != "" {
		kg.migrateFromJSON(jsonMigratePath)
	}

	return kg, nil
}

// Close closes the underlying database connection and stops the worker.
func (kg *KnowledgeGraph) Close() error {
	var err error
	kg.closeOnce.Do(func() {
		close(kg.doneChan) // signal worker to exit
		kg.wg.Wait()       // wait for worker goroutine to finish current DB write
		kg.drainAccessQueue()
		if kg.semantic != nil {
			kg.semantic.Close()
		}
		err = kg.db.Close()
	})
	return err
}

// drainAccessQueue drains pending items from the access queue before close.
// This ensures the worker has processed all pending updates before we close the DB.
func (kg *KnowledgeGraph) drainAccessQueue() {
	for {
		select {
		case hit := <-kg.accessQueue:
			ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphWriteTimeout)
			var execErr error
			switch {
			case hit.nodeID != "":
				_, execErr = kg.db.ExecContext(ctx, "UPDATE kg_nodes SET access_count = access_count + 1 WHERE id = ?", hit.nodeID)
			case hit.source != "" && hit.target != "" && hit.relation != "":
				_, execErr = kg.db.ExecContext(ctx, "UPDATE kg_edges SET access_count = access_count + 1 WHERE source = ? AND target = ? AND relation = ?", hit.source, hit.target, hit.relation)
			}
			cancel()
			if execErr != nil {
				kg.logger.Warn("KG access count update failed during drain", "hit", hit, "error", execErr)
			}
		default:
			return
		}
	}
}

// accessCountWorker drains the access queue and increments node/edge access counts.
// Runs as a single goroutine for the lifetime of the KnowledgeGraph.
// Exits when doneChan is closed or when db is closed.
func (kg *KnowledgeGraph) accessCountWorker() {
	defer kg.wg.Done()

	nodeHits := make(map[string]int)
	type edgeKey struct{ source, target, relation string }
	edgeHits := make(map[edgeKey]int)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(nodeHits) == 0 && len(edgeHits) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphWriteTimeout)
		defer cancel()

		tx, err := kg.db.BeginTx(ctx, nil)
		if err != nil {
			kg.logger.Warn("KG access count batch begin failed", "error", err)
			nodeHits = make(map[string]int)
			edgeHits = make(map[edgeKey]int)
			return
		}

		if len(nodeHits) > 0 {
			stmt, err := tx.PrepareContext(ctx, "UPDATE kg_nodes SET access_count = access_count + ? WHERE id = ?")
			if err == nil {
				for id, count := range nodeHits {
					if _, execErr := stmt.ExecContext(ctx, count, id); execErr != nil {
						kg.logger.Warn("KG access count update failed for node", "id", id, "count", count, "error", execErr)
					}
				}
				stmt.Close()
			}
		}

		if len(edgeHits) > 0 {
			stmt, err := tx.PrepareContext(ctx, "UPDATE kg_edges SET access_count = access_count + ? WHERE source = ? AND target = ? AND relation = ?")
			if err == nil {
				for e, count := range edgeHits {
					if _, execErr := stmt.ExecContext(ctx, count, e.source, e.target, e.relation); execErr != nil {
						kg.logger.Warn("KG access count update failed for edge", "source", e.source, "target", e.target, "relation", e.relation, "error", execErr)
					}
				}
				stmt.Close()
			}
		}

		if err := tx.Commit(); err != nil {
			kg.logger.Warn("KG access count batch commit failed", "error", err)
		}

		// Reset tracking
		nodeHits = make(map[string]int)
		edgeHits = make(map[edgeKey]int)
	}

	for {
		select {
		case <-kg.doneChan:
			flush()
			return
		case hit, ok := <-kg.accessQueue:
			if !ok {
				flush()
				return // channel closed
			}
			if hit.nodeID != "" {
				nodeHits[hit.nodeID]++
			} else if hit.source != "" && hit.target != "" && hit.relation != "" {
				edgeHits[edgeKey{hit.source, hit.target, hit.relation}]++
			}

			if len(nodeHits)+len(edgeHits) > 200 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// kgFTSSchemaVersion must be bumped whenever the FTS CREATE VIRTUAL TABLE or
// trigger definitions in rebuildFTSIndexes change, so old databases re-index.
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

	// Idempotent column migrations — skip if column already exists.
	type colMigration struct {
		table, column, def string
	}
	colMigrations := []colMigration{
		{"kg_nodes", "semantic_indexed_at", "DATETIME"},
		{"kg_edges", "semantic_indexed_at", "DATETIME"},
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

	// STORED generated columns cannot be added via ALTER TABLE in SQLite.
	// If the table was created without them (pre-existing DB), rebuild the table.
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

	// Create indexes on generated columns (safe after rebuild or fresh create).
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
	// FTS indices are derived entirely from kg_nodes/kg_edges, so rebuilding them on startup
	// is safe and also repairs older incompatible virtual-table definitions.
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
		// Note: Using content_rowid=rowid because kg_nodes uses implicit rowid.
		`CREATE VIRTUAL TABLE kg_nodes_fts USING fts5(
			id, label, properties, content=kg_nodes, content_rowid=rowid
		);`,
		// Note: Using content_rowid=id because kg_edges uses AUTOINCREMENT which makes 'id' an alias for rowid.
		// This is functionally equivalent to content_rowid=rowid but more explicit.
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

// migrateFromJSON imports data from an old graph.json file and renames it to .migrated.
func (kg *KnowledgeGraph) migrateFromJSON(jsonPath string) {
	f, err := os.Open(jsonPath)
	if err != nil {
		return // No file to migrate — normal case
	}

	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		f.Close()
		return
	}
	f.Close() // Close before rename

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
			continue // Skip empty IDs from legacy data
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

// ResetSemanticIndex clears the semantic_indexed_at timestamp on all nodes and edges,
// preparing them for re-indexing with a new embedding model. This method uses the
// existing database connection rather than opening a separate one.
func (kg *KnowledgeGraph) ResetSemanticIndex() error {
	if kg == nil || kg.db == nil {
		return fmt.Errorf("knowledge graph not initialized")
	}

	result, err := kg.db.Exec("UPDATE kg_nodes SET semantic_indexed_at = NULL WHERE semantic_indexed_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("reset kg node semantic_indexed_at: %w", err)
	}
	nodesReset, _ := result.RowsAffected()

	result, err = kg.db.Exec("UPDATE kg_edges SET semantic_indexed_at = NULL WHERE semantic_indexed_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("reset kg edge semantic_indexed_at: %w", err)
	}
	edgesReset, _ := result.RowsAffected()

	if kg.logger != nil && (nodesReset > 0 || edgesReset > 0) {
		kg.logger.Info("KG semantic index reset — will be rebuilt with new model",
			"nodes_reset", nodesReset, "edges_reset", edgesReset)
	}

	return nil
}

// Stats returns the number of nodes and edges in the knowledge graph.
func (kg *KnowledgeGraph) Stats() (nodes int, edges int, err error) {
	var nodeErr, edgeErr error
	if nodeErr = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&nodes); nodeErr != nil {
		kg.logger.Warn("KG Stats: failed to count nodes", "error", nodeErr)
		nodes = -1
	}
	if edgeErr = kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&edges); edgeErr != nil {
		kg.logger.Warn("KG Stats: failed to count edges", "error", edgeErr)
		edges = -1
	}
	return nodes, edges, errors.Join(nodeErr, edgeErr)
}

// QualityReport returns simple dashboard-friendly quality diagnostics for the graph.
func (kg *KnowledgeGraph) QualityReport(sampleLimit int) (*KnowledgeGraphQualityReport, error) {
	if sampleLimit <= 0 {
		sampleLimit = 5
	}
	if sampleLimit > 50 {
		sampleLimit = 50
	}

	report := &KnowledgeGraphQualityReport{
		IsolatedSample: make([]Node, 0, sampleLimit),
		UntypedSample:  make([]Node, 0, sampleLimit),
	}

	// Use a read-only transaction for all queries
	tx, err := kg.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin quality report transaction: %w", err)
	}
	defer tx.Rollback()

	tx.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.Nodes)
	tx.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.Edges)
	tx.QueryRow("SELECT COUNT(*) FROM kg_nodes WHERE protected != 0").Scan(&report.ProtectedNodes)

	// Isolated nodes
	tx.QueryRow(`SELECT COUNT(*) FROM kg_nodes n WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)`).Scan(&report.IsolatedNodes)

	isolatedRows, _ := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)
		LIMIT ?`, sampleLimit)
	if isolatedRows != nil {
		defer isolatedRows.Close()
		for isolatedRows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := isolatedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				report.IsolatedSample = append(report.IsolatedSample, n)
			}
		}
	}

	// Untyped nodes
	tx.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
	`).Scan(&report.UntypedNodes)

	untypedRows, _ := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
		LIMIT ?`, sampleLimit)
	if untypedRows != nil {
		defer untypedRows.Close()
		for untypedRows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := untypedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				report.UntypedSample = append(report.UntypedSample, n)
			}
		}
	}

	// Duplicate groups
	dupGroupRows, _ := tx.Query(`
		SELECT LOWER(TRIM(label)), COUNT(*) 
		FROM kg_nodes 
		WHERE label != ''
		GROUP BY LOWER(TRIM(label)) 
		HAVING COUNT(*) > 1
	`)
	if dupGroupRows != nil {
		defer dupGroupRows.Close()
		var labels []string
		for dupGroupRows.Next() {
			var label string
			var count int
			if err := dupGroupRows.Scan(&label, &count); err == nil {
				report.DuplicateGroups++
				report.DuplicateNodes += count
				if len(labels) < sampleLimit {
					labels = append(labels, label)
				}
			}
		}

		for _, l := range labels {
			cand := KnowledgeGraphDuplicateCandidate{
				Label:           l,
				NormalizedLabel: l,
				Count:           0,
			}
			nodesRows, _ := tx.Query(`SELECT id, label, properties, protected FROM kg_nodes WHERE LOWER(TRIM(label)) = ?`, l)
			if nodesRows != nil {
				for nodesRows.Next() {
					var n Node
					var propsJSON string
					var protected int
					if err := nodesRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
						cand.IDs = append(cand.IDs, n.ID)
						cand.Count++
					}
				}
				nodesRows.Close()
			}
			report.DuplicateCandidates = append(report.DuplicateCandidates, cand)
		}
	}

	return report, nil
}

// validateNodeSchema ensures common node types have basic property structures.
func validateNodeSchema(properties map[string]string) map[string]string {
	if properties == nil {
		properties = make(map[string]string)
	}
	nodeType := strings.ToLower(properties["type"])

	ensureKey := func(key string) {
		if _, exists := properties[key]; !exists {
			properties[key] = ""
		}
	}

	switch nodeType {
	case "device":
		ensureKey("ip")
		ensureKey("mac")
		ensureKey("os")
	case "service":
		ensureKey("port")
		ensureKey("protocol")
	case "person":
		ensureKey("role")
		ensureKey("email")
	case "container":
		ensureKey("image")
		ensureKey("state")
	case "software":
		ensureKey("version")
		ensureKey("vendor")
	}
	return properties
}

// AddNode adds or updates a node in the knowledge graph.
// On conflict, properties are merged: existing keys are preserved, new keys from
// incoming properties are added, and matching keys are overwritten with incoming values
// (same merge behavior as BulkMergeExtractedEntities).
func (kg *KnowledgeGraph) AddNode(id, label string, properties map[string]string) error {
	isProtected := strings.EqualFold(strings.TrimSpace(properties["protected"]), "true")
	properties = sanitizeKnowledgeGraphNodeProperties(properties, isProtected)

	label = strings.TrimSpace(label)

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add node: %w", err)
	}
	defer tx.Rollback()

	existingLabel, existingProps, existingProtected, _, err := loadKnowledgeGraphNode(tx, id)
	if err != nil {
		return fmt.Errorf("load existing node %s: %w", id, err)
	}

	finalLabel := mergeKnowledgeGraphLabel(existingLabel, label)
	finalProps := mergeKnowledgeGraphPropertiesOverwrite(existingProps, properties)
	isProtectedFinal := existingProtected
	if finalProps["protected"] == "true" {
		isProtectedFinal = 1
	}
	finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, isProtectedFinal != 0)

	propsJSON, err := json.Marshal(finalProps)
	if err != nil {
		return fmt.Errorf("marshal node properties: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			label = excluded.label,
			properties = excluded.properties,
			protected = excluded.protected,
			updated_at = CURRENT_TIMESTAMP
	`, id, finalLabel, string(propsJSON), isProtectedFinal)
	if err != nil {
		return fmt.Errorf("add node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	kg.upsertSemanticNodeIndex(Node{ID: id, Label: finalLabel, Properties: finalProps})
	return nil
}

// AddEdge adds or updates an edge. Auto-creates missing endpoint nodes.
// On conflict, properties are merged (new keys added, existing keys overwritten).
func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	properties = normalizeKnowledgeGraphProperties(properties)

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add edge: %w", err)
	}
	defer tx.Rollback()

	for _, id := range []string{source, target} {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO kg_nodes (id, label, properties) VALUES (?, 'Unknown', '{}')`, id); err != nil {
			kg.logger.Warn("AddEdge: failed to ensure node exists", "id", id, "error", err)
		}
	}

	existingProps, found, err := loadKnowledgeGraphEdge(tx, source, target, relation)
	if err != nil {
		return fmt.Errorf("load existing edge for add: %w", err)
	}

	var finalProps map[string]string
	if found {
		finalProps = mergeKnowledgeGraphPropertiesOverwrite(existingProps, properties)
	} else {
		finalProps = properties
	}
	propsJSON, _ := json.Marshal(finalProps)
	_, err = tx.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties
	`, source, target, relation, string(propsJSON))
	if err != nil {
		return fmt.Errorf("add edge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	kg.upsertSemanticNodeIndex(Node{ID: source, Label: source, Properties: nil})
	kg.upsertSemanticNodeIndex(Node{ID: target, Label: target, Properties: nil})
	kg.upsertSemanticEdgeIndex(Edge{Source: source, Target: target, Relation: relation})
	return nil
}

const coOccurrenceThreshold = 15

// IncrementCoOccurrence tracks how often two entities are mentioned together.
// On the first co-mention a pending edge (source="pending") is inserted.
// Once the pair reaches coOccurrenceThreshold co-mentions the edge is promoted
// to source="activity_turn" and becomes a fully active graph edge.
// Existing edges (pending or active) get their weight incremented and date updated.
func (kg *KnowledgeGraph) IncrementCoOccurrence(a, b, date string) error {
	// Enforce canonical ordering to prevent duplicate (a,b) vs (b,a) edges.
	if a > b {
		a, b = b, a
	}

	// Wrap in a transaction to prevent TOCTOU races
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin co-occurrence transaction: %w", err)
	}
	defer tx.Rollback()

	var currentWeight int
	var propsJSON string
	err = tx.QueryRow(
		"SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
		a, b,
	).Scan(&propsJSON)
	if err == nil {
		// Edge exists (pending or active) — increment weight and potentially promote.
		var props map[string]string
		if json.Unmarshal([]byte(propsJSON), &props) == nil {
			if w, e := strconv.Atoi(props["weight"]); e == nil {
				currentWeight = w
			}
		}
		currentWeight++
		props["weight"] = strconv.Itoa(currentWeight)
		props["date"] = date
		if currentWeight >= coOccurrenceThreshold {
			props["source"] = "activity_turn"
		}
		newPropsJSON, _ := json.Marshal(props)
		_, err = tx.Exec(
			"UPDATE kg_edges SET properties = ? WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
			string(newPropsJSON), a, b,
		)
		if err != nil {
			return fmt.Errorf("update co-occurrence: %w", err)
		}
	} else if err == sql.ErrNoRows {
		// Edge does not exist yet — insert as pending (weight=1, below threshold).
		initProps, _ := json.Marshal(map[string]string{
			"source": "pending",
			"weight": "1",
			"date":   date,
		})
		_, err = tx.Exec(`
			INSERT INTO kg_edges (source, target, relation, properties)
			VALUES (?, ?, 'co_mentioned_with', ?)
			ON CONFLICT(source, target, relation) DO UPDATE SET
				properties = excluded.properties
		`, a, b, string(initProps))
		if err != nil {
			return fmt.Errorf("insert co-occurrence: %w", err)
		}
	} else {
		return fmt.Errorf("query co-occurrence: %w", err)
	}

	return tx.Commit()
}

// Uses a single UNION query combining FTS5 and LIKE to avoid two database round-trips.
// Access counts are updated asynchronously via a worker pool.
func (kg *KnowledgeGraph) Search(query string) string {
	if query == "" {
		return "[]"
	}

	var matchedNodes []Node
	var matchedEdges []Edge
	var matchedNodeIDs []string
	var matchedEdgeHits []knowledgeGraphAccessHit

	// Combined FTS5 + LIKE UNION: one round-trip instead of two.
	// FTS5 results are preferred; LIKE catches non-indexed partial matches.
	// Escape LIKE wildcards in the query to prevent unintended pattern matching.
	ftsQuery := escapeFTS5(query)
	escapedLike := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(query)
	likePattern := "%" + escapedLike + "%"
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?)
		UNION
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE id LIKE ? ESCAPE '\' OR label LIKE ? ESCAPE '\' OR properties LIKE ? ESCAPE '\'
		LIMIT 50
	`, ftsQuery, likePattern, likePattern, likePattern)
	if err != nil {
		kg.logger.Warn("Search: node query failed", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "Search", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				matchedNodes = append(matchedNodes, n)
				matchedNodeIDs = append(matchedNodeIDs, n.ID)
			}
		}
	}

	// Edge search — substring match on source, target, relation
	escapedLikeEdge := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.ToLower(query))
	likeQ := "%" + escapedLikeEdge + "%"
	edgeFTSQuery := escapeFTS5(query)
	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE id IN (SELECT rowid FROM kg_edges_fts WHERE kg_edges_fts MATCH ?)
		UNION
		SELECT source, target, relation, properties FROM kg_edges
		WHERE LOWER(source) LIKE ? ESCAPE '\' OR LOWER(target) LIKE ? ESCAPE '\' OR LOWER(relation) LIKE ? ESCAPE '\' OR LOWER(properties) LIKE ? ESCAPE '\'
		LIMIT 50
	`, edgeFTSQuery, likeQ, likeQ, likeQ, likeQ)
	if err != nil {
		kg.logger.Warn("Search: edge query failed", "error", err)
	} else {
		defer edgeRows.Close()
		for edgeRows.Next() {
			var e Edge
			var propsJSON string
			if err := edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
				if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
					kg.logger.Warn("Search: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
				}
				if e.Properties == nil {
					e.Properties = make(map[string]string)
				}
				matchedEdges = append(matchedEdges, e)
				matchedEdgeHits = append(matchedEdgeHits, knowledgeGraphAccessHit{
					source:   e.Source,
					target:   e.Target,
					relation: e.Relation,
				})
			}
		}
	}

	if len(matchedNodes) == 0 && len(matchedEdges) == 0 {
		return "[]"
	}

	result := map[string]interface{}{
		"nodes": matchedNodes,
		"edges": matchedEdges,
	}
	data, _ := json.Marshal(result)

	// Queue access count updates — non-blocking, drops if buffer is full
	for _, id := range matchedNodeIDs {
		kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: id})
	}
	for _, hit := range matchedEdgeHits {
		kg.enqueueAccessHit(hit)
	}

	return string(data)
}

// GetNeighbors returns nodes directly connected to the given node ID (1-hop).
func (kg *KnowledgeGraph) GetNeighbors(nodeID string, limit int) ([]Node, []Edge) {
	if limit <= 0 {
		limit = 20
	}

	var edges []Edge
	rows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE source = ? OR target = ?
		LIMIT ?
	`, nodeID, nodeID, limit)
	if err != nil {
		return nil, nil
	}
	defer rows.Close() // defer handles all exit paths

	neighborIDs := make(map[string]bool)
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("GetNeighbors: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
			if e.Source != nodeID {
				neighborIDs[e.Source] = true
			}
			if e.Target != nodeID {
				neighborIDs[e.Target] = true
			}
		}
	}
	// rows.Close() called by defer when GetNeighbors returns

	var nodes []Node
	if len(neighborIDs) > 0 {
		var ids []interface{}
		var placeholders []string
		for id := range neighborIDs {
			ids = append(ids, id)
			placeholders = append(placeholders, "?")
		}

		query := "SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (" + strings.Join(placeholders, ",") + ")"
		rows, err := kg.db.Query(query, ids...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var n Node
				var propsJSON string
				var protected int
				if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
					n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNeighbors", n.ID, propsJSON, protected)
					n.Protected = protected != 0
					nodes = append(nodes, n)
				}
			}
		} else {
			kg.logger.Warn("GetNeighbors: batch node query failed", "error", err)
		}
	}

	kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: nodeID})
	for _, e := range edges {
		kg.enqueueAccessHit(knowledgeGraphAccessHit{source: e.Source, target: e.Target, relation: e.Relation})
	}

	return nodes, edges
}

// GetNode returns a single node by ID.
func (kg *KnowledgeGraph) GetNode(nodeID string) (*Node, error) {
	if strings.TrimSpace(nodeID) == "" {
		return nil, nil
	}

	var node Node
	var propsJSON string
	var protected int
	err := kg.db.QueryRow("SELECT id, label, properties, protected FROM kg_nodes WHERE id = ?", nodeID).Scan(&node.ID, &node.Label, &propsJSON, &protected)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", nodeID, err)
	}
	node.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNode", node.ID, propsJSON, protected)
	node.Protected = protected != 0
	return &node, nil
}

// SearchForContext finds nodes related to a query and returns a compact
// context string suitable for injection into the system prompt.
// Returns at most maxNodes relevant nodes with their 1-hop neighbors, capped at maxTokens characters.
func (kg *KnowledgeGraph) SearchForContext(query string, maxNodes int, maxChars int) string {
	if query == "" || maxNodes <= 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 2000
	}

	var nodeIDs []string
	type searchHit struct {
		score float32
		id    string
	}
	hits := make(map[string]float32)

	// Semantic Search (0.0 to 1.0)
	semScores := kg.semanticSearchNodeScores(query, maxNodes*2)
	for id, score := range semScores {
		hits[id] += score * 0.5 // Weight semantic search at 50%
	}

	// FTS5 (Fallback to LIKE)
	ftsQuery := escapeFTS5(query)
	rows, err := kg.db.Query(`
		SELECT n.id, n.access_count FROM kg_nodes_fts f
		JOIN kg_nodes n ON n.rowid = f.rowid
		WHERE kg_nodes_fts MATCH ?
		ORDER BY n.updated_at DESC
		LIMIT ?
	`, ftsQuery, maxNodes)

	count := 0
	if err != nil {
		kg.logger.Warn("SearchForContext: FTS5 query failed", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id string
			var ac sql.NullInt64
			if rows.Scan(&id, &ac) == nil {
				// FTS score: max 0.4 (decaying per position), plus access boost max 0.1
				ftsScore := float32(0.4) - (float32(count) * 0.05)
				if ftsScore < 0.1 {
					ftsScore = 0.1
				}
				accessBoost := float32(0)
				if ac.Valid && ac.Int64 > 0 {
					accessBoost = float32(ac.Int64) / 100.0
					if accessBoost > 0.1 {
						accessBoost = 0.1
					}
				}
				hits[id] += ftsScore + accessBoost
				count++
			}
		}
	}

	if count == 0 {
		likeQ := "%" + dbutil.EscapeLike(query) + "%"
		likeRows, err := kg.db.Query(`
			SELECT id, access_count FROM kg_nodes
			WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
			ORDER BY updated_at DESC, access_count DESC
			LIMIT ?
		`, likeQ, likeQ, likeQ, maxNodes)
		if err != nil {
			kg.logger.Warn("SearchForContext: LIKE fallback query failed", "error", err)
		} else {
			defer likeRows.Close()
			for likeRows.Next() {
				var id string
				var ac sql.NullInt64
				if likeRows.Scan(&id, &ac) == nil {
					likeScore := float32(0.3) - (float32(count) * 0.05)
					if likeScore < 0.1 {
						likeScore = 0.1
					}
					hits[id] += likeScore
					count++
				}
			}
		}
	}

	var rankedHits []searchHit
	for id, score := range hits {
		rankedHits = append(rankedHits, searchHit{score: score, id: id})
	}

	// Sort by score descending
	sort.Slice(rankedHits, func(i, j int) bool {
		return rankedHits[i].score > rankedHits[j].score
	})

	for i, hit := range rankedHits {
		if i >= maxNodes {
			break
		}
		nodeIDs = append(nodeIDs, hit.id)
	}

	if len(nodeIDs) == 0 {
		return ""
	}

	var accessHits []knowledgeGraphAccessHit

	var sb strings.Builder
	for _, nid := range nodeIDs {
		var label, propsJSON string
		err := kg.db.QueryRow("SELECT label, properties FROM kg_nodes WHERE id = ?", nid).Scan(&label, &propsJSON)
		if err != nil {
			continue
		}

		accessHits = append(accessHits, knowledgeGraphAccessHit{nodeID: nid})

		sb.WriteString(fmt.Sprintf("- [%s] %s", nid, label))

		var props map[string]string
		json.Unmarshal([]byte(propsJSON), &props)
		for k, v := range props {
			if k == "access_count" || k == "protected" || k == "source" || k == "extracted_at" {
				continue
			}
			sb.WriteString(fmt.Sprintf(" | %s: %s", k, v))
		}
		sb.WriteString("\n")

		edgeRows, err := kg.db.Query(`
			SELECT source, target, relation FROM kg_edges
			WHERE source = ? OR target = ?
			ORDER BY access_count DESC
			LIMIT 5
		`, nid, nid)
		if err != nil {
			kg.logger.Warn("SearchForContext: edge query failed", "nid", nid, "error", err)
		} else {
			defer edgeRows.Close()
			for edgeRows.Next() {
				var src, tgt, rel string
				if edgeRows.Scan(&src, &tgt, &rel) == nil {
					sb.WriteString(fmt.Sprintf("  - [%s] -[%s]-> [%s]\n", src, rel, tgt))
					accessHits = append(accessHits, knowledgeGraphAccessHit{
						source: src, target: tgt, relation: rel,
					})
				}
			}
		}

		if sb.Len() > maxChars {
			break
		}
	}

	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}

	result := sb.String()
	if len(result) > maxChars {
		result = truncateUTF8Safe(result, maxChars)
	}
	return result
}

func truncateUTF8Safe(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}

// DeleteNode removes a node and all its connected edges.
func (kg *KnowledgeGraph) DeleteNode(id string) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete node: %w", err)
	}
	defer tx.Rollback()

	var protected int
	err = tx.QueryRow("SELECT protected FROM kg_nodes WHERE id = ?", id).Scan(&protected)
	switch {
	case err == sql.ErrNoRows:
		return nil
	case err != nil:
		return fmt.Errorf("load node %s for delete: %w", id, err)
	case protected != 0:
		return ErrKnowledgeGraphProtectedNode
	}

	if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); err != nil {
		return fmt.Errorf("delete edges for node %s: %w", id, err)
	}
	if _, err := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete node %s: %w", id, err)
	}

	return tx.Commit()
}

// DeleteEdge removes a specific edge.
func (kg *KnowledgeGraph) DeleteEdge(source, target, relation string) error {
	_, err := kg.db.Exec("DELETE FROM kg_edges WHERE source = ? AND target = ? AND relation = ?",
		source, target, relation)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}
	return nil
}

// UpdateEdge updates an edge relation and properties. If the relation changes, the old
// edge identity is replaced atomically with the new one.
func (kg *KnowledgeGraph) UpdateEdge(source, target, relation, newRelation string, properties map[string]string) (*Edge, error) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	newRelation = strings.TrimSpace(newRelation)
	if source == "" || target == "" || relation == "" {
		return nil, nil
	}
	if newRelation == "" {
		newRelation = relation
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin update edge: %w", err)
	}
	defer tx.Rollback()

	existingProps, found, err := loadKnowledgeGraphEdge(tx, source, target, relation)
	if err != nil {
		return nil, fmt.Errorf("load edge for update: %w", err)
	}
	if !found {
		return nil, nil
	}

	finalProps := existingProps
	if properties != nil {
		finalProps = normalizeKnowledgeGraphProperties(properties)
	}
	propsJSON, err := json.Marshal(finalProps)
	if err != nil {
		return nil, fmt.Errorf("marshal edge properties: %w", err)
	}

	if relation != newRelation {
		if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? AND target = ? AND relation = ?", source, target, relation); err != nil {
			return nil, fmt.Errorf("delete old edge for update: %w", err)
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties
	`, source, target, newRelation, string(propsJSON)); err != nil {
		return nil, fmt.Errorf("upsert updated edge: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Edge{
		Source:     source,
		Target:     target,
		Relation:   newRelation,
		Properties: finalProps,
	}, nil
}

// OptimizeGraph archives low-priority nodes below the threshold.
// Priority = access_count + (degree * 2). Protected nodes are skipped.
func (kg *KnowledgeGraph) OptimizeGraph(threshold int) (int, error) {
	// Get all non-protected nodes with their priority scores
	rows, err := kg.db.Query(`
		SELECT n.id, n.access_count,
			(SELECT COUNT(*) FROM kg_edges e WHERE e.source = n.id OR e.target = n.id) as degree
		FROM kg_nodes n
		WHERE n.protected = 0
	`)
	if err != nil {
		return 0, fmt.Errorf("query for optimization: %w", err)
	}
	defer rows.Close()

	var toRemove []string
	for rows.Next() {
		var id string
		var accessCount, degree int
		if err := rows.Scan(&id, &accessCount, &degree); err == nil {
			priority := accessCount + (degree * 2)
			if priority < threshold {
				toRemove = append(toRemove, id)
			}
		}
	}

	if len(toRemove) == 0 {
		return 0, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	nodesDeleted := 0
	for _, id := range toRemove {
		if _, execErr := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); execErr != nil {
			kg.logger.Warn("OptimizeGraph: failed to delete edges for node", "id", id, "error", execErr)
		}
		if _, execErr := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); execErr != nil {
			kg.logger.Warn("OptimizeGraph: failed to delete node", "id", id, "error", execErr)
		} else {
			nodesDeleted++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nodesDeleted, nil
}

// CleanupStaleGraph removes pending edges (co_mentioned_with edges that never reached threshold)
// and unaccessed nodes (access_count = 0) that are older than the specified threshold in days.
// Protected nodes are never removed. It returns the number of deleted edges, nodes, and any error.
func (kg *KnowledgeGraph) CleanupStaleGraph(thresholdDays int) (int, int, error) {
	if thresholdDays <= 0 {
		return 0, 0, fmt.Errorf("invalid thresholdDays: %d", thresholdDays)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin cleanup graph: %w", err)
	}
	defer tx.Rollback()

	// 1. Delete stale pending edges
	edgeRes, err := tx.Exec(`
		DELETE FROM kg_edges 
		WHERE relation = 'co_mentioned_with' 
		  AND json_extract(properties, '$.source') = 'pending'
		  AND created_at <= datetime('now', '-' || ? || ' days')
	`, thresholdDays)
	if err != nil {
		return 0, 0, fmt.Errorf("delete stale pending edges: %w", err)
	}
	edgesDeleted, _ := edgeRes.RowsAffected()

	// 2. Identify unaccessed old nodes
	rows, err := tx.Query(`
		SELECT id FROM kg_nodes
		WHERE access_count = 0 
		  AND protected = 0
		  AND updated_at <= datetime('now', '-' || ? || ' days')
	`, thresholdDays)
	if err != nil {
		return 0, 0, fmt.Errorf("query unaccessed nodes: %w", err)
	}
	defer rows.Close()

	var toRemove []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		if _, execErr := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); execErr != nil {
			kg.logger.Warn("CleanupStaleGraph: failed to delete edges for node", "id", id, "error", execErr)
		}
		if _, execErr := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); execErr != nil {
			kg.logger.Warn("CleanupStaleGraph: failed to delete node", "id", id, "error", execErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit cleanup graph: %w", err)
	}

	return int(edgesDeleted), len(toRemove), nil
}

// GetAllNodes returns all nodes in the graph (for export/dashboard).
func (kg *KnowledgeGraph) GetAllNodes(limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := kg.db.Query("SELECT id, label, properties, protected FROM kg_nodes ORDER BY access_count DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetAllNodes", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// GetAllEdges returns all edges in the graph (for export/dashboard).
func (kg *KnowledgeGraph) GetAllEdges(limit int) ([]Edge, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := kg.db.Query("SELECT source, target, relation, properties FROM kg_edges LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("GetAllEdges: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
		}
	}
	return edges, nil
}

// ImportantNode extends Node with an importance score for dashboard display.
type ImportantNode struct {
	Node
	ImportanceScore int `json:"importance_score"`
}

// KnowledgeGraphStats holds structured statistics about the knowledge graph.
type KnowledgeGraphStats struct {
	TotalNodes      int            `json:"total_nodes"`
	TotalEdges      int            `json:"total_edges"`
	MeaningfulEdges int            `json:"meaningful_edges"`
	CoMentionEdges  int            `json:"co_mention_edges"`
	ByType          map[string]int `json:"by_type"`
	BySource        map[string]int `json:"by_source"`
}

// GetImportantNodes returns nodes sorted by a composite importance score.
// The score rewards: protected status, manual source, typed nodes, access count,
// meaningful connections (excluding co_mentioned_with), rich properties, and recency.
func (kg *KnowledgeGraph) GetImportantNodes(limit int, minScore int) ([]ImportantNode, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}
	if minScore < 0 {
		minScore = 0
	}

	rows, err := kg.db.Query(`
		SELECT n.id, n.label, n.properties, n.protected,
			(CASE WHEN n.protected = 1 THEN 50 ELSE 0 END) +
			(CASE WHEN json_extract(n.properties, '$.source') = 'manual' THEN 30 ELSE 0 END) +
			(CASE WHEN json_extract(n.properties, '$.source') = 'auto_extraction' THEN 5 ELSE 0 END) +
			(CASE WHEN json_extract(n.properties, '$.type') IS NOT NULL
				AND json_extract(n.properties, '$.type') != '' THEN 15 ELSE 0 END) +
			MIN(n.access_count, 20) +
			MIN((
				SELECT COUNT(*) FROM kg_edges e
				WHERE (e.source = n.id OR e.target = n.id)
				  AND e.relation != 'co_mentioned_with'
			) * 3, 30) +
			(CASE WHEN (SELECT COUNT(*) FROM json_each(n.properties)
				WHERE key NOT IN ('source','extracted_at','last_seen','session_id','channel','protected')) >= 3
				THEN 10 ELSE 0 END) +
			(CASE WHEN n.updated_at > datetime('now', '-7 days') THEN 5 ELSE 0 END)
			AS importance_score
		FROM kg_nodes n
		WHERE importance_score >= ?
		ORDER BY importance_score DESC
		LIMIT ?
	`, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("query important nodes: %w", err)
	}
	defer rows.Close()

	var result []ImportantNode
	for rows.Next() {
		var n ImportantNode
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected, &n.ImportanceScore); err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetImportantNodes", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			result = append(result, n)
		}
	}
	return result, nil
}

// GetImportantEdges returns edges excluding co_mentioned_with, filtered to the given node IDs.
// If nodeIDs is empty, all non-co-mention edges are returned.
func (kg *KnowledgeGraph) GetImportantEdges(limit int, nodeIDs []string) ([]Edge, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	var rows *sql.Rows
	var err error
	if len(nodeIDs) > 0 {
		placeholders := make([]string, len(nodeIDs))
		args := make([]interface{}, 0, len(nodeIDs)+1)
		for i, id := range nodeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		args = append(args, limit)

		query := fmt.Sprintf(`
			SELECT source, target, relation, properties FROM kg_edges
			WHERE relation != 'co_mentioned_with'
			  AND (source IN (%s) OR target IN (%s))
			ORDER BY (
				SELECT SUM(n2.access_count) FROM kg_nodes n2
				WHERE n2.id IN (kg_edges.source, kg_edges.target)
			) DESC
			LIMIT ?
		`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))
		allArgs := make([]interface{}, 0, len(nodeIDs)*2+1)
		for _, id := range nodeIDs {
			allArgs = append(allArgs, id)
		}
		for _, id := range nodeIDs {
			allArgs = append(allArgs, id)
		}
		allArgs = append(allArgs, limit)
		rows, err = kg.db.Query(query, allArgs...)
	} else {
		rows, err = kg.db.Query(`
			SELECT source, target, relation, properties FROM kg_edges
			WHERE relation != 'co_mentioned_with'
			LIMIT ?
		`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query important edges: %w", err)
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("GetImportantEdges: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
		}
	}
	return edges, nil
}

// GetStats returns structured statistics about the knowledge graph.
func (kg *KnowledgeGraph) GetStats() (*KnowledgeGraphStats, error) {
	stats := &KnowledgeGraphStats{
		ByType:   make(map[string]int),
		BySource: make(map[string]int),
	}

	kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&stats.TotalNodes)
	kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&stats.TotalEdges)
	kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE relation = 'co_mentioned_with'").Scan(&stats.CoMentionEdges)
	stats.MeaningfulEdges = stats.TotalEdges - stats.CoMentionEdges

	typeRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.type'), ''), 'untyped') AS t, COUNT(*)
		FROM kg_nodes GROUP BY t ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var t string
			var c int
			if typeRows.Scan(&t, &c) == nil {
				stats.ByType[t] = c
			}
		}
	}

	sourceRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.source'), ''), 'unknown') AS s, COUNT(*)
		FROM kg_nodes GROUP BY s ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var s string
			var c int
			if sourceRows.Scan(&s, &c) == nil {
				stats.BySource[s] = c
			}
		}
	}

	return stats, nil
}

// BulkAddEntities adds multiple nodes and edges in a single transaction.
// Used by nightly batch entity extraction.
func (kg *KnowledgeGraph) BulkAddEntities(nodes []Node, edges []Edge) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin bulk add: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	indexNodes := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if n.ID == "" {
			continue
		}
		n.Properties = sanitizeKnowledgeGraphNodeProperties(n.Properties, strings.EqualFold(strings.TrimSpace(n.Properties["protected"]), "true"))
		propsJSON, _ := json.Marshal(n.Properties)
		isProtected := boolToInt(strings.EqualFold(strings.TrimSpace(n.Properties["protected"]), "true"))
		if _, execErr := tx.Exec(`
			INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				label = CASE WHEN excluded.label != 'Unknown' THEN excluded.label ELSE kg_nodes.label END,
				properties = excluded.properties,
				protected = excluded.protected,
				updated_at = ?
		`, n.ID, n.Label, string(propsJSON), isProtected, now, now); execErr != nil {
			kg.logger.Warn("[KG] BulkAddEntities: failed to insert node", "id", n.ID, "error", execErr)
		}
		indexNodes = append(indexNodes, Node{ID: n.ID, Label: n.Label, Properties: n.Properties})
	}

	for _, e := range edges {
		// Auto-create endpoint nodes
		for _, id := range []string{e.Source, e.Target} {
			if _, execErr := tx.Exec(`INSERT OR IGNORE INTO kg_nodes (id, label, properties) VALUES (?, 'Unknown', '{}')`, id); execErr != nil {
				kg.logger.Warn("[KG] BulkAddEntities: failed to ensure endpoint node", "id", id, "error", execErr)
			}
		}
		if e.Properties == nil {
			e.Properties = make(map[string]string)
		}
		e.Properties = normalizeKnowledgeGraphProperties(e.Properties)
		propsJSON, _ := json.Marshal(e.Properties)
		if _, execErr := tx.Exec(`
			INSERT INTO kg_edges (source, target, relation, properties)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(source, target, relation) DO UPDATE SET
				properties = excluded.properties
		`, e.Source, e.Target, e.Relation, string(propsJSON)); execErr != nil {
			kg.logger.Warn("[KG] BulkAddEntities: failed to insert edge", "source", e.Source, "target", e.Target, "error", execErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	for _, node := range indexNodes {
		kg.upsertSemanticNodeIndex(node)
	}
	for _, e := range edges {
		if e.Source != "" && e.Target != "" && e.Relation != "" {
			kg.upsertSemanticEdgeIndex(e)
		}
	}
	return nil
}

// BulkMergeExtractedEntities merges auto-extracted nodes and edges into the graph
// without overwriting existing curated data. Existing non-empty properties win on
// conflicts; new properties are appended where possible.
func (kg *KnowledgeGraph) BulkMergeExtractedEntities(nodes []Node, edges []Edge) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin bulk merge: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	mergedNodes := mergeKnowledgeGraphNodes(nodes)
	mergedEdges := mergeKnowledgeGraphEdges(edges)
	indexNodes := make([]Node, 0, len(mergedNodes))
	indexEdges := make([]Edge, 0, len(mergedEdges))
	for _, n := range mergedNodes {
		if n.ID == "" {
			continue
		}
		existingLabel, existingProps, existingProtected, _, err := loadKnowledgeGraphNode(tx, n.ID)
		if err != nil {
			return fmt.Errorf("load existing node %q: %w", n.ID, err)
		}

		n.Properties = sanitizeKnowledgeGraphNodeProperties(n.Properties, strings.EqualFold(strings.TrimSpace(n.Properties["protected"]), "true"))
		finalLabel := mergeKnowledgeGraphLabel(existingLabel, n.Label)
		finalProps := mergeKnowledgeGraphProperties(existingProps, n.Properties)
		isProtected := existingProtected
		if finalProps["protected"] == "true" {
			isProtected = 1
		}
		finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, isProtected != 0)
		propsJSON, _ := json.Marshal(finalProps)

		if _, execErr := tx.Exec(`
			INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				label = CASE WHEN excluded.label = 'Unknown' THEN kg_nodes.label ELSE excluded.label END,
				properties = excluded.properties,
				protected = excluded.protected,
				updated_at = excluded.updated_at
		`, n.ID, finalLabel, string(propsJSON), isProtected, now); execErr != nil {
			kg.logger.Warn("[KG] BulkMergeExtractedEntities: failed to merge node", "id", n.ID, "error", execErr)
		}
		indexNodes = append(indexNodes, Node{ID: n.ID, Label: finalLabel, Properties: finalProps})
	}

	for _, e := range mergedEdges {
		if e.Source == "" || e.Target == "" || e.Relation == "" {
			continue
		}
		for _, id := range []string{e.Source, e.Target} {
			if _, execErr := tx.Exec(`INSERT OR IGNORE INTO kg_nodes (id, label, properties) VALUES (?, 'Unknown', '{}')`, id); execErr != nil {
				kg.logger.Warn("[KG] BulkMergeExtractedEntities: failed to ensure endpoint node", "id", id, "error", execErr)
			}
		}

		existingProps, _, err := loadKnowledgeGraphEdge(tx, e.Source, e.Target, e.Relation)
		if err != nil {
			return fmt.Errorf("load existing edge %q->%q/%q: %w", e.Source, e.Target, e.Relation, err)
		}
		e.Properties = normalizeKnowledgeGraphProperties(e.Properties)
		finalProps := mergeKnowledgeGraphProperties(existingProps, e.Properties)
		propsJSON, _ := json.Marshal(finalProps)

		if _, execErr := tx.Exec(`
			INSERT INTO kg_edges (source, target, relation, properties)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(source, target, relation) DO UPDATE SET
				properties = excluded.properties
		`, e.Source, e.Target, e.Relation, string(propsJSON)); execErr != nil {
			kg.logger.Warn("[KG] BulkMergeExtractedEntities: failed to merge edge", "source", e.Source, "target", e.Target, "error", execErr)
		}
		indexEdges = append(indexEdges, e)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	for _, node := range indexNodes {
		kg.upsertSemanticNodeIndex(node)
	}
	for _, e := range indexEdges {
		kg.upsertSemanticEdgeIndex(e)
	}
	return nil
}

// UpdateNode updates a node label and properties while preserving the protected state.
func (kg *KnowledgeGraph) UpdateNode(id, label string, properties map[string]string) (*Node, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin update node: %w", err)
	}
	defer tx.Rollback()

	existingLabel, existingProps, existingProtected, found, err := loadKnowledgeGraphNode(tx, id)
	if err != nil {
		return nil, fmt.Errorf("load node %s for update: %w", id, err)
	}
	if !found {
		return nil, nil
	}

	finalLabel := strings.TrimSpace(label)
	if finalLabel == "" {
		finalLabel = existingLabel
	}

	finalProps := existingProps
	if properties != nil {
		finalProps = sanitizeKnowledgeGraphNodeProperties(properties, existingProtected != 0)
	}
	finalProps = validateNodeSchema(finalProps)
	propsJSON, err := json.Marshal(finalProps)
	if err != nil {
		return nil, fmt.Errorf("marshal updated node properties: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE kg_nodes
		SET label = ?, properties = ?, protected = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, finalLabel, string(propsJSON), existingProtected, id); err != nil {
		return nil, fmt.Errorf("update node %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	node := &Node{ID: id, Label: finalLabel, Properties: finalProps, Protected: existingProtected != 0}
	kg.upsertSemanticNodeIndex(*node)
	return node, nil
}

// SetNodeProtected toggles the protected state for a node.
func (kg *KnowledgeGraph) SetNodeProtected(id string, protected bool) (*Node, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin set node protected: %w", err)
	}
	defer tx.Rollback()

	label, properties, _, found, err := loadKnowledgeGraphNode(tx, id)
	if err != nil {
		return nil, fmt.Errorf("load node %s for protection update: %w", id, err)
	}
	if !found {
		return nil, nil
	}

	properties = sanitizeKnowledgeGraphNodeProperties(properties, protected)
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("marshal node protection properties: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE kg_nodes
		SET properties = ?, protected = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(propsJSON), boolToInt(protected), id); err != nil {
		return nil, fmt.Errorf("set node protected %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	node := &Node{ID: id, Label: label, Properties: properties, Protected: protected}
	kg.upsertSemanticNodeIndex(*node)
	return node, nil
}

func (kg *KnowledgeGraph) enqueueAccessHit(hit knowledgeGraphAccessHit) {
	select {
	case kg.accessQueue <- hit:
	default:
		kg.logger.Debug("KG access queue full, dropping update", "hit", hit)
	}
}

func normalizeKnowledgeGraphProperties(properties map[string]string) map[string]string {
	if properties == nil {
		return make(map[string]string)
	}

	safe := make(map[string]string, len(properties))
	for k, v := range properties {
		runes := []rune(v)
		if len(runes) > knowledgeGraphPropertyValueLimit {
			safe[k] = string(runes[:knowledgeGraphPropertyValueLimit-3]) + "..."
			continue
		}
		safe[k] = v
	}
	return safe
}

func sanitizeKnowledgeGraphNodeProperties(properties map[string]string, protected bool) map[string]string {
	safe := normalizeKnowledgeGraphProperties(properties)
	delete(safe, "access_count")
	delete(safe, "protected")
	if protected {
		safe["protected"] = "true"
	}
	return safe
}

func mergeKnowledgeGraphNodes(nodes []Node) []Node {
	merged := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		if node.ID == "" {
			continue
		}
		node.Properties = normalizeKnowledgeGraphProperties(node.Properties)
		existing, ok := merged[node.ID]
		if !ok {
			if node.Properties == nil {
				node.Properties = make(map[string]string)
			}
			merged[node.ID] = node
			continue
		}

		existing.Label = choosePreferredAutoExtractedLabel(existing.Label, node.Label)
		existing.Properties = mergeAutoExtractedProperties(existing.Properties, node.Properties)
		merged[node.ID] = existing
	}
	return sortKnowledgeGraphNodes(merged)
}

func mergeKnowledgeGraphEdges(edges []Edge) []Edge {
	merged := make(map[string]Edge, len(edges))
	for _, edge := range edges {
		if edge.Source == "" || edge.Target == "" || edge.Relation == "" {
			continue
		}
		edge.Properties = normalizeKnowledgeGraphProperties(edge.Properties)
		key := knowledgeGraphEdgeKey(edge.Source, edge.Target, edge.Relation)
		existing, ok := merged[key]
		if !ok {
			if edge.Properties == nil {
				edge.Properties = make(map[string]string)
			}
			merged[key] = edge
			continue
		}
		existing.Properties = mergeAutoExtractedProperties(existing.Properties, edge.Properties)
		merged[key] = existing
	}
	return sortKnowledgeGraphEdges(merged)
}

func mergeAutoExtractedProperties(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		current := strings.TrimSpace(out[key])
		next := strings.TrimSpace(value)
		switch {
		case current == "":
			out[key] = value
		case next == "":
			continue
		case current == next:
			continue
		case len([]rune(next)) > len([]rune(current)):
			out[key] = value
		}
	}
	return out
}

func mergeKnowledgeGraphProperties(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		if strings.TrimSpace(out[key]) == "" && strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func mergeKnowledgeGraphPropertiesOverwrite(existing, incoming map[string]string) map[string]string {
	out := normalizeKnowledgeGraphProperties(existing)
	for key, value := range normalizeKnowledgeGraphProperties(incoming) {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func mergeKnowledgeGraphLabel(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	switch {
	case existing == "" || strings.EqualFold(existing, "unknown"):
		if incoming != "" {
			return incoming
		}
	case incoming == "" || strings.EqualFold(incoming, "unknown"):
		return existing
	}
	if existing == "" {
		return incoming
	}
	return existing
}

func choosePreferredAutoExtractedLabel(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	switch {
	case existing == "" || strings.EqualFold(existing, "unknown"):
		return incoming
	case incoming == "" || strings.EqualFold(incoming, "unknown"):
		return existing
	case len([]rune(incoming)) > len([]rune(existing)):
		return incoming
	default:
		return existing
	}
}

func normalizeKnowledgeGraphDuplicateLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return ""
	}
	return strings.Join(strings.Fields(label), " ")
}

func buildKnowledgeGraphDuplicateCandidates(groups map[string][]Node) []KnowledgeGraphDuplicateCandidate {
	candidates := make([]KnowledgeGraphDuplicateCandidate, 0, len(groups))
	for normalized, nodes := range groups {
		if len(nodes) < 2 {
			continue
		}
		sort.Slice(nodes, func(i, j int) bool {
			left := strings.TrimSpace(nodes[i].Label)
			right := strings.TrimSpace(nodes[j].Label)
			if left != right {
				return left < right
			}
			return nodes[i].ID < nodes[j].ID
		})

		ids := make([]string, 0, len(nodes))
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		candidates = append(candidates, KnowledgeGraphDuplicateCandidate{
			Label:           nodes[0].Label,
			NormalizedLabel: normalized,
			Count:           len(nodes),
			IDs:             ids,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].Label < candidates[j].Label
	})
	return candidates
}

func knowledgeGraphEdgeKey(source, target, relation string) string {
	return source + "\x00" + target + "\x00" + relation
}

func sortKnowledgeGraphNodes(nodes map[string]Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func sortKnowledgeGraphEdges(edges map[string]Edge) []Edge {
	out := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		return knowledgeGraphEdgeKey(out[i].Source, out[i].Target, out[i].Relation) < knowledgeGraphEdgeKey(out[j].Source, out[j].Target, out[j].Relation)
	})
	return out
}

func loadKnowledgeGraphNode(tx *sql.Tx, id string) (label string, properties map[string]string, protected int, found bool, err error) {
	var propsJSON string
	err = tx.QueryRow(`SELECT label, properties, protected FROM kg_nodes WHERE id = ?`, id).Scan(&label, &propsJSON, &protected)
	if err == sql.ErrNoRows {
		return "", make(map[string]string), 0, false, nil
	}
	if err != nil {
		return "", nil, 0, false, err
	}
	properties = make(map[string]string)
	if propsJSON != "" {
		if unmarshalErr := json.Unmarshal([]byte(propsJSON), &properties); unmarshalErr != nil {
			return "", nil, 0, false, fmt.Errorf("unmarshal node properties: %w", unmarshalErr)
		}
	}
	properties = sanitizeKnowledgeGraphNodeProperties(properties, protected != 0)
	return label, properties, protected, true, nil
}

func loadKnowledgeGraphEdge(tx *sql.Tx, source, target, relation string) (properties map[string]string, found bool, err error) {
	var propsJSON string
	err = tx.QueryRow(`SELECT properties FROM kg_edges WHERE source = ? AND target = ? AND relation = ?`, source, target, relation).Scan(&propsJSON)
	if err == sql.ErrNoRows {
		return make(map[string]string), false, nil
	}
	if err != nil {
		return nil, false, err
	}
	properties = make(map[string]string)
	if propsJSON != "" {
		if unmarshalErr := json.Unmarshal([]byte(propsJSON), &properties); unmarshalErr != nil {
			return nil, false, fmt.Errorf("unmarshal edge properties: %w", unmarshalErr)
		}
	}
	return properties, true, nil
}

// escapeFTS5 escapes a query string for use with FTS5 MATCH.
// Wraps each word in quotes to prevent FTS5 syntax errors from special characters.
func escapeFTS5(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}
	var escaped []string
	for _, w := range words {
		// Remove any existing quotes, then wrap in quotes
		w = strings.ReplaceAll(w, `"`, ``)
		if w != "" {
			escaped = append(escaped, `"`+w+`"`)
		}
	}
	if len(escaped) == 0 {
		return `""`
	}
	return strings.Join(escaped, " OR ")
}

func decodeKnowledgeGraphNodeProperties(logger *slog.Logger, scope, nodeID, propsJSON string, protected int) map[string]string {
	properties := make(map[string]string)
	if propsJSON != "" {
		if err := json.Unmarshal([]byte(propsJSON), &properties); err != nil {
			logger.Warn(scope+": corrupt node properties JSON", "id", nodeID, "error", err)
			properties = make(map[string]string)
		}
	}
	return sanitizeKnowledgeGraphNodeProperties(properties, protected != 0)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

type kgBFSLevel struct {
	nodeID string
	depth  int
}

func (kg *KnowledgeGraph) GetSubgraph(centerNodeID string, maxDepth int) ([]Node, []Edge) {
	if kg == nil || maxDepth <= 0 || strings.TrimSpace(centerNodeID) == "" {
		return nil, nil
	}
	if maxDepth > 3 {
		maxDepth = 3
	}

	center, err := kg.GetNode(centerNodeID)
	if err != nil || center == nil {
		return nil, nil
	}

	visited := make(map[string]bool)
	allNodes := make(map[string]Node)
	allEdges := make(map[string]Edge)
	allNodes[centerNodeID] = *center
	visited[centerNodeID] = true

	queue := []kgBFSLevel{{centerNodeID, 0}}
	for len(queue) > 0 {
		var levelNodeIDs []string
		maxDepthInLevel := queue[0].depth
		for _, item := range queue {
			if item.depth >= maxDepth {
				continue
			}
			levelNodeIDs = append(levelNodeIDs, item.nodeID)
		}
		if len(levelNodeIDs) == 0 {
			break
		}

		var discoveredEdges []Edge
		var neighborIDs []string
		// Batch all level nodes into a single IN(...) query instead of one query per node.
		placeholders := make([]string, len(levelNodeIDs))
		batchArgs := make([]interface{}, len(levelNodeIDs)*2)
		for i, nid := range levelNodeIDs {
			placeholders[i] = "?"
			batchArgs[i] = nid
			batchArgs[len(levelNodeIDs)+i] = nid
		}
		batchEdgeQuery := fmt.Sprintf(
			`SELECT source, target, relation, properties FROM kg_edges WHERE source IN (%s) OR target IN (%s)`,
			strings.Join(placeholders, ","),
			strings.Join(placeholders, ","),
		)
		batchRows, batchErr := kg.db.Query(batchEdgeQuery, batchArgs...)
		if batchErr != nil {
			kg.logger.Warn("GetSubgraph: batch edge query failed", "error", batchErr)
		} else {
			defer batchRows.Close()
			for batchRows.Next() {
				var e Edge
				var propsJSON string
				if batchRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON) == nil {
					json.Unmarshal([]byte(propsJSON), &e.Properties)
					if e.Properties == nil {
						e.Properties = make(map[string]string)
					}
					edgeKey := knowledgeGraphEdgeKey(e.Source, e.Target, e.Relation)
					if _, exists := allEdges[edgeKey]; !exists {
						allEdges[edgeKey] = e
						discoveredEdges = append(discoveredEdges, e)
					}
					// visited already contains all levelNodeIDs, so this correctly
					// identifies only new (not-yet-seen) neighbor nodes.
					if !visited[e.Source] {
						neighborIDs = append(neighborIDs, e.Source)
					}
					if !visited[e.Target] {
						neighborIDs = append(neighborIDs, e.Target)
					}
				}
			}
		}

		if len(neighborIDs) == 0 {
			break
		}

		uniqueNeighborIDs := make([]string, 0, len(neighborIDs))
		seen := make(map[string]bool, len(neighborIDs))
		for _, id := range neighborIDs {
			if !seen[id] && !visited[id] {
				seen[id] = true
				visited[id] = true
				uniqueNeighborIDs = append(uniqueNeighborIDs, id)
			}
		}

		batchNodes := kg.batchGetNodes(uniqueNeighborIDs)
		for _, n := range batchNodes {
			allNodes[n.ID] = n
			visited[n.ID] = true
		}

		queue = make([]kgBFSLevel, 0, len(uniqueNeighborIDs))
		for _, id := range uniqueNeighborIDs {
			if visited[id] {
				queue = append(queue, kgBFSLevel{id, maxDepthInLevel + 1})
			}
		}
	}

	nodes := make([]Node, 0, len(allNodes))
	for _, n := range allNodes {
		nodes = append(nodes, n)
	}
	edgeList := make([]Edge, 0, len(allEdges))
	for _, e := range allEdges {
		edgeList = append(edgeList, e)
	}
	return nodes, edgeList
}

func (kg *KnowledgeGraph) batchGetNodes(ids []string) []Node {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (%s)", strings.Join(placeholders, ","))
	rows, err := kg.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if rows.Scan(&n.ID, &n.Label, &propsJSON, &protected) == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "batchGetNodes", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// GetRecentChanges returns recently updated nodes.
func (kg *KnowledgeGraph) GetRecentChanges(since time.Time) ([]Node, error) {
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected 
		FROM kg_nodes 
		WHERE updated_at >= ? 
		ORDER BY updated_at DESC LIMIT 50
	`, since)
	if err != nil {
		return nil, fmt.Errorf("query recent changes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
			var props map[string]string
			if propsJSON != "" {
				if unmarshalErr := json.Unmarshal([]byte(propsJSON), &props); unmarshalErr != nil {
					kg.logger.Warn("GetRecentChanges: corrupt node properties JSON", "id", n.ID, "error", unmarshalErr)
					props = make(map[string]string)
				}
			}
			n.Properties = sanitizeKnowledgeGraphNodeProperties(props, protected != 0)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// MergeNodes merges two nodes, transferring edges and dropping the source node.
func (kg *KnowledgeGraph) MergeNodes(targetID, sourceID string) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin merge nodes: %w", err)
	}
	defer tx.Rollback()

	// Update edges pointing to source
	_, err = tx.Exec("UPDATE kg_edges SET target = ? WHERE target = ?", targetID, sourceID)
	if err != nil {
		return fmt.Errorf("update edges target: %w", err)
	}

	_, err = tx.Exec("UPDATE kg_edges SET source = ? WHERE source = ?", targetID, sourceID)
	if err != nil {
		return fmt.Errorf("update edges source: %w", err)
	}

	// Update access count of target
	var sourceAccess int
	err = tx.QueryRow("SELECT access_count FROM kg_nodes WHERE id = ?", sourceID).Scan(&sourceAccess)
	if err == nil && sourceAccess > 0 {
		_, err = tx.Exec("UPDATE kg_nodes SET access_count = access_count + ? WHERE id = ?", sourceAccess, targetID)
		if err != nil {
			return fmt.Errorf("update target access count: %w", err)
		}
	}

	// Delete source
	_, err = tx.Exec("DELETE FROM kg_nodes WHERE id = ?", sourceID)
	if err != nil {
		return fmt.Errorf("delete source node: %w", err)
	}

	return tx.Commit()
}
