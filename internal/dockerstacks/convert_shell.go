package dockerstacks

import (
	"errors"
	"strings"
)

// splitCommand splits a pasted command line into words the way a shell would, without running anything:
//
//   - blanks and newlines separate words; a backslash (POSIX) or backtick (PowerShell) before a line break joins the
//     lines, trailing blanks after it are tolerated;
//   - single quotes keep their content literally, double quotes allow an escaped quote, backslash, dollar sign or
//     backtick (the PowerShell escape character is the backtick), a backslash before any other character stays (so
//     C:\data survives);
//   - $(...) is kept as one piece, a word that starts with # begins a comment.
//
// The result is meant for a compose file, where a lone "$" starts a variable: a dollar sign the shell would have kept
// literally (single quotes, \$) is returned as "$$", so Compose reads the same text back. An unbalanced quote is an
// error.
func splitCommand(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	literal := func(c byte) { // a character that must reach the compose file unchanged
		if c == '$' {
			cur.WriteString("$$")
		} else {
			cur.WriteByte(c)
		}
		inWord = true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			flush()
		case c == '\\' || c == '`':
			if end, ok := lineContinuation(s, i+1); ok {
				i = end
				continue
			}
			if i+1 < len(s) && escapable(s[i+1], c) {
				i++
				literal(s[i])
				continue
			}
			cur.WriteByte(c) // a backslash in C:\data or a backtick pair around `pwd`
			inWord = true
		case c == '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, errors.New("unbalanced single quote")
			}
			for _, q := range []byte(s[i+1 : i+1+end]) {
				literal(q)
			}
			inWord = true
			i += end + 1
		case c == '"':
			next, err := doubleQuoted(s, i+1, &cur)
			if err != nil {
				return nil, err
			}
			inWord = true
			i = next
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			end := closingParen(s, i+1)
			cur.WriteString(s[i:end])
			inWord = true
			i = end - 1
		case c == '#' && !inWord:
			for i < len(s) && s[i] != '\n' {
				i++
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return words, nil
}

// lineContinuation reports whether s[from:] is optional blanks followed by a line break and returns the index of the
// last character of that break.
func lineContinuation(s string, from int) (end int, ok bool) {
	j := from
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r') {
		j++
	}
	if j < len(s) && s[j] == '\n' {
		return j, true
	}
	return 0, false
}

// escapable tells whether the character after an escape character is taken literally. esc is '\\' or '`'.
func escapable(next, esc byte) bool {
	if esc == '`' {
		return next == '`' || next == '"' || next == '\'' || next == '$'
	}
	return strings.IndexByte(" \t\"'\\$`#()&;|<>*?[]{}!~", next) >= 0
}

// doubleQuoted appends the content of a double-quoted section that starts at s[from] to cur and returns the index of
// its closing quote.
func doubleQuoted(s string, from int, cur *strings.Builder) (int, error) {
	for i := from; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			return i, nil
		case (c == '\\' || c == '`') && i+1 < len(s):
			if end, ok := lineContinuation(s, i+1); ok && c == '\\' {
				i = end
				continue
			}
			if n := s[i+1]; n == '"' || n == '\\' || n == '`' || n == '$' {
				i++
				if n == '$' {
					cur.WriteString("$$")
				} else {
					cur.WriteByte(n)
				}
				continue
			}
			cur.WriteByte(c)
		default:
			cur.WriteByte(c)
		}
	}
	return 0, errors.New("unbalanced double quote")
}

// closingParen returns the index just after the ")" that closes the "(" at s[open]; an unclosed one ends at the end.
func closingParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
