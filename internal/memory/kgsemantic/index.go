package kgsemantic

import (
	"context"
	"errors"
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

type queryFlight struct {
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	waiters   int
	embedding []float32
	err       error
	returned  chan struct{}
}

// Index owns the chromem collection and in-memory caches for KG semantic search.
type Index struct {
	Collection    *chromem.Collection
	EmbeddingFunc chromem.EmbeddingFunc
	Logger        *slog.Logger
	Mu            sync.Mutex
	MutationMu    sync.Mutex
	ReindexMu     sync.Mutex
	QueryCache    map[string]QueryCacheEntry
	QueryCacheTTL time.Duration
	queryMu       sync.Mutex
	queryFlights  map[string]*queryFlight
	closed        bool
	ContentCache  map[string]string
	ContentKeys   []string
}

// QueryEmbedding returns a cached embedding or shares one bounded provider call
// among callers that remain interested in the query.
func (idx *Index) QueryEmbedding(ctx context.Context, query string, embed chromem.EmbeddingFunc) ([]float32, error) {
	if idx == nil {
		return nil, errors.New("semantic query index is unavailable")
	}
	if embed == nil {
		return nil, errors.New("semantic embedding function is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	idx.queryMu.Lock()
	if idx.closed {
		idx.queryMu.Unlock()
		return nil, context.Canceled
	}
	if err := ctx.Err(); err != nil {
		idx.queryMu.Unlock()
		return nil, err
	}
	// A completed flight may have populated the cache after the first check.
	idx.Mu.Lock()
	if entry, ok := idx.QueryCache[query]; ok && time.Since(entry.Timestamp) < idx.queryCacheTTL() {
		embedding := cloneEmbedding(entry.Embedding)
		idx.Mu.Unlock()
		idx.queryMu.Unlock()
		return embedding, nil
	}
	idx.Mu.Unlock()

	if idx.queryFlights == nil {
		idx.queryFlights = make(map[string]*queryFlight)
	}
	flight := idx.queryFlights[query]
	if flight != nil {
		flight.waiters++
		idx.queryMu.Unlock()
		return idx.awaitQueryFlight(ctx, query, flight)
	}

	providerCtx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	flight = &queryFlight{ctx: providerCtx, cancel: cancel, done: make(chan struct{}), returned: make(chan struct{}), waiters: 1}
	idx.queryFlights[query] = flight
	idx.queryMu.Unlock()

	go idx.runQueryFlight(query, flight, embed)
	return idx.awaitQueryFlight(ctx, query, flight)
}

func (idx *Index) queryCacheTTL() time.Duration {
	if idx.QueryCacheTTL > 0 {
		return idx.QueryCacheTTL
	}
	return QueryCacheTTL
}

func (idx *Index) awaitQueryFlight(ctx context.Context, query string, flight *queryFlight) ([]float32, error) {
	select {
	case <-flight.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return cloneEmbedding(flight.embedding), flight.err
	case <-ctx.Done():
		idx.leaveQueryFlight(query, flight)
		return nil, ctx.Err()
	case <-flight.ctx.Done():
		select {
		case <-flight.done:
			return cloneEmbedding(flight.embedding), flight.err
		default:
		}
		idx.leaveQueryFlight(query, flight)
		return nil, flight.ctx.Err()
	}
}

func (idx *Index) leaveQueryFlight(query string, flight *queryFlight) {
	idx.queryMu.Lock()
	if idx.queryFlights[query] == flight {
		flight.waiters--
		if flight.waiters == 0 {
			delete(idx.queryFlights, query)
			flight.cancel()
		}
	}
	idx.queryMu.Unlock()
}

func (idx *Index) runQueryFlight(query string, flight *queryFlight, embed chromem.EmbeddingFunc) {
	defer close(flight.returned)
	var embedding []float32
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = errors.New("semantic embedding provider panicked")
			}
		}()
		embedding, err = embed(flight.ctx, query)
	}()
	if err == nil && flight.ctx.Err() != nil {
		err = flight.ctx.Err()
	}

	idx.queryMu.Lock()
	if idx.queryFlights[query] == flight {
		if err == nil && !idx.closed {
			flight.embedding = cloneEmbedding(embedding)
			idx.cacheQueryEmbeddingLocked(query, embedding)
		} else if err == nil {
			err = context.Canceled
		}
		flight.err = err
		delete(idx.queryFlights, query)
		close(flight.done)
	}
	idx.queryMu.Unlock()
	flight.cancel()
}

// cacheQueryEmbeddingLocked is called with queryMu held so an abandoned older
// flight can never overwrite the result from a newer flight for the same key.
func (idx *Index) cacheQueryEmbeddingLocked(query string, embedding []float32) {
	now := time.Now()
	idx.Mu.Lock()
	defer idx.Mu.Unlock()
	if idx.QueryCache == nil {
		idx.QueryCache = make(map[string]QueryCacheEntry)
	}
	idx.QueryCache[query] = QueryCacheEntry{Embedding: cloneEmbedding(embedding), Timestamp: now}
	if len(idx.QueryCache) <= QueryCacheMaxSize {
		return
	}
	var expiredKeys []string
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range idx.QueryCache {
		if now.Sub(entry.Timestamp) > idx.queryCacheTTL() {
			expiredKeys = append(expiredKeys, key)
		} else if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
			oldestKey, oldestTime = key, entry.Timestamp
		}
	}
	if len(expiredKeys) > 0 {
		for _, key := range expiredKeys {
			delete(idx.QueryCache, key)
		}
	} else if len(idx.QueryCache) > QueryCacheMaxSize && oldestKey != "" {
		delete(idx.QueryCache, oldestKey)
	}
}

func cloneEmbedding(embedding []float32) []float32 {
	if embedding == nil {
		return nil
	}
	return append([]float32(nil), embedding...)
}

func (idx *Index) cancelQueryFlights() {
	idx.queryMu.Lock()
	idx.closed = true
	for query, flight := range idx.queryFlights {
		delete(idx.queryFlights, query)
		flight.err = context.Canceled
		flight.cancel()
		close(flight.done)
	}
	idx.queryMu.Unlock()
}

// Close releases resources held by the semantic index and clears the embedding cache.
func (idx *Index) Close() {
	idx.cancelQueryFlights()
	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()
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
	if _, exists := idx.ContentCache[nodeID]; exists {
		idx.removeContentCacheKey(nodeID)
	}
	idx.ContentKeys = append(idx.ContentKeys, nodeID)
	idx.ContentCache[nodeID] = content
	idx.TrimContentCache()
}

// RemoveContentCacheEntry drops a node from the content cache.
func (idx *Index) RemoveContentCacheEntry(nodeID string) {
	delete(idx.ContentCache, nodeID)
	idx.removeContentCacheKey(nodeID)
}

func (idx *Index) removeContentCacheKey(nodeID string) {
	if len(idx.ContentKeys) == 0 {
		return
	}
	filtered := idx.ContentKeys[:0]
	for _, key := range idx.ContentKeys {
		if key != nodeID {
			filtered = append(filtered, key)
		}
	}
	idx.ContentKeys = filtered
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
