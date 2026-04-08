package agent

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestApplyTokenEstimationFallback_PartialProviderUsageDoesNotDoubleCount(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "any",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "System prompt."},
			{Role: openai.ChatMessageRoleUser, Content: "User message."},
		},
	}

	completionText := "Assistant reply."
	completionEstimate := estimateTokensForModel(completionText, req.Model)

	providerPrompt := 123
	promptTokens, completionTokens, totalTokens, tokenSource, usedFallback := applyTokenEstimationFallback(
		providerPrompt,
		0,
		0,
		"provider_usage",
		req,
		completionText,
	)

	if !usedFallback {
		t.Fatalf("expected usedFallback=true")
	}
	if tokenSource != "fallback_estimate" {
		t.Fatalf("expected tokenSource=fallback_estimate, got %q", tokenSource)
	}
	if promptTokens != providerPrompt {
		t.Fatalf("expected promptTokens=%d, got %d", providerPrompt, promptTokens)
	}
	if completionTokens != completionEstimate {
		t.Fatalf("expected completionTokens=%d, got %d", completionEstimate, completionTokens)
	}
	if totalTokens != providerPrompt+completionEstimate {
		t.Fatalf("expected totalTokens=%d, got %d", providerPrompt+completionEstimate, totalTokens)
	}
}

func TestApplyTokenEstimationFallback_FullyMissingUsageEstimatesAll(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "any",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Hello world."},
		},
	}

	completionText := "Hi!"
	expectedPrompt := estimateTokensForModel(messageText(req.Messages[0]), req.Model)
	expectedCompletion := estimateTokensForModel(completionText, req.Model)

	promptTokens, completionTokens, totalTokens, tokenSource, usedFallback := applyTokenEstimationFallback(
		0,
		0,
		0,
		"provider_usage",
		req,
		completionText,
	)

	if !usedFallback {
		t.Fatalf("expected usedFallback=true")
	}
	if tokenSource != "fallback_estimate" {
		t.Fatalf("expected tokenSource=fallback_estimate, got %q", tokenSource)
	}
	if promptTokens != expectedPrompt {
		t.Fatalf("expected promptTokens=%d, got %d", expectedPrompt, promptTokens)
	}
	if completionTokens != expectedCompletion {
		t.Fatalf("expected completionTokens=%d, got %d", expectedCompletion, completionTokens)
	}
	if totalTokens != expectedPrompt+expectedCompletion {
		t.Fatalf("expected totalTokens=%d, got %d", expectedPrompt+expectedCompletion, totalTokens)
	}
}

func TestApplyTokenEstimationFallback_NoFallbackWhenTotalProvided(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "any",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Hello."},
		},
	}

	promptTokens, completionTokens, totalTokens, tokenSource, usedFallback := applyTokenEstimationFallback(
		10,
		20,
		30,
		"provider_usage",
		req,
		"ignored",
	)

	if usedFallback {
		t.Fatalf("expected usedFallback=false")
	}
	if tokenSource != "provider_usage" {
		t.Fatalf("expected tokenSource=provider_usage, got %q", tokenSource)
	}
	if promptTokens != 10 || completionTokens != 20 || totalTokens != 30 {
		t.Fatalf("unexpected tokens: prompt=%d completion=%d total=%d", promptTokens, completionTokens, totalTokens)
	}
}

