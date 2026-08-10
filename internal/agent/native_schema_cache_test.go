package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestBoundedNativeSchemaCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := newBoundedNativeSchemaCache(2)
	key1 := dynamicToolSchemaCacheKey{SkillsFingerprint: "one"}
	key2 := dynamicToolSchemaCacheKey{SkillsFingerprint: "two"}
	key3 := dynamicToolSchemaCacheKey{SkillsFingerprint: "three"}
	cache.Store(key1, &nativeToolSchemaSnapshot{})
	cache.Store(key2, &nativeToolSchemaSnapshot{})
	if _, ok := cache.Load(key1); !ok {
		t.Fatal("expected first key before eviction")
	}
	cache.Store(key3, &nativeToolSchemaSnapshot{})
	if cache.Len() != 2 {
		t.Fatalf("cache length = %d, want 2", cache.Len())
	}
	if _, ok := cache.Load(key2); ok {
		t.Fatal("least recently used key was not evicted")
	}
	if _, ok := cache.Load(key1); !ok {
		t.Fatal("recently touched key was evicted")
	}
}

func TestNativeSkillsRevisionCacheHandlesExternalAndExplicitInvalidation(t *testing.T) {
	dir := t.TempDir()
	nativeSkillsRevisionCache.mu.Lock()
	nativeSkillsRevisionCache.entries = make(map[string]nativeSkillsRevisionCacheEntry)
	nativeSkillsRevisionCache.mu.Unlock()
	first := nativeSkillsFingerprint(dir)
	if err := os.WriteFile(filepath.Join(dir, "external.json"), []byte(`{"name":"external"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	absDir, _ := filepath.Abs(dir)
	nativeSkillsRevisionCache.mu.Lock()
	entry := nativeSkillsRevisionCache.entries[absDir]
	entry.checked = time.Now().Add(-nativeSkillsRevisionTTL - time.Second)
	nativeSkillsRevisionCache.entries[absDir] = entry
	nativeSkillsRevisionCache.mu.Unlock()
	second := nativeSkillsFingerprint(dir)
	if second == first {
		t.Fatal("external skill file revision was not observed after TTL")
	}
	if err := os.WriteFile(filepath.Join(dir, "owned.json"), []byte(`{"name":"owned"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tools.InvalidateSkillsCache(dir)
	third := nativeSkillsFingerprint(dir)
	if third == second {
		t.Fatal("explicit AuraGo skill invalidation did not bypass TTL")
	}
}
