package memory

import (
	"encoding/json"
	"strings"
)

// Explore runs a semantic/keyword search and returns a sub-graph of matched nodes plus their 1-hop neighbors and connecting edges.
func (kg *KnowledgeGraph) Explore(query string) string {
	nodesMap := make(map[string]Node)
	var edges []Edge
	var matchedNodeIDs []string

	if kg.semantic != nil {
		results := kg.semanticSearchNodes(query, 0.4, 5)
		for _, n := range results {
			nodesMap[n.ID] = n
			matchedNodeIDs = append(matchedNodeIDs, n.ID)
		}
	} else {
		// fallback to standard FTS search
		ftsQuery := escapeFTS5(query)
		likePattern := "%" + strings.ToLower(query) + "%"
		rows, err := kg.db.Query(`SELECT id, label, properties, protected FROM kg_nodes WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?) UNION SELECT id, label, properties, protected FROM kg_nodes WHERE id LIKE ? OR label LIKE ? OR properties LIKE ? LIMIT 5`, ftsQuery, likePattern, likePattern, likePattern)
		if err == nil {
			for rows.Next() {
				var n Node
				var propsJSON string
				var protected int
				if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
					n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "Explore", n.ID, propsJSON, protected)
					n.Protected = protected != 0
					nodesMap[n.ID] = n
					matchedNodeIDs = append(matchedNodeIDs, n.ID)
				}
			}
			rows.Close()
		}
	}

	for _, id := range matchedNodeIDs {
		nbs, es := kg.GetNeighbors(id, 10)
		for _, nb := range nbs {
			if _, exists := nodesMap[nb.ID]; !exists {
				nodesMap[nb.ID] = nb
			}
		}
		for _, e := range es {
			edges = append(edges, e)
		}
	}

	var finalList []Node
	for _, n := range nodesMap {
		finalList = append(finalList, n)
	}

	result := map[string]interface{}{
		"nodes": finalList,
		"edges": edges,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// SuggestRelations finds nodes that might be related based on common properties or labels, but aren't connected yet.
func (kg *KnowledgeGraph) SuggestRelations(limit int) string {
	if limit <= 0 {
		limit = 10
	}
	rows, err := kg.db.Query(`
		SELECT id1, label1, id2, label2, reason FROM (
			SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_type' as reason
			FROM kg_nodes n1 
			JOIN kg_nodes n2 ON n1.node_type = n2.node_type AND n1.id < n2.id
			WHERE n1.node_type IS NOT NULL AND n1.node_type != 'activity_entity' AND n1.node_type != 'unknown'

			UNION

			SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_ip' as reason
			FROM kg_nodes n1 
			JOIN kg_nodes n2 ON json_extract(n1.properties, '$.ip') = json_extract(n2.properties, '$.ip') AND n1.id < n2.id
			WHERE json_extract(n1.properties, '$.ip') IS NOT NULL

			UNION

			SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_location' as reason
			FROM kg_nodes n1 
			JOIN kg_nodes n2 ON json_extract(n1.properties, '$.location') = json_extract(n2.properties, '$.location') AND n1.id < n2.id
			WHERE json_extract(n1.properties, '$.location') IS NOT NULL
		) results
		WHERE NOT EXISTS (
			SELECT 1 FROM kg_edges e 
			WHERE (e.source = results.id1 AND e.target = results.id2) OR (e.source = results.id2 AND e.target = results.id1)
		)
		LIMIT ?`, limit)
	if err != nil {
		return "[]"
	}
	defer rows.Close()

	var suggestions []map[string]string
	for rows.Next() {
		var id1, label1, id2, label2, reason string
		if err := rows.Scan(&id1, &label1, &id2, &label2, &reason); err == nil {
			suggestions = append(suggestions, map[string]string{
				"source":       id1,
				"source_label": label1,
				"target":       id2,
				"target_label": label2,
				"reason":       "same_type",
			})
		}
	}
	if len(suggestions) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(suggestions)
	return string(data)
}
