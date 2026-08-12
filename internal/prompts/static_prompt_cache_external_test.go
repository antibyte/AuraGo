package prompts

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStaticPromptCachesObserveExternalFileChangesAfterRevisionCheck(t *testing.T) {
	ClearPromptCache()
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "external.md")
	personalityDir := filepath.Join(dir, "personalities")
	if err := os.MkdirAll(personalityDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	personalityPath := filepath.Join(personalityDir, "neutral.md")
	if err := os.WriteFile(modulePath, []byte("first module"), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	if err := os.WriteFile(personalityPath, []byte("first personality"), 0o644); err != nil {
		t.Fatalf("write personality: %v", err)
	}

	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "first module") {
		t.Fatal("initial disk prompt module was not loaded")
	}
	if got := loadCorePersonalityContent(dir, "neutral", slog.Default()); got != "first personality" {
		t.Fatalf("initial personality = %q", got)
	}

	if err := os.WriteFile(modulePath, []byte("second module with changed size"), 0o644); err != nil {
		t.Fatalf("rewrite module: %v", err)
	}
	if err := os.WriteFile(personalityPath, []byte("second personality"), 0o644); err != nil {
		t.Fatalf("rewrite personality: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(modulePath, future, future); err != nil {
		t.Fatalf("Chtimes module: %v", err)
	}
	if err := os.Chtimes(personalityPath, future, future); err != nil {
		t.Fatalf("Chtimes personality: %v", err)
	}

	promptCacheMu.Lock()
	moduleEntry := promptCacheByDir[dir]
	moduleEntry.checked = time.Now().Add(-time.Minute - time.Second)
	promptCacheByDir[dir] = moduleEntry
	promptCacheMu.Unlock()
	personalityCacheMu.Lock()
	personalityKey := filepath.Clean(dir) + "\x00neutral"
	personalityEntry := personalityCache[personalityKey]
	personalityEntry.checked = time.Now().Add(-time.Minute - time.Second)
	personalityCache[personalityKey] = personalityEntry
	personalityCacheMu.Unlock()

	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "second module with changed size") {
		t.Fatal("external prompt-module revision did not invalidate the cache")
	}
	if got := loadCorePersonalityContent(dir, "neutral", slog.Default()); got != "second personality" {
		t.Fatalf("external personality revision was not loaded: %q", got)
	}
}

func TestStaticPromptCachesObserveReplacementWithOlderModTime(t *testing.T) {
	ClearPromptCache()
	metaCacheMu.Lock()
	metaCache = make(map[string]metaCacheEntry)
	metaCacheMu.Unlock()
	personalityCacheMu.Lock()
	personalityCache = make(map[string]personalityCacheEntry)
	personalityCacheMu.Unlock()

	dir := t.TempDir()
	personalityDir := filepath.Join(dir, "personalities")
	if err := os.MkdirAll(personalityDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	modulePath := filepath.Join(dir, "external.md")
	guidePath := filepath.Join(dir, "external-guide.md")
	personalityPath := filepath.Join(personalityDir, "rollback.md")
	writePersonality := func(body, response string) {
		t.Helper()
		raw := "---\nid: rollback\ntags: [core]\npriority: 100\nmeta:\n  conflict_response: " + response + "\n---\n\n" + body
		if err := os.WriteFile(personalityPath, []byte(raw), 0o644); err != nil {
			t.Fatalf("write personality: %v", err)
		}
	}
	if err := os.WriteFile(modulePath, []byte("module version one"), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	if err := os.WriteFile(guidePath, []byte("guide version one"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	writePersonality("personality version one", "neutral")

	newer := time.Now().Add(2 * time.Hour)
	for _, path := range []string{modulePath, guidePath, personalityPath} {
		if err := os.Chtimes(path, newer, newer); err != nil {
			t.Fatalf("set initial mtime for %s: %v", path, err)
		}
	}
	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "module version one") {
		t.Fatal("initial module was not loaded")
	}
	if guide, ok := readToolGuide(guidePath, nil); !ok || !strings.Contains(guide, "guide version one") {
		t.Fatalf("initial guide = %q, ok=%v", guide, ok)
	}
	if got := loadCorePersonalityContent(dir, "rollback", slog.Default()); got != "personality version one" {
		t.Fatalf("initial personality = %q", got)
	}
	if got := GetCorePersonalityMeta(dir, "rollback").ConflictResponse; got != "neutral" {
		t.Fatalf("initial personality metadata = %q", got)
	}

	if err := os.WriteFile(modulePath, []byte("module version two"), 0o644); err != nil {
		t.Fatalf("replace module: %v", err)
	}
	if err := os.WriteFile(guidePath, []byte("guide version two"), 0o644); err != nil {
		t.Fatalf("replace guide: %v", err)
	}
	writePersonality("personality version two", "assertive")
	older := newer.Add(-time.Hour)
	for _, path := range []string{modulePath, guidePath, personalityPath} {
		if err := os.Chtimes(path, older, older); err != nil {
			t.Fatalf("set replacement mtime for %s: %v", path, err)
		}
	}

	promptCacheMu.Lock()
	moduleEntry := promptCacheByDir[dir]
	moduleEntry.checked = time.Now().Add(-time.Minute - time.Second)
	promptCacheByDir[dir] = moduleEntry
	promptCacheMu.Unlock()
	personalityCacheMu.Lock()
	personalityKey := filepath.Clean(dir) + "\x00rollback"
	personalityEntry := personalityCache[personalityKey]
	personalityEntry.checked = time.Now().Add(-time.Minute - time.Second)
	personalityCache[personalityKey] = personalityEntry
	personalityCacheMu.Unlock()

	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "module version two") {
		t.Fatal("module replacement with older mtime did not invalidate cache")
	}
	if guide, ok := readToolGuide(guidePath, nil); !ok || !strings.Contains(guide, "guide version two") {
		t.Fatalf("guide replacement with older mtime = %q, ok=%v", guide, ok)
	}
	if got := loadCorePersonalityContent(dir, "rollback", slog.Default()); got != "personality version two" {
		t.Fatalf("personality replacement with older mtime = %q", got)
	}
	if got := GetCorePersonalityMeta(dir, "rollback").ConflictResponse; got != "assertive" {
		t.Fatalf("personality metadata replacement with older mtime = %q", got)
	}
}

func TestStaticPromptCachesObserveSameMetadataContentChanges(t *testing.T) {
	ClearPromptCache()
	dir := t.TempDir()
	personalityDir := filepath.Join(dir, "personalities")
	if err := os.MkdirAll(personalityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	modulePath := filepath.Join(dir, "same.md")
	guidePath := filepath.Join(dir, "same-guide.md")
	personalityPath := filepath.Join(personalityDir, "same.md")
	first := map[string]string{
		modulePath:      "# module\nCACHE ONE",
		guidePath:       "# guide\nCACHE ONE",
		personalityPath: "---\nid: same\ntags: [core]\nmeta:\n  conflict_response: submissive\n---\n\npersona one",
	}
	second := map[string]string{
		modulePath:      "# module\nCACHE TWO",
		guidePath:       "# guide\nCACHE TWO",
		personalityPath: "---\nid: same\ntags: [core]\nmeta:\n  conflict_response: diplomatic\n---\n\npersona two",
	}
	mtimes := make(map[string]time.Time, len(first))
	for path, content := range first {
		if len(content) != len(second[path]) {
			t.Fatalf("fixture sizes differ for %s", filepath.Base(path))
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		mtimes[path] = info.ModTime()
	}

	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "CACHE ONE") {
		t.Fatal("initial module content missing")
	}
	if got, ok := readToolGuide(guidePath, nil); !ok || !strings.Contains(got, "CACHE ONE") {
		t.Fatalf("initial guide = %q, ok=%v", got, ok)
	}
	if got := loadCorePersonalityContent(dir, "same", slog.Default()); got != "persona one" {
		t.Fatalf("initial personality = %q", got)
	}
	if got := GetCorePersonalityMeta(dir, "same").ConflictResponse; got != "submissive" {
		t.Fatalf("initial metadata = %q", got)
	}

	for path, content := range second {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtimes[path], mtimes[path]); err != nil {
			t.Fatal(err)
		}
	}

	if modules := loadPromptModules(dir, slog.Default()); !promptModulesContain(modules, "CACHE TWO") {
		t.Fatal("same-metadata module replacement was not detected")
	}
	if got, ok := readToolGuide(guidePath, nil); !ok || !strings.Contains(got, "CACHE TWO") {
		t.Fatalf("same-metadata guide replacement = %q, ok=%v", got, ok)
	}
	if got := loadCorePersonalityContent(dir, "same", slog.Default()); got != "persona two" {
		t.Fatalf("same-metadata personality replacement = %q", got)
	}
	if got := GetCorePersonalityMeta(dir, "same").ConflictResponse; got != "diplomatic" {
		t.Fatalf("same-metadata personality metadata = %q", got)
	}
}

func promptModulesContain(modules []PromptModule, content string) bool {
	for _, module := range modules {
		if strings.Contains(module.Content, content) {
			return true
		}
	}
	return false
}
