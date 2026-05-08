package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// MinimalLoopResult is the outcome of a single minimal agent turn.
type MinimalLoopResult struct {
	Response  string
	ToolCalls int
	Duration  time.Duration
}

// MinimalLoopOptions controls optional behaviour of ExecuteMinimalLoop.
type MinimalLoopOptions struct {
	// MaxToolRounds caps the number of tool-call follow-up rounds.
	// 0 means "no tools at all" (the request is sent without tool schemas).
	// -1 or unset defaults to 3.
	MaxToolRounds int
}

const defaultMaxToolRounds = 3

// ExecuteMinimalLoop runs a single-turn agent execution with tools but without
// personality, memory, RAG, or other heavy agent-loop features. It is used by
// the Looper app and similar structured workflows.
//
// If history is non-empty the conversation is continued; the systemPrompt is
// only injected when history is empty so the caller can maintain a multi-turn
// thread across loop steps.
//
// opts is optional; a sensible default is used when nil.
func ExecuteMinimalLoop(
	ctx context.Context,
	client llm.ChatClient,
	model string,
	systemPrompt string,
	userPrompt string,
	tools []openai.Tool,
	dispatchCtx *DispatchContext,
	history []openai.ChatCompletionMessage,
	logger *slog.Logger,
	opts *MinimalLoopOptions,
) (MinimalLoopResult, []openai.ChatCompletionMessage, error) {
	start := time.Now()
	result := MinimalLoopResult{}

	if client == nil {
		return result, history, fmt.Errorf("llm client is required")
	}
	if dispatchCtx == nil {
		return result, history, fmt.Errorf("dispatch context is required")
	}

	maxRounds := defaultMaxToolRounds
	noTools := false
	if opts != nil {
		if opts.MaxToolRounds > 0 {
			maxRounds = opts.MaxToolRounds
		} else if opts.MaxToolRounds == 0 {
			noTools = true
		}
		// -1 or unset → keep defaultMaxToolRounds
	}

	// When no tools are needed, strip tool schemas to save thousands of tokens.
	reqTools := tools
	if noTools || len(tools) == 0 {
		reqTools = nil
		maxRounds = 0
	}

	messages := make([]openai.ChatCompletionMessage, 0, len(history)+2)
	if len(history) == 0 && systemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: systemPrompt})
	} else {
		messages = append(messages, history...)
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: userPrompt})

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    reqTools,
	}

	for round := 0; round <= maxRounds; round++ {
		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return result, req.Messages, fmt.Errorf("llm call failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			return result, req.Messages, fmt.Errorf("empty response from llm")
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// If no tool calls, we're done
		if len(msg.ToolCalls) == 0 {
			result.Response = security.StripThinkingTags(msg.Content)
			result.Duration = time.Since(start)
			return result, req.Messages, nil
		}

		// Execute tool calls
		result.ToolCalls += len(msg.ToolCalls)
		req.Messages = append(req.Messages, msg)

		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			toolResult := executeMinimalToolCall(ctx, tc, dispatchCtx, userPrompt, logger)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    toolResult,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max rounds exceeded — return last assistant content if any
	lastMsg := req.Messages[len(req.Messages)-1]
	if lastMsg.Role == openai.ChatMessageRoleAssistant {
		result.Response = security.StripThinkingTags(lastMsg.Content)
	} else {
		result.Response = ""
	}
	result.Duration = time.Since(start)
	return result, req.Messages, nil
}

func executeMinimalToolCall(
	ctx context.Context,
	tc openai.ToolCall,
	dispatchCtx *DispatchContext,
	userContext string,
	logger *slog.Logger,
) string {
	if logger == nil {
		logger = slog.Default()
	}

	name := tc.Function.Name
	args := tc.Function.Arguments

	logger.Debug("[MinimalLoop] executing tool", "name", name)

	// Build a minimal tool call representation matching the agent's ToolCall struct
	toolCall := ToolCall{
		Action:        name,
		NativeCallID:  tc.ID,
		NativeArgsRaw: args,
		IsTool:        true,
	}

	// Parse arguments into Params so builtin dispatchers can read them
	if args != "" {
		var rawMap map[string]interface{}
		if err := json.Unmarshal([]byte(args), &rawMap); err == nil {
			toolCall.Params = rawMap
		}
	}

	// Use the existing dispatch infrastructure
	result := DispatchToolCall(ctx, &toolCall, dispatchCtx, userContext)

	// Truncate very large results to avoid context explosion
	const maxResultLen = 4000
	if len(result) > maxResultLen {
		result = result[:maxResultLen] + fmt.Sprintf("\n... (%d more chars)", len(result)-maxResultLen)
	}

	return result
}

// GetBuiltinToolSchemas returns the cached builtin tool schemas for the given config.
func GetBuiltinToolSchemas(cfg *config.Config) []openai.Tool {
	policy := BuildToolingPolicy(cfg, "")
	ff := buildToolFeatureFlags(RunConfig{Config: cfg}, policy)
	return builtinToolSchemasCached(ff)
}

// GetLooperToolSchemas returns a minimal base set for the Looper plus
// discover_tools/invoke_tool so the agent can dynamically access all
// enabled tools without sending 100+ schemas upfront.
func GetLooperToolSchemas(cfg *config.Config) []openai.Tool {
	all := GetBuiltinToolSchemas(cfg)

	looperBase := map[string]bool{
		"filesystem": true, "file_editor": true, "execute_shell": true,
		"execute_python": true, "docker": true, "api_request": true,
		"smart_file_read": true, "file_reader_advanced": true,
		"web_scraper": true, "manage_memory": true, "query_memory": true,
		"virtual_desktop": true,
		"discover_tools": true, "invoke_tool": true, "execute_skill": true,
		"run_tool": true,
	}

	var filtered []openai.Tool
	for _, t := range all {
		if t.Function != nil && looperBase[t.Function.Name] {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return all
	}

	SetDiscoverToolsState("looper", all, filtered, cfg.Directories.PromptsDir)

	return filtered
}

// MinimalSystemPromptBuilder creates a minimal system prompt for the Looper.
func MinimalSystemPromptBuilder(availableTools []string) string {
	var b strings.Builder
	b.WriteString("You are a precise execution agent. Your job is to follow the instruction exactly and produce the requested output.\n")
	b.WriteString("You have access to AuraGo tools. Use them when needed to accomplish the task.\n")
	b.WriteString("Be concise and direct. Do not add unnecessary commentary.\n")
	if len(availableTools) > 0 {
		b.WriteString("\nAvailable tools: ")
		b.WriteString(strings.Join(availableTools, ", "))
		b.WriteString("\n")
	}
	return b.String()
}

// TrimHistory keeps the history within a manageable token budget by truncating
// long assistant and tool messages. The first message (system prompt) is always
// preserved. maxChars is the total character budget for all non-system messages.
func TrimHistory(history []openai.ChatCompletionMessage, maxChars int) []openai.ChatCompletionMessage {
	if len(history) <= 1 {
		return history
	}

	// Calculate total chars
	total := 0
	for _, m := range history {
		total += len(m.Content)
	}
	if total <= maxChars {
		return history
	}

	// Truncate from the oldest messages first, keeping system prompt and last N messages.
	// We keep the first message (system) and the last 6 messages (recent context).
	const keepRecent = 6
	trimmed := make([]openai.ChatCompletionMessage, len(history))
	copy(trimmed, history)

	// Start trimming from index 1 (skip system prompt), up to len-keepRecent
	for i := 1; i < len(trimmed)-keepRecent && total > maxChars; i++ {
		msgLen := len(trimmed[i].Content)
		if msgLen > 500 {
			excess := msgLen - 500
			trimmed[i].Content = trimmed[i].Content[:500] + fmt.Sprintf("\n... (%d more chars)", excess)
			total -= excess
		}
	}

	// If still too long, aggressively truncate middle messages
	for i := 1; i < len(trimmed)-keepRecent && total > maxChars; i++ {
		if len(trimmed[i].Content) > 200 {
			excess := len(trimmed[i].Content) - 200
			trimmed[i].Content = trimmed[i].Content[:200] + "..."
			total -= excess
		}
	}

	return trimmed
}

// ParseExitBoolean parses an LLM response into a boolean for loop exit conditions.
func ParseExitBoolean(raw string) bool {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return false
	}

	// Direct boolean strings
	if s == "true" || s == "yes" || s == "ja" || s == "1" {
		return true
	}
	if s == "false" || s == "no" || s == "nein" || s == "0" {
		return false
	}

	// JSON boolean extraction
	if idx := strings.Index(s, `"true"`); idx != -1 {
		return true
	}
	if idx := strings.Index(s, `"false"`); idx != -1 {
		return false
	}

	// Look for standalone true/false words — scan backwards so the last
	// boolean in the response takes precedence.
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?'
	})
	for i := len(words) - 1; i >= 0; i-- {
		w := words[i]
		if w == "true" {
			return true
		}
		if w == "false" {
			return false
		}
	}

	// Default: continue looping
	return false
}
