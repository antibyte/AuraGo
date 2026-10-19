package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

var errStreamIdleTimeout = errors.New("model stream stalled")
var errStreamFirstTokenTimeout = errors.New("model first-token deadline exceeded")

// streamingResponseResult holds the output of handleStreamingResponse.
type streamingResponseResult struct {
	resp                 openai.ChatCompletionResponse
	content              string
	promptTokens         int
	completionTokens     int
	totalTokens          int
	tokenSource          string
	contextCancelled     bool
	interruptedReasoning string
	usedFallbackEstimate bool
	err                  error
	recoveryContinue     bool
	recoveredMessages    []openai.ChatCompletionMessage
}

func shouldSuppressStreamedToolCallJSON(content string) bool {
	trimmed := normalizeStreamedToolCallJSONCandidate(content)
	if !strings.HasPrefix(trimmed, "{") {
		return false
	}

	toolKeys := []string{`"action"`, `"tool":`, `"tool_call"`, `"tool_name"`, `"tool_call_path"`}
	argKeys := []string{`"parameters"`, `"params"`, `"arguments"`, `"operation"`, `"command"`}
	toolKeyCount := 0
	for _, key := range toolKeys {
		if strings.Contains(trimmed, key) {
			toolKeyCount++
		}
	}
	argKeyCount := 0
	for _, key := range argKeys {
		if strings.Contains(trimmed, key) {
			argKeyCount++
		}
	}
	if toolKeyCount > 0 && argKeyCount > 0 {
		return true
	}
	if strings.Contains(trimmed, `"name"`) && strings.Contains(trimmed, `"arguments"`) {
		return true
	}
	return toolKeyCount >= 2 || argKeyCount >= 3
}

func shouldHoldPotentialStreamedToolCallJSON(content string) bool {
	rawTrimmed := strings.ToLower(strings.TrimLeft(content, " \t\r\n"))
	if isStreamedJSONFencePrefix(rawTrimmed) {
		return true
	}
	trimmed := normalizeStreamedToolCallJSONCandidate(content)
	if !strings.HasPrefix(trimmed, "{") {
		return false
	}
	for _, keyPrefix := range []string{`"action`, `"tool`, `"parameters`, `"params`, `"arguments`, `"operation`, `"command`, `"name`} {
		if strings.Contains(trimmed, keyPrefix) {
			return true
		}
	}
	return false
}

func normalizeStreamedToolCallJSONCandidate(content string) string {
	trimmed := strings.TrimLeft(content, " \t\r\n")
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "```") {
		return lower
	}

	body := trimmed[3:]
	lineEnd := strings.IndexAny(body, "\r\n")
	if lineEnd == -1 {
		return lower
	}
	lang := strings.TrimSpace(strings.ToLower(body[:lineEnd]))
	if lang != "" && lang != "json" && lang != "jsonc" {
		return lower
	}

	body = strings.TrimLeft(body[lineEnd:], " \t\r\n")
	lowerBody := strings.ToLower(body)
	if fenceIdx := strings.LastIndex(lowerBody, "\n```"); fenceIdx != -1 {
		lowerBody = strings.TrimSpace(lowerBody[:fenceIdx])
	}
	return lowerBody
}

func isStreamedJSONFencePrefix(trimmedLower string) bool {
	if !strings.HasPrefix(trimmedLower, "```") {
		return false
	}
	rest := strings.TrimPrefix(trimmedLower, "```")
	lineEnd := strings.IndexAny(rest, "\r\n")
	if lineEnd == -1 {
		langPrefix := strings.TrimSpace(rest)
		return langPrefix == "" || strings.HasPrefix("json", langPrefix) || strings.HasPrefix("jsonc", langPrefix)
	}
	lang := strings.TrimSpace(rest[:lineEnd])
	return lang == "" || lang == "json" || lang == "jsonc"
}

// utf8SafePrefixSplit keeps the byte-sized stream hold buffer from cutting a
// multibyte rune into two invalid chunks before SSE/JSON encoding.
func utf8SafePrefixSplit(s string, maxPrefixBytes int) (string, string) {
	if maxPrefixBytes <= 0 {
		return "", s
	}
	if maxPrefixBytes >= len(s) {
		return s, ""
	}
	split := maxPrefixBytes
	for split > 0 && split < len(s) && !utf8.RuneStart(s[split]) {
		split--
	}
	return s[:split], s[split:]
}

// utf8SafeSuffix returns at least minSuffixBytes without starting inside a rune.
func utf8SafeSuffix(s string, minSuffixBytes int) string {
	if minSuffixBytes <= 0 {
		return ""
	}
	if minSuffixBytes >= len(s) {
		return s
	}
	start := len(s) - minSuffixBytes
	for start > 0 && start < len(s) && !utf8.RuneStart(s[start]) {
		start--
	}
	return s[start:]
}

func streamedToolCallTextPrefixes() []string {
	return []string{
		"minimax:tool_call",
		"<tool_call",
		"<action>",
		"<tts",
		"<tool_response",
		"[tool_call]",
		"<function",
		"<invoke",
	}
}

func shouldSuppressStreamedToolCallText(content string) (int, bool) {
	combined := strings.ToLower(content)
	for _, prefix := range streamedToolCallTextPrefixes() {
		if idx := strings.Index(combined, prefix); idx != -1 {
			return idx, true
		}
	}
	return -1, false
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
	retry422Count *int,
	requireComplete ...bool,
) streamingResponseResult {
	llmCtx, thinkingProgress := withStreamThinkingProgress(llmCtx, broker)
	strict := len(requireComplete) > 0 && requireComplete[0]
	streamAcct := streamingAccountingState{}
	contextCancelled := false
	var stm llm.CompletionStream
	streamCancel := func() {}
	var streamErr error
	var midStreamError error
	if emptyRetried {
		stm, streamCancel, streamErr = llm.ExecuteStreamWithCustomRetry(llmCtx, client, req, currentLogger, broker, recoveryPolicy.emptyRetryIntervals(), recoveryPolicy.emptyRetryBaseDelay())
	} else {
		stm, streamCancel, streamErr = llm.ExecuteStreamWithRetry(llmCtx, client, req, currentLogger, broker)
	}
	if streamErr != nil {
		cancelResp()
		telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
		if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, streamErr, retry422Count, &req, currentLogger, broker, "Stream", telemetryScope); recovered {
			return streamingResponseResult{recoveryContinue: true, recoveredMessages: req.Messages}
		} else if recErr != nil {
			return streamingResponseResult{err: recErr}
		}
		return streamingResponseResult{err: streamErr}
	}
	defer streamCancel()
	defer stm.Close()

	var assembledResponse strings.Builder
	legacyFunctionCallSeen := false
	var assembledReasoning strings.Builder
	var lastFinishReason string
	var responseModel string
	tcAssembler := NewStreamToolCallAssembler()

	const doneTagStr = "<done/>"
	// holdLen must cover the longest tag prefix so that it is never split across
	// the send/hold boundary.  With holdLen == P the entire P-byte prefix stays
	// in the hold buffer until the next chunk arrives, guaranteeing detection.
	const doneTagHoldLen = len("minimax:tool_call") // 17 bytes (>= all other prefixes)
	const doneTagStreamBufMaxLen = 8192             // max buffer to prevent unbounded growth
	const toolCallJSONHoldMaxLen = 512              // enough to classify common JSON tool-call wrappers
	doneTagStreamBuf := ""
	xmlToolCallSuppressed := false // once true, suppress all remaining stream chunks
	type recvResult struct {
		chunk openai.ChatCompletionStreamResponse
		err   error
	}

	recvCh := make(chan recvResult, 1)
	recvCtx, cancelRecv := context.WithCancel(llmCtx)
	defer cancelRecv()
	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		defer close(recvCh)
		defer func() {
			if recover() != nil {
				select {
				case recvCh <- recvResult{err: ErrRuntimePanic}:
				case <-recvCtx.Done():
				}
			}
		}()
		for {
			chunk, rErr := stm.Recv()
			select {
			case recvCh <- recvResult{chunk: chunk, err: rErr}:
			case <-recvCtx.Done():
				return
			}
			if rErr != nil {
				return
			}
		}
	}()

	idleTimeout := chunkIdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second
	}
	// The first generated token may need the complete call budget. After
	// progress begins, only output (not empty keepalives) renews the idle timer.
	waitTimeout := idleTimeout
	if deadline, ok := llmCtx.Deadline(); ok && time.Until(deadline) > waitTimeout {
		waitTimeout = time.Until(deadline)
	}
	timer := time.NewTimer(waitTimeout)
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
		waitTimeout = idleTimeout
		timer.Reset(idleTimeout)
	}

	for {
		var rr recvResult
		var ok bool
		select {
		case <-thinkingProgress.ready:
			markProgress()
			continue
		case <-llmCtx.Done():
			midStreamError = fmt.Errorf("model stream interrupted: %w", llmCtx.Err())
			if !progressSeen && errors.Is(llmCtx.Err(), context.DeadlineExceeded) {
				midStreamError = fmt.Errorf("%w: %w", errStreamFirstTokenTimeout, llmCtx.Err())
			}
			contextCancelled = true
			cancelRecv()
			stm.Close()
			ok = false
		case <-timer.C:
			midStreamError = fmt.Errorf("%w for %s: %w", errStreamIdleTimeout, waitTimeout, context.DeadlineExceeded)
			operation := "stream_idle_timeout"
			if !progressSeen {
				midStreamError = fmt.Errorf("%w after %s: %w", errStreamFirstTokenTimeout, waitTimeout, context.DeadlineExceeded)
				operation = "stream_first_token_timeout"
			}
			currentLogger.Warn("[Stream] No chunks received within idle timeout; aborting stream", "timeout", idleTimeout.String())
			telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
			llm.ReportLLMHealthEvent(llm.HealthEvent{
				Operation:         operation,
				Provider:          telemetryScope.ProviderType,
				Model:             telemetryScope.Model,
				ErrorCategory:     llm.ErrCategoryContextDeadline,
				ErrorSummary:      fmt.Sprintf("no stream chunks received within %s", idleTimeout),
				Retryable:         true,
				PerAttemptTimeout: idleTimeout,
			})
			cancelResp()
			stm.Close()
			contextCancelled = true
			ok = false
		case rr, ok = <-recvCh:
			if !ok {
				break
			}
		}
		if !ok {
			break
		}

		chunk, rErr := rr.chunk, rr.err
		if rErr != nil {
			if !errors.Is(rErr, io.EOF) {
				midStreamError = fmt.Errorf("model stream interrupted: %w", rErr)
			}
			if !errors.Is(rErr, io.EOF) {
				if llmCtx.Err() == context.Canceled || llm.IsContextError(rErr) {
					currentLogger.Debug("Stream canceled", "error", rErr)
					contextCancelled = true
				} else {
					currentLogger.Error("Stream error", "error", rErr)
					midStreamError = fmt.Errorf("stream error before done: %w", rErr)
				}
			}
			if llmCtx.Err() == context.Canceled {
				contextCancelled = true
			}
			break
		}
		if chunk.Usage != nil {
			cachedTokens := 0
			if chunk.Usage.PromptTokensDetails != nil {
				streamAcct.providerCacheReported = true
				cachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
			}
			streamAcct.recordProviderUsage(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens, chunk.Usage.TotalTokens, cachedTokens)
		}
		if chunk.Model != "" {
			responseModel = chunk.Model
		}
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].FinishReason != "" {
				lastFinishReason = string(chunk.Choices[0].FinishReason)
			}
			delta := chunk.Choices[0].Delta
			legacyFunctionCallSeen = legacyFunctionCallSeen || delta.FunctionCall != nil
			streamed := thinkingProgress.reasoningChars(delta.ReasoningContent) + len(delta.Content)
			for _, call := range delta.ToolCalls {
				streamed += len(call.Function.Arguments)
			}
			llm.ObserveStreamOutput(llmCtx, streamed)
			if streamed > 0 || len(delta.ToolCalls) > 0 || chunk.Choices[0].FinishReason != "" {
				markProgress()
			}
			if delta.ReasoningContent != "" {
				assembledReasoning.WriteString(delta.ReasoningContent)
			}
			if strict && (delta.ReasoningContent != "" || delta.Content != "" || len(delta.ToolCalls) > 0) {
				broker.Send("model_progress", "receiving")
			}
			if delta.Content != "" {
				assembledResponse.WriteString(delta.Content)
				suppressToolCallJSON := shouldSuppressStreamedToolCallJSON(delta.Content)
				if suppressToolCallJSON {
					xmlToolCallSuppressed = true
					doneTagStreamBuf = ""
				}
				if !suppressToolCallJSON && !xmlToolCallSuppressed {
					if len(doneTagStreamBuf)+len(delta.Content) > doneTagStreamBufMaxLen {
						doneTagStreamBuf = utf8SafeSuffix(doneTagStreamBuf, doneTagHoldLen)
					}
					doneTagStreamBuf += delta.Content
					if shouldSuppressStreamedToolCallJSON(doneTagStreamBuf) {
						xmlToolCallSuppressed = true
						doneTagStreamBuf = ""
						continue
					}
					if shouldHoldPotentialStreamedToolCallJSON(doneTagStreamBuf) && len(doneTagStreamBuf) < toolCallJSONHoldMaxLen {
						continue
					}
					var toSend string
					if len(doneTagStreamBuf) > doneTagHoldLen {
						toSend, doneTagStreamBuf = utf8SafePrefixSplit(doneTagStreamBuf, len(doneTagStreamBuf)-doneTagHoldLen)
					}
					toSend = strings.ReplaceAll(toSend, doneTagStr, "")
					// Check combined toSend+holdBuffer for prefixes so that a prefix
					// straddling the send/hold boundary is still detected and suppressed.
					if idx, ok := shouldSuppressStreamedToolCallText(toSend + doneTagStreamBuf); ok {
						if idx < len(toSend) {
							toSend = toSend[:idx]
						}
						xmlToolCallSuppressed = true
						doneTagStreamBuf = ""
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
	cancelRecv()
	stm.Close()
	select {
	case <-recvDone:
	case <-time.After(time.Second):
		currentLogger.Error("Model stream receiver did not stop after cancellation")
	}
	if midStreamError == nil {
		if llmCtx.Err() != nil {
			midStreamError = fmt.Errorf("model stream interrupted: %w", llmCtx.Err())
		} else if lastFinishReason == "" {
			midStreamError = incompleteCompletion("", assembledResponse.String(), assembledReasoning.String(), legacyFunctionCallSeen || len(tcAssembler.Assemble()) > 0)
		} else if lastFinishReason != "stop" && lastFinishReason != "tool_calls" {
			midStreamError = incompleteCompletion(openai.FinishReason(lastFinishReason), assembledResponse.String(), assembledReasoning.String(), legacyFunctionCallSeen || len(tcAssembler.Assemble()) > 0)
		}
	}
	if strict {
		currentLogger.Info("[Stream] Completion summary", "finish_reason", lastFinishReason, "text_bytes", assembledResponse.Len(), "reasoning_bytes", assembledReasoning.Len(), "tool_calls", len(tcAssembler.Assemble()), "interrupted", midStreamError != nil)
	}
	if midStreamError == nil && doneTagStreamBuf != "" && !xmlToolCallSuppressed {
		remaining := strings.ReplaceAll(doneTagStreamBuf, doneTagStr, "")
		if idx, ok := shouldSuppressStreamedToolCallText(remaining); ok {
			remaining = remaining[:idx]
		}
		if remaining != "" {
			broker.SendLLMStreamDelta(remaining, "", "", 0, "")
		}
		doneTagStreamBuf = ""
	}
	if midStreamError == nil {
		broker.SendLLMStreamDone(lastFinishReason)
	}
	content := assembledResponse.String()
	reasoningContent := assembledReasoning.String()

	assembledToolCalls := tcAssembler.Assemble()
	if len(assembledToolCalls) > 0 {
		currentLogger.Info("[Stream] Assembled streamed tool calls", "count", len(assembledToolCalls))
	}

	usage := openai.Usage{
		PromptTokens:     streamAcct.providerPrompt,
		CompletionTokens: streamAcct.providerCompletion,
		TotalTokens:      streamAcct.providerTotal,
	}
	if streamAcct.providerCacheReported {
		usage.PromptTokensDetails = &openai.PromptTokensDetails{CachedTokens: streamAcct.providerCached}
	}

	resp := openai.ChatCompletionResponse{
		Model: responseModel,
		Choices: []openai.ChatCompletionChoice{
			{FinishReason: openai.FinishReason(lastFinishReason), Message: openai.ChatCompletionMessage{
				Role:             openai.ChatMessageRoleAssistant,
				Content:          content,
				ReasoningContent: reasoningContent,
				ToolCalls:        assembledToolCalls,
			}},
		},
		Usage: usage,
	}
	resp, promptTokens, completionTokens, totalTokens, tokenSource, usedFallbackEstimate, shouldAccount := normalizeResponseUsage(req, resp)
	if !shouldAccount {
		promptTokens, completionTokens, totalTokens, tokenSource, usedFallbackEstimate = 0, 0, 0, "", false
	}
	if midStreamError != nil {
		resp.Choices = nil
	}
	return streamingResponseResult{
		err:                  midStreamError,
		interruptedReasoning: reasoningContent,
		resp:                 resp,
		content:              content,
		promptTokens:         promptTokens,
		completionTokens:     completionTokens,
		totalTokens:          totalTokens,
		tokenSource:          tokenSource,
		usedFallbackEstimate: usedFallbackEstimate,
		contextCancelled:     contextCancelled,
	}
}

// recoveryResult holds output of handleSyncLLMCall.
type recoveryResult struct {
	resp              openai.ChatCompletionResponse
	content           string
	err               error
	telemetryScope    AgentTelemetryScope
	recoveryContinue  bool
	recoveredMessages []openai.ChatCompletionMessage
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
				return recoveryResult{resp: resp, content: syncResponseContent(resp), recoveryContinue: true, recoveredMessages: req.Messages, telemetryScope: telemetryScope}
			} else if recErr != nil {
				return recoveryResult{resp: resp, content: syncResponseContent(resp), err: recErr, telemetryScope: telemetryScope}
			}
			return recoveryResult{resp: resp, content: syncResponseContent(resp), err: err, telemetryScope: telemetryScope}
		}
		if len(resp.Choices) == 0 {
			cancelResp()
			return recoveryResult{resp: resp, err: fmt.Errorf("no choices returned from LLM"), telemetryScope: telemetryScope}
		}
		return recoveryResult{resp: resp, content: resp.Choices[0].Message.Content, err: syncCompletionError(llmCtx, resp), telemetryScope: telemetryScope}
	} else {
		resp, err := llm.ExecuteWithRetry(llmCtx, client, req, currentLogger, broker)
		if err != nil {
			cancelResp()
			telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
			if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, err, retry422Count, &req, currentLogger, broker, "Sync", telemetryScope); recovered {
				return recoveryResult{resp: resp, content: syncResponseContent(resp), recoveryContinue: true, recoveredMessages: req.Messages, telemetryScope: telemetryScope}
			} else if recErr != nil {
				return recoveryResult{resp: resp, content: syncResponseContent(resp), err: recErr, telemetryScope: telemetryScope}
			}
			return recoveryResult{resp: resp, content: syncResponseContent(resp), err: err, telemetryScope: telemetryScope}
		}
		if len(resp.Choices) == 0 {
			cancelResp()
			return recoveryResult{resp: resp, err: fmt.Errorf("no choices returned from LLM"), telemetryScope: telemetryScope}
		}
		return recoveryResult{resp: resp, content: resp.Choices[0].Message.Content, err: syncCompletionError(llmCtx, resp), telemetryScope: telemetryScope}
	}
}

func syncCompletionError(ctx context.Context, resp openai.ChatCompletionResponse) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("model returned no completion")
	}
	reason := resp.Choices[0].FinishReason
	if reason != openai.FinishReasonStop && reason != openai.FinishReasonToolCalls {
		message := resp.Choices[0].Message
		return incompleteCompletion(reason, message.Content, message.ReasoningContent, len(message.ToolCalls) > 0 || message.FunctionCall != nil)
	}
	return nil
}

func syncResponseContent(resp openai.ChatCompletionResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}
