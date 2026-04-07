package agent

import (
	"aurago/internal/llm"
	"aurago/internal/prompts"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// compressionCooldown is the minimum number of messages between two compression runs
// to avoid repeatedly compressing the same conversation window.
const compressionCooldown = 5

// compressionThresholdPct triggers compression when total tokens exceed this fraction
// of the context window (after reserving the completion margin).
const compressionThresholdPct = 0.80

// compressionKeepTail is the number of recent messages (excluding system) to always preserve.
// These are never compressed so the LLM retains recent conversational context.
const compressionKeepTail = 6

// summaryMaxTokens caps the LLM response for the summary generation.
const summaryMaxTokens = 300

// CompressHistoryResult describes what happened during a compression attempt.
type CompressHistoryResult struct {
	Compressed    bool
	DroppedCount  int
	SummaryTokens int
}

// CompressHistory checks whether the conversation history exceeds the compression
// threshold and, if so, summarises the oldest messages into a single system-role
// message. The summary replaces the compressed messages in-place.
//
// Parameters:
//   - messages: the full message slice (index 0 is the system prompt)
//   - maxHistoryTokens: the effective context budget (context_window - completion_margin)
//   - model: the LLM model identifier to use for the summary call
//   - client: LLM client for the summary request
//   - lastCompressionMsg: the message count at the last compression (for cooldown)
//   - logger: structured logger
//
// Returns the (possibly rewritten) messages, the updated lastCompressionMsg counter,
// and a result struct.
func CompressHistory(
	ctx context.Context,
	messages []openai.ChatCompletionMessage,
	maxHistoryTokens int,
	model string,
	client llm.ChatClient,
	lastCompressionMsg int,
	logger *slog.Logger,
) ([]openai.ChatCompletionMessage, int, CompressHistoryResult) {
	result := CompressHistoryResult{}

	// Need at least system + cooldown-tail + 1 compressible message
	if len(messages) < 2+compressionKeepTail {
		return messages, lastCompressionMsg, result
	}

	// Cooldown: don't compress again too soon
	if len(messages)-lastCompressionMsg < compressionCooldown {
		return messages, lastCompressionMsg, result
	}

	// Calculate total tokens in the compressible window (exclude system message at index 0,
	// which is never compressed and is counted separately in the prompt budget).
	totalTokens := 0
	for _, m := range messages[1:] {
		totalTokens += prompts.CountTokens(messageText(m)) + 4
	}

	threshold := int(float64(maxHistoryTokens) * compressionThresholdPct)
	if totalTokens <= threshold {
		return messages, lastCompressionMsg, result
	}

	logger.Info("[Compression] Token threshold exceeded, compressing history",
		"tokens", totalTokens, "threshold", threshold, "messages", len(messages))

	// Identify compressible window: everything between system (0) and the tail.
	//
	// Extend the protected tail backward to avoid splitting a tool-call group:
	// an assistant message with ToolCalls and its following role=tool messages
	// must stay together so the LLM retains multi-turn tool context.
	tailStart := len(messages) - compressionKeepTail
	if tailStart < 1 {
		tailStart = 1
	}
	// Walk backward from tailStart: if the message just before tailStart is a
	// role=tool result (it has a ToolCallID), keep pulling in earlier messages
	// until we reach the assistant message that triggered those tool calls.
	for tailStart > 1 {
		prev := messages[tailStart-1]
		if prev.Role == openai.ChatMessageRoleTool || prev.ToolCallID != "" {
			tailStart--
			continue
		}
		// Also protect the assistant message that contains the ToolCalls slice.
		if prev.Role == openai.ChatMessageRoleAssistant && len(prev.ToolCalls) > 0 {
			tailStart--
			continue
		}
		break
	}
	compressible := messages[1:tailStart]
	if len(compressible) == 0 {
		return messages, lastCompressionMsg, result
	}

	// Build a condensed transcript for the summary LLM call
	var transcript strings.Builder
	for _, m := range compressible {
		role := m.Role
		content := messageText(m)
		if len(content) > 500 {
			content = truncateUTF8ToLimit(content, 503, "...")
		}
		fmt.Fprintf(&transcript, "[%s]: %s\n", role, content)
	}

	summaryPrompt := "Compress the following conversation excerpt into a concise factual summary. " +
		"Preserve key decisions, tool results, facts learned, and action items. " +
		"Omit greetings, filler, and redundant exchanges. Output ONLY the summary, no preamble.\n\n" +
		transcript.String()

	summaryReq := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: summaryPrompt},
		},
		MaxTokens:   summaryMaxTokens,
		Temperature: 0.2,
	}

	summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(summaryCtx, summaryReq)
	if err != nil {
		logger.Warn("[Compression] LLM summary failed, skipping compression", "error", err)
		return messages, lastCompressionMsg, result
	}

	summary := ""
	if len(resp.Choices) > 0 {
		summary = strings.TrimSpace(resp.Choices[0].Message.Content)
	}
	if summary == "" {
		logger.Warn("[Compression] LLM returned empty summary, skipping compression")
		return messages, lastCompressionMsg, result
	}

	// Build the replacement message
	summaryMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "[CONVERSATION SUMMARY]\n" + summary,
	}

	// Reconstruct: system + summary + tail
	newMessages := make([]openai.ChatCompletionMessage, 0, 2+compressionKeepTail)
	newMessages = append(newMessages, messages[0]) // system prompt
	newMessages = append(newMessages, summaryMsg)
	newMessages = append(newMessages, messages[tailStart:]...)

	result.Compressed = true
	result.DroppedCount = len(compressible)
	result.SummaryTokens = prompts.CountTokens(summary)

	logger.Info("[Compression] History compressed",
		"dropped_messages", result.DroppedCount,
		"summary_tokens", result.SummaryTokens,
		"new_message_count", len(newMessages))

	return newMessages, len(newMessages), result
}
