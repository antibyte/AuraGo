package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReaderReadLines(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("read_lines", "test.txt", "", 2, 4, 0, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["total_read"].(float64) != 3 {
		t.Fatalf("expected 3 lines, got %v", data["total_read"])
	}
	if data["content"].(string) != "line2\nline3\nline4" {
		t.Fatalf("unexpected content: %s", data["content"])
	}
}

func TestFileReaderHead(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("head", "test.txt", "", 0, 0, 3, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["total_read"].(float64) != 3 {
		t.Fatalf("expected 3 lines, got %v", data["total_read"])
	}
}

func TestFileReaderTail(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("tail", "test.txt", "", 0, 0, 2, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["content"].(string) != "line4\nline5" {
		t.Fatalf("unexpected content: %q", data["content"])
	}
}

func TestFileReaderCountLines(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("count_lines", "test.txt", "", 0, 0, 0, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["lines"].(float64) != 3 {
		t.Fatalf("expected 3 lines, got %v", data["lines"])
	}
}

func TestFileReaderSearchContext(t *testing.T) {
	dir := t.TempDir()
	content := "aaa\nbbb\nccc\nTARGET\nddd\neee\nfff\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("search_context", "test.txt", "TARGET", 0, 0, 2, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["count"].(float64) != 1 {
		t.Fatalf("expected 1 match, got %v", data["count"])
	}
}

func TestFileReaderSearchContextMatchNearEOF(t *testing.T) {
	dir := t.TempDir()
	content := "aaa\nbbb\nccc\nTARGET\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("search_context", "test.txt", "TARGET", 0, 0, 2, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["count"].(float64) != 1 {
		t.Fatalf("expected 1 match, got %v", data["count"])
	}
	matches := data["matches"].([]interface{})
	first := matches[0].(map[string]interface{})
	if first["content"].(string) != "bbb\nccc\nTARGET" {
		t.Fatalf("unexpected content: %q", first["content"])
	}
}

func TestFileReaderTailHandlesLargeLineCountsWithoutFullBufferExpectation(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result := ExecuteFileReaderAdvanced("tail", "test.txt", "", 0, 0, 50, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data := r.Data.(map[string]interface{})
	if data["start_line"].(float64) != 1 {
		t.Fatalf("expected start_line 1, got %v", data["start_line"])
	}
	if data["total_read"].(float64) != 5 {
		t.Fatalf("expected to read 5 lines, got %v", data["total_read"])
	}
}

func TestFileReaderMissingPath(t *testing.T) {
	result := ExecuteFileReaderAdvanced("read_lines", "", "", 1, 10, 0, t.TempDir())
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "error" {
		t.Fatalf("expected error for missing path")
	}
}

func TestFileReaderUnknownOp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("x"), 0644)
	result := ExecuteFileReaderAdvanced("invalid", "test.txt", "", 0, 0, 0, dir)
	var r FileReaderResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "error" {
		t.Fatalf("expected error for unknown op")
	}
}
