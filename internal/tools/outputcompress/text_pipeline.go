// Package outputcompress – shell text pipeline compressors.
//
// Text processing tools like sort, uniq, cut, sed, awk, xargs, jq, tr, column,
// diff, comm, paste produce line-oriented output that can be very large.
//
// Strategy:
//   - Strip ANSI codes
//   - Collapse excessive whitespace
//   - Deduplicate consecutive identical lines
//   - Apply TailFocus for outputs exceeding maxLines
//   - jq: additionally compact JSON output
//   - diff: apply git-diff-style compression
package outputcompress

import (
	"bytes"
	"encoding/json"
	"strings"
)

// maxPipelineLines is the threshold above which TailFocus is applied.
const maxPipelineLines = 200

// compressTextPipeline is the shared compressor for text processing tools.
// It applies: StripANSI → CollapseWhitespace → DeduplicateLines → TailFocus.
func compressTextPipeline(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Count lines before dedup
	totalLines := strings.Count(result, "\n") + 1

	// Deduplicate consecutive identical lines
	result = DeduplicateLines(result)

	// Apply tail-focus for very long outputs
	if totalLines > maxPipelineLines {
		result = TailFocus(result, 5, maxPipelineLines-5, 3)
	}

	return result
}

// compressSort handles sort and sort -u outputs.
// Sorted output may have many consecutive duplicates.
func compressSort(output string) string {
	return compressTextPipeline(output)
}

// compressUniq handles uniq output.
// Already deduplicated but may be very long.
func compressUniq(output string) string {
	return compressTextPipeline(output)
}

// compressCut handles cut output.
// Columnar data that may be very wide or long.
func compressCut(output string) string {
	return compressTextPipeline(output)
}

// compressSed handles sed output.
// Transformed text that can be very large.
func compressSed(output string) string {
	return compressTextPipeline(output)
}

// compressAwk handles awk output.
// Output varies widely depending on the awk script.
func compressAwk(output string) string {
	return compressTextPipeline(output)
}

// compressXargs handles xargs output.
// Output is the result of executed commands.
func compressXargs(output string) string {
	return compressTextPipeline(output)
}

// compressJq handles jq output.
// JSON output is minified (indentation removed) then compacted.
func compressJq(output string) string {
	result := StripANSI(output)

	// Try JSON minification first (remove pretty-printing indentation)
	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(trimmed)); err == nil {
			result = buf.String()
		} else {
			// Fallback to field-level compaction
			result = compactJSON(result)
		}
	}

	result = CollapseWhitespace(result)

	// Count lines
	totalLines := strings.Count(result, "\n") + 1

	// Deduplicate
	result = DeduplicateLines(result)

	// Apply tail-focus for very long outputs
	if totalLines > maxPipelineLines {
		result = TailFocus(result, 5, maxPipelineLines-5, 3)
	}

	return result
}

// compressTr handles tr output.
// Character-level transformation, typically compact.
func compressTr(output string) string {
	return compressTextPipeline(output)
}

// compressColumn handles column output.
// Columnar formatting that may be wide.
func compressColumn(output string) string {
	return compressTextPipeline(output)
}

// compressDiff handles plain diff output (unified diff).
// Reuses git-diff compression logic.
func compressDiff(output string) string {
	return compressGitDiff(output)
}

// compressComm handles comm output (compare sorted files).
// Three-column output that is typically compact.
func compressComm(output string) string {
	return compressTextPipeline(output)
}

// compressPaste handles paste output (merge lines).
// Merged lines that may be wide.
func compressPaste(output string) string {
	return compressTextPipeline(output)
}

// compressSortU handles "sort -u" (sort + unique).
// Same as sort but explicitly deduplicated.
func compressSortU(output string) string {
	return compressTextPipeline(output)
}
