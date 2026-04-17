package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

func (kg *KnowledgeGraph) FindOrphanedFileSyncEntities(activeFiles []string) ([]Node, []Edge, error) {
	activeMap := make(map[string]struct{}, len(activeFiles))
	for _, f := range activeFiles {
		activeMap[f] = struct{}{}
	}

	var orphanNodes []Node
	nodeRows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		  AND json_extract(properties, '$.source_file') IS NOT NULL
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("query nodes for orphan check: %w", err)
	}
	defer nodeRows.Close()
	for nodeRows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := nodeRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "FindOrphanedFileSyncEntities", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			if sourceFile := n.Properties["source_file"]; sourceFile != "" {
				if _, ok := activeMap[sourceFile]; !ok {
					orphanNodes = append(orphanNodes, n)
				}
			}
		}
	}

	var orphanEdges []Edge
	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE json_extract(properties, '$.source') = 'file_sync'
		  AND json_extract(properties, '$.source_file') IS NOT NULL
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("query edges for orphan check: %w", err)
	}
	defer edgeRows.Close()
	for edgeRows.Next() {
		var e Edge
		var propsJSON string
		if err := edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("FindOrphanedFileSyncEntities: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
				e.Properties = make(map[string]string)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			if sourceFile := e.Properties["source_file"]; sourceFile != "" {
				if _, ok := activeMap[sourceFile]; !ok {
					orphanEdges = append(orphanEdges, e)
				}
			}
		}
	}

	return orphanNodes, orphanEdges, nil
}

func (kg *KnowledgeGraph) GetSourceFilesByNodeID(nodeID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	seen := make(map[string]struct{})
	var files []string

	var nodePropsJSON string
	err := kg.db.QueryRow("SELECT properties FROM kg_nodes WHERE id = ?", nodeID).Scan(&nodePropsJSON)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query node properties: %w", err)
	}
	if err == nil && nodePropsJSON != "" {
		var props map[string]string
		if json.Unmarshal([]byte(nodePropsJSON), &props) == nil {
			if sf := strings.TrimSpace(props["source_file"]); sf != "" {
				seen[sf] = struct{}{}
				files = append(files, sf)
			}
		}
	}
	if len(files) >= limit {
		return files, nil
	}

	rows, err := kg.db.Query(`
		SELECT properties FROM kg_edges
		WHERE source = ? OR target = ?
	`, nodeID, nodeID)
	if err != nil {
		return files, fmt.Errorf("query edges for node: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var propsJSON string
		if err := rows.Scan(&propsJSON); err != nil {
			continue
		}
		var props map[string]string
		if json.Unmarshal([]byte(propsJSON), &props) != nil {
			continue
		}
		if sf := strings.TrimSpace(props["source_file"]); sf != "" {
			if _, ok := seen[sf]; !ok {
				seen[sf] = struct{}{}
				files = append(files, sf)
				if len(files) >= limit {
					break
				}
			}
		}
	}

	return files, nil
}

func (kg *KnowledgeGraph) GetFileSyncStats() (*FileSyncStats, error) {
	stats := &FileSyncStats{
		ByEntityType: make(map[string]int),
		ByCollection: make(map[string]int),
	}

	kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
	`).Scan(&stats.NodeCount)

	kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges
		WHERE json_extract(properties, '$.source') = 'file_sync'
	`).Scan(&stats.EdgeCount)

	typeRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.type'), ''), 'untyped') AS t, COUNT(*)
		FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		GROUP BY t
	`)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var t string
			var c int
			if typeRows.Scan(&t, &c) == nil {
				stats.ByEntityType[t] = c
			}
		}
	}

	collRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.collection'), ''), 'default') AS c, COUNT(*)
		FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		GROUP BY c
	`)
	if err == nil {
		defer collRows.Close()
		for collRows.Next() {
			var c string
			var cnt int
			if collRows.Scan(&c, &cnt) == nil {
				stats.ByCollection[c] = cnt
			}
		}
	}

	return stats, nil
}

func (kg *KnowledgeGraph) GetCollectionFileSyncStats(collection string) (*KGCollectionStats, error) {
	stats := &KGCollectionStats{}

	kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		AND json_extract(properties, '$.collection') = ?
	`, collection).Scan(&stats.NodeCount)

	kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges
		WHERE json_extract(properties, '$.source') = 'file_sync'
		AND json_extract(properties, '$.collection') = ?
	`, collection).Scan(&stats.EdgeCount)

	kg.db.QueryRow(`
		SELECT COUNT(DISTINCT json_extract(properties, '$.source_file')) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		AND json_extract(properties, '$.collection') = ?
	`, collection).Scan(&stats.FileCount)

	var lastSync string
	err := kg.db.QueryRow(`
		SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		AND json_extract(properties, '$.collection') = ?
	`, collection).Scan(&lastSync)
	if err == nil && lastSync != "" {
		if t, err := time.Parse("2006-01-02", lastSync); err == nil {
			stats.LastSyncAt = &t
		}
	}

	return stats, nil
}

func (kg *KnowledgeGraph) GetLastFileSyncTime(collection string) (*time.Time, error) {
	var lastSync string
	var query string
	var args []interface{}

	if collection == "" {
		query = `SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes WHERE json_extract(properties, '$.source') = 'file_sync'`
	} else {
		query = `SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes WHERE json_extract(properties, '$.source') = 'file_sync' AND json_extract(properties, '$.collection') = ?`
		args = append(args, collection)
	}

	err := kg.db.QueryRow(query, args...).Scan(&lastSync)
	if err != nil || lastSync == "" {
		return nil, err
	}

	t, err := time.Parse("2006-01-02", lastSync)
	if err != nil {
		return nil, fmt.Errorf("parse last sync time: %w", err)
	}
	return &t, nil
}
