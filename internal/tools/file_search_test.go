package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSearchGrep(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nfoo bar\nhello again\n"), 0644)

	result := ExecuteFileSearch("grep", "hello", "test.txt", "", "", dir)
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	matches, ok := r.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array data, got %T", r.Data)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestFileSearchGrepCount(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\nfoo bar\nhello again\n"), 0644)

	result := ExecuteFileSearch("grep", "hello", "test.txt", "", "count", dir)
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data, ok := r.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data")
	}
	if data["count"].(float64) != 2 {
		t.Fatalf("expected count 2, got %v", data["count"])
	}
}

func TestFileSearchGrepRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("target line\n"), 0644)
	os.WriteFile(filepath.Join(sub, "b.txt"), []byte("another target\n"), 0644)
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("no match here\n"), 0644)

	result := ExecuteFileSearch("grep_recursive", "target", "", "*.txt", "", dir)
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	matches, ok := r.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array data")
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestFileSearchFind(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(dir, "file1.yaml"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(sub, "file2.yaml"), []byte("y"), 0644)
	os.WriteFile(filepath.Join(dir, "file3.txt"), []byte("z"), 0644)

	result := ExecuteFileSearch("find", "*.yaml", "", "*.yaml", "", dir)
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "success" {
		t.Fatalf("expected success, got %s: %s", r.Status, r.Message)
	}
	data, ok := r.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data")
	}
	if data["count"].(float64) != 2 {
		t.Fatalf("expected 2 files, got %v", data["count"])
	}
}

func TestFileSearchMissingPattern(t *testing.T) {
	result := ExecuteFileSearch("grep", "", "test.txt", "", "", t.TempDir())
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "error" {
		t.Fatalf("expected error for missing pattern")
	}
}

func TestFileSearchUnknownOp(t *testing.T) {
	result := ExecuteFileSearch("invalid", "x", "test.txt", "", "", t.TempDir())
	var r FileSearchResult
	json.Unmarshal([]byte(result), &r)
	if r.Status != "error" {
		t.Fatalf("expected error for unknown op")
	}
}
