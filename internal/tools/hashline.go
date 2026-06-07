package tools

import (
	"fmt"
	"hash/fnv"
	"strings"
)

const maxHashlineReadChars = 45 * 1024

// HashlineEntry represents one file line annotated with a content-only hash.
type HashlineEntry struct {
	LineNum int    `json:"line_number"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

func hashLineContent(content string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(content))
	return fmt.Sprintf("%08x", h.Sum32())
}

func buildHashlineEntries(data []byte) []HashlineEntry {
	lines := strings.Split(string(data), "\n")
	entries := make([]HashlineEntry, 0, len(lines))
	for i, line := range lines {
		entries = append(entries, HashlineEntry{
			LineNum: i + 1,
			Hash:    hashLineContent(line),
			Content: line,
		})
	}
	return entries
}

func formatHashlineOutput(entries []HashlineEntry) string {
	output, _, _ := formatHashlineOutputLimited(entries, 0)
	return output
}

func formatHashlineOutputLimited(entries []HashlineEntry, maxChars int) (string, int, bool) {
	var sb strings.Builder
	linesReturned := 0
	for _, entry := range entries {
		line := fmt.Sprintf("%d#%s:%s\n", entry.LineNum, entry.Hash, entry.Content)
		if maxChars > 0 && sb.Len()+len(line) > maxChars {
			return sb.String(), linesReturned, true
		}
		sb.WriteString(line)
		linesReturned++
	}
	return sb.String(), linesReturned, false
}

func validateHashlineAnchor(entries []HashlineEntry, lineNum int, expectedHash string) error {
	if lineNum <= 0 || strings.TrimSpace(expectedHash) == "" {
		return fmt.Errorf("anchor_line and anchor_hash are required for hashline operations")
	}
	if lineNum > len(entries) {
		return fmt.Errorf("anchor_line %d is out of range (file has %d lines)", lineNum, len(entries))
	}
	actualHash := entries[lineNum-1].Hash
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	if actualHash != expectedHash {
		return fmt.Errorf(
			"STALE CONTEXT: line %d has content hash %q, expected %q. The file has changed since you last read it. Re-read the file with include_hashes=true and try again",
			lineNum, actualHash, expectedHash,
		)
	}
	return nil
}
