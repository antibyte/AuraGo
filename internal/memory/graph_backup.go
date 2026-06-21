package memory

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	indexEdges := make([]Edge, 0, len(edges))
	var bulkErrors []error
	for _, n := range nodes {
		if n.ID == "" {
			continue
		}
		existingLabel, existingProps, existingProtected, _, err := loadKnowledgeGraphNode(tx, n.ID)
		if err != nil {
			return fmt.Errorf("load existing node %q for bulk add: %w", n.ID, err)
		}

		n.Properties = sanitizeKnowledgeGraphNodeProperties(n.Properties, strings.EqualFold(strings.TrimSpace(n.Properties["protected"]), "true"))
		finalLabel := mergeKnowledgeGraphLabel(existingLabel, n.Label)
		finalProps := mergeKnowledgeGraphPropertiesOverwrite(existingProps, n.Properties)
		isProtected := existingProtected
		if finalProps["protected"] == "true" {
			isProtected = 1
		}
		finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, isProtected != 0)
		finalProps = validateNodeSchema(finalProps)
		propsJSON, err := json.Marshal(finalProps)
		if err != nil {
			return fmt.Errorf("marshal bulk add node properties: %w", err)
		}
		if _, execErr := tx.Exec(`
			INSERT INTO kg_nodes (id, label, properties, protected, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				label = excluded.label,
				properties = excluded.properties,
				protected = excluded.protected,
				updated_at = excluded.updated_at
		`, n.ID, finalLabel, string(propsJSON), isProtected, now); execErr != nil {
			bulkErrors = append(bulkErrors, fmt.Errorf("insert node %q: %w", n.ID, execErr))
			continue
		}
		indexNodes = append(indexNodes, Node{ID: n.ID, Label: finalLabel, Properties: finalProps, Protected: isProtected != 0})
	}

	for _, e := range edges {
		if e.Source == "" || e.Target == "" || e.Relation == "" {
			continue
		}
		for _, id := range []string{e.Source, e.Target} {
			if execErr := ensureKnowledgeGraphPlaceholderNodeTx(tx, id); execErr != nil {
				bulkErrors = append(bulkErrors, fmt.Errorf("ensure endpoint node %q for edge %q->%q: %w", id, e.Source, e.Target, execErr))
			}
		}
		if e.Properties == nil {
			e.Properties = make(map[string]string)
		}
		e.Properties = normalizeKnowledgeGraphProperties(e.Properties)
		existingProps, _, err := loadKnowledgeGraphEdge(tx, e.Source, e.Target, e.Relation)
		if err != nil {
			return fmt.Errorf("load existing edge %q->%q/%q for bulk add: %w", e.Source, e.Target, e.Relation, err)
		}
		finalProps := mergeKnowledgeGraphPropertiesOverwrite(existingProps, e.Properties)
		propsJSON, err := json.Marshal(finalProps)
		if err != nil {
			return fmt.Errorf("marshal bulk add edge properties: %w", err)
		}
		if _, execErr := tx.Exec(`
			INSERT INTO kg_edges (source, target, relation, properties, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(source, target, relation) DO UPDATE SET
				properties = excluded.properties,
				updated_at = excluded.updated_at
		`, e.Source, e.Target, e.Relation, string(propsJSON), now); execErr != nil {
			bulkErrors = append(bulkErrors, fmt.Errorf("insert edge %q->%q/%q: %w", e.Source, e.Target, e.Relation, execErr))
			continue
		}
		indexEdges = append(indexEdges, Edge{Source: e.Source, Target: e.Target, Relation: e.Relation, Properties: finalProps})
	}

	if len(bulkErrors) > 0 {
		return fmt.Errorf("bulk add entities failed: %w", errors.Join(bulkErrors...))
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

func (kg *KnowledgeGraph) BulkMergeExtractedEntities(nodes []Node, edges []Edge) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin bulk merge: %w", err)
	}
	defer tx.Rollback()

	indexNodes, indexEdges, err := kg.mergeExtractedEntitiesTx(tx, nodes, edges)
	if err != nil {
		return err
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

func (kg *KnowledgeGraph) ReplaceExtractedEntitiesBySourceFile(path string, nodes []Node, edges []Edge) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return kg.BulkMergeExtractedEntities(nodes, edges)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin replace extracted entities by source file: %w", err)
	}
	defer tx.Rollback()

	removedEdges := kg.collectSemanticEdgeIdentities(tx, `
		SELECT source, target, relation FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path)
	if _, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path); err != nil {
		return fmt.Errorf("delete stale source-file edges: %w", err)
	}

	keepIDs := make(map[string]struct{}, len(nodes))
	for _, node := range mergeKnowledgeGraphNodes(nodes) {
		if node.ID != "" {
			keepIDs[node.ID] = struct{}{}
		}
	}

	candidateRows, err := tx.Query(`
		SELECT id FROM kg_nodes
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
		  AND protected = 0
	`, path)
	if err != nil {
		return fmt.Errorf("query stale source-file nodes: %w", err)
	}
	var candidateIDs []string
	for candidateRows.Next() {
		var id string
		if err := candidateRows.Scan(&id); err != nil {
			candidateRows.Close()
			return fmt.Errorf("scan stale source-file node: %w", err)
		}
		if _, keep := keepIDs[id]; keep {
			continue
		}
		candidateIDs = append(candidateIDs, id)
	}
	if err := candidateRows.Err(); err != nil {
		candidateRows.Close()
		return fmt.Errorf("iterate stale source-file nodes: %w", err)
	}
	candidateRows.Close()

	var deleteIDs []string
	for _, id := range candidateIDs {
		var degree int
		if err := tx.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE source = ? OR target = ?", id, id).Scan(&degree); err != nil {
			return fmt.Errorf("count remaining source-file node edges: %w", err)
		}
		if degree == 0 {
			deleteIDs = append(deleteIDs, id)
		}
	}

	if len(deleteIDs) > 0 {
		placeholders := knowledgeGraphSQLInPlaceholders(len(deleteIDs))
		args := make([]interface{}, len(deleteIDs))
		for i, id := range deleteIDs {
			args[i] = id
		}
		if _, err := tx.Exec(fmt.Sprintf("DELETE FROM kg_nodes WHERE id IN (%s)", placeholders), args...); err != nil {
			return fmt.Errorf("delete stale source-file nodes: %w", err)
		}
	}

	indexNodes, indexEdges, err := kg.mergeExtractedEntitiesTx(tx, nodes, edges)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	kg.removeSemanticIndexesForDeletedGraphData(deleteIDs, removedEdges)
	for _, node := range indexNodes {
		kg.upsertSemanticNodeIndex(node)
	}
	for _, e := range indexEdges {
		kg.upsertSemanticEdgeIndex(e)
	}
	return nil
}

func (kg *KnowledgeGraph) mergeExtractedEntitiesTx(tx *sql.Tx, nodes []Node, edges []Edge) ([]Node, []Edge, error) {
	nowTime := time.Now()
	now := nowTime.Format(time.RFC3339)
	mergedNodes := mergeKnowledgeGraphNodes(nodes)
	mergedEdges := mergeKnowledgeGraphEdges(edges)
	indexNodes := make([]Node, 0, len(mergedNodes))
	indexEdges := make([]Edge, 0, len(mergedEdges))
	var bulkErrors []error
	for _, n := range mergedNodes {
		if n.ID == "" {
			continue
		}
		existingLabel, existingProps, existingProtected, _, err := loadKnowledgeGraphNode(tx, n.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("load existing node %q: %w", n.ID, err)
		}

		n.Properties = sanitizeKnowledgeGraphNodeProperties(n.Properties, strings.EqualFold(strings.TrimSpace(n.Properties["protected"]), "true"))
		finalLabel := mergeKnowledgeGraphLabel(existingLabel, n.Label)
		finalProps := mergeKnowledgeGraphPropertiesForExtraction(existingProps, n.Properties)
		isProtected := existingProtected
		if finalProps["protected"] == "true" {
			isProtected = 1
		}
		finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, isProtected != 0)
		finalProps = validateNodeSchema(finalProps)
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
			bulkErrors = append(bulkErrors, fmt.Errorf("merge node %q: %w", n.ID, execErr))
			continue
		}
		indexNodes = append(indexNodes, Node{ID: n.ID, Label: finalLabel, Properties: finalProps})
	}

	for _, e := range mergedEdges {
		if e.Source == "" || e.Target == "" || e.Relation == "" {
			continue
		}
		for _, id := range []string{e.Source, e.Target} {
			if execErr := ensureKnowledgeGraphPlaceholderNodeTx(tx, id); execErr != nil {
				bulkErrors = append(bulkErrors, fmt.Errorf("ensure endpoint node %q for edge %q->%q: %w", id, e.Source, e.Target, execErr))
			}
		}

		existingProps, _, err := loadKnowledgeGraphEdge(tx, e.Source, e.Target, e.Relation)
		if err != nil {
			return nil, nil, fmt.Errorf("load existing edge %q->%q/%q: %w", e.Source, e.Target, e.Relation, err)
		}
		defaultSource := strings.TrimSpace(e.Properties["source"])
		if defaultSource == "" {
			defaultSource = "auto_extraction"
		}
		e.Properties = ensureKnowledgeGraphEdgeQualityProperties(e.Properties, defaultSource, nowTime)
		finalProps := mergeKnowledgeGraphPropertiesForExtraction(existingProps, e.Properties)
		propsJSON, _ := json.Marshal(finalProps)

		if _, execErr := tx.Exec(`
			INSERT INTO kg_edges (source, target, relation, properties, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(source, target, relation) DO UPDATE SET
				properties = excluded.properties,
				updated_at = excluded.updated_at
		`, e.Source, e.Target, e.Relation, string(propsJSON), now); execErr != nil {
			bulkErrors = append(bulkErrors, fmt.Errorf("merge edge %q->%q/%q: %w", e.Source, e.Target, e.Relation, execErr))
			continue
		}
		indexEdges = append(indexEdges, Edge{Source: e.Source, Target: e.Target, Relation: e.Relation, Properties: finalProps})
	}

	if len(bulkErrors) > 0 {
		return nil, nil, fmt.Errorf("bulk merge extracted entities failed: %w", errors.Join(bulkErrors...))
	}
	return indexNodes, indexEdges, nil
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

	if err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
	`).Scan(&stats.NodeCount); err != nil {
		return nil, fmt.Errorf("count file sync nodes: %w", err)
	}

	if err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges
		WHERE json_extract(properties, '$.source') = 'file_sync'
	`).Scan(&stats.EdgeCount); err != nil {
		return nil, fmt.Errorf("count file sync edges: %w", err)
	}

	typeRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.type'), ''), 'untyped') AS t, COUNT(*)
		FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		GROUP BY t
	`)
	if err != nil {
		return nil, fmt.Errorf("query file sync node types: %w", err)
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var t string
		var c int
		if err := typeRows.Scan(&t, &c); err != nil {
			return nil, fmt.Errorf("scan file sync node type count: %w", err)
		}
		stats.ByEntityType[t] = c
	}
	if err := typeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file sync node type counts: %w", err)
	}

	collRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.collection'), ''), 'default') AS c, COUNT(*)
		FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		GROUP BY c
	`)
	if err != nil {
		return nil, fmt.Errorf("query file sync collections: %w", err)
	}
	defer collRows.Close()
	for collRows.Next() {
		var c string
		var cnt int
		if err := collRows.Scan(&c, &cnt); err != nil {
			return nil, fmt.Errorf("scan file sync collection count: %w", err)
		}
		stats.ByCollection[c] = cnt
	}
	if err := collRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file sync collection counts: %w", err)
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

	var lastSync sql.NullString
	err := kg.db.QueryRow(`
		SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes
		WHERE json_extract(properties, '$.source') = 'file_sync'
		AND json_extract(properties, '$.collection') = ?
	`, collection).Scan(&lastSync)
	if err == nil && lastSync.Valid && lastSync.String != "" {
		if t, err := time.Parse("2006-01-02", lastSync.String); err == nil {
			stats.LastSyncAt = &t
		}
	}

	return stats, nil
}

func (kg *KnowledgeGraph) GetLastFileSyncTime(collection string) (*time.Time, error) {
	var lastSync sql.NullString
	var query string
	var args []interface{}

	if collection == "" {
		query = `SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes WHERE json_extract(properties, '$.source') = 'file_sync'`
	} else {
		query = `SELECT MAX(json_extract(properties, '$.extracted_at')) FROM kg_nodes WHERE json_extract(properties, '$.source') = 'file_sync' AND json_extract(properties, '$.collection') = ?`
		args = append(args, collection)
	}

	err := kg.db.QueryRow(query, args...).Scan(&lastSync)
	if err != nil {
		return nil, err
	}
	if !lastSync.Valid || lastSync.String == "" {
		return nil, nil
	}

	t, err := time.Parse("2006-01-02", lastSync.String)
	if err != nil {
		return nil, fmt.Errorf("parse last sync time: %w", err)
	}
	return &t, nil
}
