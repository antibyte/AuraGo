package kgsemantic

import "testing"

func TestContentCacheRefreshesRecencyAndRemovesAllKeyEntries(t *testing.T) {
	idx := &Index{}

	idx.SetContentCacheEntry("old", "old content")
	idx.SetContentCacheEntry("middle", "middle content")
	idx.SetContentCacheEntry("old", "fresh content")

	if got := idx.ContentKeys; len(got) != 2 || got[0] != "middle" || got[1] != "old" {
		t.Fatalf("ContentKeys after refresh = %#v, want middle then old", got)
	}
	if idx.ContentCache["old"] != "fresh content" {
		t.Fatalf("ContentCache[old] = %q, want refreshed content", idx.ContentCache["old"])
	}

	idx.ContentKeys = append(idx.ContentKeys, "old")
	idx.RemoveContentCacheEntry("old")
	if _, ok := idx.ContentCache["old"]; ok {
		t.Fatal("expected old cache entry removed")
	}
	for _, key := range idx.ContentKeys {
		if key == "old" {
			t.Fatalf("expected all old keys removed, got %#v", idx.ContentKeys)
		}
	}
}
