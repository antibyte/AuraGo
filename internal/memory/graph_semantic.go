package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

const knowledgeGraphSemanticCollection = "kg_embeddings"
const knowledgeGraphSemanticTimeout = 20 * time.Second
const knowledgeGraphSemanticMinSimilarity = 0.45

const knowledgeGraphSemanticEdgeMinSimilarity = 0.35

const knowledgeGraphSemanticQueryCacheTTL = 5 * time.Minute
const knowledgeGraphSemanticEdgeMaxResults = 50

type knowledgeGraphSemanticIndex struct {
	collection    *chromem.Collection
	embeddingFunc chromem.EmbeddingFunc
	logger        *slog.Logger
	mu            sync.Mutex
	queryCache    map[string]queryCacheEntry
	queryCacheTTL time.Duration
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

	db, err := chromem.NewPersistentDB(cfg.Directories.VectorDBDir, false)
	if err != nil {
		return fmt.Errorf("open semantic vector db: %w", err)
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
	}

	if err := kg.validateSemanticIndex(index); err != nil {
		return err
	}
	kg.semantic = index
	return kg.reindexSemanticNodes()
}

func (kg *KnowledgeGraph) validateSemanticIndex(index *knowledgeGraphSemanticIndex) error {
	ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphSemanticTimeout)
	defer cancel()
	if _, err := index.embeddingFunc(ctx, "knowledge graph semantic validation"); err != nil {
		return fmt.Errorf("validate semantic embeddings: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) reindexSemanticNodes() error {
	if kg.semantic == nil {
		return nil
	}

	now := time.Now().Format(time.RFC3339)

	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected FROM kg_nodes
		WHERE semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at
		LIMIT 5000
	`)
	if err != nil {
		return fmt.Errorf("load dirty nodes for semantic reindex: %w", err)
	}
	var nodes []Node
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

	var indexedNodeIDs []string
	for _, node := range nodes {
		if kg.upsertSemanticNodeIndex(node) {
			indexedNodeIDs = append(indexedNodeIDs, node.ID)
		}
	}
	if len(indexedNodeIDs) > 0 {
		placeholders := strings.Repeat("?,", len(indexedNodeIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]interface{}, 0, len(indexedNodeIDs)+1)
		args = append(args, now)
		for _, id := range indexedNodeIDs {
			args = append(args, id)
		}
		_, _ = kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = ? WHERE id IN (`+placeholders+`)`, args...)
	}

	edgeRows, err := kg.db.Query(`
		SELECT source, target, relation, properties FROM kg_edges
		WHERE semantic_indexed_at IS NULL OR semantic_indexed_at < (
			SELECT COALESCE(MAX(n2.updated_at), '1970-01-01') FROM kg_nodes n2
			WHERE n2.id = kg_edges.source OR n2.id = kg_edges.target
		)
		LIMIT 5000
	`)
	if err == nil {
		var edges []Edge
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
		for _, edge := range edges {
			kg.upsertSemanticEdgeIndex(edge)
		}
		if len(edges) > 0 {
			_, _ = kg.db.Exec(`UPDATE kg_edges SET semantic_indexed_at = ? WHERE semantic_indexed_at IS NULL`, now)
		}
	}

	return nil
}

func (kg *KnowledgeGraph) upsertSemanticNodeIndex(node Node) bool {
	if kg.semantic == nil || !shouldIndexKnowledgeGraphNode(node) {
		return true // skip is not a failure
	}

	content := buildKnowledgeGraphSemanticContent(node)
	if content == "" {
		return true // nothing to index is not a failure
	}

	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphSemanticTimeout)
	defer cancel()

	_ = kg.semantic.collection.Delete(ctx, nil, nil, node.ID)
	err := kg.semantic.collection.AddDocument(ctx, chromem.Document{
		ID:      node.ID,
		Content: content,
		Metadata: map[string]string{
			"node_id": node.ID,
			"label":   node.Label,
		},
	})
	if err != nil {
		if kg.semantic.logger != nil {
			kg.semantic.logger.Warn("KG semantic node index update failed", "node_id", node.ID, "error", err)
		}
		return false
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

	ctx, cancel := context.WithTimeout(context.Background(), knowledgeGraphSemanticTimeout)
	defer cancel()

	edgeDocID := "edge_" + edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
	_ = kg.semantic.collection.Delete(ctx, nil, nil, edgeDocID)
	err := kg.semantic.collection.AddDocument(ctx, chromem.Document{
		ID:      edgeDocID,
		Content: content,
		Metadata: map[string]string{
			"source":   edge.Source,
			"target":   edge.Target,
			"relation": edge.Relation,
		},
	})
	if err != nil && kg.semantic.logger != nil {
		kg.semantic.logger.Warn("KG semantic edge index update failed", "source", edge.Source, "target", edge.Target, "error", err)
	}
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
	return strings.TrimSpace(strings.Join(parts, ". "))
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
		if !strings.HasPrefix(result.ID, "edge_") {
			out = append(out, result.ID)
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
		if strings.HasPrefix(result.ID, "edge_") {
			out = append(out, result.ID)
		}
	}
	return out
}

func (kg *KnowledgeGraph) getSemanticQueryEmbedding(query string) ([]float32, error) {
	if kg.semantic == nil {
		return nil, fmt.Errorf("semantic search is disabled")
	}

	kg.semantic.mu.Lock()
	defer kg.semantic.mu.Unlock()

	if entry, ok := kg.semantic.queryCache[query]; ok && time.Since(entry.timestamp) < kg.semantic.queryCacheTTL {
		return entry.embedding, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	embedding, err := kg.semantic.embeddingFunc(ctx, query)
	if err != nil {
		return nil, err
	}

	kg.semantic.queryCache[query] = queryCacheEntry{embedding: embedding, timestamp: time.Now()}
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
	if query == "" || query == "*" || len([]rune(query)) < 3 {
		return true
	}
	return false
}
