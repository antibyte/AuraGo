package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

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

func (kg *KnowledgeGraph) DeleteNode(id string) error {
	var edgesToClean []Edge
	if kg.semantic != nil {
		rows, err := kg.db.Query("SELECT source, target, relation FROM kg_edges WHERE source = ? OR target = ?", id, id)
		if err == nil {
			for rows.Next() {
				var src, tgt, rel string
				if rows.Scan(&src, &tgt, &rel) == nil {
					edgesToClean = append(edgesToClean, Edge{Source: src, Target: tgt, Relation: rel})
				}
			}
			rows.Close()
		}
	}

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

	if err := tx.Commit(); err != nil {
		return err
	}

	if kg.semantic != nil {
		kg.semantic.mu.Lock()
		delete(kg.semantic.contentCache, id)
		kg.semantic.mu.Unlock()

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
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetAllNodes", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
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

func (kg *KnowledgeGraph) MergeNodes(targetID, sourceID string) error {
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin merge nodes: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE kg_edges SET target = ? WHERE target = ?", targetID, sourceID)
	if err != nil {
		return fmt.Errorf("update edges target: %w", err)
	}

	_, err = tx.Exec("UPDATE kg_edges SET source = ? WHERE source = ?", targetID, sourceID)
	if err != nil {
		return fmt.Errorf("update edges source: %w", err)
	}

	var sourceAccess int
	err = tx.QueryRow("SELECT access_count FROM kg_nodes WHERE id = ?", sourceID).Scan(&sourceAccess)
	if err == nil && sourceAccess > 0 {
		_, err = tx.Exec("UPDATE kg_nodes SET access_count = access_count + ? WHERE id = ?", sourceAccess, targetID)
		if err != nil {
			return fmt.Errorf("update target access count: %w", err)
		}
	}

	_, err = tx.Exec("DELETE FROM kg_nodes WHERE id = ?", sourceID)
	if err != nil {
		return fmt.Errorf("delete source node: %w", err)
	}

	return tx.Commit()
}

func (kg *KnowledgeGraph) DeleteNodesBySourceFile(path string) (int, error) {
	tx, err := kg.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete nodes by source file: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT id FROM kg_nodes
		WHERE json_extract(properties, '$.source_file') = ?
		  AND protected = 0
	`, path)
	if err != nil {
		return 0, fmt.Errorf("query nodes by source file: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	for _, id := range ids {
		if _, err := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); err != nil {
			kg.logger.Warn("DeleteNodesBySourceFile: failed to delete edges for node", "id", id, "error", err)
		}
		if _, err := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); err != nil {
			kg.logger.Warn("DeleteNodesBySourceFile: failed to delete node", "id", id, "error", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete nodes by source file: %w", err)
	}
	return len(ids), nil
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
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNodesBySourceFile", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
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
