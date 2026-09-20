package agent

import openai "github.com/sashabaranov/go-openai"

// Preserve normal conversations as a unit. Oversized conversations may be
// summarized incrementally, but native calls and all their results stay atomic.
func persistentCompressionGroups(messages []openai.ChatCompletionMessage) []conversationGroup {
	groups := buildConversationGroups(messages)
	rounds := make(map[int]int)
	for _, round := range findCompleteNativeToolRounds(messages) {
		rounds[round.start] = round.end
	}
	var out []conversationGroup
	for _, group := range groups {
		chars := 0
		for _, message := range messages[group.start:group.end] {
			chars += len(persistentSummaryMessageText(message)) + len(message.Role) + len("[]: \n\n")
		}
		if group.end-group.start <= persistentCompressionMaxMessages && chars <= persistentCompressionMaxTranscriptChars {
			out = append(out, group)
			continue
		}
		for i := group.start; i < group.end; {
			end := i + 1
			if len(messages[i].ToolCalls) > 0 {
				var complete bool
				end, complete = rounds[i]
				if !complete || end > group.end {
					// An incomplete round is a barrier, never a deletion candidate.
					return out
				}
			} else if messages[i].Role == openai.ChatMessageRoleTool || isTextModeToolResult(messages[i]) {
				return out
			} else if end < group.end && isTextModeToolResult(messages[end]) {
				end++
			}
			out = append(out, conversationGroup{start: i, end: end})
			i = end
		}
	}
	return out
}
