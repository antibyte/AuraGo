package memory

import (
	"path/filepath"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func TestMemoryExpiryRoundTripAndClear(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	ctx := t.Context()
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("fresh expiry ok=%v err=%v", ok, err)
	}
	at := time.Date(2026, 11, 2, 23, 59, 59, 0, time.FixedZone("CET", 3600))
	if err := s.SetMemoryExpiry(ctx, []string{"doc", " ", "other"}, at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.MemoryExpiry(ctx, "doc")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("expiry=%v ok=%v err=%v, want %v", got, ok, err, at)
	}
	if err := s.SetMemoryExpiry(ctx, []string{"doc"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("cleared expiry ok=%v err=%v", ok, err)
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "other"); !ok {
		t.Fatal("clearing one document removed another")
	}
}

func TestExpiredMemoriesStayOutOfRetrieval(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	stm := cv.fileIndexMemory.Load()
	for _, id := range []string{"expired", "active"} {
		doc := chromem.Document{ID: id, Content: "topic\n\n" + id + " fact", Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
		cv.mu.Lock()
		err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
		cv.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.SetMemoryExpiry(t.Context(), []string{"expired"}, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryExpiry(t.Context(), []string{"active"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	results, err := cv.queryVisibleCollection(t.Context(), cv.collection, []float32{1, 0, 0}, 2)
	if err != nil || len(results) != 1 || results[0].ID != "active" {
		t.Fatalf("expired memory visible: %+v %v", results, err)
	}
}
