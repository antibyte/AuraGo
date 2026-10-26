package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func addBoundsTestDocuments(t *testing.T, col *chromem.Collection, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("bounds-%d", i)
		// Spread the vectors so the nearest-neighbour order is stable.
		doc := chromem.Document{ID: id, Content: "content " + id, Embedding: []float32{1, float32(i) * 0.1, 0}}
		if err := col.AddDocument(t.Context(), doc); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

// chromem rejects nResults above the document count. A delete between the
// caller's Count() and the query must not discard the whole collection.
func TestQueryCollectionClampedAbsorbsCollectionShrinkingBeforeQuery(t *testing.T) {
	db := chromem.NewDB()
	col, err := db.GetOrCreateCollection("bounds", nil, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	if err != nil {
		t.Fatal(err)
	}
	query := []float32{1, 0, 0}

	if res, err := queryCollectionClamped(t.Context(), col, query, 3); err != nil || len(res) != 0 {
		t.Fatalf("empty collection: results=%v err=%v, want none and no error", res, err)
	}

	ids := addBoundsTestDocuments(t, col, 4)
	res, err := queryCollectionClamped(t.Context(), col, query, 10)
	if err != nil || len(res) != 4 {
		t.Fatalf("k above document count: got %d results, err=%v, want all 4", len(res), err)
	}

	// Two documents disappear after the count was read: the first query is
	// rejected by chromem and must be retried with the fresh count.
	shrank := false
	res, err = queryCollectionClampedWithHook(t.Context(), col, query, 4, func() {
		if shrank {
			return
		}
		shrank = true
		if err := col.Delete(t.Context(), nil, nil, ids[2], ids[3]); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || len(res) != 2 {
		t.Fatalf("shrinking collection: got %d results, err=%v, want the 2 remaining", len(res), err)
	}

	// The whole collection vanishes in the same window.
	shrank = false
	res, err = queryCollectionClampedWithHook(t.Context(), col, query, 2, func() {
		if shrank {
			return
		}
		shrank = true
		if err := col.Delete(t.Context(), nil, nil, ids[0], ids[1]); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || len(res) != 0 {
		t.Fatalf("emptied collection: got %d results, err=%v, want none and no error", len(res), err)
	}
}

// The backfill loop doubles k up to the count it read at the start. When the
// collection shrinks between rounds, the next round must not fail.
func TestQueryVisibleCollectionSurvivesCollectionShrinkingBetweenRounds(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	stm := cv.fileIndexMemory.Load()
	vectors := [][]float32{{1, 0, 0}, {0.9, 0.1, 0}, {0.6, 0.4, 0}, {0.3, 0.7, 0}}
	ids := make([]string, 0, len(vectors))
	for i, vector := range vectors {
		id := fmt.Sprintf("shrink-%d", i)
		doc := chromem.Document{ID: id, Content: "content " + id, Metadata: cv.addEmbeddingMetadata(nil), Embedding: vector}
		cv.mu.Lock()
		err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
		cv.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	// The two best candidates are archived, so the first round finds nothing
	// visible and the loop has to expand past the documents that get deleted.
	for _, id := range ids[:2] {
		if _, err := stm.db.Exec(`UPDATE memory_meta SET verification_status='archived' WHERE doc_id=?`, id); err != nil {
			t.Fatal(err)
		}
	}
	shrank := false
	results, err := cv.queryVisibleCollectionWithBackfillHook(t.Context(), cv.collection, []float32{1, 0, 0}, 2, func(int) {
		if shrank {
			return
		}
		shrank = true
		if err := cv.collection.Delete(t.Context(), nil, nil, ids[0], ids[1]); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatalf("query after concurrent delete: %v", err)
	}
	if len(results) != 2 || results[0].ID != ids[2] || results[1].ID != ids[3] {
		t.Fatalf("results = %+v, want the two visible survivors %v", results, ids[2:])
	}
}

// A non-positive topK cannot match anything: it must neither panic on the
// final slice nor pay for a query embedding.
func TestSearchWithNonPositiveTopKReturnsNothingWithoutEmbedding(t *testing.T) {
	var calls atomic.Int64
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		return []float32{1, 0, 0}, nil
	})
	defer cv.Close()
	addBoundsTestDocuments(t, cv.collection, 3)
	calls.Store(0)

	searches := map[string]func(topK int) ([]SearchResult, error){
		"SearchSimilarScoredContext": func(topK int) ([]SearchResult, error) {
			return cv.SearchSimilarScoredContext(t.Context(), "bounds query", topK)
		},
		"SearchMemoriesOnlyScoredContext": func(topK int) ([]SearchResult, error) {
			return cv.SearchMemoriesOnlyScoredContext(t.Context(), "bounds query", topK)
		},
	}
	for name, search := range searches {
		for _, topK := range []int{-1, 0} {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s(topK=%d) panicked: %v", name, topK, r)
					}
				}()
				results, err := search(topK)
				if err != nil || results != nil {
					t.Errorf("%s(topK=%d) = %v, %v; want nil, nil", name, topK, results, err)
				}
			}()
		}
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("query embedding was computed %d times for a non-positive topK", got)
	}
	// Sanity: a positive topK still reaches the embedding provider.
	if _, err := cv.SearchMemoriesOnlyScoredContext(t.Context(), "bounds query", 1); err != nil || calls.Load() == 0 {
		t.Fatalf("positive topK did not search: calls=%d err=%v", calls.Load(), err)
	}
}
