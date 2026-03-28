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

	raw := ExecuteFilesystem("read_file", "image.png", "", "", nil, workdir)
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

	raw := ExecuteFilesystem("read_file", "notes.txt", "", "", nil, workdir)
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

	raw := ExecuteFilesystem("read_file", "large.log", "", "", nil, workdir)
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

	raw := ExecuteFilesystem("read_file", "../../../etc/passwd", "", "", nil, workdir)
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

	raw := ExecuteFilesystem("read_file", "missing.txt", "", "", nil, workdir)
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

func TestExecuteFilesystemCopyFile(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "source.txt"), []byte("copy me"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	raw := ExecuteFilesystem("copy", "source.txt", "copies/destination.txt", "", nil, workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	got, err := os.ReadFile(filepath.Join(workdir, "copies", "destination.txt"))
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "copy me" {
		t.Fatalf("copied content = %q, want copy me", string(got))
	}
}

func TestExecuteFilesystemCopyBatchPartial(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	items := []map[string]interface{}{
		{"file_path": "a.txt", "destination": "out/a.txt"},
		{"file_path": "../../../etc/passwd", "destination": "out/passwd"},
	}
	raw := ExecuteFilesystem("copy_batch", "", "", "", items, workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured data, got %T", result.Data)
	}
	summary, ok := data["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("summary missing: %#v", data)
	}
	if summary["succeeded"] != float64(1) || summary["failed"] != float64(1) {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}

func TestExecuteFilesystemDeleteBatch(t *testing.T) {
	workdir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(workdir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	items := []map[string]interface{}{
		{"file_path": "a.txt"},
		{"file_path": "b.txt"},
	}
	raw := ExecuteFilesystem("delete_batch", "", "", "", items, workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(workdir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be deleted, stat err=%v", name, err)
		}
	}
}

func TestExecuteFilesystemCreateDirBatch(t *testing.T) {
	workdir := t.TempDir()
	items := []map[string]interface{}{
		{"file_path": "one"},
		{"file_path": "two/nested"},
	}
	raw := ExecuteFilesystem("create_dir_batch", "", "", "", items, workdir)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	for _, rel := range []string{"one", filepath.Join("two", "nested")} {
		info, err := os.Stat(filepath.Join(workdir, rel))
		if err != nil || !info.IsDir() {
			t.Fatalf("%s should exist as directory, err=%v", rel, err)
		}
	}
}
