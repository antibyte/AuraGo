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
