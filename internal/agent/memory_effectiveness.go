package agent

import (
	"sort"
	"strings"
)

func assessMemoryEffectiveness(response string, candidates map[string]string) ([]string, []string) {
	responseTerms := memoryEffectivenessTerms(response)
	if len(responseTerms) == 0 || len(candidates) == 0 {
		return nil, nil
	}

	useful := make([]string, 0, len(candidates))
	useless := make([]string, 0, len(candidates))
	for memoryID, text := range candidates {
		if strings.TrimSpace(memoryID) == "" || strings.TrimSpace(text) == "" {
			continue
		}
		keywords := memoryEffectivenessTerms(text)
		if len(keywords) == 0 {
			continue
		}

		matches := 0
		for term := range keywords {
			if _, ok := responseTerms[term]; ok {
				matches++
			}
		}

		if matches >= 2 || (matches == 1 && len(keywords) <= 3) {
			useful = append(useful, memoryID)
		} else {
			useless = append(useless, memoryID)
		}
	}

	sort.Strings(useful)
	sort.Strings(useless)
	return useful, useless
}

func memoryEffectivenessTerms(text string) map[string]struct{} {
	normalized := normalizePredictiveQuery(text)
	if normalized == "" {
		return nil
	}
	stopWords := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "that": {}, "this": {}, "into": {}, "about": {}, "please": {},
		"your": {}, "have": {}, "will": {}, "just": {}, "when": {}, "what": {}, "were": {}, "them": {}, "then": {}, "also": {},
		"der": {}, "die": {}, "das": {}, "und": {}, "mit": {}, "den": {}, "dem": {}, "eine": {}, "einen": {}, "bitte": {}, "noch": {},
		"dass": {}, "sind": {}, "wird": {}, "kann": {}, "konnte": {}, "dies": {}, "dieser": {}, "diese": {},
	}

	terms := make(map[string]struct{})
	for _, token := range strings.Fields(normalized) {
		if len(token) <= 3 {
			continue
		}
		if _, ok := stopWords[token]; ok {
			continue
		}
		terms[token] = struct{}{}
		if len(terms) >= 8 {
			break
		}
	}
	if len(terms) == 0 {
		return nil
	}
	return terms
}
