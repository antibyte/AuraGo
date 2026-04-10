package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchContextBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.log")
	content := `line 1: INFO starting
line 2: DEBUG hello
line 3: INFO processing
line 4: ERROR failed
line 5: INFO done
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := searchContext(path, "ERROR", 1, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	matches := data["matches"].([]interface{})
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	if data["truncated"] != false {
		t.Errorf("truncated = %v, want false", data["truncated"])
	}
}

func TestSearchContextTooLargeFile(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "large.log")

	// Create a file larger than searchContextMaxFileSize (50 MB)
	// We don't actually write 50 MB in a unit test; instead we verify the
	// size-check path by creating a file that would be > 50 MB if fully written,
	// but we can verify the logic by checking the constant.
	largeSize := searchContextMaxFileSize + 1

	// Create a file with a known size using sparse file or just write enough to exceed
	// For unit test speed, we create a small file that would exceed the threshold
	// if the threshold were lower. To properly test, we need to test with actual
	// file size > limit. Let's create a file and truncate it to appear larger.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Write a small amount but seek to create sparse file
	if _, err := f.Seek(int64(largeSize)-1, 0); err != nil {
		f.Close()
		t.Fatalf("seek: %v", err)
	}
	if _, err := f.Write([]byte{0}); err != nil {
		f.Close()
		t.Fatalf("write: %v", err)
	}
	f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() <= searchContextMaxFileSize {
		t.Fatalf("test setup: file size %d should exceed limit %d", info.Size(), searchContextMaxFileSize)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := searchContext(path, "ERROR", 1, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "error" {
		t.Fatalf("status = %q, want error for oversized file", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "too large") {
		t.Errorf("message = %q, want 'too large' guidance", parsed.Message)
	}
	data, ok := parsed.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data with limit metadata")
	}
	if data["limit"] != "file_size" {
		t.Errorf("limit = %v, want file_size", data["limit"])
	}
}

func TestSearchContextManyMatches(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "many.log")

	// Generate a file with more than searchContextMaxMatches (50) occurrences
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('0' + i%10)))
		b.WriteString(": ERROR match\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := searchContext(path, "ERROR", 0, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	if data["truncated"] != true {
		t.Errorf("truncated = %v, want true", data["truncated"])
	}
	if data["limit"] != "max_matches" {
		t.Errorf("limit = %v, want max_matches", data["limit"])
	}
	if int(data["count"].(float64)) != searchContextMaxMatches {
		t.Errorf("count = %v, want %d", data["count"], searchContextMaxMatches)
	}
}

func TestSearchContextEmptyPattern(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.log")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := searchContext(path, "", 1, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "error" {
		t.Fatalf("status = %q, want error", parsed.Status)
	}
	if !strings.Contains(parsed.Message, "pattern") {
		t.Errorf("message = %q, want pattern error", parsed.Message)
	}
}

func TestSearchContextContextLines(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.log")
	content := `line 1: alpha
line 2: beta
line 3: ERROR gamma
line 4: delta
line 5: epsilon
line 6: ERROR zeta
line 7: eta
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := searchContext(path, "ERROR", 2, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	matches := data["matches"].([]interface{})

	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}

	// First match should have context before the ERROR line
	first := matches[0].(map[string]interface{})
	firstContent := first["content"].(string)
	if !strings.Contains(firstContent, "beta") || !strings.Contains(firstContent, "ERROR") || !strings.Contains(firstContent, "gamma") {
		t.Errorf("first match content = %q, want context around ERROR", firstContent)
	}
}

func TestReadLinesBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := readLines(path, 2, 4, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	if int(data["start_line"].(float64)) != 2 {
		t.Errorf("start_line = %v, want 2", data["start_line"])
	}
	if int(data["end_line"].(float64)) != 4 {
		t.Errorf("end_line = %v, want 4", data["end_line"])
	}
}

func TestReadTailBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('0' + i)))
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := readTail(path, 3, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	if int(data["total_read"].(float64)) != 3 {
		t.Errorf("total_read = %v, want 3", data["total_read"])
	}
}

func TestCountLinesBasic(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	encode := func(r FileReaderResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	result := countLines(path, encode)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
	data := parsed.Data.(map[string]interface{})
	if int(data["lines"].(float64)) != 5 {
		t.Errorf("lines = %v, want 5", data["lines"])
	}
}

func TestExecuteFileReaderAdvancedSearchContext(t *testing.T) {
	workdir := t.TempDir()
	path := filepath.Join(workdir, "test.log")
	content := "INFO start\nDEBUG debug\nERROR problem\nINFO end\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result := ExecuteFileReaderAdvanced("search_context", "test.log", "ERROR", 0, 0, 1, workdir)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "success" {
		t.Fatalf("status = %q, want success", parsed.Status)
	}
}

func TestExecuteFileReaderAdvancedInvalidOperation(t *testing.T) {
	workdir := t.TempDir()

	result := ExecuteFileReaderAdvanced("invalid_op", "test.log", "", 0, 0, 0, workdir)
	var parsed FileReaderResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Status != "error" {
		t.Fatalf("status = %q, want error", parsed.Status)
	}
}

func TestClampFileReaderContent(t *testing.T) {
	// Test within limit
	short := "hello world"
	got, truncated := clampFileReaderContent(short)
	if got != short {
		t.Errorf("short content modified: got %q", got)
	}
	if truncated {
		t.Errorf("short content should not be truncated")
	}

	// Test truncation
	long := strings.Repeat("x", 50000)
	got, truncated = clampFileReaderContent(long)
	if !truncated {
		t.Errorf("long content should be truncated")
	}
	if len(got) > fileReaderAdvancedMaxChars+100 {
		t.Errorf("clamped content too long: %d chars", len(got))
	}
}
