// Package outputcompress - Koofr cloud storage output compressors.
//
// Koofr list responses can include full hashes, tags, content metadata, and
// dozens or hundreds of files. The agent usually needs names, rough sizes, and
// counts, so list output is reduced to a compact directory summary.
package outputcompress

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	koofrDirListLimit  = 16
	koofrFileListLimit = 24
)

type koofrListFile struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Modified    int64  `json:"modified"`
}

// compressKoofrOutput routes Koofr tool output to the appropriate sub-compressor.
func compressKoofrOutput(output string) (string, string) {
	clean := StripANSI(output)
	clean = strings.TrimPrefix(clean, "Tool Output: ")
	clean = strings.TrimSpace(clean)

	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "koofr-nonjson"
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "koofr-parse-err"
	}

	if statusStr := jsonString(raw["status"]); strings.EqualFold(statusStr, "error") {
		return clean, "koofr-error"
	}

	files, ok := koofrFilesFromRaw(raw)
	if ok {
		return compressKoofrList(files), "koofr-list"
	}

	return compactJSON(clean), "koofr-generic"
}

func koofrFilesFromRaw(raw map[string]json.RawMessage) ([]koofrListFile, bool) {
	if raw["files"] != nil {
		var files []koofrListFile
		if err := json.Unmarshal(raw["files"], &files); err == nil {
			return files, true
		}
	}

	if raw["response"] == nil {
		return nil, false
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(raw["response"], &response); err != nil {
		return nil, false
	}
	if response["files"] == nil {
		return nil, false
	}
	var files []koofrListFile
	if err := json.Unmarshal(response["files"], &files); err != nil {
		return nil, false
	}
	return files, true
}

func compressKoofrList(files []koofrListFile) string {
	dirs := make([]koofrListFile, 0)
	regularFiles := make([]koofrListFile, 0)
	typeCounts := make(map[string]int)
	var totalBytes int64

	for _, file := range files {
		if strings.EqualFold(file.Type, "dir") {
			dirs = append(dirs, file)
			continue
		}
		regularFiles = append(regularFiles, file)
		totalBytes += file.Size
		if file.ContentType != "" {
			typeCounts[file.ContentType]++
		} else if file.Type != "" {
			typeCounts[file.Type]++
		} else {
			typeCounts["unknown"]++
		}
	}

	sort.SliceStable(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.SliceStable(regularFiles, func(i, j int) bool {
		return strings.ToLower(regularFiles[i].Name) < strings.ToLower(regularFiles[j].Name)
	})

	var sb strings.Builder
	fmt.Fprintf(&sb, "Koofr list: %d items (%d dirs, %d files", len(files), len(dirs), len(regularFiles))
	if totalBytes > 0 {
		fmt.Fprintf(&sb, ", files total %s", formatFileSize(totalBytes))
	}
	sb.WriteString(")")
	if len(typeCounts) > 0 {
		sb.WriteString("; types: ")
		sb.WriteString(formatKoofrTypeCounts(typeCounts))
	}
	sb.WriteString("\n")

	if len(dirs) > 0 {
		sb.WriteString("Directories:\n")
		limit := min(len(dirs), koofrDirListLimit)
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&sb, "  %s/\n", dirs[i].Name)
		}
		if len(dirs) > limit {
			fmt.Fprintf(&sb, "  + %d more dirs\n", len(dirs)-limit)
		}
	}

	if len(regularFiles) > 0 {
		sb.WriteString("Files:\n")
		limit := min(len(regularFiles), koofrFileListLimit)
		for i := 0; i < limit; i++ {
			file := regularFiles[i]
			fmt.Fprintf(&sb, "  %s", file.Name)
			if file.Size > 0 {
				fmt.Fprintf(&sb, " (%s)", formatFileSize(file.Size))
			}
			if file.ContentType != "" {
				fmt.Fprintf(&sb, " [%s]", file.ContentType)
			}
			sb.WriteString("\n")
		}
		if len(regularFiles) > limit {
			fmt.Fprintf(&sb, "  + %d more files\n", len(regularFiles)-limit)
			sb.WriteString("Use a narrower Koofr path or read/download a named file for details.\n")
		}
	}

	return sb.String()
}

func formatKoofrTypeCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}
