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

func TestExecuteFilesystemReadFileLargeFileIncludesGuidance(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "large.log")
	content := strings.Repeat("0123456789abcdef", 3000)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write text fixture: %v", err)
	}

	raw := ExecuteFilesystem("read_file", "large.log", "", "", workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}
	if !strings.Contains(result.Message, "smart_file_read") || !strings.Contains(result.Message, "file_reader_advanced") {
		t.Fatalf("expected large-file guidance in message, got: %s", result.Message)
	}
}

func TestExecuteFilesystemResolveErrorIncludesPathContext(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	raw := ExecuteFilesystem("read_file", "../../../etc/passwd", "", "", workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured error data, got %T", result.Data)
	}
	if data["error_code"] != "path_resolution_error" {
		t.Fatalf("error_code = %v, want path_resolution_error", data["error_code"])
	}
	if data["requested_path"] != "../../../etc/passwd" {
		t.Fatalf("requested_path = %v", data["requested_path"])
	}
	if _, ok := data["workspace_root"].(string); !ok {
		t.Fatalf("workspace_root missing from error data: %#v", data)
	}
}

func TestExecuteFilesystemReadErrorIncludesResolvedPath(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	raw := ExecuteFilesystem("read_file", "missing.txt", "", "", workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured error data, got %T", result.Data)
	}
	if data["error_code"] != "io_error" {
		t.Fatalf("error_code = %v, want io_error", data["error_code"])
	}
	if _, ok := data["resolved_path"].(string); !ok {
		t.Fatalf("resolved_path missing from error data: %#v", data)
	}
}
