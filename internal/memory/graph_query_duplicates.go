package memory

import (
	"database/sql"
	"fmt"
	"log/slog"

	"aurago/internal/memory/kgquery"
)

func duplicateNodesForKGQuery(nodes []Node) []kgquery.DuplicateNode {
	out := make([]kgquery.DuplicateNode, len(nodes))
	for i, node := range nodes {
		out[i] = kgquery.DuplicateNode{
			ID:         node.ID,
			Label:      node.Label,
			Properties: node.Properties,
		}
	}
	return out
}

func duplicateCandidatesFromKGQuery(candidates []kgquery.DuplicateCandidate) []KnowledgeGraphDuplicateCandidate {
	out := make([]KnowledgeGraphDuplicateCandidate, len(candidates))
	for i, candidate := range candidates {
		out[i] = KnowledgeGraphDuplicateCandidate{
			Label:           candidate.Label,
			NormalizedLabel: candidate.NormalizedLabel,
			Count:           candidate.Count,
			IDs:             candidate.IDs,
		}
	}
	return out
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
	grouped := kgquery.GroupNodesByNormalizedID(duplicateNodesForKGQuery(nodes))
	qualified := kgquery.FilterQualifiedIDDuplicateGroups(grouped)
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

	grouped := kgquery.FilterQualifiedIDDuplicateGroups(kgquery.GroupNodesByNormalizedID(duplicateNodesForKGQuery(nodesLoaded)))
	allCandidates := duplicateCandidatesFromKGQuery(kgquery.BuildDuplicateCandidates(grouped))
	for _, candidate := range allCandidates {
		nodes += candidate.Count
	}
	groups = len(allCandidates)
	if sampleLimit > 0 && len(allCandidates) > sampleLimit {
		allCandidates = allCandidates[:sampleLimit]
	}
	return groups, nodes, allCandidates, nil
}