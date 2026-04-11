package agent

import (
	"aurago/internal/config"
	"fmt"
	"log/slog"

	"github.com/sashabaranov/go-openai"
)

// RecoverySessionState consolidates all per-session recovery counters and the
// ConsolidatedRecoveryHandler into a single object.  It replaces the 8+ loose
// counter variables that were previously scattered through agent_loop.go.
//
// Usage:
//
//	rss := NewRecoverySessionState(logger, broker, cfg)
//	// … inside the agent loop …
//	result := rss.HandleRecovery(tc, content, parsedToolResp, …)
//	if result.ShouldRecover && result.ContinueLoop {
//	    rss.PersistRecovery(…)
//	    continue
//	}
type RecoverySessionState struct {
	handler                *ConsolidatedRecoveryHandler
	logger                 *slog.Logger
	broker                 FeedbackBroker
	cfg                    *config.Config
	announcementMaxRetries int // from cfg.Agent.AnnouncementDetector.MaxRetries
}

// NewRecoverySessionState creates a new recovery session state for one conversation.
func NewRecoverySessionState(
	logger *slog.Logger,
	broker FeedbackBroker,
	cfg *config.Config,
) *RecoverySessionState {
	rss := &RecoverySessionState{
		logger: logger,
		broker: broker,
		cfg:    cfg,
	}
	if cfg != nil && cfg.Agent.AnnouncementDetector.MaxRetries > 0 {
		rss.announcementMaxRetries = cfg.Agent.AnnouncementDetector.MaxRetries
	} else {
		rss.announcementMaxRetries = 2 // default
	}
	rss.handler = newConsolidatedRecoveryHandler(cfg, broker, logger)
	return rss
}

// HandleRecovery classifies the current LLM response and decides whether a
// recovery action is needed.  It delegates to ConsolidatedRecoveryHandler but
// also respects the announcement-specific max-retries from config.
func (rss *RecoverySessionState) HandleRecovery(
	tc ToolCall,
	content string,
	parsedToolResp ParsedToolResponse,
	useNativeFunctions bool,
	announcementContent string,
	useNativePath bool,
	lastResponseWasTool bool,
	lastUserMsg string,
	recentTools []string,
) RecoveryResult {
	return rss.handler.HandleRecovery(
		tc, content, parsedToolResp, useNativeFunctions,
		announcementContent, useNativePath, lastResponseWasTool,
		lastUserMsg, recentTools,
	)
}

// persistRecoveryMessages is the shared helper that persists recovery messages
// to SQLite, the history manager, and the LLM request context.
// It returns the messages that should be appended to req.Messages.
//
// This function centralizes the 4-step persist pattern used by every recovery loop:
//  1. Persist assistant message to SQLite
//  2. Add assistant message to history manager
//  3. Persist feedback message to SQLite
//  4. Add feedback message to history manager
//
// The caller is responsible for appending the returned messages to req.Messages.
type PersistRecoveryParams struct {
	SessionID        string
	AssistantContent string // Content to persist as assistant message (empty = skip)
	FeedbackMsg      string // Feedback message to send and persist
	BrokerEventType  string // Event type for broker.Send (default: "error_recovery")
	// Optional overrides
	SkipAssistantPersist bool // Set true when assistant content was already persisted
}

// PersistRecoveryMessages persists recovery messages to SQLite, history, and returns
// the messages to append to req.Messages.
func (rss *RecoverySessionState) PersistRecoveryMessages(
	params PersistRecoveryParams,
	shortTermMem interface {
		InsertMessage(sessionID string, role string, content string, isSystem bool, isHidden bool) (int64, error)
	},
	historyManager interface {
		Add(role string, content string, id int64, isSystem bool, isHidden bool) error
	},
) []openai.ChatCompletionMessage {
	var msgs []openai.ChatCompletionMessage

	// Send broker event
	eventType := params.BrokerEventType
	if eventType == "" {
		eventType = "error_recovery"
	}
	rss.broker.Send(eventType, params.FeedbackMsg)

	// Persist assistant message (if provided and not already persisted)
	if params.AssistantContent != "" && !params.SkipAssistantPersist {
		id, err := shortTermMem.InsertMessage(params.SessionID, openai.ChatMessageRoleAssistant, params.AssistantContent, false, true)
		if err != nil {
			rss.logger.Error("Failed to persist assistant message to SQLite", "error", err)
		}
		if params.SessionID == "default" {
			historyManager.Add(openai.ChatMessageRoleAssistant, params.AssistantContent, id, false, true)
		}
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: params.AssistantContent,
		})
	}

	// Persist feedback message
	id, err := shortTermMem.InsertMessage(params.SessionID, openai.ChatMessageRoleUser, params.FeedbackMsg, false, true)
	if err != nil {
		rss.logger.Error("Failed to persist feedback message to SQLite", "error", err)
	}
	if params.SessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleUser, params.FeedbackMsg, id, false, true)
	}
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: params.FeedbackMsg,
	})

	return msgs
}

// FormatRawCodeFeedback returns the feedback message for raw code detection.
func FormatRawCodeFeedback() string {
	return "ERROR: You sent raw Python code instead of a JSON tool call. My supervisor only understands JSON tool calls. Please wrap your code in a valid JSON object: {\"action\": \"save_tool\", \"name\": \"script.py\", \"description\": \"...\", \"code\": \"<your python code with \\n escaped>\"}."
}

// FormatInvalidNativeToolFeedback returns the feedback message for invalid native tool calls.
func FormatInvalidNativeToolFeedback(toolName string) string {
	if toolName == "" {
		toolName = "the requested tool"
	}
	return fmt.Sprintf(
		"ERROR: Your last native function call for %q had invalid function arguments JSON and was discarded. Emit the function call again with valid JSON arguments only. Do not include source code, XML/HTML, or prose inside the function name or outside the JSON arguments.",
		toolName,
	)
}

// FormatIncompleteToolCallFeedback returns the feedback message for incomplete tool call tags.
func FormatIncompleteToolCallFeedback(useNativeFunctions bool, retryCount int) string {
	if useNativeFunctions {
		return "ERROR: You emitted a bare  or <minimax:tool_call> tag but did not produce an actual tool call. You MUST use the native function-calling mechanism to invoke tools. Do NOT output any XML tags in text — use the structured function call API instead."
	}
	if retryCount >= 2 {
		return "CRITICAL ERROR: You sent '' as raw text again. This is not a valid tool call format. Do NOT output any XML tags at all. Output a raw JSON object starting with '{'."
	}
	return "ERROR: You emitted a bare  tag but did not include the JSON body. Do NOT output XML tags. Output ONLY the raw JSON tool call object - no XML tags, no explanation, no preamble."
}

// FormatOrphanedBracketTagFeedback returns the feedback message for orphaned [TOOL_CALL] tags.
func FormatOrphanedBracketTagFeedback(useNativeFunctions bool) string {
	if useNativeFunctions {
		return "ERROR: Your response contained the literal text \"[TOOL_CALL]\" but no actual function call was made. You MUST use the native function-calling mechanism to invoke a tool. Do NOT write [TOOL_CALL] as text — call the function directly using the tool call interface."
	}
	return "ERROR: Your response contained the literal text \"[TOOL_CALL]\" but no valid tool call JSON. Do NOT write [TOOL_CALL] as text. Your ENTIRE response must be ONLY the raw JSON tool call — no explanation, no tags. Output the JSON tool call NOW."
}

// FormatBareXMLInNativeModeFeedback returns the feedback message for bare XML in native mode.
func FormatBareXMLInNativeModeFeedback() string {
	return "ERROR: Your response contained a literal  XML tag but no actual function call was made. You MUST use the native function-calling mechanism — do not write XML tags. Call the function directly using the tool call interface now."
}

// FormatMissedToolInFenceFeedback returns the feedback message for tool calls wrapped in markdown fences.
func FormatMissedToolInFenceFeedback() string {
	return "ERROR: Your response contained explanation text and/or markdown fences (```json). Tool calls MUST be a raw JSON object ONLY - no explanation before or after, no markdown, no fences. Output ONLY the JSON object, starting with { and ending with }. Example: {\"action\": \"<tool_name>\", \"<param>\": \"<value>\"}"
}
