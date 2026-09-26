package agent

import (
	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func routeMessageTokens(message openai.ChatCompletionMessage, route llm.ModelRoute, cache *tokenCountCache) int {
	projected, keep := llm.MessageForRouteAccounting(route, message)
	if !keep {
		return 0
	}
	return cache.Count(messageTextWithReasoningForAccounting(projected), route.Model) + 4
}
