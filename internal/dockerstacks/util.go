package dockerstacks

import (
	"strings"
	"time"
)

// cleanupTimeout bounds the removal of a helper container, which runs on a
// context of its own because the caller's may already be cancelled.
const cleanupTimeout = 30 * time.Second

// firstNonBlankLine returns the first non-blank line of s, trimmed.
func firstNonBlankLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
