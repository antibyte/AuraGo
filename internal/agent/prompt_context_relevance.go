package agent

import (
	"strings"
	"unicode"
)

// shouldInjectRecentMemoryContext decides whether broad recent activity and
// episodic reminders are helpful for the current user message. Short greetings
// should start clean instead of dragging stale follow-ups into the prompt.
func shouldInjectRecentMemoryContext(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if lower == "" {
		return false
	}
	lowSignal := map[string]struct{}{
		"hi": {}, "hello": {}, "hey": {}, "hallo": {}, "moin": {}, "servus": {}, "yo": {}, "ok": {}, "okay": {},
	}
	if _, ok := lowSignal[lower]; ok {
		return false
	}
	statusKeywords := []string{
		"was neues", "gibts", "gibt es", "status", "offen", "todo", "aufgabe", "erinner", "follow",
		"pending", "open", "what's new", "whats new", "anything new", "remind",
	}
	for _, keyword := range statusKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	for _, term := range strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(term)) >= 4 {
			return true
		}
	}
	return false
}
