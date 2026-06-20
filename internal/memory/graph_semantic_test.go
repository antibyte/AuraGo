package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestKGSemanticUpsertDoesNotDeleteOnEmbeddingFailure(t *testing.T) {
	kg := &KnowledgeGraph{}

	embeddingFunc := func(_ context.Context, _ string) ([]float32, error) {
		return nil, context.DeadlineExceeded
	}

	db := chromem.NewDB()
	collection, err := db.GetOrCreateCollection(knowledgeGraphSemanticCollection, nil, embeddingFunc)
	if err != nil {
		t.Fatalf("GetOrCreateCollection: %v", err)
	}

	kg.semantic = &knowledgeGraphSemanticIndex{
		collection:    collection,
		embeddingFunc: embeddingFunc,
		queryCache:    make(map[string]queryCacheEntry),
		queryCacheTTL: knowledgeGraphSemanticQueryCacheTTL,
		contentCache:  make(map[string]string),
	}

	// Seed an existing document so we can verify it's not removed when the embedding
	// provider times out.
	if err := collection.AddDocument(context.Background(), chromem.Document{
		ID:        "node-1",
		Content:   "old content",
		Embedding: []float32{1, 0},
		Metadata:  map[string]string{"node_id": "node-1", "label": "Old"},
	}); err != nil {
		t.Fatalf("seed AddDocument: %v", err)
	}

	ok := kg.upsertSemanticNodeIndex(Node{
		ID:         "node-1",
		Label:      "Test",
		Properties: map[string]string{"foo": "bar"},
	})
	if ok {
		t.Fatal("expected upsert to fail due to embedding timeout")
	}

	doc, err := collection.GetByID(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("GetByID after failed upsert: %v", err)
	}
	if doc.Content != "old content" {
		t.Fatalf("expected existing document content to remain, got %q", doc.Content)
	}
}

func TestKGDeleteBySourceFileRemovesSemanticIndexEntries(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		if text == "" {
			return []float32{0, 0}, nil
		}
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("file-node", "File Node", map[string]string{"source": "file_sync", "source_file": "/docs/a.md"}); err != nil {
		t.Fatalf("AddNode file-node: %v", err)
	}
	if err := kg.AddNode("other-node", "Other Node", map[string]string{"source": "file_sync", "source_file": "/docs/b.md"}); err != nil {
		t.Fatalf("AddNode other-node: %v", err)
	}
	if err := kg.AddEdge("file-node", "other-node", "mentions", map[string]string{"source": "file_sync", "source_file": "/docs/a.md"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	edgeDocID := "edge://file-node\x00other-node\x00mentions"
	if _, err := kg.semantic.collection.GetByID(context.Background(), "file-node"); err != nil {
		t.Fatalf("expected node semantic document before delete: %v", err)
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), edgeDocID); err != nil {
		t.Fatalf("expected edge semantic document before delete: %v", err)
	}

	if deleted, err := kg.DeleteNodesBySourceFile("/docs/a.md"); err != nil || deleted != 1 {
		t.Fatalf("DeleteNodesBySourceFile deleted=%d err=%v", deleted, err)
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), "file-node"); err == nil {
		t.Fatalf("expected node semantic document removed, got %v", err)
	}
	if _, ok := kg.semantic.contentCache["file-node"]; ok {
		t.Fatalf("expected node semantic content cache to be removed")
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), edgeDocID); err == nil {
		t.Fatalf("expected incident edge semantic document removed, got %v", err)
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), "other-node"); err != nil {
		t.Fatalf("expected unrelated node semantic document to remain: %v", err)
	}
}

func TestKGSemanticNodeUpsertRefreshesCachedContent(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("service", "Old Service", map[string]string{"type": "service", "notes": "old details"}); err != nil {
		t.Fatalf("AddNode old: %v", err)
	}
	if err := kg.AddNode("service", "New Service", map[string]string{"type": "service", "notes": "new details"}); err != nil {
		t.Fatalf("AddNode new: %v", err)
	}

	doc, err := kg.semantic.collection.GetByID(context.Background(), "service")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !strings.Contains(doc.Content, "new details") {
		t.Fatalf("semantic content = %q, want refreshed node properties", doc.Content)
	}
}

func TestKGAddEdgePreservesRichNodeSemanticIndex(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("app", "Application Server", map[string]string{"type": "service", "notes": "rich deployment details"}); err != nil {
		t.Fatalf("AddNode app: %v", err)
	}
	if err := kg.AddNode("server", "Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode server: %v", err)
	}
	if err := kg.AddEdge("app", "server", "runs_on", map[string]string{"notes": "nightly workload"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	doc, err := kg.semantic.collection.GetByID(context.Background(), "app")
	if err != nil {
		t.Fatalf("GetByID app: %v", err)
	}
	if !strings.Contains(doc.Content, "rich deployment details") {
		t.Fatalf("semantic content = %q, want original node properties preserved", doc.Content)
	}
}

func TestKGOptimizeGraphRemovesSemanticIndexEntries(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("temp", "Temporary Node", map[string]string{"notes": "short lived"}); err != nil {
		t.Fatalf("AddNode temp: %v", err)
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), "temp"); err != nil {
		t.Fatalf("expected semantic doc before optimize: %v", err)
	}

	removed, err := kg.OptimizeGraph(1)
	if err != nil {
		t.Fatalf("OptimizeGraph: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := kg.semantic.collection.GetByID(context.Background(), "temp"); err == nil {
		t.Fatal("expected semantic doc removed after optimize")
	}
}

func TestKGUpdateEdgeRefreshesSemanticIndex(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("app", "App", map[string]string{"type": "software"}); err != nil {
		t.Fatalf("AddNode app: %v", err)
	}
	if err := kg.AddNode("server", "Server", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode server: %v", err)
	}
	if err := kg.AddEdge("app", "server", "runs_on", map[string]string{"notes": "old edge"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if _, err := kg.UpdateEdge("app", "server", "runs_on", "depends_on", map[string]string{"notes": "new edge"}); err != nil {
		t.Fatalf("UpdateEdge: %v", err)
	}

	oldID := "edge://app\x00server\x00runs_on"
	if _, err := kg.semantic.collection.GetByID(context.Background(), oldID); err == nil {
		t.Fatalf("expected old edge semantic document removed")
	}
	newID := "edge://app\x00server\x00depends_on"
	doc, err := kg.semantic.collection.GetByID(context.Background(), newID)
	if err != nil {
		t.Fatalf("expected updated edge semantic document: %v", err)
	}
	if !strings.Contains(doc.Content, "depends_on") || !strings.Contains(doc.Content, "new edge") {
		t.Fatalf("semantic edge content = %q, want updated relation and properties", doc.Content)
	}
}

func TestKGConsistencyCheckDetectsMissingIndexedNodeDocument(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}
	if err := kg.AddNode("nas", "NAS", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := kg.db.Exec("UPDATE kg_nodes SET semantic_indexed_at = CURRENT_TIMESTAMP WHERE id = 'nas'"); err != nil {
		t.Fatalf("mark indexed: %v", err)
	}
	if err := kg.semantic.collection.Delete(context.Background(), nil, nil, "nas"); err != nil {
		t.Fatalf("delete semantic doc: %v", err)
	}

	report, err := kg.ConsistencyCheck()
	if err != nil {
		t.Fatalf("ConsistencyCheck: %v", err)
	}
	if report.NodesMissingFromIndex != 1 {
		t.Fatalf("NodesMissingFromIndex = %d, want 1", report.NodesMissingFromIndex)
	}
}

func TestKGSemanticSearchAllowsShortEntityQueries(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"docker", false},
		{"proxmox", false},
		{"debian", false},
		{"ansible", false},
		{"grafana", false},
		{"home_assistant", false},
		{"S3", false},
		{"NAS", false},
		{"hi", true},
		{"a", true},
		{"status?", true},
		{"", true},
		{"*", true},
	}
	for _, tc := range cases {
		got := shouldSkipKnowledgeGraphSemanticQuery(tc.query)
		if got != tc.want {
			t.Errorf("shouldSkipKnowledgeGraphSemanticQuery(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestKGSemanticSearchFiltersActivityEntity(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "docker"):
			return []float32{1, 0}, nil
		default:
			return []float32{0, 1}, nil
		}
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("docker", "Docker", map[string]string{"type": "software"}); err != nil {
		t.Fatalf("AddNode docker: %v", err)
	}
	if err := kg.AddNode("chat_turn", "Chat Turn", map[string]string{"type": "activity_entity"}); err != nil {
		t.Fatalf("AddNode activity_entity: %v", err)
	}

	nodes := kg.semanticSearchNodes("docker", 0.5, 5)
	for _, n := range nodes {
		if n.Properties["type"] == "activity_entity" {
			t.Fatalf("semantic search returned activity_entity node: %+v", n)
		}
	}
}

func TestKGSearchForContextExcludesActivityEntity(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		lower := strings.ToLower(text)
		switch {
		case strings.Contains(lower, "docker"):
			return []float32{1, 0}, nil
		default:
			return []float32{0, 1}, nil
		}
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("docker", "Docker", map[string]string{"type": "software"}); err != nil {
		t.Fatalf("AddNode docker: %v", err)
	}
	if err := kg.AddNode("random_turn", "Random Turn", map[string]string{"type": "activity_entity"}); err != nil {
		t.Fatalf("AddNode activity_entity: %v", err)
	}

	ctx := kg.SearchForContext("docker", 5, 800)
	if !strings.Contains(ctx, "docker") {
		t.Fatalf("expected context to contain docker, got %q", ctx)
	}
	if strings.Contains(ctx, "random_turn") {
		t.Fatalf("expected context to exclude activity_entity node, got %q", ctx)
	}
}

func TestKGRunSemanticReindexIfDueRespectsInterval(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, _ string) ([]float32, error) {
		return []float32{1, 0}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	kg.SetSemanticReindexInterval("1h")
	ran, err := kg.RunSemanticReindexIfDue()
	if err != nil {
		t.Fatalf("RunSemanticReindexIfDue: %v", err)
	}
	if ran {
		t.Fatal("expected semantic reindex to be skipped before interval elapsed")
	}
}

func TestKGSemanticContentCacheTrimsOldestEntries(t *testing.T) {
	idx := &knowledgeGraphSemanticIndex{
		contentCache:     make(map[string]string),
		contentCacheKeys: make([]string, 0),
	}
	for i := 0; i < knowledgeGraphSemanticContentCacheMaxSize+1; i++ {
		idx.setContentCacheEntry(fmt.Sprintf("node-%d", i), "content")
	}
	if len(idx.contentCache) > knowledgeGraphSemanticContentCacheMaxSize {
		t.Fatalf("cache size = %d, want <= %d", len(idx.contentCache), knowledgeGraphSemanticContentCacheMaxSize)
	}
	if _, ok := idx.contentCache["node-0"]; ok {
		t.Fatal("expected oldest cache entry to be evicted")
	}
	if _, ok := idx.contentCache[fmt.Sprintf("node-%d", knowledgeGraphSemanticContentCacheMaxSize)]; !ok {
		t.Fatal("expected newest cache entry to remain")
	}
}

func TestKGIncrementCoOccurrenceUpsertsSemanticEdge(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("alice", "Alice", nil); err != nil {
		t.Fatalf("AddNode alice: %v", err)
	}
	if err := kg.AddNode("bob", "Bob", nil); err != nil {
		t.Fatalf("AddNode bob: %v", err)
	}
	if err := kg.IncrementCoOccurrence("alice", "bob", "2026-01-01"); err != nil {
		t.Fatalf("IncrementCoOccurrence: %v", err)
	}

	edgeDocID := "edge://alice\x00bob\x00co_mentioned_with"
	doc, err := kg.semantic.collection.GetByID(context.Background(), edgeDocID)
	if err != nil {
		t.Fatalf("expected co-occurrence semantic edge document: %v", err)
	}
	if !strings.Contains(doc.Content, "co_mentioned_with") {
		t.Fatalf("semantic edge content = %q, want co_mentioned_with relation", doc.Content)
	}
}

func TestKGMergeNodesRemovesSourceSemanticEdges(t *testing.T) {
	kg := newTestKG(t)

	embeddingFunc := func(_ context.Context, text string) ([]float32, error) {
		return []float32{float32(len(text)), 1}, nil
	}
	db := chromem.NewDB()
	if err := kg.enableSemanticSearchWithCollection(db, embeddingFunc, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}

	if err := kg.AddNode("target", "Target", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode target: %v", err)
	}
	if err := kg.AddNode("source", "Source", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode source: %v", err)
	}
	if err := kg.AddEdge("source", "peer", "connects_to", map[string]string{"notes": "outgoing"}); err != nil {
		t.Fatalf("AddEdge outgoing: %v", err)
	}
	if err := kg.AddEdge("client", "source", "uses", map[string]string{"notes": "incoming"}); err != nil {
		t.Fatalf("AddEdge incoming: %v", err)
	}

	sourceOutgoingID := "edge://source\x00peer\x00connects_to"
	sourceIncomingID := "edge://client\x00source\x00uses"
	for _, docID := range []string{"source", sourceOutgoingID, sourceIncomingID} {
		if _, err := kg.semantic.collection.GetByID(context.Background(), docID); err != nil {
			t.Fatalf("expected semantic document %q before merge: %v", docID, err)
		}
	}

	if err := kg.MergeNodes("target", "source"); err != nil {
		t.Fatalf("MergeNodes: %v", err)
	}

	for _, docID := range []string{"source", sourceOutgoingID, sourceIncomingID} {
		if _, err := kg.semantic.collection.GetByID(context.Background(), docID); err == nil {
			t.Fatalf("expected stale semantic document %q removed after merge", docID)
		}
	}

	targetOutgoingID := "edge://target\x00peer\x00connects_to"
	targetIncomingID := "edge://client\x00target\x00uses"
	for _, docID := range []string{"target", targetOutgoingID, targetIncomingID} {
		if _, err := kg.semantic.collection.GetByID(context.Background(), docID); err != nil {
			t.Fatalf("expected merged semantic document %q after merge: %v", docID, err)
		}
	}
}

func TestKGSearchForContextWildcardUsesImportantNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("proxmox", "Proxmox Server", map[string]string{"type": "software"}); err != nil {
		t.Fatalf("AddNode proxmox: %v", err)
	}
	if err := kg.AddNode("activity_xyz", "Activity XYZ", map[string]string{"type": "activity_entity"}); err != nil {
		t.Fatalf("AddNode activity_xyz: %v", err)
	}

	ctx := kg.SearchForContext("*", 5, 800)
	if !strings.Contains(ctx, "proxmox") {
		t.Fatalf("expected wildcard context to contain important node proxmox, got %q", ctx)
	}
	if strings.Contains(ctx, "activity_xyz") {
		t.Fatalf("expected wildcard context to exclude activity_entity, got %q", ctx)
	}
}
