package dockerstacks

import (
	"regexp"
	"strconv"
	"strings"
)

// problemOpenQuote marks a value whose opening quote is never closed; such a line defines nothing.
const problemOpenQuote = "the quote is never closed"

// maxEnvWarnings bounds the env_syntax warnings of one file; a .env with more broken lines than this is not worth listing.
const maxEnvWarnings = 1000

// envKeyPattern is what a .env key may look like (Compose's reader is a little wider than interpolation itself).
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// parseEnvFile reads a .env file the way Compose does for the cases that matter to lint: comments, blank lines,
// `export KEY=VALUE`, unquoted values (an inline " #" starts a comment), single-quoted values (literal), double-quoted
// values (a few escapes, may span lines). It returns the defined variables and one env_syntax warning per line that
// is not KEY=VALUE. A later definition of a key replaces an earlier one.
func parseEnvFile(src string) (map[string]string, []Diagnostic) {
	env := map[string]string{}
	diags := []Diagnostic{}
	src = strings.TrimPrefix(src, "\xef\xbb\xbf") // UTF-8 byte order mark
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	warn := func(line int, detail string) {
		if len(diags) >= maxEnvWarnings {
			return
		}
		diags = append(diags, Diagnostic{
			Line: line, Severity: "warning", Code: "env_syntax",
			Message: ".env line " + strconv.Itoa(line) + " is not KEY=VALUE, a comment or blank: " + detail,
			Params:  map[string]string{"file": ".env", "detail": detail},
		})
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
			line = strings.TrimLeft(line[len("export"):], " \t")
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			warn(i+1, "there is no '=' after the name")
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if !envKeyPattern.MatchString(key) {
			warn(i+1, "invalid variable name")
			continue
		}
		value, extra, problem := parseEnvValue(strings.TrimLeft(line[eq+1:], " \t"), lines[i+1:])
		if problem != "" {
			warn(i+1, problem)
		}
		if problem != problemOpenQuote {
			env[key] = value
		}
		i += extra
	}
	return env, diags
}

// parseEnvValue parses the text after "KEY=". following holds the lines after the current one, which a
// double- or single-quoted value may continue into; extra is how many of them were consumed.
func parseEnvValue(rest string, following []string) (value string, extra int, problem string) {
	if rest == "" {
		return "", 0, ""
	}
	q := rest[0]
	if q != '"' && q != '\'' {
		for k := 1; k < len(rest); k++ {
			if rest[k] == '#' && (rest[k-1] == ' ' || rest[k-1] == '\t') {
				rest = rest[:k] // an inline comment starts at a "#" that follows a blank
				break
			}
		}
		return strings.TrimSpace(rest), 0, ""
	}
	body := rest[1:]
	for n := 0; ; n++ {
		if end := closingQuote(body, q); end >= 0 {
			tail := strings.TrimSpace(body[end+1:])
			if tail != "" && !strings.HasPrefix(tail, "#") {
				problem = "unexpected text after the closing quote"
			}
			text := body[:end]
			if q == '"' {
				text = unescapeDoubleQuoted(text)
			}
			return text, n, problem
		}
		if n >= len(following) {
			return "", 0, problemOpenQuote
		}
		body += "\n" + strings.TrimSuffix(following[n], "\r")
	}
}

// closingQuote returns the index of the first unescaped q in s, or -1.
func closingQuote(s string, q byte) int {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && q == '"':
			i++
		case s[i] == q:
			return i
		}
	}
	return -1
}

var doubleQuotedEscapes = strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t", `\"`, `"`, `\\`, `\`, `\$`, `$`)

func unescapeDoubleQuoted(s string) string { return doubleQuotedEscapes.Replace(s) }

// varRef is one use of a Compose variable that interpolate saw.
type varRef struct {
	Name     string
	Default  bool // ${X:-d} ${X-d} ${X:+r} ${X+r}: the file says what happens when X is unset
	Required bool // ${X:?msg} ${X?msg}
	Set      bool // X has a value for this operator (non-empty for the ":" forms)
}

func isNameStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isNameChar(c byte) bool  { return isNameStart(c) || (c >= '0' && c <= '9') }

// interpolate expands Compose variable syntax in s from env: "$$" is a literal dollar, $NAME and ${NAME} are
// substituted, ${NAME:-d} ${NAME-d} ${NAME:+r} ${NAME+r} ${NAME:?m} ${NAME?m} follow Compose's rules and may nest.
// visit (optional) is told about every variable that is actually evaluated. unresolved reports that a variable
// without a default had no value, so the result is not what Compose would deploy. Text that is not valid
// variable syntax stays as it is.
func interpolate(s string, env map[string]string, visit func(varRef)) (out string, unresolved bool) {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c != '$' || i+1 >= len(s) {
			b.WriteByte(c)
			i++
			continue
		}
		next := s[i+1]
		switch {
		case next == '$':
			b.WriteByte('$')
			i += 2
		case next == '{':
			end := matchBrace(s, i+1)
			if end < 0 {
				b.WriteString(s[i:])
				return b.String(), unresolved
			}
			text, unres, ok := expandBraced(s[i+2:end], env, visit)
			if !ok {
				text = s[i : end+1]
			}
			b.WriteString(text)
			unresolved = unresolved || unres
			i = end + 1
		case isNameStart(next):
			j := i + 2
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			val, set := env[s[i+1:j]]
			if visit != nil {
				visit(varRef{Name: s[i+1 : j], Set: set})
			}
			unresolved = unresolved || !set
			b.WriteString(val)
			i = j
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), unresolved
}

// matchBrace returns the index of the "}" that closes the "{" at s[open], or -1.
func matchBrace(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '$':
			if i+1 < len(s) && s[i+1] == '$' {
				i++
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// expandBraced expands the inside of ${...}. ok is false when the text is not valid variable syntax.
func expandBraced(body string, env map[string]string, visit func(varRef)) (text string, unresolved, ok bool) {
	if body == "" || !isNameStart(body[0]) {
		return "", false, false
	}
	n := 1
	for n < len(body) && isNameChar(body[n]) {
		n++
	}
	name, op := body[:n], body[n:]
	val, defined := env[name]
	report := func(r varRef) {
		r.Name = name
		if visit != nil {
			visit(r)
		}
	}
	sub := func(arg string) (string, bool) { return interpolate(arg, env, visit) }
	switch {
	case op == "":
		report(varRef{Set: defined})
		return val, !defined, true
	case strings.HasPrefix(op, ":-"), strings.HasPrefix(op, "-"):
		colon := op[0] == ':'
		set := defined && (!colon || val != "")
		report(varRef{Default: true, Set: set})
		if set {
			return val, false, true
		}
		d, unres := sub(op[strings.IndexByte(op, '-')+1:])
		return d, unres, true
	case strings.HasPrefix(op, ":+"), strings.HasPrefix(op, "+"):
		colon := op[0] == ':'
		set := defined && (!colon || val != "")
		report(varRef{Default: true, Set: set})
		if !set {
			return "", false, true
		}
		r, unres := sub(op[strings.IndexByte(op, '+')+1:])
		return r, unres, true
	case strings.HasPrefix(op, ":?"), strings.HasPrefix(op, "?"):
		colon := op[0] == ':'
		set := defined && (!colon || val != "")
		report(varRef{Required: true, Set: set})
		return val, !set, true
	}
	return "", false, false
}
