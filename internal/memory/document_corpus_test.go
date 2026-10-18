package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func newCorpusTestVectorDB(t *testing.T, dir string) *ChromemVectorDB {
	t.Helper()
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	c, err := openDocumentCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	cv.corpus = c
	cv.dataDir = dir
	cv.embeddingDimension.Store(3)
	stm, err := NewSQLiteMemory(filepath.Join(t.TempDir(), "metadata.db"), cv.logger)
	if err != nil {
		t.Fatal(err)
	}
	cv.BindMemoryMetadata(stm)
	t.Cleanup(func() { cv.Close(); stm.Close() })
	if err := cv.loadCorpusIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	return cv
}

func TestDocumentCorpusKeepsRawIDsAndCollidingCollectionNamesAcrossRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	names := []string{"raw_71894", "raw_73468"}
	left, right := sha256.Sum256([]byte(names[0])), sha256.Sum256([]byte(names[1]))
	if string(left[:4]) != string(right[:4]) {
		t.Fatal("fixture hashes no longer collide")
	}
	for _, name := range names {
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range append(names, " raw/ID?%_ä ") {
			doc := chromem.Document{ID: id, Content: name + ":" + id, Metadata: cv.addEmbeddingMetadata(map[string]string{"origin": "test"}), Embedding: []float32{1, 0, 0}}
			cv.mu.Lock()
			err = cv.persistDocumentsLocked(t.Context(), col, []chromem.Document{doc})
			cv.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if cv.Count() != 6 {
		t.Fatalf("count=%d", cv.Count())
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	for _, name := range names {
		for _, id := range append(names, " raw/ID?%_ä ") {
			got, err := cv.GetByIDFromCollection(id, name)
			if err != nil || got != name+":"+id {
				t.Fatalf("raw identity lost: %q %q %v", name, id, err)
			}
		}
	}
	if cv.Count() != 6 {
		t.Fatalf("restart count=%d", cv.Count())
	}
}

func TestDocumentCorpusMigratesLegacyWithBackupAndReindexesModelChange(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "vectors")
	legacy, err := chromem.NewPersistentDB(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aurago_memories", "custom/collection"} {
		col, err := legacy.GetOrCreateCollection(name, map[string]string{"kind": "legacy"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := col.AddDocument(t.Context(), chromem.Document{ID: " raw?ID_ä ", Content: "complete original source", Metadata: map[string]string{"embedding_fingerprint": "old-model", "provenance": "original"}, Embedding: []float32{1, 0}}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := legacy.GetCollection("aurago_memories", nil).GetByID(t.Context(), " raw?ID_ä ")
	if err != nil {
		t.Fatal(err)
	}
	cv := newCorpusTestVectorDB(t, dir)
	if cv.collection.Count() != 0 {
		t.Fatal("incompatible legacy vectors became searchable")
	}
	got, err := cv.GetByID(" raw?ID_ä ")
	if err != nil || got != before.Content {
		t.Fatalf("source not readable: %q %v", got, err)
	}
	backups, err := filepath.Glob(dir + ".legacy-backup-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	backup, err := chromem.NewPersistentDB(backups[0], false)
	if err != nil {
		t.Fatal(err)
	}
	preserved, err := backup.GetCollection("aurago_memories", nil).GetByID(t.Context(), before.ID)
	if err != nil || preserved.Content != before.Content {
		t.Fatal("backup content differs")
	}
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	doc, err := cv.corpusDocument(t.Context(), "aurago_memories", before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Embedding) != 3 || doc.Metadata["provenance"] != "original" || doc.Metadata["embedding_fingerprint"] != cv.EmbeddingFingerprint() {
		t.Fatalf("wrong regenerated vector/provenance: %+v", doc.Metadata)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	if cv.collection.Count() != 1 || cv.Count() != 2 {
		t.Fatalf("restart index=%d corpus=%d", cv.collection.Count(), cv.Count())
	}
	if backups, _ := filepath.Glob(dir + ".legacy-backup-*"); len(backups) != 1 {
		t.Fatalf("migration repeated: %v", backups)
	}
}

func TestDocumentCorpusRejectsCorruptAndOrphanLegacyWithoutVerifyingEmpty(t *testing.T) {
	for _, kind := range []string{"corrupt", "missing_metadata", "orphan"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "vectors")
			sub := filepath.Join(dir, "abcdef12")
			if err := os.MkdirAll(sub, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(sub, "00000000.gob")
			if kind == "missing_metadata" {
				path = filepath.Join(sub, "12345678.gob")
			}
			if kind == "orphan" {
				path = filepath.Join(dir, "12345678.gob")
			}
			original := []byte("original corrupt gob")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if c, err := openDocumentCorpus(dir); err == nil {
				c.db.Close()
				t.Fatal("incomplete legacy store was accepted")
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != string(original) {
				t.Fatalf("original changed: %v", err)
			}
			db, err := sql.Open("sqlite", filepath.Join(dir, "documents.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var verified string
			if err := db.QueryRow(`SELECT value FROM corpus_meta WHERE key='verified'`).Scan(&verified); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("failed migration marked verified: %q %v", verified, err)
			}
		})
	}
}

func TestDocumentCorpusPendingWriteReconcilesAfterRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	tx, err := cv.corpus.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	doc := chromem.Document{ID: "raw-id", Content: "recoverable pending source", Metadata: map[string]string{"embedding_fingerprint": "old"}, Embedding: []float32{0, 1, 0}}
	if err := writeCorpusDocument(t.Context(), tx, "aurago_memories", doc); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if cv.collection.Count() != 0 {
		t.Fatal("fixture accidentally indexed")
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	if cv.collection.Count() != 0 {
		t.Fatal("pending vector loaded")
	}
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	if cv.collection.Count() != 1 {
		t.Fatal("pending work was lost")
	}
	var pending int
	if err := cv.corpus.db.QueryRow(`SELECT pending FROM documents WHERE id='raw-id'`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("pending=%d %v", pending, err)
	}
}

func TestDocumentCorpusCheatsheetFailurePreservesOldSourceAndIndex(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	if err := cv.StoreCheatsheet("guide", "Guide", "original source"); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpus.db.Exec(`CREATE TRIGGER reject_new_source BEFORE INSERT ON documents BEGIN SELECT RAISE(FAIL,'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := cv.StoreCheatsheet("guide", "Guide", "replacement source"); err == nil {
		t.Fatal("expected persistence failure")
	}
	doc, err := cv.corpusDocument(t.Context(), "aurago_memories", "cs_guide")
	if err != nil || !strings.Contains(doc.Content, "original source") {
		t.Fatalf("previous source lost: %v", err)
	}
	cached, err := cv.collection.GetByID(t.Context(), "cs_guide")
	if err != nil || cached.Content != doc.Content {
		t.Fatalf("cache changed before source commit: %v", err)
	}
}

func TestDocumentCorpusArchivesDoNotConsumeTopKOrDeduplicate(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "vectors"))
	stm := cv.fileIndexMemory.Load()
	for _, id := range []string{"archived", "timestamp_archived", "active"} {
		doc := chromem.Document{ID: id, Content: "topic\n\nidentical complete fact", Embedding: []float32{1, 0, 0}, Metadata: cv.addEmbeddingMetadata(nil)}
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
	if _, err := stm.db.Exec(`UPDATE memory_meta SET verification_status='archived' WHERE doc_id='archived'; UPDATE memory_meta SET archived_at=CURRENT_TIMESTAMP WHERE doc_id='timestamp_archived'`); err != nil {
		t.Fatal(err)
	}
	results, err := cv.queryVisibleCollection(t.Context(), cv.collection, []float32{1, 0, 0}, 1)
	if err != nil || len(results) != 1 || results[0].ID != "active" {
		t.Fatalf("archive consumed topK: %+v %v", results, err)
	}
	id, err := cv.findIdenticalMemory(t.Context(), "topic", "identical complete fact", "", []float32{1, 0, 0})
	if err != nil || id != "active" {
		t.Fatalf("archived dedupe: %q %v", id, err)
	}
	if _, err := stm.db.Exec(`UPDATE memory_meta SET archived_at=CURRENT_TIMESTAMP WHERE doc_id='active'`); err != nil {
		t.Fatal(err)
	}
	id, err = cv.findIdenticalMemory(t.Context(), "topic", "identical complete fact", "", []float32{1, 0, 0})
	if err != nil || id != "" {
		t.Fatalf("reused archived fact: %q %v", id, err)
	}
}

func TestDocumentCorpusMultimodalResetRetainsSourceAndReportsBlockedWork(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	cv := newCorpusTestVectorDB(t, dir)
	id, err := cv.StoreDocumentWithEmbedding("picture", "image description", []float32{1, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleDocumentReindex(dir); err != nil {
		t.Fatal(err)
	}
	cv = newCorpusTestVectorDB(t, dir)
	if err := cv.reconcileCorpus(t.Context()); err != nil {
		t.Fatal(err)
	}
	if content, err := cv.GetByID(id); err != nil || content != "image description" {
		t.Fatalf("multimodal source lost: %v", err)
	}
	status := cv.EmbeddingStatus()
	if status.IndexPending != 1 || status.IndexBlocked != 1 || status.IndexError != "source_reindex_required" {
		t.Fatalf("blocked work invisible: %+v", status)
	}
	if cv.collection.Count() != 0 {
		t.Fatal("incompatible multimodal vector loaded")
	}
}
