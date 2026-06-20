package memory

import (
	"aurago/internal/dbutil"
	"encoding/json"
	"fmt"
	"strings"
)

// Explore runs a semantic/keyword search and returns a sub-graph of matched nodes plus their 1-hop neighbors and connecting edges.
func (kg *KnowledgeGraph) Explore(query string) string {
	tx, err := kg.beginReadTx("Explore")
	if err != nil {
		return kg.jsonError("Explore", err)
	}
	defer tx.Rollback()

	nodesMap := make(map[string]Node)
	var edges []Edge
	seenEdges := make(map[string]struct{})
	var matchedNodeIDs []string
	var accessHits []knowledgeGraphAccessHit

	if kg.semanticIndex() != nil {
		results := kg.semanticSearchNodesWithQueryer(tx, query, 0.4, 5)
		for _, n := range results {
			nodesMap[n.ID] = n
			matchedNodeIDs = append(matchedNodeIDs, n.ID)
		}
	}
	if len(matchedNodeIDs) == 0 {
		results, err := kg.exploreFTS(tx, query, 5)
		if err != nil {
			kg.logger.Warn("Explore: fallback query failed", "error", err)
		}
		for _, n := range results {
			nodesMap[n.ID] = n
			matchedNodeIDs = append(matchedNodeIDs, n.ID)
		}
	}

	for _, id := range matchedNodeIDs {
		nbs, es, hits, err := kg.getNeighborsWithQueryer(tx, id, 10)
		if err != nil {
			kg.logger.Warn("Explore: neighbor query failed", "node_id", id, "error", err)
			continue
		}
		accessHits = append(accessHits, hits...)
		for _, nb := range nbs {
			if _, exists := nodesMap[nb.ID]; !exists {
				nodesMap[nb.ID] = nb
			}
		}
		for _, e := range es {
			edgeKey := knowledgeGraphEdgeKey(e.Source, e.Target, e.Relation)
			if _, exists := seenEdges[edgeKey]; exists {
				continue
			}
			seenEdges[edgeKey] = struct{}{}
			edges = append(edges, e)
		}
	}
	if err := tx.Commit(); err != nil {
		return kg.jsonError("Explore", fmt.Errorf("commit read transaction: %w", err))
	}

	var finalList []Node
	for _, n := range nodesMap {
		finalList = append(finalList, n)
	}

	result := map[string]interface{}{
		"nodes": finalList,
		"edges": edges,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return kg.jsonError("Explore", fmt.Errorf("marshal explore result: %w", err))
	}
	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}
	return string(data)
}

func (kg *KnowledgeGraph) exploreFTS(q knowledgeGraphQueryer, query string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 5
	}
	ftsQuery := escapeFTS5(query)
	likePattern := "%" + dbutil.EscapeLike(strings.ToLower(query)) + "%"
	rows, err := q.Query(`
		SELECT id, label, properties, protected
		FROM kg_nodes
		WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?)
		UNION
		SELECT id, label, properties, protected
		FROM kg_nodes
		WHERE LOWER(id) LIKE ? ESCAPE '\' OR LOWER(label) LIKE ? ESCAPE '\' OR LOWER(properties) LIKE ? ESCAPE '\'
		LIMIT ?
	`, ftsQuery, likePattern, likePattern, likePattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]Node, 0, limit)
	for rows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan explore fallback node: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "Explore", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate explore fallback nodes: %w", err)
	}
	return nodes, nil
}

func (kg *KnowledgeGraph) jsonError(operation string, err error) string {
	if kg.logger != nil {
		kg.logger.Warn(operation+": JSON response failed", "error", err)
	}
	data, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		return `{"error":"json response failed"}`
	}
	return string(data)
}

const knowledgeGraphSuggestRelationsBranchLimit = 50

func knowledgeGraphSuggestRelationEligibleSQL(alias string, sourceCount int) string {
	placeholders := knowledgeGraphSQLInPlaceholders(sourceCount)
	return fmt.Sprintf("(%s.access_count > 0 OR %s.protected != 0 OR %s.source_type IN (%s))", alias, alias, alias, placeholders)
}

func appendKnowledgeGraphSuggestRelationSourceArgs(args []interface{}, sources []string) []interface{} {
	for _, source := range sources {
		args = append(args, source)
	}
	for _, source := range sources {
		args = append(args, source)
	}
	return args
}

// SuggestRelations finds nodes that might be related based on common properties or labels, but aren't connected yet.
func (kg *KnowledgeGraph) SuggestRelations(limit int) string {
	if limit <= 0 {
		limit = 10
	}
	sources := kg.listProtectOptimizeSources()
	n1Eligible := knowledgeGraphSuggestRelationEligibleSQL("n1", len(sources))
	n2Eligible := knowledgeGraphSuggestRelationEligibleSQL("n2", len(sources))

	args := make([]interface{}, 0, len(sources)*6+4)
	for i := 0; i < 3; i++ {
		args = appendKnowledgeGraphSuggestRelationSourceArgs(args, sources)
		args = append(args, knowledgeGraphSuggestRelationsBranchLimit)
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id1, label1, id2, label2, reason FROM (
			SELECT id1, label1, id2, label2, reason FROM (
				SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_type' as reason
				FROM kg_nodes n1 
				JOIN kg_nodes n2 ON n1.node_type = n2.node_type AND n1.id < n2.id
				WHERE n1.node_type IS NOT NULL AND n1.node_type != 'activity_entity' AND n1.node_type != 'unknown'
				  AND %s AND %s
				LIMIT ?
			)
			UNION
			SELECT id1, label1, id2, label2, reason FROM (
				SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_ip' as reason
				FROM kg_nodes n1 
				JOIN kg_nodes n2 ON json_extract(n1.properties, '$.ip') = json_extract(n2.properties, '$.ip') AND n1.id < n2.id
				WHERE NULLIF(TRIM(json_extract(n1.properties, '$.ip')), '') IS NOT NULL
				  AND %s AND %s
				LIMIT ?
			)
			UNION
			SELECT id1, label1, id2, label2, reason FROM (
				SELECT n1.id as id1, n1.label as label1, n2.id as id2, n2.label as label2, 'same_location' as reason
				FROM kg_nodes n1 
				JOIN kg_nodes n2 ON json_extract(n1.properties, '$.location') = json_extract(n2.properties, '$.location') AND n1.id < n2.id
				WHERE NULLIF(TRIM(json_extract(n1.properties, '$.location')), '') IS NOT NULL
				  AND %s AND %s
				LIMIT ?
			)
		) results
		WHERE NOT EXISTS (
			SELECT 1 FROM kg_edges e 
			WHERE (e.source = results.id1 AND e.target = results.id2) OR (e.source = results.id2 AND e.target = results.id1)
		)
		LIMIT ?`, n1Eligible, n2Eligible, n1Eligible, n2Eligible, n1Eligible, n2Eligible)

	rows, err := kg.db.Query(query, args...)
	if err != nil {
		if kg.logger != nil {
			kg.logger.Warn("SuggestRelations: query failed", "error", err)
		}
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
				"reason":       reason,
			})
		}
	}
	if len(suggestions) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(suggestions)
	return string(data)
}
