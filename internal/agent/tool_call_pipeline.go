package agent

import (
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

type ToolCallParseSource string

const (
	ToolCallParseSourceNone               ToolCallParseSource = "none"
	ToolCallParseSourceNative             ToolCallParseSource = "native"
	ToolCallParseSourceReasoningCleanJSON ToolCallParseSource = "reasoning_clean_json"
	ToolCallParseSourceContentJSON        ToolCallParseSource = "content_json"
)

type ParsedToolResponse struct {
	Content            string
	SanitizedContent   string
	ToolCall           ToolCall
	PendingToolCalls   []ToolCall
	UseNativePath      bool
	NativeAssistantMsg openai.ChatCompletionMessage
	ParseSource        ToolCallParseSource
}

func parseToolResponse(resp openai.ChatCompletionResponse, logger *slog.Logger, scope AgentTelemetryScope) ParsedToolResponse {
	result := ParsedToolResponse{
		ParseSource: ToolCallParseSourceNone,
	}
	if len(resp.Choices) == 0 {
		return result
	}

	msg := resp.Choices[0].Message
	result.Content = msg.Content
	result.SanitizedContent = strings.TrimSpace(security.StripThinkingTags(msg.Content))
	result.NativeAssistantMsg = msg

	if len(msg.ToolCalls) > 0 {
		result.UseNativePath = true
		result.ParseSource = ToolCallParseSourceNative
		result.ToolCall = NativeToolCallToToolCall(msg.ToolCalls[0], logger)
		if len(msg.ToolCalls) > 1 {
			result.PendingToolCalls = make([]ToolCall, 0, len(msg.ToolCalls)-1)
			for _, extra := range msg.ToolCalls[1:] {
				result.PendingToolCalls = append(result.PendingToolCalls, NativeToolCallToToolCall(extra, logger))
			}
		}
		RecordToolParseSourceForScope(scope, result.ParseSource)
		return result
	}

	if result.SanitizedContent != "" {
		result.ToolCall = ParseToolCall(result.SanitizedContent)
		if result.ToolCall.IsTool {
			if result.SanitizedContent != result.Content {
				result.ParseSource = ToolCallParseSourceReasoningCleanJSON
			} else {
				result.ParseSource = ToolCallParseSourceContentJSON
			}
			result.PendingToolCalls = extractExtraToolCalls(result.SanitizedContent, result.ToolCall.RawJSON)
			RecordToolParseSourceForScope(scope, result.ParseSource)
			return result
		}
	}

	if result.SanitizedContent != result.Content {
		result.ToolCall = ParseToolCall(result.Content)
		if result.ToolCall.IsTool {
			result.ParseSource = ToolCallParseSourceContentJSON
			result.PendingToolCalls = extractExtraToolCalls(result.Content, result.ToolCall.RawJSON)
		}
	}
	if result.ParseSource != ToolCallParseSourceNone {
		RecordToolParseSourceForScope(scope, result.ParseSource)
	}

	return result
}

func isUnprocessableProviderError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "422") || strings.Contains(strings.ToLower(msg), "unprocessable")
}

func recoverFrom422(err error, retryCount *int, req *openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackBroker, path string, scope AgentTelemetryScope) (bool, error) {
	return recoverFrom422WithPolicy(defaultRecoveryPolicy(), err, retryCount, req, logger, broker, path, scope)
}

func recoverFrom422WithPolicy(policy RecoveryPolicy, err error, retryCount *int, req *openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackBroker, path string, scope AgentTelemetryScope) (bool, error) {
	if !isUnprocessableProviderError(err) {
		return false, nil
	}
	*retryCount = *retryCount + 1
	RecordToolRecoveryEventForScope(scope, "provider_422_recovered")
	if *retryCount > policy.maxProvider422Recoveries() {
		RecordToolRecoveryEventForScope(scope, "provider_422_aborted")
		if logger != nil {
			logger.Error("["+path+"] 422 retry limit reached — aborting", "attempts", *retryCount)
		}
		return false, fmt.Errorf("422 Unprocessable: retry limit exceeded after %d attempts: %w", *retryCount, err)
	}
	if logger != nil {
		logger.Warn("["+path+"] 422 Unprocessable from provider — trimming malformed history", "error", err, "attempt", *retryCount)
	}
	if broker != nil {
		broker.Send("thinking", "Context error recovered — retrying...")
	}
	req.Messages = trim422Messages(req.Messages)
	if logger != nil {
		logger.Info("["+path+"] Context trimmed after 422, retrying", "new_messages_count", len(req.Messages), "attempt", *retryCount)
	}
	return true, nil
}

func trimMessagesForEmptyResponse(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(msgs) == 0 {
		return msgs
	}

	trimmed := []openai.ChatCompletionMessage{msgs[0]}
	startIdx := 1
	if len(msgs) > 1 && msgs[1].Role == openai.ChatMessageRoleSystem {
		trimmed = append(trimmed, msgs[1])
		startIdx = 2
	}
	historyMsgs := msgs[startIdx:]
	if len(historyMsgs) > 4 {
		historyMsgs = historyMsgs[len(historyMsgs)-4:]
	}
	return append(trimmed, historyMsgs...)
}

func recoverFromEmptyResponse(resp openai.ChatCompletionResponse, content string, req *openai.ChatCompletionRequest, emptyRetried *bool, logger *slog.Logger, broker FeedbackBroker, scope AgentTelemetryScope) bool {
	return recoverFromEmptyResponseWithPolicy(defaultRecoveryPolicy(), resp, content, req, emptyRetried, logger, broker, scope)
}

func recoverFromEmptyResponseWithPolicy(policy RecoveryPolicy, resp openai.ChatCompletionResponse, content string, req *openai.ChatCompletionRequest, emptyRetried *bool, logger *slog.Logger, broker FeedbackBroker, scope AgentTelemetryScope) bool {
	if *emptyRetried || strings.TrimSpace(content) != "" || len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) > 0 || len(req.Messages) < policy.minMessagesForEmptyRetry() {
		return false
	}

	*emptyRetried = true
	RecordToolRecoveryEventForScope(scope, "empty_response_recovered")
	if logger != nil {
		logger.Warn("[Sync] Empty LLM response detected, trimming history and retrying", "messages_count", len(req.Messages))
	}
	if broker != nil {
		broker.Send("thinking", "Context too large, retrimming...")
	}
	req.Messages = trimMessagesForEmptyResponse(req.Messages)
	if logger != nil {
		logger.Info("[Sync] Retrying with trimmed context", "new_messages_count", len(req.Messages))
	}
	return true
}
