package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexDirectoryAcceptsShippedDocumentation(t *testing.T) {
	const dir = "../../documentation"
	cv := newIndexBatchTestDB(t, &recordingEmbedder{})
	stm := newIndexDirectoryMemory(t)
	if err := cv.IndexDirectory(dir, "documentation", stm, true); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		checked++
		ids, err := stm.GetFileEmbeddingDocIDs(filepath.Join(dir, file.Name()), "documentation")
		if err != nil || len(ids) == 0 {
			t.Errorf("documentation %s has no indexed chunks: %v", file.Name(), err)
		}
	}
	if checked == 0 {
		t.Fatal("no shipped documentation was checked")
	}
}
