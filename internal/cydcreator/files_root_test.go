package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRootReadFileStaysInsideAndBounded(t *testing.T) {
	base := t.TempDir()
	inside := filepath.Join(base, "project")
	if err := os.MkdirAll(filepath.Join(inside, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "secret.txt"), []byte("secret"), 0o640); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(inside)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := rootWriteFile(context.Background(), root, "sub/data.txt", []byte("12345")); err != nil {
		t.Fatal(err)
	}

	if data, err := rootReadFile(root, "sub/data.txt", 5); err != nil || !bytes.Equal(data, []byte("12345")) {
		t.Fatalf("read at the limit = %q, %v", data, err)
	}
	if _, err := rootReadFile(root, "sub/data.txt", 4); !errors.Is(err, errFileTooLarge) {
		t.Fatalf("oversize read = %v, want errFileTooLarge", err)
	}
	if _, err := rootReadFile(root, "sub", 1<<10); !errors.Is(err, errNotRegularFile) {
		t.Fatalf("directory read = %v, want errNotRegularFile", err)
	}
	if _, err := rootReadFile(root, "missing.txt", 1<<10); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read = %v, want not exist", err)
	}
	for _, rel := range []string{"../secret.txt", "sub/../../secret.txt", filepath.ToSlash(filepath.Join(base, "secret.txt"))} {
		if data, err := rootReadFile(root, rel, 1<<10); err == nil {
			t.Errorf("rootReadFile(%q) escaped the root: %q", rel, data)
		}
	}
	entries, err := os.ReadDir(filepath.Join(inside, "sub"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left after the write: %v, %v", entries, err)
	}
}
