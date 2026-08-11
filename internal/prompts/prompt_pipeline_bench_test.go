package prompts

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func BenchmarkBuildSystemPromptDetailedCold(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	flags := &ContextFlags{
		Tier:           "full",
		SystemLanguage: "English",
		Model:          "gpt-4o",
		TokenBudget:    12000,
	}
	promptsDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coldKey := filepath.Join(promptsDir, fmt.Sprintf("cold-%d", i))
		_ = BuildSystemPromptDetailed(context.Background(), coldKey, flags, "", logger)
	}
}

func BenchmarkFitSystemPromptCacheHitPath(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	flags := &ContextFlags{Tier: "full", SystemLanguage: "English", Model: "gpt-4o"}
	base := BuildSystemPromptBaseDetailed(context.Background(), b.TempDir(), flags, "", logger)
	req := PromptFitRequest{Text: base.Text, Tokens: base.Tokens, Model: flags.Model, TokenBudget: 12000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FitSystemPromptToBudget(context.Background(), req, logger)
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
