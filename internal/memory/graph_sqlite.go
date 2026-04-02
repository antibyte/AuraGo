package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open knowledge graph db: %w", err)
	}

	// SQLite is single-writer; cap connections to prevent locking errors
	db.SetMaxOpenConns(1)
	// Retry for up to 5 s when another writer holds the lock (prevents SQLITE_BUSY).
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		logger.Warn("KG: failed to set busy_timeout", "error", err)
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
		case <-kg.accessQueue:
			// drain
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
	for {
		select {
		case <-kg.doneChan:
			return
		case hit, ok := <-kg.accessQueue:
			if !ok {
				return // channel closed
			}
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
				kg.logger.Warn("KG access count update failed", "hit", hit, "error", execErr)
			}
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
		semantic_indexed_at DATETIME
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

	migrations := []string{
		`ALTER TABLE kg_nodes ADD COLUMN semantic_indexed_at DATETIME`,
		`ALTER TABLE kg_edges ADD COLUMN semantic_indexed_at DATETIME`,
	}
	for _, m := range migrations {
		kg.db.Exec(m)
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
		propsJSON, _ := json.Marshal(n.Properties)
		accessCount := 0
		if countStr, ok := n.Properties["access_count"]; ok {
			fmt.Sscanf(countStr, "%d", &accessCount)
		}
		isProtected := 0
		if n.Properties["protected"] == "true" {
			isProtected = 1
		}
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO kg_nodes (id, label, properties, access_count, protected) VALUES (?, ?, ?, ?, ?)",
			n.ID, n.Label, string(propsJSON), accessCount, isProtected,
		)
		if err == nil {
			migrated++
		}
	}
	for _, e := range state.Edges {
		propsJSON, _ := json.Marshal(e.Properties)
		if _, err := tx.Exec(
			"INSERT OR IGNORE INTO kg_edges (source, target, relation, properties) VALUES (?, ?, ?, ?)",
			e.Source, e.Target, e.Relation, string(propsJSON),
		); err != nil {
			kg.logger.Warn("[KG] Failed to migrate edge", "source", e.Source, "target", e.Target, "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		kg.logger.Error("[KG] Migration commit failed", "error", err)
		return
	}

	if renameErr := os.Rename(jsonPath, jsonPath+".migrated"); renameErr != nil {
		kg.logger.Warn("[KG] Could not rename migrated JSON file", "error", renameErr)
	}
	kg.logger.Info("[KG] Migrated graph.json to SQLite", "nodes", migrated, "edges", len(state.Edges))
}

// Stats returns the number of nodes and edges in the knowledge graph.
func (kg *KnowledgeGraph) Stats() (nodes int, edges int, err error) {
	if scanErr := kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&nodes); scanErr != nil {
		kg.logger.Warn("KG Stats: failed to count nodes", "error", scanErr)
		nodes = -1
	}
	if scanErr := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&edges); scanErr != nil {
		kg.logger.Warn("KG Stats: failed to count edges", "error", scanErr)
		edges = -1
	}
	return nodes, edges, nil
}

// QualityReport returns simple dashboard-friendly quality diagnostics for the graph.
func (kg *KnowledgeGraph) QualityReport(sampleLimit int) (*KnowledgeGraphQualityReport, error) {
	if sampleLimit <= 0 {
		sampleLimit = 5
	}
	if sampleLimit > 50 {
		sampleLimit = 50
	}

	rows, err := kg.db.Query(`SELECT id, label, properties, protected FROM kg_nodes ORDER BY label COLLATE NOCASE, id COLLATE NOCASE LIMIT 5000`)
	if err != nil {
		return nil, fmt.Errorf("query kg nodes for quality report: %w", err)
	}
	defer rows.Close()

	nodes := make([]Node, 0, 128)
	duplicateGroups := make(map[string][]Node)
	for rows.Next() {
		var (
			node      Node
			propsJSON string
			protected int
		)
		if err := rows.Scan(&node.ID, &node.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan kg node for quality report: %w", err)
		}
		node.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", node.ID, propsJSON, protected)
		node.Protected = protected != 0
		nodes = append(nodes, node)

		labelKey := normalizeKnowledgeGraphDuplicateLabel(node.Label)
		if labelKey != "" {
			duplicateGroups[labelKey] = append(duplicateGroups[labelKey], node)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kg nodes for quality report: %w", err)
	}

	edgeRows, err := kg.db.Query(`SELECT source, target FROM kg_edges`)
	if err != nil {
		return nil, fmt.Errorf("query kg edges for quality report: %w", err)
	}
	defer edgeRows.Close()

	degrees := make(map[string]int, len(nodes))
	edgeCount := 0
	for edgeRows.Next() {
		var source, target string
		if err := edgeRows.Scan(&source, &target); err != nil {
			return nil, fmt.Errorf("scan kg edge for quality report: %w", err)
		}
		edgeCount++
		degrees[source]++
		degrees[target]++
	}
	if err := edgeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kg edges for quality report: %w", err)
	}

	report := &KnowledgeGraphQualityReport{
		Nodes:          len(nodes),
		Edges:          edgeCount,
		IsolatedSample: make([]Node, 0, sampleLimit),
		UntypedSample:  make([]Node, 0, sampleLimit),
	}

	for _, node := range nodes {
		if node.Protected {
			report.ProtectedNodes++
		}
		if degrees[node.ID] == 0 {
			report.IsolatedNodes++
			if len(report.IsolatedSample) < sampleLimit {
				report.IsolatedSample = append(report.IsolatedSample, node)
			}
		}
		if strings.TrimSpace(node.Properties["type"]) == "" {
			report.UntypedNodes++
			if len(report.UntypedSample) < sampleLimit {
				report.UntypedSample = append(report.UntypedSample, node)
			}
		}
	}

	allDuplicateCandidates := buildKnowledgeGraphDuplicateCandidates(duplicateGroups)
	report.DuplicateGroups = len(allDuplicateCandidates)
	for _, candidate := range allDuplicateCandidates {
		report.DuplicateNodes += candidate.Count
	}
	if len(allDuplicateCandidates) > sampleLimit {
		report.DuplicateCandidates = allDuplicateCandidates[:sampleLimit]
	} else {
		report.DuplicateCandidates = allDuplicateCandidates
	}

	return report, nil
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

const coOccurrenceThreshold = 3

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
	var currentWeight int
	var propsJSON string
	err := kg.db.QueryRow(
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
		_, err = kg.db.Exec(
			"UPDATE kg_edges SET properties = ? WHERE source = ? AND target = ? AND relation = 'co_mentioned_with'",
			string(newPropsJSON), a, b,
		)
		return err
	}

	// Edge does not exist yet — insert as pending (weight=1, below threshold).
	initProps, _ := json.Marshal(map[string]string{
		"source": "pending",
		"weight": "1",
		"date":   date,
	})
	_, err = kg.db.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties)
		VALUES (?, ?, 'co_mentioned_with', ?)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties
	`, a, b, string(initProps))
	return err
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
	ftsQuery := escapeFTS5(query)
	likePattern := "%" + query + "%"
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?)
		UNION
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
		LIMIT 50
	`, ftsQuery, likePattern, likePattern, likePattern)
	if err == nil {
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
		rows.Close()
	}

	// Edge search — substring match on source, target, relation
	likeQ := "%" + strings.ToLower(query) + "%"
	edgeFTSQuery := escapeFTS5(query)
	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE id IN (SELECT rowid FROM kg_edges_fts WHERE kg_edges_fts MATCH ?)
		UNION
		SELECT source, target, relation, properties FROM kg_edges
		WHERE LOWER(source) LIKE ? OR LOWER(target) LIKE ? OR LOWER(relation) LIKE ? OR LOWER(properties) LIKE ?
		LIMIT 50
	`, edgeFTSQuery, likeQ, likeQ, likeQ, likeQ)
	if err == nil {
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
		edgeRows.Close()
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
	for id := range neighborIDs {
		var n Node
		var propsJSON string
		var protected int
		err := kg.db.QueryRow("SELECT id, label, properties, protected FROM kg_nodes WHERE id = ?", id).Scan(&n.ID, &n.Label, &propsJSON, &protected)
		if err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNeighbors", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
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
		maxChars = 800
	}

	// Find relevant nodes via FTS5 or LIKE fallback
	var nodeIDs []string
	ftsQuery := escapeFTS5(query)
	rows, err := kg.db.Query(`
		SELECT n.id FROM kg_nodes_fts f
		JOIN kg_nodes n ON n.rowid = f.rowid
		WHERE kg_nodes_fts MATCH ?
		ORDER BY n.updated_at DESC
		LIMIT ?
	`, ftsQuery, maxNodes)
	if err == nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				nodeIDs = append(nodeIDs, id)
			}
		}
		rows.Close()
	}

	// Fallback to LIKE if FTS5 found nothing
	if len(nodeIDs) == 0 {
		likeQ := "%" + query + "%"
		rows, err := kg.db.Query(`
			SELECT id FROM kg_nodes
			WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
			ORDER BY updated_at DESC, access_count DESC
			LIMIT ?
		`, likeQ, likeQ, likeQ, maxNodes)
		if err == nil {
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					nodeIDs = append(nodeIDs, id)
				}
			}
			rows.Close()
		}
	}

	if len(nodeIDs) < maxNodes {
		seen := make(map[string]bool, len(nodeIDs))
		for _, id := range nodeIDs {
			seen[id] = true
		}
		for _, id := range kg.semanticSearchNodeIDs(query, maxNodes) {
			if seen[id] {
				continue
			}
			nodeIDs = append(nodeIDs, id)
			seen[id] = true
			if len(nodeIDs) >= maxNodes {
				break
			}
		}
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
		if err == nil {
			for edgeRows.Next() {
				var src, tgt, rel string
				if edgeRows.Scan(&src, &tgt, &rel) == nil {
					sb.WriteString(fmt.Sprintf("  - [%s] -[%s]-> [%s]\n", src, rel, tgt))
					accessHits = append(accessHits, knowledgeGraphAccessHit{
						source: src, target: tgt, relation: rel,
					})
				}
			}
			edgeRows.Close()
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
	rows.Close()

	if len(toRemove) == 0 {
		return 0, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for _, id := range toRemove {
		tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id)
		tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(toRemove), nil
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
				label = excluded.label,
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
		if batchErr == nil {
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
			batchRows.Close()
		}

		if len(neighborIDs) == 0 {
			break
		}

		uniqueNeighborIDs := make([]string, 0, len(neighborIDs))
		seen := make(map[string]bool, len(neighborIDs))
		for _, id := range neighborIDs {
			if !seen[id] && !visited[id] {
				seen[id] = true
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
