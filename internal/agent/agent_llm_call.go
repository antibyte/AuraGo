package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"
)

// streamingResponseResult holds the output of handleStreamingResponse.
type streamingResponseResult struct {
	resp             openai.ChatCompletionResponse
	content          string
	promptTokens     int
	completionTokens int
	totalTokens      int
	tokenSource      string
	contextCancelled bool
	err              error
	recoveryContinue bool
}

// handleStreamingResponse executes the streaming LLM call and assembles the response.
func handleStreamingResponse(
	llmCtx context.Context,
	req openai.ChatCompletionRequest,
	client llm.ChatClient,
	emptyRetried bool,
	recoveryPolicy RecoveryPolicy,
	currentLogger *slog.Logger,
	broker FeedbackBroker,
	telemetryScope AgentTelemetryScope,
	cancelResp func(),
	chunkIdleTimeout time.Duration,
) streamingResponseResult {
	streamAcct := streamingAccountingState{}
	contextCancelled := false
	var stm *openai.ChatCompletionStream
	var streamErr error
	var midStreamError error
	if emptyRetried {
		stm, streamErr = llm.ExecuteStreamWithCustomRetry(llmCtx, client, req, currentLogger, broker, recoveryPolicy.emptyRetryIntervals(), recoveryPolicy.emptyRetryBaseDelay())
	} else {
		stm, streamErr = llm.ExecuteStreamWithRetry(llmCtx, client, req, currentLogger, broker)
	}
	if streamErr != nil {
		cancelResp()
		telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
		if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, streamErr, new(int), &req, currentLogger, broker, "Stream", telemetryScope); recovered {
			return streamingResponseResult{recoveryContinue: true}
		} else if recErr != nil {
			return streamingResponseResult{err: recErr}
		}
		return streamingResponseResult{err: streamErr}
	}

	var assembledResponse strings.Builder
	var lastFinishReason string
	tcAssembler := NewStreamToolCallAssembler()

	const doneTagStr = "<done/>"
	const minimaxToolCallPrefix = "minimax:tool_call"
	const xmlToolCallPrefix = "<tool_call"      // matches <tool_call> and <tool_call\n> variants
	const actionTagPrefix = "<action>"          // bare <action>toolname</action> emitted by some models
	const toolResponsePrefix = "<tool_response" // model hallucinating a tool response XML block
	// holdLen must cover the longest tag prefix so that it is never split across
	// the send/hold boundary.  With holdLen == P the entire P-byte prefix stays
	// in the hold buffer until the next chunk arrives, guaranteeing detection.
	const doneTagHoldLen = len(minimaxToolCallPrefix) // 17 bytes (≥ all other prefixes)
	const doneTagStreamBufMaxLen = 8192               // max buffer to prevent unbounded growth
	doneTagStreamBuf := ""
	xmlToolCallSuppressed := false // once true, suppress all remaining stream chunks
	type recvResult struct {
		chunk openai.ChatCompletionStreamResponse
		err   error
	}

	recvCh := make(chan recvResult, 1)
	recvEg, recvCtx := errgroup.WithContext(llmCtx)
	recvEg.Go(func() error {
		defer close(recvCh)
		for {
			chunk, rErr := stm.Recv()
			select {
			case recvCh <- recvResult{chunk: chunk, err: rErr}:
			case <-recvCtx.Done():
				return nil
			}
			if rErr != nil {
				return nil
			}
		}
	})

	idleTimeout := chunkIdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second
	}
	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	for {
		var rr recvResult
		var ok bool
		select {
		case <-timer.C:
			currentLogger.Warn("[Stream] No chunks received within idle timeout; aborting stream", "timeout", idleTimeout.String())
			cancelResp()
			contextCancelled = true
			_ = recvEg.Wait()
			ok = false
		case rr, ok = <-recvCh:
			if !ok {
				break
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idleTimeout)
		}
		if !ok {
			break
		}

		chunk, rErr := rr.chunk, rr.err
		if rErr != nil {
			if rErr.Error() != "EOF" {
				currentLogger.Error("Stream error", "error", rErr)
				midStreamError = fmt.Errorf("stream error before done: %w", rErr)
			}
			if llmCtx.Err() == context.Canceled {
				contextCancelled = true
			}
			break
		}
		if chunk.Usage != nil {
			streamAcct.recordProviderUsage(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens)
		}
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].FinishReason != "" {
				lastFinishReason = string(chunk.Choices[0].FinishReason)
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				assembledResponse.WriteString(delta.Content)
				trimmed := strings.TrimLeft(delta.Content, " \t\r\n")
				suppressToolCallJSON := false
				if len(trimmed) > 0 && trimmed[0] == '{' {
					highSpecKeys := []string{`"tool_call"`, `"tool_name"`, `"tool_call_path"`}
					ambiguousKeys := []string{`"tool":`, `"command"`, `"operation"`, `"name"`, `"arguments"`}
					highCount := 0
					for _, k := range highSpecKeys {
						if strings.Contains(trimmed, k) {
							highCount++
						}
					}
					ambCount := 0
					for _, k := range ambiguousKeys {
						if strings.Contains(trimmed, k) {
							ambCount++
						}
					}
					suppressToolCallJSON = highCount >= 1 || (highCount+ambCount >= 2 && ambCount >= 1) || ambCount >= 3
					if !suppressToolCallJSON && strings.Contains(trimmed, `"action"`) &&
						(strings.Contains(trimmed, `"arguments"`) || strings.Contains(trimmed, `"tool"`) || strings.Contains(trimmed, `"tool_name"`)) {
						suppressToolCallJSON = true
					}
				}
				if !suppressToolCallJSON && !xmlToolCallSuppressed {
					if len(doneTagStreamBuf)+len(delta.Content) > doneTagStreamBufMaxLen {
						doneTagStreamBuf = doneTagStreamBuf[len(doneTagStreamBuf)-doneTagHoldLen:]
					}
					doneTagStreamBuf += delta.Content
					var toSend string
					if len(doneTagStreamBuf) > doneTagHoldLen {
						toSend = doneTagStreamBuf[:len(doneTagStreamBuf)-doneTagHoldLen]
						doneTagStreamBuf = doneTagStreamBuf[len(doneTagStreamBuf)-doneTagHoldLen:]
					}
					toSend = strings.ReplaceAll(toSend, doneTagStr, "")
					if idx := strings.Index(strings.ToLower(toSend), minimaxToolCallPrefix); idx != -1 {
						toSend = toSend[:idx]
						xmlToolCallSuppressed = true
						doneTagStreamBuf = ""
					}
					if !xmlToolCallSuppressed {
						if idx := strings.Index(strings.ToLower(toSend), xmlToolCallPrefix); idx != -1 {
							toSend = toSend[:idx]
							xmlToolCallSuppressed = true
							doneTagStreamBuf = ""
						}
					}
					if !xmlToolCallSuppressed {
						if idx := strings.Index(strings.ToLower(toSend), actionTagPrefix); idx != -1 {
							toSend = toSend[:idx]
							xmlToolCallSuppressed = true
							doneTagStreamBuf = ""
						}
					}
					if !xmlToolCallSuppressed {
						if idx := strings.Index(strings.ToLower(toSend), toolResponsePrefix); idx != -1 {
							toSend = toSend[:idx]
							xmlToolCallSuppressed = true
							doneTagStreamBuf = ""
						}
					}
					if toSend != "" {
						broker.SendLLMStreamDelta(toSend, "", "", chunk.Choices[0].Index, "")
					}
				}
			}
			for _, tc := range delta.ToolCalls {
				tcAssembler.Merge(tc)
			}
		}
	}
	_ = recvEg.Wait()
	stm.Close()
	if midStreamError != nil {
		return streamingResponseResult{
			err:              midStreamError,
			contextCancelled: contextCancelled,
		}
	}
	if doneTagStreamBuf != "" && !xmlToolCallSuppressed {
		remaining := strings.ReplaceAll(doneTagStreamBuf, doneTagStr, "")
		if idx := strings.Index(strings.ToLower(remaining), minimaxToolCallPrefix); idx != -1 {
			remaining = remaining[:idx]
		}
		if idx := strings.Index(strings.ToLower(remaining), xmlToolCallPrefix); idx != -1 {
			remaining = remaining[:idx]
		}
		if idx := strings.Index(strings.ToLower(remaining), actionTagPrefix); idx != -1 {
			remaining = remaining[:idx]
		}
		if idx := strings.Index(strings.ToLower(remaining), toolResponsePrefix); idx != -1 {
			remaining = remaining[:idx]
		}
		if remaining != "" {
			broker.SendLLMStreamDelta(remaining, "", "", 0, "")
		}
		doneTagStreamBuf = ""
	}
	broker.SendLLMStreamDone(lastFinishReason)
	content := assembledResponse.String()

	assembledToolCalls := tcAssembler.Assemble()
	if len(assembledToolCalls) > 0 {
		currentLogger.Info("[Stream] Assembled streamed tool calls", "count", len(assembledToolCalls))
	}

	var promptTokens, completionTokens, totalTokens int
	tokenSource := ""
	if streamAcct.hasProviderUsage {
		promptTokens = streamAcct.providerPrompt
		completionTokens = streamAcct.providerCompletion
		totalTokens = promptTokens + completionTokens
		tokenSource = "provider_usage"
	} else if contextCancelled {
		promptTokens = 0
		completionTokens = 0
		totalTokens = 0
		tokenSource = "provider_usage"
	} else {
		completionTokens = estimateTokensForModel(content, req.Model)
		for _, m := range req.Messages {
			promptTokens += estimateTokensForModel(messageText(m), req.Model)
		}
		totalTokens = promptTokens + completionTokens
		tokenSource = "fallback_estimate"
	}

	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   content,
				ToolCalls: assembledToolCalls,
			}},
		},
		Usage: openai.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		},
	}
	return streamingResponseResult{
		resp:             resp,
		content:          content,
		promptTokens:     promptTokens,
		completionTokens: completionTokens,
		totalTokens:      totalTokens,
		tokenSource:      tokenSource,
		contextCancelled: contextCancelled,
	}
}

// recoveryResult holds output of handleSyncLLMCall.
type recoveryResult struct {
	resp             openai.ChatCompletionResponse
	content          string
	err              error
	telemetryScope   AgentTelemetryScope
	recoveryContinue bool
}

// handleSyncLLMCall executes the non-streaming LLM call with retry and recovery.
func handleSyncLLMCall(
	llmCtx context.Context,
	req openai.ChatCompletionRequest,
	client llm.ChatClient,
	emptyRetried bool,
	recoveryPolicy RecoveryPolicy,
	currentLogger *slog.Logger,
	broker FeedbackBroker,
	telemetryScope AgentTelemetryScope,
	cancelResp func(),
	retry422Count *int,
) recoveryResult {
	if emptyRetried {
		resp, err := llm.ExecuteWithCustomRetry(llmCtx, client, req, currentLogger, broker, recoveryPolicy.emptyRetryIntervals(), recoveryPolicy.emptyRetryBaseDelay())
		if err != nil {
			cancelResp()
			telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
			if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, err, retry422Count, &req, currentLogger, broker, "Sync", telemetryScope); recovered {
				return recoveryResult{recoveryContinue: true, telemetryScope: telemetryScope}
			} else if recErr != nil {
				return recoveryResult{err: recErr, telemetryScope: telemetryScope}
			}
			return recoveryResult{err: err, telemetryScope: telemetryScope}
		}
		if len(resp.Choices) == 0 {
			cancelResp()
			return recoveryResult{err: fmt.Errorf("no choices returned from LLM"), telemetryScope: telemetryScope}
		}
		return recoveryResult{resp: resp, content: resp.Choices[0].Message.Content, telemetryScope: telemetryScope}
	} else {
		resp, err := llm.ExecuteWithRetry(llmCtx, client, req, currentLogger, broker)
		if err != nil {
			cancelResp()
			telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
			if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, err, retry422Count, &req, currentLogger, broker, "Sync", telemetryScope); recovered {
				return recoveryResult{recoveryContinue: true, telemetryScope: telemetryScope}
			} else if recErr != nil {
				return recoveryResult{err: recErr, telemetryScope: telemetryScope}
			}
			return recoveryResult{err: err, telemetryScope: telemetryScope}
		}
		if len(resp.Choices) == 0 {
			cancelResp()
			return recoveryResult{err: fmt.Errorf("no choices returned from LLM"), telemetryScope: telemetryScope}
		}
		return recoveryResult{resp: resp, content: resp.Choices[0].Message.Content, telemetryScope: telemetryScope}
	}
}
