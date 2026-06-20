package kgsemantic

import (
	"log/slog"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// QueryCacheEntry stores a pre-computed query embedding with a timestamp for TTL expiry.
type QueryCacheEntry struct {
	Embedding []float32
	Timestamp time.Time
}

// Index owns the chromem collection and in-memory caches for KG semantic search.
type Index struct {
	Collection    *chromem.Collection
	EmbeddingFunc chromem.EmbeddingFunc
	Logger        *slog.Logger
	Mu            sync.Mutex
	ReindexMu     sync.Mutex
	QueryCache    map[string]QueryCacheEntry
	QueryCacheTTL time.Duration
	ContentCache  map[string]string
	ContentKeys   []string
}

// Close releases resources held by the semantic index and clears the embedding cache.
func (idx *Index) Close() {
	idx.Mu.Lock()
	defer idx.Mu.Unlock()
	idx.Collection = nil
	idx.QueryCache = nil
}

// SetContentCacheEntry stores rendered node content for change detection.
func (idx *Index) SetContentCacheEntry(nodeID, content string) {
	if idx.ContentCache == nil {
		idx.ContentCache = make(map[string]string)
	}
	if _, exists := idx.ContentCache[nodeID]; !exists {
		idx.ContentKeys = append(idx.ContentKeys, nodeID)
	}
	idx.ContentCache[nodeID] = content
	idx.TrimContentCache()
}

// RemoveContentCacheEntry drops a node from the content cache.
func (idx *Index) RemoveContentCacheEntry(nodeID string) {
	delete(idx.ContentCache, nodeID)
	for i, key := range idx.ContentKeys {
		if key == nodeID {
			idx.ContentKeys = append(idx.ContentKeys[:i], idx.ContentKeys[i+1:]...)
			return
		}
	}
}

// TrimContentCache evicts oldest entries when the cache grows too large.
func (idx *Index) TrimContentCache() {
	if len(idx.ContentCache) <= ContentCacheMaxSize {
		return
	}
	removeCount := len(idx.ContentCache) / 5
	if removeCount < 1 {
		removeCount = 1
	}
	for i := 0; i < removeCount && len(idx.ContentKeys) > 0; i++ {
		oldestID := idx.ContentKeys[0]
		idx.ContentKeys = idx.ContentKeys[1:]
		delete(idx.ContentCache, oldestID)
	}
}