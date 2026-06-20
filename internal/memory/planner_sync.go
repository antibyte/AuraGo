package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

func isPlannerManagedKGNodeID(id string) bool {
	return strings.HasPrefix(id, "appointment_") || strings.HasPrefix(id, "todo_")
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

	removed := 0
	for _, target := range staleTargets {
		if err := kg.DeleteEdge(source, target, relation); err != nil {
			return removed, fmt.Errorf("delete stale planner edge %s->%s/%s: %w", source, target, relation, err)
		}
		removed++
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

	removed := 0
	for _, edge := range staleEdges {
		if err := kg.DeleteEdge(edge.source, edge.target, edge.relation); err != nil {
			return removed, fmt.Errorf("delete stale planner sync edge %s->%s/%s: %w", edge.source, edge.target, edge.relation, err)
		}
		removed++
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