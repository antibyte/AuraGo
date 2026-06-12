package prompts

import (
	"log/slog"
	"os"
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

func TestReadToolGuideOverrideBlockedWithoutSourceWhenFlagsSet(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	withActivePromptOverridesForTest(t, map[string]string{
		"phantom_tool": "# Optimized Guide\nOverride without canonical manual source.",
	})

	guidePath := filepath.Join(t.TempDir(), "tools_manuals", "phantom_tool.md")
	flags := ContextFlags{AllowShell: true}
	if guide, ok := readToolGuide(guidePath, &flags); ok {
		t.Fatalf("expected override without source to be blocked for dynamic injection, got: %q", guide)
	}

	if guide, ok := ReadToolGuide(guidePath); !ok || !strings.Contains(guide, "Override without canonical manual source.") {
		t.Fatalf("expected explicit ReadToolGuide lookup to keep override available, got ok=%v guide=%q", ok, guide)
	}
}

func TestReadToolGuideOverrideRespectsManualConditionsWhenFlagsSet(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) {
		return charRatioEncoder{}, nil
	}, time.Second, time.Second)
	withActivePromptOverridesForTest(t, map[string]string{
		"execute_sudo": "# Optimized Sudo Guide\nUse sudo carefully.",
	})

	dir := filepath.Join(t.TempDir(), "tools_manuals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manual := "---\nconditions: [\"sudo_enabled\"]\n---\n# execute_sudo\nCanonical manual.\n"
	guidePath := filepath.Join(dir, "execute_sudo.md")
	if err := os.WriteFile(guidePath, []byte(manual), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	disabled := ContextFlags{SudoEnabled: false}
	if guide, ok := readToolGuide(guidePath, &disabled); ok {
		t.Fatalf("expected sudo override to be blocked when sudo is disabled, got: %q", guide)
	}

	enabled := ContextFlags{SudoEnabled: true}
	guide, ok := readToolGuide(guidePath, &enabled)
	if !ok || !strings.Contains(guide, "Use sudo carefully.") {
		t.Fatalf("expected sudo override when sudo is enabled, got ok=%v guide=%q", ok, guide)
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
