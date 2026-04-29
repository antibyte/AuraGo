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
const knowledgeGraphSemanticMinSimilarity = 0.45

const knowledgeGraphSemanticEdgeMinSimilarity = 0.35

const knowledgeGraphSemanticQueryCacheTTL = 5 * time.Minute
const knowledgeGraphSemanticEdgeMaxResults = 50
const knowledgeGraphSemanticQueryCacheMaxSize = 100

const knowledgeGraphSemanticRetryMaxAttempts = 3
const knowledgeGraphSemanticRetryBackoffBase = 250 * time.Millisecond

type knowledgeGraphSemanticIndex struct {
	collection    *chromem.Collection
	embeddingFunc chromem.EmbeddingFunc
	logger        *slog.Logger
	mu            sync.Mutex
	reindexMu     sync.Mutex
	queryCache    map[string]queryCacheEntry
	queryCacheTTL time.Duration
	contentCache  map[string]string
}

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
	kg.logger.Warn("EnableSemanticSearch: non-shared path is deprecated, use EnableSemanticSearchShared instead; skipping")
	return nil
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
		contentCache:  make(map[string]string),
	}

	if err := kg.validateSemanticIndex(index); err != nil {
		return err
	}
	kg.semantic = index
	return kg.reindexSemanticNodes()
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

func (kg *KnowledgeGraph) reindexSemanticNodes() error {
	if kg.semantic == nil {
		return nil
	}

	now := time.Now().Format(time.RFC3339)

	// Load dirty nodes under reindexMu, then release before expensive embedding I/O.
	kg.semantic.reindexMu.Lock()
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
				n.Properties = decodeKnowledgeGraphNodeProperties(kg.semantic.logger, "reindex", n.ID, propsJSON, protected)
				n.Protected = protected != 0
				nodes = append(nodes, n)
			}
		}
		rows.Close()
	}
	kg.semantic.reindexMu.Unlock()
	if err != nil {
		return fmt.Errorf("load dirty nodes for semantic reindex: %w", err)
	}

	var indexedNodeIDs []string
	for _, node := range nodes {
		if kg.upsertSemanticNodeIndex(node) {
			indexedNodeIDs = append(indexedNodeIDs, node.ID)
		}
	}
	if len(indexedNodeIDs) > 0 && kg.semantic.logger != nil {
		kg.semantic.logger.Info("KG semantic reindex: nodes indexed", "count", len(indexedNodeIDs))
	}
	if len(indexedNodeIDs) > 0 {
		kg.semantic.reindexMu.Lock()
		placeholders := strings.Repeat("?,", len(indexedNodeIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]interface{}, 0, len(indexedNodeIDs)+1)
		args = append(args, now)
		for _, id := range indexedNodeIDs {
			args = append(args, id)
		}
		_, _ = kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = ? WHERE id IN (`+placeholders+`)`, args...)
		kg.semantic.reindexMu.Unlock()
	}

	// Load dirty edges under reindexMu, then release before expensive embedding I/O.
	kg.semantic.reindexMu.Lock()
	edgeRows, err := kg.db.Query(`
		SELECT e.source, e.target, e.relation, e.properties
		FROM kg_edges e
		LEFT JOIN (
			SELECT e2.source, e2.target, e2.relation, MAX(n.updated_at) AS max_updated_at
			FROM kg_edges e2
			JOIN kg_nodes n ON n.id = e2.source OR n.id = e2.target
			GROUP BY e2.source, e2.target, e2.relation
		) node_updates ON e.source = node_updates.source AND e.target = node_updates.target AND e.relation = node_updates.relation
		WHERE e.semantic_indexed_at IS NULL
		   OR e.semantic_indexed_at < COALESCE(e.updated_at, '1970-01-01')
		   OR e.semantic_indexed_at < COALESCE(node_updates.max_updated_at, '1970-01-01')
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
	kg.semantic.reindexMu.Unlock()
	if err != nil {
		kg.semantic.logger.Warn("reindexSemanticNodes: edge query failed", "error", err)
	} else {
		for _, edge := range edges {
			kg.upsertSemanticEdgeIndex(edge)
		}
		if len(edges) > 0 {
			kg.semantic.reindexMu.Lock()
			edgePlaceholders := make([]string, len(edges))
			edgeArgs := make([]interface{}, 0, len(edges)+1)
			edgeArgs = append(edgeArgs, now)
			for i, edge := range edges {
				edgePlaceholders[i] = "(? IS NOT NULL AND source = ? AND target = ? AND relation = ?)"
				edgeArgs = append(edgeArgs, now, edge.Source, edge.Target, edge.Relation)
			}
			// Update both NULL and stale edges by matching their exact identity
			_, _ = kg.db.Exec(
				`UPDATE kg_edges SET semantic_indexed_at = ? WHERE `+strings.Join(edgePlaceholders, " OR "),
				edgeArgs...,
			)
			kg.semantic.reindexMu.Unlock()
		}
	}

	return nil
}

func (kg *KnowledgeGraph) upsertSemanticNodeIndex(node Node) bool {
	if kg.semantic == nil || !shouldIndexKnowledgeGraphNode(node) {
		return true
	}

	content := buildKnowledgeGraphSemanticContent(node)
	if content == "" {
		return true
	}

	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	err := kg.retrySemanticEmbedding("node_upsert", func(ctx context.Context) error {
		return kg.semantic.collection.AddDocument(ctx, chromem.Document{
			ID:      node.ID,
			Content: content,
			Metadata: map[string]string{
				"node_id": node.ID,
				"label":   node.Label,
			},
		})
	})
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Warn("KG semantic node index update failed", "node_id", node.ID, "error", err)
		}
		return false
	}
	kg.semantic.contentCache[node.ID] = content
	if len(kg.semantic.contentCache) > 5000 {
		kg.semantic.contentCache = make(map[string]string)
	}
	return true
}

func (kg *KnowledgeGraph) upsertSemanticEdgeIndex(edge Edge) {
	if kg.semantic == nil {
		return
	}
	content := buildKnowledgeGraphEdgeSemanticContent(edge)
	if content == "" {
		return
	}

	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	edgeDocID := "edge://" + edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
	err := kg.retrySemanticEmbedding("edge_upsert", func(ctx context.Context) error {
		return kg.semantic.collection.AddDocument(ctx, chromem.Document{
			ID:      edgeDocID,
			Content: content,
			Metadata: map[string]string{
				"source":   edge.Source,
				"target":   edge.Target,
				"relation": edge.Relation,
			},
		})
	})
	if err != nil && kg.semantic.logger != nil {
		kg.semantic.logger.Warn("KG semantic edge index update failed", "source", edge.Source, "target", edge.Target, "error", err)
	}
}

// removeSemanticNodeIndex removes a node's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticNodeIndex(nodeID string) error {
	if kg.semantic == nil {
		return nil
	}
	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	delete(kg.semantic.contentCache, nodeID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return kg.semantic.collection.Delete(ctx, nil, nil, nodeID)
}

// removeSemanticEdgeIndex removes an edge's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticEdgeIndex(source, target, relation string) error {
	if kg.semantic == nil {
		return nil
	}
	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return kg.semantic.collection.Delete(ctx, nil, nil, edgeDocID)
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
	if kg.semantic == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := kg.semantic.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := kg.semantic.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic search failed", "error", err)
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
				out = append(out, *n)
			}
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchNodeIDs(query string, maxNodes int) []string {
	if kg.semantic == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := kg.semantic.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := kg.semantic.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	out := make([]string, 0, len(results))
	for _, result := range results {
		if result.Similarity < knowledgeGraphSemanticMinSimilarity {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			out = append(out, result.ID)
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchNodeScores(query string, maxNodes int) map[string]float32 {
	if kg.semantic == nil || maxNodes <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := kg.semantic.collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := kg.semantic.collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	out := make(map[string]float32)
	for _, result := range results {
		if result.Similarity < knowledgeGraphSemanticMinSimilarity {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			out[result.ID] = result.Similarity
		}
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchEdgeIDs(query string, maxEdges int) []string {
	if kg.semantic == nil || maxEdges <= 0 || shouldSkipKnowledgeGraphSemanticQuery(query) {
		return nil
	}
	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count := kg.semantic.collection.Count()
	if count == 0 {
		return nil
	}
	if maxEdges > count {
		maxEdges = count
	}
	results, err := kg.semantic.collection.QueryEmbedding(ctx, embedding, maxEdges, nil, nil)
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
	if kg.semantic == nil {
		return nil, fmt.Errorf("semantic search is disabled")
	}

	// Fast path: check cache under read-lock
	kg.semantic.mu.Lock()
	if entry, ok := kg.semantic.queryCache[query]; ok && time.Since(entry.timestamp) < kg.semantic.queryCacheTTL {
		embedding := entry.embedding
		kg.semantic.mu.Unlock()
		return embedding, nil
	}
	kg.semantic.mu.Unlock()

	// Slow path: call embedding API WITHOUT holding the lock to avoid blocking other goroutines.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	embedding, err := kg.semantic.embeddingFunc(ctx, query)
	if err != nil {
		return nil, err
	}

	// Store result under lock; re-check cache in case another goroutine raced us.
	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()
	if existing, ok := kg.semantic.queryCache[query]; ok && time.Since(existing.timestamp) < kg.semantic.queryCacheTTL {
		return existing.embedding, nil
	}
	kg.semantic.queryCache[query] = queryCacheEntry{embedding: embedding, timestamp: time.Now()}
	if len(kg.semantic.queryCache) > knowledgeGraphSemanticQueryCacheMaxSize {
		now := time.Now()
		var toDelete []string
		var oldestKey string
		var oldestTime time.Time
		for k, v := range kg.semantic.queryCache {
			if now.Sub(v.timestamp) > kg.semantic.queryCacheTTL {
				toDelete = append(toDelete, k)
			} else if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		if len(toDelete) > 0 {
			for _, k := range toDelete {
				delete(kg.semantic.queryCache, k)
			}
		} else if oldestKey != "" {
			delete(kg.semantic.queryCache, oldestKey)
		}
	}
	return embedding, nil
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
	if runeLen >= 2 && looksLikeCompactEntityQuery(query) {
		return false
	}
	return true
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
		if kg.semantic != nil && kg.semantic.logger != nil {
			kg.semantic.logger.Debug("KG semantic embedding op failed; retrying", "op", op, "attempt", attempt, "backoff", backoff, "error", err)
		}
		time.Sleep(backoff)
	}
	return lastErr
}

// ConsistencyCheck verifies that the KG semantic index is in sync with SQLite nodes/edges.
// Returns counts of detected drift and an error if significant drift is found.
type KGConsistencyReport struct {
	NodesMissingFromIndex int  `json:"nodes_missing_from_index"`
	EdgesMissingFromIndex int  `json:"edges_missing_from_index"`
	StaleNodes            int  `json:"stale_nodes"`
	IndexOrphans          int  `json:"index_orphans"`
	TotalNodes            int  `json:"total_nodes"`
	TotalIndexed          uint `json:"total_indexed"`
	NeedsReindex          bool `json:"needs_reindex"`
}

func (kg *KnowledgeGraph) ConsistencyCheck() (*KGConsistencyReport, error) {
	if kg.semantic == nil {
		return nil, fmt.Errorf("semantic search is disabled")
	}

	report := &KGConsistencyReport{}

	_ = kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&report.TotalNodes)
	report.TotalIndexed = uint(kg.semantic.collection.Count())

	rows, err := kg.db.Query(`
		SELECT id, semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at AS dirty
		FROM kg_nodes
	`)
	if err == nil {
		for rows.Next() {
			var id string
			var dirty bool
			if rows.Scan(&id, &dirty) != nil {
				continue
			}
			if dirty {
				report.NodesMissingFromIndex++
				continue
			}
			if _, getErr := kg.semantic.collection.GetByID(context.Background(), id); getErr != nil {
				report.NodesMissingFromIndex++
			}
		}
		rows.Close()
	}

	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, semantic_indexed_at IS NULL AS dirty
		FROM kg_edges
	`)
	if err == nil {
		for edgeRows.Next() {
			var source, target, relation string
			var dirty bool
			if edgeRows.Scan(&source, &target, &relation, &dirty) != nil {
				continue
			}
			if dirty {
				report.EdgesMissingFromIndex++
				continue
			}
			edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
			if _, getErr := kg.semantic.collection.GetByID(context.Background(), edgeDocID); getErr != nil {
				report.EdgesMissingFromIndex++
			}
		}
		edgeRows.Close()
	}

	report.NeedsReindex = report.NodesMissingFromIndex > 50 || report.EdgesMissingFromIndex > 50

	return report, nil
}
