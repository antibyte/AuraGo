package shellpolicy

import (
	"regexp"
	"strings"
)

var commandAssignment = regexp.MustCompile("^[A-Za-z_][A-Za-z0-9_]*=")

// shortcut: normalize literal command heads only; expansions and arbitrary shell
// programs still need the existing OS isolation and trusted host-execution grants.
func normalizeCommandHeads(command string, escape byte) (string, bool) {
	var out strings.Builder
	breaks := " \t;&|\n"
	if escape != '\\' {
		breaks += "\r"
	}
	atHead, envOptions, envValue := true, false, false
	for i := 0; i < len(command); {
		c := command[i]
		if strings.IndexByte(breaks, c) >= 0 {
			out.WriteByte(c)
			i++
			if c != ' ' && c != '\t' {
				atHead, envOptions, envValue = true, false, false
			}
			continue
		}
		if c == '#' && escape != '^' {
			for i < len(command) && command[i] != '\n' && (escape != '`' || command[i] != '\r') {
				i++
			}
			continue
		}
		start := i
		word, end := literalCommandWord(command, i, escape, breaks)
		i = end
		if !atHead {
			if strings.HasPrefix(word, "-") {
				out.WriteString(word)
			} else {
				out.WriteString(command[start:end])
			}
			continue
		}
		if envValue {
			envValue = false
			continue
		}
		if envOptions && strings.HasPrefix(word, "-") {
			switch {
			case word == "--":
				envOptions = false
			case word == "-u" || word == "--unset" || word == "-C" || word == "--chdir":
				envValue = true
			case strings.HasPrefix(word, "-S") || strings.HasPrefix(word, "--split-string"):
				return out.String(), true
			}
			continue
		}
		if commandAssignment.MatchString(word) {
			continue
		}
		name := word[strings.LastIndexAny(word, "/\\")+1:]
		name = strings.TrimSuffix(strings.ToLower(name), ".exe")
		if name == "env" {
			envOptions = true
			continue
		}
		out.WriteString(name)
		atHead = false
	}
	return out.String(), false
}

// literalCommandWord removes literal quotes without evaluating shell expressions.
// Path backslashes stay intact; escaped separators never become command boundaries.
func literalCommandWord(command string, from int, escape byte, breaks string) (string, int) {
	var word strings.Builder
	var quote byte
	i := from
	for ; i < len(command); i++ {
		c := command[i]
		if quote == 0 && strings.IndexByte(breaks, c) >= 0 {
			break
		}
		if c == escape && quote != '\'' && (escape != '^' || quote == 0) && i+1 < len(command) {
			next := command[i+1]
			if next == '\n' {
				i++
				continue
			}
			if escape != '\\' && next == '\r' && i+2 < len(command) && command[i+2] == '\n' {
				i += 2
				continue
			}
			if next == escape || quote == 0 && strings.IndexByte(" \t\"';&|", next) >= 0 || quote == '"' && next == '"' {
				i++
				word.WriteByte(next)
				continue
			}
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		} else if c == '"' || c == '\'' && escape != '^' {
			quote = c
		} else {
			word.WriteByte(c)
		}
	}
	return word.String(), i
}
