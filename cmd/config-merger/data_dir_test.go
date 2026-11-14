package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDirectoryQueryUsesConfigLocationWithoutWritingState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	source := []byte("directories:\n  data_dir: state/private\n")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	dir, err := resolvedDataDirectory(path)
	if err != nil || dir != filepath.Join(root, "state/private") {
		t.Fatalf("directory = %q: %v", dir, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("query created runtime state", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(source) {
		t.Fatal("query rewrote source configuration")
	}
	for _, data := range []string{"directories:\n  data_dir: ''\n", "directories: [invalid"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := resolvedDataDirectory(path); err == nil {
			t.Fatal("unusable configuration accepted")
		}
	}
}
