package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	_, err := kg.AddEdgeWithProvenance(source, target, relation, properties, KGProvenanceInput{
		SourceKind: "manual",
		Confidence: 1.0,
	})
	return err
}

// PruneOutgoingRelationEdges removes outgoing edges from source with relation where
// target is not in keepTargets.
func (kg *KnowledgeGraph) PruneOutgoingRelationEdges(source, relation string, keepTargets map[string]struct{}) (int, error) {
	source = strings.TrimSpace(source)
	relation = strings.TrimSpace(relation)
	if source == "" || relation == "" {
		return 0, nil
	}

	rows, err := kg.db.Query(`
		SELECT target FROM kg_edges
		WHERE source = ? AND relation = ?
		  AND `+activeKGEdgePredicate("")+`
	`, source, relation)
	if err != nil {
		return 0, fmt.Errorf("query outgoing relation edges for prune: %w", err)
	}
	defer rows.Close()

	var staleTargets []string
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			return 0, fmt.Errorf("scan outgoing relation edge for prune: %w", err)
		}
		if _, keep := keepTargets[target]; keep {
			continue
		}
		staleTargets = append(staleTargets, target)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate outgoing relation edges for prune: %w", err)
	}

	removed := 0
	for _, target := range staleTargets {
		if err := kg.DeleteEdge(source, target, relation); err != nil {
			return removed, fmt.Errorf("delete stale relation edge %s->%s/%s: %w", source, target, relation, err)
		}
		removed++
	}
	return removed, nil
}

func (kg *KnowledgeGraph) DeleteEdge(source, target, relation string) error {
	_, err := kg.db.Exec("DELETE FROM kg_edges WHERE source = ? AND target = ? AND relation = ?",
		source, target, relation)
	if err != nil {
		return fmt.Errorf("delete edge: %w", err)
	}
	if err := kg.removeSemanticEdgeIndex(source, target, relation); err != nil && kg.logger != nil {
		kg.logger.Warn("DeleteEdge: failed to remove semantic edge index", "source", source, "target", target, "relation", relation, "error", err)
	}
	return nil
}

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
		finalProps = ensureKnowledgeGraphEdgeQualityProperties(properties, "manual", time.Now())
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
		INSERT INTO kg_edges (
			source, target, relation, properties, updated_at,
			status, status_reason, superseded_by_claim_id, retracted_at
		)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, '', '', NULL)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = excluded.properties,
			updated_at = CURRENT_TIMESTAMP,
			status = excluded.status,
			status_reason = '',
			superseded_by_claim_id = '',
			retracted_at = NULL
	`, source, target, newRelation, string(propsJSON), string(KGClaimAccepted)); err != nil {
		return nil, fmt.Errorf("upsert updated edge: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	updated := &Edge{
		Source:     source,
		Target:     target,
		Relation:   newRelation,
		Properties: finalProps,
	}
	if relation != newRelation {
		if err := kg.removeSemanticEdgeIndex(source, target, relation); err != nil && kg.logger != nil {
			kg.logger.Warn("UpdateEdge: failed to remove old semantic edge index", "source", source, "target", target, "relation", relation, "error", err)
		}
	}
	kg.indexSemanticEdgeAfterWrite(*updated)
	return updated, nil
}

func (kg *KnowledgeGraph) GetAllEdges(limit int) ([]Edge, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := kg.db.Query("SELECT source, target, relation, properties FROM kg_edges WHERE "+activeKGEdgePredicate("")+" LIMIT ?", limit)
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
			SELECT e.source, e.target, e.relation, e.properties
			FROM kg_edges e
			LEFT JOIN kg_nodes ns ON ns.id = e.source
			LEFT JOIN kg_nodes nt ON nt.id = e.target
			WHERE e.relation != 'co_mentioned_with'
			  AND `+activeKGEdgePredicate("e")+`
			  AND (e.source IN (%s) OR e.target IN (%s))
			ORDER BY (COALESCE(ns.access_count, 0) + COALESCE(nt.access_count, 0)) DESC
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
			SELECT e.source, e.target, e.relation, e.properties
			FROM kg_edges e
			LEFT JOIN kg_nodes ns ON ns.id = e.source
			LEFT JOIN kg_nodes nt ON nt.id = e.target
			WHERE e.relation != 'co_mentioned_with'
			  AND `+activeKGEdgePredicate("e")+`
			ORDER BY (COALESCE(ns.access_count, 0) + COALESCE(nt.access_count, 0)) DESC
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

func (kg *KnowledgeGraph) DeleteEdgesBySourceFile(path string) (int, error) {
	rows, err := kg.db.Query(`
		SELECT source, target, relation FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path)
	if err != nil {
		return 0, fmt.Errorf("query edges by source file: %w", err)
	}
	var edges []Edge
	for rows.Next() {
		var edge Edge
		if err := rows.Scan(&edge.Source, &edge.Target, &edge.Relation); err == nil {
			edges = append(edges, edge)
		}
	}
	rows.Close()

	res, err := kg.db.Exec(`
		DELETE FROM kg_edges
		WHERE json_valid(properties)
		  AND json_extract(properties, '$.source_file') = ?
	`, path)
	if err != nil {
		return 0, fmt.Errorf("delete edges by source file: %w", err)
	}
	n, _ := res.RowsAffected()
	for _, edge := range edges {
		if err := kg.removeSemanticEdgeIndex(edge.Source, edge.Target, edge.Relation); err != nil && kg.logger != nil {
			kg.logger.Warn("DeleteEdgesBySourceFile: failed to remove semantic edge index",
				"source", edge.Source, "target", edge.Target, "relation", edge.Relation, "error", err)
		}
	}
	return int(n), nil
}

func (kg *KnowledgeGraph) GetEdgesBySourceFile(path string, limit int) ([]Edge, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE json_extract(properties, '$.source_file') = ?
		  AND `+activeKGEdgePredicate("")+`
		LIMIT ?
	`, path, limit)
	if err != nil {
		return nil, fmt.Errorf("query edges by source file: %w", err)
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("GetEdgesBySourceFile: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
				e.Properties = make(map[string]string)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
		}
	}
	return edges, nil
}

func (kg *KnowledgeGraph) IncrementCoOccurrence(a, b, date string) error {
	if a > b {
		a, b = b, a
	}

	initProps, err := json.Marshal(map[string]string{
		"source": "pending",
		"weight": "1",
		"date":   date,
	})
	if err != nil {
		return fmt.Errorf("marshal co-occurrence properties: %w", err)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin co-occurrence transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO kg_edges (source, target, relation, properties, updated_at)
		VALUES (?, ?, 'co_mentioned_with', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			properties = json_set(
				json_set(
					json_set(
						kg_edges.properties,
						'$.weight',
						CAST(
							CAST(COALESCE(NULLIF(json_extract(kg_edges.properties, '$.weight'), ''), '0') AS INTEGER) + 1
							AS TEXT
						)
					),
					'$.date',
					?
				),
				'$.source',
				CASE
					WHEN CAST(COALESCE(NULLIF(json_extract(kg_edges.properties, '$.weight'), ''), '0') AS INTEGER) + 1 >= ?
					THEN 'activity_turn'
					ELSE COALESCE(json_extract(kg_edges.properties, '$.source'), 'pending')
				END
			),
			updated_at = CURRENT_TIMESTAMP
	`, a, b, string(initProps), date, coOccurrenceThreshold)
	if err != nil {
		return fmt.Errorf("upsert co-occurrence: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	kg.indexSemanticEdgeAfterWrite(Edge{Source: a, Target: b, Relation: "co_mentioned_with"})
	return nil
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
