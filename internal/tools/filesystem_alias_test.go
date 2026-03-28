package tools

import (
	"strings"
	"testing"
)

func TestNormalizeFilesystemOperationAliases(t *testing.T) {
	if got := normalizeFilesystemOperation("read"); got != "read_file" {
		t.Fatalf("normalizeFilesystemOperation(read) = %q, want read_file", got)
	}
	if got := normalizeFilesystemOperation("write"); got != "write_file" {
		t.Fatalf("normalizeFilesystemOperation(write) = %q, want write_file", got)
	}
}

func TestExecuteFilesystemAcceptsReadAlias(t *testing.T) {
	dir := t.TempDir()
	result := ExecuteFilesystem("write_file", "note.txt", "", "hello", nil, dir)
	if !strings.Contains(result, `"status":"success"`) {
		t.Fatalf("write_file failed unexpectedly: %s", result)
	}

	aliasResult := ExecuteFilesystem("read", "note.txt", "", "", nil, dir)
	if !strings.Contains(aliasResult, `"status":"success"`) {
		t.Fatalf("read alias should succeed, got: %s", aliasResult)
	}
	if !strings.Contains(aliasResult, "hello") {
		t.Fatalf("expected file contents in alias read result, got: %s", aliasResult)
	}
}
