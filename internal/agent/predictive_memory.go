package agent

import (
	"sort"
	"strings"
)

func buildPredictiveMemoryQueries(currentQuery, lastTool string, temporalPredictions []string, limit int) []string {
	if limit <= 0 {
		limit = 3
	}

	type candidate struct {
		query  string
		weight int
	}

	candidates := make([]candidate, 0, limit+6)
	for _, prediction := range temporalPredictions {
		candidates = append(candidates, candidate{query: strings.TrimSpace(prediction), weight: 90})
	}

	if trimmed := strings.TrimSpace(currentQuery); trimmed != "" {
		candidates = append(candidates, candidate{query: trimmed, weight: 120})
		for _, hint := range extractPredictiveQueryHints(trimmed, 2) {
			candidates = append(candidates, candidate{query: hint, weight: 105})
		}
	}

	family := inferToolFamilyFromQuery(currentQuery)
	if family == "" {
		family = classifyToolFamily(lastTool)
	}
	for _, hint := range predictiveFamilyHints(family) {
		candidates = append(candidates, candidate{query: hint, weight: 80})
	}

	best := make(map[string]int)
	for _, item := range candidates {
		key := normalizePredictiveQuery(item.query)
		if key == "" {
			continue
		}
		if item.weight > best[key] {
			best[key] = item.weight
		}
	}

	ranked := make([]candidate, 0, len(best))
	for key, weight := range best {
		ranked = append(ranked, candidate{query: key, weight: weight})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].weight == ranked[j].weight {
			return ranked[i].query < ranked[j].query
		}
		return ranked[i].weight > ranked[j].weight
	})

	out := make([]string, 0, min(limit, len(ranked)))
	for _, item := range ranked {
		out = append(out, item.query)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func extractPredictiveQueryHints(query string, limit int) []string {
	if limit <= 0 {
		limit = 2
	}
	normalized := normalizePredictiveQuery(query)
	if normalized == "" {
		return nil
	}

	tokens := strings.Fields(normalized)
	if len(tokens) <= 2 {
		return []string{normalized}
	}

	stopWords := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "that": {}, "this": {}, "into": {}, "about": {}, "please": {},
		"der": {}, "die": {}, "das": {}, "und": {}, "mit": {}, "den": {}, "dem": {}, "eine": {}, "einen": {}, "bitte": {}, "noch": {},
	}
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(token) <= 2 {
			continue
		}
		if _, ok := stopWords[token]; ok {
			continue
		}
		filtered = append(filtered, token)
	}
	if len(filtered) == 0 {
		filtered = tokens
	}

	hints := make([]string, 0, limit)
	if len(filtered) >= 2 {
		hints = append(hints, strings.Join(filtered[:min(3, len(filtered))], " "))
	}
	for _, token := range filtered {
		if len(hints) >= limit {
			break
		}
		hints = append(hints, token)
	}
	return dedupeStrings(hints)
}

func predictiveFamilyHints(family string) []string {
	switch family {
	case "deployment":
		return []string{"deployment", "homepage", "netlify"}
	case "infra":
		return []string{"docker", "server", "infrastructure"}
	case "network":
		return []string{"network", "dns", "connectivity"}
	case "memory":
		return []string{"memory", "journal", "notes"}
	case "files":
		return []string{"files", "workspace", "documents"}
	case "coding":
		return []string{"code", "implementation", "script"}
	case "web":
		return []string{"website", "api", "web"}
	default:
		return nil
	}
}

func normalizePredictiveQuery(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(",", " ", ".", " ", ":", " ", ";", " ", "\n", " ", "\r", " ", "\"", " ", "'", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
