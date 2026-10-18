package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRestrictToOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private")
	if err := os.WriteFile(path, []byte("fixture"), 0666); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dir, path} {
		if err := RestrictToOwner(p); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
		info, _ = os.Stat(dir)
		if info.Mode().Perm() != 0700 {
			t.Fatal(info.Mode())
		}
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "fixture" {
		t.Fatalf("owner cannot read: %v", err)
	}
}
