package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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

func (kg *KnowledgeGraph) DeleteEdgesBySourceFile(path string) (int, error) {
	rows, err := kg.db.Query(`
		SELECT source, target, relation FROM kg_edges
		WHERE json_extract(properties, '$.source_file') = ?
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
		WHERE json_extract(properties, '$.source_file') = ?
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
