package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

// ErrIncompleteTextStream marks an EOF without the provider's completion marker.
// It never makes buffered text safe to apply. An isolated caller may retry the
// entire request within its own deadline and retry budget.
var ErrIncompleteTextStream = errors.New("text stream ended without a finish reason")

// Buffer text privately: incomplete source must never reach a file or tool.
func minimalLoopStreamText(ctx context.Context, client llm.ChatClient, req openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, retErr error) {
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	defer stream.Close()
	var text strings.Builder
	var reasoning strings.Builder
	var finish openai.FinishReason
	var usage openai.Usage
	defer func() {
		if retErr != nil && reasoning.Len() > 0 {
			response = openai.ChatCompletionResponse{Usage: usage, Choices: []openai.ChatCompletionChoice{{Message: interruptedReasoningMessage(reasoning.String())}}}
		}
	}()
	chunks, lastChunk := 0, time.Now()
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return openai.ChatCompletionResponse{}, fmt.Errorf("text stream interrupted (%d chunks, %d text bytes, %s since last chunk): %w", chunks, text.Len(), time.Since(lastChunk).Round(time.Second), err)
		}
		chunks++
		lastChunk = time.Now()
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 || len(chunk.Choices) > 1 || len(choice.Delta.ToolCalls) > 0 || choice.Delta.FunctionCall != nil {
				return openai.ChatCompletionResponse{}, fmt.Errorf("unexpected choice or tool call in text stream")
			}
			if text.Len()+len(choice.Delta.Content) > 4*1024*1024 {
				return openai.ChatCompletionResponse{}, fmt.Errorf("text stream exceeds 4 MiB")
			}
			text.WriteString(choice.Delta.Content)
			if reasoning.Len()+len(choice.Delta.ReasoningContent) > 4*1024*1024 {
				return openai.ChatCompletionResponse{}, fmt.Errorf("reasoning stream exceeds 4 MiB")
			}
			reasoning.WriteString(choice.Delta.ReasoningContent)
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if finish == "" {
		return openai.ChatCompletionResponse{}, fmt.Errorf("%w (%d chunks, %d text bytes, %d reasoning bytes); incomplete output was discarded", ErrIncompleteTextStream, chunks, text.Len(), reasoning.Len())
	}
	return openai.ChatCompletionResponse{Usage: usage, Choices: []openai.ChatCompletionChoice{{
		FinishReason: finish,
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: text.String(), ReasoningContent: reasoning.String()},
	}}}, nil
}
