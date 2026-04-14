// Package outputcompress – Filesystem tool output compressors.
//
// The filesystem tool (ExecuteFilesystem) returns JSON responses for various
// operations: list_dir, stat, read_file, write_file, copy, move, delete,
// create_dir, and batch variants.
//
// Compression strategy:
//   - list_dir: group entries by type (dirs vs files), compact listing
//   - stat: already compact, pass through
//   - read_file: PRESERVE content, only compact the wrapper metadata
//   - write_file/copy/move/delete/create_dir: already compact, pass through
//   - batch operations: compact per-item results to summary
//   - errors: always pass through unchanged
package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compressFilesystemOutput routes filesystem tool output to the appropriate sub-compressor.
func compressFilesystemOutput(output string) (string, string) {
	clean := strings.TrimSpace(output)

	// Must be JSON
	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "fs-nonjson"
	}

	// Parse top-level
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "fs-parse-err"
	}

	// Error responses: return as-is
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "fs-error"
	}

	// Detect operation type by the data structure
	data := raw["data"]

	// list_dir: data has "entries" array
	if data != nil {
		var dataObj map[string]json.RawMessage
		if err := json.Unmarshal(data, &dataObj); err == nil {
			if dataObj["entries"] != nil {
				return compressFSListDir(raw, dataObj), "fs-list-dir"
			}
			// stat: data has "name", "is_dir", "size", "modified" but no "entries"
			if dataObj["name"] != nil && dataObj["is_dir"] != nil && dataObj["entries"] == nil {
				// Already compact, pass through
				return clean, "fs-stat"
			}
			// read_file: data is a string (file content), not an object
			// This case is handled below
		}

		// read_file: data is a raw string (file content)
		var dataStr string
		if err := json.Unmarshal(data, &dataStr); err == nil {
			// It's a string = file content. Preserve content, compact wrapper.
			return compressFSReadFile(raw, dataStr), "fs-read-file"
		}
	}

	// Batch: data has "summary" and "results"
	if data != nil {
		var dataObj map[string]json.RawMessage
		if err := json.Unmarshal(data, &dataObj); err == nil {
			if dataObj["summary"] != nil && dataObj["results"] != nil {
				return compressFSBatch(raw, dataObj), "fs-batch"
			}
		}
	}

	// Simple success messages (write, copy, move, delete, create_dir)
	// Already compact – pass through
	return clean, "fs-simple"
}

// compressFSListDir compacts large directory listings.
// From: {"status":"success","message":"Listed 500 entries (of 1500 total)","data":{"entries":[...],"total_count":1500,...}}
// To:   "1500 entries (500 shown, 50 dirs + 450 files):\n  dir1/\n  dir2/\n  file1.txt (1.2KB)\n  ..."
func compressFSListDir(raw, dataObj map[string]json.RawMessage) string {
	totalCount := jsonInt(dataObj["total_count"])

	type entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"modified"`
	}

	var entries []entry
	if err := json.Unmarshal(dataObj["entries"], &entries); err != nil {
		return rawToString(raw)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d entries", totalCount)

	dirCount := 0
	fileCount := 0
	for _, e := range entries {
		if e.IsDir {
			dirCount++
		} else {
			fileCount++
		}
	}
	fmt.Fprintf(&sb, " (%d dirs, %d files):\n", dirCount, fileCount)

	limit := 50
	if len(entries) < limit {
		limit = len(entries)
	}

	// Show directories first, then files
	for i := 0; i < limit; i++ {
		e := entries[i]
		if e.IsDir {
			sb.WriteString("  " + e.Name + "/\n")
		}
	}
	for i := 0; i < limit; i++ {
		e := entries[i]
		if !e.IsDir {
			sb.WriteString("  " + e.Name)
			if e.Size > 0 {
				sb.WriteString(" (" + formatFileSize(e.Size) + ")")
			}
			sb.WriteString("\n")
		}
	}

	if len(entries) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(entries)-limit)
	}

	truncated := jsonBool(dataObj["truncated"])
	if truncated {
		nextOffset := jsonInt(dataObj["next_offset"])
		fmt.Fprintf(&sb, "  [paginated, next_offset=%d]\n", nextOffset)
	}

	return sb.String()
}

// compressFSReadFile preserves file content but compacts the wrapper.
// From: {"status":"success","message":"Read 12345 bytes","data":"<file content>"}
// To:   "Read 12345 bytes:\n<file content>"
func compressFSReadFile(raw map[string]json.RawMessage, content string) string {
	message := jsonString(raw["message"])

	var sb strings.Builder
	if message != "" {
		sb.WriteString(message + ":\n")
	}
	sb.WriteString(content)
	return sb.String()
}

// compressFSBatch compacts batch operation results.
// From: {"status":"partial","message":"copy_batch processed 10 items (8 succeeded, 2 failed)","data":{"summary":{...},"results":[...]}}
// To:   "copy_batch: 8/10 succeeded, 2 failed:\n  [0] path → dest OK\n  [5] path ERROR: msg\n  ..."
func compressFSBatch(raw, dataObj map[string]json.RawMessage) string {
	message := jsonString(raw["message"])

	type summary struct {
		Requested int `json:"requested"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
	}

	var sum summary
	json.Unmarshal(dataObj["summary"], &sum)

	type batchItem struct {
		Index     int    `json:"index"`
		FilePath  string `json:"file_path"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		ErrorCode string `json:"error_code"`
	}

	var results []batchItem
	if err := json.Unmarshal(dataObj["results"], &results); err != nil {
		return message
	}

	var sb strings.Builder
	if message != "" {
		sb.WriteString(message + "\n")
	} else {
		fmt.Fprintf(&sb, "%d/%d succeeded", sum.Succeeded, sum.Requested)
		if sum.Failed > 0 {
			fmt.Fprintf(&sb, ", %d failed", sum.Failed)
		}
		sb.WriteString("\n")
	}

	// Show only failed items in detail, summarize succeeded
	for _, r := range results {
		if r.Status != "success" {
			fmt.Fprintf(&sb, "  [%d] %s FAILED: %s\n", r.Index, r.FilePath, r.Message)
		}
	}

	succeededShown := sum.Succeeded
	if succeededShown > 0 {
		fmt.Fprintf(&sb, "  [%d succeeded items omitted]\n", succeededShown)
	}

	return sb.String()
}

// formatFileSize wird jetzt aus utils.go importiert (zentraler JSON-Helper)
