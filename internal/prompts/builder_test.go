package prompts

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubTokenEncoder struct {
	tokensPerCall int
}

func (s stubTokenEncoder) Encode(text string, allowedSpecial, disallowedSpecial []string) []int {
	if s.tokensPerCall <= 0 {
		return nil
	}
	return make([]int, s.tokensPerCall)
}

func resetTokenEncoderStateForTest(tb testing.TB, loader func() (tokenEncoder, error), timeout, backoff time.Duration) {
	tb.Helper()

	tiktokenMu.Lock()
	oldEnc := tiktokenEnc
	oldDone := tiktokenInitDone
	oldInFlight := tiktokenInitInFlight
	oldNextRetry := tiktokenNextRetry
	oldLoader := tiktokenLoadEncoding
	oldTimeout := tiktokenInitTimeout
	oldBackoff := tiktokenRetryBackoff
	oldWarnOnce := tiktokenWarnOnce
	tiktokenMu.Unlock()

	tiktokenMu.Lock()
	tiktokenEnc = nil
	tiktokenInitDone = nil
	tiktokenInitInFlight = false
	tiktokenNextRetry = time.Time{}
	tiktokenLoadEncoding = loader
	tiktokenInitTimeout = timeout
	tiktokenRetryBackoff = backoff
	tiktokenWarnOnce = sync.Once{}
	tiktokenMu.Unlock()

	tb.Cleanup(func() {
		tiktokenMu.Lock()
		tiktokenEnc = oldEnc
		tiktokenInitDone = oldDone
		tiktokenInitInFlight = oldInFlight
		tiktokenNextRetry = oldNextRetry
		tiktokenLoadEncoding = oldLoader
		tiktokenInitTimeout = oldTimeout
		tiktokenRetryBackoff = oldBackoff
		tiktokenWarnOnce = oldWarnOnce
		tiktokenMu.Unlock()
	})
}

func TestBudgetShed_RemovesUserProfilingSection(t *testing.T) {
	// Create a prompt with the ## USER PROFILING section that should be removed
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

# TOOL GUIDES
tool guide content here

## USER PROFILING
Your goal: build a comprehensive user profile over time.

### Known User Profile
User name: Test User

# RETRIEVED MEMORIES
memory entry 1

# NOW
2026-04-09 12:00`

	flags := ContextFlags{
		Tier:        "full",
		TokenBudget: 5, // Very small budget - should shed everything possible
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections, got: %v", shedSections)
	}

	// Verify the section is actually removed from the result
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result, but it was found")
	}
	if strings.Contains(result, "Your goal: build a comprehensive user profile") {
		t.Errorf("expected user profiling content to be removed from result")
	}
}

func TestBudgetShed_HardTruncateWhenCoreExceedsBudget(t *testing.T) {
	prompt := strings.Repeat("word ", 5000)

	flags := ContextFlags{
		Tier:        "minimal",
		TokenBudget: 10,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	if !strings.Contains(result, "[BUDGET TRUNCATED]") {
		t.Errorf("expected hard-truncate marker, got result len=%d", len(result))
	}

	foundHardTruncate := false
	for _, s := range shedSections {
		if s == "HARD_TRUNCATE" {
			foundHardTruncate = true
		}
	}
	if !foundHardTruncate {
		t.Errorf("expected HARD_TRUNCATE in shedSections, got: %v", shedSections)
	}

	if len(result) >= len(prompt) {
		t.Errorf("expected result to be shorter than original, got len=%d vs orig=%d", len(result), len(prompt))
	}
}

func TestBudgetShed_UnifiedMemoryRemovesUserProfile(t *testing.T) {
	// Test with UnifiedMemoryBlock enabled
	prompt := `# SYSTEM IDENTITY
You are AuraGo.

## USER PROFILING
Your goal: build a comprehensive user profile.

### Known User Profile
User name: Test User

# UNIFIED MEMORY CONTEXT
## Recent Activity
some activity`

	flags := ContextFlags{
		Tier:               "compact",
		TokenBudget:        5,
		UnifiedMemoryBlock: true,
	}

	logger := slog.Default()
	result, shedSections := budgetShed(prompt, &flags, "", "", time.Now(), logger)

	// Check that ## USER PROFILING was shed (under UnifiedMemoryBlock path)
	foundProfiling := false
	for _, section := range shedSections {
		if section == "## USER PROFILING" {
			foundProfiling = true
			break
		}
	}
	if !foundProfiling {
		t.Errorf("expected ## USER PROFILING to be in shedSections (unified path), got: %v", shedSections)
	}

	// Verify the section is actually removed
	if strings.Contains(result, "## USER PROFILING") {
		t.Errorf("expected ## USER PROFILING to be removed from result")
	}
}

func TestBuildSystemPromptIncludesPlannerContext(t *testing.T) {
	flags := ContextFlags{
		SystemLanguage: "en",
		PlannerContext: "Open todos: 2\n- [HIGH] Patch planner\nUpcoming appointments (next 48h):\n- 2026-04-18T09:00:00Z: Review",
	}

	prompt, _ := buildSystemPromptInner("", &flags, "", slog.Default())
	if !strings.Contains(prompt, "### PLANNER CONTEXT ###") {
		t.Fatalf("prompt = %q, want planner context header", prompt)
	}
	if !strings.Contains(prompt, "Open todos: 2") {
		t.Fatalf("prompt = %q, want planner context content", prompt)
	}
}

func TestFallbackSystemPromptIncludesEmbeddedSafetyRules(t *testing.T) {
	flags := ContextFlags{SystemLanguage: "en"}

	prompt, _ := fallbackSystemPrompt("", &flags, "Remember this", slog.Default())

	if !strings.Contains(prompt, "# CORE IDENTITY") {
		t.Fatalf("fallback prompt missing identity section: %q", prompt)
	}
	if !strings.Contains(prompt, "## SAFETY & SECURITY") {
		t.Fatalf("fallback prompt missing safety rules: %q", prompt)
	}
	if !strings.Contains(prompt, "Remember this") {
		t.Fatalf("fallback prompt missing core memory: %q", prompt)
	}
}

func TestCountTokensRetriesAfterTimeoutAndUsesLateSuccess(t *testing.T) {
	block := make(chan struct{})
	callCount := 0
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		callCount++
		if callCount == 1 {
			<-block
			return stubTokenEncoder{tokensPerCall: 3}, nil
		}
		return stubTokenEncoder{tokensPerCall: 5}, nil
	}, 5*time.Millisecond, 0)

	first := CountTokens("first call falls back")
	if first <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", first)
	}

	close(block)

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if CountTokens("second call should use initialized encoder") == 3 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("expected late encoder initialization to be reused; loader calls=%d", callCount)
}

func TestCountTokensRetriesAfterFailure(t *testing.T) {
	callCount := 0
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("boom")
		}
		return stubTokenEncoder{tokensPerCall: 7}, nil
	}, 25*time.Millisecond, 0)

	first := CountTokens("fallback after error")
	second := CountTokens("second attempt should retry")

	if first <= 0 {
		t.Fatalf("expected fallback token count to be positive, got %d", first)
	}
	if second != 7 {
		t.Fatalf("expected retry to use encoder token count 7, got %d", second)
	}
	if callCount < 2 {
		t.Fatalf("expected loader to be retried, got %d calls", callCount)
	}
}

func TestHardTruncateToBudgetStaysWithinBudget(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return stubTokenEncoder{tokensPerCall: 0}, nil
	}, 25*time.Millisecond, 0)

	// Swap in a token encoder that counts one token per rune for predictable truncation.
	tiktokenMu.Lock()
	tiktokenEnc = runeCountingEncoder{}
	tiktokenMu.Unlock()

	prompt := strings.Repeat("abcdef", 20)
	truncated := hardTruncateToBudget(prompt, 15, "")
	if got := CountTokens(truncated); got > 15 {
		t.Fatalf("expected truncated prompt to fit budget, got %d tokens", got)
	}
	if truncated == "" {
		t.Fatal("expected some truncated content")
	}
}

type runeCountingEncoder struct{}

func (runeCountingEncoder) Encode(text string, allowedSpecial, disallowedSpecial []string) []int {
	return make([]int, len([]rune(text)))
}

func TestShouldIncludeMainAgentConditions(t *testing.T) {
	mod := PromptModule{
		Metadata: PromptMetadata{
			Conditions: []string{"main_agent"},
		},
	}

	if !mod.ShouldInclude(&ContextFlags{}) {
		t.Fatal("expected main agent module to be included for default flags")
	}
	if mod.ShouldInclude(&ContextFlags{IsCoAgent: true}) {
		t.Fatal("expected main agent module to be excluded for co-agent")
	}
	if mod.ShouldInclude(&ContextFlags{IsEgg: true}) {
		t.Fatal("expected main agent module to be excluded for egg")
	}
}

func TestFilterModulesKeepsCoreModulesWithoutConditions(t *testing.T) {
	modules := []PromptModule{
		{
			Metadata: PromptMetadata{
				ID:   "core",
				Tags: []string{"core"},
			},
			Content: "core",
		},
		{
			Metadata: PromptMetadata{
				ID:         "coagent",
				Conditions: []string{"coagent"},
			},
			Content: "coagent",
		},
	}

	filtered := filterModules(modules, &ContextFlags{})
	if len(filtered) != 1 || filtered[0].Metadata.ID != "core" {
		t.Fatalf("unexpected filtered modules: %+v", filtered)
	}
}

func TestParsePromptModuleHandlesBOMAndCRLF(t *testing.T) {
	raw := "\xEF\xBB\xBF\r\n---\r\nid: test\r\ntags: [\"core\"]\r\npriority: 1\r\n---\r\n# Body\r\nLine\r\n"

	mod, err := parsePromptModule(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if mod.Metadata.ID != "test" {
		t.Fatalf("unexpected id: %q", mod.Metadata.ID)
	}
	if !strings.Contains(mod.Content, "# Body") {
		t.Fatalf("unexpected content: %q", mod.Content)
	}
}

func TestParsePromptModuleRejectsMissingClosingFrontmatter(t *testing.T) {
	raw := "---\nid: broken\ntags: [\"core\"]\npriority: 1\n# no closing delimiter"

	if _, err := parsePromptModule(raw); err == nil {
		t.Fatal("expected parsePromptModule to reject missing closing frontmatter")
	}
}
