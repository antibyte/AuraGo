package agent

import "github.com/sashabaranov/go-openai"

type preparedHistoryGroup struct {
	start, end int
	required   bool
	native     bool
}

// Prepared callers supply the bounded phase view, not an entire chat archive.
// Keep its user data packets and the two newest complete native rounds even
// when a correction follows them. Older rounds are removed atomically.
func preparedHistoryGroups(messages []openai.ChatCompletionMessage) []preparedHistoryGroup {
	var groups []preparedHistoryGroup
	lastReasoning := -1
	for i := 0; i < len(messages); {
		m := messages[i]
		group := preparedHistoryGroup{start: i, end: i + 1}
		group.required = m.Role == "user" && !isTextModeToolResult(m) || m.Role == "system" && !isSheddableHistorySystem(m)
		if m.ReasoningContent != "" {
			lastReasoning = len(groups)
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			group.native = true
			for group.end < len(messages) && messages[group.end].Role == "tool" {
				group.end++
			}
		} else if m.Role == "tool" {
			group.required = true
		} // Let protocol sanitization judge orphans.
		groups = append(groups, group)
		i = group.end
	}
	if lastReasoning >= 0 {
		groups[lastReasoning].required = true
	}
	remaining := 2
	for i := len(groups) - 1; i >= 0 && remaining > 0; i-- {
		if groups[i].native {
			groups[i].required = true
			remaining--
		}
	}
	return groups
}

func minimumPreparedMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	var kept []openai.ChatCompletionMessage
	for _, group := range preparedHistoryGroups(messages) {
		if group.required {
			kept = append(kept, messages[group.start:group.end]...)
		}
	}
	return kept
}

func trimPreparedHistory(budget *RequestBudget, messages []openai.ChatCompletionMessage, currentUser, system string, tools []openai.Tool, cache *tokenCountCache) ([]openai.ChatCompletionMessage, error) {
	working := append([]openai.ChatCompletionMessage(nil), messages...)
	fits := func() bool {
		_, err := budget.validate(working, tools, cache)
		return err == nil && budget.historyWorkingSetFits(working, currentUserMessageIndex(working, currentUser), system, tools, cache)
	}
	if fits() {
		return working, nil
	}
	var dropped []openai.ChatCompletionMessage
	for !fits() {
		removed := false
		for _, group := range preparedHistoryGroups(working) {
			if group.required {
				continue
			}
			dropped = append(dropped, working[group.start:group.end]...)
			working = append(append([]openai.ChatCompletionMessage(nil), working[:group.start]...), working[group.end:]...)
			removed = true
			break
		}
		if !removed {
			if _, err := budget.validate(working, tools, cache); err != nil {
				return nil, err
			}
			return nil, promptBudgetExceededForRoutes(budget, budget.maxMessagesTokens(working, cache))
		}
	}
	return appendRecapWithinWorkingSet(budget, working, currentUser, system, tools, dropped, cache), nil
}
