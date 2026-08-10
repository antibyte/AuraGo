package prompts

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func BenchmarkBuildSystemPromptDetailedRepeated(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	flags := &ContextFlags{
		Tier:           "full",
		SystemLanguage: "English",
		Model:          "gpt-4o",
		TokenBudget:    12000,
	}
	promptsDir := b.TempDir()
	_ = BuildSystemPromptDetailed(context.Background(), promptsDir, flags, "", logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildSystemPromptDetailed(context.Background(), promptsDir, flags, "", logger)
	}
}

func BenchmarkPromptBudgetShedding(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	prompt := "# REQUIRED CORE\nKeep this instruction.\n\n" +
		"# TOOL GUIDES\n" + strings.Repeat("guide content ", 1200) +
		"\n# RELEVANT KNOWLEDGE\n" + strings.Repeat("knowledge ", 900)
	flags := &ContextFlags{TokenBudget: 256, Model: "gpt-4o"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := budgetShedContext(context.Background(), prompt, flags, "", "", time.Time{}, logger)
		if err != nil {
			b.Fatal(err)
		}
	}
}
