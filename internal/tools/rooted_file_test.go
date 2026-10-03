package tools

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestWriteRootFromReaderAtomicKeepsDestinationOnReadError(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(dst, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	source := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errors.New("source failed")))
	if err := writeRootFromReaderAtomic(root, "dst.txt", source, 0o644, false); err == nil || !strings.Contains(err.Error(), "source failed") {
		t.Fatalf("write error = %v, want source failure", err)
	}
	if data, err := os.ReadFile(dst); err != nil || string(data) != "original" {
		t.Fatalf("destination = %q, %v; want original content", data, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".aurago_edit_*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestExecuteFilesystemCopyReplacesExistingDestination(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "source.txt"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "dest.txt"), []byte("old content that is longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	var result FSResult
	if err := json.Unmarshal([]byte(ExecuteFilesystem("copy", "source.txt", "dest.txt", "", nil, workdir, 0, 0)), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "success" {
		t.Fatalf("copy status = %s (%s)", result.Status, result.Message)
	}
	if data, err := os.ReadFile(filepath.Join(workdir, "dest.txt")); err != nil || string(data) != "new content" {
		t.Fatalf("destination = %q, %v", data, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(workdir, ".aurago_edit_*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}
