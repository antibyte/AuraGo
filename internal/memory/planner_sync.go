package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

func isPlannerManagedKGNodeID(id string) bool {
	return strings.HasPrefix(id, "appointment_") || strings.HasPrefix(id, "todo_")
}

func isPlannerRootKGNodeID(id string) bool {
	if strings.HasPrefix(id, "appointment_") {
		return true
	}
	return strings.HasPrefix(id, "todo_") && !strings.Contains(id, "_item_")
}

func isPlannerItemKGNodeID(id string) bool {
	return strings.HasPrefix(id, "todo_") && strings.Contains(id, "_item_")
}

// PrunePlannerEdges removes planner-sourced edges from source with relation where target is not kept.
func (kg *KnowledgeGraph) PrunePlannerEdges(source, relation string, keepTargets map[string]struct{}) (int, error) {
	source = strings.TrimSpace(source)
	relation = strings.TrimSpace(relation)
	if source == "" || relation == "" {
		return 0, nil
	}

	rows, err := kg.db.Query(`
		SELECT target, properties FROM kg_edges
		WHERE source = ? AND relation = ?
	`, source, relation)
	if err != nil {
		return 0, fmt.Errorf("query planner edges for prune: %w", err)
	}
	defer rows.Close()

	var staleTargets []string
	for rows.Next() {
		var target, propsJSON string
		if err := rows.Scan(&target, &propsJSON); err != nil {
			return 0, fmt.Errorf("scan planner edge for prune: %w", err)
		}
		props := make(map[string]string)
		if propsJSON != "" {
			_ = json.Unmarshal([]byte(propsJSON), &props)
		}
		if props["source"] != "planner" {
			continue
		}
		if _, keep := keepTargets[target]; keep {
			continue
		}
		staleTargets = append(staleTargets, target)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate planner edges for prune: %w", err)
	}
	if len(staleTargets) == 0 {
		return 0, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin planner edge prune transaction: %w", err)
	}
	defer tx.Rollback()

	placeholders := knowledgeGraphSQLInPlaceholders(len(staleTargets))
	args := make([]interface{}, 0, 2+len(staleTargets))
	args = append(args, source, relation)
	for _, target := range staleTargets {
		args = append(args, target)
	}
	res, err := tx.Exec(fmt.Sprintf(`
		DELETE FROM kg_edges
		WHERE source = ? AND relation = ?
		  AND target IN (%s)
		  AND json_extract(properties, '$.source') = 'planner'
	`, placeholders), args...)
	if err != nil {
		return 0, fmt.Errorf("batch delete stale planner edges: %w", err)
	}
	removed64, _ := res.RowsAffected()
	removed := int(removed64)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit planner edge prune transaction: %w", err)
	}

	for _, target := range staleTargets {
		if err := kg.removeSemanticEdgeIndex(source, target, relation); err != nil && kg.logger != nil {
			kg.logger.Warn("PrunePlannerEdges: failed to remove semantic edge index",
				"source", source, "target", target, "relation", relation, "error", err)
		}
	}
	return removed, nil
}

// PrunePlannerNodesByPrefix deletes planner-sourced nodes with the given ID prefix that are not kept.
func (kg *KnowledgeGraph) PrunePlannerNodesByPrefix(prefix string, keepIDs map[string]struct{}) (int, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, nil
	}

	nodes, err := kg.ListNodesByIDPrefix(prefix, 10000)
	if err != nil {
		return 0, fmt.Errorf("list planner nodes by prefix %q: %w", prefix, err)
	}

	removed := 0
	for _, node := range nodes {
		if keepIDs != nil {
			if _, keep := keepIDs[node.ID]; keep {
				continue
			}
		}
		if node.Properties["source"] != "planner" {
			continue
		}
		if node.Protected {
			continue
		}
		if err := kg.DeleteNode(node.ID); err != nil {
			return removed, fmt.Errorf("delete stale planner node %s: %w", node.ID, err)
		}
		removed++
	}
	return removed, nil
}

// PruneStalePlannerRootNodes deletes planner-sourced appointment/todo root nodes not in keepIDs.
func (kg *KnowledgeGraph) PruneStalePlannerRootNodes(keepIDs map[string]struct{}) (int, error) {
	removed := 0
	for _, prefix := range []string{"appointment_", "todo_"} {
		nodes, err := kg.ListNodesByIDPrefix(prefix, 10000)
		if err != nil {
			return removed, fmt.Errorf("list planner root nodes by prefix %q: %w", prefix, err)
		}
		for _, node := range nodes {
			if !isPlannerRootKGNodeID(node.ID) {
				continue
			}
			if keepIDs != nil {
				if _, keep := keepIDs[node.ID]; keep {
					continue
				}
			}
			if node.Properties["source"] != "planner" {
				continue
			}
			if node.Protected {
				continue
			}
			if err := kg.DeleteNode(node.ID); err != nil {
				return removed, fmt.Errorf("delete stale planner root node %s: %w", node.ID, err)
			}
			removed++
		}
	}
	return removed, nil
}

// PruneStalePlannerItemNodes deletes planner-sourced todo checklist item nodes not in keepIDs.
func (kg *KnowledgeGraph) PruneStalePlannerItemNodes(keepIDs map[string]struct{}) (int, error) {
	nodes, err := kg.ListNodesByIDPrefix("todo_", 10000)
	if err != nil {
		return 0, fmt.Errorf("list planner item nodes: %w", err)
	}

	removed := 0
	for _, node := range nodes {
		if !isPlannerItemKGNodeID(node.ID) {
			continue
		}
		if keepIDs != nil {
			if _, keep := keepIDs[node.ID]; keep {
				continue
			}
		}
		if node.Properties["source"] != "planner" {
			continue
		}
		if node.Protected {
			continue
		}
		if err := kg.DeleteNode(node.ID); err != nil {
			return removed, fmt.Errorf("delete stale planner item node %s: %w", node.ID, err)
		}
		removed++
	}
	return removed, nil
}

// DeleteStalePlannerSyncEdges removes planner sync edges that are no longer expected or reference removed planner nodes.
func (kg *KnowledgeGraph) DeleteStalePlannerSyncEdges(expectedEdges map[string]struct{}, activePlannerNodes map[string]struct{}) (int, error) {
	rows, err := kg.db.Query(`
		SELECT source, target, relation FROM kg_edges
		WHERE json_extract(properties, '$.source') = 'planner'
	`)
	if err != nil {
		return 0, fmt.Errorf("query planner sync edges: %w", err)
	}
	defer rows.Close()

	type plannerEdgeRef struct {
		source, target, relation string
	}
	var staleEdges []plannerEdgeRef
	for rows.Next() {
		var source, target, relation string
		if err := rows.Scan(&source, &target, &relation); err != nil {
			return 0, fmt.Errorf("scan planner sync edge: %w", err)
		}
		edgeKey := knowledgeGraphEdgeKey(source, target, relation)
		_, expected := expectedEdges[edgeKey]
		staleEndpoint := (isPlannerManagedKGNodeID(source) && !plannerNodeActive(activePlannerNodes, source)) ||
			(isPlannerManagedKGNodeID(target) && !plannerNodeActive(activePlannerNodes, target))
		if expected && !staleEndpoint {
			continue
		}
		staleEdges = append(staleEdges, plannerEdgeRef{source: source, target: target, relation: relation})
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate planner sync edges: %w", err)
	}
	if len(staleEdges) == 0 {
		return 0, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin planner sync edge delete transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS kg_planner_stale_edges (
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		relation TEXT NOT NULL,
		PRIMARY KEY (source, target, relation)
	)`); err != nil {
		return 0, fmt.Errorf("create planner stale edge batch table: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM kg_planner_stale_edges`); err != nil {
		return 0, fmt.Errorf("reset planner stale edge batch table: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO kg_planner_stale_edges (source, target, relation) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare planner stale edge batch insert: %w", err)
	}
	for _, edge := range staleEdges {
		if _, err := stmt.Exec(edge.source, edge.target, edge.relation); err != nil {
			stmt.Close()
			return 0, fmt.Errorf("insert planner stale edge batch row: %w", err)
		}
	}
	stmt.Close()

	res, err := tx.Exec(`
		DELETE FROM kg_edges
		WHERE EXISTS (
			SELECT 1
			FROM kg_planner_stale_edges s
			WHERE s.source = kg_edges.source
			  AND s.target = kg_edges.target
			  AND s.relation = kg_edges.relation
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("batch delete stale planner sync edges: %w", err)
	}
	removed64, _ := res.RowsAffected()
	removed := int(removed64)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit planner sync edge delete transaction: %w", err)
	}

	for _, edge := range staleEdges {
		if err := kg.removeSemanticEdgeIndex(edge.source, edge.target, edge.relation); err != nil && kg.logger != nil {
			kg.logger.Warn("DeleteStalePlannerSyncEdges: failed to remove semantic edge index",
				"source", edge.source, "target", edge.target, "relation", edge.relation, "error", err)
		}
	}
	return removed, nil
}

func plannerNodeActive(active map[string]struct{}, id string) bool {
	if len(active) == 0 {
		return false
	}
	_, ok := active[id]
	return ok
}