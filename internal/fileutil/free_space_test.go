package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// skipWithoutFreeSpaceProbe skips on platforms that have no free-space probe (errors.ErrUnsupported).
func skipWithoutFreeSpaceProbe(t *testing.T) {
	t.Helper()
	if _, err := FreeDiskBytes(t.TempDir()); errors.Is(err, errors.ErrUnsupported) {
		t.Skip("no free-space probe on this platform")
	}
}

func TestFreeDiskBytesMeasuresExistingDirectory(t *testing.T) {
	skipWithoutFreeSpaceProbe(t)
	free, err := FreeDiskBytes(t.TempDir())
	if err != nil {
		t.Fatalf("FreeDiskBytes() error = %v", err)
	}
	if free <= 0 {
		t.Fatalf("FreeDiskBytes() = %d, want > 0", free)
	}
}

func TestFreeDiskBytesUsesNearestExistingParent(t *testing.T) {
	skipWithoutFreeSpaceProbe(t)
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
	skipWithoutFreeSpaceProbe(t)
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

func TestNearestExistingDirWalksUpFromPathBelowAFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "edition.zim")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := nearestExistingDir(filepath.Join(file, "child", "grandchild"))
	if err != nil {
		t.Fatalf("nearestExistingDir(below file) error = %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("nearestExistingDir(below file) = %q, want %q", got, root)
	}
}

func TestNearestExistingDirSurfacesNonMissingStatErrors(t *testing.T) {
	// A NUL byte makes os.Stat fail with EINVAL on every platform, which is neither
	// "does not exist" nor "not a directory": it must be returned, not walked past.
	bad := filepath.Join(t.TempDir(), "bad\x00name")
	if got, err := nearestExistingDir(bad); err == nil {
		t.Fatalf("nearestExistingDir(NUL path) = %q, nil; want an error", got)
	}
	if free, err := FreeDiskBytes(bad); err == nil {
		t.Fatalf("FreeDiskBytes(NUL path) = %d, nil; want an error", free)
	}
}
