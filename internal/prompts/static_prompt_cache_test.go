package prompts

import (
	"log/slog"
	"testing"
)

func TestStaticPromptCachesStayBounded(t *testing.T) {
	promptCacheMu.Lock()
	promptCacheByDir = make(map[string]promptDirCache)
	promptCacheMu.Unlock()
	personalityCacheMu.Lock()
	personalityCache = make(map[string]personalityCacheEntry)
	personalityCacheMu.Unlock()

	for i := 0; i < staticPromptCacheLimit+12; i++ {
		dir := t.TempDir()
		_ = loadPromptModules(dir, slog.Default())
		_ = loadCorePersonalityContent(dir, "neutral", slog.Default())
	}
	promptCacheMu.RLock()
	moduleCount := len(promptCacheByDir)
	promptCacheMu.RUnlock()
	personalityCacheMu.RLock()
	personalityCount := len(personalityCache)
	personalityCacheMu.RUnlock()
	if moduleCount > staticPromptCacheLimit {
		t.Fatalf("prompt module cache size = %d, limit = %d", moduleCount, staticPromptCacheLimit)
	}
	if personalityCount > staticPromptCacheLimit {
		t.Fatalf("personality cache size = %d, limit = %d", personalityCount, staticPromptCacheLimit)
	}
}
