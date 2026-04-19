package memory

import (
	"aurago/internal/dbutil"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func (kg *KnowledgeGraph) Search(query string) string {
	if query == "" {
		return "[]"
	}

	var matchedNodes []Node
	var matchedEdges []Edge
	var matchedNodeIDs []string
	var matchedEdgeHits []knowledgeGraphAccessHit

	ftsQuery := escapeFTS5(query)
	escapedLike := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(query)
	likePattern := "%" + escapedLike + "%"
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE rowid IN (SELECT rowid FROM kg_nodes_fts WHERE kg_nodes_fts MATCH ?)
		UNION
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE id LIKE ? ESCAPE '\' OR label LIKE ? ESCAPE '\' OR properties LIKE ? ESCAPE '\'
		LIMIT 50
	`, ftsQuery, likePattern, likePattern, likePattern)
	if err != nil {
		kg.logger.Warn("Search: node query failed", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "Search", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				matchedNodes = append(matchedNodes, n)
				matchedNodeIDs = append(matchedNodeIDs, n.ID)
			}
		}
	}

	escapedLikeEdge := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.ToLower(query))
	likeQ := "%" + escapedLikeEdge + "%"
	edgeFTSQuery := escapeFTS5(query)
	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE id IN (SELECT rowid FROM kg_edges_fts WHERE kg_edges_fts MATCH ?)
		UNION
		SELECT source, target, relation, properties FROM kg_edges
		WHERE LOWER(source) LIKE ? ESCAPE '\' OR LOWER(target) LIKE ? ESCAPE '\' OR LOWER(relation) LIKE ? ESCAPE '\' OR LOWER(properties) LIKE ? ESCAPE '\'
		LIMIT 50
	`, edgeFTSQuery, likeQ, likeQ, likeQ, likeQ)
	if err != nil {
		kg.logger.Warn("Search: edge query failed", "error", err)
	} else {
		defer edgeRows.Close()
		for edgeRows.Next() {
			var e Edge
			var propsJSON string
			if err := edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
				if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
					kg.logger.Warn("Search: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
				}
				if e.Properties == nil {
					e.Properties = make(map[string]string)
				}
				matchedEdges = append(matchedEdges, e)
				matchedEdgeHits = append(matchedEdgeHits, knowledgeGraphAccessHit{
					source:   e.Source,
					target:   e.Target,
					relation: e.Relation,
				})
			}
		}
	}

	if len(matchedNodes) == 0 && len(matchedEdges) == 0 {
		return "[]"
	}

	result := map[string]interface{}{
		"nodes": matchedNodes,
		"edges": matchedEdges,
	}
	data, _ := json.Marshal(result)

	for _, id := range matchedNodeIDs {
		kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: id})
	}
	for _, hit := range matchedEdgeHits {
		kg.enqueueAccessHit(hit)
	}

	return string(data)
}

func (kg *KnowledgeGraph) GetNeighbors(nodeID string, limit int) ([]Node, []Edge) {
	if limit <= 0 {
		limit = 20
	}

	var edges []Edge
	rows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE source = ? OR target = ?
		LIMIT ?
	`, nodeID, nodeID, limit)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	neighborIDs := make(map[string]bool)
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err == nil {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				kg.logger.Warn("GetNeighbors: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
			if e.Source != nodeID {
				neighborIDs[e.Source] = true
			}
			if e.Target != nodeID {
				neighborIDs[e.Target] = true
			}
		}
	}

	var nodes []Node
	if len(neighborIDs) > 0 {
		var ids []interface{}
		var placeholders []string
		for id := range neighborIDs {
			ids = append(ids, id)
			placeholders = append(placeholders, "?")
		}

		query := "SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (" + strings.Join(placeholders, ",") + ")"
		rows, err := kg.db.Query(query, ids...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var n Node
				var propsJSON string
				var protected int
				if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
					n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNeighbors", n.ID, propsJSON, protected)
					n.Protected = protected != 0
					nodes = append(nodes, n)
				}
			}
		} else {
			kg.logger.Warn("GetNeighbors: batch node query failed", "error", err)
		}
	}

	kg.enqueueAccessHit(knowledgeGraphAccessHit{nodeID: nodeID})
	for _, e := range edges {
		kg.enqueueAccessHit(knowledgeGraphAccessHit{source: e.Source, target: e.Target, relation: e.Relation})
	}

	return nodes, edges
}

func (kg *KnowledgeGraph) SearchForContext(query string, maxNodes int, maxChars int) string {
	if query == "" || maxNodes <= 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 2000
	}

	var nodeIDs []string
	type searchHit struct {
		score float32
		id    string
	}
	hits := make(map[string]float32)

	semScores := kg.semanticSearchNodeScores(query, maxNodes*2)
	for id, score := range semScores {
		hits[id] += score * 0.5
	}

	ftsQuery := escapeFTS5(query)
	rows, err := kg.db.Query(`
		SELECT n.id, n.access_count FROM kg_nodes_fts f
		JOIN kg_nodes n ON n.rowid = f.rowid
		WHERE kg_nodes_fts MATCH ?
		ORDER BY n.updated_at DESC
		LIMIT ?
	`, ftsQuery, maxNodes)

	count := 0
	if err != nil {
		kg.logger.Warn("SearchForContext: FTS5 query failed", "error", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id string
			var ac sql.NullInt64
			if rows.Scan(&id, &ac) == nil {
				ftsScore := float32(0.4) - (float32(count) * 0.05)
				if ftsScore < 0.1 {
					ftsScore = 0.1
				}
				accessBoost := float32(0)
				if ac.Valid && ac.Int64 > 0 {
					accessBoost = float32(ac.Int64) / 100.0
					if accessBoost > 0.1 {
						accessBoost = 0.1
					}
				}
				hits[id] += ftsScore + accessBoost
				count++
			}
		}
	}

	if count == 0 {
		likeQ := "%" + dbutil.EscapeLike(query) + "%"
		likeRows, err := kg.db.Query(`
			SELECT id, access_count FROM kg_nodes
			WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
			ORDER BY updated_at DESC, access_count DESC
			LIMIT ?
		`, likeQ, likeQ, likeQ, maxNodes)
		if err != nil {
			kg.logger.Warn("SearchForContext: LIKE fallback query failed", "error", err)
		} else {
			defer likeRows.Close()
			for likeRows.Next() {
				var id string
				var ac sql.NullInt64
				if likeRows.Scan(&id, &ac) == nil {
					likeScore := float32(0.3) - (float32(count) * 0.05)
					if likeScore < 0.1 {
						likeScore = 0.1
					}
					hits[id] += likeScore
					count++
				}
			}
		}
	}

	var rankedHits []searchHit
	for id, score := range hits {
		rankedHits = append(rankedHits, searchHit{score: score, id: id})
	}

	sort.Slice(rankedHits, func(i, j int) bool {
		return rankedHits[i].score > rankedHits[j].score
	})

	for i, hit := range rankedHits {
		if i >= maxNodes {
			break
		}
		nodeIDs = append(nodeIDs, hit.id)
	}

	if len(nodeIDs) == 0 {
		return ""
	}

	nodesByID, edgesByNodeID, accessHits := kg.loadSearchContextData(nodeIDs)

	var sb strings.Builder
	for _, nid := range nodeIDs {
		node, ok := nodesByID[nid]
		if !ok {
			continue
		}

		sb.WriteString(fmt.Sprintf("- [%s] %s", nid, node.Label))
		for k, v := range node.Properties {
			if k == "access_count" || k == "protected" || k == "source" || k == "extracted_at" {
				continue
			}
			sb.WriteString(fmt.Sprintf(" | %s: %s", k, v))
		}
		sb.WriteString("\n")

		for _, edge := range edgesByNodeID[nid] {
			sb.WriteString(fmt.Sprintf("  - [%s] -[%s]-> [%s]\n", edge.Source, edge.Relation, edge.Target))
		}

		if sb.Len() > maxChars {
			break
		}
	}

	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}

	result := sb.String()
	if len(result) > maxChars {
		result = truncateUTF8Safe(result, maxChars)
	}
	return result
}

func (kg *KnowledgeGraph) loadSearchContextData(nodeIDs []string) (map[string]Node, map[string][]Edge, []knowledgeGraphAccessHit) {
	nodesByID := make(map[string]Node, len(nodeIDs))
	edgesByNodeID := make(map[string][]Edge, len(nodeIDs))
	accessHits := make([]knowledgeGraphAccessHit, 0, len(nodeIDs)*6)
	if len(nodeIDs) == 0 {
		return nodesByID, edgesByNodeID, accessHits
	}

	placeholders := make([]string, len(nodeIDs))
	nodeArgs := make([]interface{}, len(nodeIDs))
	nodeIDSet := make(map[string]struct{}, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		placeholders[i] = "?"
		nodeArgs[i] = nodeID
		nodeIDSet[nodeID] = struct{}{}
	}

	nodeRows, err := kg.db.Query(
		fmt.Sprintf("SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (%s)", strings.Join(placeholders, ",")),
		nodeArgs...,
	)
	if err != nil {
		kg.logger.Warn("SearchForContext: batch node query failed", "error", err)
	} else {
		defer nodeRows.Close()
		for nodeRows.Next() {
			var node Node
			var propsJSON string
			var protected int
			if nodeRows.Scan(&node.ID, &node.Label, &propsJSON, &protected) == nil {
				node.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "SearchForContext", node.ID, propsJSON, protected)
				node.Protected = protected != 0
				nodesByID[node.ID] = node
				accessHits = append(accessHits, knowledgeGraphAccessHit{nodeID: node.ID})
			}
		}
	}

	edgeArgs := make([]interface{}, 0, len(nodeIDs)*2)
	edgeArgs = append(edgeArgs, nodeArgs...)
	edgeArgs = append(edgeArgs, nodeArgs...)
	edgeRows, err := kg.db.Query(
		fmt.Sprintf(`
			SELECT source, target, relation
			FROM kg_edges
			WHERE source IN (%[1]s) OR target IN (%[1]s)
			ORDER BY access_count DESC
		`, strings.Join(placeholders, ",")),
		edgeArgs...,
	)
	if err != nil {
		kg.logger.Warn("SearchForContext: batch edge query failed", "error", err)
		return nodesByID, edgesByNodeID, accessHits
	}
	defer edgeRows.Close()

	edgeCounts := make(map[string]int, len(nodeIDs))
	for edgeRows.Next() {
		var edge Edge
		if edgeRows.Scan(&edge.Source, &edge.Target, &edge.Relation) != nil {
			continue
		}
		recorded := false
		for _, nodeID := range []string{edge.Source, edge.Target} {
			if _, ok := nodeIDSet[nodeID]; !ok || edgeCounts[nodeID] >= 5 {
				continue
			}
			edgesByNodeID[nodeID] = append(edgesByNodeID[nodeID], edge)
			edgeCounts[nodeID]++
			recorded = true
		}
		if recorded {
			accessHits = append(accessHits, knowledgeGraphAccessHit{
				source: edge.Source, target: edge.Target, relation: edge.Relation,
			})
		}
	}

	return nodesByID, edgesByNodeID, accessHits
}

func (kg *KnowledgeGraph) GetSubgraph(centerNodeID string, maxDepth int) ([]Node, []Edge) {
	if kg == nil || maxDepth <= 0 || strings.TrimSpace(centerNodeID) == "" {
		return nil, nil
	}
	if maxDepth > 3 {
		maxDepth = 3
	}

	center, err := kg.GetNode(centerNodeID)
	if err != nil || center == nil {
		return nil, nil
	}

	visited := make(map[string]bool)
	allNodes := make(map[string]Node)
	allEdges := make(map[string]Edge)
	allNodes[centerNodeID] = *center
	visited[centerNodeID] = true

	queue := []kgBFSLevel{{centerNodeID, 0}}
	for len(queue) > 0 {
		var levelNodeIDs []string
		maxDepthInLevel := queue[0].depth
		for _, item := range queue {
			if item.depth >= maxDepth {
				continue
			}
			levelNodeIDs = append(levelNodeIDs, item.nodeID)
		}
		if len(levelNodeIDs) == 0 {
			break
		}

		var discoveredEdges []Edge
		var neighborIDs []string
		placeholders := make([]string, len(levelNodeIDs))
		batchArgs := make([]interface{}, len(levelNodeIDs)*2)
		for i, nid := range levelNodeIDs {
			placeholders[i] = "?"
			batchArgs[i] = nid
			batchArgs[len(levelNodeIDs)+i] = nid
		}
		batchEdgeQuery := fmt.Sprintf(
			`SELECT source, target, relation, properties FROM kg_edges WHERE source IN (%s) OR target IN (%s)`,
			strings.Join(placeholders, ","),
			strings.Join(placeholders, ","),
		)
		batchRows, batchErr := kg.db.Query(batchEdgeQuery, batchArgs...)
		if batchErr != nil {
			kg.logger.Warn("GetSubgraph: batch edge query failed", "error", batchErr)
		} else {
			for batchRows.Next() {
				var e Edge
				var propsJSON string
				if batchRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON) == nil {
					json.Unmarshal([]byte(propsJSON), &e.Properties)
					if e.Properties == nil {
						e.Properties = make(map[string]string)
					}
					edgeKey := knowledgeGraphEdgeKey(e.Source, e.Target, e.Relation)
					if _, exists := allEdges[edgeKey]; !exists {
						allEdges[edgeKey] = e
						discoveredEdges = append(discoveredEdges, e)
					}
					if !visited[e.Source] {
						neighborIDs = append(neighborIDs, e.Source)
					}
					if !visited[e.Target] {
						neighborIDs = append(neighborIDs, e.Target)
					}
				}
			}
			batchRows.Close()
		}

		if len(neighborIDs) == 0 {
			break
		}

		uniqueNeighborIDs := make([]string, 0, len(neighborIDs))
		seen := make(map[string]bool, len(neighborIDs))
		for _, id := range neighborIDs {
			if !seen[id] && !visited[id] {
				seen[id] = true
				visited[id] = true
				uniqueNeighborIDs = append(uniqueNeighborIDs, id)
			}
		}

		batchNodes := kg.batchGetNodes(uniqueNeighborIDs)
		for _, n := range batchNodes {
			allNodes[n.ID] = n
			visited[n.ID] = true
		}

		queue = make([]kgBFSLevel, 0, len(uniqueNeighborIDs))
		for _, id := range uniqueNeighborIDs {
			if visited[id] {
				queue = append(queue, kgBFSLevel{id, maxDepthInLevel + 1})
			}
		}
	}

	nodes := make([]Node, 0, len(allNodes))
	for _, n := range allNodes {
		nodes = append(nodes, n)
	}
	edgeList := make([]Edge, 0, len(allEdges))
	for _, e := range allEdges {
		edgeList = append(edgeList, e)
	}
	return nodes, edgeList
}

func (kg *KnowledgeGraph) QualityReport(sampleLimit int) (*KnowledgeGraphQualityReport, error) {
	if sampleLimit <= 0 {
		sampleLimit = 5
	}
	if sampleLimit > 50 {
		sampleLimit = 50
	}

	report := &KnowledgeGraphQualityReport{
		IsolatedSample: make([]Node, 0, sampleLimit),
		UntypedSample:  make([]Node, 0, sampleLimit),
	}

	tx, err := kg.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin quality report transaction: %w", err)
	}
	defer tx.Rollback()

	tx.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.Nodes)
	tx.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.Edges)
	tx.QueryRow("SELECT COUNT(*) FROM kg_nodes WHERE protected != 0").Scan(&report.ProtectedNodes)

	tx.QueryRow(`SELECT COUNT(*) FROM kg_nodes n WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)`).Scan(&report.IsolatedNodes)

	isolatedRows, _ := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)
		LIMIT ?`, sampleLimit)
	if isolatedRows != nil {
		defer isolatedRows.Close()
		for isolatedRows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := isolatedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				report.IsolatedSample = append(report.IsolatedSample, n)
			}
		}
	}

	tx.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
	`).Scan(&report.UntypedNodes)

	untypedRows, _ := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
		LIMIT ?`, sampleLimit)
	if untypedRows != nil {
		defer untypedRows.Close()
		for untypedRows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := untypedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				report.UntypedSample = append(report.UntypedSample, n)
			}
		}
	}

	dupGroupRows, _ := tx.Query(`
		SELECT LOWER(TRIM(label)), COUNT(*) 
		FROM kg_nodes 
		WHERE label != ''
		GROUP BY LOWER(TRIM(label)) 
		HAVING COUNT(*) > 1
	`)
	if dupGroupRows != nil {
		defer dupGroupRows.Close()
		var labels []string
		for dupGroupRows.Next() {
			var label string
			var count int
			if err := dupGroupRows.Scan(&label, &count); err == nil {
				report.DuplicateGroups++
				report.DuplicateNodes += count
				if len(labels) < sampleLimit {
					labels = append(labels, label)
				}
			}
		}

		for _, l := range labels {
			cand := KnowledgeGraphDuplicateCandidate{
				Label:           l,
				NormalizedLabel: l,
				Count:           0,
			}
			nodesRows, _ := tx.Query(`SELECT id, label, properties, protected FROM kg_nodes WHERE LOWER(TRIM(label)) = ?`, l)
			if nodesRows != nil {
				for nodesRows.Next() {
					var n Node
					var propsJSON string
					var protected int
					if err := nodesRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err == nil {
						cand.IDs = append(cand.IDs, n.ID)
						cand.Count++
					}
				}
				nodesRows.Close()
			}
			report.DuplicateCandidates = append(report.DuplicateCandidates, cand)
		}
	}

	return report, nil
}

func (kg *KnowledgeGraph) OptimizeGraph(threshold int) (int, error) {
	rows, err := kg.db.Query(`
		SELECT n.id, n.access_count,
			(SELECT COUNT(*) FROM kg_edges e WHERE e.source = n.id OR e.target = n.id) as degree
		FROM kg_nodes n
		WHERE n.protected = 0
	`)
	if err != nil {
		return 0, fmt.Errorf("query for optimization: %w", err)
	}
	defer rows.Close()

	var toRemove []string
	for rows.Next() {
		var id string
		var accessCount, degree int
		if err := rows.Scan(&id, &accessCount, &degree); err == nil {
			priority := accessCount + (degree * 2)
			if priority < threshold {
				toRemove = append(toRemove, id)
			}
		}
	}

	if len(toRemove) == 0 {
		return 0, nil
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	nodesDeleted := 0
	for _, id := range toRemove {
		if _, execErr := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); execErr != nil {
			kg.logger.Warn("OptimizeGraph: failed to delete edges for node", "id", id, "error", execErr)
		}
		if _, execErr := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); execErr != nil {
			kg.logger.Warn("OptimizeGraph: failed to delete node", "id", id, "error", execErr)
		} else {
			nodesDeleted++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nodesDeleted, nil
}

func (kg *KnowledgeGraph) CleanupStaleGraph(thresholdDays int) (int, int, error) {
	if thresholdDays <= 0 {
		return 0, 0, fmt.Errorf("invalid thresholdDays: %d", thresholdDays)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin cleanup graph: %w", err)
	}
	defer tx.Rollback()

	edgeRes, err := tx.Exec(`
		DELETE FROM kg_edges 
		WHERE relation = 'co_mentioned_with' 
		  AND json_extract(properties, '$.source') = 'pending'
		  AND created_at <= datetime('now', '-' || ? || ' days')
	`, thresholdDays)
	if err != nil {
		return 0, 0, fmt.Errorf("delete stale pending edges: %w", err)
	}
	edgesDeleted, _ := edgeRes.RowsAffected()

	rows, err := tx.Query(`
		SELECT id FROM kg_nodes
		WHERE access_count = 0 
		  AND protected = 0
		  AND updated_at <= datetime('now', '-' || ? || ' days')
	`, thresholdDays)
	if err != nil {
		return 0, 0, fmt.Errorf("query unaccessed nodes: %w", err)
	}
	defer rows.Close()

	var toRemove []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		if _, execErr := tx.Exec("DELETE FROM kg_edges WHERE source = ? OR target = ?", id, id); execErr != nil {
			kg.logger.Warn("CleanupStaleGraph: failed to delete edges for node", "id", id, "error", execErr)
		}
		if _, execErr := tx.Exec("DELETE FROM kg_nodes WHERE id = ?", id); execErr != nil {
			kg.logger.Warn("CleanupStaleGraph: failed to delete node", "id", id, "error", execErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit cleanup graph: %w", err)
	}

	return int(edgesDeleted), len(toRemove), nil
}

func (kg *KnowledgeGraph) GetStats() (*KnowledgeGraphStats, error) {
	stats := &KnowledgeGraphStats{
		ByType:   make(map[string]int),
		BySource: make(map[string]int),
	}

	kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&stats.TotalNodes)
	kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&stats.TotalEdges)
	kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE relation = 'co_mentioned_with'").Scan(&stats.CoMentionEdges)
	stats.MeaningfulEdges = stats.TotalEdges - stats.CoMentionEdges

	typeRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.type'), ''), 'untyped') AS t, COUNT(*)
		FROM kg_nodes GROUP BY t ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var t string
			var c int
			if typeRows.Scan(&t, &c) == nil {
				stats.ByType[t] = c
			}
		}
	}

	sourceRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.source'), ''), 'unknown') AS s, COUNT(*)
		FROM kg_nodes GROUP BY s ORDER BY COUNT(*) DESC
	`)
	if err == nil {
		defer sourceRows.Close()
		for sourceRows.Next() {
			var s string
			var c int
			if sourceRows.Scan(&s, &c) == nil {
				stats.BySource[s] = c
			}
		}
	}

	return stats, nil
}

func escapeFTS5(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}
	var escaped []string
	for _, w := range words {
		w = strings.ReplaceAll(w, `"`, ``)
		if w != "" {
			escaped = append(escaped, `"`+w+`"`)
		}
	}
	if len(escaped) == 0 {
		return `""`
	}
	return strings.Join(escaped, " OR ")
}

func truncateUTF8Safe(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}

func buildKnowledgeGraphDuplicateCandidates(groups map[string][]Node) []KnowledgeGraphDuplicateCandidate {
	candidates := make([]KnowledgeGraphDuplicateCandidate, 0, len(groups))
	for normalized, nodes := range groups {
		if len(nodes) < 2 {
			continue
		}
		sort.Slice(nodes, func(i, j int) bool {
			left := strings.TrimSpace(nodes[i].Label)
			right := strings.TrimSpace(nodes[j].Label)
			if left != right {
				return left < right
			}
			return nodes[i].ID < nodes[j].ID
		})

		ids := make([]string, 0, len(nodes))
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		candidates = append(candidates, KnowledgeGraphDuplicateCandidate{
			Label:           nodes[0].Label,
			NormalizedLabel: normalized,
			Count:           len(nodes),
			IDs:             ids,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].Label < candidates[j].Label
	})
	return candidates
}

func normalizeKnowledgeGraphDuplicateLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return ""
	}
	return strings.Join(strings.Fields(label), " ")
}
