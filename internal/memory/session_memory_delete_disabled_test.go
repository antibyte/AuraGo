package memory

import (
	"path/filepath"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestPrivacyDeleteCanonicalSourcesWhileEmbeddingsDisabledKeepsSharedAndProtected(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	stm := cv.fileIndexMemory.Load()
	for _, id := range []string{"exclusive", "shared", "protected", "unknown"} {
		doc := chromem.Document{ID: id, Content: "complete private source " + id, Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
		cv.mu.Lock()
		err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
		cv.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.EnsureMemoryMetaWithDetails(id, MemoryMetaUpdate{SourceType: "memory_analysis"}); err != nil {
			t.Fatal(err)
		}
		if err := stm.RecordMemoryExtractionSource(id, "memory_analysis", "private"); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.RecordMemoryExtractionSource("shared", "memory_analysis", "other"); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMetaProtection("protected", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE memory_meta SET source_type='unknown' WHERE doc_id='unknown'`); err != nil {
		t.Fatal(err)
	}
	cv.disabled.Store(true)
	cv.ready.Store(false)
	cv.db = nil
	cv.collection = nil
	report, err := stm.DeleteChatSessionWithMemories(t.Context(), "private", cv, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.DeletedMemories != 1 || report.RetainedMemories != 3 {
		t.Fatalf("unsafe disabled deletion: %+v", report)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	if cv.Count() != 3 {
		t.Fatalf("disabled deletion was not durable: %d", cv.Count())
	}
	for _, id := range []string{"shared", "protected", "unknown"} {
		if source, err := cv.corpusDocument(t.Context(), "aurago_memories", id); err != nil || source.Content != "complete private source "+id {
			t.Fatalf("unrelated source %s changed: %v", id, err)
		}
	}
	meta, err := stm.GetMemoryMeta("protected")
	if err != nil || !meta.Protected || !meta.KeepForever {
		t.Fatalf("protection changed: %+v %v", meta, err)
	}
	sources, err := stm.GetMemoryExtractionSources("shared")
	if err != nil || len(sources) != 2 {
		t.Fatalf("shared provenance changed: %+v %v", sources, err)
	}
}
