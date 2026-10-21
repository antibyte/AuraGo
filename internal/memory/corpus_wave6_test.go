package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestIndexToolGuidesUsesOneFileSnapshotForHashAndDocuments(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	toolsDir := t.TempDir()
	guidePath := filepath.Join(toolsDir, "zz_wave6_snapshot.md")
	oldContent := "# Snapshot guide\n\nOld snapshot phrase."
	newContent := "# Snapshot guide\n\nNew snapshot phrase."
	if err := os.WriteFile(guidePath, []byte(oldContent), 0o600); err != nil {
		t.Fatal(err)
	}
	initialFiles, err := loadToolGuideFilesWithWarnings(toolsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	initialHash := "disclosure-v2:" + cv.computeToolGuidesHashSnapshot(initialFiles)

	var once sync.Once
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) {
		var writeErr error
		once.Do(func() { writeErr = os.WriteFile(guidePath, []byte(newContent), 0o600) })
		if writeErr != nil {
			return nil, writeErr
		}
		return []float32{1, 0, 0}, nil
	}
	if err := cv.IndexToolGuides(toolsDir, true); err != nil {
		t.Fatalf("index initial snapshot: %v", err)
	}
	storedHash, err := os.ReadFile(filepath.Join(cv.dataDir, ".tool_guides_hash"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(storedHash)) != initialHash {
		t.Fatalf("stored hash = %q, want hash for initial file snapshot %q", strings.TrimSpace(string(storedHash)), initialHash)
	}

	if err := cv.IndexToolGuides(toolsDir, false); err != nil {
		t.Fatalf("reindex changed snapshot: %v", err)
	}
	collection, err := cv.db.GetOrCreateCollection("tool_guides", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := collection.GetByID(t.Context(), "tool_zz_wave6_snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Content, "New snapshot phrase") {
		t.Fatalf("document content = %q, want latest guide content", doc.Content)
	}
}

func TestIndexToolGuidesDoesNotAdvanceHashAfterCorpusCleanupFailure(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	cv.embeddingFunc = func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil }
	toolsDir := t.TempDir()
	removedPath := filepath.Join(toolsDir, "zz_wave6_cleanup.md")
	if err := os.WriteFile(removedPath, []byte("# Cleanup guide\n\nThis guide will be removed."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "zz_wave6_keep.md"), []byte("# Kept guide\n\nThis guide remains."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cv.IndexToolGuides(toolsDir, true); err != nil {
		t.Fatalf("initial guide index: %v", err)
	}
	markerPath := filepath.Join(cv.dataDir, ".tool_guides_hash")
	oldHash, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpus.db.Exec(`CREATE TRIGGER fail_toolguide_cleanup BEFORE DELETE ON documents WHEN OLD.collection='tool_guides' AND OLD.id='tool_zz_wave6_cleanup' BEGIN SELECT RAISE(FAIL,'injected corpus cleanup failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(removedPath); err != nil {
		t.Fatal(err)
	}
	if err := cv.IndexToolGuides(toolsDir, true); err == nil {
		t.Fatal("reindex error = nil, want stale corpus cleanup failure")
	}
	newHash, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newHash) != string(oldHash) {
		t.Fatalf("hash marker advanced after cleanup failure: got %q, want %q", newHash, oldHash)
	}
}

func TestVisibleCandidatesBatchesMoreThanFiveHundredCurrentChecks(t *testing.T) {
	const count = 620
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	stm := cv.fileIndexMemory.Load()
	results := make([]chromem.Result, 0, count)
	tx, err := cv.corpus.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO collections(name) VALUES('aurago_memories')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("batch-%04d", i)
		metadata := cv.addEmbeddingMetadata(map[string]string{"batch": fmt.Sprint(i)})
		if i < 10 {
			metadata["index_generation"] = "generation-1"
			metadata["source_path"] = filepath.Join(t.TempDir(), fmt.Sprintf("source-%d.md", i))
			metadata["collection"] = "aurago_memories"
		}
		doc := chromem.Document{ID: id, Content: "batch content " + id, Metadata: metadata, Embedding: []float32{1, 0, 0}}
		if err := writeCorpusDocument(t.Context(), tx, "aurago_memories", doc); err != nil {
			t.Fatal(err)
		}
		results = append(results, chromem.Result{ID: id, Content: doc.Content, Metadata: metadata})
	}
	if _, err := tx.Exec(`UPDATE documents SET pending=0 WHERE collection='aurago_memories'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE documents SET pending=1 WHERE collection='aurago_memories' AND id>=? AND id<?`, "batch-0040", "batch-0050"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		key := results[i].Metadata["source_path"]
		if _, err := stm.db.Exec(`INSERT INTO file_embedding_docs(file_path,collection,doc_id) VALUES(?,?,?)`, filepath.Clean(key), "aurago_memories", results[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	for i := 20; i < 40; i++ {
		if err := stm.UpsertMemoryMetaWithDetails(results[i].ID, MemoryMetaUpdate{VerificationStatus: "archived"}); err != nil {
			t.Fatal(err)
		}
	}

	eligible, err := cv.visibleCandidates(t.Context(), "aurago_memories", results)
	if err != nil {
		t.Fatalf("visibleCandidates: %v", err)
	}
	if len(eligible) != count {
		t.Fatalf("eligibility count = %d, want %d", len(eligible), count)
	}
	visible := 0
	for i, ok := range eligible {
		want := true
		if i >= 4 && i < 10 || i >= 20 && i < 40 || i >= 40 && i < 50 {
			want = false
		}
		if ok != want {
			t.Fatalf("candidate %d eligibility = %v, want %v", i, ok, want)
		}
		if ok {
			visible++
		}
	}
	if visible != count-6-20-10 {
		t.Fatalf("visible count = %d, want %d", visible, count-6-20-10)
	}
}

func TestQueryVisibleCollectionRechecksEligibilityBeforeReturningBackfill(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	stm := cv.fileIndexMemory.Load()
	items := []struct {
		id        string
		embedding []float32
	}{
		{id: "wave6-first", embedding: []float32{1, 0, 0}},
		{id: "wave6-second", embedding: []float32{0.8, 0.6, 0}},
		{id: "wave6-third", embedding: []float32{0, 1, 0}},
	}
	for _, item := range items {
		doc := chromem.Document{
			ID:        item.id,
			Content:   "content " + item.id,
			Metadata:  cv.addEmbeddingMetadata(nil),
			Embedding: item.embedding,
		}
		cv.mu.Lock()
		err := cv.persistDocumentsLocked(t.Context(), cv.collection, []chromem.Document{doc})
		cv.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if err := stm.UpsertMemoryMeta(item.id); err != nil {
			t.Fatal(err)
		}
	}
	initial, err := cv.collection.QueryEmbedding(t.Context(), []float32{1, 0, 0}, 1, nil, nil)
	if err != nil || len(initial) != 1 || initial[0].ID != items[0].id {
		t.Fatalf("initial top candidate = %+v, %v; want %q", initial, err, items[0].id)
	}

	var rounds []int
	archived := false
	results, err := cv.queryVisibleCollectionWithBackfillHook(t.Context(), cv.collection, []float32{1, 0, 0}, 1, func(k int) {
		rounds = append(rounds, k)
		if k == 1 && !archived {
			if _, err := stm.db.Exec(`UPDATE memory_meta SET verification_status='archived' WHERE doc_id=?`, items[0].id); err != nil {
				t.Fatalf("archive top candidate between rounds: %v", err)
			}
			archived = true
		}
	})
	if err != nil {
		t.Fatalf("query with changing eligibility: %v", err)
	}
	if len(rounds) < 2 || rounds[0] != 1 || rounds[1] <= rounds[0] {
		t.Fatalf("backfill rounds = %v, want expansion after the first candidate became archived", rounds)
	}
	if len(results) != 1 || results[0].ID != items[1].id {
		t.Fatalf("visible results = %+v, want active backfill candidate %q", results, items[1].id)
	}
}
