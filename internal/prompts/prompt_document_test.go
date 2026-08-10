package prompts

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBuildSystemPromptDetailedReturnsLedgerAndRevision(t *testing.T) {
	result := BuildSystemPromptDetailed(context.Background(), t.TempDir(), &ContextFlags{
		Tier:           "full",
		SystemLanguage: "English",
		Model:          "gpt-4o",
		TokenBudget:    5000,
	}, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result.Text == "" || result.Tokens <= 0 || result.Revision == "" {
		t.Fatalf("incomplete detailed result: %+v", result)
	}
	if result.Document.Revision != result.Revision || result.Document.TotalTokens != result.Tokens {
		t.Fatalf("document metadata drift: result=%+v document=%+v", result, result.Document)
	}
	if len(result.Document.Sections) == 0 {
		t.Fatal("expected typed prompt sections")
	}
	for _, section := range result.Document.Sections {
		if section.Text != "" && section.Tokens <= 0 {
			t.Fatalf("section %q has no precomputed tokens", section.ID)
		}
	}
}

func TestPromptDocumentSectionTokenLedgerKeepsDuplicateOrder(t *testing.T) {
	document := PromptDocument{Sections: []PromptSection{
		{ID: "# TOOL GUIDES", Tokens: 11},
		{ID: "# REQUIRED", Tokens: 23},
		{ID: "# TOOL GUIDES", Tokens: 37},
	}}
	ledger := document.sectionTokenLedger()
	if got := takeSectionTokens(ledger, "# TOOL GUIDES"); got != 11 {
		t.Fatalf("first duplicate tokens = %d, want 11", got)
	}
	if got := takeSectionTokens(ledger, "# TOOL GUIDES"); got != 37 {
		t.Fatalf("second duplicate tokens = %d, want 37", got)
	}
	if got := takeSectionTokens(ledger, "# TOOL GUIDES"); got != 0 {
		t.Fatalf("exhausted duplicate tokens = %d, want 0", got)
	}
}

func TestBudgetShedUsesAtMostTwoFullDocumentTokenizations(t *testing.T) {
	count := 0
	budgetShedFullTokenizeHook = func() { count++ }
	defer func() { budgetShedFullTokenizeHook = nil }()
	prompt := "# TOOL GUIDES\n" + strings.Repeat("guide content ", 400) +
		"\n# RELEVANT KNOWLEDGE\n" + strings.Repeat("knowledge ", 300) +
		"\n# REQUIRED CORE\nKeep this instruction."
	flags := &ContextFlags{TokenBudget: 80, Model: "gpt-4o"}
	result, _, err := budgetShedContext(context.Background(), prompt, flags, "", "", time.Now(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if count > 2 {
		t.Fatalf("full document tokenizations = %d, want at most 2", count)
	}
	if CountTokensForModel(result, flags.Model) > flags.TokenBudget {
		t.Fatalf("shed result exceeds budget")
	}
}

func TestBudgetShedUsesTypedGroupsAndPreservesRequiredSections(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	prompt := "# REQUIRED CORE\nKeep this instruction.\n\n# TOOL GUIDES\n" +
		strings.Repeat("optional guide ", 200) +
		"\n## Nested Guide\n" + strings.Repeat("nested optional ", 100) +
		"\n# FINAL REQUIRED\nKeep this too."
	flags := &ContextFlags{TokenBudget: 40, Model: "gpt-4o"}
	result, shed, err := budgetShedContext(context.Background(), prompt, flags, "", "", time.Now(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "# REQUIRED CORE") || !strings.Contains(result, "# FINAL REQUIRED") {
		t.Fatalf("required sections were removed:\n%s", result)
	}
	if strings.Contains(result, "# TOOL GUIDES") || strings.Contains(result, "## Nested Guide") {
		t.Fatalf("optional group was not removed atomically:\n%s", result)
	}
	if !containsString(shed, "# TOOL GUIDES") {
		t.Fatalf("shed = %v, want tool-guide group", shed)
	}
}

func TestBuildSystemPromptDetailedReportsMandatoryOverflow(t *testing.T) {
	result := BuildSystemPromptDetailed(context.Background(), t.TempDir(), &ContextFlags{
		Tier: "minimal", SystemLanguage: "English", Model: "gpt-4o", TokenBudget: 1,
	}, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result.BudgetExceeded == nil {
		t.Fatal("expected mandatory prompt budget error")
	}
	if strings.Contains(result.Text, "[BUDGET TRUNCATED]") {
		t.Fatal("mandatory prompt was hard-truncated")
	}
}

func TestHardTruncatePreservesUTF8AndTokenLimit(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "cjk", text: strings.Repeat("你好世界こんにちは世界", 200)},
		{name: "emoji", text: strings.Repeat("🧑🏽‍💻🚀✨", 200)},
		{name: "combining", text: strings.Repeat("Cafe\u0301 A\u030angstro\u0308m ", 200)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const budget = 64
			result, err := hardTruncateToBudgetContext(context.Background(), tt.text, budget, "gpt-4o")
			if err != nil {
				t.Fatal(err)
			}
			if !utf8.ValidString(result) {
				t.Fatalf("result is not valid UTF-8: %q", result)
			}
			if got := CountTokensForModel(result, "gpt-4o"); got > budget {
				t.Fatalf("tokens = %d, want <= %d", got, budget)
			}
		})
	}
}

func TestTranslateTokenBudgetPreservesCrossModelCapacity(t *testing.T) {
	for _, tt := range []struct {
		from string
		to   string
	}{
		{from: "claude-3-7-sonnet", to: "gpt-4o"},
		{from: "gpt-4o", to: "claude-3-7-sonnet"},
		{from: "gemini-2.5-pro", to: "deepseek-r1"},
	} {
		for budget := 1; budget <= 8192; budget += 97 {
			translated := TranslateTokenBudget(budget, tt.from, tt.to)
			baseCapacity := TranslateTokenBudget(translated, tt.to, "")
			if fromCapacity := TranslateTokenBudget(budget, tt.from, ""); baseCapacity > fromCapacity {
				t.Fatalf("%s -> %s budget %d expanded base capacity: %d > %d", tt.from, tt.to, budget, baseCapacity, fromCapacity)
			}
		}
	}
}
