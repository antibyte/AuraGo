package outputcompress

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ─── TailFocus Konstanten ────────────────────────────────────────────────────
// Konsistente Parameter für TailFocus-Funktionen
const (
	// Default: 30 head + 70 tail + 10 min gap = 110 lines threshold
	tailFocusHeadDefault = 30
	tailFocusTailDefault = 70
	tailFocusMinGap      = 10

	// Logs: 20 head + 80 tail (mehr Kontext am Ende wichtig)
	tailFocusLogsHead   = 20
	tailFocusLogsTail   = 80
	tailFocusLogsMinGap = 5

	// Code/Python: 25 head + 75 tail (Stacktrace braucht beides)
	tailFocusCodeHead   = 25
	tailFocusCodeTail   = 75
	tailFocusCodeMinGap = 5

	// API/JSON: 20 head + 50 tail (JSON-Struktur oft am Anfang)
	tailFocusAPIHead   = 20
	tailFocusAPITail   = 50
	tailFocusAPIMinGap = 5

	// Generic threshold: Ab wann TailFocus sinnvoll ist
	// Berechnung: headDefault + tailDefault + minGap + 20% Puffer
	minLinesForTailFocus = 150
)

// DeduplicateLines collapses consecutive identical lines into a single
// occurrence with a count marker, e.g. "×42 identical lines omitted".
// Lines that differ even by whitespace are treated as distinct.
func DeduplicateLines(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	if len(lines) <= 1 {
		return input
	}

	var sb strings.Builder
	prev := lines[0]
	count := 1

	for i := 1; i < len(lines); i++ {
		if lines[i] == prev {
			count++
			continue
		}
		sb.WriteString(prev)
		sb.WriteByte('\n')
		// Marker ist ~35 chars: "  [X identical lines omitted]\n"
		// Break-even bereits bei 2 Wiederholungen (2x 20-char lines = 40 chars)
		if count > 1 {
			fmt.Fprintf(&sb, "  [%d identical lines omitted]\n", count)
		}
		prev = lines[i]
		count = 1
	}

	// Flush last group
	sb.WriteString(prev)
	if count > 1 {
		fmt.Fprintf(&sb, "\n  [%d identical lines omitted]", count)
	}

	return sb.String()
}

// CollapseWhitespace removes excessive blank lines and trims trailing
// whitespace from each line. It preserves single blank lines as separators.
func CollapseWhitespace(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	var sb strings.Builder
	prevBlank := false

	for _, line := range lines {
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		isBlank := trimmed == ""

		if isBlank && prevBlank {
			// Skip consecutive blank lines
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(trimmed)
		prevBlank = isBlank
	}

	return sb.String()
}

// TailFocus keeps the first headLines and the last tailLines of the input,
// inserting a marker for the omitted middle section. If the total line count
// is less than headLines+tailLines+minGap, the input is returned unchanged.
func TailFocus(input string, headLines, tailLines, minGap int) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	total := len(lines)

	threshold := headLines + tailLines + minGap
	if total <= threshold {
		return input
	}

	var sb strings.Builder
	for i := 0; i < headLines && i < total; i++ {
		sb.WriteString(lines[i])
		sb.WriteByte('\n')
	}

	fmt.Fprintf(&sb, "\n  [... %d lines omitted ...]\n\n", total-headLines-tailLines)

	start := total - tailLines
	if start < headLines {
		start = headLines
	}
	for i := start; i < total; i++ {
		sb.WriteString(lines[i])
		if i < total-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// ─── ANSI Escape Sequence Handling ───────────────────────────────────────────

// ansiRegex matches all common ANSI escape sequences including:
// - SGR codes (colors, styles): \x1b[0m, \x1b[31m, \x1b[1;31m
// - 256 color codes: \x1b[38;5;NNm
// - True color: \x1b[38;2;R;G;Bm
// - Cursor movement: \x1b[H, \x1b[2J
// - Window title: \x1b]0;title\x07
// - Hyperlinks: \x1b]8;;url\x1b\\text\x1b]8;;\x1b\\
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\\`)

// StripANSI removes all ANSI escape sequences from the output.
// Many shell commands produce coloured output that wastes tokens.
func StripANSI(input string) string {
	if !strings.ContainsAny(input, "\x1b\033") {
		return input
	}
	return ansiRegex.ReplaceAllString(input, "")
}

// compressGeneric applies the standard generic compression pipeline:
// 1. Strip ANSI escape codes
// 2. Collapse whitespace
// 3. Deduplicate consecutive identical lines
// 4. Apply tail-focus if still too long
func compressGeneric(input string) string {
	result := StripANSI(input)
	result = CollapseWhitespace(result)
	result = DeduplicateLines(result)

	// If still over threshold, apply tail-focus with default parameters
	lines := strings.Count(result, "\n") + 1
	if lines > minLinesForTailFocus {
		result = TailFocus(result, tailFocusHeadDefault, tailFocusTailDefault, tailFocusMinGap)
	}

	return result
}
