package agent

import (
	"context"
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"aurago/internal/memory"
	"aurago/internal/security"
	"github.com/sashabaranov/go-openai"
)

// serializeConsolidationSources bounds complete requests and repeats provenance on every continuation.
func serializeConsolidationSources(messages []memory.ArchivedMessage) []string {
	const limit = 4000 // Includes the complete external-data encoding and every provenance byte.
	var parts []string
	var current strings.Builder
	appendChunks := func(header func(int) string, content string) {
		for offset, part := 0, 1; offset < len(content); part++ {
			prefix, suffix := header(part)+"\n", "\n[end source part]\n"
			remaining := limit - len(security.IsolateExternalData(prefix+suffix))
			end := offset
			for end < len(content) {
				_, width := utf8.DecodeRuneInString(content[end:])
				cost := len(html.EscapeString(content[end : end+width]))
				if cost > remaining {
					break
				}
				remaining -= cost
				end += width
			}
			record := prefix + content[offset:end] + suffix
			if current.Len() > 0 && len(security.IsolateExternalData(current.String()+record)) > limit {
				parts = append(parts, current.String())
				current.Reset()
			}
			current.WriteString(record)
			offset = end
		}
	}
	for _, message := range messages {
		header := fmt.Sprintf("[%s] session=%q role=%q message_id=%d", message.Timestamp, message.SessionID, message.Role, message.ID)
		if len(security.IsolateExternalData(header)) > limit/2 {
			appendChunks(func(part int) string { return fmt.Sprintf("message_id=%d provenance_part=%d", message.ID, part) }, header)
			role := message.Role
			if role != "user" && role != "assistant" {
				role = "unknown"
			}
			header = fmt.Sprintf("message_id=%d source_ref=%d role=%q", message.ID, message.ID, role)
		}
		appendChunks(func(part int) string { return fmt.Sprintf("%s part=%d", header, part) }, message.Content)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func extractionSessionIDs(messages []openai.ChatCompletionMessage, entries []memory.JournalEntry, turns []memory.ActivityTurn) []string {
	seen := map[string]bool{}
	var sessions []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			sessions = append(sessions, id)
		}
	}
	for _, m := range messages {
		add(m.Name)
	}
	for _, entry := range entries {
		add(entry.SessionID)
	}
	for _, turn := range turns {
		add(turn.SessionID)
	}
	return sessions
}

func finishConsolidationError(ctx context.Context, stm *memory.SQLiteMemory, ids []int64, err error) error {
	if ctx.Err() != nil {
		return stm.ReleaseConsolidationClaims(ids)
	}
	if isTransientHelperError(err) {
		return stm.DeferConsolidationClaims(ids)
	}
	return stm.MarkConsolidationFailure(ids, "extraction_invalid")
}
