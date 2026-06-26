package agent

import (
	"strings"

	"aurago/internal/security"
)

func safePromptMetadataText(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	text = strings.NewReplacer("`", "'", "<", "[", ">", "]").Replace(text)
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func isolateAgentPromptExternalData(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return security.IsolateExternalData(escapePromptBoundaryLines(text))
}

func escapePromptBoundaryLines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") {
			prefixLen := len(line) - len(trimmed)
			lines[i] = line[:prefixLen] + "\\" + trimmed
		}
	}
	return strings.Join(lines, "\n")
}
