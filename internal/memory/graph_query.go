package memory

import (
	"aurago/internal/dbutil"
	"aurago/internal/security"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

type semanticEdgeIdentity struct {
	source   string
	target   string
	relation string
}

type knowledgeGraphQueryer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func (kg *KnowledgeGraph) beginReadTx(operation string) (*sql.Tx, error) {
	tx, err := kg.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		if kg.logger != nil {
			kg.logger.Warn(operation+": begin read transaction failed", "error", err)
		}
		return nil, err
	}
	return tx, nil
}

func (kg *KnowledgeGraph) Search(query string) string {
	if query == "" {
		return "[]"
	}

	tx, err := kg.beginReadTx("Search")
	if err != nil {
		return "[]"
	}
	defer tx.Rollback()

	var matchedNodes []Node
	var matchedEdges []Edge
	var matchedNodeIDs []string
	var matchedEdgeHits []knowledgeGraphAccessHit

	ftsQuery := escapeFTS5(query)
	escapedLike := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(query)
	likePattern := "%" + escapedLike + "%"
	rows, err := tx.Query(`
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
		seenNodes := make(map[string]struct{})
		for rows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
				kg.logger.Warn("Search: scan node failed", "error", err)
				continue
			}
			if _, exists := seenNodes[n.ID]; exists {
				continue
			}
			seenNodes[n.ID] = struct{}{}
			n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "Search", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			matchedNodes = append(matchedNodes, n)
			matchedNodeIDs = append(matchedNodeIDs, n.ID)
		}
		if err := rows.Err(); err != nil {
			kg.logger.Warn("Search: iterate nodes failed", "error", err)
		}
		rows.Close()
	}

	escapedLikeEdge := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.ToLower(query))
	likeQ := "%" + escapedLikeEdge + "%"
	edgeFTSQuery := escapeFTS5(query)
	edgeRows, err := tx.Query(`
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
		seenEdges := make(map[string]struct{})
		for edgeRows.Next() {
			var e Edge
			var propsJSON string
			if err := edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err != nil {
				kg.logger.Warn("Search: scan edge failed", "error", err)
				continue
			}
			edgeKey := knowledgeGraphEdgeKey(e.Source, e.Target, e.Relation)
			if _, exists := seenEdges[edgeKey]; exists {
				continue
			}
			seenEdges[edgeKey] = struct{}{}
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
		if err := edgeRows.Err(); err != nil {
			kg.logger.Warn("Search: iterate edges failed", "error", err)
		}
		edgeRows.Close()
	}

	if len(matchedNodes) == 0 && len(matchedEdges) == 0 {
		return "[]"
	}
	if err := tx.Commit(); err != nil {
		kg.logger.Warn("Search: commit read transaction failed", "error", err)
		if len(matchedNodes) == 0 && len(matchedEdges) == 0 {
			return "[]"
		}
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

	tx, err := kg.beginReadTx("GetNeighbors")
	if err != nil {
		return nil, nil
	}
	defer tx.Rollback()

	nodes, edges, accessHits, err := kg.getNeighborsWithQueryer(tx, nodeID, limit)
	if err != nil {
		kg.logger.Warn("GetNeighbors: read failed", "node_id", nodeID, "error", err)
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		kg.logger.Warn("GetNeighbors: commit read transaction failed", "node_id", nodeID, "error", err)
		return nil, nil
	}
	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}

	return nodes, edges
}

func (kg *KnowledgeGraph) getNeighborsWithQueryer(q knowledgeGraphQueryer, nodeID string, limit int) ([]Node, []Edge, []knowledgeGraphAccessHit, error) {
	if limit <= 0 {
		limit = 20
	}

	var allEdges []Edge
	rows, err := q.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE source = ? OR target = ?
		ORDER BY updated_at DESC
	`, nodeID, nodeID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	type neighborCandidate struct {
		id string
	}
	neighborOrder := make([]neighborCandidate, 0)
	seenNeighbors := make(map[string]struct{})
	for rows.Next() {
		var e Edge
		var propsJSON string
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON); err != nil {
			return nil, nil, nil, fmt.Errorf("scan neighbor edge: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
			kg.logger.Warn("GetNeighbors: corrupt edge properties JSON", "source", e.Source, "target", e.Target, "error", err)
		}
		if e.Properties == nil {
			e.Properties = make(map[string]string)
		}
		allEdges = append(allEdges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("iterate neighbor edges: %w", err)
	}

	for _, e := range allEdges {
		neighborID := e.Target
		if neighborID == nodeID {
			neighborID = e.Source
		}
		if _, exists := seenNeighbors[neighborID]; exists {
			continue
		}
		seenNeighbors[neighborID] = struct{}{}
		neighborOrder = append(neighborOrder, neighborCandidate{id: neighborID})
		if len(neighborOrder) >= limit {
			break
		}
	}

	selectedNeighbors := make(map[string]struct{}, len(neighborOrder))
	for _, candidate := range neighborOrder {
		selectedNeighbors[candidate.id] = struct{}{}
	}

	var edges []Edge
	accessHits := []knowledgeGraphAccessHit{{nodeID: nodeID}}
	for _, e := range allEdges {
		otherID := e.Target
		if otherID == nodeID {
			otherID = e.Source
		}
		if _, ok := selectedNeighbors[otherID]; ok {
			edges = append(edges, e)
			accessHits = append(accessHits, knowledgeGraphAccessHit{source: e.Source, target: e.Target, relation: e.Relation})
		}
	}

	var nodes []Node
	if len(selectedNeighbors) > 0 {
		var ids []interface{}
		var placeholders []string
		for _, candidate := range neighborOrder {
			ids = append(ids, candidate.id)
			placeholders = append(placeholders, "?")
		}

		query := "SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (" + strings.Join(placeholders, ",") + ")"
		rows, err := q.Query(query, ids...)
		if err == nil {
			defer rows.Close()
			nodeByID := make(map[string]Node, len(neighborOrder))
			for rows.Next() {
				var n Node
				var propsJSON string
				var protected int
				if err := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
					return nil, nil, nil, fmt.Errorf("scan neighbor node: %w", err)
				}
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "GetNeighbors", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				nodeByID[n.ID] = n
			}
			if err := rows.Err(); err != nil {
				return nil, nil, nil, fmt.Errorf("iterate neighbor nodes: %w", err)
			}
			for _, candidate := range neighborOrder {
				if n, ok := nodeByID[candidate.id]; ok {
					nodes = append(nodes, n)
				}
			}
		} else {
			return nil, nil, nil, fmt.Errorf("query neighbor nodes: %w", err)
		}
	}

	return nodes, edges, accessHits, nil
}

func appendKnowledgeGraphContextProperties(sb *strings.Builder, properties map[string]string) {
	for k, v := range properties {
		if k == "access_count" || k == "protected" || k == "source" || k == "extracted_at" {
			continue
		}
		if isSensitiveKnowledgeGraphPropertyKey(k) {
			continue
		}
		sb.WriteString(fmt.Sprintf(" | %s: %s", k, v))
	}
}

func finalizeKnowledgeGraphContextResult(sb strings.Builder, maxChars int) string {
	result := sb.String()
	if len(result) > maxChars {
		result = truncateUTF8Safe(result, maxChars)
	}
	return security.Scrub(result)
}

func (kg *KnowledgeGraph) SearchForContext(query string, maxNodes int, maxChars int) string {
	if query == "" || maxNodes <= 0 {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 2000
	}

	// Wildcard fallback: return important nodes instead of trying to FTS-match "*".
	if strings.TrimSpace(query) == "*" {
		return kg.searchForContextImportantNodes(maxNodes, maxChars)
	}

	var nodeIDs []string
	type searchHit struct {
		score float32
		id    string
	}
	hits := make(map[string]float32)

	semScores := kg.semanticSearchNodeScores(query, maxNodes*2)

	tx, err := kg.beginReadTx("SearchForContext")
	if err != nil {
		return ""
	}
	defer tx.Rollback()

	if len(semScores) > 0 {
		semIDs := make([]string, 0, len(semScores))
		for id := range semScores {
			semIDs = append(semIDs, id)
		}
		semNodes, loadErr := loadNodesByIDs(tx, semIDs, kg.logger, "SearchForContext")
		if loadErr != nil {
			kg.logger.Warn("SearchForContext: filter semantic hits failed", "error", loadErr)
		} else {
			allowed := make(map[string]struct{}, len(semNodes))
			for _, n := range semNodes {
				if !kg.isExcludedNodeType(n.Properties["type"]) {
					allowed[n.ID] = struct{}{}
				}
			}
			for id, score := range semScores {
				if _, ok := allowed[id]; ok {
					hits[id] += score * 0.5
				}
			}
		}
	}

	ftsQuery := escapeFTS5(query)
	rows, err := tx.Query(`
		SELECT n.id, n.access_count FROM kg_nodes_fts f
		JOIN kg_nodes n ON n.rowid = f.rowid
		WHERE kg_nodes_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, maxNodes)

	count := 0
	if err != nil {
		kg.logger.Warn("SearchForContext: FTS5 query failed", "error", err)
	} else {
		for rows.Next() {
			var id string
			var ac sql.NullInt64
			if err := rows.Scan(&id, &ac); err != nil {
				kg.logger.Warn("SearchForContext: scan FTS hit failed", "error", err)
				continue
			}
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
		if err := rows.Err(); err != nil {
			kg.logger.Warn("SearchForContext: iterate FTS hits failed", "error", err)
		}
		rows.Close()
	}

	if count == 0 {
		likeQ := "%" + dbutil.EscapeLike(query) + "%"
		likeRows, err := tx.Query(`
			SELECT id, access_count FROM kg_nodes
			WHERE id LIKE ? OR label LIKE ? OR properties LIKE ?
			ORDER BY updated_at DESC, access_count DESC
			LIMIT ?
		`, likeQ, likeQ, likeQ, maxNodes)
		if err != nil {
			kg.logger.Warn("SearchForContext: LIKE fallback query failed", "error", err)
		} else {
			for likeRows.Next() {
				var id string
				var ac sql.NullInt64
				if err := likeRows.Scan(&id, &ac); err != nil {
					kg.logger.Warn("SearchForContext: scan LIKE hit failed", "error", err)
					continue
				}
				likeScore := float32(0.3) - (float32(count) * 0.05)
				if likeScore < 0.1 {
					likeScore = 0.1
				}
				hits[id] += likeScore
				count++
			}
			if err := likeRows.Err(); err != nil {
				kg.logger.Warn("SearchForContext: iterate LIKE hits failed", "error", err)
			}
			likeRows.Close()
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

	nodesByID, edgesByNodeID, accessHits, err := kg.loadSearchContextData(tx, nodeIDs)
	if err != nil {
		kg.logger.Warn("SearchForContext: load context data failed", "error", err)
		return ""
	}
	if err := tx.Commit(); err != nil {
		kg.logger.Warn("SearchForContext: commit read transaction failed", "error", err)
		return ""
	}

	var sb strings.Builder
	for _, nid := range nodeIDs {
		node, ok := nodesByID[nid]
		if !ok {
			continue
		}
		if kg.isExcludedNodeType(node.Properties["type"]) {
			continue
		}

		sb.WriteString(fmt.Sprintf("- [%s] %s", nid, node.Label))
		appendKnowledgeGraphContextProperties(&sb, node.Properties)
		sb.WriteString("\n")

		for _, edge := range edgesByNodeID[nid] {
			if edge.Relation == "co_mentioned_with" && edge.Properties["source"] != "activity_turn" {
				continue
			}
			sb.WriteString(fmt.Sprintf("  - [%s] -[%s]-> [%s]\n", edge.Source, edge.Relation, edge.Target))
		}

		if sb.Len() > maxChars {
			break
		}
	}

	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}

	return finalizeKnowledgeGraphContextResult(sb, maxChars)
}

// searchForContextImportantNodes returns a formatted context string built from
// the most important nodes. It is used as a fallback for wildcard queries.
func (kg *KnowledgeGraph) searchForContextImportantNodes(maxNodes int, maxChars int) string {
	if maxNodes <= 0 {
		maxNodes = 20
	}
	if maxChars <= 0 {
		maxChars = 2000
	}

	nodes, err := kg.GetImportantNodes(maxNodes, 15)
	if err != nil || len(nodes) == 0 {
		return ""
	}

	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	tx, err := kg.beginReadTx("SearchForContextImportantNodes")
	if err != nil {
		return ""
	}
	defer tx.Rollback()

	nodesByID, edgesByNodeID, accessHits, err := kg.loadSearchContextData(tx, nodeIDs)
	if err != nil {
		kg.logger.Warn("SearchForContext: load important nodes context data failed", "error", err)
		return ""
	}
	if err := tx.Commit(); err != nil {
		kg.logger.Warn("SearchForContext: commit important nodes read transaction failed", "error", err)
		return ""
	}

	var sb strings.Builder
	for _, n := range nodes {
		node, ok := nodesByID[n.ID]
		if !ok {
			continue
		}
		if kg.isExcludedNodeType(node.Properties["type"]) {
			continue
		}

		sb.WriteString(fmt.Sprintf("- [%s] %s", n.ID, node.Label))
		appendKnowledgeGraphContextProperties(&sb, node.Properties)
		sb.WriteString("\n")

		for _, edge := range edgesByNodeID[n.ID] {
			if edge.Relation == "co_mentioned_with" && edge.Properties["source"] != "activity_turn" {
				continue
			}
			sb.WriteString(fmt.Sprintf("  - [%s] -[%s]-> [%s]\n", edge.Source, edge.Relation, edge.Target))
		}

		if sb.Len() > maxChars {
			break
		}
	}

	for _, hit := range accessHits {
		kg.enqueueAccessHit(hit)
	}

	return finalizeKnowledgeGraphContextResult(sb, maxChars)
}

func (kg *KnowledgeGraph) loadSearchContextData(q knowledgeGraphQueryer, nodeIDs []string) (map[string]Node, map[string][]Edge, []knowledgeGraphAccessHit, error) {
	nodesByID := make(map[string]Node, len(nodeIDs))
	edgesByNodeID := make(map[string][]Edge, len(nodeIDs))
	accessHits := make([]knowledgeGraphAccessHit, 0, len(nodeIDs)*6)
	if len(nodeIDs) == 0 {
		return nodesByID, edgesByNodeID, accessHits, nil
	}

	placeholders := make([]string, len(nodeIDs))
	nodeArgs := make([]interface{}, len(nodeIDs))
	nodeIDSet := make(map[string]struct{}, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		placeholders[i] = "?"
		nodeArgs[i] = nodeID
		nodeIDSet[nodeID] = struct{}{}
	}

	nodeRows, err := q.Query(
		fmt.Sprintf("SELECT id, label, properties, protected FROM kg_nodes WHERE id IN (%s)", strings.Join(placeholders, ",")),
		nodeArgs...,
	)
	if err != nil {
		return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("batch node query: %w", err)
	} else {
		defer nodeRows.Close()
		for nodeRows.Next() {
			var node Node
			var propsJSON string
			var protected int
			if err := nodeRows.Scan(&node.ID, &node.Label, &propsJSON, &protected); err != nil {
				return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("scan context node: %w", err)
			}
			node.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "SearchForContext", node.ID, propsJSON, protected)
			node.Protected = protected != 0
			nodesByID[node.ID] = node
			accessHits = append(accessHits, knowledgeGraphAccessHit{nodeID: node.ID})
		}
		if err := nodeRows.Err(); err != nil {
			return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("iterate context nodes: %w", err)
		}
	}

	edgeArgs := make([]interface{}, 0, len(nodeIDs)*2)
	edgeArgs = append(edgeArgs, nodeArgs...)
	edgeArgs = append(edgeArgs, nodeArgs...)
	edgeRows, err := q.Query(
		fmt.Sprintf(`
			SELECT source, target, relation
			FROM kg_edges
			WHERE source IN (%[1]s) OR target IN (%[1]s)
			ORDER BY access_count DESC
		`, strings.Join(placeholders, ",")),
		edgeArgs...,
	)
	if err != nil {
		return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("batch edge query: %w", err)
	}
	defer edgeRows.Close()

	edgeCounts := make(map[string]int, len(nodeIDs))
	for edgeRows.Next() {
		var edge Edge
		if err := edgeRows.Scan(&edge.Source, &edge.Target, &edge.Relation); err != nil {
			return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("scan context edge: %w", err)
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
	if err := edgeRows.Err(); err != nil {
		return nodesByID, edgesByNodeID, accessHits, fmt.Errorf("iterate context edges: %w", err)
	}

	return nodesByID, edgesByNodeID, accessHits, nil
}

func (kg *KnowledgeGraph) GetSubgraph(centerNodeID string, maxDepth int) ([]Node, []Edge) {
	if kg == nil || maxDepth <= 0 || strings.TrimSpace(centerNodeID) == "" {
		return nil, nil
	}
	if maxDepth > 3 {
		maxDepth = 3
	}

	tx, err := kg.beginReadTx("GetSubgraph")
	if err != nil {
		return nil, nil
	}
	defer tx.Rollback()

	centerNodes, err := loadNodesByIDs(tx, []string{centerNodeID}, kg.logger, "GetSubgraph")
	if err != nil || len(centerNodes) == 0 {
		return nil, nil
	}
	center := centerNodes[0]

	visited := make(map[string]bool)
	allNodes := make(map[string]Node)
	allEdges := make(map[string]Edge)
	allNodes[centerNodeID] = center
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
		batchRows, batchErr := tx.Query(batchEdgeQuery, batchArgs...)
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

		batchNodes, batchErr := loadNodesByIDs(tx, uniqueNeighborIDs, kg.logger, "GetSubgraph")
		if batchErr != nil {
			kg.logger.Warn("GetSubgraph: batch node query failed", "error", batchErr)
		} else {
			for _, n := range batchNodes {
				allNodes[n.ID] = n
				visited[n.ID] = true
			}
		}

		queue = make([]kgBFSLevel, 0, len(uniqueNeighborIDs))
		for _, id := range uniqueNeighborIDs {
			if visited[id] {
				queue = append(queue, kgBFSLevel{id, maxDepthInLevel + 1})
			}
		}
	}

	if err := tx.Commit(); err != nil && kg.logger != nil {
		kg.logger.Warn("GetSubgraph: commit read transaction failed", "error", err)
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

	if err := tx.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.Nodes); err != nil {
		return nil, fmt.Errorf("count knowledge graph nodes: %w", err)
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.Edges); err != nil {
		return nil, fmt.Errorf("count knowledge graph edges: %w", err)
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM kg_nodes WHERE protected != 0").Scan(&report.ProtectedNodes); err != nil {
		return nil, fmt.Errorf("count protected knowledge graph nodes: %w", err)
	}

	if err := tx.QueryRow(`SELECT COUNT(*) FROM kg_nodes n WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)`).Scan(&report.IsolatedNodes); err != nil {
		return nil, fmt.Errorf("count isolated knowledge graph nodes: %w", err)
	}

	isolatedRows, err := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)
		LIMIT ?`, sampleLimit)
	if err != nil {
		return nil, fmt.Errorf("query isolated knowledge graph sample: %w", err)
	}
	defer isolatedRows.Close()
	for isolatedRows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := isolatedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan isolated knowledge graph sample: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		report.IsolatedSample = append(report.IsolatedSample, n)
	}
	if err := isolatedRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate isolated knowledge graph sample: %w", err)
	}

	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
	`).Scan(&report.UntypedNodes); err != nil {
		return nil, fmt.Errorf("count untyped knowledge graph nodes: %w", err)
	}

	untypedRows, err := tx.Query(`
		SELECT id, label, properties, protected FROM kg_nodes n 
		WHERE json_extract(properties, '$.type') IS NULL OR json_extract(properties, '$.type') = ''
		LIMIT ?`, sampleLimit)
	if err != nil {
		return nil, fmt.Errorf("query untyped knowledge graph sample: %w", err)
	}
	defer untypedRows.Close()
	for untypedRows.Next() {
		var n Node
		var propsJSON string
		var protected int
		if err := untypedRows.Scan(&n.ID, &n.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan untyped knowledge graph sample: %w", err)
		}
		n.Properties = decodeKnowledgeGraphNodeProperties(kg.logger, "QualityReport", n.ID, propsJSON, protected)
		n.Protected = protected != 0
		report.UntypedSample = append(report.UntypedSample, n)
	}
	if err := untypedRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate untyped knowledge graph sample: %w", err)
	}

	dupGroupRows, err := tx.Query(`
		SELECT LOWER(TRIM(label)), COUNT(*) 
		FROM kg_nodes 
		WHERE label != ''
		GROUP BY LOWER(TRIM(label)) 
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return nil, fmt.Errorf("query duplicate knowledge graph groups: %w", err)
	}
	defer dupGroupRows.Close()
	var labels []string
	for dupGroupRows.Next() {
		var label string
		var count int
		if err := dupGroupRows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("scan duplicate knowledge graph group: %w", err)
		}
		report.DuplicateGroups++
		report.DuplicateNodes += count
		if len(labels) < sampleLimit {
			labels = append(labels, label)
		}
	}
	if err := dupGroupRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate duplicate knowledge graph groups: %w", err)
	}

	if len(labels) > 0 {
		labelPlaceholders := knowledgeGraphSQLInPlaceholders(len(labels))
		labelArgs := make([]interface{}, len(labels))
		for i, l := range labels {
			labelArgs[i] = l
		}
		nodesRows, err := tx.Query(fmt.Sprintf(`
			SELECT id, label, properties, protected, LOWER(TRIM(label))
			FROM kg_nodes
			WHERE label != '' AND LOWER(TRIM(label)) IN (%s)
			ORDER BY LOWER(TRIM(label)), id
		`, labelPlaceholders), labelArgs...)
		if err != nil {
			return nil, fmt.Errorf("query duplicate knowledge graph node batch: %w", err)
		}
		candByLabel := make(map[string]*KnowledgeGraphDuplicateCandidate, len(labels))
		for _, l := range labels {
			candByLabel[l] = &KnowledgeGraphDuplicateCandidate{
				Label:           l,
				NormalizedLabel: normalizeKnowledgeGraphDuplicateLabel(l),
			}
		}
		for nodesRows.Next() {
			var n Node
			var propsJSON string
			var protected int
			var normLabel string
			if err := nodesRows.Scan(&n.ID, &n.Label, &propsJSON, &protected, &normLabel); err != nil {
				nodesRows.Close()
				return nil, fmt.Errorf("scan duplicate knowledge graph node batch: %w", err)
			}
			cand, ok := candByLabel[normLabel]
			if !ok {
				continue
			}
			cand.IDs = append(cand.IDs, n.ID)
			cand.Count++
		}
		if err := nodesRows.Err(); err != nil {
			nodesRows.Close()
			return nil, fmt.Errorf("iterate duplicate knowledge graph node batch: %w", err)
		}
		nodesRows.Close()
		for _, l := range labels {
			if cand := candByLabel[l]; cand != nil && cand.Count > 0 {
				report.DuplicateCandidates = append(report.DuplicateCandidates, *cand)
			}
		}
	}

	idDuplicateGroups, idDuplicateNodes, idDuplicateCandidates, err := knowledgeGraphIDDuplicateSummary(kg.logger, tx, sampleLimit)
	if err != nil {
		return nil, err
	}
	report.IDDuplicateGroups = idDuplicateGroups
	report.IDDuplicateNodes = idDuplicateNodes
	report.IDDuplicateCandidates = idDuplicateCandidates

	return report, nil
}

func (kg *KnowledgeGraph) OptimizeGraph(threshold int) (int, error) {
	if err := kg.FlushAccessHits(); err != nil && kg.logger != nil {
		kg.logger.Warn("OptimizeGraph: failed to flush access hits", "error", err)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT n.id, n.access_count, COALESCE(n.source_type, ''),
			(SELECT COUNT(*) FROM kg_edges e WHERE e.source = n.id OR e.target = n.id) as degree
		FROM kg_nodes n
		WHERE n.protected = 0
	`)
	if err != nil {
		return 0, fmt.Errorf("query for optimization: %w", err)
	}

	var toRemove []string
	for rows.Next() {
		var id, source string
		var accessCount, degree int
		if err := rows.Scan(&id, &accessCount, &source, &degree); err == nil {
			if kg.isKnowledgeGraphOptimizeProtected(id, source) {
				continue
			}
			priority := degree * 2
			if kg.accessCountReliable() {
				priority += accessCount
			}
			if priority < threshold {
				toRemove = append(toRemove, id)
			}
		}
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close optimization rows: %w", err)
	}

	if len(toRemove) == 0 {
		return 0, nil
	}

	inPlaceholders := knowledgeGraphSQLInPlaceholders(len(toRemove))
	inArgs := make([]interface{}, len(toRemove))
	for i, id := range toRemove {
		inArgs[i] = id
	}

	edgeArgs := make([]interface{}, 0, len(toRemove)*2)
	for _, id := range toRemove {
		edgeArgs = append(edgeArgs, id)
	}
	for _, id := range toRemove {
		edgeArgs = append(edgeArgs, id)
	}
	removedEdges := kg.collectSemanticEdgeIdentities(tx,
		fmt.Sprintf(`SELECT source, target, relation FROM kg_edges WHERE source IN (%s) OR target IN (%s)`, inPlaceholders, inPlaceholders),
		edgeArgs...,
	)

	deleteRes, execErr := tx.Exec(
		fmt.Sprintf("DELETE FROM kg_nodes WHERE id IN (%s)", inPlaceholders),
		inArgs...,
	)
	if execErr != nil {
		return 0, fmt.Errorf("batch delete optimized nodes: %w", execErr)
	}
	nodesDeleted64, _ := deleteRes.RowsAffected()
	nodesDeleted := int(nodesDeleted64)

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	kg.removeSemanticIndexesForDeletedGraphData(toRemove, removedEdges)
	return nodesDeleted, nil
}

func (kg *KnowledgeGraph) CleanupStaleGraph(thresholdDays int) (int, int, error) {
	if thresholdDays <= 0 {
		return 0, 0, fmt.Errorf("invalid thresholdDays: %d", thresholdDays)
	}

	if err := kg.FlushAccessHits(); err != nil && kg.logger != nil {
		kg.logger.Warn("CleanupStaleGraph: failed to flush access hits", "error", err)
	}

	tx, err := kg.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin cleanup graph: %w", err)
	}
	defer tx.Rollback()

	staleEdges := kg.collectSemanticEdgeIdentities(tx, `
		SELECT source, target, relation FROM kg_edges
		WHERE relation = 'co_mentioned_with'
		  AND json_extract(properties, '$.source') = 'pending'
		  AND created_at <= datetime('now', '-' || ? || ' days')
	`, thresholdDays)

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

	var toRemove []string

	placeholderGrace := thresholdDays
	if placeholderGrace > knowledgeGraphPlaceholderGraceDays {
		placeholderGrace = knowledgeGraphPlaceholderGraceDays
	}

	placeholderRows, err := tx.Query(`
		SELECT id FROM kg_nodes n
		WHERE json_extract(n.properties, '$.source') = ?
		  AND LOWER(TRIM(n.label)) = 'unknown'
		  AND n.protected = 0
		  AND n.updated_at <= datetime('now', '-' || ? || ' days')
		  AND NOT EXISTS (
			SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id
		  )
	`, knowledgeGraphPlaceholderSource, placeholderGrace)
	if err != nil {
		return 0, 0, fmt.Errorf("query stale placeholder nodes: %w", err)
	}
	for placeholderRows.Next() {
		var id string
		if err := placeholderRows.Scan(&id); err == nil {
			toRemove = append(toRemove, id)
		}
	}
	if err := placeholderRows.Close(); err != nil {
		return 0, 0, fmt.Errorf("close stale placeholder rows: %w", err)
	}

	if kg.accessCountReliable() {
		rows, err := tx.Query(`
			SELECT id, COALESCE(source_type, '') FROM kg_nodes
			WHERE access_count = 0
			  AND protected = 0
			  AND updated_at <= datetime('now', '-' || ? || ' days')
		`, thresholdDays)
		if err != nil {
			return 0, 0, fmt.Errorf("query unaccessed nodes: %w", err)
		}

		for rows.Next() {
			var id, source string
			if err := rows.Scan(&id, &source); err == nil {
				if kg.isKnowledgeGraphOptimizeProtected(id, source) {
					continue
				}
				toRemove = append(toRemove, id)
			}
		}
		if err := rows.Close(); err != nil {
			return 0, 0, fmt.Errorf("close stale node rows: %w", err)
		}
	} else if kg.logger != nil {
		kg.logger.Warn("CleanupStaleGraph: skipping access_count stale removal because access hits were dropped",
			"dropped_hits", kg.DroppedAccessHits())
	}

	seenRemove := make(map[string]struct{}, len(toRemove))
	uniqueRemove := make([]string, 0, len(toRemove))
	for _, id := range toRemove {
		if _, ok := seenRemove[id]; ok {
			continue
		}
		seenRemove[id] = struct{}{}
		uniqueRemove = append(uniqueRemove, id)
	}
	toRemove = uniqueRemove

	removedEdges := append([]semanticEdgeIdentity(nil), staleEdges...)
	for _, id := range toRemove {
		removedEdges = append(removedEdges, kg.collectSemanticEdgeIdentities(tx, "SELECT source, target, relation FROM kg_edges WHERE source = ? OR target = ?", id, id)...)
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

	kg.removeSemanticIndexesForDeletedGraphData(toRemove, removedEdges)
	return int(edgesDeleted), len(toRemove), nil
}

func (kg *KnowledgeGraph) collectSemanticEdgeIdentities(tx *sql.Tx, query string, args ...interface{}) []semanticEdgeIdentity {
	if kg.semanticIndex() == nil {
		return nil
	}
	rows, err := tx.Query(query, args...)
	if err != nil {
		if kg.logger != nil {
			kg.logger.Warn("KnowledgeGraph: failed to collect semantic edge identities", "error", err)
		}
		return nil
	}
	defer rows.Close()

	var edges []semanticEdgeIdentity
	for rows.Next() {
		var edge semanticEdgeIdentity
		if err := rows.Scan(&edge.source, &edge.target, &edge.relation); err != nil {
			if kg.logger != nil {
				kg.logger.Warn("KnowledgeGraph: failed to scan semantic edge identity", "error", err)
			}
			continue
		}
		edges = append(edges, edge)
	}
	return edges
}

func (kg *KnowledgeGraph) removeSemanticIndexesForDeletedGraphData(nodeIDs []string, edges []semanticEdgeIdentity) {
	if kg.semanticIndex() == nil {
		return
	}
	seenEdges := make(map[semanticEdgeIdentity]struct{}, len(edges))
	for _, edge := range edges {
		if _, ok := seenEdges[edge]; ok {
			continue
		}
		seenEdges[edge] = struct{}{}
		if err := kg.removeSemanticEdgeIndex(edge.source, edge.target, edge.relation); err != nil && kg.logger != nil {
			kg.logger.Warn("KnowledgeGraph: failed to remove stale semantic edge index", "source", edge.source, "target", edge.target, "relation", edge.relation, "error", err)
		}
	}
	seenNodes := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if _, ok := seenNodes[nodeID]; ok {
			continue
		}
		seenNodes[nodeID] = struct{}{}
		if err := kg.removeSemanticNodeIndex(nodeID); err != nil && kg.logger != nil {
			kg.logger.Warn("KnowledgeGraph: failed to remove stale semantic node index", "id", nodeID, "error", err)
		}
	}
}

func (kg *KnowledgeGraph) GetStats() (*KnowledgeGraphStats, error) {
	stats := &KnowledgeGraphStats{
		ByType:   make(map[string]int),
		BySource: make(map[string]int),
	}

	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&stats.TotalNodes); err != nil {
		return nil, fmt.Errorf("count knowledge graph nodes: %w", err)
	}
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&stats.TotalEdges); err != nil {
		return nil, fmt.Errorf("count knowledge graph edges: %w", err)
	}
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges WHERE relation = 'co_mentioned_with'").Scan(&stats.CoMentionEdges); err != nil {
		return nil, fmt.Errorf("count co-mention edges: %w", err)
	}
	stats.MeaningfulEdges = stats.TotalEdges - stats.CoMentionEdges

	typeRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.type'), ''), 'untyped') AS t, COUNT(*)
		FROM kg_nodes GROUP BY t ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query knowledge graph node types: %w", err)
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var t string
		var c int
		if err := typeRows.Scan(&t, &c); err != nil {
			return nil, fmt.Errorf("scan knowledge graph node type count: %w", err)
		}
		stats.ByType[t] = c
	}
	if err := typeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph node type counts: %w", err)
	}

	sourceRows, err := kg.db.Query(`
		SELECT COALESCE(NULLIF(json_extract(properties, '$.source'), ''), 'unknown') AS s, COUNT(*)
		FROM kg_nodes GROUP BY s ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query knowledge graph node sources: %w", err)
	}
	defer sourceRows.Close()
	for sourceRows.Next() {
		var s string
		var c int
		if err := sourceRows.Scan(&s, &c); err != nil {
			return nil, fmt.Errorf("scan knowledge graph node source count: %w", err)
		}
		stats.BySource[s] = c
	}
	if err := sourceRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph node source counts: %w", err)
	}

	return stats, nil
}

func escapeFTS5(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return `""`
	}
	if len(query) >= 2 && strings.HasPrefix(query, `"`) && strings.HasSuffix(query, `"`) {
		inner := strings.TrimSpace(query[1 : len(query)-1])
		if inner == "" {
			return `""`
		}
		return `"` + strings.ReplaceAll(inner, `"`, `""`) + `"`
	}
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}
	var escaped []string
	for _, w := range words {
		w = strings.ReplaceAll(w, `"`, `""`)
		if w != "" {
			escaped = append(escaped, `"`+w+`"`)
		}
	}
	if len(escaped) == 0 {
		return `""`
	}
	if len(escaped) == 1 {
		return escaped[0]
	}
	return strings.Join(escaped, " AND ")
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

func normalizeKnowledgeGraphDuplicateID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return ""
	}
	id = strings.ReplaceAll(id, "_", "")
	id = strings.ReplaceAll(id, "-", "")
	return id
}

func countKnowledgeGraphLabelDuplicateGroups(querier interface {
	QueryRow(query string, args ...any) *sql.Row
}) (int, error) {
	var count int
	err := querier.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT 1
			FROM kg_nodes
			WHERE label != ''
			GROUP BY LOWER(TRIM(label))
			HAVING COUNT(*) > 1
		)
	`).Scan(&count)
	return count, err
}

func countKnowledgeGraphIDDuplicateGroups(querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, logger *slog.Logger) (int, error) {
	nodes, err := knowledgeGraphLoadNodesForIDDuplicateCheck(querier, logger)
	if err != nil {
		return 0, err
	}
	grouped := knowledgeGraphGroupNodesByNormalizedID(nodes)
	qualified := knowledgeGraphFilterQualifiedIDDuplicateGroups(grouped)
	return len(qualified), nil
}

func countKnowledgeGraphIsolatedNodes(querier interface {
	QueryRow(query string, args ...any) *sql.Row
}) (int, error) {
	var count int
	err := querier.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes n
		WHERE NOT EXISTS (SELECT 1 FROM kg_edges e WHERE e.source = n.id OR e.target = n.id)
	`).Scan(&count)
	return count, err
}

func knowledgeGraphNodesQualifyAsIDDuplicates(a, b Node) bool {
	srcA := strings.TrimSpace(a.Properties["source"])
	srcB := strings.TrimSpace(b.Properties["source"])
	if srcA != "" && srcA == srcB {
		return true
	}

	la := normalizeKnowledgeGraphDuplicateLabel(a.Label)
	lb := normalizeKnowledgeGraphDuplicateLabel(b.Label)
	if la == "" && lb == "" {
		return true
	}
	if la == lb {
		return true
	}
	if la != "" && lb != "" && (strings.HasPrefix(la, lb) || strings.HasPrefix(lb, la)) {
		return true
	}
	return false
}

func knowledgeGraphIDDuplicateGroupQualifies(nodes []Node) bool {
	if len(nodes) < 2 {
		return false
	}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if knowledgeGraphNodesQualifyAsIDDuplicates(nodes[i], nodes[j]) {
				return true
			}
		}
	}
	return false
}

func knowledgeGraphGroupNodesByNormalizedID(nodes []Node) map[string][]Node {
	grouped := make(map[string][]Node)
	for _, node := range nodes {
		normalized := normalizeKnowledgeGraphDuplicateID(node.ID)
		if normalized == "" {
			continue
		}
		grouped[normalized] = append(grouped[normalized], node)
	}
	return grouped
}

func knowledgeGraphFilterQualifiedIDDuplicateGroups(grouped map[string][]Node) map[string][]Node {
	qualified := make(map[string][]Node, len(grouped))
	for key, nodes := range grouped {
		if len(nodes) < 2 || !knowledgeGraphIDDuplicateGroupQualifies(nodes) {
			continue
		}
		qualified[key] = nodes
	}
	return qualified
}

const knowledgeGraphIDDuplicateCandidateSQL = `
	WITH normalized AS (
		SELECT id, label, properties, protected,
			REPLACE(REPLACE(LOWER(TRIM(id)), '_', ''), '-', '') AS norm_id
		FROM kg_nodes
	),
	dup_groups AS (
		SELECT norm_id
		FROM normalized
		WHERE norm_id != ''
		GROUP BY norm_id
		HAVING COUNT(*) > 1
	)
	SELECT n.id, n.label, n.properties, n.protected
	FROM normalized n
	INNER JOIN dup_groups g ON g.norm_id = n.norm_id
	ORDER BY n.norm_id, n.id`

func knowledgeGraphLoadNodesForIDDuplicateCheck(querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, logger *slog.Logger) ([]Node, error) {
	rows, err := querier.Query(knowledgeGraphIDDuplicateCandidateSQL)
	if err != nil {
		return nil, fmt.Errorf("query knowledge graph id duplicate candidates: %w", err)
	}
	defer rows.Close()

	nodes := make([]Node, 0)
	for rows.Next() {
		var node Node
		var propsJSON string
		var protected int
		if err := rows.Scan(&node.ID, &node.Label, &propsJSON, &protected); err != nil {
			return nil, fmt.Errorf("scan knowledge graph node for id duplicates: %w", err)
		}
		node.Properties = decodeKnowledgeGraphNodeProperties(logger, "IDDuplicateCheck", node.ID, propsJSON, protected)
		node.Protected = protected != 0
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph nodes for id duplicates: %w", err)
	}
	return nodes, nil
}

func knowledgeGraphIDDuplicateSummary(logger *slog.Logger, tx *sql.Tx, sampleLimit int) (groups int, nodes int, candidates []KnowledgeGraphDuplicateCandidate, err error) {
	nodesLoaded, err := knowledgeGraphLoadNodesForIDDuplicateCheck(tx, logger)
	if err != nil {
		return 0, 0, nil, err
	}

	grouped := knowledgeGraphFilterQualifiedIDDuplicateGroups(knowledgeGraphGroupNodesByNormalizedID(nodesLoaded))
	allCandidates := buildKnowledgeGraphDuplicateCandidates(grouped)
	for _, candidate := range allCandidates {
		nodes += candidate.Count
	}
	groups = len(allCandidates)
	if sampleLimit > 0 && len(allCandidates) > sampleLimit {
		allCandidates = allCandidates[:sampleLimit]
	}
	return groups, nodes, allCandidates, nil
}
