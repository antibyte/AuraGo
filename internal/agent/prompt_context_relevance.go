package agent

import (
	"strings"
	"unicode"

	"aurago/internal/memory"
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
	for _, phrase := range []string{"was neues", "gibts", "gibt es", "what's new", "whats new", "anything new"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		switch token {
		case "status", "offen", "todo", "aufgabe", "erinnerung", "followup", "pending", "open", "remind":
			return true
		}
	}
	return false
}

func selectRelevantRecentMemoryLines(query string, candidates []string, limit int) []string {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	queryTerms := memory.TopicTermSet(query)
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
		candidateTerms := memory.TopicTermSet(candidate)
		matched := false
		for term := range queryTerms {
			if _, ok := candidateTerms[term]; ok {
				matched = true
				break
			}
		}
		if !matched {
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
	return memory.TopicTermSet(text)
}
