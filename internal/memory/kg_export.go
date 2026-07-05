package memory

import (
	"encoding/json"
	"fmt"
)

type KGJSONLDDocument struct {
	Context  map[string]interface{} `json:"@context"`
	Graph    []interface{}          `json:"@graph"`
	Metadata map[string]interface{} `json:"metadata"`
}

type KGJSONLDEntity struct {
	ID         string            `json:"@id"`
	Type       string            `json:"@type"`
	KGID       string            `json:"kg:id"`
	Name       string            `json:"name"`
	Properties map[string]string `json:"kg:properties,omitempty"`
	Protected  bool              `json:"kg:protected,omitempty"`
}

type KGJSONLDRelationship struct {
	ID                  string            `json:"@id"`
	Type                string            `json:"@type"`
	Source              map[string]string `json:"kg:source"`
	Target              map[string]string `json:"kg:target"`
	Relation            string            `json:"kg:relation"`
	Status              KGClaimStatus     `json:"kg:status"`
	StatusReason        string            `json:"kg:statusReason,omitempty"`
	SupersededByClaimID string            `json:"kg:supersededByClaimId,omitempty"`
	RetractedAt         string            `json:"kg:retractedAt,omitempty"`
	Properties          map[string]string `json:"kg:properties,omitempty"`
	Claims              []KGClaim         `json:"kg:claims,omitempty"`
}

func (kg *KnowledgeGraph) ExportJSONLD(includeInactive bool, limit int) (*KGJSONLDDocument, error) {
	if kg == nil || kg.db == nil {
		return nil, fmt.Errorf("knowledge graph not initialized")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}

	nodes, err := kg.GetAllNodes(limit)
	if err != nil {
		return nil, fmt.Errorf("load nodes for json-ld export: %w", err)
	}
	edges, err := kg.exportJSONLDEdges(includeInactive, limit)
	if err != nil {
		return nil, err
	}

	graph := make([]interface{}, 0, len(nodes)+len(edges))
	for _, node := range nodes {
		graph = append(graph, KGJSONLDEntity{
			ID:         kgNodeJSONLDID(node.ID),
			Type:       "kg:Entity",
			KGID:       node.ID,
			Name:       node.Label,
			Properties: node.Properties,
			Protected:  node.Protected,
		})
	}
	for _, edge := range edges {
		graph = append(graph, edge)
	}

	return &KGJSONLDDocument{
		Context: map[string]interface{}{
			"kg":         "https://aurago.local/kg#",
			"name":       "http://schema.org/name",
			"source":     map[string]string{"@id": "kg:source", "@type": "@id"},
			"target":     map[string]string{"@id": "kg:target", "@type": "@id"},
			"relation":   "kg:relation",
			"status":     "kg:status",
			"properties": "kg:properties",
			"claims":     "kg:claims",
		},
		Graph: graph,
		Metadata: map[string]interface{}{
			"node_count":         len(nodes),
			"relationship_count": len(edges),
			"include_inactive":   includeInactive,
			"limit":              limit,
		},
	}, nil
}

func (kg *KnowledgeGraph) exportJSONLDEdges(includeInactive bool, limit int) ([]KGJSONLDRelationship, error) {
	where := activeKGEdgePredicate("")
	if includeInactive {
		where = "1=1"
	}
	rows, err := kg.db.Query(`
		SELECT source, target, relation, properties, status, status_reason, superseded_by_claim_id, COALESCE(retracted_at, '')
		FROM kg_edges
		WHERE `+where+`
		ORDER BY source ASC, relation ASC, target ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("load edges for json-ld export: %w", err)
	}
	defer rows.Close()

	relationships := make([]KGJSONLDRelationship, 0)
	for rows.Next() {
		var source, target, relation, propsJSON, status, statusReason, supersededByClaimID, retractedAt string
		if err := rows.Scan(&source, &target, &relation, &propsJSON, &status, &statusReason, &supersededByClaimID, &retractedAt); err != nil {
			return nil, fmt.Errorf("scan json-ld export edge: %w", err)
		}
		props := make(map[string]string)
		if propsJSON != "" {
			if err := json.Unmarshal([]byte(propsJSON), &props); err != nil && kg.logger != nil {
				kg.logger.Warn("ExportJSONLD: corrupt edge properties JSON", "source", source, "target", target, "relation", relation, "error", err)
			}
		}
		relationships = append(relationships, KGJSONLDRelationship{
			ID:                  kgEdgeJSONLDID(source, relation, target),
			Type:                "kg:Relationship",
			Source:              map[string]string{"@id": kgNodeJSONLDID(source)},
			Target:              map[string]string{"@id": kgNodeJSONLDID(target)},
			Relation:            relation,
			Status:              KGClaimStatus(status),
			StatusReason:        statusReason,
			SupersededByClaimID: supersededByClaimID,
			RetractedAt:         retractedAt,
			Properties:          props,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate json-ld export edges: %w", err)
	}
	rows.Close()

	for i := range relationships {
		source := relationships[i].Source["@id"]
		target := relationships[i].Target["@id"]
		source = trimKGNodeJSONLDID(source)
		target = trimKGNodeJSONLDID(target)
		claims, err := kg.GetClaimsForEdge(source, target, relationships[i].Relation, includeInactive, 100)
		if err != nil {
			return nil, fmt.Errorf("load claims for json-ld edge %s/%s/%s: %w", source, relationships[i].Relation, target, err)
		}
		relationships[i].Claims = claims
	}
	return relationships, nil
}

func kgNodeJSONLDID(id string) string {
	return "kg:node:" + id
}

func kgEdgeJSONLDID(source, relation, target string) string {
	return "kg:edge:" + source + ":" + relation + ":" + target
}

func trimKGNodeJSONLDID(id string) string {
	const prefix = "kg:node:"
	if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
		return id[len(prefix):]
	}
	return id
}
