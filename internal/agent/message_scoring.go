package agent

import (
	"log/slog"
	"sort"
	"strings"

	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

// MessageImportance represents the importance level of a conversation message.
// Higher values mean the message is more critical and should be preserved.
type MessageImportance int

const (
	ImportanceFiller   MessageImportance = 0 // Redundant confirmations, empty responses
	ImportanceLow      MessageImportance = 1 // Utility outputs (ls, pwd), short acks
	ImportanceMedium   MessageImportance = 2 // Normal queries, successful tool results
	ImportanceHigh     MessageImportance = 3 // Tool errors, user intent, plans/decisions
	ImportanceCritical MessageImportance = 4 // System prompt, fatal errors (never dropped)
)

// scoredMessage attaches a score and metadata to a chat message.
type scoredMessage struct {
	idx     int
	msg     openai.ChatCompletionMessage
	score   MessageImportance
	reason  string
	tokens  int
	isGroup bool // true if this message is part of a tool-call group
}

// ScoreMessage evaluates a single message and returns its importance score
// together with a human-readable reason.
func ScoreMessage(msg openai.ChatCompletionMessage, prevMsg *openai.ChatCompletionMessage) (MessageImportance, string) {
	content := messageText(msg)
	lower := strings.ToLower(content)

	switch msg.Role {
	case openai.ChatMessageRoleSystem:
		return ImportanceCritical, "system"

	case openai.ChatMessageRoleUser:
		if len(content) < 20 && (strings.Contains(lower, "yes") || strings.Contains(lower, "ok")) {
			return ImportanceLow, "short_ack"
		}
		return ImportanceHigh, "user_intent"

	case openai.ChatMessageRoleAssistant:
		if len(msg.ToolCalls) > 0 {
			return ImportanceMedium, "tool_calls"
		}
		if len(content) < 50 {
			return ImportanceLow, "short_ack"
		}
		if containsPlanningMarker(lower) {
			return ImportanceHigh, "plan"
		}
		return ImportanceMedium, "response"

	case openai.ChatMessageRoleTool:
		if isToolError(content) || strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
			return ImportanceHigh, "tool_error"
		}
		if prevMsg != nil && prevMsg.Role == openai.ChatMessageRoleAssistant {
			for _, tc := range prevMsg.ToolCalls {
				if isUtilityTool(tc.Function.Name) && len(content) < 500 {
					return ImportanceLow, "utility_output"
				}
			}
		}
		return ImportanceMedium, "tool_result"
	}

	return ImportanceMedium, "default"
}

// containsPlanningMarker detects if a message contains planning or decision language.
func containsPlanningMarker(s string) bool {
	markers := []string{
		"plan:", "i will", "decision:", "let me", "approach:",
		"strategy:", "next steps", "i'll", "i shall",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// isUtilityTool returns true for tools whose outputs are typically low-value.
func isUtilityTool(name string) bool {
	switch name {
	case "execute_shell", "ssh_exec", "execute_sudo":
		return true
	}
	return false
}

// buildScoredMessages computes importance scores for all messages that are
// candidates for trimming (everything between system prompt and the tail).
//
// systemIdx is always 0. tailStart is the first index that must never be
// trimmed. tokenFn must return the cached/pre-computed token count for a
// message text.
func buildScoredMessages(
	messages []openai.ChatCompletionMessage,
	tailStart int,
	tokenFn func(string) int,
	logger *slog.Logger,
) []scoredMessage {
	if tailStart <= 1 {
		return nil
	}

	scored := make([]scoredMessage, 0, tailStart-1)

	for i := 1; i < tailStart; i++ {
		var prevMsg *openai.ChatCompletionMessage
		if i > 0 {
			prevMsg = &messages[i-1]
		}
		score, reason := ScoreMessage(messages[i], prevMsg)
		scored = append(scored, scoredMessage{
			idx:    i,
			msg:    messages[i],
			score:  score,
			reason: reason,
			tokens: tokenFn(messageText(messages[i])) + 4,
		})
	}

	return scored
}

// buildToolCallGroups identifies contiguous tool-call groups in messages.
// A group starts with an assistant message that has ToolCalls and includes
// all consecutive role=tool messages that follow it.
//
// Returns a map from message index → group leader index.
func buildToolCallGroups(messages []openai.ChatCompletionMessage) map[int]int {
	groups := make(map[int]int)
	for i := 0; i < len(messages); i++ {
		if messages[i].Role == openai.ChatMessageRoleAssistant && len(messages[i].ToolCalls) > 0 {
			leader := i
			groups[i] = leader
			for j := i + 1; j < len(messages); j++ {
				if messages[j].Role == openai.ChatMessageRoleTool || messages[j].ToolCallID != "" {
					groups[j] = leader
				} else {
					break
				}
			}
		}
	}
	return groups
}

// scoreBasedTrimming drops the lowest-importance messages first.
// It returns the trimmed slice, the list of dropped indices, and the new
// estimated token total.
//
// The algorithm:
//  1. Score all candidate messages (between system and tail).
//  2. Identify tool-call groups and compute a group score = max(member scores).
//  3. Sort candidates by (score asc, age asc).
//  4. Drop entire groups or individual messages until under budget.
//  5. Always preserve ImportanceCritical messages.
func scoreBasedTrimming(
	messages []openai.ChatCompletionMessage,
	maxHistoryTokens int,
	totalTokens int,
	tokenFn func(string) int,
	logger *slog.Logger,
) ([]openai.ChatCompletionMessage, []int, int) {
	if len(messages) <= 3 {
		return messages, nil, totalTokens
	}

	const minPreservedMessages = 4
	tailStart := len(messages) - minPreservedMessages
	if tailStart < 2 {
		tailStart = 2
	}

	scored := buildScoredMessages(messages, tailStart, tokenFn, logger)
	if len(scored) == 0 {
		return messages, nil, totalTokens
	}

	groups := buildToolCallGroups(messages)

	// Compute group scores: a group's score is the maximum of its members.
	groupScores := make(map[int]MessageImportance)
	for _, s := range scored {
		if leader, ok := groups[s.idx]; ok {
			if groupScores[leader] < s.score {
				groupScores[leader] = s.score
			}
		}
	}

	// Sort candidates: lowest score first, then oldest first.
	sort.SliceStable(scored, func(a, b int) bool {
		sa, sb := scored[a], scored[b]
		// Use group score for sorting when inside a group.
		scoreA := sa.score
		if leader, ok := groups[sa.idx]; ok {
			scoreA = groupScores[leader]
		}
		scoreB := sb.score
		if leader, ok := groups[sb.idx]; ok {
			scoreB = groupScores[leader]
		}
		if scoreA != scoreB {
			return scoreA < scoreB
		}
		return sa.idx < sb.idx
	})

	removed := make(map[int]bool)
	currentTokens := totalTokens

	for _, item := range scored {
		if currentTokens <= maxHistoryTokens {
			break
		}
		if item.score >= ImportanceCritical {
			break
		}

		// If this message is part of a tool-call group, remove the whole group.
		if leader, ok := groups[item.idx]; ok && !removed[item.idx] {
			// Collect all group members.
			groupIndices := []int{leader}
			for j := leader + 1; j < len(messages); j++ {
				if messages[j].Role == openai.ChatMessageRoleTool || messages[j].ToolCallID != "" {
					groupIndices = append(groupIndices, j)
				} else {
					break
				}
			}
			// Check if any member has already been removed individually.
			allRemovable := true
			for _, gi := range groupIndices {
				if removed[gi] {
					allRemovable = false
					break
				}
			}
			if !allRemovable {
				continue
			}
			for _, gi := range groupIndices {
				removed[gi] = true
				currentTokens -= tokenFn(messageText(messages[gi])) + 4
			}
		} else if !removed[item.idx] {
			removed[item.idx] = true
			currentTokens -= item.tokens
		}
	}

	if len(removed) == 0 {
		return messages, nil, totalTokens
	}

	// Rebuild message list preserving order.
	result := make([]openai.ChatCompletionMessage, 0, len(messages)-len(removed))
	var droppedIndices []int
	for i, m := range messages {
		if removed[i] {
			droppedIndices = append(droppedIndices, i)
		} else {
			result = append(result, m)
		}
	}

	return result, droppedIndices, currentTokens
}

// TrimByImportance is the public entry point for score-based history trimming.
// It drops the lowest-importance messages first, preserving tool-call groups
// and the system prompt. Returns the trimmed slice, dropped indices, and the
// new estimated token count.
func TrimByImportance(
	messages []openai.ChatCompletionMessage,
	maxTokens int,
	model string,
	logger *slog.Logger,
) ([]openai.ChatCompletionMessage, []int, int) {
	tokenFn := func(s string) int {
		return prompts.CountTokensForModel(s, model)
	}
	totalTokens := 0
	for _, m := range messages {
		totalTokens += tokenFn(messageText(m)) + 4
	}
	return scoreBasedTrimming(messages, maxTokens, totalTokens, tokenFn, logger)
}

// logImportanceScores logs what would be dropped without actually dropping.
// Used in the "log_only" beta mode.
func logImportanceScores(
	scored []scoredMessage,
	groups map[int]int,
	logger *slog.Logger,
) {
	if logger == nil {
		return
	}
	for _, s := range scored {
		if leader, ok := groups[s.idx]; ok {
			logger.Debug("[ImportanceScoring] scored message",
				"idx", s.idx,
				"role", s.msg.Role,
				"score", s.score,
				"reason", s.reason,
				"tokens", s.tokens,
				"group_leader", leader,
			)
		} else {
			logger.Debug("[ImportanceScoring] scored message",
				"idx", s.idx,
				"role", s.msg.Role,
				"score", s.score,
				"reason", s.reason,
				"tokens", s.tokens,
			)
		}
	}
}
