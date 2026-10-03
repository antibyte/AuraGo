package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/philippgille/chromem-go"
)

type failingVectorDeleter struct{}

func (failingVectorDeleter) Delete(context.Context, map[string]string, map[string]string, ...string) error {
	return errors.New("vector store unavailable")
}

func TestIndexDirectoryRetainsTrackingWhenStaleVectorDeleteFails(t *testing.T) {
	cv := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) {
		return []float32{1, 0, 0}, nil
	})
	defer cv.Close()
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(path, []byte("Guide that will be removed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cv.IndexDirectory(dir, "documentation", s, true); err != nil {
		t.Fatal(err)
	}
	docIDs, err := s.GetFileEmbeddingDocIDs(path, "documentation")
	if err != nil || len(docIDs) == 0 {
		t.Fatalf("docIDs=%v err=%v", docIDs, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	original := markdownVectorDeleter
	markdownVectorDeleter = func(*chromem.Collection) vectorDocDeleter { return failingVectorDeleter{} }
	err = cv.IndexDirectory(dir, "documentation", s, false)
	markdownVectorDeleter = original
	if err == nil || !strings.Contains(err.Error(), "retain tracking") {
		t.Fatalf("IndexDirectory error = %v, want retain tracking failure", err)
	}
	tracked, err := s.ListIndexedFiles("documentation")
	if err != nil || !slices.Contains(tracked, path) {
		t.Fatalf("tracking dropped after failed vector delete: tracked=%v err=%v", tracked, err)
	}
	if kept, err := s.GetFileEmbeddingDocIDs(path, "documentation"); err != nil || !slices.Equal(kept, docIDs) {
		t.Fatalf("doc ids = %v, want %v (err %v)", kept, docIDs, err)
	}

	if err := cv.IndexDirectory(dir, "documentation", s, false); err != nil {
		t.Fatalf("retry after vector store recovery: %v", err)
	}
	tracked, err = s.ListIndexedFiles("documentation")
	if err != nil || slices.Contains(tracked, path) {
		t.Fatalf("tracking kept after successful cleanup: tracked=%v err=%v", tracked, err)
	}
	if _, err := cv.db.GetCollection("documentation", nil).GetByID(context.Background(), docIDs[0]); err == nil {
		t.Fatal("stale vector survived successful cleanup")
	}
}
