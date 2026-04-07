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
	IsFinished         bool
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
	sanitized := strings.TrimSpace(security.StripThinkingTags(msg.Content))
	// Detect explicit completion signal from LLM. Strip the tag so it is never
	// shown to the user or passed to the announcement detector as text.
	if strings.Contains(sanitized, "<done/>") {
		result.IsFinished = true
		sanitized = strings.TrimSpace(strings.ReplaceAll(sanitized, "<done/>", ""))
	}
	result.SanitizedContent = sanitized
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

	// Fast path: detect [TOOL_CALL] bracket format on raw content before sanitization.
	// StripThinkingTags may remove [/TOOL_CALL] closing tags via hallucinatedRagRe,
	// which breaks bracket detection on the sanitized content path. When content was
	// modified by stripping AND contains [TOOL_CALL], try the raw content first.
	if result.SanitizedContent != result.Content && strings.Contains(strings.ToLower(result.Content), "[tool_call]") {
		bracketTC := ParseToolCall(result.Content)
		if bracketTC.IsTool {
			result.ToolCall = bracketTC
			result.ParseSource = ToolCallParseSourceReasoningCleanJSON
			result.PendingToolCalls = extractExtraToolCalls(result.Content, bracketTC.RawJSON)
			RecordToolParseSourceForScope(scope, result.ParseSource)
			return result
		}
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
	lowerMsg := strings.ToLower(msg)
	return strings.Contains(msg, "422") ||
		strings.Contains(lowerMsg, "unprocessable") ||
		(strings.Contains(msg, "400") &&
			(strings.Contains(lowerMsg, "invalid function arguments json string") ||
				strings.Contains(lowerMsg, "invalid params") ||
				strings.Contains(lowerMsg, "tool_call_id")))
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
			logger.Error("["+path+"] Provider tool-call recovery retry limit reached — aborting", "attempts", *retryCount)
		}
		return false, fmt.Errorf("provider tool-call recovery retry limit exceeded after %d attempts: %w", *retryCount, err)
	}
	if logger != nil {
		logger.Warn("["+path+"] Provider rejected tool-call history — trimming malformed history", "error", err, "attempt", *retryCount)
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

	// Always keep the system prompt (index 0, and optionally index 1 if also system).
	trimmed := []openai.ChatCompletionMessage{msgs[0]}
	startIdx := 1
	if len(msgs) > 1 && msgs[1].Role == openai.ChatMessageRoleSystem {
		trimmed = append(trimmed, msgs[1])
		startIdx = 2
	}
	historyMsgs := msgs[startIdx:]

	// Find the last genuine user message (non-empty, non-internal tool noise).
	// It represents the original user intent and must survive the trim so the LLM
	// knows what task it was working on.  Without this, after an XML-fallback cycle
	// inflates context and triggers an empty response, the trimmed context loses the
	// task entirely and the LLM replies with just descriptive text instead of acting.
	lastUserIdx := -1
	for i := len(historyMsgs) - 1; i >= 0; i-- {
		if historyMsgs[i].Role == openai.ChatMessageRoleUser {
			lastUserIdx = i
			break
		}
	}

	const keepTail = 4
	if len(historyMsgs) <= keepTail {
		return append(trimmed, historyMsgs...)
	}

	tail := historyMsgs[len(historyMsgs)-keepTail:]

	// If the last user message is already inside the tail window, nothing extra needed.
	if lastUserIdx >= len(historyMsgs)-keepTail {
		return append(trimmed, tail...)
	}

	// Last user message is outside the tail — prepend it so the LLM retains the intent.
	if lastUserIdx >= 0 {
		return append(trimmed, append([]openai.ChatCompletionMessage{historyMsgs[lastUserIdx]}, tail...)...)
	}

	return append(trimmed, tail...)
}

func recoverFromEmptyResponse(resp openai.ChatCompletionResponse, content string, req *openai.ChatCompletionRequest, emptyRetried *bool, logger *slog.Logger, broker FeedbackBroker, scope AgentTelemetryScope) bool {
	return recoverFromEmptyResponseWithPolicy(defaultRecoveryPolicy(), resp, content, req, emptyRetried, logger, broker, scope)
}

func recoverFromEmptyResponseWithPolicy(policy RecoveryPolicy, resp openai.ChatCompletionResponse, content string, req *openai.ChatCompletionRequest, emptyRetried *bool, logger *slog.Logger, broker FeedbackBroker, scope AgentTelemetryScope) bool {
	// Treat a response that contains only <think>…</think> blocks (and nothing visible
	// after stripping them) as effectively empty — the model spent all tokens reasoning
	// but produced no actual output.  Without this check the raw content is non-empty
	// (it holds the think tags), bypassing the empty-response recovery and silently
	// ending the agent loop with no user-visible output.
	effectivelyEmpty := strings.TrimSpace(content) == "" || strings.TrimSpace(security.StripThinkingTags(content)) == ""
	if *emptyRetried || !effectivelyEmpty || len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) > 0 || len(req.Messages) < policy.minMessagesForEmptyRetry() {
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
