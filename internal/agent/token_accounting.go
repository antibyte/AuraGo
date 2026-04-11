package agent

import "github.com/sashabaranov/go-openai"

// applyTokenEstimationFallback fills missing token accounting when the provider does not
// return a usable total token count. It avoids double counting when a provider returns
// partial usage (e.g. prompt tokens set, but total tokens missing/zero).
func applyTokenEstimationFallback(promptTokens, completionTokens, totalTokens int, tokenSource string, req openai.ChatCompletionRequest, completionText string) (int, int, int, string, bool) {
	if totalTokens != 0 {
		return promptTokens, completionTokens, totalTokens, tokenSource, false
	}

	estimatedPromptTokens := 0
	for _, m := range req.Messages {
		estimatedPromptTokens += estimateTokensForModel(messageText(m), req.Model)
	}

	if promptTokens == 0 {
		promptTokens = estimatedPromptTokens
	}
	if completionTokens == 0 {
		completionTokens = estimateTokensForModel(completionText, req.Model)
	}

	totalTokens = promptTokens + completionTokens
	return promptTokens, completionTokens, totalTokens, "fallback_estimate", true
}
