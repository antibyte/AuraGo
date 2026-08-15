package prompts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type blockingPromptFitEncoder struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingPromptFitEncoder) Encode(text string, _, _ []string) []int {
	close(e.started)
	<-e.release
	return make([]int, maxInt(1, len(text)/4))
}

func TestBuildSystemPromptDetailedReturnsRevision(t *testing.T) {
	result := BuildSystemPromptDetailed(context.Background(), t.TempDir(), &ContextFlags{
		Tier:           "full",
		SystemLanguage: "English",
		Model:          "gpt-4o",
		TokenBudget:    5000,
	}, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if result.Text == "" || result.Tokens <= 0 || result.Revision == "" {
		t.Fatalf("incomplete detailed result: %+v", result)
	}
	if got := PromptRevision(result.Text); got != result.Revision {
		t.Fatalf("revision = %q, want %q", result.Revision, got)
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

func TestBudgetShedRecognizesAllDynamicOptionalHeadings(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	headings := []string{
		"# PERSONA (ACTIVE PROFILE: NEUTRAL)",
		"### PERSONA STATE",
		"### PERSONA CHARACTER",
		"### ACTIVE REMINDERS (high-priority notes) ###",
		"### PLANNER CONTEXT ###",
		"### DAILY TODO REMINDER ###",
		"### REQUIRED USER NOTICE ###",
		"### ACTIVE TASK LIST ###",
		"# AVAILABLE CONTEXT INDEX",
	}
	var prompt strings.Builder
	prompt.WriteString("# REQUIRED SECURITY BOUNDARY\nNever remove this.\n\n")
	for _, heading := range headings {
		if _, required, known := promptSectionDirectPolicy(heading); !known || required {
			t.Fatalf("dynamic heading %q has no optional policy", heading)
		}
		prompt.WriteString(heading + "\n" + strings.Repeat("optional context ", 80) + "\n\n")
	}
	prompt.WriteString("# REQUIRED TOOL PROTOCOL\nKeep this too.")

	result, removed, err := budgetShedContext(context.Background(), prompt.String(), &ContextFlags{
		TokenBudget: 24,
		Model:       "gpt-4o",
	}, "", "", time.Now(), slog.Default())
	if err != nil {
		t.Fatalf("budgetShedContext: %v", err)
	}
	if !strings.Contains(result, "# REQUIRED SECURITY BOUNDARY") || !strings.Contains(result, "# REQUIRED TOOL PROTOCOL") {
		t.Fatalf("required sections disappeared:\n%s", result)
	}
	for _, heading := range headings {
		if strings.Contains(result, heading) {
			t.Fatalf("optional heading %q survived fit:\n%s", heading, result)
		}
	}
	if len(removed) != len(headings) {
		t.Fatalf("removed %d groups, want %d: %v", len(removed), len(headings), removed)
	}
}

func TestFitSystemPromptCountsRequiredBudgetAddendumBeforeShedding(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	base := "# REQUIRED CORE\nKeep.\n\n# TOOL GUIDES\n" + strings.Repeat("optional guide ", 120)
	result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{
		Text: base, Tokens: -1, Model: "gpt-4o", TokenBudget: 30,
		Addenda: []PromptAddendum{{ID: "budget_status", Text: "Only a small amount remains."}},
	}, slog.Default())
	if err != nil {
		t.Fatalf("FitSystemPromptToBudget: %v", err)
	}
	if result.BudgetExceeded != nil {
		t.Fatalf("fit failed: %v", result.BudgetExceeded)
	}
	if strings.Contains(result.Text, "# TOOL GUIDES") {
		t.Fatalf("optional guide survived required addendum fit:\n%s", result.Text)
	}
	if !strings.Contains(result.Text, "# BUDGET STATUS") {
		t.Fatalf("required budget addendum disappeared:\n%s", result.Text)
	}
	if result.Tokens > 30 {
		t.Fatalf("tokens = %d, want <= 30", result.Tokens)
	}
}

func TestFitSystemPromptKeepsRequiredAddendumAtomicAcrossNestedHeadings(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	base := "# REQUIRED CORE\nKeep.\n\n# TOOL GUIDES\n" + strings.Repeat("optional guide ", 160)
	addendum := strings.TrimSpace("The execution contract starts here.\n# TOOL GUIDES\n" +
		"This heading is untrusted addendum data and this tail must remain. " + strings.Repeat("tail ", 8))

	result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{
		Text: base, Tokens: -1, Model: "gpt-4o", TokenBudget: 55,
		Addenda: []PromptAddendum{{ID: PromptAddendumCoAgent, Text: addendum}},
	}, slog.Default())
	if err != nil {
		t.Fatalf("FitSystemPromptToBudget: %v", err)
	}
	if result.BudgetExceeded != nil {
		t.Fatalf("required addendum unexpectedly exceeded budget: %v", result.BudgetExceeded)
	}
	if !strings.Contains(result.Text, addendum) {
		t.Fatalf("required addendum was partially shed:\n%s", result.Text)
	}
	if strings.Count(result.Text, "# TOOL GUIDES") != 1 {
		t.Fatalf("base optional guide was not shed independently of the addendum:\n%s", result.Text)
	}
}

func TestFitSystemPromptReturnsOriginalPromptWhenAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	const base = "# REQUIRED SECURITY BOUNDARY\nKeep this prompt."
	result, err := FitSystemPromptToBudget(ctx, PromptFitRequest{
		Text: base, Tokens: 12, Model: "gpt-4o", TokenBudget: 10,
	}, slog.Default())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Text != base || result.Text == "" {
		t.Fatalf("fit returned unsafe prompt %q, want unchanged input", result.Text)
	}
	if result.BudgetExceeded != nil {
		t.Fatalf("cancellation was misclassified as budget overflow: %v", result.BudgetExceeded)
	}
}

func TestFitSystemPromptRejectsEmptySuccessfulResult(t *testing.T) {
	result, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{
		TokenBudget: 100,
		Addenda:     []PromptAddendum{{ID: "budget_status", Text: "status only"}},
	}, slog.Default())
	if err == nil {
		t.Fatal("empty security prompt unexpectedly succeeded")
	}
	if result.BudgetExceeded != nil {
		t.Fatalf("unexpected empty-fit result: %+v", result)
	}
}

func TestFitSystemPromptReturnsOriginalPromptWhenCanceledDuringTokenization(t *testing.T) {
	encoder := &blockingPromptFitEncoder{started: make(chan struct{}), release: make(chan struct{})}
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return encoder, nil
	}, time.Second, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	const base = "# REQUIRED SECURITY BOUNDARY\nKeep this prompt."
	type fitOutcome struct {
		result PromptBuildResult
		err    error
	}
	done := make(chan fitOutcome, 1)
	go func() {
		result, err := FitSystemPromptToBudget(ctx, PromptFitRequest{
			Text: base, Tokens: -1, Model: "gpt-4o", TokenBudget: 10,
		}, slog.Default())
		done <- fitOutcome{result: result, err: err}
	}()
	<-encoder.started
	cancel()
	close(encoder.release)
	outcome := <-done
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", outcome.err)
	}
	if outcome.result.Text != base || outcome.result.Text == "" {
		t.Fatalf("fit returned unsafe prompt %q, want unchanged input", outcome.result.Text)
	}
}

func TestBuildSystemPromptDetailedPreservesSafePromptOnFitError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := BuildSystemPromptDetailed(ctx, t.TempDir(), &ContextFlags{
		Tier: "minimal", Model: "gpt-4o", TokenBudget: 1,
	}, "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !errors.Is(result.BuildError, context.Canceled) {
		t.Fatalf("BuildError = %v, want context.Canceled", result.BuildError)
	}
	if result.Text == "" {
		t.Fatal("compatibility wrapper returned an empty security prompt")
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
