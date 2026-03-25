package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeEditorResult(t *testing.T, raw string) FileEditorResult {
	t.Helper()
	var r FileEditorResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("failed to decode result: %v — raw: %s", err, raw)
	}
	return r
}

func setupEditorTest(t *testing.T, filename, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "agent_workspace", "workdir")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	fpath := filepath.Join(wsDir, filename)
	if content != "" {
		if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return wsDir, filename
}

func TestFileEditorStrReplace(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "hello world\nfoo bar\nhello world\n")

	// Unique match
	res := decodeEditorResult(t, ExecuteFileEditor("str_replace", fname, "foo bar", "baz qux", "", "", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.Contains(string(data), "baz qux") {
		t.Fatalf("expected replaced text, got: %s", data)
	}

	// Non-unique match should fail
	res = decodeEditorResult(t, ExecuteFileEditor("str_replace", fname, "hello world", "bye", "", "", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for non-unique match, got %s", res.Status)
	}
	if !strings.Contains(res.Message, "2 times") {
		t.Fatalf("expected count in message, got: %s", res.Message)
	}
}

func TestFileEditorStrReplaceAll(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "aaa\nbbb\naaa\n")

	res := decodeEditorResult(t, ExecuteFileEditor("str_replace_all", fname, "aaa", "ccc", "", "", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if strings.Contains(string(data), "aaa") {
		t.Fatalf("expected all replacements, got: %s", data)
	}
	if strings.Count(string(data), "ccc") != 2 {
		t.Fatalf("expected 2 replacements, got: %s", data)
	}
}

func TestFileEditorStrReplaceNotFound(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "hello world\n")

	res := decodeEditorResult(t, ExecuteFileEditor("str_replace", fname, "nonexistent", "replacement", "", "", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
	if !strings.Contains(res.Message, "not found") {
		t.Fatalf("expected 'not found' in message, got: %s", res.Message)
	}
}

func TestFileEditorInsertAfter(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\nline3\n")

	res := decodeEditorResult(t, ExecuteFileEditor("insert_after", fname, "", "", "line2", "inserted_line", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	lines := strings.Split(string(data), "\n")
	if len(lines) < 4 || lines[2] != "inserted_line" {
		t.Fatalf("expected inserted line after line2, got: %v", lines)
	}
}

func TestFileEditorInsertBefore(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\nline3\n")

	res := decodeEditorResult(t, ExecuteFileEditor("insert_before", fname, "", "", "line2", "inserted_line", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	lines := strings.Split(string(data), "\n")
	if len(lines) < 4 || lines[1] != "inserted_line" {
		t.Fatalf("expected inserted line before line2, got: %v", lines)
	}
}

func TestFileEditorInsertMarkerNotFound(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\n")

	res := decodeEditorResult(t, ExecuteFileEditor("insert_after", fname, "", "", "nonexistent", "content", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}

func TestFileEditorInsertMarkerMultiple(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "dup\nother\ndup\n")

	res := decodeEditorResult(t, ExecuteFileEditor("insert_after", fname, "", "", "dup", "content", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for ambiguous marker, got %s", res.Status)
	}
}

func TestFileEditorAppend(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "existing content")

	res := decodeEditorResult(t, ExecuteFileEditor("append", fname, "", "", "", "new tail", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.HasSuffix(string(data), "new tail") {
		t.Fatalf("expected appended content, got: %s", data)
	}
}

func TestFileEditorAppendCreatesFile(t *testing.T) {
	wsDir, _ := setupEditorTest(t, "dummy.txt", "")
	fname := "new_file.txt"

	res := decodeEditorResult(t, ExecuteFileEditor("append", fname, "", "", "", "fresh content", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, err := os.ReadFile(filepath.Join(wsDir, fname))
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(data) != "fresh content" {
		t.Fatalf("expected 'fresh content', got: %s", data)
	}
}

func TestFileEditorPrepend(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "existing content")

	res := decodeEditorResult(t, ExecuteFileEditor("prepend", fname, "", "", "", "new head", 0, 0, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.HasPrefix(string(data), "new head") {
		t.Fatalf("expected prepended content, got: %s", data)
	}
}

func TestFileEditorDeleteLines(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\nline3\nline4\nline5\n")

	res := decodeEditorResult(t, ExecuteFileEditor("delete_lines", fname, "", "", "", "", 2, 4, 0, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.LinesChanged != 3 {
		t.Fatalf("expected 3 lines deleted, got %d", res.LinesChanged)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	text := string(data)
	if strings.Contains(text, "line2") || strings.Contains(text, "line3") || strings.Contains(text, "line4") {
		t.Fatalf("expected lines 2-4 deleted, got: %s", text)
	}
	if !strings.Contains(text, "line1") || !strings.Contains(text, "line5") {
		t.Fatalf("expected lines 1 and 5 preserved, got: %s", text)
	}
}

func TestFileEditorDeleteLinesOutOfRange(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\n")

	res := decodeEditorResult(t, ExecuteFileEditor("delete_lines", fname, "", "", "", "", 5, 10, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for out-of-range, got %s", res.Status)
	}
}

func TestFileEditorDeleteLinesInvalidRange(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "line1\nline2\n")

	res := decodeEditorResult(t, ExecuteFileEditor("delete_lines", fname, "", "", "", "", 0, 1, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for start_line < 1, got %s", res.Status)
	}

	res = decodeEditorResult(t, ExecuteFileEditor("delete_lines", fname, "", "", "", "", 3, 2, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for end < start, got %s", res.Status)
	}
}

func TestFileEditorMissingFilePath(t *testing.T) {
	res := decodeEditorResult(t, ExecuteFileEditor("str_replace", "", "a", "b", "", "", 0, 0, 0, "."))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}

func TestFileEditorUnknownOperation(t *testing.T) {
	wsDir, fname := setupEditorTest(t, "test.txt", "content")
	res := decodeEditorResult(t, ExecuteFileEditor("invalid_op", fname, "", "", "", "", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}

func TestFileEditorPathTraversal(t *testing.T) {
	wsDir, _ := setupEditorTest(t, "test.txt", "content")
	res := decodeEditorResult(t, ExecuteFileEditor("str_replace", "../../../etc/passwd", "root", "hacked", "", "", 0, 0, 0, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for path traversal, got %s", res.Status)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic_test.txt")

	// Write new file
	if err := writeFileAtomic(path, []byte("test content")); err != nil {
		t.Fatalf("writeFileAtomic failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(data) != "test content" {
		t.Fatalf("expected 'test content', got: %s", data)
	}

	// Overwrite existing file
	if err := writeFileAtomic(path, []byte("updated")); err != nil {
		t.Fatalf("writeFileAtomic overwrite failed: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should exist after overwrite: %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("expected 'updated', got: %s", data)
	}
}
