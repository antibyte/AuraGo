package prompts

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

func TestPromptCompactionRealTokenBaseline(t *testing.T) {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		t.Fatalf("real tokenizer required for prompt compaction baseline (populate TIKTOKEN_CACHE_DIR): %v", err)
	}
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return enc, nil }, time.Second, time.Second)
	// Measured before compaction on 2026-10-16, with these exact synthetic inputs.
	cases := []struct {
		mode, role string
		before     int
	}{
		{"native", "chat", 2326}, {"native", "mission", 2262}, {"native", "coagent", 1892},
		{"text", "chat", 2467}, {"text", "mission", 2403}, {"text", "coagent", 2033},
	}
	fixedTime := regexp.MustCompile(`(?m)^\d{4}-\d\d-\d\dT[^\n]+$`)
	for _, tc := range cases {
		t.Run(tc.mode+"_"+tc.role, func(t *testing.T) {
			flags := &ContextFlags{Tier: "full", SystemLanguage: "en", CorePersonality: "neutral", NativeToolsEnabled: tc.mode == "native", IsTextModeModel: tc.mode == "text", IsMission: tc.role == "mission", IsCoAgent: tc.role == "coagent", MessageSource: "web_chat"}
			if tc.role == "mission" {
				flags.MessageSource = "mission"
			}
			if tc.role == "coagent" {
				flags.MessageSource = ""
			}
			base := BuildSystemPromptBaseDetailed(context.Background(), t.TempDir(), flags, "", nil)
			text := fixedTime.ReplaceAllString(base.Text, "2026-10-16T12:00:00Z")
			tokens := len(enc.Encode(text, nil, nil))
			limit := tc.before
			if tc.role == "chat" {
				limit = tc.before * 90 / 100
			}
			if tokens > limit {
				t.Fatalf("tokens=%d exceeds %d (before=%d)", tokens, limit, tc.before)
			}
			t.Logf("before=%d after=%d saved=%.1f%% bytes=%d", tc.before, tokens, float64(tc.before-tokens)*100/float64(tc.before), len(text))
		})
	}
}

func TestOptimizePromptAccountsForEntireInput(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"empty", "", ""},
		{"comments", "<!-- remove me -->\n# Title:  \n\n\nText   here  \n", "# Title\n\nText here"},
		{"unicode", "Grüße   🌍\n", "Grüße 🌍"},
		{"fenced comment", "~~~html\n<!-- keep me -->\n~~~\n", "~~~html\n<!-- keep me -->\n~~~"},
		{"json", "```json\n{\n  \"a\": 1\n}\n```", "```json\n{\"a\":1}\n```"},
		{"unclosed json", "```json\n{\n  \"a\": 1", "```json\n{\n  \"a\": 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, saved := OptimizePrompt(tc.input)
			if got != tc.want || saved != len(tc.input)-len(got) {
				t.Fatalf("got=%q saved=%d want=%q delta=%d", got, saved, tc.want, len(tc.input)-len(got))
			}
			if again, extra := OptimizePrompt(got); again != got || extra != 0 {
				t.Fatalf("not idempotent: %q / %d", again, extra)
			}
		})
	}
}

func TestPromptBuildAndFitMetricsUseSeparatePopulations(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	flags := &ContextFlags{Tier: "full", NativeToolsEnabled: true, PredictedGuides: []string{"A synthetic guide. " + strings.Repeat("Details. ", 1000)}}
	base := BuildSystemPromptBaseDetailed(context.Background(), t.TempDir(), flags, "", nil)
	agg := GetAggregatedStats()
	if agg.TotalBuilds != 1 || agg.AvgModulesFiltered <= 0 || agg.TotalFilterSavings <= 0 {
		t.Fatalf("filtering missing: %+v", agg)
	}
	build := agg.Recent[0]
	if build.RawLen-build.OptimizedLen != build.FormatSavings || agg.TotalSavedChars != int64(build.FormatSavings) {
		t.Fatalf("filtering included in base delta: %+v", build)
	}
	for _, budget := range []int{0, base.Tokens + 5} {
		fit, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: base.Text, Tokens: base.Tokens, OptionalSections: base.OptionalSections, TokenBudget: budget}, nil)
		if err != nil || fit.BudgetExceeded != nil {
			t.Fatalf("fit error: %v / %v", err, fit.BudgetExceeded)
		}
		wantGuides := 1
		if budget > 0 {
			wantGuides = 0
		}
		if fit.InputGuides != 1 || fit.OutputGuides != wantGuides {
			t.Fatalf("guide counts: %d -> %d, want 1 -> %d", fit.InputGuides, fit.OutputGuides, wantGuides)
		}
		RecordPromptFit(PromptFitRecord{CacheHit: budget > 0, InputChars: fit.InputChars, OutputChars: len(fit.Text), InputTokens: fit.InputTokens, OutputTokens: fit.Tokens, InputGuides: fit.InputGuides, OutputGuides: fit.OutputGuides, RemovedSections: fit.RemovedSections})
	}
	after := GetAggregatedStats()
	if after.TotalBuilds != 1 || after.FitCount != 2 || after.CacheHitCount != 1 || after.TotalSavedChars != agg.TotalSavedChars {
		t.Fatalf("populations mixed: %+v", after)
	}
	if after.AvgFitInputGuides != 1 || after.AvgFitOutputGuides != 0.5 || after.AvgFitSavedTokens <= 0 || after.BudgetShedCount != 1 {
		t.Fatalf("fit aggregates wrong: %+v", after)
	}
	// Token growth is not negative savings; failed/unknown token counts are not measured zeros.
	RecordPromptFit(PromptFitRecord{InputTokens: 10, OutputTokens: 12, BudgetExceeded: true})
	RecordPromptFit(PromptFitRecord{InputTokens: -1, FitFailed: true})
	final := GetAggregatedStats()
	if final.MeasuredFitCount != 3 || final.BudgetExceededCount != 1 || final.AvgFitSavedTokens != after.AvgFitSavedTokens*2/3 {
		t.Fatalf("invalid measurement handling: %+v", final)
	}
}
