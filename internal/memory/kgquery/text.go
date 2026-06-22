package kgquery

import "strings"

// TruncateUTF8Safe shortens text to maxLen runes without splitting multibyte characters.
func TruncateUTF8Safe(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}