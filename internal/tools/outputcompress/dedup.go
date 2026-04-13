package outputcompress

import (
	"fmt"
	"strings"
	"unicode"
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
		if count > 3 {
			fmt.Fprintf(&sb, "  [%d identical lines omitted]\n", count-1)
		} else if count > 1 {
			// For small repeats (2-3), just keep them – not worth the marker overhead
			for j := 1; j < count; j++ {
				sb.WriteString(prev)
				sb.WriteByte('\n')
			}
		}
		prev = lines[i]
		count = 1
	}

	// Flush last group
	sb.WriteString(prev)
	if count > 3 {
		fmt.Fprintf(&sb, "\n  [%d identical lines omitted]", count-1)
	} else if count > 1 {
		for j := 1; j < count; j++ {
			sb.WriteByte('\n')
			sb.WriteString(prev)
		}
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

// StripANSI removes common ANSI escape sequences from the output.
// Many shell commands produce coloured output that wastes tokens.
func StripANSI(input string) string {
	if !strings.Contains(input, "\x1b[") && !strings.Contains(input, "\033[") {
		return input
	}
	var sb strings.Builder
	sb.Grow(len(input))
	i := 0
	for i < len(input) {
		if input[i] == '\x1b' || (i+1 < len(input) && input[i] == '\033') {
			// Skip escape sequence: ESC [ ... letter
			i++
			if i < len(input) && input[i] == '[' {
				i++
				for i < len(input) && (input[i] < 'A' || input[i] > 'Z') && (input[i] < 'a' || input[i] > 'z') {
					i++
				}
				if i < len(input) {
					i++ // skip the terminating letter
				}
			}
			continue
		}
		sb.WriteByte(input[i])
		i++
	}
	return sb.String()
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

	// If still over 300 lines, apply tail-focus
	lines := strings.Count(result, "\n") + 1
	if lines > 300 {
		result = TailFocus(result, 50, 100, 10)
	}

	return result
}
