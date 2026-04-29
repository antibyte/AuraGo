package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInitialPasswordFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-password")
	if err := os.WriteFile(path, []byte("correct horse battery staple\n"), 0600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	got, err := resolveInitialPassword("", path)
	if err != nil {
		t.Fatalf("resolveInitialPassword returned error: %v", err)
	}
	if got != "correct horse battery staple" {
		t.Fatalf("password = %q", got)
	}
}

func TestResolveInitialPasswordRejectsFlagAndFileTogether(t *testing.T) {
	_, err := resolveInitialPassword("secret", "secret-file")
	if err == nil {
		t.Fatal("expected error when both password flag and password file are set")
	}
}
