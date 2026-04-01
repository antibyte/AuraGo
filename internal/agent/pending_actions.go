package agent

import (
	"sort"
	"strings"

	"aurago/internal/memory"
)

func resolveCompletedPendingActions(stm *memory.SQLiteMemory, userMsg string, assistantResp string, pending []memory.EpisodicMemory) []int64 {
	if stm == nil || len(pending) == 0 {
		return nil
	}
	responseTerms := pendingActionTerms(assistantResp)
	requestTerms := pendingActionTerms(userMsg)
	if len(responseTerms) == 0 {
		return nil
	}
	if responseLooksDeferred(assistantResp) {
		return nil
	}

	resolved := make([]int64, 0, len(pending))
	for _, item := range pending {
		if item.ID <= 0 {
			continue
		}
		triggerTerms := pendingActionTerms(item.TriggerQuery + " " + item.Title + " " + item.Summary)
		if len(triggerTerms) == 0 {
			continue
		}
		requestOverlap := overlapCount(requestTerms, triggerTerms)
		responseOverlap := overlapCount(responseTerms, triggerTerms)
		if responseOverlap >= 2 || (requestOverlap >= 1 && responseOverlap >= 1 && len(responseTerms) >= 6) {
			if err := stm.ResolvePendingEpisodicAction(item.ID); err == nil {
				resolved = append(resolved, item.ID)
			}
		}
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i] < resolved[j] })
	return resolved
}

func responseLooksDeferred(text string) bool {
	value := strings.ToLower(strings.TrimSpace(text))
	if value == "" {
		return true
	}
	markers := []string{
		"let me know if you want", "if you want, i can", "later", "später", "can do next", "next step",
		"need more info", "brauche mehr information", "cannot complete", "kann ich nicht abschließen",
		"not enough information", "ich kann später", "follow up later", "remind me",
	}
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func pendingActionTerms(text string) map[string]struct{} {
	value := normalizePredictiveQuery(text)
	if value == "" {
		return nil
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "with": {}, "from": {}, "that": {}, "this": {}, "have": {}, "will": {}, "your": {},
		"docker": {}, "setup": {}, "help": {}, "user": {}, "agent": {}, "next": {}, "step": {},
		"der": {}, "die": {}, "das": {}, "und": {}, "mit": {}, "für": {}, "fur": {}, "dies": {}, "diese": {},
	}
	terms := make(map[string]struct{})
	for _, token := range strings.Fields(value) {
		if len(token) <= 3 {
			continue
		}
		if _, ok := stop[token]; ok {
			continue
		}
		terms[token] = struct{}{}
	}
	if len(terms) == 0 {
		return nil
	}
	return terms
}

func overlapCount(left, right map[string]struct{}) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	count := 0
	for token := range left {
		if _, ok := right[token]; ok {
			count++
		}
	}
	return count
}
