package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory/kgsemantic"

	chromem "github.com/philippgille/chromem-go"
)

func semanticNodeContent(node Node) kgsemantic.NodeContent {
	return kgsemantic.NodeContent{
		ID:         node.ID,
		Label:      node.Label,
		Properties: node.Properties,
	}
}

func semanticEdgeContent(edge Edge) kgsemantic.EdgeContent {
	return kgsemantic.EdgeContent{
		Source:     edge.Source,
		Target:     edge.Target,
		Relation:   edge.Relation,
		Properties: edge.Properties,
	}
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

	collection, err := db.GetOrCreateCollection(kgsemantic.CollectionName, nil, embeddingFunc)
	if err != nil {
		return fmt.Errorf("get/create semantic collection: %w", err)
	}

	index := &kgsemantic.Index{
		Collection:    collection,
		EmbeddingFunc: embeddingFunc,
		Logger:        logger,
		QueryCache:    make(map[string]kgsemantic.QueryCacheEntry),
		QueryCacheTTL: kgsemantic.QueryCacheTTL,
		ContentCache:  make(map[string]string),
		ContentKeys:   make([]string, 0),
	}

	if err := kg.validateSemanticIndex(index); err != nil {
		return err
	}
	kg.semanticMu.Lock()
	kg.semantic = index
	kg.semanticMu.Unlock()
	kg.startSemanticReindexBacklogDrain()
	return nil
}

func (kg *KnowledgeGraph) validateSemanticIndex(index *kgsemantic.Index) error {
	err := kg.retrySemanticEmbedding("validate", func(ctx context.Context) error {
		_, err := index.EmbeddingFunc(ctx, "knowledge graph semantic validation")
		return err
	})
	if err != nil {
		return fmt.Errorf("validate semantic embeddings: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) startSemanticReindexBacklogDrain() {
	idx := kg.semanticIndex()
	if idx == nil {
		return
	}
	logger := idx.Logger
	if logger != nil {
		logger.Debug("KG semantic reindex backlog drain scheduled")
	}
	kg.wg.Add(1)
	go func() {
		defer kg.wg.Done()
		attempted, err := kg.RunSemanticReindexIfDue()
		if err != nil {
			if logger != nil {
				logger.Warn("KG semantic reindex backlog drain failed", "error", err)
			}
			return
		}
		if attempted && logger != nil {
			logger.Info("KG semantic reindex backlog drain completed")
		}
	}()
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
	if !kg.reindexInProgress.CompareAndSwap(false, true) {
		return false, nil
	}
	defer kg.reindexInProgress.Store(false)

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
	idx.ReindexMu.Lock()
	rows, err := kg.db.Query(`
		SELECT id, label, properties, protected, COALESCE(updated_at, '')
		FROM kg_nodes
		WHERE semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at
		LIMIT 5000
	`)
	var nodes []Node
	nodeUpdatedAt := make(map[string]string)
	if err == nil {
		for rows.Next() {
			var n Node
			var propsJSON string
			var updatedAt string
			var protected int
			if scanErr := rows.Scan(&n.ID, &n.Label, &propsJSON, &protected, &updatedAt); scanErr != nil {
				rows.Close()
				idx.ReindexMu.Unlock()
				return fmt.Errorf("scan dirty node for semantic reindex: %w", scanErr)
			}
			n.Properties = decodeKnowledgeGraphNodeProperties(idx.Logger, "reindex", n.ID, propsJSON, protected)
			n.Protected = protected != 0
			nodes = append(nodes, n)
			nodeUpdatedAt[n.ID] = updatedAt
		}
		if rowErr := rows.Err(); rowErr != nil {
			rows.Close()
			idx.ReindexMu.Unlock()
			return fmt.Errorf("iterate dirty nodes for semantic reindex: %w", rowErr)
		}
		rows.Close()
	}
	idx.ReindexMu.Unlock()
	if err != nil {
		return fmt.Errorf("load dirty nodes for semantic reindex: %w", err)
	}

	indexedNodeIDs, err := kg.upsertSemanticNodeReindexBatch(idx, nodes)
	if err != nil {
		return err
	}
	if len(indexedNodeIDs) > 0 && idx.Logger != nil {
		idx.Logger.Info("KG semantic reindex: nodes indexed", "count", len(indexedNodeIDs))
	}
	if len(indexedNodeIDs) > 0 {
		idx.ReindexMu.Lock()
		for _, id := range indexedNodeIDs {
			loadedUpdatedAt := nodeUpdatedAt[id]
			if strings.TrimSpace(loadedUpdatedAt) == "" {
				continue
			}
			_, _ = kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = ? WHERE id = ? AND updated_at <= ?`, now, id, loadedUpdatedAt)
		}
		idx.ReindexMu.Unlock()
	}

	// Load dirty edges under reindexMu, then release before expensive embedding I/O.
	idx.ReindexMu.Lock()
	edgeRows, err := kg.db.Query(`
		SELECT e.source, e.target, e.relation, e.properties, COALESCE(e.updated_at, '')
		FROM kg_edges e
		WHERE ` + kgsemantic.EdgeDirtyCondition("e") + `
		LIMIT 5000
	`)
	var edges []Edge
	edgeUpdatedAt := make(map[string]string)
	if err == nil {
		for edgeRows.Next() {
			var e Edge
			var propsJSON string
			var updatedAt string
			if scanErr := edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &propsJSON, &updatedAt); scanErr != nil {
				edgeRows.Close()
				idx.ReindexMu.Unlock()
				return fmt.Errorf("scan dirty edge for semantic reindex: %w", scanErr)
			}
			if strings.TrimSpace(propsJSON) != "" {
				if unmarshalErr := json.Unmarshal([]byte(propsJSON), &e.Properties); unmarshalErr != nil {
					edgeRows.Close()
					idx.ReindexMu.Unlock()
					return fmt.Errorf("decode dirty edge properties for semantic reindex %s -> %s (%s): %w", e.Source, e.Target, e.Relation, unmarshalErr)
				}
			}
			if e.Properties == nil {
				e.Properties = make(map[string]string)
			}
			edges = append(edges, e)
			edgeUpdatedAt[knowledgeGraphEdgeKey(e.Source, e.Target, e.Relation)] = updatedAt
		}
		if rowErr := edgeRows.Err(); rowErr != nil {
			edgeRows.Close()
			idx.ReindexMu.Unlock()
			return fmt.Errorf("iterate dirty edges for semantic reindex: %w", rowErr)
		}
		edgeRows.Close()
	}
	idx.ReindexMu.Unlock()
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Warn("reindexSemanticNodes: edge query failed", "error", err)
		}
	} else {
		indexedEdges, indexErr := kg.upsertSemanticEdgeReindexBatch(idx, edges)
		if indexErr != nil {
			return indexErr
		}
		if len(indexedEdges) > 0 {
			idx.ReindexMu.Lock()
			if err := kg.markSemanticEdgesIndexedAt(indexedEdges, edgeUpdatedAt, now); err != nil && idx.Logger != nil {
				idx.Logger.Warn("reindexSemanticNodes: failed to mark edges indexed", "error", err)
			}
			idx.ReindexMu.Unlock()
		}
	}

	return nil
}

func (kg *KnowledgeGraph) upsertSemanticNodeReindexBatch(idx *kgsemantic.Index, nodes []Node) ([]string, error) {
	if idx == nil || len(nodes) == 0 {
		return nil, nil
	}
	docs := make([]chromem.Document, 0, len(nodes))
	contentByID := make(map[string]string, len(nodes))
	indexedIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if !kgsemantic.ShouldIndexNode(semanticNodeContent(node)) {
			indexedIDs = append(indexedIDs, node.ID)
			continue
		}
		content := kgsemantic.BuildNodeContent(semanticNodeContent(node))
		if content == "" {
			indexedIDs = append(indexedIDs, node.ID)
			continue
		}
		docs = append(docs, chromem.Document{
			ID:      node.ID,
			Content: content,
			Metadata: map[string]string{
				"node_id": node.ID,
				"label":   node.Label,
			},
		})
		contentByID[node.ID] = content
		indexedIDs = append(indexedIDs, node.ID)
	}
	if len(docs) == 0 {
		return indexedIDs, nil
	}

	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()
	for start := 0; start < len(docs); start += kgsemantic.ReindexDocumentBatchSize {
		end := start + kgsemantic.ReindexDocumentBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		chunk := docs[start:end]
		err := kg.retrySemanticEmbedding("node_reindex_batch", func(ctx context.Context) error {
			return idx.Collection.AddDocuments(ctx, chunk, 1)
		})
		if err != nil {
			if idx.Logger != nil {
				idx.Logger.Warn("KG semantic node batch reindex failed", "count", len(chunk), "total", len(docs), "error", err)
			}
			return nil, fmt.Errorf("semantic node batch reindex: %w", err)
		}
	}
	idx.Mu.Lock()
	for id, content := range contentByID {
		idx.SetContentCacheEntry(id, content)
	}
	idx.Mu.Unlock()
	return indexedIDs, nil
}

func (kg *KnowledgeGraph) upsertSemanticEdgeReindexBatch(idx *kgsemantic.Index, edges []Edge) ([]Edge, error) {
	if idx == nil || len(edges) == 0 {
		return nil, nil
	}
	docs := make([]chromem.Document, 0, len(edges))
	indexedEdges := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		content := kgsemantic.BuildEdgeContent(semanticEdgeContent(edge))
		if content == "" {
			indexedEdges = append(indexedEdges, edge)
			continue
		}
		edgeDocID := "edge://" + edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
		docs = append(docs, chromem.Document{
			ID:      edgeDocID,
			Content: content,
			Metadata: map[string]string{
				"source":   edge.Source,
				"target":   edge.Target,
				"relation": edge.Relation,
			},
		})
		indexedEdges = append(indexedEdges, edge)
	}
	if len(docs) == 0 {
		return indexedEdges, nil
	}

	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()
	for start := 0; start < len(docs); start += kgsemantic.ReindexDocumentBatchSize {
		end := start + kgsemantic.ReindexDocumentBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		chunk := docs[start:end]
		err := kg.retrySemanticEmbedding("edge_reindex_batch", func(ctx context.Context) error {
			return idx.Collection.AddDocuments(ctx, chunk, 1)
		})
		if err != nil {
			if idx.Logger != nil {
				idx.Logger.Warn("KG semantic edge batch reindex failed", "count", len(chunk), "total", len(docs), "error", err)
			}
			return nil, fmt.Errorf("semantic edge batch reindex: %w", err)
		}
	}
	return indexedEdges, nil
}

func (kg *KnowledgeGraph) markSemanticEdgesIndexedAt(edges []Edge, loadedUpdatedAtByKey map[string]string, indexedAt string) error {
	if kg == nil || kg.db == nil || len(edges) == 0 {
		return nil
	}
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin semantic edge timestamp update: %w", err)
	}
	defer tx.Rollback()
	for _, edge := range edges {
		loadedUpdatedAt := loadedUpdatedAtByKey[knowledgeGraphEdgeKey(edge.Source, edge.Target, edge.Relation)]
		if strings.TrimSpace(loadedUpdatedAt) == "" {
			continue
		}
		if _, err := tx.Exec(`
			UPDATE kg_edges
			SET semantic_indexed_at = ?
			WHERE source = ? AND target = ? AND relation = ? AND updated_at <= ?
		`, indexedAt, edge.Source, edge.Target, edge.Relation, loadedUpdatedAt); err != nil {
			return fmt.Errorf("update semantic edge timestamp: %w", err)
		}
	}
	return tx.Commit()
}

func (kg *KnowledgeGraph) upsertSemanticNodeIndex(node Node) bool {
	idx := kg.semanticIndex()
	if idx == nil || !kgsemantic.ShouldIndexNode(semanticNodeContent(node)) {
		return true
	}

	content := kgsemantic.BuildNodeContent(semanticNodeContent(node))
	if content == "" {
		return true
	}

	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()

	err := kg.retrySemanticEmbedding("node_upsert", func(ctx context.Context) error {
		return idx.Collection.AddDocument(ctx, chromem.Document{
			ID:      node.ID,
			Content: content,
			Metadata: map[string]string{
				"node_id": node.ID,
				"label":   node.Label,
			},
		})
	})
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Warn("KG semantic node index update failed", "node_id", node.ID, "error", err)
		}
		return false
	}
	idx.Mu.Lock()
	idx.SetContentCacheEntry(node.ID, content)
	idx.Mu.Unlock()
	return true
}

func (kg *KnowledgeGraph) upsertSemanticEdgeIndex(edge Edge) bool {
	idx := kg.semanticIndex()
	if idx == nil {
		return true
	}
	content := kgsemantic.BuildEdgeContent(semanticEdgeContent(edge))
	if content == "" {
		return true
	}

	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()

	edgeDocID := "edge://" + edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
	err := kg.retrySemanticEmbedding("edge_upsert", func(ctx context.Context) error {
		return idx.Collection.AddDocument(ctx, chromem.Document{
			ID:      edgeDocID,
			Content: content,
			Metadata: map[string]string{
				"source":   edge.Source,
				"target":   edge.Target,
				"relation": edge.Relation,
			},
		})
	})
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Warn("KG semantic edge index update failed", "source", edge.Source, "target", edge.Target, "error", err)
		}
		return false
	}
	return true
}

func (kg *KnowledgeGraph) markSemanticNodeDirty(nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if kg == nil || kg.db == nil || nodeID == "" {
		return
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = NULL WHERE id = ?`, nodeID); err != nil && kg.logger != nil {
		kg.logger.Warn("markSemanticNodeDirty failed", "node_id", nodeID, "error", err)
	}
}

func (kg *KnowledgeGraph) markSemanticEdgeDirty(source, target, relation string) {
	if kg == nil || kg.db == nil {
		return
	}
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	if source == "" || target == "" || relation == "" {
		return
	}
	if _, err := kg.db.Exec(`
		UPDATE kg_edges SET semantic_indexed_at = NULL
		WHERE source = ? AND target = ? AND relation = ?
	`, source, target, relation); err != nil && kg.logger != nil {
		kg.logger.Warn("markSemanticEdgeDirty failed", "source", source, "target", target, "relation", relation, "error", err)
	}
}

func (kg *KnowledgeGraph) indexSemanticNodeAfterWrite(node Node) {
	if kg.upsertSemanticNodeIndex(node) {
		return
	}
	kg.markSemanticNodeDirty(node.ID)
}

func (kg *KnowledgeGraph) hydrateSemanticEdgeForIndex(edge Edge) (Edge, bool) {
	if edge.Properties != nil && len(edge.Properties) > 0 {
		return edge, true
	}
	if kg == nil || kg.db == nil {
		return edge, false
	}

	var propsJSON string
	err := kg.db.QueryRow(`
		SELECT properties FROM kg_edges
		WHERE source = ? AND target = ? AND relation = ?
	`, edge.Source, edge.Target, edge.Relation).Scan(&propsJSON)
	if err == sql.ErrNoRows {
		return edge, false
	}
	if err != nil {
		if kg.logger != nil {
			kg.logger.Warn("indexSemanticEdgeAfterWrite: failed to reload edge properties",
				"source", edge.Source, "target", edge.Target, "relation", edge.Relation, "error", err)
		}
		return edge, false
	}

	props := make(map[string]string)
	if strings.TrimSpace(propsJSON) != "" {
		if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
			if kg.logger != nil {
				kg.logger.Warn("indexSemanticEdgeAfterWrite: corrupt edge properties JSON",
					"source", edge.Source, "target", edge.Target, "relation", edge.Relation, "error", err)
			}
			return edge, false
		}
	}
	edge.Properties = props
	return edge, true
}

func (kg *KnowledgeGraph) indexSemanticEdgeAfterWrite(edge Edge) {
	edge.Source = strings.TrimSpace(edge.Source)
	edge.Target = strings.TrimSpace(edge.Target)
	edge.Relation = strings.TrimSpace(edge.Relation)
	if edge.Source == "" || edge.Target == "" || edge.Relation == "" {
		return
	}
	var ok bool
	edge, ok = kg.hydrateSemanticEdgeForIndex(edge)
	if !ok {
		kg.markSemanticEdgeDirty(edge.Source, edge.Target, edge.Relation)
		return
	}
	if kg.upsertSemanticEdgeIndex(edge) {
		return
	}
	kg.markSemanticEdgeDirty(edge.Source, edge.Target, edge.Relation)
}

// removeSemanticNodeIndex removes a node's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticNodeIndex(nodeID string) error {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil
	}
	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()
	idx.Mu.Lock()
	idx.RemoveContentCacheEntry(nodeID)
	idx.Mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return idx.Collection.Delete(ctx, nil, nil, nodeID)
}

// removeSemanticEdgeIndex removes an edge's entry from the semantic index.
func (kg *KnowledgeGraph) removeSemanticEdgeIndex(source, target, relation string) error {
	idx := kg.semanticIndex()
	if idx == nil {
		return nil
	}
	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()

	edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return idx.Collection.Delete(ctx, nil, nil, edgeDocID)
}

func (kg *KnowledgeGraph) removeSemanticEdgeIndexBatch(source string, targets []string, relation string) error {
	idx := kg.semanticIndex()
	if idx == nil || len(targets) == 0 {
		return nil
	}
	idx.MutationMu.Lock()
	defer idx.MutationMu.Unlock()

	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, "edge://"+source+"\x00"+target+"\x00"+relation)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return idx.Collection.Delete(ctx, nil, nil, ids...)
}

func (kg *KnowledgeGraph) semanticSearchNodes(query string, minSim float32, maxNodes int) []Node {
	return kg.semanticSearchNodesWithQueryer(kg.db, query, minSim, maxNodes)
}

func (kg *KnowledgeGraph) semanticSearchNodesWithQueryer(q knowledgeGraphQueryer, query string, minSim float32, maxNodes int) []Node {
	idx := kg.semanticIndex()
	if idx == nil || maxNodes <= 0 || kgsemantic.ShouldSkipQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.Collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.Collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	candidateIDs := make([]string, 0, len(results))
	for _, result := range results {
		if result.Similarity < minSim {
			continue
		}
		if !strings.HasPrefix(result.ID, "edge://") {
			candidateIDs = append(candidateIDs, result.ID)
		}
	}
	if len(candidateIDs) == 0 {
		return nil
	}

	nodes, err := loadNodesByIDs(q, candidateIDs, kg.logger, "semanticSearchNodes")
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Warn("semanticSearchNodes: batch load failed", "error", err)
		}
		return nil
	}
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if kg.isExcludedNodeType(n.Properties["type"]) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func (kg *KnowledgeGraph) semanticSearchNodeIDs(query string, maxNodes int) []string {
	idx := kg.semanticIndex()
	if idx == nil || maxNodes <= 0 || kgsemantic.ShouldSkipQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.Collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.Collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic search failed", "error", err)
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
	if idx == nil || maxNodes <= 0 || kgsemantic.ShouldSkipQuery(query) {
		return nil
	}

	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic query embedding failed", "error", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count := idx.Collection.Count()
	if count == 0 {
		return nil
	}
	if maxNodes > count {
		maxNodes = count
	}
	results, err := idx.Collection.QueryEmbedding(ctx, embedding, maxNodes, nil, nil)
	if err != nil {
		if idx.Logger != nil {
			idx.Logger.Debug("KG semantic search failed", "error", err)
		}
		return nil
	}

	minSim := kg.getMinSemanticSimilarity()
	type candidateHit struct {
		id         string
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
	if idx == nil || maxEdges <= 0 || kgsemantic.ShouldSkipQuery(query) {
		return nil
	}
	embedding, err := kg.getSemanticQueryEmbedding(query)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count := idx.Collection.Count()
	if count == 0 {
		return nil
	}
	if maxEdges > count {
		maxEdges = count
	}
	results, err := idx.Collection.QueryEmbedding(ctx, embedding, maxEdges, nil, nil)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(results))
	for _, result := range results {
		if result.Similarity < kgsemantic.EdgeMinSimilarity {
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
	idx.Mu.Lock()
	if entry, ok := idx.QueryCache[query]; ok && time.Since(entry.Timestamp) < idx.QueryCacheTTL {
		embedding := cloneFloat32Slice(entry.Embedding)
		idx.Mu.Unlock()
		return embedding, nil
	}
	idx.Mu.Unlock()

	// Slow path: call embedding API WITHOUT holding the lock to avoid blocking other goroutines.
	result, err, _ := idx.QueryGroup.Do(query, func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), kgsemantic.QueryTimeout)
		defer cancel()
		embedding, err := idx.EmbeddingFunc(ctx, query)
		if err != nil {
			return nil, err
		}
		return cloneFloat32Slice(embedding), nil
	})
	if err != nil {
		idx.QueryGroup.Forget(query)
		return nil, err
	}
	embedding := result.([]float32)

	// Store result under lock; re-check cache in case another goroutine raced us.
	idx.Mu.Lock()
	defer idx.Mu.Unlock()
	if existing, ok := idx.QueryCache[query]; ok && time.Since(existing.Timestamp) < idx.QueryCacheTTL {
		return cloneFloat32Slice(existing.Embedding), nil
	}
	cachedEmbedding := cloneFloat32Slice(embedding)
	idx.QueryCache[query] = kgsemantic.QueryCacheEntry{Embedding: cachedEmbedding, Timestamp: time.Now()}
	if len(idx.QueryCache) > kgsemantic.QueryCacheMaxSize {
		now := time.Now()
		var toDelete []string
		var oldestKey string
		var oldestTime time.Time
		for k, v := range idx.QueryCache {
			if now.Sub(v.Timestamp) > idx.QueryCacheTTL {
				toDelete = append(toDelete, k)
			} else if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		if len(toDelete) > 0 {
			for _, k := range toDelete {
				delete(idx.QueryCache, k)
			}
		} else if oldestKey != "" {
			delete(idx.QueryCache, oldestKey)
		}
	}
	return cloneFloat32Slice(cachedEmbedding), nil
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
		if idx := kg.semanticIndex(); idx != nil && idx.Logger != nil {
			idx.Logger.Warn("filterExcludedKnowledgeGraphNodeTypes: query failed", "error", err)
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
		if idx := kg.semanticIndex(); idx != nil && idx.Logger != nil {
			idx.Logger.Warn("filterExcludedKnowledgeGraphNodeTypes: row iteration failed", "error", err)
		}
	}
	return allowed
}

func (kg *KnowledgeGraph) retrySemanticEmbedding(op string, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= kgsemantic.RetryMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), kgsemantic.QueryTimeout)
		err := fn(ctx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == kgsemantic.RetryMaxAttempts || !kgsemantic.ShouldRetryEmbeddingErr(err) {
			return err
		}

		backoff := time.Duration(attempt*attempt) * kgsemantic.RetryBackoffBase
		if idx := kg.semanticIndex(); idx != nil && idx.Logger != nil {
			idx.Logger.Debug("KG semantic embedding op failed; retrying", "op", op, "attempt", attempt, "backoff", backoff, "error", err)
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
	SemanticEnabled      bool                 `json:"semantic_enabled"`
	DirtyNodes           int                  `json:"dirty_nodes"`
	DirtyEdges           int                  `json:"dirty_edges"`
	DroppedAccessHits    int64                `json:"dropped_access_hits"`
	NeedsReindex         bool                 `json:"needs_reindex"`
	ReindexBacklog       bool                 `json:"reindex_backlog"`
	TotalNodes           int                  `json:"total_nodes"`
	TotalEdges           int                  `json:"total_edges"`
	IsolatedNodes        int                  `json:"isolated_nodes"`
	DuplicateGroups      int                  `json:"duplicate_groups"`
	LabelDuplicateGroups int                  `json:"label_duplicate_groups"`
	IDDuplicateGroups    int                  `json:"id_duplicate_groups"`
	AcceptedEdges        int                  `json:"accepted_edges"`
	SupersededEdges      int                  `json:"superseded_edges"`
	RetractedEdges       int                  `json:"retracted_edges"`
	OpenConflicts        int                  `json:"open_conflicts"`
	Consistency          *KGConsistencyReport `json:"consistency,omitempty"`
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
		WHERE ` + kgsemantic.EdgeDirtyCondition("e"),
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
	lifecycle, err := kg.GetLifecycleCounts()
	if err != nil {
		return nil, fmt.Errorf("count kg lifecycle state: %w", err)
	}
	report.AcceptedEdges = lifecycle.AcceptedEdges
	report.SupersededEdges = lifecycle.SupersededEdges
	report.RetractedEdges = lifecycle.RetractedEdges
	report.OpenConflicts = lifecycle.OpenConflicts

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
		consistency, err := kg.ConsistencyCheckSample(kgsemantic.ConsistencyCheckSampleSize)
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
	report.TotalIndexed = uint(idx.Collection.Count())

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
			if scanErr := rows.Scan(&id); scanErr != nil {
				rows.Close()
				return nil, fmt.Errorf("scan indexed semantic node: %w", scanErr)
			}
			if _, getErr := idx.Collection.GetByID(context.Background(), id); getErr != nil {
				report.NodesMissingFromIndex++
			}
		}
		if rowErr := rows.Err(); rowErr != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate indexed semantic nodes: %w", rowErr)
		}
		rows.Close()
	}

	edgeQuery := `
		SELECT e.source, e.target, e.relation
		FROM kg_edges e
		WHERE NOT ` + kgsemantic.EdgeDirtyCondition("e") + `
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
			if scanErr := edgeRows.Scan(&source, &target, &relation); scanErr != nil {
				edgeRows.Close()
				return nil, fmt.Errorf("scan indexed semantic edge: %w", scanErr)
			}
			edgeDocID := "edge://" + source + "\x00" + target + "\x00" + relation
			if _, getErr := idx.Collection.GetByID(context.Background(), edgeDocID); getErr != nil {
				report.EdgesMissingFromIndex++
			}
		}
		if rowErr := edgeRows.Err(); rowErr != nil {
			edgeRows.Close()
			return nil, fmt.Errorf("iterate indexed semantic edges: %w", rowErr)
		}
		edgeRows.Close()
	}

	var cleanNodes int
	if err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_nodes
		WHERE NOT (semantic_indexed_at IS NULL OR semantic_indexed_at < updated_at)
	`).Scan(&cleanNodes); err != nil {
		return nil, fmt.Errorf("count indexed semantic nodes: %w", err)
	}
	var cleanEdges int
	if err := kg.db.QueryRow(`
		SELECT COUNT(*) FROM kg_edges e
		WHERE NOT ` + kgsemantic.EdgeDirtyCondition("e"),
	).Scan(&cleanEdges); err != nil {
		return nil, fmt.Errorf("count indexed semantic edges: %w", err)
	}
	if indexed := int(report.TotalIndexed); indexed > cleanNodes+cleanEdges {
		report.IndexOrphans = indexed - cleanNodes - cleanEdges
	}

	report.ReindexBacklog = report.DirtyNodes > knowledgeGraphSemanticReindexBacklogLimit || report.DirtyEdges > knowledgeGraphSemanticReindexBacklogLimit
	report.NeedsReindex = report.DirtyNodes > 0 || report.DirtyEdges > 0 || report.NodesMissingFromIndex > 50 || report.EdgesMissingFromIndex > 50 || report.IndexOrphans > 0

	return report, nil
}
