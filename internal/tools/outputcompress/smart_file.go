// Package outputcompress – smart_file_read output compressors.
//
// smart_file_read returns JSON with operation-specific data:
//   - analyze: file metadata (path, size, mime, line_count, is_text_like, ...)
//   - structure: format detection (json/xml/csv/text with keys/headers)
//   - sample: sampled content sections with metadata
//   - summarize: LLM-generated summary with metadata
//
// Strategy:
//   - analyze: compact to key metadata only
//   - structure: compact to format + key info
//   - sample: preserve content, compact wrapper
//   - summarize: preserve content, compact wrapper
package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compressSmartFileOutput routes smart_file_read output to sub-compressors.
func compressSmartFileOutput(output string) (string, string) {
	clean := strings.TrimSpace(output)

	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "sf-nonjson"
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "sf-parse-err"
	}

	// Error responses: return as-is
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "sf-error"
	}

	data := raw["data"]
	if data == nil {
		return clean, "sf-no-data"
	}

	var dataObj map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataObj); err != nil {
		return clean, "sf-simple"
	}

	// Detect operation by data fields
	switch {
	case dataObj["is_text_like"] != nil && dataObj["recommended_tool"] != nil:
		// analyze
		return compressSFAnalyze(dataObj), "sf-analyze"
	case dataObj["format"] != nil && dataObj["is_text_like"] != nil && dataObj["recommended_tool"] == nil:
		// structure
		return compressSFStructure(dataObj), "sf-structure"
	case dataObj["content"] != nil && dataObj["sampling_strategy"] != nil:
		// sample or summarize: preserve content, compact wrapper
		return compressSFContent(raw, dataObj), "sf-content"
	default:
		return clean, "sf-generic"
	}
}

// compressSFAnalyze compacts analyze output to essential metadata.
// From: {"path":"...","size_bytes":12345,"line_count":500,"mime":"text/plain","extension":".go",
//
//	"group":"text","is_text_like":true,"is_large":true,"recommended_tool":"file_reader_advanced",
//	"default_strategy":"head_tail","next_steps":[...],"detected_encoding":"utf-8"}
//
// To: "file.go (12.1KB, 500 lines, text/plain, text, utf-8)\n  Recommended: file_reader_advanced (head_tail)"
func compressSFAnalyze(dataObj map[string]json.RawMessage) string {
	path := jsonString(dataObj["path"])
	sizeBytes := jsonInt(dataObj["size_bytes"])
	lineCount := jsonInt(dataObj["line_count"])
	mime := jsonString(dataObj["mime"])
	ext := jsonString(dataObj["extension"])
	group := jsonString(dataObj["group"])
	isText := jsonBool(dataObj["is_text_like"])
	isLarge := jsonBool(dataObj["is_large"])
	recTool := jsonString(dataObj["recommended_tool"])
	strategy := jsonString(dataObj["default_strategy"])
	encoding := jsonString(dataObj["detected_encoding"])

	var sb strings.Builder

	// Path and basic info
	sb.WriteString(path)
	if ext != "" {
		sb.WriteString(" (" + formatFileSize(int64(sizeBytes)))
		if lineCount > 0 {
			fmt.Fprintf(&sb, ", %d lines", lineCount)
		}
		if mime != "" {
			sb.WriteString(", " + mime)
		}
		sb.WriteString(")")
	}
	sb.WriteString("\n")

	// Text/binary status
	if isText {
		sb.WriteString("  Text file")
	} else {
		sb.WriteString("  Binary/non-text file")
	}
	if group != "" {
		sb.WriteString(" [" + group + "]")
	}
	if encoding != "" {
		sb.WriteString(" (" + encoding + ")")
	}
	if isLarge {
		sb.WriteString(" [large]")
	}
	sb.WriteString("\n")

	// Recommendation
	if recTool != "" {
		sb.WriteString("  Recommended: " + recTool)
		if strategy != "" {
			sb.WriteString(" (" + strategy + ")")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// compressSFStructure compacts structure detection output.
// From: {"path":"...","size_bytes":12345,"mime":"application/json","extension":".json",
//
//	"is_text_like":true,"format":"json","root_type":"array","top_level_keys":["name","age"]}
//
// To: "file.json (JSON, array, keys: name, age)"
func compressSFStructure(dataObj map[string]json.RawMessage) string {
	path := jsonString(dataObj["path"])
	sizeBytes := jsonInt(dataObj["size_bytes"])
	format := jsonString(dataObj["format"])
	isText := jsonBool(dataObj["is_text_like"])

	var sb strings.Builder

	sb.WriteString(path)
	sb.WriteString(" (" + formatFileSize(int64(sizeBytes)) + ")")
	sb.WriteString("\n")

	if format != "" {
		sb.WriteString("  Format: " + format)
	}
	if !isText {
		sb.WriteString(" [binary]")
	}
	sb.WriteString("\n")

	// JSON-specific
	if rootType := jsonString(dataObj["root_type"]); rootType != "" {
		sb.WriteString("  Root type: " + rootType + "\n")
	}
	if keys := dataObj["top_level_keys"]; keys != nil {
		var keyList []string
		if err := json.Unmarshal(keys, &keyList); err == nil && len(keyList) > 0 {
			limit := 15
			if len(keyList) < limit {
				limit = len(keyList)
			}
			sb.WriteString("  Keys: " + strings.Join(keyList[:limit], ", "))
			if len(keyList) > limit {
				fmt.Fprintf(&sb, " + %d more", len(keyList)-limit)
			}
			sb.WriteString("\n")
		}
	}

	// CSV-specific
	if headers := dataObj["headers"]; headers != nil {
		var headerList []string
		if err := json.Unmarshal(headers, &headerList); err == nil {
			sb.WriteString("  Headers: " + strings.Join(headerList, ", ") + "\n")
		}
	}
	if colCount := jsonInt(dataObj["column_count"]); colCount > 0 {
		fmt.Fprintf(&sb, "  Columns: %d\n", colCount)
	}

	// XML-specific
	if root := jsonString(dataObj["root_element"]); root != "" {
		sb.WriteString("  Root element: " + root + "\n")
	}

	// Text preview
	if preview := jsonString(dataObj["preview"]); preview != "" {
		if len(preview) > 200 {
			preview = preview[:197] + "..."
		}
		sb.WriteString("  Preview: " + preview + "\n")
	}

	return sb.String()
}

// compressSFContent preserves content from sample/summarize but compacts the wrapper.
// From: {"status":"success","message":"Built head_tail sample from file.go","data":{"path":"file.go",
//
//	"size_bytes":12345,"sampling_strategy":"head_tail","sample_sections":2,"content":"...","next_steps":[...]}}
//
// To: "file.go (12.1KB, head_tail, 2 sections):\n<content>"
func compressSFContent(raw map[string]json.RawMessage, dataObj map[string]json.RawMessage) string {
	path := jsonString(dataObj["path"])
	sizeBytes := jsonInt(dataObj["size_bytes"])
	strategy := jsonString(dataObj["sampling_strategy"])
	sections := jsonInt(dataObj["sample_sections"])
	content := jsonString(dataObj["content"])

	var sb strings.Builder

	if path != "" {
		sb.WriteString(path)
		if sizeBytes > 0 {
			sb.WriteString(" (" + formatFileSize(int64(sizeBytes)) + ")")
		}
	}
	if strategy != "" {
		sb.WriteString(" [" + strategy + "]")
	}
	if sections > 0 {
		fmt.Fprintf(&sb, " %d sections", sections)
	}
	sb.WriteString(":\n")

	// Content is preserved as-is
	sb.WriteString(content)

	return sb.String()
}
