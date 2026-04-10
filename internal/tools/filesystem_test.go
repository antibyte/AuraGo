package tools

import (
	"encoding/json"
	"fmt"
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

	raw := ExecuteFilesystem("read_file", "image.png", "", "", nil, workdir, 0, 0)
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

	raw := ExecuteFilesystem("read_file", "notes.txt", "", "", nil, workdir, 0, 0)
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

	raw := ExecuteFilesystem("read_file", "large.log", "", "", nil, workdir, 0, 0)
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

	raw := ExecuteFilesystem("read_file", "../../../etc/passwd", "", "", nil, workdir, 0, 0)
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

	raw := ExecuteFilesystem("read_file", "missing.txt", "", "", nil, workdir, 0, 0)
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

	raw := ExecuteFilesystem("copy", "source.txt", "copies/destination.txt", "", nil, workdir, 0, 0)
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
	raw := ExecuteFilesystem("copy_batch", "", "", "", items, workdir, 0, 0)
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
	raw := ExecuteFilesystem("delete_batch", "", "", "", items, workdir, 0, 0)
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
	raw := ExecuteFilesystem("create_dir_batch", "", "", "", items, workdir, 0, 0)
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

func TestSecureResolvePlainWorkspaceRejectsParentEscape(t *testing.T) {
	workdir := t.TempDir()
	_, err := secureResolve(workdir, filepath.Join("..", "outside.txt"))
	if err == nil {
		t.Fatal("expected escape error")
	}
}

func TestFilesystemRootsDetectAgentWorkspaceAncestor(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "nested", "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}

	workspaceRoot, projectRoot := filesystemRoots(workdir)
	wantProjectRoot := filepath.Join(root, "nested")
	if workspaceRoot != workdir {
		t.Fatalf("workspaceRoot = %q, want %q", workspaceRoot, workdir)
	}
	if projectRoot != wantProjectRoot {
		t.Fatalf("projectRoot = %q, want %q", projectRoot, wantProjectRoot)
	}
}

// listDirPaginationResult is the response structure for paginated list_dir
type listDirPaginationResult struct {
	Entries    []FileInfoEntry `json:"entries"`
	TotalCount int             `json:"total_count"`
	Truncated  bool            `json:"truncated"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	NextOffset *int            `json:"next_offset,omitempty"`
}

func TestExecuteFilesystemListDirSmallDirNoTruncation(t *testing.T) {
	workdir := t.TempDir()
	// Create a few files
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 0, 0)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Data)
	}

	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries field missing or wrong type")
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if data["truncated"] != false {
		t.Fatalf("truncated = %v, want false", data["truncated"])
	}
	if tc, ok := data["total_count"].(float64); !ok || int(tc) != 3 {
		t.Fatalf("total_count = %v, want 3", data["total_count"])
	}
}

func TestExecuteFilesystemListDirPaginationTruncation(t *testing.T) {
	workdir := t.TempDir()
	// Create 10 files
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request only 3 entries
	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 3, 0)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Data)
	}

	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries field missing or wrong type")
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if data["truncated"] != true {
		t.Fatalf("truncated = %v, want true", data["truncated"])
	}
	if tc, ok := data["total_count"].(float64); !ok || int(tc) != 10 {
		t.Fatalf("total_count = %v, want 10", data["total_count"])
	}
	if lim, ok := data["limit"].(float64); !ok || int(lim) != 3 {
		t.Fatalf("limit = %v, want 3", data["limit"])
	}
	if off, ok := data["offset"].(float64); !ok || int(off) != 0 {
		t.Fatalf("offset = %v, want 0", data["offset"])
	}
	nextOffset, ok := data["next_offset"]
	if !ok {
		t.Fatalf("next_offset missing when truncated=true")
	}
	if no, ok := nextOffset.(float64); !ok || int(no) != 3 {
		t.Fatalf("next_offset = %v, want 3", nextOffset)
	}
}

func TestExecuteFilesystemListDirPaginationOffset(t *testing.T) {
	workdir := t.TempDir()
	// Create 10 files
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request 3 entries starting at offset 5
	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 3, 5)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Data)
	}

	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries field missing or wrong type")
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	if data["truncated"] != true {
		t.Fatalf("truncated = %v, want true", data["truncated"])
	}
	if tc, ok := data["total_count"].(float64); !ok || int(tc) != 10 {
		t.Fatalf("total_count = %v, want 10", data["total_count"])
	}
	if off, ok := data["offset"].(float64); !ok || int(off) != 5 {
		t.Fatalf("offset = %v, want 5", data["offset"])
	}
	nextOffset, ok := data["next_offset"]
	if !ok {
		t.Fatalf("next_offset missing when more entries available")
	}
	if no, ok := nextOffset.(float64); !ok || int(no) != 8 {
		t.Fatalf("next_offset = %v, want 8", nextOffset)
	}
}

func TestExecuteFilesystemListDirPaginationLastPage(t *testing.T) {
	workdir := t.TempDir()
	// Create 10 files
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request 5 entries starting at offset 8 (last page with 2 entries)
	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 5, 8)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map response, got %T", result.Data)
	}

	entries, ok := data["entries"].([]interface{})
	if !ok {
		t.Fatalf("entries field missing or wrong type")
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (last page)", len(entries))
	}

	// Last page should not be truncated if we got all remaining
	if data["truncated"] != false {
		t.Fatalf("truncated = %v, want false on last page", data["truncated"])
	}
	if data["next_offset"] != nil {
		t.Fatalf("next_offset should be nil on last page, got %v", data["next_offset"])
	}
}

func TestExecuteFilesystemListDirSortingDeterministic(t *testing.T) {
	workdir := t.TempDir()
	// Create files with names that would sort differently case-sensitively
	files := []string{"apple", "Banana", "cherry", "DATE", "elderberry"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request twice and verify order is the same
	raw1 := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 10, 0)
	raw2 := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 10, 0)

	var result1, result2 FSResult
	if err := json.Unmarshal([]byte(raw1), &result1); err != nil {
		t.Fatalf("unmarshal result1: %v", err)
	}
	if err := json.Unmarshal([]byte(raw2), &result2); err != nil {
		t.Fatalf("unmarshal result2: %v", err)
	}

	data1 := result1.Data.(map[string]interface{})
	data2 := result2.Data.(map[string]interface{})

	entries1 := data1["entries"].([]interface{})
	entries2 := data2["entries"].([]interface{})

	if len(entries1) != len(entries2) {
		t.Fatalf("different number of entries: %d vs %d", len(entries1), len(entries2))
	}

	for i := 0; i < len(entries1); i++ {
		e1 := entries1[i].(map[string]interface{})
		e2 := entries2[i].(map[string]interface{})
		if e1["name"] != e2["name"] {
			t.Fatalf("entry %d differs: %q vs %q", i, e1["name"], e2["name"])
		}
	}
}

func TestExecuteFilesystemListDirDefaultLimit(t *testing.T) {
	workdir := t.TempDir()
	// Create 600 files
	for i := 0; i < 600; i++ {
		name := fmt.Sprintf("file%03d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request without explicit limit (0) - should use default 500
	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 0, 0)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	data := result.Data.(map[string]interface{})

	// Should be truncated since we have 600 entries but default limit is 500
	if data["truncated"] != true {
		t.Fatalf("truncated = %v, want true for 600 entries with default limit", data["truncated"])
	}
	if tc, ok := data["total_count"].(float64); !ok || int(tc) != 600 {
		t.Fatalf("total_count = %v, want 600", data["total_count"])
	}
	if lim, ok := data["limit"].(float64); !ok || int(lim) != 500 {
		t.Fatalf("limit = %v, want 500 (default)", data["limit"])
	}
}

func TestExecuteFilesystemListDirMaxLimit(t *testing.T) {
	workdir := t.TempDir()
	// Create 1500 files
	for i := 0; i < 1500; i++ {
		name := fmt.Sprintf("file%04d.txt", i)
		if err := os.WriteFile(filepath.Join(workdir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	// Request with limit > maxLimit (1000)
	raw := ExecuteFilesystem("list_dir", "", "", "", nil, workdir, 1500, 0)
	var result FSResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	data := result.Data.(map[string]interface{})

	// Should be truncated since we have 1500 entries but max limit is 1000
	if data["truncated"] != true {
		t.Fatalf("truncated = %v, want true for 1500 entries", data["truncated"])
	}
	if tc, ok := data["total_count"].(float64); !ok || int(tc) != 1500 {
		t.Fatalf("total_count = %v, want 1500", data["total_count"])
	}
	if lim, ok := data["limit"].(float64); !ok || int(lim) != 1000 {
		t.Fatalf("limit = %v, want 1000 (max enforced)", data["limit"])
	}
}
