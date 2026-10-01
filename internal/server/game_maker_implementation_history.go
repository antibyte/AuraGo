package server

import (
	"strings"

	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// Server-written assistant notes that carry earlier reasoning into the
// tool-free phase. They are never source; a response repeating one is rejected.
const (
	gameStarterHistoryReasoningNote     = "Earlier implementation reasoning is context only; follow the current source-generation request."
	gameStarterRetryReasoningNote       = "Earlier implementation reasoning is context only; follow the source-generation request."
	gameStarterInterruptedReasoningNote = "Generation was interrupted; this retained reasoning is context only."
)

// gameStarterEchoesHistoryNote reports a response that copied one of the notes.
func gameStarterEchoesHistoryNote(response string) bool {
	for _, note := range []string{gameStarterHistoryReasoningNote, gameStarterRetryReasoningNote, gameStarterInterruptedReasoningNote} {
		if strings.Contains(response, note) {
			return true
		}
	}
	return false
}

// Project the private archive into this tool-free phase. Old calls, results and
// rejected/partial source are not examples for the next answer. Keep available
// reasoning and, on retry, the current snapshot exactly once. The checkpoint
// closure still owns the complete archive, including every earlier tool group.
func gameStarterRequestHistory(history []openai.ChatCompletionMessage, sourcePrompt string) []openai.ChatCompletionMessage {
	sourceStart := -1
	if sourcePrompt != "" {
		sourcePrompt = security.Scrub(sourcePrompt)
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == openai.ChatMessageRoleUser && history[i].Content == sourcePrompt {
				sourceStart = i
				break
			}
		}
	}
	var view []openai.ChatCompletionMessage
	for i, message := range history {
		if message.Role == openai.ChatMessageRoleAssistant && message.ReasoningContent != "" {
			view = append(view, openai.ChatCompletionMessage{
				Role:             openai.ChatMessageRoleAssistant,
				Content:          gameStarterHistoryReasoningNote,
				ReasoningContent: message.ReasoningContent,
			})
		} else if sourceStart >= 0 && i >= sourceStart && message.Role == openai.ChatMessageRoleUser {
			view = append(view, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: message.Content})
		}
	}
	if sourcePrompt != "" && sourceStart < 0 {
		view = append(view, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: sourcePrompt})
	}
	return view
}
