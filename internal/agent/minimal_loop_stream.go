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
func minimalLoopStreamText(ctx context.Context, client llm.ChatClient, req openai.ChatCompletionRequest, idleBudgets ...time.Duration) (response openai.ChatCompletionResponse, retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx, thinkingProgress := withStreamThinkingProgress(ctx, nil)
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	type receivedChunk struct {
		chunk openai.ChatCompletionStreamResponse
		err   error
	}
	received := make(chan receivedChunk, 1)
	receiverDone := make(chan struct{})
	go func() {
		defer close(receiverDone)
		defer close(received)
		defer func() {
			if recover() != nil {
				select {
				case received <- receivedChunk{err: ErrRuntimePanic}:
				case <-ctx.Done():
				}
			}
		}()
		for {
			chunk, err := stream.Recv()
			select {
			case received <- receivedChunk{chunk, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		cancel()
		_ = stream.Close()
		select {
		case <-receiverDone:
		case <-time.After(time.Second):
		}
	}()
	idle := 30 * time.Second
	if len(idleBudgets) > 0 && idleBudgets[0] > 0 {
		idle = idleBudgets[0]
	}
	wait := idle
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) > wait {
		wait = time.Until(deadline)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	progressSeen := false
	markProgress := func() {
		progressSeen = true
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(idle)
	}
	var text strings.Builder
	var reasoning strings.Builder
	var finish openai.FinishReason
	var usage openai.Usage
	var responseModel string
	defer func() {
		if retErr == nil {
			return
		}
		response = openai.ChatCompletionResponse{Model: responseModel, Usage: usage}
		if text.Len() > 0 || reasoning.Len() > 0 || finish != "" {
			response.Choices = []openai.ChatCompletionChoice{{FinishReason: finish, Message: openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleAssistant, Content: text.String(), ReasoningContent: reasoning.String(),
			}}}
		}
	}()
	chunks, lastChunk := 0, time.Now()
	for {
		var next receivedChunk
		select {
		case <-ctx.Done():
			if !progressSeen && errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return response, fmt.Errorf("%w: %w", errStreamFirstTokenTimeout, ctx.Err())
			}
			return response, ctx.Err()
		case <-thinkingProgress.ready:
			markProgress()
			continue
		case <-timer.C:
			if !progressSeen {
				return response, fmt.Errorf("%w: %w", errStreamFirstTokenTimeout, context.DeadlineExceeded)
			}
			return response, fmt.Errorf("%w: %w", errStreamIdleTimeout, context.DeadlineExceeded)
		case item, ok := <-received:
			if !ok {
				next.err = io.EOF
			} else {
				next = item
			}
		}
		chunk, err := next.chunk, next.err
		if err == io.EOF {
			break
		}
		if err != nil {
			return openai.ChatCompletionResponse{}, fmt.Errorf("text stream interrupted (%d chunks, %d text bytes, %d reasoning bytes, %s since last chunk): %w", chunks, text.Len(), reasoning.Len(), time.Since(lastChunk).Round(time.Second), err)
		}
		chunks++
		lastChunk = time.Now()
		if chunk.Model != "" {
			responseModel = chunk.Model
		}
		if chunk.Usage != nil {
			usage.PromptTokens = max(usage.PromptTokens, chunk.Usage.PromptTokens)
			usage.CompletionTokens = max(usage.CompletionTokens, chunk.Usage.CompletionTokens)
			usage.TotalTokens = max(usage.TotalTokens, chunk.Usage.TotalTokens)
			if details := chunk.Usage.PromptTokensDetails; details != nil && (usage.PromptTokensDetails == nil || details.CachedTokens >= usage.PromptTokensDetails.CachedTokens) {
				copy := *details
				usage.PromptTokensDetails = &copy
			}
			if chunk.Usage.CompletionTokensDetails != nil {
				usage.CompletionTokensDetails = chunk.Usage.CompletionTokensDetails
			}
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
			measured := len(choice.Delta.Content) + thinkingProgress.reasoningChars(choice.Delta.ReasoningContent)
			llm.ObserveStreamOutput(ctx, measured)
			if measured > 0 || choice.FinishReason != "" {
				markProgress()
			}
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
	response = openai.ChatCompletionResponse{Model: responseModel, Usage: usage, Choices: []openai.ChatCompletionChoice{{
		FinishReason: finish,
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: text.String(), ReasoningContent: reasoning.String()},
	}}}
	return response, syncCompletionError(ctx, response)
}
