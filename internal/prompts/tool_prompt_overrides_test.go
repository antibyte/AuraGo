package prompts

import (
	"fmt"
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

func TestGuideCacheImmediatelySwitchesBetweenEmbedAndDisk(t *testing.T) {
	clearGuideCacheForTest()
	root := t.TempDir()
	path := filepath.Join(root, "tools_manuals", "filesystem.md")
	embedded, ok := ReadToolGuide(path)
	if !ok || embedded == "" {
		t.Fatal("expected embedded filesystem guide")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const override = "# filesystem\nIMMEDIATE DISK OVERRIDE"
	if err := os.WriteFile(path, []byte(override), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadToolGuide(path); !ok || !strings.Contains(got, "IMMEDIATE DISK OVERRIDE") {
		t.Fatalf("embed-to-disk transition = %q, ok=%v", got, ok)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadToolGuide(path); !ok || got != embedded {
		t.Fatalf("disk-to-embed transition = %q, want embedded guide", got)
	}
}

func TestMalformedDiskGuideFallsBackToEmbedAndRecoversAfterFix(t *testing.T) {
	clearGuideCacheForTest()
	root := t.TempDir()
	path := filepath.Join(root, "tools_manuals", "filesystem.md")
	embedded, ok := ReadToolGuide(path)
	if !ok || embedded == "" {
		t.Fatal("expected embedded filesystem guide")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nconditions: [filesystem_enabled]\n# missing delimiter\nMALFORMED GUIDE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadToolGuide(path); !ok || got != embedded {
		t.Fatalf("malformed override = %q, ok=%v, want embedded fallback", got, ok)
	}

	const fixed = "# filesystem\nFIXED DISK GUIDE"
	if err := os.WriteFile(path, []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadToolGuide(path); !ok || !strings.Contains(got, "FIXED DISK GUIDE") {
		t.Fatalf("corrected disk guide = %q, ok=%v", got, ok)
	}
}

func TestMalformedCustomOnlyGuideIsUnavailable(t *testing.T) {
	clearGuideCacheForTest()
	path := filepath.Join(t.TempDir(), "custom_manuals", "phantom.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nconditions: [shell_enabled]\n# missing delimiter\nMALFORMED GUIDE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := ReadToolGuide(path); ok || got != "" {
		t.Fatalf("malformed custom guide = %q, ok=%v, want unavailable", got, ok)
	}
}

func TestMalformedDiskGuideFallbackKeepsEmbeddedConditions(t *testing.T) {
	clearGuideCacheForTest()
	path := filepath.Join(t.TempDir(), "tools_manuals", "execute_sudo.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nconditions: [sudo_enabled]\n# missing delimiter\nMALFORMED SUDO GUIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, ok := readToolGuide(path, &ContextFlags{SudoEnabled: false}); ok || got != "" {
		t.Fatalf("embedded sudo condition was lost on fallback: %q, ok=%v", got, ok)
	}
	if got, ok := readToolGuide(path, &ContextFlags{SudoEnabled: true}); !ok || strings.Contains(got, "MALFORMED SUDO GUIDE") {
		t.Fatalf("valid embedded sudo guide was not used: %q, ok=%v", got, ok)
	}
}

func TestGuideCacheIsBoundedLRU(t *testing.T) {
	clearGuideCacheForTest()
	dir := t.TempDir()
	for i := 0; i < guideCacheLimit+12; i++ {
		path := filepath.Join(dir, fmt.Sprintf("guide-%03d.md", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("# guide_%03d\nbody", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := ReadToolGuide(path); !ok {
			t.Fatalf("guide %d was not read", i)
		}
	}
	guideCacheMu.RLock()
	count := len(guideCache)
	guideCacheMu.RUnlock()
	if count > guideCacheLimit {
		t.Fatalf("guide cache size = %d, want <= %d", count, guideCacheLimit)
	}
}

func TestClearPromptCacheAdvancesGeneration(t *testing.T) {
	before := PromptCacheGeneration()
	ClearPromptCache()
	if after := PromptCacheGeneration(); after <= before {
		t.Fatalf("generation = %d, want > %d", after, before)
	}
}
