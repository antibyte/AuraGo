package kgreasoner

import (
	"sort"
	"strings"
)

type EdgeFact struct {
	Source     string            `json:"source"`
	Relation   string            `json:"relation"`
	Target     string            `json:"target"`
	Properties map[string]string `json:"properties,omitempty"`
}

type RuleSet struct {
	TransitiveRelations map[string]bool   `json:"transitive_relations,omitempty"`
	InverseRelations    map[string]string `json:"inverse_relations,omitempty"`
}

type InferredFact struct {
	Source     string   `json:"source"`
	Relation   string   `json:"relation"`
	Target     string   `json:"target"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason"`
	Evidence   []string `json:"evidence"`
}

func DefaultRules() RuleSet {
	return RuleSet{
		TransitiveRelations: map[string]bool{
			"connected_to": true,
			"depends_on":   true,
			"part_of":      true,
		},
		InverseRelations: map[string]string{
			"depends_on": "dependency_of",
			"hosts":      "hosted_on",
			"located_in": "contains",
			"manages":    "managed_by",
			"owns":       "owned_by",
			"part_of":    "has_part",
			"uses":       "used_by",
		},
	}
}

func Infer(edges []EdgeFact, rules RuleSet, limit int) []InferredFact {
	if limit <= 0 {
		limit = 50
	}
	if rules.TransitiveRelations == nil && rules.InverseRelations == nil {
		rules = DefaultRules()
	}

	normalized := make([]EdgeFact, 0, len(edges))
	existing := make(map[string]struct{}, len(edges))
	outgoingByRelationSource := make(map[string][]EdgeFact)
	for _, edge := range edges {
		edge = normalizeEdge(edge)
		if edge.Source == "" || edge.Target == "" || edge.Relation == "" || edge.Source == edge.Target {
			continue
		}
		normalized = append(normalized, edge)
		existing[edgeKey(edge.Source, edge.Relation, edge.Target)] = struct{}{}
		outgoingByRelationSource[edge.Relation+"\x00"+edge.Source] = append(outgoingByRelationSource[edge.Relation+"\x00"+edge.Source], edge)
	}

	candidates := make(map[string]InferredFact)
	addCandidate := func(candidate InferredFact) {
		candidate.Source = strings.TrimSpace(candidate.Source)
		candidate.Relation = normalizeRelation(candidate.Relation)
		candidate.Target = strings.TrimSpace(candidate.Target)
		if candidate.Source == "" || candidate.Target == "" || candidate.Relation == "" || candidate.Source == candidate.Target {
			return
		}
		key := edgeKey(candidate.Source, candidate.Relation, candidate.Target)
		if _, ok := existing[key]; ok {
			return
		}
		if current, ok := candidates[key]; ok && current.Confidence >= candidate.Confidence {
			return
		}
		candidates[key] = candidate
	}

	for _, edge := range normalized {
		if inverse := normalizeRelation(rules.InverseRelations[edge.Relation]); inverse != "" {
			addCandidate(InferredFact{
				Source:     edge.Target,
				Relation:   inverse,
				Target:     edge.Source,
				Confidence: 0.75,
				Reason:     "inverse_relation",
				Evidence:   []string{edgeKey(edge.Source, edge.Relation, edge.Target)},
			})
		}
		if !rules.TransitiveRelations[edge.Relation] {
			continue
		}
		for _, next := range outgoingByRelationSource[edge.Relation+"\x00"+edge.Target] {
			addCandidate(InferredFact{
				Source:     edge.Source,
				Relation:   edge.Relation,
				Target:     next.Target,
				Confidence: 0.70,
				Reason:     "transitive_relation",
				Evidence: []string{
					edgeKey(edge.Source, edge.Relation, edge.Target),
					edgeKey(next.Source, next.Relation, next.Target),
				},
			})
		}
	}

	results := make([]InferredFact, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, candidate)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence > results[j].Confidence
		}
		return edgeKey(results[i].Source, results[i].Relation, results[i].Target) < edgeKey(results[j].Source, results[j].Relation, results[j].Target)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func normalizeEdge(edge EdgeFact) EdgeFact {
	edge.Source = strings.TrimSpace(edge.Source)
	edge.Target = strings.TrimSpace(edge.Target)
	edge.Relation = normalizeRelation(edge.Relation)
	return edge
}

func normalizeRelation(relation string) string {
	return strings.ToLower(strings.TrimSpace(relation))
}

func edgeKey(source, relation, target string) string {
	return source + "|" + relation + "|" + target
}
