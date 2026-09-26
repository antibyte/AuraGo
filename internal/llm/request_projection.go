package llm

import (
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// MessageForRouteAccounting mirrors the StepFun transport's text projection.
// Return a value copy so private checkpoints and other eligible routes retain
// their reasoning. The transport remains responsible for exact JSON framing.
func MessageForRouteAccounting(route ModelRoute, message openai.ChatCompletionMessage) (openai.ChatCompletionMessage, bool) {
	direct := route.ProviderType == "stepfun" || isStepFunAPIBaseURL(route.BaseURL)
	model := strings.ToLower(route.Model)
	if !direct && !(route.ProviderType == "openrouter" && (strings.HasPrefix(model, "stepfun/") || strings.HasPrefix(model, "stepfun-ai/"))) {
		return message, true
	}
	message.ReasoningContent = ""
	if message.Role == openai.ChatMessageRoleAssistant && message.Content == "" && len(message.MultiContent) == 0 && len(message.ToolCalls) == 0 {
		return message, false
	}
	if message.Role == openai.ChatMessageRoleTool {
		message.Name = ""
	}
	return message, true
}
