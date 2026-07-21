package agent

import (
	"strings"
	"unicode"
)

// shouldInjectRecentMemoryContext is intentionally limited to explicit status
// queries. Topic-based episode injection is decided against the candidate text
// itself by selectRelevantRecentMemoryLines.
func shouldInjectRecentMemoryContext(msg string) bool {
	return isRecentMemoryStatusQuery(msg)
}

func isRecentMemoryStatusQuery(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if lower == "" {
		return false
	}
	lowSignal := map[string]struct{}{
		"hi": {}, "hello": {}, "hey": {}, "hallo": {}, "moin": {}, "servus": {}, "yo": {}, "ok": {}, "okay": {},
	}
	if _, ok := lowSignal[lower]; ok {
		return false
	}
	statusKeywords := []string{
		"was neues", "gibts", "gibt es", "status", "offen", "todo", "aufgabe", "erinner", "follow",
		"pending", "open", "what's new", "whats new", "anything new", "remind",
	}
	for _, keyword := range statusKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func selectRelevantRecentMemoryLines(query string, candidates []string, limit int) []string {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	queryTerms := meaningfulRecentMemoryTerms(query)
	for _, generic := range []string{"status", "offen", "todo", "aufgabe", "pending", "open", "remind", "prüfe", "check", "gibts", "neues", "anything"} {
		delete(queryTerms, generic)
	}
	if isRecentMemoryStatusQuery(query) && len(queryTerms) == 0 {
		if len(candidates) > limit {
			return append([]string(nil), candidates[:limit]...)
		}
		return append([]string(nil), candidates...)
	}
	if len(queryTerms) == 0 {
		return nil
	}
	var selected []string
	for _, candidate := range candidates {
		candidateTerms := meaningfulRecentMemoryTerms(candidate)
		matches := 0
		for term := range queryTerms {
			if _, ok := candidateTerms[term]; ok {
				matches++
			}
		}
		if matches == 0 || (len(queryTerms) > 2 && matches < 2) {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func meaningfulRecentMemoryTerms(text string) map[string]struct{} {
	stop := map[string]struct{}{
		"aber": {}, "again": {}, "auch": {}, "bitte": {}, "dann": {}, "erneut": {},
		"nochmal": {}, "nochmals": {}, "please": {}, "retry": {}, "the": {}, "this": {},
		"try": {}, "versuch": {}, "versuche": {}, "wieder": {}, "with": {}, "noch": {},
	}
	terms := make(map[string]struct{})
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if _, blocked := stop[term]; blocked {
			continue
		}
		if len([]rune(term)) < 4 && term != "pkw" && term != "api" && term != "ssl" && term != "gpu" && term != "nas" {
			continue
		}
		terms[term] = struct{}{}
	}
	return terms
}
