package llm

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// ChatClient is satisfied by the SDK client adapter and by FailoverManager.
// Components use this interface so the failover layer can be injected without
// depending on the SDK's concrete stream type.
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (CompletionStream, error)
}

// CompletionStream is the part of the SDK stream contract used by AuraGo.
// Keeping it small lets failover observe completion without exposing SDK
// implementation details to callers.
type CompletionStream interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

// WrapChatCompletionStream adapts a go-openai stream to CompletionStream.
func WrapChatCompletionStream(stream *openai.ChatCompletionStream) CompletionStream {
	if stream == nil {
		return nil
	}
	return stream
}

type openAIClientAdapter struct {
	client *openai.Client
}

// WrapOpenAIClient adapts the SDK client to ChatClient's stream interface.
func WrapOpenAIClient(client *openai.Client) ChatClient {
	if client == nil {
		return nil
	}
	return openAIClientAdapter{client: client}
}

func (c openAIClientAdapter) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return c.client.CreateChatCompletion(ctx, request)
}

func (c openAIClientAdapter) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (CompletionStream, error) {
	stream, err := c.client.CreateChatCompletionStream(ctx, request)
	return WrapChatCompletionStream(stream), err
}

// RequestRouteProvider is an optional capability implemented by clients that
// can route a request to more than one provider/model. Keeping it separate from
// ChatClient preserves compatibility with existing clients and test doubles.
type RequestRouteProvider interface {
	CandidateRoutes(request openai.ChatCompletionRequest) []ModelRoute
}
