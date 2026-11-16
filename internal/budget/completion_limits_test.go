package budget

import (
	"testing"

	"aurago/internal/llm"
	"github.com/sashabaranov/go-openai"
)

func TestCompleteChatRejectsInvalidLimitsBeforeRewriting(t *testing.T) {
	for _, request := range []openai.ChatCompletionRequest{
		{MaxTokens: -1}, {MaxCompletionTokens: -1}, {MaxTokens: 16, MaxCompletionTokens: 32},
	} {
		client := &auxiliaryBudgetClient{}
		_, err := CompleteChat(t.Context(), client, request, llm.ModelRoute{Model: "fixture"}, "test", 128)
		if llm.ClassifyError(err) != llm.ErrCategoryNonRetryableConfig || client.calls != 0 {
			t.Fatalf("invalid limits were lost during budgeting: calls=%d err=%v", client.calls, err)
		}
	}
}
