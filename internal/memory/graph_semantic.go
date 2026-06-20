package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

const knowledgeGraphSemanticCollection = "kg_embeddings"
const knowledgeGraphSemanticTimeout = 60 * time.Second
const knowledgeGraphSemanticEdgeMinSimilarity = 0.35

const knowledgeGraphSemanticQueryCacheTTL = 5 * time.Minute
const knowledgeGraphSemanticEdgeMaxResults = 50
const knowledgeGraphSemanticQueryCacheMaxSize = 100

const knowledgeGraphSemanticRetryMaxAttempts = 3
const knowledgeGraphSemanticRetryBackoffBase = 250 * time.Millisecond
const knowledgeGraphConsistencyCheckSampleSize = 200
const knowledgeGraphSemanticEdgeReindexBatchSize = 100

type knowledgeGraphSemanticIndex struct {
	collection    *chromem.Collection
	embeddingFunc chromem.EmbeddingFunc
	logger        *slog.Logger
	mu            sync.Mutex
	reindexMu     sync.Mutex
	queryCache       map[string]queryCacheEntry
	queryCacheTTL    time.Duration
	contentCache     map[string]string
	contentCacheKeys []string
}

const knowledgeGraphSemanticContentCacheMaxSize = 5000

// Close releases resources held by the semantic index and clears the embedding cache.
func (idx *knowledgeGraphSemanticIndex) Close() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.collection = nil
	idx.queryCache = nil
}

func (kg *KnowledgeGraph) EnableSemanticSearch(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}

	embeddingFunc, _, disabled := buildEmbeddingFuncFromConfig(cfg, kg.logger)
	if disabled {
		if kg.logger != nil {
			kg.logger.Info("KG semantic search disabled because embeddings are disabled")
		}
		return nil
	}

	_ = embeddingFunc
	if kg.logger != nil {
		kg.logger.Warn("EnableSemanticSearch: non-shared path is deprecated, use EnableSemanticSearchShared instead")
	}
	return fmt.Errorf("EnableSemanticSearch is deprecated: use EnableSemanticSearchShared")
}

// EnableSemanticSearchShared reuses an already-open chromem.DB and its embedding
// function from the long-term memory subsystem, avoiding a second database
// handle for the same on-disk VectorDB directory.
//
// Use this instead of EnableSemanticSearch when longTermMem.IsDisabled() == false.
func (kg *KnowledgeGraph) EnableSemanticSearchShared(db *chromem.DB, embeddingFunc chromem.EmbeddingFunc) error {
	if db == nil {
		return fmt.Errorf("shared db is required")
	}
	if embeddingFunc == nil {
		return fmt.Errorf("embedding func is required")
	}
	// Validate that the LTM ChromemVectorDB is initialized before sharing.
	if cols := db.ListCollections(); cols == nil {
		return fmt.Errorf("shared db is not initialized")
	}
	if kg.logger != nil {
		kg.logger.Info("KG semantic search shared DB initialized")
	}
	return kg.enableSemanticSearchWithCollection(db, embeddingFunc, kg.logger)
}

func (kg *KnowledgeGraph) enableSemanticSearchWithCollection(db *chromem.DB, embeddingFunc chromem.EmbeddingFunc, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("semantic db is required")
	}
	if embeddingFunc == nil {
		return fmt.Errorf("embedding func is required")
	}

	collection, err := db.GetOrCreateCollection(knowledgeGraphSemanticCollection, nil, embeddingFunc)
	if err != nil {
		return fmt.Errorf("get/create semantic collection: %w", err)
	}

	index := &knowledgeGraphSemanticIndex{
		collection:    collection,
		embeddingFunc: embeddingFunc,
		logger:        logger,
		queryCache:    make(map[string]queryCacheEntry),
		queryCacheTTL: knowledgeGraphSemanticQueryCacheTTL,
		contentCache:     make(map[string]string),
		contentCacheKeys: make([]string, 0),
	}

	if err := kg.validateSemanticIndex(index); err != nil {
		return err
	}
	kg.semanticMu.Lock()
	kg.semantic = index
	kg.semanticMu.Unlock()
	if err := kg.reindexSemanticNodes(); err != nil {
		return err
	}
	kg.markSemanticReindexComplete()
	return nil
}

func (kg *KnowledgeGraph) validateSemanticIndex(index *knowledgeGraphSemanticIndex) error {
	err := kg.retrySemanticEmbedding("validate", func(ctx context.Context) error {
		_, err := index.embeddingFunc(ctx, "knowledge graph semantic validation")
		return err
	})
	if err != nil {
		return fmt.Errorf("validate semantic embeddings: %w", err)
	}
	return nil
}

// RunSemanticReindex triggers a reindex of dirty KG nodes and edges into the
// semantic vector index. It is safe to call concurrently with searches.
func (kg *KnowledgeGraph) RunSemanticReindex() error {
	err := kg.reindexSemanticNodes()
	if err == nil {
		kg.markSemanticReindexComplete()
	}
	return err
}

// DrainSemanticReindexBacklog runs RunSemanticReindex up to maxPasses times while
// dirty semantic rows remain. It is intended for maintenance follow-up passes.
func (kg *KnowledgeGraph) DrainSemanticReindexBacklog(maxPasses int) (int, error) {
	if kg == nil || kg.semanticIndex() == nil || maxPasses <= 0 {
		return 0, nil
	}

	passes := 0
	for pass := 0; pass < maxPasses; pass++ {
		backlog, _, _, err := kg.HasSemanticReindexBacklog()
		if err != nil {
			return passes, err
		}
		if !backlog {
			break
		}
		if err := kg.RunSemanticReindex(); err != nil {
			return passes, err
		}
		passes++
	}
	return passes, nil
}

// RunSemanticReindexIfDue runs RunSemanticReindex only when the configured
// semantic_reindex_interval has elapsed since the last successful reindex.
// It returns whether a reindex was attempted.
func (kg *KnowledgeGraph) RunSemanticReindexIfDue() (bool, error) {
	if kg.semanticIndex() == nil {
		return false, nil
	}

	kg.lastSemanticReindexMu.Lock()
	interval := kg.semanticReindexInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	due := kg.lastSemanticReindex.IsZero() || time.Since(kg.lastSemanticReindex) >= interval
	kg.lastSemanticReindexMu.Unlock()
	if !due {
		return false, nil
	}

	if err := kg.RunSemanticReindex(); err != nil {
		return true, err
	}
	return true, nil
}

func (kg *KnowledgeGraph) reindexSemanticNodes() error {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil
	}

	now := time.Now().Format(time.RFC3339)

	// Load dirty nodes under reindexMu, then release before expensive embedding I/O.
	idx.reindexMu.Lock()
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at
		LIMIT 5000
	`)
	var nodes []Node
	if err == nil {
		for rows.Next() {
			var n Node
			var propsJSON string
			var protected int
			if rows.Scan(&n.ID, &n.Label, &propsJSON, &protected) == nil {
				n.Properties = decodeKnowledgeGraphNodeProperties(idx.logger, "reindex", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				nodes = append(nodes, n)
			}
		}
		rows.Close()
	}
	idx.reindexMu.Unlock()
	if err != nil {
		return fmt.Errorf("load dirty nodes for semantic reindex: %w", err)
	}

	var indexedNodeIDs []string
	for _, node := range nodes {
		if kg.upsertSemanticNodeIndex(node) {
			indexedNodeIDs = append(indexedNodeIDs, node.ID)
		}
	}
	if len(indexedNodeIDs) > 0 && idx.logger != nil {
		idx.logger.Info("KG semantic reindex: nodes indexed", "count", len(indexedNodeIDs))
	}
	if len(indexedNodeIDs) > 0 {
		idx.reindexMu.Lock()
		placeholders := strings.Repeat("?,", len(indexedNodeIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]interface{}, 0, len(indexedNodeIDs)+1)
		args = append(args, now)
		for _, id := range indexedNodeIDs {
			args = append(args, id)
		}
		_, _ = kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = ? WHERE id IN (`+placeholders+`)`, args...)
		idx.reindexMu.Unlock()
	}

	// Load dirty edges under reindexMu, then release before expensive embedding I/O.
	idx.reindexMu.Lock()
	edgeRows, err := kg.db.Query(`
		SELECT e.source, e.target, e.relation, e.properties
		FROM kg_edges e
		WHERE `+knowledgeGraphSemanticEdgeDirtyCondition("e")+`
		LIMIT 5000
	`)
	var edges []Edge
	if err == nil {
		for edgeRows.Next() {
			var e Edge
			var propsJSON string
			if edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON) == nil {
				json.Unmarshal([]byte(propsJSON), &e.Properties)
				if e.Properties == nil {
					e.Properties = make(map[string]string)
				}
				edges = append(edges, e)
			}
		}
		edgeRows.Close()
	}
	idx.reindexMu.Unlock()
	if err != nil {
		if idx.logger != nil {
			idx.logger.Warn("reindexSemanticNodes: edge query failed", "error", err)
		}
	} else {
		for _, edge := range edges {
			kg.upsertSemanticEdgeIndex(edge)
		}
		if len(edges) > 0 {
			idx.reindexMu.Lock()
			if err := kg.markSemanticEdgesIndexedAt(edges, now); err != nil && idx.logger != nil {
				idx.logger.Warn("reindexSemanticNodes: failed to mark edges indexed", "error", err)
			}
			idx.reindexMu.Unlock()
		}
	}

	return nil
}

func knowledgeGraphSemanticEdgeDirtyCondition(edgeAlias string) string {
	return fmt.Sprintf(`(
		%s.semantic_indexed_at IS NULL
		OR %s.semantic_indexed_at < COALESCE(%s.updated_at, '1970-01-01')
		OR EXISTS (
			SELECT 1 FROM kg_nodes n
			WHERE (n.id = %s.source OR n.id = %s.target)
			  AND n.updated_at > COALESCE(%s.semantic_indexed_at, '1970-01-01')
		)
	)`, edgeAlias, edgeAlias, edgeAlias, edgeAlias, edgeAlias, edgeAlias)
}

func (kg *KnowledgeGraph) markSemanticEdgesIndexedAt(edges []Edge, indexedAt string) error {
	if kg == nil || kg.db == nil || len(edges) == 0 {
		return nil
	}
	for start := 0; start < len(edges); start += knowledgeGraphSemanticEdgeReindexBatchSize {
		end := start + knowledgeGraphSemanticEdgeReindexBatchSize
		if end > len(edges) {
			end = len(edges)
		}
		chunk := edges[start:end]
		tx, err := kg.db.Begin()
		if err != nil {
			return fmt.Errorf("begin semantic edge batch update: %w", err)
		}
		if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS kg_semantic_edge_batch (
			source TEXT NOT NULL,
			target TEXT NOT NULL,
			relation TEXT NOT NULL,
			PRIMARY KEY (source, target, relation)
		)`); err != nil {
			tx.Rollback()
			return fmt.Errorf("create semantic edge batch table: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM kg_semantic_edge_batch`); err != nil {
			tx.Rollback()
			return fmt.Errorf("reset semantic edge batch table: %w", err)
		}
		stmt, err := tx.Prepare(`INSERT INTO kg_semantic_edge_batch (source, target, relation) VALUES (?, ?, ?)`)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("prepare semantic edge batch insert: %w", err)
		}
		for _, edge := range chunk {
			if _, err := stmt.Exec(edge.Source, edge.Target, edge.Relation); err != nil {
				stmt.Close()
				tx.Rollback()
				return fmt.Errorf("insert semantic edge batch row: %w", err)
			}
		}
		stmt.Close()
		if _, err := tx.Exec(`
			UPDATE kg_edges
			SET semantic_indexed_at = ?
			WHERE EXISTS (
				SELECT 1
				FROM kg_semantic_edge_batch b
				WHERE b.source = kg_edges.source
				  AND b.target = kg_edges.target
				  AND b.relation = kg_edges.relation
			)
		`, indexedAt); err != nil {
			tx.Rollback()
			return fmt.Errorf("update semantic edge batch timestamps: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit semantic edge batch update: %w", err)
		}
	}
	return nil
}

func (kg *KnowledgeGraph) upsertSemanticNodeIndex(node Node) bool {
	idx := kg.semanticIndex()
	if idx == nil || !shouldIndexKnowledgeGraphNode(node) {
		return true
	}

	content := buildKnowledgeGraphSemanticContent(node)
	if content == "" {
		return true
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	err := kg.retrySemanticEmbedding("node_upsert", func(ctx context.Context) error {
		return idx.collection.AddDocument(ctx, chromem.Document{
			ID:      node.ID,
			Content: content,
			Metadata: map[string]string{
				"node_id": node.ID,
				"label":   node.Label,
			},
		})
	})
	if err != nil {
		if idx.logger != nil {
			idx.logger.Warn("KG semantic node index update failed", "node_id", node.ID, "error", err)
		}
		return false
	}
	idx.setContentCacheEntry(node.ID, content)
	return true
}

func (idx *knowledgeGraphSemanticIndex) setContentCacheEntry(nodeID, content string) {
	if idx.contentCache == nil {
		idx.contentCache = make(map[string]string)
	}
	if _, exists := idx.contentCache[nodeID]; !exists {
		idx.contentCacheKeys = append(idx.contentCacheKeys, nodeID)
	}
	idx.contentCache[nodeID] = content
	idx.trimContentCache()
}

func (idx *knowledgeGraphSemanticIndex) removeContentCacheEntry(nodeID string) {
	delete(idx.contentCache, nodeID)
	for i, key := range idx.contentCacheKeys {
		if key == nodeID {
			idx.contentCacheKeys = append(idx.contentCacheKeys[:i], idx.contentCacheKeys[i+1:]...)
			return
		}
	}
}

func (idx *knowledgeGraphSemanticIndex) trimContentCache() {
	if len(idx.contentCache) <= knowledgeGraphSemanticContentCacheMaxSize {
		return
	}
	removeCount := len(idx.contentCache) / 5
	if removeCount < 1 {
		removeCount = 1
	}
	for i := 0; i < removeCount && len(idx.contentCacheKeys) > 0; i++ {
		oldestID := idx.contentCacheKeys[0]
		idx.contentCacheKeys = idx.contentCacheKeys[1:]
		delete(idx.contentCache, oldestID)
	}
}

func (kg *KnowledgeGraph) upsertSemanticEdgeIndex(edge Edge) {
	idx := kg.semanticIndex()
	if idx == nil {
		return
	}
	content := buildKnowledgeGraphEdgeSemanticContent(edge)
	if content == "" {
		return
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	edgeDocID := "edge://" + edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
	err := kg.retrySemanticEmbedding("edge_upsert", func(ctx context.Context) error {
		return idx.collection.AddDocument(ctx, chromem.Document{
			ID:      edgeDocID,
			Content: content,
			Metadata: map[string]string{
				"source":   edge.Source,
				"target":   edge.Target,
				"relation": edge.Relation,
			},
		})
	})
	if err != nil && idx.logger != nil {
		idx.logger.Warn("KG semantic edge index update failed", "source", edge.Source, "target", edge.Target, "error", err)
	}
}

// removeSemanticNodeIndex removes a node's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticNodeIndex(nodeID string) error {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeContentCacheEntry(nodeID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return idx.collection.Delete(ctx, nil, nil, nodeID)
}

// removeSemanticEdgeIndex removes an edge's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticEdgeIndex(source, target, relation string) error {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()

	edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return idx.collection.Delete(ctx, nil, nil, edgeDocID)
}

func buildKnowledgeGraphEdgeSemanticContent(edge Edge) string {
	var parts []string
	if strings.TrimSpace(edge.Relation) != "" {
		parts = append(parts, edge.Relation)
	}
	srcLabel := strings.TrimSpace(edge.Source)
	tgtLabel := strings.TrimSpace(edge.Target)
	if srcLabel != "" && tgtLabel != "" {
		parts = append(parts, srcLabel+" "+edge.Relation+" "+tgtLabel)
	}
	keys := make([]string, 0, len(edge.Properties))
	for key := range edge.Properties {
		switch key {
		case "source", "extracted_at", "last_seen", "session_id", "date", "channel", "protected":
			continue
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(edge.Properties[key])
		if value != "" {
			parts = append(parts, key+": "+value)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ". "))
}

func (kg *KnowledgeGraph) semanticSearchNodes(query string, minSim float32, maxNodes int) []Node {
	idx := kg.semanticIndex()
	if idx == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	var out []Node
	for _, result := range results {
		if result.Similarity < minSim {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			// Resolve full node from SQLite to return Node struct
			if n, err := kg.GetNode(result.ID); err == nil && n != nil {
				if kg.isExcludedNodeType(n.Properties["type"]) {
					continue
				}
				out = append(out, *n)
			}
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchNodeIDs(query string, maxNodes int) []string {
	idx := kg.semanticIndex()
	if idx == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	minSim := kg.getMinSemanticSimilarity()
	candidateIDs := make([]string, 0, len(results))
	for _, result := range results {
		if result.Similarity < minSim {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			candidateIDs = append(candidateIDs, result.ID)
		}
	}
	allowed := kg.filterExcludedKnowledgeGraphNodeTypes(candidateIDs)
	out := make([]string, 0, len(allowed))
	for _, id := range candidateIDs {
		if _, ok := allowed[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchNodeScores(query string, maxNodes int) map[string]float32 {
	idx := kg.semanticIndex()
	if idx == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.logger != nil {
			idx.logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	minSim := kg.getMinSemanticSimilarity()
	type candidateHit struct {
		id       string
		similarity float32
	}
	var candidates []candidateHit
	for _, result := range results {
		if result.Similarity < minSim {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			candidates = append(candidates, candidateHit{id: result.ID, similarity: result.Similarity})
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	candidateIDs := make([]string, len(candidates))
	for i, c := range candidates {
		candidateIDs[i] = c.id
	}
	allowed := kg.filterExcludedKnowledgeGraphNodeTypes(candidateIDs)

	out := make(map[string]float32, len(allowed))
	for _, c := range candidates {
		if _, ok := allowed[c.id]; ok {
			out[c.id] = c.similarity
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchEdgeIDs(query string, maxEdges int) []string {
	idx := kg.semanticIndex()
	if idx == nil || maxEdges <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}
	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count := idx.collection.Count()
	if count == 0 {
		return nil
	}
	if maxEdges > count {
		maxEdges = count
	}
	results, err := idx.collection.QueryEmbedding(ctx, embedding, maxEdges, nil, nil)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(results))
	for _, result := range results {
		if result.Similarity < knowledgeGraphSemanticEdgeMinSimilarity {
			continue
		}
		if strings.HasPrefix(result.ID, "edge://") {
			out = append(out, result.ID)
		}
	}
	return out
}

func (kg *KnowledgeGraph) getSemanticQueryEmbedding(query string) ([]float32, error) {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil, fmt.Errorf("semantic search is disabled")
	}

	// Fast path: check cache under read-lock
	idx.mu.Lock()
	if entry, ok := idx.queryCache[query]; ok && time.Since(entry.timestamp) < idx.queryCacheTTL {
		embedding := cloneFloat32Slice(entry.embedding)
		idx.mu.Unlock()
		return embedding, nil
	}
	idx.mu.Unlock()

	// Slow path: call embedding API WITHOUT holding the lock to avoid blocking other goroutines.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	embedding, err := idx.embeddingFunc(ctx, query)
	if err != nil {
		return nil, err
	}

	// Store result under lock; re-check cache in case another goroutine raced us.
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if existing, ok := idx.queryCache[query]; ok && time.Since(existing.timestamp) < idx.queryCacheTTL {
		return cloneFloat32Slice(existing.embedding), nil
	}
	cachedEmbedding := cloneFloat32Slice(embedding)
	idx.queryCache[query] = queryCacheEntry{embedding: cachedEmbedding, timestamp: time.Now()}
	if len(idx.queryCache) > knowledgeGraphSemanticQueryCacheMaxSize {
		now := time.Now()
		var toDelete []string
		var oldestKey string
		var oldestTime time.Time
		for k, v := range idx.queryCache {
			if now.Sub(v.timestamp) > idx.queryCacheTTL {
				toDelete = append(toDelete, k)
			} else if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		if len(toDelete) > 0 {
			for _, k := range toDelete {
				delete(idx.queryCache, k)
			}
		} else if oldestKey != "" {
			delete(idx.queryCache, oldestKey)
		}
	}
	return cloneFloat32Slice(cachedEmbedding), nil
}

func buildKnowledgeGraphSemanticContent(node Node) string {
	var parts []string
	if strings.TrimSpace(node.Label) != "" {
		parts = append(parts, node.Label)
	}

	keys := make([]string, 0, len(node.Properties))
	for key := range node.Properties {
		switch key {
		case "source", "extracted_at", "last_seen", "session_id", "date", "channel", "protected":
			continue
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(node.Properties[key])
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func shouldIndexKnowledgeGraphNode(node Node) bool {
	if strings.TrimSpace(node.ID) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(node.Label), "Unknown") {
		return false
	}
	return buildKnowledgeGraphSemanticContent(node) != ""
}

func shouldSkipKnowledgeGraphSemanticQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" || query == "*" {
		return true
	}
	runeLen := len([]rune(query))
	if runeLen >= 8 {
		return false
	}
	if runeLen < 2 {
		return true
	}
	if runeLen >= 2 && looksLikeCompactEntityQuery(query) {
		return false
	}
	if runeLen >= 3 && looksLikeKnowledgeGraphSlug(query) {
		return false
	}
	return true
}

func looksLikeKnowledgeGraphSlug(query string) bool {
	for _, r := range query {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// filterExcludedKnowledgeGraphNodeTypes returns the subset of ids whose node_type
// is not considered low-signal noise (e.g. activity_entity, unknown).
func (kg *KnowledgeGraph) filterExcludedKnowledgeGraphNodeTypes(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		"SELECT id, json_extract(properties, '$.type') FROM kg_nodes WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)
	rows, err := kg.db.Query(query, args...)
	if err != nil {
		if idx := kg.semanticIndex(); idx != nil && idx.logger != nil {
			idx.logger.Warn("filterExcludedKnowledgeGraphNodeTypes: query failed", "error", err)
		}
		return nil
	}
	defer rows.Close()

	allowed := make(map[string]struct{}, len(ids))
	for rows.Next() {
		var id, nodeType string
		if err := rows.Scan(&id, &nodeType); err != nil {
			continue
		}
		if !kg.isExcludedNodeType(nodeType) {
			allowed[id] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		if idx := kg.semanticIndex(); idx != nil && idx.logger != nil {
			idx.logger.Warn("filterExcludedKnowledgeGraphNodeTypes: row iteration failed", "error", err)
		}
	}
	return allowed
}

func looksLikeCompactEntityQuery(query string) bool {
	hasUpperOrDigit := false
	for _, r := range query {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpperOrDigit = true
		case r >= '0' && r <= '9':
			hasUpperOrDigit = true
		case r >= 'a' && r <= 'z':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return hasUpperOrDigit
}

func shouldRetrySemanticEmbeddingErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	// Best-effort handling for provider-side throttling / transient upstream issues.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return true
	}
	if strings.Contains(msg, " 429 ") || strings.Contains(msg, "429") {
		return true
	}
	if strings.Contains(msg, " 5") && strings.Contains(msg, "http") {
		return true
	}
	return false
}

func (kg *KnowledgeGraph) retrySemanticEmbedding(op string, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= knowledgeGraphSemanticRetryMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphSemanticTimeout)
		err := fn(ctx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == knowledgeGraphSemanticRetryMaxAttempts || !shouldRetrySemanticEmbeddingErr(err) {
			return err
		}

		backoff := time.Duration(attempt*attempt) * knowledgeGraphSemanticRetryBackoffBase
		if idx := kg.semanticIndex(); idx != nil && idx.logger != nil {
			idx.logger.Debug("KG semantic embedding op failed; retrying", "op", op, "attempt", attempt, "backoff", backoff, "error", err)
		}
		time.Sleep(backoff)
	}
	return lastErr
}

const knowledgeGraphSemanticReindexBacklogLimit = 5000

// ConsistencyCheck verifies that the KG semantic index is in sync with SQLite nodes/edges.
type KGConsistencyReport struct {
	NodesMissingFromIndex int  `json:"nodes_missing_from_index"`
	EdgesMissingFromIndex int  `json:"edges_missing_from_index"`
	DirtyNodes            int  `json:"dirty_nodes"`
	DirtyEdges            int  `json:"dirty_edges"`
	StaleNodes            int  `json:"stale_nodes"`
	IndexOrphans          int  `json:"index_orphans"`
	TotalNodes            int  `json:"total_nodes"`
	TotalEdges            int  `json:"total_edges"`
	TotalIndexed          uint `json:"total_indexed"`
	SemanticEnabled       bool `json:"semantic_enabled"`
	Sampled               bool `json:"sampled"`
	SampleSize            int  `json:"sample_size,omitempty"`
	NeedsReindex          bool `json:"needs_reindex"`
	ReindexBacklog        bool `json:"reindex_backlog"`
}

// KnowledgeGraphHealthReport summarizes KG runtime health for dashboards and operators.
type KnowledgeGraphHealthReport struct {
	SemanticEnabled   bool                 `json:"semantic_enabled"`
	DirtyNodes        int                  `json:"dirty_nodes"`
	DirtyEdges        int                  `json:"dirty_edges"`
	DroppedAccessHits int64                `json:"dropped_access_hits"`
	NeedsReindex      bool                 `json:"needs_reindex"`
	ReindexBacklog    bool                 `json:"reindex_backlog"`
	TotalNodes        int                  `json:"total_nodes"`
	TotalEdges        int                  `json:"total_edges"`
	IsolatedNodes          int                  `json:"isolated_nodes"`
	DuplicateGroups        int                  `json:"duplicate_groups"`
	LabelDuplicateGroups   int                  `json:"label_duplicate_groups"`
	IDDuplicateGroups      int                  `json:"id_duplicate_groups"`
	Consistency            *KGConsistencyReport `json:"consistency,omitempty"`
}

// SemanticSearchEnabled reports whether the KG semantic index is active.
func (kg *KnowledgeGraph) SemanticSearchEnabled() bool {
	return kg != nil && kg.semanticIndex() != nil
}

// DirtySemanticCounts returns how many KG nodes and edges still need semantic reindexing.
func (kg *KnowledgeGraph) DirtySemanticCounts() (nodes int, edges int, err error) {
	if kg == nil || kg.db == nil {
		return 0, 0, fmt.Errorf("knowledge graph not initialized")
	}
	nodes, err = kg.countDirtySemanticNodes()
	if err != nil {
		return 0, 0, err
	}
	edges, err = kg.countDirtySemanticEdges()
	return nodes, edges, err
}

// HasSemanticReindexBacklog reports whether dirty semantic rows exceed the nightly reindex batch size.
func (kg *KnowledgeGraph) HasSemanticReindexBacklog() (bool, int, int, error) {
	nodes, edges, err := kg.DirtySemanticCounts()
	if err != nil {
		return false, 0, 0, err
	}
	return nodes > knowledgeGraphSemanticReindexBacklogLimit || edges > knowledgeGraphSemanticReindexBacklogLimit, nodes, edges, nil
}

func (kg *KnowledgeGraph) countDirtySemanticNodes() (int, error) {
	var count int
	err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes
		WHERE semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at
	`).Scan(&count)
	return count, err
}

func (kg *KnowledgeGraph) countDirtySemanticEdges() (int, error) {
	var count int
	err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges e
		WHERE `+knowledgeGraphSemanticEdgeDirtyCondition("e"),
	).Scan(&count)
	return count, err
}

func (kg *KnowledgeGraph) HealthReport() (*KnowledgeGraphHealthReport, error) {
	if kg == nil || kg.db == nil {
		return nil, fmt.Errorf("knowledge graph not initialized")
	}

	report := &KnowledgeGraphHealthReport{
		SemanticEnabled:   kg.semanticIndex() != nil,
		DroppedAccessHits: kg.DroppedAccessHits(),
	}
	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.TotalNodes)
	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.TotalEdges)

	isolatedNodes, err := countKnowledgeGraphIsolatedNodes(kg.db)
	if err != nil {
		return nil, fmt.Errorf("count isolated knowledge graph nodes: %w", err)
	}
	report.IsolatedNodes = isolatedNodes

	labelDuplicateGroups, err := countKnowledgeGraphLabelDuplicateGroups(kg.db)
	if err != nil {
		return nil, fmt.Errorf("count label duplicate groups: %w", err)
	}
	idDuplicateGroups, err := countKnowledgeGraphIDDuplicateGroups(kg.db, kg.logger)
	if err != nil {
		return nil, fmt.Errorf("count id duplicate groups: %w", err)
	}
	report.LabelDuplicateGroups = labelDuplicateGroups
	report.IDDuplicateGroups = idDuplicateGroups
	report.DuplicateGroups = labelDuplicateGroups + idDuplicateGroups

	dirtyNodes, err := kg.countDirtySemanticNodes()
	if err != nil {
		return nil, fmt.Errorf("count dirty semantic nodes: %w", err)
	}
	report.DirtyNodes = dirtyNodes

	dirtyEdges, err := kg.countDirtySemanticEdges()
	if err != nil {
		return nil, fmt.Errorf("count dirty semantic edges: %w", err)
	}
	report.DirtyEdges = dirtyEdges

	report.ReindexBacklog = dirtyNodes > knowledgeGraphSemanticReindexBacklogLimit || dirtyEdges > knowledgeGraphSemanticReindexBacklogLimit
	report.NeedsReindex = dirtyNodes > 0 || dirtyEdges > 0

	if kg.semanticIndex() != nil {
		consistency, err := kg.ConsistencyCheckSample(knowledgeGraphConsistencyCheckSampleSize)
		if err != nil {
			return nil, err
		}
		report.Consistency = consistency
		report.NeedsReindex = consistency.NeedsReindex || report.NeedsReindex
		report.ReindexBacklog = consistency.ReindexBacklog || report.ReindexBacklog
	}

	return report, nil
}

// ConsistencyCheck performs a full semantic index consistency scan.
func (kg *KnowledgeGraph) ConsistencyCheck() (*KGConsistencyReport, error) {
	return kg.ConsistencyCheckSample(0)
}

// ConsistencyCheckSample verifies semantic index sync. sampleSize <= 0 scans all
// indexed rows; sampleSize > 0 limits expensive vector lookups to a stable subset.
func (kg *KnowledgeGraph) ConsistencyCheckSample(sampleSize int) (*KGConsistencyReport, error) {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil, fmt.Errorf("semantic search is disabled")
	}

	report := &KGConsistencyReport{
		SemanticEnabled: true,
	}
	if sampleSize > 0 {
		report.Sampled = true
		report.SampleSize = sampleSize
	}

	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.TotalNodes)
	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&report.TotalEdges)
	report.TotalIndexed = uint(idx.collection.Count())

	dirtyNodes, err := kg.countDirtySemanticNodes()
	if err != nil {
		return nil, fmt.Errorf("count dirty semantic nodes: %w", err)
	}
	report.DirtyNodes = dirtyNodes
	report.StaleNodes = dirtyNodes

	dirtyEdges, err := kg.countDirtySemanticEdges()
	if err != nil {
		return nil, fmt.Errorf("count dirty semantic edges: %w", err)
	}
	report.DirtyEdges = dirtyEdges

	nodeQuery := `
		SELECT id
		FROM kg_nodes
		WHERE NOT (semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at)
		ORDER BY id`
	nodeArgs := []interface{}(nil)
	if sampleSize > 0 {
		nodeQuery += ` LIMIT ?`
		nodeArgs = append(nodeArgs, sampleSize)
	}
	rows, err := kg.db.Query(nodeQuery, nodeArgs...)
	if err == nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) != nil {
				continue
			}
			if _, getErr := idx.collection.GetByID(context.Background(), id); getErr != nil {
				report.NodesMissingFromIndex++
			}
		}
		rows.Close()
	}

	edgeQuery := `
		SELECT e.source, e.target, e.relation
		FROM kg_edges e
		WHERE NOT ` + knowledgeGraphSemanticEdgeDirtyCondition("e") + `
		ORDER BY e.source, e.target, e.relation`
	edgeArgs := []interface{}(nil)
	if sampleSize > 0 {
		edgeQuery += ` LIMIT ?`
		edgeArgs = append(edgeArgs, sampleSize)
	}
	edgeRows, err := kg.db.Query(edgeQuery, edgeArgs...)
	if err == nil {
		for edgeRows.Next() {
			var source, target, relation string
			if edgeRows.Scan(&source, &target, &relation) != nil {
				continue
			}
			edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
			if _, getErr := idx.collection.GetByID(context.Background(), edgeDocID); getErr != nil {
				report.EdgesMissingFromIndex++
			}
		}
		edgeRows.Close()
	}

	report.ReindexBacklog = report.DirtyNodes > knowledgeGraphSemanticReindexBacklogLimit || report.DirtyEdges > knowledgeGraphSemanticReindexBacklogLimit
	report.NeedsReindex = report.DirtyNodes > 0 || report.DirtyEdges > 0 || report.NodesMissingFromIndex > 50 || report.EdgesMissingFromIndex > 50

	return report, nil
}
