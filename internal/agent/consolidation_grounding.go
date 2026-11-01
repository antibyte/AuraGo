package agent

import (
	"strings"

	"aurago/internal/memory"
)

func validConsolidationFactResponse(facts []helperConsolidationFact) bool {
	if facts == nil || len(facts) > 10 || countValidConsolidationFacts(facts) != len(facts) {
		return false
	}
	for _, fact := range facts {
		if strings.TrimSpace(fact.Quote) == "" {
			return false
		}
	}
	return true
}

// The source is the archived human message, never a model-supplied role or quote.
func consolidationFactGrounded(source memory.ArchivedMessage, fact helperConsolidationFact) bool {
	if !strings.EqualFold(strings.TrimSpace(source.Role), "user") || directiveShaped(fact.Concept, fact.Content) ||
		!shouldStoreExtractedMemory(fact.Concept+"\n"+fact.Content, "") {
		return false
	}
	quote, ok := newHumanGrounding(source.Content).quote(fact.Quote)
	return ok && memory.DerivedFrom(fact.Content, quote)
}
