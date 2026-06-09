package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ShouldAppendHistoryMessage reports whether a SQLite insert succeeded with a
// valid row ID. HistoryManager must only mirror messages that were persisted to
// short-term memory so history.json and SQLite stay aligned.
func ShouldAppendHistoryMessage(sqliteID int64, insertErr error) bool {
	return insertErr == nil && sqliteID > 0
}

// NativeToolCallHistoryMessage builds the assistant message that pairs with a
// native tool result in history.json. Batched native tool calls reuse this so
// STM and HistoryManager stay aligned.
func NativeToolCallHistoryMessage(tc ToolCall, histContent string) openai.ChatCompletionMessage {
	if tc.NativeCallID == "" {
		return openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: histContent,
		}
	}

	args := strings.TrimSpace(tc.NativeArgsRaw)
	if args == "" {
		args = strings.TrimSpace(tc.RawJSON)
	}
	if args == "" && tc.Params != nil {
		if encoded, err := json.Marshal(tc.Params); err == nil {
			args = string(encoded)
		}
	}
	if args == "" {
		args = fmt.Sprintf(`{"action":"%s"}`, tc.Action)
	}

	name := strings.TrimSpace(tc.Action)
	if name == "" {
		name = "unknown"
	}

	return openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{{
			ID:   tc.NativeCallID,
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		}},
	}
}