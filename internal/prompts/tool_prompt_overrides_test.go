package prompts

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clearGuideCacheForTest() {
	guideCacheMu.Lock()
	guideCache = make(map[string]guideCacheEntry)
	guideCacheMu.Unlock()
}

func withActivePromptOverridesForTest(tb testing.TB, overrides map[string]string) {
	tb.Helper()

	old := GetActivePromptOverrides
	GetActivePromptOverrides = func() map[string]string {
		return overrides
	}
	ClearPromptCache()
	clearGuideCacheForTest()

	tb.Cleanup(func() {
		GetActivePromptOverrides = old
		ClearPromptCache()
		clearGuideCacheForTest()
	})
}

func TestActiveToolPromptOverridesAreNotGlobalPromptModules(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	withActivePromptOverridesForTest(t, map[string]string{
		"filesystem": "GLOBAL TOOL MANUAL OVERRIDE POISON",
	})

	prompt, _ := buildSystemPromptInner(t.TempDir(), &ContextFlags{
		Tier:           "full",
		SystemLanguage: "en",
	}, "", slog.Default())

	if strings.Contains(prompt, "GLOBAL TOOL MANUAL OVERRIDE POISON") {
		t.Fatalf("active tool manual override leaked into the global system prompt")
	}
}

func TestReadToolGuideUsesSanitizedActiveOverride(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	withActivePromptOverridesForTest(t, map[string]string{
		"filesystem": "<think>hidden optimizer reasoning</think>\n# Clean Guide\nUse exact operations.",
	})

	guidePath := filepath.Join(t.TempDir(), "tools_manuals", "filesystem.md")
	guide, ok := ReadToolGuide(guidePath)
	if !ok {
		t.Fatalf("expected filesystem tool guide to be available")
	}
	if !strings.Contains(guide, "Use exact operations.") {
		t.Fatalf("expected sanitized active override to be used, got: %q", guide)
	}
	if strings.Contains(guide, "<think>") || strings.Contains(guide, "hidden optimizer reasoning") {
		t.Fatalf("expected hidden reasoning to be stripped from optimized guide, got: %q", guide)
	}
}
