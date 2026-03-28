package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteFilesystemReadFileRejectsBinaryContent(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "image.png")
	data := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 32)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	raw := ExecuteFilesystem("read_file", "image.png", "", "", workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "binary file") {
		t.Fatalf("expected binary guidance, got: %s", result.Message)
	}
}

func TestExecuteFilesystemReadFileReturnsTextContent(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write text fixture: %v", err)
	}

	raw := ExecuteFilesystem("read_file", "notes.txt", "", "", workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}
	if got, _ := result.Data.(string); got != "hello world" {
		t.Fatalf("data = %q, want hello world", got)
	}
}
