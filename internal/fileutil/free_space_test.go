package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreeDiskBytesMeasuresExistingDirectory(t *testing.T) {
	free, err := FreeDiskBytes(t.TempDir())
	if err != nil {
		t.Fatalf("FreeDiskBytes() error = %v", err)
	}
	if free <= 0 {
		t.Fatalf("FreeDiskBytes() = %d, want > 0", free)
	}
}

func TestFreeDiskBytesUsesNearestExistingParent(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "not", "created", "yet", "wikipedia")
	free, err := FreeDiskBytes(missing)
	if err != nil {
		t.Fatalf("FreeDiskBytes(missing) error = %v", err)
	}
	if free <= 0 {
		t.Fatalf("FreeDiskBytes(missing) = %d, want > 0", free)
	}
	if _, err := os.Stat(filepath.Join(root, "not")); !os.IsNotExist(err) {
		t.Fatalf("FreeDiskBytes created directories: stat error = %v", err)
	}
}

func TestFreeDiskBytesAcceptsFilePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "edition.zim.part")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if free, err := FreeDiskBytes(file); err != nil || free <= 0 {
		t.Fatalf("FreeDiskBytes(file) = %d, %v; want > 0, nil", free, err)
	}
}

func TestFreeDiskBytesRejectsEmptyPath(t *testing.T) {
	if _, err := FreeDiskBytes(""); err == nil {
		t.Fatal("FreeDiskBytes(\"\") error = nil, want error")
	}
}

func TestNearestExistingDirWalksUp(t *testing.T) {
	root := t.TempDir()
	got, err := nearestExistingDir(filepath.Join(root, "a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("nearestExistingDir() = %q, want %q", got, root)
	}
}
