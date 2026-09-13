package agent

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

// Buffer text privately: incomplete source must never reach a file or tool.
func minimalLoopStreamText(ctx context.Context, client llm.ChatClient, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	defer stream.Close()
	var text strings.Builder
	var finish openai.FinishReason
	var usage openai.Usage
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
			if choice.FinishReason != "" {
				finish = choice.FinishReason
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if finish == "" {
		return openai.ChatCompletionResponse{}, fmt.Errorf("text stream ended without a finish reason")
	}
	return openai.ChatCompletionResponse{Usage: usage, Choices: []openai.ChatCompletionChoice{{
		FinishReason: finish,
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: text.String()},
	}}}, nil
}
