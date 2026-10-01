package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestMultiCollectionSearchPreservesResultsAndErrors(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	t.Cleanup(func() { _ = cv.Close() })
	if results, err := cv.SearchSimilarScored("empty", 3); err != nil || len(results) != 0 {
		t.Fatalf("empty search = %v, %v", results, err)
	}
	if err := cv.collection.AddDocument(context.Background(), chromem.Document{ID: "healthy", Content: "healthy memory", Embedding: []float32{1, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	bad, err := cv.db.GetOrCreateCollection("broken", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	cv.RegisterCollections([]string{"broken"})
	if err := bad.AddDocument(context.Background(), chromem.Document{ID: "old-model", Content: "incompatible memory", Embedding: []float32{1, 0}}); err != nil {
		t.Fatal(err)
	}
	for _, scored := range []bool{false, true} {
		var ids []string
		var err error
		if scored {
			results, resultErr := cv.SearchSimilarScored("query", 3)
			_, ids = splitSearchResults(results)
			err = resultErr
		} else {
			_, ids, err = cv.SearchSimilar("query", 3)
		}
		if err == nil || !strings.Contains(err.Error(), "broken") || len(ids) != 1 || ids[0] != "healthy" {
			t.Fatalf("scored=%v ids=%v err=%v", scored, ids, err)
		}
	}
	if _, err := cv.SearchSimilarScored("query", 3, "aurago_memories"); err == nil {
		t.Fatal("all failed collections must return an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cv.SearchSimilarScoredContext(ctx, "query", 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

func TestAuditMultiCollectionSearchReportsFailedCollection(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0}, nil })
	t.Cleanup(func() { _ = cv.Close() })
	if err := cv.collection.AddDocument(context.Background(), chromem.Document{ID: "old-model", Content: "audit data", Embedding: []float32{1, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.SearchMemoriesOnlyScored("audit query", 1); err == nil {
		t.Fatal("single-collection search must detect incompatible dimensions")
	}
	if results, err := cv.SearchSimilarScored("audit query", 1); err == nil {
		t.Fatalf("multi-collection search hid the same failure: results=%d", len(results))
	}
}
