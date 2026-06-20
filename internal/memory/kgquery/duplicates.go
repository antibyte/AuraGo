package kgquery

import (
	"sort"
	"strings"
)

// DuplicateNode is a lightweight node snapshot for duplicate analysis.
type DuplicateNode struct {
	ID         string
	Label      string
	Properties map[string]string
}

// DuplicateCandidate summarizes a duplicate label or normalized ID group.
type DuplicateCandidate struct {
	Label           string
	NormalizedLabel string
	Count           int
	IDs             []string
}

// NormalizeDuplicateLabel lowercases and collapses whitespace for duplicate grouping.
func NormalizeDuplicateLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return ""
	}
	return strings.Join(strings.Fields(label), " ")
}

// NormalizeDuplicateID strips separators and lowercases IDs for duplicate grouping.
func NormalizeDuplicateID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return ""
	}
	id = strings.ReplaceAll(id, "_", "")
	id = strings.ReplaceAll(id, "-", "")
	return id
}

// NodesQualifyAsIDDuplicates reports whether two nodes are likely the same entity.
func NodesQualifyAsIDDuplicates(a, b DuplicateNode) bool {
	srcA := strings.TrimSpace(a.Properties["source"])
	srcB := strings.TrimSpace(b.Properties["source"])
	if srcA != "" && srcA == srcB {
		return true
	}

	la := NormalizeDuplicateLabel(a.Label)
	lb := NormalizeDuplicateLabel(b.Label)
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

// IDDuplicateGroupQualifies reports whether a normalized ID group should be surfaced.
func IDDuplicateGroupQualifies(nodes []DuplicateNode) bool {
	if len(nodes) < 2 {
		return false
	}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if NodesQualifyAsIDDuplicates(nodes[i], nodes[j]) {
				return true
			}
		}
	}
	return false
}

// GroupNodesByNormalizedID groups nodes by normalized ID.
func GroupNodesByNormalizedID(nodes []DuplicateNode) map[string][]DuplicateNode {
	grouped := make(map[string][]DuplicateNode)
	for _, node := range nodes {
		normalized := NormalizeDuplicateID(node.ID)
		if normalized == "" {
			continue
		}
		grouped[normalized] = append(grouped[normalized], node)
	}
	return grouped
}

// FilterQualifiedIDDuplicateGroups keeps only groups that pass qualification rules.
func FilterQualifiedIDDuplicateGroups(grouped map[string][]DuplicateNode) map[string][]DuplicateNode {
	qualified := make(map[string][]DuplicateNode, len(grouped))
	for key, nodes := range grouped {
		if len(nodes) < 2 || !IDDuplicateGroupQualifies(nodes) {
			continue
		}
		qualified[key] = nodes
	}
	return qualified
}

// BuildDuplicateCandidates converts grouped nodes into sorted duplicate candidates.
func BuildDuplicateCandidates(groups map[string][]DuplicateNode) []DuplicateCandidate {
	candidates := make([]DuplicateCandidate, 0, len(groups))
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
		candidates = append(candidates, DuplicateCandidate{
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