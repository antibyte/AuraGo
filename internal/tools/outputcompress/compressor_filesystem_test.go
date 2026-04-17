package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ─── V9A Tests ───────────────────────────────────────────────────────────────

func TestCompressShell_V9A_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"tar -czf archive.tar.gz .", "tar"},
		{"tar -tf archive.tar", "tar"},
		{"tar -xvf archive.tar", "tar"},
		{"zip -r archive.zip dir/", "zip"},
		{"unzip -l archive.zip", "unzip"},
		{"unzip archive.zip", "unzip"},
		{"rsync -avz src/ dest/", "rsync"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("file\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
func TestCompressTar_LargeListing(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("src/\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "src/module%d/main.go\n", i)
		fmt.Fprintf(&sb, "src/module%d/util.go\n", i)
	}
	sb.WriteString("src/README.md\n")

	input := sb.String()
	result := compressTar(input)

	// Should group by directory
	if !strings.Contains(result, "src/") {
		t.Error("expected directory grouping for src/")
	}
	// Output should be significantly shorter than input
	if len(result) >= len(input) {
		t.Errorf("expected compression: input=%d, output=%d", len(input), len(result))
	}
}

func TestCompressTar_ShortListing(t *testing.T) {
	output := "file1.txt\nfile2.txt\nfile3.txt\n"
	result := compressTar(output)

	// Short output should be returned as-is
	if result != output {
		t.Errorf("expected short output preserved, got: %s", result)
	}
}

func TestCompressTar_Errors(t *testing.T) {
	output := "tar: Error opening archive: No such file\nfile1.txt\nfile2.txt\n"
	result := compressTar(output)

	if !strings.Contains(result, "Error opening") {
		t.Error("expected error preserved")
	}
}

func TestCompressZip_LargeOutput(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("  adding: src/main.go (deflated 45%)\n")
	sb.WriteString("  adding: src/util.go (deflated 30%)\n")
	sb.WriteString("  adding: src/config.go (stored 0%)\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "  adding: src/mod%d/app.go (deflated 25%%)\n", i)
	}
	sb.WriteString("  adding: README.md (stored 0%)\n")

	input := sb.String()
	result := compressZip(input)

	// Should extract file paths without compression ratios
	if strings.Contains(result, "deflated") {
		t.Error("expected compression ratios removed")
	}
	// Should contain file paths (groupByDir format: "src/ (N): main.go, ...")
	if !strings.Contains(result, "main.go") {
		t.Error("expected main.go preserved")
	}
	if !strings.Contains(result, "src/") {
		t.Error("expected src/ directory grouping")
	}
	// Output should be shorter than input
	if len(result) >= len(input) {
		t.Errorf("expected compression: input=%d, output=%d", len(input), len(result))
	}
}

func TestCompressZip_UnzipListing(t *testing.T) {
	output := "Archive:  archive.zip\n  Length      Date    Time    Name\n---------  ---------- -----   ----\n    12345  2024-01-15 10:30   src/main.go\n     5678  2024-01-15 10:30   src/util.go\n     9012  2024-01-15 10:30   README.md\n---------                     -------\n    27035                     3 files\n"

	result := compressZip(output)

	if !strings.Contains(result, "src/main.go") {
		t.Error("expected file paths preserved")
	}
	if !strings.Contains(result, "3 files") {
		t.Error("expected summary preserved")
	}
}

func TestCompressRsync_LargeOutput(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("sending incremental file list\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, ">f++++++++x src/module%d/main.go\n", i)
	}
	sb.WriteString("sent 12,345 bytes  received 678 bytes  25,346.00 bytes/sec\n")
	sb.WriteString("total size is 1,234,567  speedup is 98.76\n")

	result := compressRsync(sb.String())

	// Should keep summary
	if !strings.Contains(result, "sending incremental file list") {
		t.Error("expected 'sending' summary preserved")
	}
	if !strings.Contains(result, "total size") {
		t.Error("expected total size summary preserved")
	}
	// Should group files by directory
	if !strings.Contains(result, "src/") {
		t.Error("expected src/ directory grouping")
	}
}

func TestCompressRsync_Errors(t *testing.T) {
	output := "rsync: link_stat \"/missing\" failed: No such file (2)\nrsync error: some errors could not be transferred\n"
	result := compressRsync(output)

	if !strings.Contains(result, "rsync:") {
		t.Error("expected rsync error preserved")
	}
}
// ─── V10A Tests: Filesystem / File Reader / Smart File ──────────────────────

func TestCompress_APIRouting_FileTools(t *testing.T) {
	// Verify file tool families are recognized
	if !isAPITool("filesystem") {
		t.Error("expected 'filesystem' to be an API tool")
	}
	if !isAPITool("filesystem_op") {
		t.Error("expected 'filesystem_op' to be an API tool")
	}
	if !isAPITool("file_reader_advanced") {
		t.Error("expected 'file_reader_advanced' to be an API tool")
	}
	if !isAPITool("smart_file_read") {
		t.Error("expected 'smart_file_read' to be an API tool")
	}
	if !isFilesystemTool("filesystem") {
		t.Error("expected isFilesystemTool('filesystem') = true")
	}
	if !isFileReaderTool("file_reader_advanced") {
		t.Error("expected isFileReaderTool('file_reader_advanced') = true")
	}
	if !isSmartFileTool("smart_file_read") {
		t.Error("expected isSmartFileTool('smart_file_read') = true")
	}
}

func TestCompressFS_ListDir_Large(t *testing.T) {
	type entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"modified"`
	}

	entries := make([]entry, 200)
	for i := 0; i < 200; i++ {
		isDir := i < 20
		name := fmt.Sprintf("file_%04d.txt", i)
		if isDir {
			name = fmt.Sprintf("dir_%04d", i)
		}
		entries[i] = entry{
			Name:    name,
			IsDir:   isDir,
			Size:    int64(i * 1024),
			ModTime: "2024-01-15T10:30:00Z",
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Listed 200 entries (of 200 total)",
		"data": map[string]interface{}{
			"entries":     entries,
			"total_count": 200,
			"truncated":   false,
			"limit":       500,
			"offset":      0,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-list-dir" {
		t.Errorf("expected filter fs-list-dir, got %s", filter)
	}
	if !strings.Contains(result, "200 entries") {
		t.Error("expected entry count")
	}
	if !strings.Contains(result, "20 dirs") {
		t.Error("expected dir count")
	}
	if !strings.Contains(result, "180 files") {
		t.Error("expected file count")
	}
	if !strings.Contains(result, "+ 150 more") {
		t.Error("expected truncation")
	}
}

func TestCompressFS_ListDir_Paginated(t *testing.T) {
	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}

	entries := make([]entry, 10)
	for i := 0; i < 10; i++ {
		entries[i] = entry{Name: fmt.Sprintf("file_%d.txt", i), IsDir: false, Size: 1024}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Listed 10 entries (of 1500 total) — use next_offset for more",
		"data": map[string]interface{}{
			"entries":     entries,
			"total_count": 1500,
			"truncated":   true,
			"limit":       10,
			"offset":      0,
			"next_offset": 10,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-list-dir" {
		t.Errorf("expected filter fs-list-dir, got %s", filter)
	}
	if !strings.Contains(result, "1500 entries") {
		t.Error("expected total count")
	}
	if !strings.Contains(result, "paginated") {
		t.Error("expected pagination indicator")
	}
}

func TestCompressFS_ReadFile_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Read 52 bytes",
		"data":    content,
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-read-file" {
		t.Errorf("expected filter fs-read-file, got %s", filter)
	}
	// Content must be preserved exactly
	if !strings.Contains(result, content) {
		t.Error("expected file content preserved")
	}
	if !strings.Contains(result, "Read 52 bytes") {
		t.Error("expected message preserved")
	}
}

func TestCompressFS_Batch_Partial(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		status := "success"
		msg := "OK"
		if i == 3 || i == 7 {
			status = "error"
			msg = "Permission denied"
		}
		items[i] = map[string]interface{}{
			"index":     i,
			"file_path": fmt.Sprintf("/tmp/file_%d.txt", i),
			"status":    status,
			"message":   msg,
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status":  "partial",
		"message": "copy_batch processed 10 items (8 succeeded, 2 failed)",
		"data": map[string]interface{}{
			"summary": map[string]int{
				"requested": 10,
				"succeeded": 8,
				"failed":    2,
			},
			"results": items,
		},
	})

	result, filter := compressAPIOutput("filesystem", string(data))

	if filter != "fs-batch" {
		t.Errorf("expected filter fs-batch, got %s", filter)
	}
	if !strings.Contains(result, "8 succeeded") {
		t.Error("expected succeeded count")
	}
	if !strings.Contains(result, "2 failed") {
		t.Error("expected failed count")
	}
	// Failed items should be shown
	if !strings.Contains(result, "FAILED") {
		t.Error("expected failed items shown")
	}
	// Succeeded items should be summarized
	if !strings.Contains(result, "succeeded items omitted") {
		t.Error("expected succeeded summary")
	}
}

func TestCompressFS_Error(t *testing.T) {
	output := `{"status":"error","message":"Failed to list directory: permission denied","data":{"error_code":"io_error"}}`

	result, filter := compressAPIOutput("filesystem", output)

	if filter != "fs-error" {
		t.Errorf("expected filter fs-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}

func TestCompressFR_Content_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello world\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"start_line": 1,
			"end_line":   5,
			"total_read": 5,
			"content":    content,
			"truncated":  false,
		},
	})

	result, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-content" {
		t.Errorf("expected filter fr-content, got %s", filter)
	}
	// Content must be preserved
	if !strings.Contains(result, content) {
		t.Error("expected file content preserved")
	}
	if !strings.Contains(result, "Lines 1-5") {
		t.Error("expected line range")
	}
}

func TestCompressFR_SearchContext_ManyMatches(t *testing.T) {
	type match struct {
		MatchLine int    `json:"match_line"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}

	matches := make([]match, 30)
	for i := 0; i < 30; i++ {
		matches[i] = match{
			MatchLine: 10 + i*5,
			StartLine: 8 + i*5,
			EndLine:   12 + i*5,
			Content:   fmt.Sprintf("Line with error: something went wrong at step %d", i),
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"pattern":       "error",
			"total_matches": 30,
			"matches":       matches,
		},
	})

	result, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-search" {
		t.Errorf("expected filter fr-search, got %s", filter)
	}
	if !strings.Contains(result, "30 matches") {
		t.Error("expected match count")
	}
	if !strings.Contains(result, "+ 15 more matches") {
		t.Error("expected truncation")
	}
	if !strings.Contains(result, "L8-12:") {
		t.Error("expected line range format")
	}
}

func TestCompressFR_CountLines(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"lines": 12345,
			"bytes": 67890,
		},
	})

	_, filter := compressAPIOutput("file_reader_advanced", string(data))

	if filter != "fr-count-lines" {
		t.Errorf("expected filter fr-count-lines, got %s", filter)
	}
	// count_lines is already compact, should pass through
}

func TestCompressFR_Error(t *testing.T) {
	output := `{"status":"error","message":"'file_path' is required"}`

	result, filter := compressAPIOutput("file_reader_advanced", output)

	if filter != "fr-error" {
		t.Errorf("expected filter fr-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}
func TestCompressSF_Analyze(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Analyzed main.go",
		"data": map[string]interface{}{
			"path":              "main.go",
			"size_bytes":        15360,
			"line_count":        450,
			"mime":              "text/plain",
			"extension":         ".go",
			"group":             "text",
			"is_text_like":      true,
			"is_large":          false,
			"recommended_tool":  "file_reader_advanced",
			"default_strategy":  "head_tail",
			"next_steps":        []string{"Use file_reader_advanced read_lines"},
			"detected_encoding": "utf-8",
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-analyze" {
		t.Errorf("expected filter sf-analyze, got %s", filter)
	}
	if !strings.Contains(result, "main.go") {
		t.Error("expected file name")
	}
	if !strings.Contains(result, "450 lines") {
		t.Error("expected line count")
	}
	if !strings.Contains(result, "text/plain") {
		t.Error("expected mime type")
	}
	if !strings.Contains(result, "Recommended: file_reader_advanced") {
		t.Error("expected recommendation")
	}
}

func TestCompressSF_Structure(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Detected structure for config.json",
		"data": map[string]interface{}{
			"path":           "config.json",
			"size_bytes":     8192,
			"mime":           "application/json",
			"extension":      ".json",
			"is_text_like":   true,
			"format":         "json",
			"root_type":      "object",
			"top_level_keys": []string{"server", "database", "logging", "auth", "cache", "features", "version"},
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-structure" {
		t.Errorf("expected filter sf-structure, got %s", filter)
	}
	if !strings.Contains(result, "config.json") {
		t.Error("expected file name")
	}
	if !strings.Contains(result, "JSON") && !strings.Contains(result, "json") {
		t.Error("expected format")
	}
	if !strings.Contains(result, "Keys:") {
		t.Error("expected keys listing")
	}
	if !strings.Contains(result, "server") {
		t.Error("expected key names")
	}
}

func TestCompressSF_Sample_PreservesContent(t *testing.T) {
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"message": "Built head_tail sample from main.go",
		"data": map[string]interface{}{
			"path":              "main.go",
			"size_bytes":        15360,
			"sampling_strategy": "head_tail",
			"sample_sections":   2,
			"content":           content,
			"next_steps":        []string{"Use file_reader_advanced"},
		},
	})

	result, filter := compressAPIOutput("smart_file_read", string(data))

	if filter != "sf-content" {
		t.Errorf("expected filter sf-content, got %s", filter)
	}
	// Content must be preserved
	if !strings.Contains(result, content) {
		t.Error("expected content preserved")
	}
	if !strings.Contains(result, "head_tail") {
		t.Error("expected strategy")
	}
}

func TestCompressSF_Error(t *testing.T) {
	output := `{"status":"error","message":"'file_path' is required"}`

	result, filter := compressAPIOutput("smart_file_read", output)

	if filter != "sf-error" {
		t.Errorf("expected filter sf-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Error("expected error preserved")
	}
}
// ─── V10B: Process tool tests ──────────────────────────────────────────────

func TestCompress_APIRouting_ProcessTools(t *testing.T) {
	// Verify process tools are routed through API compressor
	_, filter := compressAPIOutput("list_processes", "Tool Output: No active background processes.")
	if filter != "proc-list" {
		t.Errorf("expected proc-list, got %s", filter)
	}
	_, filter = compressAPIOutput("read_process_logs", "Tool Output: [LOGS for PID 123]\nhello")
	if filter != "proc-logs" {
		t.Errorf("expected proc-logs, got %s", filter)
	}
}

func TestCompressListProcesses_Empty(t *testing.T) {
	input := "Tool Output: No active background processes."
	result, filter := compressAPIOutput("list_processes", input)
	if filter != "proc-list" {
		t.Errorf("expected filter proc-list, got %s", filter)
	}
	if !strings.Contains(result, "No active processes") {
		t.Errorf("expected empty message, got %q", result)
	}
}

func TestCompressListProcesses_ManyProcesses(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Tool Output: Active processes:\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&sb, "- PID: %d, Started: 2024-01-15T10:%02d:00Z\n", 10000+i, i)
	}

	result, filter := compressAPIOutput("list_processes", sb.String())
	if filter != "proc-list" {
		t.Errorf("expected filter proc-list, got %s", filter)
	}
	if !strings.Contains(result, "20 processes:") {
		t.Errorf("expected count summary, got %q", result)
	}
	if !strings.Contains(result, "10001") || !strings.Contains(result, "10020") {
		t.Errorf("expected PID numbers in output, got %q", result)
	}
	// Should NOT contain "Started:" timestamps
	if strings.Contains(result, "Started:") {
		t.Error("should not contain timestamps after compression")
	}
}

func TestCompressReadProcessLogs_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Tool Output: [LOGS for PID 12345]\n")
	for i := 1; i <= 500; i++ {
		fmt.Fprintf(&sb, "Log line %d: some output here\n", i)
	}

	result, filter := compressAPIOutput("read_process_logs", sb.String())
	if filter != "proc-logs" {
		t.Errorf("expected filter proc-logs, got %s", filter)
	}
	if !strings.Contains(result, "[PID 12345]") {
		t.Errorf("expected PID info, got %q", result)
	}
	// Should be significantly shorter than input
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression, got %d >= %d", len(result), sb.Len())
	}
}

func TestCompressReadProcessLogs_Duplicates(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Tool Output: [LOGS for PID 99]\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("repeated line\n")
	}
	sb.WriteString("unique final line\n")

	result, filter := compressAPIOutput("read_process_logs", sb.String())
	if filter != "proc-logs" {
		t.Errorf("expected filter proc-logs, got %s", filter)
	}
	if !strings.Contains(result, "duplicates removed") {
		t.Errorf("expected dedup info, got %q", result)
	}
	if !strings.Contains(result, "unique final line") {
		t.Error("expected unique final line preserved")
	}
}

func TestCompressReadProcessLogs_Empty(t *testing.T) {
	input := "Tool Output: [LOGS for PID 42]\n"
	result, _ := compressAPIOutput("read_process_logs", input)
	if !strings.Contains(result, "[PID 42]") {
		t.Errorf("expected PID info, got %q", result)
	}
	if !strings.Contains(result, "empty") {
		t.Errorf("expected empty indicator, got %q", result)
	}
}
// ─── V10C: Agent status tool tests ─────────────────────────────────────────

func TestCompress_APIRouting_AgentStatusTools(t *testing.T) {
	_, filter := compressAPIOutput("manage_daemon", `Tool Output: {"status":"success","count":0,"daemons":[]}`)
	if filter != "agent-daemon" {
		t.Errorf("expected agent-daemon, got %s", filter)
	}
	_, filter = compressAPIOutput("manage_plan", `Tool Output: {"status":"success","count":0,"plans":[]}`)
	if filter != "agent-plan" {
		t.Errorf("expected agent-plan, got %s", filter)
	}
}

func TestCompressDaemon_List(t *testing.T) {
	daemons := []map[string]interface{}{
		{"skill_id": "health-check", "status": "running", "uptime": "2h", "restart_count": 0},
		{"skill_id": "backup-job", "status": "idle", "interval": "6h"},
		{"skill_id": "monitor", "status": "running", "uptime": "30m", "restart_count": 3, "next_run": "2024-01-15T12:00:00Z"},
	}
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"count":   len(daemons),
		"daemons": daemons,
	})
	input := "Tool Output: " + string(data)

	result, filter := compressAPIOutput("manage_daemon", input)
	if filter != "agent-daemon" {
		t.Errorf("expected filter agent-daemon, got %s", filter)
	}
	if !strings.Contains(result, "3 daemons:") {
		t.Errorf("expected count, got %q", result)
	}
	if !strings.Contains(result, "health-check") || !strings.Contains(result, "backup-job") {
		t.Errorf("expected daemon names, got %q", result)
	}
	if !strings.Contains(result, "running") {
		t.Errorf("expected status, got %q", result)
	}
}

func TestCompressDaemon_Status(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"daemon": map[string]interface{}{
			"skill_id":      "health-check",
			"status":        "running",
			"uptime":        "2h",
			"restart_count": 0,
		},
	})
	input := "Tool Output: " + string(data)

	result, filter := compressAPIOutput("manage_daemon", input)
	if filter != "agent-daemon" {
		t.Errorf("expected filter agent-daemon, got %s", filter)
	}
	if !strings.Contains(result, "health-check") {
		t.Errorf("expected skill name, got %q", result)
	}
	if !strings.Contains(result, "running") {
		t.Errorf("expected status, got %q", result)
	}
}

func TestCompressDaemon_Error(t *testing.T) {
	input := `Tool Output: {"status":"error","message":"'skill_id' is required for status"}`
	result, filter := compressAPIOutput("manage_daemon", input)
	if filter != "agent-status-error" {
		t.Errorf("expected filter agent-status-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("expected error preserved, got %q", result)
	}
}

func TestCompressPlan_List(t *testing.T) {
	plans := []map[string]interface{}{
		{"id": "p1", "title": "Deploy app", "status": "active", "priority": 2.0,
			"tasks": []interface{}{
				map[string]interface{}{"title": "Build", "status": "done"},
				map[string]interface{}{"title": "Test", "status": "pending"},
			}},
		{"id": "p2", "title": "Clean up", "status": "completed", "priority": 1.0,
			"tasks": []interface{}{
				map[string]interface{}{"title": "Remove old files", "status": "done"},
			}},
	}
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"count":  len(plans),
		"plans":  plans,
	})
	input := "Tool Output: " + string(data)

	result, filter := compressAPIOutput("manage_plan", input)
	if filter != "agent-plan" {
		t.Errorf("expected filter agent-plan, got %s", filter)
	}
	if !strings.Contains(result, "2 plans:") {
		t.Errorf("expected count, got %q", result)
	}
	if !strings.Contains(result, "Deploy app") || !strings.Contains(result, "Clean up") {
		t.Errorf("expected plan titles, got %q", result)
	}
	if !strings.Contains(result, "[active]") || !strings.Contains(result, "[completed]") {
		t.Errorf("expected status brackets, got %q", result)
	}
}

func TestCompressPlan_Get(t *testing.T) {
	plan := map[string]interface{}{
		"id":         "p1",
		"title":      "Deploy app",
		"status":     "active",
		"priority":   2.0,
		"created_at": "2024-01-15T10:00:00Z",
		"tasks": []interface{}{
			map[string]interface{}{"title": "Build", "status": "done"},
			map[string]interface{}{"title": "Test", "status": "pending"},
			map[string]interface{}{"title": "Deploy", "status": "pending"},
		},
	}
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"plan":   plan,
	})
	input := "Tool Output: " + string(data)

	result, filter := compressAPIOutput("manage_plan", input)
	if filter != "agent-plan" {
		t.Errorf("expected filter agent-plan, got %s", filter)
	}
	if !strings.Contains(result, "[active] Deploy app") {
		t.Errorf("expected title with status, got %q", result)
	}
	if !strings.Contains(result, "[done] Build") {
		t.Errorf("expected task with status, got %q", result)
	}
	if !strings.Contains(result, "[pending] Test") {
		t.Errorf("expected task with status, got %q", result)
	}
}

func TestCompressPlan_Error(t *testing.T) {
	input := `Tool Output: {"status":"error","message":"'title' is required for create"}`
	result, filter := compressAPIOutput("manage_plan", input)
	if filter != "agent-status-error" {
		t.Errorf("expected filter agent-status-error, got %s", filter)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("expected error preserved, got %q", result)
	}
}

func TestCompressDaemon_EmptyList(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"count":   0,
		"daemons": []interface{}{},
	})
	input := "Tool Output: " + string(data)

	result, _ := compressAPIOutput("manage_daemon", input)
	if !strings.Contains(result, "No daemons") {
		t.Errorf("expected empty message, got %q", result)
	}
}

func TestCompressPlan_EmptyList(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"count":  0,
		"plans":  []interface{}{},
	})
	input := "Tool Output: " + string(data)

	result, _ := compressAPIOutput("manage_plan", input)
	if !strings.Contains(result, "No plans") {
		t.Errorf("expected empty message, got %q", result)
	}
}
