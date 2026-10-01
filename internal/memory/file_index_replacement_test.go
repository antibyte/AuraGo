package memory

import (
	"aurago/internal/chunking"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/philippgille/chromem-go"
)

func TestIndexedFileReplacementKeepsActiveGenerationOnFailure(t *testing.T) {
	var fail atomic.Bool
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		if fail.Load() {
			return nil, errors.New("provider failed")
		}
		return []float32{1, 0, 0}, nil
	})
	defer cv.Close()
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(t.TempDir(), "guide.md")
	if err := os.WriteFile(path, []byte("Previous complete guide"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cv.IndexDirectory(filepath.Dir(path), "documentation", s, true); err != nil {
		t.Fatal(err)
	}
	old, err := s.GetFileEmbeddingDocIDs(path, "documentation")
	if err != nil || len(old) != 1 {
		t.Fatalf("old=%v err=%v", old, err)
	}
	fail.Store(true)
	if err := os.WriteFile(path, []byte(strings.Repeat("Changed complete guide.\n\n", 500)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cv.IndexDirectory(filepath.Dir(path), "documentation", s, true); err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	active, err := s.GetFileEmbeddingDocIDs(path, "documentation")
	if err != nil || !slices.Equal(old, active) {
		t.Fatalf("active=%v old=%v err=%v", active, old, err)
	}
	if body, err := cv.GetByIDFromCollection(old[0], "documentation"); err != nil || !strings.Contains(body, "Previous") {
		t.Fatalf("old content=%q err=%v", body, err)
	}
	fail.Store(false)
	if err := cv.IndexDirectory(filepath.Dir(path), "documentation", s, true); err != nil {
		t.Fatal(err)
	}
	active, err = s.GetFileEmbeddingDocIDs(path, "documentation")
	if err != nil || slices.Equal(active, old) {
		t.Fatalf("generation not replaced: %v %v", active, err)
	}
	if _, err := cv.db.GetCollection("documentation", nil).GetByID(context.Background(), old[0]); err == nil {
		t.Fatal("owned obsolete vector survived successful replacement")
	}
}

func TestFileIndexPublishRechecksPointerAndKeepsOldDataOnSQLFailure(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	defer cv.Close()
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "file.txt")
	state := FileIndexState{LastModified: time.Now().UTC(), ContentHash: "first", IndexFingerprint: "v1"}
	old, err := cv.ReplaceIndexedFile(ctx, s, path, "file_index", "file", "Old document", nil, chunking.DefaultOptions(), nil, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_file_publish BEFORE UPDATE ON file_indices BEGIN SELECT RAISE(ABORT,'controlled failure'); END`); err != nil {
		t.Fatal(err)
	}
	state.ContentHash = "second"
	if _, err := cv.ReplaceIndexedFile(ctx, s, path, "file_index", "file", "New document", nil, chunking.DefaultOptions(), nil, state); err == nil {
		t.Fatal("SQL failure was lost")
	}
	active, err := s.GetFileEmbeddingDocIDs(path, "file_index")
	if err != nil || !slices.Equal(active, old) {
		t.Fatalf("active=%v err=%v", active, err)
	}
	if _, err := cv.GetByID(old[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_file_publish`); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetFileIndexState(path, "file_index")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PublishFileIndex(path, "file_index", before, []string{"stale-pointer"}, state, []string{"other"}); err == nil {
		t.Fatal("stale full pointer was accepted")
	}
}

func TestFileIndexRecoveryUsesPublishedPointerAfterRestart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	s, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	defer cv.Close()
	ctx := context.Background()
	col, err := cv.db.GetOrCreateCollection("file_index", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "file.txt")
	key := fileReplacementPrefix + contentSHA256("file_index") + ".test"
	doc := chromem.Document{ID: "file_staged", Content: "staged", Metadata: map[string]string{"source_path": path, "collection": "file_index", "index_generation": key}}
	if err := col.AddDocument(ctx, doc); err != nil {
		t.Fatal(err)
	}
	record := fileReplacement{Version: 1, Path: path, Collection: "file_index", New: []fileDocumentReceipt{{ID: doc.ID, Hash: contentSHA256(doc.Content)}}}
	if err := s.saveFileReplacement(key, record); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cv.fileIndexMemory.Store(s)
	if _, err := cv.GetByIDFromCollection(doc.ID, "file_index"); err == nil {
		t.Fatal("unpublished staging was visible")
	}
	if err := cv.RecoverIndexedFiles(ctx, s, "file_index"); err != nil {
		t.Fatal(err)
	}
	if _, err := col.GetByID(ctx, doc.ID); err == nil {
		t.Fatal("unpublished owned generation survived recovery")
	}
	if err := col.AddDocument(ctx, doc); err != nil {
		t.Fatal(err)
	}
	if err := s.saveFileReplacement(key, record); err != nil {
		t.Fatal(err)
	}
	if err := s.PublishFileIndex(path, "file_index", FileIndexState{}, nil, FileIndexState{LastModified: time.Now()}, []string{doc.ID}); err != nil {
		t.Fatal(err)
	}
	if err := cv.RecoverIndexedFiles(ctx, s, "file_index"); err != nil {
		t.Fatal(err)
	}
	if body, err := cv.GetByIDFromCollection(doc.ID, "file_index"); err != nil || body != doc.Content {
		t.Fatalf("committed generation lost: %q %v", body, err)
	}
}

func TestFileIndexRecoveryRetainsReferencedAndChangedDocuments(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	defer cv.Close()
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	col, err := cv.db.GetOrCreateCollection("file_index", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "file.txt")
	key := fileReplacementPrefix + contentSHA256("file_index") + ".protected"
	for _, id := range []string{"owned", "changed", "referenced"} {
		if err := col.AddDocument(ctx, chromem.Document{ID: id, Content: "original", Metadata: map[string]string{"source_path": path}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdateFileIndexWithDocs("other-path", "other-collection", time.Now(), []string{"referenced"}); err != nil {
		t.Fatal(err)
	}
	if err := col.AddDocument(ctx, chromem.Document{ID: "changed", Content: "intervening edit", Metadata: map[string]string{"source_path": path}}); err != nil {
		t.Fatal(err)
	}
	record := fileReplacement{Version: 1, Path: path, Collection: "file_index"}
	for _, id := range []string{"owned", "changed", "referenced"} {
		record.New = append(record.New, fileDocumentReceipt{ID: id, Hash: contentSHA256("original")})
	}
	if err := s.saveFileReplacement(key, record); err != nil {
		t.Fatal(err)
	}
	if err := cv.RecoverIndexedFiles(ctx, s, "file_index"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"changed", "referenced"} {
		if _, err := col.GetByID(ctx, id); err != nil {
			t.Fatalf("retained %s lost: %v", id, err)
		}
	}
	if _, err := col.GetByID(ctx, "owned"); err == nil {
		t.Fatal("owned unreferenced staging survived")
	}
}

func TestFileIndexRecoveryKeepsFailedDeletionReceiptUntilReload(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(filepath.Join(root, "memory.db"), logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	vectorPath := filepath.Join(root, "vectors")
	db, err := chromem.NewPersistentDB(vectorPath, false)
	if err != nil {
		t.Fatal(err)
	}
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	cv.db = db
	defer cv.Close()
	col, err := db.GetOrCreateCollection("file_index", nil, cv.embeddingFunc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "document.txt")
	id := "file_failed_delete"
	doc := chromem.Document{ID: id, Content: "unpublished", Metadata: map[string]string{"source_path": path}}
	if err := col.AddDocument(ctx, doc); err != nil {
		t.Fatal(err)
	}
	key := fileReplacementPrefix + contentSHA256("file_index") + ".failed"
	record := fileReplacement{Version: 1, Path: path, Collection: "file_index", New: []fileDocumentReceipt{{ID: id, Hash: contentSHA256(doc.Content)}}}
	if err := s.saveFileReplacement(key, record); err != nil {
		t.Fatal(err)
	}
	// Replace only this synthetic vector file with a nonempty directory to force a disk deletion failure.
	diskPath := filepath.Join(vectorPath, contentSHA256("file_index")[:8], contentSHA256(id)[:8]+".gob")
	backup := diskPath + ".saved"
	if err := os.Rename(diskPath, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(diskPath, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(diskPath, "blocked")
	if err := os.WriteFile(marker, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cv.RecoverIndexedFiles(ctx, s, "file_index"); err == nil {
		t.Fatal("disk deletion failure was lost")
	}
	if value, err := s.GetMemoryMaintenanceState(key); err != nil || value == "" {
		t.Fatalf("recovery receipt lost: %v", err)
	}
	if err := cv.RecoverIndexedFiles(ctx, s, "file_index"); err == nil {
		t.Fatal("in-memory absence discarded a failed disk deletion")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(diskPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, diskPath); err != nil {
		t.Fatal(err)
	}
	if err := cv.Close(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := chromem.NewPersistentDB(vectorPath, false)
	if err != nil {
		t.Fatal(err)
	}
	next := newTestChromemVectorDB(t, cv.embeddingFunc)
	next.db = reloaded
	defer next.Close()
	if err := next.RecoverIndexedFiles(ctx, s, "file_index"); err != nil {
		t.Fatal(err)
	}
	if value, err := s.GetMemoryMaintenanceState(key); err != nil || value != "" {
		t.Fatalf("completed recovery retained receipt: %v", err)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("recovered vector still on disk: %v", err)
	}
}
