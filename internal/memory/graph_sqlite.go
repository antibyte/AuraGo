package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Node represents an entity in the knowledge graph.
type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Properties map[string]string `json:"properties"`
}

// Edge represents a relationship between two nodes.
type Edge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Relation   string            `json:"relation"`
	Properties map[string]string `json:"properties"`
}

// KnowledgeGraph implements the same interface as the old JSON-backed graph
// but stores all data in SQLite with FTS5 for full-text search.
type KnowledgeGraph struct {
	db          *sql.DB
	logger      *slog.Logger
	accessQueue chan string   // buffered channel for async access-count updates
	doneChan    chan struct{} // signals worker to exit
	closeOnce   sync.Once     // ensures Close is only called once
}

const knowledgeGraphWriteTimeout = 5 * time.Second

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

	kg := &KnowledgeGraph{
		db:          db,
		logger:      logger,
		accessQueue: make(chan string, 1000), // buffer for 1000 node IDs
		doneChan:    make(chan struct{}),
	}
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
		kg.drainAccessQueue()
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

// accessCountWorker drains the access queue and increments node access counts.
// Runs as a single goroutine for the lifetime of the KnowledgeGraph.
// Exits when doneChan is closed or when db is closed.
func (kg *KnowledgeGraph) accessCountWorker() {
	for {
		select {
		case <-kg.doneChan:
			return
		case id, ok := <-kg.accessQueue:
			if !ok {
				return // channel closed
			}
			ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphWriteTimeout)
			_, execErr := kg.db.ExecContext(ctx, "UPDATE kg_nodes SET access_count = access_count + 1 WHERE id = ?", id)
			cancel()
			if execErr != nil {
				kg.logger.Warn("KG access count update failed", "id", id, "error", execErr)
			}
		}
	}
}

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
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS kg_edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		relation TEXT NOT NULL,
		properties TEXT NOT NULL DEFAULT '{}',
		access_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source, target, relation)
	);

	CREATE INDEX IF NOT EXISTS idx_kg_edges_source ON kg_edges(source);
	CREATE INDEX IF NOT EXISTS idx_kg_edges_target ON kg_edges(target);
	`
	if _, err := kg.db.Exec(schema); err != nil {
		return fmt.Errorf("create kg tables: %w", err)
	}

	// FTS5 virtual table for full-text search on nodes
	// Note: content_rowid references the explicit rowid column which is INTEGER PRIMARY KEY
	fts := `CREATE VIRTUAL TABLE IF NOT EXISTS kg_nodes_fts USING fts5(
		id, label, properties_text, content=kg_nodes, content_rowid=rowid
	);`
	if _, err := kg.db.Exec(fts); err != nil {
		return fmt.Errorf("create kg FTS5 table: %w", err)
	}

	// Triggers to keep FTS index in sync
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS kg_nodes_ai AFTER INSERT ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(rowid, id, label, properties_text)
			VALUES (new.rowid, new.id, new.label, new.properties);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS kg_nodes_ad AFTER DELETE ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(kg_nodes_fts, rowid, id, label, properties_text)
			VALUES ('delete', old.rowid, old.id, old.label, old.properties);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS kg_nodes_au AFTER UPDATE ON kg_nodes BEGIN
			INSERT INTO kg_nodes_fts(kg_nodes_fts, rowid, id, label, properties_text)
			VALUES ('delete', old.rowid, old.id, old.label, old.properties);
			INSERT INTO kg_nodes_fts(rowid, id, label, properties_text)
			VALUES (new.rowid, new.id, new.label, new.properties);
		END;`,
	}
	for _, t := range triggers {
		if _, err := kg.db.Exec(t); err != nil {
			return fmt.Errorf("create kg FTS trigger: %w", err)
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

// AddNode adds or updates a node in the knowledge graph.
func (kg *KnowledgeGraph) AddNode(id, label string, properties map[string]string) error {
	if properties == nil {
		properties = make(map[string]string)
	}

	// Copy to avoid mutating the caller's map during property truncation.
	safe := make(map[string]string, len(properties))
	for k, v := range properties {
		if len(v) > 50 {
			safe[k] = v[:47] + "..."
		} else {
			safe[k] = v
		}
	}
	properties = safe

	propsJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal node properties: %w", err)
	}

	isProtected := 0
	if properties["protected"] == "true" {
		isProtected = 1
	}

	_, err = kg.db.Exec(`
		INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			label = excluded.label,
			properties = excluded.properties,
			protected = excluded.protected,
			updated_at = CURRENT_TIMESTAMP
	`, id, label, string(propsJSON), isProtected)
	if err != nil {
		return fmt.Errorf("add node: %w", err)
	}
	return nil
}

// AddEdge adds or updates an edge. Auto-creates missing endpoint nodes.
func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	if properties == nil {
		properties = make(map[string]string)
	}

	// Copy to avoid mutating the caller's map during property truncation.
	safe := make(map[string]string, len(properties))
	for k, v := range properties {
		if len(v) > 50 {
			safe[k] = v[:47] + "..."
		} else {
			safe[k] = v
		}
	}
	properties = safe

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin add edge: %w", err)
	}
	defer tx.Rollback()

	// Ensure endpoint nodes exist
	for _, id := range []string{source, target} {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO kg_nodes (id, label, properties) VALUES (?, 'Unknown', '{}')`, id); err != nil {
			kg.logger.Warn("AddEdge: failed to ensure node exists", "id", id, "error", err)
		}
	}

	propsJSON, _ := json.Marshal(properties)
	_, err = tx.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties
	`, source, target, relation, string(propsJSON))
	if err != nil {
		return fmt.Errorf("add edge: %w", err)
	}

	return tx.Commit()
}

// Search returns a JSON string of nodes and edges matching the query.
// Uses a single UNION query combining FTS5 and LIKE to avoid two database round-trips.
// Access counts are updated asynchronously via a worker pool.
func (kg *KnowledgeGraph) Search(query string) string {
	if query == "" {
		return "[]"
	}

	var matchedNodes []Node
	var matchedEdges []Edge
	var matchedNodeIDs []string

	// Combined FTS5 + LIKE UNION: one round-trip instead of two.
	// FTS5 results are preferred; LIKE catches non-indexed partial matches.
	ftsQuery := escapeFTS5(query)
	likePattern := "%" + query + "%"
	rows, err := kg.db.Query(`
		SELECT id, label, properties FROM kg_nodes
		WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?)
		UNION
		SELECT id, label, properties FROM kg_nodes
		WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
		LIMIT 50
	`, ftsQuery, likePattern, likePattern, likePattern)
	if err == nil {
		for rows.Next() {
			var n Node
			var propsJSON string
			if err := rows.Scan(&n.ID, &n.Label, &propsJSON); err == nil {
				if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
					kg.logger.Warn("Search: corrupt node properties JSON", "id", n.ID, "error", err)
				}
				if n.Properties == nil {
					n.Properties = make(map[string]string)
				}
				matchedNodes = append(matchedNodes, n)
				matchedNodeIDs = append(matchedNodeIDs, n.ID)
			}
		}
		rows.Close()
	}

	// Edge search — substring match on source, target, relation
	likeQ := "%" + strings.ToLower(query) + "%"
	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE LOWER(source) LIKE ? OR LOWER(target) LIKE ? OR LOWER(relation) LIKE ?
		LIMIT 50
	`, likeQ, likeQ, likeQ)
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
		select {
		case kg.accessQueue <- id:
		default:
			kg.logger.Debug("KG access queue full, dropping update", "id", id)
		}
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
		err := kg.db.QueryRow("SELECT id, label, properties FROM kg_nodes WHERE id = ?", id).Scan(&n.ID, &n.Label, &propsJSON)
		if err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
				kg.logger.Warn("GetNeighbors: corrupt node properties JSON", "id", n.ID, "error", err)
			}
			if n.Properties == nil {
				n.Properties = make(map[string]string)
			}
			nodes = append(nodes, n)
		}
	}

	return nodes, edges
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
			ORDER BY access_count DESC
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

	if len(nodeIDs) == 0 {
		return ""
	}

	// Build context: for each node, include label + 1-hop edges
	var sb strings.Builder
	for _, nid := range nodeIDs {
		var label, propsJSON string
		err := kg.db.QueryRow("SELECT label, properties FROM kg_nodes WHERE id = ?", nid).Scan(&label, &propsJSON)
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("- %s (%s)", nid, label))

		// Parse properties for relevant context
		var props map[string]string
		json.Unmarshal([]byte(propsJSON), &props)
		for k, v := range props {
			if k == "access_count" || k == "protected" {
				continue
			}
			sb.WriteString(fmt.Sprintf(" [%s: %s]", k, v))
		}
		sb.WriteString("\n")

		// 1-hop edges
		edgeRows, err := kg.db.Query(`
			SELECT source, target, relation FROM kg_edges
			WHERE source = ? OR target = ?
			LIMIT 5
		`, nid, nid)
		if err == nil {
			for edgeRows.Next() {
				var src, tgt, rel string
				if edgeRows.Scan(&src, &tgt, &rel) == nil {
					sb.WriteString(fmt.Sprintf("  -> %s -[%s]-> %s\n", src, rel, tgt))
				}
			}
			edgeRows.Close()
		}

		if sb.Len() > maxChars {
			break
		}
	}

	result := sb.String()
	if len(result) > maxChars {
		result = result[:maxChars]
	}
	return result
}

// DeleteNode removes a node and all its connected edges.
func (kg *KnowledgeGraph) DeleteNode(id string) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete node: %w", err)
	}
	defer tx.Rollback()

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
	rows, err := kg.db.Query("SELECT id, label, properties FROM kg_nodes ORDER BY access_count DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
				kg.logger.Warn("GetAllNodes: corrupt node properties JSON", "id", n.ID, "error", err)
			}
			if n.Properties == nil {
				n.Properties = make(map[string]string)
			}
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
	for _, n := range nodes {
		if n.ID == "" {
			continue
		}
		if n.Properties == nil {
			n.Properties = make(map[string]string)
		}
		propsJSON, _ := json.Marshal(n.Properties)
		isProtected := 0
		if n.Properties["protected"] == "true" {
			isProtected = 1
		}
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

	return tx.Commit()
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
