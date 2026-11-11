package memory

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/chunking"
	"aurago/internal/config"
	chromem "github.com/philippgille/chromem-go"
)

func TestCorpusLimitAppliesToEveryConnection(t *testing.T) {
	const limit = 96 << 10
	c, err := openDocumentCorpus(t.TempDir(), limit)
	if err != nil {
		t.Fatal(err)
	}
	defer c.db.Close()
	if _, err := c.db.Exec(`INSERT INTO collections(name) VALUES('fixture')`); err != nil {
		t.Fatal(err)
	}
	connections := []*sql.Conn{}
	for i := 0; i < 4; i++ {
		conn, err := c.db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, conn)
		defer conn.Close()
	}
	for _, conn := range connections {
		var pages, size int64
		if err := conn.QueryRowContext(t.Context(), `PRAGMA max_page_count`).Scan(&pages); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(t.Context(), `PRAGMA page_size`).Scan(&size); err != nil {
			t.Fatal(err)
		}
		if pages*size != limit {
			t.Fatalf("connection bypasses native cap: %d", pages*size)
		}
		_, err := conn.ExecContext(t.Context(), `INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint) VALUES('fixture','too-big',?,'{}','[]',0,'')`, strings.Repeat("large payload ", 12000))
		var coded interface{ Code() int }
		if !errors.As(err, &coded) || coded.Code()&255 != 13 {
			t.Fatalf("native limit not enforced: %v", err)
		}
	}
}

func TestCorpusLimitPausesLegacyMigrationWithoutLosingSources(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	legacy, err := chromem.NewPersistentDB(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	col, err := legacy.GetOrCreateCollection("aurago_memories", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := chromem.Document{ID: "legacy", Content: strings.Repeat("retained legacy content ", 10000), Embedding: []float32{1, 0, 0}}
	if err := col.AddDocument(t.Context(), doc); err != nil {
		t.Fatal(err)
	}
	c, err := openDocumentCorpus(dir, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	defer c.db.Close()
	if !c.indexPaused || c.pauseReason != "corpus_limit_exceeded" {
		t.Fatalf("migration was not paused: %+v", c)
	}
	var count int
	if err := c.db.QueryRow(`SELECT count(*) FROM documents`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial migration: %d %v", count, err)
	}
	reopened, err := chromem.NewPersistentDB(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.GetCollection("aurago_memories", nil).GetByID(t.Context(), doc.ID)
	if err != nil || got.Content != doc.Content {
		t.Fatalf("legacy source lost: %v", err)
	}
}

func TestCorpusDiskReserveRejectsBeforeEmbeddingAndAllowsDeletion(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	doc := chromem.Document{ID: "old", Content: "previous complete document", Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
	if err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc}); err != nil {
		t.Fatal(err)
	}
	cv.corpus.freeDisk = func(string) (int64, error) { return CorpusDiskReserveBytes + 1, nil }
	calls := 0
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) { calls++; return []float32{1, 0, 0}, nil }
	for i := 0; i < 3; i++ {
		err := cv.embedDocumentsOutsideLock(t.Context(), []chromem.Document{{ID: "new", Content: "new document"}})
		if !errors.Is(err, ErrCorpusCapacity) {
			t.Fatal(err)
		}
	}
	if calls != 0 {
		t.Fatal("capacity retry called embedding provider")
	}
	status := cv.CorpusStatus(t.Context())
	if !status.PublicationPaused || status.PauseReason != "corpus_disk_reserve" || status.WALBytes < 0 {
		t.Fatalf("missing reserve status: %+v", status)
	}
	if err := cv.deleteDocumentsWithCorpus(t.Context(), cv.collection, nil, doc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpusDocument(t.Context(), cv.collection.Name, doc.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deletion blocked: %v", err)
	}
}

func TestOversizeCorpusPausesIndexWithoutDeletingDocuments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	doc := chromem.Document{ID: "retained", Content: strings.Repeat("retained content ", 80000), Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
	if err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc}); err != nil {
		t.Fatal(err)
	}
	cv.Close()
	c, err := openDocumentCorpus(dir, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	defer c.db.Close()
	if !c.indexPaused {
		t.Fatal("oversize index was not paused")
	}
	cv.corpus = c
	if err := cv.loadCorpusIndex(t.Context()); !errors.Is(err, ErrCorpusCapacity) {
		t.Fatalf("oversize index loaded: %v", err)
	}
	status := cv.CorpusStatus(t.Context())
	if !status.Available || !status.CapacityWarning || !status.IndexPaused || status.PauseReason != "corpus_limit_exceeded" {
		t.Fatalf("status: %+v", status)
	}
	if got, err := cv.corpusDocument(t.Context(), cv.collection.Name, doc.ID); err != nil || got.Content != doc.Content {
		t.Fatalf("old content lost: %v", err)
	}
	cfg := &config.Config{}
	cfg.Directories.VectorDBDir = dir
	cfg.Embeddings.Provider = "disabled"
	cfg.Embeddings.CorpusMaxSizeMB = 1
	// A reopened runtime above a smaller test cap still admits administrative reads.
	if err := c.checkCapacity(t.Context(), 1); !errors.Is(err, ErrCorpusCapacity) {
		t.Fatal(err)
	}
	c.db.Close()
	runtime, err := NewChromemVectorDB(cfg, cv.logger)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if !runtime.ready.Load() || !runtime.IsDisabled() || runtime.collection.Count() != 0 || !runtime.CorpusStatus(t.Context()).IndexPaused {
		t.Fatal("oversize corpus blocked startup or loaded semantic index")
	}
}

func TestCorpusLimitRejectedFileReplacementKeepsActiveGeneration(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	stm := cv.fileIndexMemory.Load()
	path := filepath.Join(t.TempDir(), "document.txt")
	state := FileIndexState{LastModified: time.Now(), ContentHash: "old", IndexFingerprint: "fixture"}
	old, err := cv.ReplaceIndexedFile(t.Context(), stm, path, "file_index", "file", "previous complete document", nil, chunking.DefaultOptions(), nil, state)
	if err != nil {
		t.Fatal(err)
	}
	used, err := cv.corpus.usedBytes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cv.corpus.limitBytes = used + 4096
	state.ContentHash = "replacement"
	_, err = cv.ReplaceIndexedFile(t.Context(), stm, path, "file_index", "file", strings.Repeat("changed source ", 2000), nil, chunking.DefaultOptions(), nil, state)
	if !errors.Is(err, ErrCorpusCapacity) {
		t.Fatalf("replacement was not refused: %v", err)
	}
	active, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil || !slices.Equal(active, old) {
		t.Fatalf("active generation changed: %v", err)
	}
	if body, err := cv.GetByIDFromCollection(old[0], "file_index"); err != nil || !strings.Contains(body, "previous complete document") {
		t.Fatalf("old index lost: %v", err)
	}
}
