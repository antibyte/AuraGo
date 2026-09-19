package agent

import openai "github.com/sashabaranov/go-openai"

func toolDetailBudgetCheck(budget *RequestBudget, req openai.ChatCompletionRequest) func(string) bool {
	if budget == nil {
		return nil // Standalone tool bridges have no model request to fit.
	}
	messages := append([]openai.ChatCompletionMessage(nil), req.Messages...)
	return func(output string) bool {
		candidate := append(append([]openai.ChatCompletionMessage(nil), messages...), openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleTool, Content: output, ToolCallID: "discovery_detail",
		})
		_, err := budget.validate(candidate, req.Tools, newTokenCountCache(64))
		return err == nil
	}
}
