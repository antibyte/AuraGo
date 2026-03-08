package llm

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// ChatClient is satisfied by *openai.Client and by FailoverManager.
// All components that talk to the LLM use this interface so the failover
// layer can be injected without changing call-sites.
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
}
