package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingVariablesOnly(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("AURAGO_TEST_LOAD=from-file\nAURAGO_TEST_KEEP=from-file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("AURAGO_TEST_KEEP", "existing")
	_ = os.Unsetenv("AURAGO_TEST_LOAD")

	loadDotEnv(envPath, slog.Default())

	if got := os.Getenv("AURAGO_TEST_LOAD"); got != "from-file" {
		t.Fatalf("AURAGO_TEST_LOAD = %q, want from-file", got)
	}
	if got := os.Getenv("AURAGO_TEST_KEEP"); got != "existing" {
		t.Fatalf("AURAGO_TEST_KEEP = %q, want existing", got)
	}
}
