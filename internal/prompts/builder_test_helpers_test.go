package prompts

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

func buildSystemPromptInner(promptsDir string, flags *ContextFlags, coreMemory string, logger *slog.Logger) (string, int) {
	prompt, tokens, _ := buildSystemPromptInnerContext(context.Background(), promptsDir, flags, coreMemory, logger)
	return prompt, tokens
}

func removeLineByPrefix(text, prefix string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	skipNext := false
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			out = append(out, line)
			continue
		}
		if inCodeBlock {
			out = append(out, line)
			skipNext = false
			continue
		}
		if skipNext {
			skipNext = false
			if trimmed == "" {
				continue
			}
		}
		if strings.HasPrefix(trimmed, prefix) {
			skipNext = true
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func fallbackSystemPrompt(promptsDir string, flags *ContextFlags, coreMemory string, logger *slog.Logger) (string, int) {
	return fallbackSystemPromptContext(context.Background(), promptsDir, flags, coreMemory, logger)
}

func budgetShed(prompt string, flags *ContextFlags, personalityContent, coreMemory string, now time.Time, logger *slog.Logger) (string, []string) {
	result, shedList, _, _ := budgetShedDetailedContextWithTokens(context.Background(), prompt, flags, -1, logger)
	return result, shedList
}

func budgetShedContext(ctx context.Context, prompt string, flags *ContextFlags, personalityContent, coreMemory string, now time.Time, logger *slog.Logger) (string, []string, error) {
	result, shedList, _, err := budgetShedDetailedContextWithTokens(ctx, prompt, flags, -1, logger)
	return result, shedList, err
}
