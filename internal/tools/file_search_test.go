package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGrepFileBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	content := "hello world\nfoo bar\nhello again\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches, err := grepFile(path, regexp.MustCompile("(?i)hello"), "test.txt")
	if err != nil {
		t.Fatalf("grepFile failed: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("len(matches) = %d, want 2", len(matches))
	}
}

func TestGrepFileMatchLimit(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "many.txt")

	// Create a file with more than grepFileMaxMatchesPerFile matches
	var b strings.Builder
	for i := 0; i < 15000; i++ {
		b.WriteString("match\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches, err := grepFile(path, regexp.MustCompile("match"), "many.txt")
	if err != nil {
		t.Fatalf("grepFile failed: %v", err)
	}
	if len(matches) != grepFileMaxMatchesPerFile {
		t.Errorf("len(matches) = %d, want %d (max per file)", len(matches), grepFileMaxMatchesPerFile)
	}
}

func TestFileGrepBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	content := "hello world\nfoo bar\nhello again\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := fileGrep("test.txt", "hello", "match", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	matchesRaw, ok := parsed.Data.([]interface{})
	if !ok {
		t.Fatal("expected array data")
	}
	if len(matchesRaw) != 2 {
		t.Errorf("len(matches) = %d, want 2", len(matchesRaw))
	}
}

func TestFileGrepEmptyPattern(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Use ExecuteFileSearch which validates empty pattern at the top level
	result := ExecuteFileSearch("grep", "", "test.txt", "", "match", workdir)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "error" {
		t.Fatalf("status = %q, want error", parsed.Status)
	}
}

func TestFileGrepRecursiveBasic(t *testing.T) {
	workdir := t.TempDir()
	// Create nested structure
	subdir := filepath.Join(workdir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workdir, "a.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "b.txt"), []byte("hello there"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := fileGrepRecursive("*.txt", "hello", "match", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	matches := parsed.Data.([]interface{})
	if len(matches) != 2 {
		t.Errorf("len(matches) = %d, want 2", len(matches))
	}
}

func TestFileGrepRecursiveSkipsLargeFiles(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "large.log")

	// Create a file larger than grepRecursiveMaxFileSize (10 MB) using sparse file
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	largeSize := grepRecursiveMaxFileSize + 1
	if _, err := f.Seek(int64(largeSize)-1, 0); err != nil {
		f.Close()
		t.Fatalf("seek: %v", err)
	}
	if _, err := f.Write([]byte{0}); err != nil {
		f.Close()
		t.Fatalf("write: %v", err)
	}
	f.Close()

	// Also create a small file with matches
	smallPath := filepath.Join(workdir, "small.log")
	if err := os.WriteFile(smallPath, []byte("match line 1\nmatch line 2\n"), 0644); err != nil {
		t.Fatalf("write small: %v", err)
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := fileGrepRecursive("*.log", "match", "match", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	matches := parsed.Data.([]interface{})
	// Should only find matches in small.log, not large.log
	if len(matches) != 2 {
		t.Errorf("len(matches) = %d, want 2 (only from small.log)", len(matches))
	}
	for _, mRaw := range matches {
		m := mRaw.(map[string]interface{})
		if m["file"] != "small.log" {
			t.Errorf("unexpected file: %v", m["file"])
		}
	}
}

func TestFileGrepRecursiveMaxResults(t *testing.T) {
	workdir := t.TempDir()

	// Create many small files, each with one match, to exceed grepRecursiveMaxResults (500)
	for i := 0; i < 600; i++ {
		// Use i directly in filename to ensure uniqueness
		name := fmt.Sprintf("file%04d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("match"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := fileGrepRecursive("**/*.txt", "match", "match", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	matches := parsed.Data.([]interface{})
	if len(matches) != grepRecursiveMaxResults {
		t.Errorf("len(matches) = %d, want %d (max results)", len(matches), grepRecursiveMaxResults)
	}
}

func TestFileFindBasic(t *testing.T) {
	workdir := t.TempDir()
	subdir := filepath.Join(workdir, "sub")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"a.txt", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	for _, name := range []string{"d.txt", "e.go"} {
		if err := os.WriteFile(filepath.Join(subdir, name), []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := fileFind("*.txt", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	filesRaw, ok := data["files"].([]interface{})
	if !ok {
		t.Fatal("expected files to be an array")
	}
	if len(filesRaw) != 3 {
		t.Errorf("len(files) = %d, want 3", len(filesRaw))
	}
}

func TestExecuteFileSearchGrep(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result := ExecuteFileSearch("grep", "hello", "test.txt", "", "match", workdir)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
}

func TestExecuteFileSearchInvalidOperation(t *testing.T) {
	workdir := t.TempDir()

	result := ExecuteFileSearch("invalid", "", "", "", "", workdir)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "error" {
		t.Fatalf("status = %q, want error", parsed.Status)
	}
}

func TestFileSearchConstants(t *testing.T) {
	// Verify constants are properly defined
	if grepRecursiveMaxResults <= 0 {
		t.Error("grepRecursiveMaxResults should be positive")
	}
	if grepRecursiveMaxFileSize <= 0 {
		t.Error("grepRecursiveMaxFileSize should be positive")
	}
	if grepFileMaxMatchesPerFile <= 0 {
		t.Error("grepFileMaxMatchesPerFile should be positive")
	}
}

func TestGrepFileEmptyFile(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	matches, err := grepFile(path, regexp.MustCompile("(?i)hello"), "empty.txt")
	if err != nil {
		t.Fatalf("grepFile failed: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("len(matches) = %d, want 0", len(matches))
	}
}

func TestFileGrepRecursiveSkipsBinaryLikeFiles(t *testing.T) {
	workdir := t.TempDir()

	// Create a small file with binary-like content (null bytes)
	path := filepath.Join(workdir, "binary.log")
	data := []byte{0x00, 0x01, 0x02, 'm', 'a', 't', 'c', 'h', 0x00}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	// This tests that we don't crash on binary content
	result := fileGrepRecursive("*.log", "match", "match", workdir, encode)
	var parsed FileSearchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Status should still be success (file is skipped due to size or read error)
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
}

// Helper for regexp import in tests - compile once
var _ = regexp.MustCompile
