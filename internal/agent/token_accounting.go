package agent

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

// applyTokenEstimationFallback fills missing token accounting when the provider does not
// return a usable total token count. It avoids double counting when a provider returns
// partial usage (e.g. prompt tokens set, but total tokens missing/zero).
func applyTokenEstimationFallback(promptTokens, completionTokens, totalTokens int, tokenSource string, req openai.ChatCompletionRequest, completionText string) (int, int, int, string, bool) {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	if totalTokens < 0 {
		totalTokens = 0
	}
	providerTotal := totalTokens > 0
	usedFallback := false
	if providerTotal {
		// Provider gave a total but may be missing one side. Derive the missing
		// component so budget/input/output tracking stays accurate.
		if promptTokens == 0 && completionTokens > 0 {
			promptTokens = totalTokens - completionTokens
			if promptTokens < 0 {
				promptTokens = 0
			}
			return promptTokens, completionTokens, totalTokens, tokenSource, false
		}
		if completionTokens == 0 && promptTokens > 0 {
			completionTokens = totalTokens - promptTokens
			if completionTokens < 0 {
				completionTokens = 0
			}
			return promptTokens, completionTokens, totalTokens, tokenSource, false
		}
		if promptTokens == 0 && completionTokens == 0 {
			// Only total provided: estimate prompt from request, rest is completion
			promptTokens = estimatePromptTokensForRequest(req)
			if promptTokens > totalTokens {
				promptTokens = totalTokens
			}
			usedFallback = true
			completionTokens = totalTokens - promptTokens
			if completionTokens < 0 {
				completionTokens = 0
			}
			return promptTokens, completionTokens, totalTokens, "fallback_estimate", usedFallback
		}
		return promptTokens, completionTokens, totalTokens, tokenSource, false
	}

	if promptTokens == 0 {
		promptTokens = estimatePromptTokensForRequest(req)
		usedFallback = promptTokens > 0
	}
	if completionTokens == 0 && strings.TrimSpace(completionText) != "" {
		completionTokens = estimateTokensForModel(completionText, req.Model)
		usedFallback = usedFallback || completionTokens > 0
	}

	if usedFallback {
		tokenSource = "fallback_estimate"
	} else if tokenSource == "" {
		tokenSource = "provider_usage"
	}
	totalTokens = promptTokens + completionTokens
	return promptTokens, completionTokens, totalTokens, tokenSource, usedFallback
}

func estimatePromptTokensForRequest(req openai.ChatCompletionRequest) int {
	total := 0
	for _, message := range req.Messages {
		total += estimateTokensForModel(messageTextWithReasoningForAccounting(message), req.Model)
	}
	return total
}

// normalizeResponseUsage returns provider usage completed with estimates for
// missing components. It only estimates or accounts a response when the
// provider reported usage or the response contains observable output.
func normalizeResponseUsage(req openai.ChatCompletionRequest, resp openai.ChatCompletionResponse) (openai.ChatCompletionResponse, int, int, int, string, bool, bool) {
	usage := resp.Usage
	completionText := ""
	outputEvidence := false
	for _, choice := range resp.Choices {
		message := choice.Message
		if message.Content != "" || message.ReasoningContent != "" || len(message.ToolCalls) > 0 || message.FunctionCall != nil {
			outputEvidence = true
		}
		completionText += message.Content + message.ReasoningContent
		for _, call := range message.ToolCalls {
			completionText += call.Function.Name + call.Function.Arguments
		}
		if message.FunctionCall != nil {
			completionText += message.FunctionCall.Name + message.FunctionCall.Arguments
		}
	}
	hasProviderUsage := usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0
	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
		hasProviderUsage = true
	}
	if !hasProviderUsage && !outputEvidence {
		return resp, 0, 0, 0, "", false, false
	}
	if resp.Model != "" {
		req.Model = resp.Model
	}
	tokenSource := "provider_usage"
	if !hasProviderUsage {
		tokenSource = "fallback_estimate"
	}
	promptTokens, completionTokens, totalTokens, tokenSource, estimated := applyTokenEstimationFallback(
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, tokenSource, req, completionText,
	)
	usage.PromptTokens = promptTokens
	usage.CompletionTokens = completionTokens
	usage.TotalTokens = totalTokens
	resp.Usage = usage
	return resp, promptTokens, completionTokens, totalTokens, tokenSource, estimated, true
}
