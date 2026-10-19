package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/llm"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

// EnhanceImagePrompt uses the main LLM to improve a user's image generation prompt.
// Returns the enhanced prompt or the original on error.
func EnhanceImagePrompt(llmClient llm.ChatClient, model string, userPrompt string) (string, error) {
	return EnhanceImagePromptContext(context.Background(), llmClient, model, userPrompt)
}

func EnhanceImagePromptContext(ctx context.Context, llmClient llm.ChatClient, model, userPrompt string) (string, error) {
	if llmClient == nil {
		return userPrompt, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: "You are an expert image prompt engineer. Your task is to enhance the user's image generation prompt for maximum visual quality and detail. " +
					"Add specific details about lighting, composition, style, mood, and technical aspects. " +
					"Keep it under 200 words. Return ONLY the enhanced prompt text, nothing else. " +
					"Do not add explanations, prefixes, or formatting.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   300,
	}

	resp, err := budget.CompleteChat(ctx, llmClient, req, llm.ModelRoute{Model: model}, "image_prompt", 300, prompts.CountTokensForModelContext)
	if err != nil {
		return userPrompt, fmt.Errorf("prompt enhancement failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return userPrompt, nil
	}

	enhanced := strings.TrimSpace(resp.Choices[0].Message.Content)
	if enhanced == "" {
		return userPrompt, nil
	}
	return enhanced, nil
}
