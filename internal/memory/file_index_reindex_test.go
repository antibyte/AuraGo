package memory

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"aurago/internal/chunking"
)

func TestDocumentCorpusAvailableMultimodalSourceReindexKeepsRawIDs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	stm := cv.fileIndexMemory.Load()
	path := filepath.Join(t.TempDir(), "source.png")
	source := []byte("available image source")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	state := FileIndexState{LastModified: time.Now().UTC(), ContentHash: contentSHA256(string(source)), IndexFingerprint: "old-model"}
	ids, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "file_index", "Image", "original image descriptor", []float32{1, 0, 0}, chunking.DefaultOptions(), nil, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := stm.EnsureMemoryMetaWithDetails(ids[0], MemoryMetaUpdate{VerificationStatus: "confirmed", SourceType: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMetaProtection(ids[0], true, true); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleDocumentReindex(dir); err != nil {
		t.Fatal(err)
	}
	if err := stm.ScheduleFileSourceReindex(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0,1,0]}]}`))
	}))
	defer server.Close()
	// This is the existing file indexer's authorized source embedding helper.
	embedder := NewMultimodalEmbedder(server.URL, "", "new-model", "openai", "", cv.logger)
	vector, err := embedder.EmbedFile(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	cv.embeddingFingerprint = "new-model"
	state.IndexFingerprint = "new-model"
	updated, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "file_index", "Image", "original image descriptor", vector, chunking.DefaultOptions(), nil, state)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, updated) {
		t.Fatalf("reindex changed raw IDs: %v -> %v", ids, updated)
	}
	doc, err := cv.corpusDocument(t.Context(), "file_index", ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if doc.Metadata["embedding_fingerprint"] != "new-model" || !slices.Equal(doc.Embedding, vector) {
		t.Fatalf("existing source was not regenerated: %+v", doc.Metadata)
	}
	meta, err := stm.GetMemoryMeta(ids[0])
	if err != nil || !meta.Protected || !meta.KeepForever || meta.VerificationStatus != "confirmed" {
		t.Fatalf("reindex changed curation: %+v %v", meta, err)
	}
	tracked, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil || !slices.Equal(ids, tracked) {
		t.Fatalf("reindex changed file pointer: %v %v", tracked, err)
	}
}
