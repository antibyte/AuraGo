package memory

import (
	"context"
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
