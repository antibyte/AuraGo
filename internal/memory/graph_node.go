package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

const knowledgeGraphPlaceholderLabel = "Unknown"
const knowledgeGraphPlaceholderSource = "auto_placeholder"
const knowledgeGraphPlaceholderGraceDays = 7

func knowledgeGraphPlaceholderNodeProperties() map[string]string {
	return map[string]string{
		"type":   "unknown",
		"source": knowledgeGraphPlaceholderSource,
	}
}

func ensureKnowledgeGraphPlaceholderNodeTx(tx *sql.Tx, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	propsJSON, err := json.Marshal(knowledgeGraphPlaceholderNodeProperties())
	if err != nil {
		return fmt.Errorf("marshal placeholder node properties: %w", err)
	}
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO kg_nodes (id, label, properties, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, id, knowledgeGraphPlaceholderLabel, string(propsJSON))
	return err
}

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

func (kg *KnowledgeGraph) AddNode(id, label string, properties map[string]string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("node id is required")
	}
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
	finalProps = validateNodeSchema(finalProps)

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
	kg.indexSemanticNodeAfterWrite(Node{ID: id, Label: finalLabel, Properties: finalProps})
	return nil
}

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

// ListNodesByIDPrefix returns nodes whose IDs start with prefix.
func (kg *KnowledgeGraph) ListNodesByIDPrefix(prefix string, limit int) ([]Node, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1000
	}

	rows, err := kg.db.Query(
		`SELECT id, label, properties, protected FROM kg_nodes WHERE id LIKE ? ESCAPE '\' ORDER BY id ASC LIMIT ?`,
		escapeLike(prefix)+"%",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes by id prefix %s: %w", prefix, err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&node.ID, &node.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan node by id prefix %s: %w", prefix, err)
		}
		node.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "ListNodesByIDPrefix", node.ID, propsJSON, protected)
		node.Protected = protected != 0
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

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
		finalProps = mergeKnowledgeGraphPropertiesOverwrite(existingProps, properties)
		finalProps = sanitizeKnowledgeGraphNodeProperties(finalProps, existingProtected != 0)
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
	kg.indexSemanticNodeAfterWrite(*node)
	return node, nil
}

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
	kg.indexSemanticNodeAfterWrite(*node)
	return node, nil
}

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

	var edgesToClean []Edge
	if kg.semanticIndex() != nil {
		edgeRows, err := tx.Query("SELECT source, target, relation FROM kg_edges WHERE source = ? OR target = ?", id, id)
		if err != nil {
			return fmt.Errorf("load incident edges for node %s: %w", id, err)
		}
		for edgeRows.Next() {
			var src, tgt, rel string
			if err := edgeRows.Scan(&src, &tgt, &rel); err != nil {
				edgeRows.Close()
				return fmt.Errorf("scan incident edge for node %s: %w", id, err)
			}
			edgesToClean = append(edgesToClean, Edge{Source: src, Target: tgt, Relation: rel})
		}
		if err := edgeRows.Err(); err != nil {
			edgeRows.Close()
			return fmt.Errorf("iterate incident edges for node %s: %w", id, err)
		}
		edgeRows.Close()
	}

	if err := cleanupKGClaimsForDeletedNodeTx(tx, id); err != nil {
		return fmt.Errorf("cleanup kg provenance for node %s: %w", id, err)
	}
	if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); err != nil {
		return fmt.Errorf("delete edges for node %s: %w", id, err)
	}
	if _, err := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete node %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if kg.semanticIndex() != nil {
		for _, e := range edgesToClean {
			if err := kg.removeSemanticEdgeIndex(e.Source, e.Target, e.Relation); err != nil && kg.logger != nil {
				kg.logger.Warn("DeleteNode: failed to remove semantic edge index", "source", e.Source, "target", e.Target, "relation", e.Relation, "error", err)
			}
		}
		if err := kg.removeSemanticNodeIndex(id); err != nil && kg.logger != nil {
			kg.logger.Warn("DeleteNode: failed to remove semantic node index", "node_id", id, "error", err)
		}
	}

	return nil
}

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
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan node in GetAllNodes: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetAllNodes", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GetAllNodes rows: %w", err)
	}
	return nodes, nil
}

// GetNodesByType returns KG nodes filtered by their generated node_type column.
func (kg *KnowledgeGraph) GetNodesByType(nodeType string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 100
	}
	nodeType = strings.TrimSpace(strings.ToLower(nodeType))
	if nodeType == "" {
		return kg.GetAllNodes(limit)
	}
	rows, err := kg.db.Query(
		"SELECT id, label, properties, protected FROM kg_nodes WHERE node_type = ? ORDER BY access_count DESC LIMIT ?",
		nodeType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get nodes by type %q: %w", nodeType, err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan node in GetNodesByType: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNodesByType", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GetNodesByType rows: %w", err)
	}
	if nodes == nil {
		nodes = []Node{}
	}
	return nodes, nil
}

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
			MIN(COALESCE(deg.meaningful_degree, 0) * 3, 30) +
			(CASE WHEN (SELECT COUNT(*) FROM json_each(n.properties)
				WHERE key NOT IN ('source','extracted_at','last_seen','session_id','channel','protected')) >= 3
				THEN 10 ELSE 0 END) +
			(CASE WHEN n.updated_at > datetime('now', '-7 days') THEN 5 ELSE 0 END)
			AS importance_score
		FROM kg_nodes n
		LEFT JOIN (
			SELECT node_id, COUNT(*) AS meaningful_degree
			FROM (
				SELECT source AS node_id FROM kg_edges WHERE relation != 'co_mentioned_with'
				UNION ALL
				SELECT target AS node_id FROM kg_edges WHERE relation != 'co_mentioned_with'
			)
			GROUP BY node_id
		) deg ON deg.node_id = n.id
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
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected, &n.ImportanceScore); err != nil {
			return nil, fmt.Errorf("scan important node: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetImportantNodes", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		result = append(result, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate important nodes: %w", err)
	}
	return result, nil
}

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
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan recent change node: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetRecentChanges", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent changes: %w", err)
	}
	return nodes, nil
}

func (kg *KnowledgeGraph) MergeNodes(targetID, sourceID string) error {
	targetID = strings.TrimSpace(targetID)
	sourceID = strings.TrimSpace(sourceID)
	if targetID == "" || sourceID == "" {
		return fmt.Errorf("targetID and sourceID are required")
	}
	if targetID == sourceID {
		return nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin merge nodes: %w", err)
	}
	defer tx.Rollback()

	targetLabel, targetProps, targetProtected, targetFound, err := loadKnowledgeGraphNode(tx, targetID)
	if err != nil {
		return fmt.Errorf("load target node %s: %w", targetID, err)
	}
	if !targetFound {
		return fmt.Errorf("target node not found: %s", targetID)
	}

	sourceLabel, sourceProps, sourceProtected, sourceFound, err := loadKnowledgeGraphNode(tx, sourceID)
	if err != nil {
		return fmt.Errorf("load source node %s: %w", sourceID, err)
	}
	if !sourceFound {
		return fmt.Errorf("source node not found: %s", sourceID)
	}
	if sourceProtected != 0 {
		return ErrKnowledgeGraphProtectedNode
	}

	removedEdges := kg.collectSemanticEdgeIdentities(tx, `
		SELECT source, target, relation FROM kg_edges
		WHERE source = ? OR target = ?
	`, sourceID, sourceID)

	mergedLabel := mergeKnowledgeGraphLabel(targetLabel, sourceLabel)
	mergedProtected := targetProtected != 0
	mergedProps := mergeKnowledgeGraphProperties(targetProps, sourceProps)
	mergedProps = sanitizeKnowledgeGraphNodeProperties(mergedProps, mergedProtected)
	propsJSON, err := json.Marshal(mergedProps)
	if err != nil {
		return fmt.Errorf("marshal merged node properties: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE kg_nodes
		SET label = ?, properties = ?, protected = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, mergedLabel, string(propsJSON), boolToInt(mergedProtected), targetID); err != nil {
		return fmt.Errorf("update target node during merge: %w", err)
	}

	var sourceAccess int
	err = tx.QueryRow("SELECT access_count FROM kg_nodes WHERE id = ?", sourceID).Scan(&sourceAccess)
	if err == nil && sourceAccess > 0 {
		_, err = tx.Exec("UPDATE kg_nodes SET access_count = access_count + ? WHERE id = ?", sourceAccess, targetID)
		if err != nil {
			return fmt.Errorf("update target access count: %w", err)
		}
	}

	if err := cleanupKGSelfClaimFactsTx(tx, targetID); err != nil {
		return fmt.Errorf("cleanup target self-edge provenance before merge: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? AND target = ?", targetID, targetID); err != nil {
		return fmt.Errorf("delete pre-existing self edges: %w", err)
	}
	if err := cleanupMergedCollisionClaimsTx(tx, targetID, sourceID); err != nil {
		return fmt.Errorf("cleanup merge collision claims: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE source = ? AND EXISTS (
			SELECT 1
			FROM kg_edges existing
			WHERE existing.source = ?
			  AND existing.target = kg_edges.target
			  AND existing.relation = kg_edges.relation
		)
	`, sourceID, targetID); err != nil {
		return fmt.Errorf("delete outgoing edge collisions before merge: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE target = ? AND EXISTS (
			SELECT 1
			FROM kg_edges existing
			WHERE existing.target = ?
			  AND existing.source = kg_edges.source
			  AND existing.relation = kg_edges.relation
		)
	`, sourceID, targetID); err != nil {
		return fmt.Errorf("delete incoming edge collisions before merge: %w", err)
	}
	if _, err := tx.Exec("UPDATE kg_edges SET target = ? WHERE target = ?", targetID, sourceID); err != nil {
		return fmt.Errorf("update edges target: %w", err)
	}
	if _, err := tx.Exec("UPDATE kg_edges SET source = ? WHERE source = ?", targetID, sourceID); err != nil {
		return fmt.Errorf("update edges source: %w", err)
	}
	if _, err := tx.Exec("UPDATE kg_claims SET object_id = ? WHERE object_id = ?", targetID, sourceID); err != nil {
		return fmt.Errorf("update claim objects during merge: %w", err)
	}
	if _, err := tx.Exec("UPDATE kg_claims SET subject_id = ? WHERE subject_id = ?", targetID, sourceID); err != nil {
		return fmt.Errorf("update claim subjects during merge: %w", err)
	}
	if err := cleanupKGSelfClaimFactsTx(tx, targetID); err != nil {
		return fmt.Errorf("cleanup merged self-claim facts: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? AND target = ?", targetID, targetID); err != nil {
		return fmt.Errorf("delete merged self edges: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE (source IN (?, ?) OR target IN (?, ?))
		  AND rowid NOT IN (
			SELECT MIN(rowid)
			FROM kg_edges
			WHERE source IN (?, ?) OR target IN (?, ?)
			GROUP BY source, target, relation
		)
	`, targetID, sourceID, targetID, sourceID, targetID, sourceID, targetID, sourceID); err != nil {
		return fmt.Errorf("deduplicate merged edges: %w", err)
	}

	_, err = tx.Exec("DELETE FROM kg_nodes WHERE id = ?", sourceID)
	if err != nil {
		return fmt.Errorf("delete source node: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	kg.removeSemanticIndexesForDeletedGraphData([]string{sourceID}, removedEdges)
	kg.indexSemanticNodeAfterWrite(Node{ID: targetID, Label: mergedLabel, Properties: mergedProps, Protected: mergedProtected})
	incidentEdges, err := kg.GetImportantEdges(500, []string{targetID})
	if err != nil {
		return fmt.Errorf("reload merged incident edges: %w", err)
	}
	for _, edge := range incidentEdges {
		kg.indexSemanticEdgeAfterWrite(edge)
	}

	return nil
}

func (kg *KnowledgeGraph) DeleteNodesBySourceFile(path string) (int, error) {
	tx, err := kg.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete nodes by source file: %w", err)
	}
	defer tx.Rollback()

	sourceFileEdges := kg.collectSemanticEdgeIdentities(tx, `
		SELECT source, target, relation FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path)
	if err := cleanupKGClaimsForDeletedSemanticEdgesTx(tx, sourceFileEdges); err != nil {
		return 0, fmt.Errorf("cleanup source-file edge provenance: %w", err)
	}
	if _, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path); err != nil {
		return 0, fmt.Errorf("delete source-file edges before node cleanup: %w", err)
	}

	rows, err := tx.Query(`
		SELECT id FROM kg_nodes
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
		  AND protected = 0
	`, path)
	if err != nil {
		return 0, fmt.Errorf("query nodes by source file: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan node id for source file delete: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate node ids for source file delete: %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit delete source-file edges: %w", err)
		}
		kg.removeSemanticIndexesForDeletedGraphData(nil, sourceFileEdges)
		return 0, nil
	}

	var toDelete []string
	for _, id := range ids {
		var degree int
		if err := tx.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE source = ? OR target = ?", id, id).Scan(&degree); err != nil {
			return 0, fmt.Errorf("count remaining source-file node edges: %w", err)
		}
		if degree == 0 {
			toDelete = append(toDelete, id)
		}
	}

	var deleted int
	if len(toDelete) > 0 {
		for _, id := range toDelete {
			if err := cleanupKGClaimsForDeletedNodeTx(tx, id); err != nil {
				return 0, fmt.Errorf("cleanup source-file node provenance %s: %w", id, err)
			}
		}
		deleteRes, err := execChunkedInDeleteStringsResult(tx, "kg_nodes", "id", toDelete, defaultInClauseChunkSize)
		if err != nil {
			return 0, fmt.Errorf("batch delete nodes by source file: %w", err)
		}
		deleted64, _ := deleteRes.RowsAffected()
		deleted = int(deleted64)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete nodes by source file: %w", err)
	}
	kg.removeSemanticIndexesForDeletedGraphData(toDelete, sourceFileEdges)
	return deleted, nil
}

func (kg *KnowledgeGraph) GetNodesBySourceFile(path string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE json_extract(properties, '$.source_file') = ?
		LIMIT ?
	`, path, limit)
	if err != nil {
		return nil, fmt.Errorf("query nodes by source file: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan node by source file: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNodesBySourceFile", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes by source file: %w", err)
	}
	return nodes, nil
}

func (kg *KnowledgeGraph) batchGetNodes(ids []string) []Node {
	nodes, err := loadNodesByIDs(kg.db, ids, kg.logger, "batchGetNodes")
	if err != nil && kg.logger != nil {
		kg.logger.Warn("batchGetNodes failed", "error", err)
	}
	return nodes
}

func loadNodesByIDs(q knowledgeGraphQueryer, ids []string, logger *slog.Logger, op string) ([]Node, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	chunkSize := defaultInClauseChunkSize
	nodes := make([]Node, 0, len(ids))
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		placeholders := knowledgeGraphSQLInPlaceholders(len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := q.Query(fmt.Sprintf(
			"SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (%s) ORDER BY id",
			placeholders,
		), args...)
		if err != nil {
			return nil, fmt.Errorf("query nodes by ids chunk %d-%d: %w", start, end-1, err)
		}
		for rows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan node by id: %w", err)
			}
			n.Properties = decodeKnowledgeGraphNodeProperties(logger, op, n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate nodes by id chunk %d-%d: %w", start, end-1, err)
		}
		rows.Close()
	}
	return nodes, nil
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

func choosePreferredAutoExtractedLabel(existing, incoming string) string {
	return mergeKnowledgeGraphLabels(existing, incoming, true)
}
