package security

import "strings"

// Datamark changes word separators, preserving indentation and Markdown code.
func Datamark(text string) string {
	var out strings.Builder
	fence := ""
	inline := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trim := strings.TrimLeft(line, " \t")
		if inline == 0 && (strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~")) {
			c := trim[0]
			n := 0
			for n < len(trim) && trim[n] == c {
				n++
			}
			if fence == "" {
				fence = trim[:n]
			} else if c == fence[0] && n >= len(fence) && strings.TrimSpace(trim[n:]) == "" {
				fence = ""
			}
			out.WriteString(line)
			continue
		}
		if fence != "" || (inline == 0 && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t"))) {
			out.WriteString(line)
			continue
		}
		indent := len(line) - len(trim)
		out.WriteString(line[:indent])
		line = line[indent:]
		for i := 0; i < len(line); {
			if line[i] == '`' {
				end := i + 1
				for end < len(line) && line[end] == '`' {
					end++
				}
				n := end - i
				if inline == 0 {
					inline = n
				} else if inline == n {
					inline = 0
				}
				out.WriteString(line[i:end])
				i = end
				continue
			}
			if inline == 0 && (line[i] == ' ' || line[i] == '\t') {
				end := i + 1
				for end < len(line) && (line[end] == ' ' || line[end] == '\t') {
					end++
				}
				if i > 0 && end < len(line) && line[end] != '\r' && line[end] != '\n' {
					out.WriteByte('^')
				} else {
					out.WriteString(line[i:end])
				}
				i = end
				continue
			}
			out.WriteByte(line[i])
			i++
		}
	}
	return out.String()
}
