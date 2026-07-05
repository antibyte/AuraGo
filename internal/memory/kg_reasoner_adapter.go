package memory

import (
	"fmt"

	"aurago/internal/memory/kgreasoner"
)

func (kg *KnowledgeGraph) SuggestInferredRelations(limit int) ([]kgreasoner.InferredFact, error) {
	if kg == nil || kg.db == nil {
		return nil, fmt.Errorf("knowledge graph not initialized")
	}
	edges, err := kg.GetAllEdges(0)
	if err != nil {
		return nil, fmt.Errorf("load knowledge graph edges for reasoning: %w", err)
	}
	facts := make([]kgreasoner.EdgeFact, 0, len(edges))
	for _, edge := range edges {
		facts = append(facts, kgreasoner.EdgeFact{
			Source:     edge.Source,
			Relation:   edge.Relation,
			Target:     edge.Target,
			Properties: edge.Properties,
		})
	}
	return kgreasoner.Infer(facts, kgreasoner.DefaultRules(), limit), nil
}
