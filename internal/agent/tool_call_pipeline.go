package agent

import (
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// bareToolCallTagRe matches bare <tool_call>, </tool_call>, <tool_call/>, or
// minimax:tool_call markers that have no JSON body following them.
var bareToolCallTagRe = regexp.MustCompile(`(?i)</?tool_call/?>|minimax:tool_call`)
var ttsBlockRe = regexp.MustCompile(`(?is)<tts\b[^>]*>(.*?)</tts>`)
var ttsTagRe = regexp.MustCompile(`(?i)</?tts\b[^>]*>`)
var trailingLineSpaceRe = regexp.MustCompile(`[ \t]+\n`)
var excessiveBlankLinesRe = regexp.MustCompile(`\n{3,}`)

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
	IncompleteToolCall bool // Model emitted a bare <tool_call> tag without a body
	ToolCall           ToolCall
	PendingToolCalls   []ToolCall
	UseNativePath      bool
	NativeAssistantMsg openai.ChatCompletionMessage
	ParseSource        ToolCallParseSource
}

type recoveryTrimSummary struct {
	Trigger                   string
	BeforeMessages            int
	AfterMessages             int
	LeadingSystemMessages     int
	PreservedLastUserIntent   bool
	PreservedLatestToolResult bool
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
	sanitized := strings.TrimSpace(stripTTSMarkup(security.StripThinkingTags(msg.Content)))
	// Detect explicit completion signal from LLM. Strip the tag so it is never
	// shown to the user or passed to the announcement detector as text.
	if strings.Contains(sanitized, "<done/>") {
		result.IsFinished = true
		sanitized = strings.TrimSpace(strings.ReplaceAll(sanitized, "<done/>", ""))
	}
	result.SanitizedContent = sanitized
	result.NativeAssistantMsg = msg

	// Detect bare <tool_call> tags without body — the model tried to tool-call
	// but emitted only the tag marker. Strip the tag from displayed content and
	// flag the response so the agent loop can nudge the model.
	if !result.IsFinished && bareToolCallTagRe.MatchString(sanitized) {
		cleaned := strings.TrimSpace(bareToolCallTagRe.ReplaceAllString(sanitized, ""))
		// Only flag as incomplete when stripping the tag doesn't reveal a
		// parseable tool call (some models emit JSON right after the tag).
		if tc := ParseToolCall(sanitized); !tc.IsTool {
			result.IncompleteToolCall = true
			result.SanitizedContent = cleaned
			if logger != nil {
				logger.Warn("[ToolCallPipeline] Bare <tool_call> tag detected without body", "raw_len", len(msg.Content))
			}
		}
	}

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
	// which breaks bracket detection on the sanitized content path. Parse the raw
	// content directly when it contains [TOOL_CALL].
	if strings.Contains(strings.ToLower(result.Content), "[tool_call]") {
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

func stripTTSMarkup(content string) string {
	if content == "" {
		return ""
	}
	cleaned := ttsBlockRe.ReplaceAllString(content, "$1")
	cleaned = ttsTagRe.ReplaceAllString(cleaned, "")
	cleaned = trailingLineSpaceRe.ReplaceAllString(cleaned, "\n")
	cleaned = excessiveBlankLinesRe.ReplaceAllString(cleaned, "\n\n")
	return strings.TrimSpace(cleaned)
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
				strings.Contains(lowerMsg, "tool_call_id") ||
				strings.Contains(lowerMsg, "tool id") ||
				strings.Contains(lowerMsg, "tool result"))) ||
		// OpenRouter error code 2013: tool result's tool_call_id not found
		strings.Contains(msg, "2013")
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
	beforeTrim := req.Messages
	req.Messages = trim422Messages(req.Messages)
	trimSummary := summarizeRecoveryTrim("provider_422", beforeTrim, req.Messages)
	if logger != nil {
		logger.Info("["+path+"] Context trimmed after 422, retrying",
			"attempt", *retryCount,
			"trigger", trimSummary.Trigger,
			"before_messages", trimSummary.BeforeMessages,
			"after_messages", trimSummary.AfterMessages,
			"leading_system_messages", trimSummary.LeadingSystemMessages,
			"preserved_last_user_intent", trimSummary.PreservedLastUserIntent,
			"preserved_latest_tool_result", trimSummary.PreservedLatestToolResult)
	}
	return true, nil
}

func trimMessagesForEmptyResponse(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	trimmed, _ := trimMessagesForEmptyResponseWithSummary(msgs)
	return trimmed
}

func trimMessagesForEmptyResponseWithSummary(msgs []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, recoveryTrimSummary) {
	if len(msgs) == 0 {
		return msgs, recoveryTrimSummary{Trigger: "empty_response"}
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
		trimmed = append(trimmed, historyMsgs...)
		return trimmed, summarizeRecoveryTrim("empty_response", msgs, trimmed)
	}

	tail := historyMsgs[len(historyMsgs)-keepTail:]
	preservedIndexes := make([]int, 0, 2)
	tailStart := len(historyMsgs) - keepTail

	if lastUserIdx >= 0 && lastUserIdx < tailStart {
		preservedIndexes = append(preservedIndexes, lastUserIdx)
	}
	if lastToolIdx := findLastRoleIndex(historyMsgs, openai.ChatMessageRoleTool); lastToolIdx >= 0 && lastToolIdx < tailStart {
		preservedIndexes = append(preservedIndexes, lastToolIdx)
	}
	if len(preservedIndexes) > 1 {
		sort.Ints(preservedIndexes)
	}
	lastIdx := -1
	for _, idx := range preservedIndexes {
		if idx == lastIdx {
			continue
		}
		trimmed = append(trimmed, historyMsgs[idx])
		lastIdx = idx
	}
	trimmed = append(trimmed, tail...)
	return trimmed, summarizeRecoveryTrim("empty_response", msgs, trimmed)
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
	strippedContent := strings.TrimSpace(security.StripThinkingTags(content))
	effectivelyEmpty := strings.TrimSpace(content) == "" || strippedContent == ""
	// Also treat unclosed <think> blocks as effectively empty.  This happens when the
	// model hits its token limit mid-reasoning: the response begins with <think> but
	// no closing </think> is emitted.  StripThinkingTags requires a closing tag, so
	// the raw <think>… text remains after stripping and the check above misses it.
	if !effectivelyEmpty {
		lowerStripped := strings.ToLower(strippedContent)
		if strings.HasPrefix(lowerStripped, "<think") && !strings.Contains(lowerStripped, "</think") {
			effectivelyEmpty = true
		}
	}
	if *emptyRetried || !effectivelyEmpty || len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) > 0 || len(req.Messages) < policy.minMessagesForEmptyRetry() {
		return false
	}

	*emptyRetried = true
	RecordToolRecoveryEventForScope(scope, "empty_response_recovered")
	emptyReason := "empty_response"
	if strings.TrimSpace(content) != "" && strippedContent == "" {
		emptyReason = "reasoning_only_response"
	} else if strings.HasPrefix(strings.ToLower(strings.TrimSpace(strippedContent)), "<think") {
		emptyReason = "unclosed_reasoning_response"
	}
	if logger != nil {
		logger.Warn("[Sync] Empty LLM response detected, trimming history and retrying",
			"messages_count", len(req.Messages),
			"trigger", emptyReason)
	}
	if broker != nil {
		broker.Send("thinking", "Context too large, retrimming...")
	}
	var trimSummary recoveryTrimSummary
	req.Messages, trimSummary = trimMessagesForEmptyResponseWithSummary(req.Messages)
	if logger != nil {
		logger.Info("[Sync] Retrying with trimmed context",
			"trigger", trimSummary.Trigger,
			"before_messages", trimSummary.BeforeMessages,
			"after_messages", trimSummary.AfterMessages,
			"leading_system_messages", trimSummary.LeadingSystemMessages,
			"preserved_last_user_intent", trimSummary.PreservedLastUserIntent,
			"preserved_latest_tool_result", trimSummary.PreservedLatestToolResult)
	}
	return true
}

func findLastRoleIndex(msgs []openai.ChatCompletionMessage, role string) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == role {
			return i
		}
	}
	return -1
}

func findLastMeaningfulUserMessage(msgs []openai.ChatCompletionMessage) (openai.ChatCompletionMessage, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == openai.ChatMessageRoleUser && strings.TrimSpace(msgs[i].Content) != "" {
			return msgs[i], true
		}
	}
	return openai.ChatCompletionMessage{}, false
}

func findLastRoleMessage(msgs []openai.ChatCompletionMessage, role string) (openai.ChatCompletionMessage, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == role {
			return msgs[i], true
		}
	}
	return openai.ChatCompletionMessage{}, false
}

func messageSliceContains(msgs []openai.ChatCompletionMessage, target openai.ChatCompletionMessage) bool {
	for _, msg := range msgs {
		if reflect.DeepEqual(msg, target) {
			return true
		}
	}
	return false
}

func countLeadingSystemMessages(msgs []openai.ChatCompletionMessage) int {
	count := 0
	for count < len(msgs) && msgs[count].Role == openai.ChatMessageRoleSystem {
		count++
	}
	return count
}

func summarizeRecoveryTrim(trigger string, original, trimmed []openai.ChatCompletionMessage) recoveryTrimSummary {
	summary := recoveryTrimSummary{
		Trigger:               trigger,
		BeforeMessages:        len(original),
		AfterMessages:         len(trimmed),
		LeadingSystemMessages: countLeadingSystemMessages(trimmed),
	}
	if lastUser, ok := findLastMeaningfulUserMessage(original); ok {
		summary.PreservedLastUserIntent = messageSliceContains(trimmed, lastUser)
	} else {
		summary.PreservedLastUserIntent = true
	}
	if lastTool, ok := findLastRoleMessage(original, openai.ChatMessageRoleTool); ok {
		summary.PreservedLatestToolResult = messageSliceContains(trimmed, lastTool)
	} else {
		summary.PreservedLatestToolResult = true
	}
	return summary
}
