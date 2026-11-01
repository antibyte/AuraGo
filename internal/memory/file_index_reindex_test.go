package memory

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestDocumentCorpusMissingFileChunksRebuildFromSource(t *testing.T) {
	for _, missing := range []string{"one", "all"} {
		t.Run(missing, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			stm := cv.fileIndexMemory.Load()
			path := filepath.Join(t.TempDir(), "guide.md")
			content := strings.Repeat("An available source file can restore its missing search documents.\n", 150)
			state := FileIndexState{LastModified: time.Now().UTC(), ContentHash: contentSHA256(content), IndexFingerprint: "old-model"}
			old, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "documentation", "Guide", content, nil, chunking.DefaultOptions(), nil, state)
			if err != nil || len(old) < 2 {
				t.Fatalf("initial chunks: %v, %v", old, err)
			}
			if err := stm.EnsureMemoryMetaWithDetails(old[1], MemoryMetaUpdate{VerificationStatus: "confirmed", SourceType: "user"}); err != nil {
				t.Fatal(err)
			}
			if err := stm.SetMemoryMetaProtection(old[1], true, true); err != nil {
				t.Fatal(err)
			}
			lost := old[:1]
			if missing == "all" {
				lost = old
			}
			for _, id := range lost {
				if _, err := cv.corpus.db.Exec(`DELETE FROM documents WHERE collection=? AND id=?`, "documentation", id); err != nil {
					t.Fatal(err)
				}
			}
			cv.embeddingFingerprint = "new-model"
			state.IndexFingerprint = "new-model"
			updated, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "documentation", "Guide", content, nil, chunking.DefaultOptions(), nil, state)
			if err != nil || len(updated) != len(old) || slices.Equal(updated, old) {
				t.Fatalf("rebuild: old=%v updated=%v err=%v", old, updated, err)
			}
			tracked, err := stm.GetFileEmbeddingDocIDs(path, "documentation")
			if err != nil || !slices.Equal(updated, tracked) {
				t.Fatalf("published IDs = %v, %v; want %v", tracked, err, updated)
			}
			for _, id := range updated {
				doc, err := cv.corpusDocument(t.Context(), "documentation", id)
				if err != nil || doc.Metadata["embedding_fingerprint"] != "new-model" || len(doc.Embedding) == 0 {
					t.Fatalf("missing rebuilt chunk %s: %v", id, err)
				}
			}
			meta, err := stm.GetMemoryMeta(old[1])
			if err != nil || !meta.Protected || !meta.KeepForever || meta.VerificationStatus != "confirmed" {
				t.Fatalf("lost old curation: %+v, %v", meta, err)
			}
			if missing == "one" {
				if _, err := cv.corpusDocument(t.Context(), "documentation", old[1]); err != nil {
					t.Fatalf("removed surviving protected document: %v", err)
				}
			}
		})
	}
}

func TestDocumentCorpusUnreadableFileChunksKeepTracking(t *testing.T) {
	for _, failure := range []string{"closed store", "missing chunk and unreadable survivor"} {
		t.Run(failure, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
			stm := cv.fileIndexMemory.Load()
			path := filepath.Join(t.TempDir(), "guide.md")
			content := strings.Repeat("The complete source remains available.\n", 250)
			state := FileIndexState{LastModified: time.Now().UTC(), ContentHash: contentSHA256(content), IndexFingerprint: "old-model"}
			old, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "documentation", "Guide", content, nil, chunking.DefaultOptions(), nil, state)
			if err != nil || len(old) < 2 {
				t.Fatalf("initial chunks: %v, %v", old, err)
			}
			if failure == "closed store" {
				if err := cv.corpus.db.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := cv.corpus.db.Exec(`DELETE FROM documents WHERE collection=? AND id=?`, "documentation", old[0]); err != nil {
					t.Fatal(err)
				}
				if _, err := cv.corpus.db.Exec(`UPDATE documents SET metadata='{' WHERE collection=? AND id=?`, "documentation", old[1]); err != nil {
					t.Fatal(err)
				}
			}
			state.IndexFingerprint = "new-model"
			_, err = cv.ReplaceIndexedFile(t.Context(), stm, path, "documentation", "Guide", content, nil, chunking.DefaultOptions(), nil, state)
			if err == nil || !strings.Contains(err.Error(), "retain unchanged file") {
				t.Fatalf("unreadable corpus error = %v", err)
			}
			tracked, err := stm.GetFileEmbeddingDocIDs(path, "documentation")
			if err != nil || !slices.Equal(old, tracked) {
				t.Fatalf("changed file pointer after read failure: %v, %v", tracked, err)
			}
		})
	}
}
