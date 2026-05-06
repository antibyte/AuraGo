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
	Response string
	ToolCalls int
	Duration time.Duration
}

// ExecuteMinimalLoop runs a single-turn agent execution with tools but without
// personality, memory, RAG, or other heavy agent-loop features. It is used by
// the Looper app and similar structured workflows.
func ExecuteMinimalLoop(
	ctx context.Context,
	client llm.ChatClient,
	model string,
	systemPrompt string,
	userPrompt string,
	tools []openai.Tool,
	dispatchCtx *DispatchContext,
	logger *slog.Logger,
) (MinimalLoopResult, error) {
	start := time.Now()
	result := MinimalLoopResult{}

	if client == nil {
		return result, fmt.Errorf("llm client is required")
	}
	if dispatchCtx == nil {
		return result, fmt.Errorf("dispatch context is required")
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	// Single-turn with up to 10 tool-call follow-ups
	maxToolRounds := 10
	for round := 0; round <= maxToolRounds; round++ {
		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return result, fmt.Errorf("llm call failed: %w", err)
		}
		if len(resp.Choices) == 0 {
			return result, fmt.Errorf("empty response from llm")
		}

		choice := resp.Choices[0]
		msg := choice.Message

		// If no tool calls, we're done
		if len(msg.ToolCalls) == 0 {
			result.Response = security.StripThinkingTags(msg.Content)
			result.Duration = time.Since(start)
			return result, nil
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
	return result, nil
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
	const maxResultLen = 8000
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

	// Look for standalone true/false words
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?'
	})
	for _, w := range words {
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
