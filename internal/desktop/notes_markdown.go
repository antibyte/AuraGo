package desktop

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// NoteRefLink is one link found in a note. Start/End are the byte span of Value in the content.
type NoteRefLink struct {
	Kind  string // "wiki" or "markdown"
	Value string // wiki inner text or markdown URL
	Embed bool
	Start int
	End   int
}

// NoteRefs is everything the notes index needs from one Markdown note.
type NoteRefs struct {
	Title       string
	TitleSource string // "frontmatter", "heading", "body" or "" (no title found)
	Aliases     []string
	Tags        []string // lowercased, de-duplicated, frontmatter first then inline order
	Links       []NoteRefLink
}

// WikiValue is the parsed inner text of [[target#subpath|alias]].
type WikiValue struct{ Target, Subpath, Alias string }

const maxWikiValueRunes = 500

var (
	noteURLScheme   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)
	noteAttachExtOK = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".pdf": true}
)

// splitFrontmatter returns the YAML text and the offset of the first body byte.
func splitFrontmatter(text string) (meta string, bodyStart int, ok bool) {
	nl := strings.IndexByte(text, '\n')
	if nl < 0 || strings.TrimSpace(text[:nl]) != "---" {
		return "", 0, false
	}
	pos := nl + 1
	for pos <= len(text) {
		end := strings.IndexByte(text[pos:], '\n')
		lineEnd, next := len(text), len(text)
		if end >= 0 {
			lineEnd, next = pos+end, pos+end+1
		}
		if strings.TrimSpace(text[pos:lineEnd]) == "---" {
			return text[nl+1 : pos], next, true
		}
		if end < 0 {
			break
		}
		pos = next
	}
	return "", 0, false
}

func yamlScalarString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case nil, map[string]interface{}, []interface{}:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

// yamlStringList reads a string or a list; strings are split on commas (and whitespace when words is set).
func yamlStringList(v interface{}, words bool) []string {
	split := func(s string) []string {
		if words {
			return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
		}
		return strings.Split(s, ",")
	}
	var out []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	switch value := v.(type) {
	case string:
		for _, part := range split(value) {
			add(part)
		}
	case []interface{}:
		for _, item := range value {
			if text, ok := item.(string); ok && words {
				for _, part := range split(text) {
					add(part)
				}
			} else {
				add(yamlScalarString(item))
			}
		}
	case nil, map[string]interface{}:
	default:
		add(yamlScalarString(value))
	}
	return out
}

func noteLinkURLOK(url string) bool {
	if url == "" || noteURLScheme.MatchString(url) || strings.HasPrefix(url, "/") || strings.HasPrefix(url, "#") {
		return false
	}
	pathPart := url
	if i := strings.IndexAny(pathPart, "#?"); i >= 0 {
		pathPart = pathPart[:i]
	}
	lower := strings.ToLower(pathPart)
	if strings.HasSuffix(lower, ".md") {
		return true
	}
	dot := strings.LastIndexByte(lower, '.')
	return dot >= 0 && noteAttachExtOK[lower[dot:]]
}

// wikiLinkSpan finds the end of a wikilink whose "[[" starts at index start of line.
func wikiLinkSpan(line string, start int) (valueStart, valueEnd int, ok bool) {
	valueStart = start + 2
	j := valueStart
	for j < len(line) {
		switch c := line[j]; {
		case c == '\\':
			j += 2
			continue
		case c == '[' && j+1 < len(line) && line[j+1] == '[':
			return 0, 0, false
		case c == ']' && j+1 < len(line) && line[j+1] == ']':
			n := utf8.RuneCountInString(line[valueStart:j])
			return valueStart, j, n > 0 && n <= maxWikiValueRunes
		}
		j++
	}
	return 0, 0, false
}

func isNoteTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' || r == '/'
}

// ExtractNoteRefs returns title, aliases, tags and links of a Markdown note.
// Frontmatter, fenced code, inline code spans and HTML comments are not scanned for inline syntax.
func ExtractNoteRefs(content string) NoteRefs {
	refs := NoteRefs{Aliases: []string{}, Tags: []string{}, Links: []NoteRefLink{}}
	seen := map[string]bool{}
	addTag := func(tag string) {
		tag = strings.ToLower(strings.TrimLeft(strings.TrimSpace(tag), "#"))
		if tag != "" && !seen[tag] {
			seen[tag] = true
			refs.Tags = append(refs.Tags, tag)
		}
	}
	bodyStart := 0
	fmTitle := ""
	if meta, start, ok := splitFrontmatter(content); ok {
		bodyStart = start
		var data map[string]interface{}
		if yaml.Unmarshal([]byte(meta), &data) == nil {
			fmTitle = yamlScalarString(data["title"])
			for _, key := range []string{"aliases", "alias"} {
				refs.Aliases = append(refs.Aliases, yamlStringList(data[key], false)...)
			}
			for _, tag := range yamlStringList(data["tags"], true) {
				addTag(tag)
			}
		}
	}

	heading, firstLine := "", ""
	var fenceChar byte
	fenceLen := 0
	inComment := false
	for pos := bodyStart; pos < len(content); {
		lineEnd, next := len(content), len(content)
		if end := strings.IndexByte(content[pos:], '\n'); end >= 0 {
			lineEnd, next = pos+end, pos+end+1
		}
		line := content[pos:lineEnd]
		base := pos
		pos = next
		trimmed := strings.TrimRight(line, "\r")

		if fenceLen > 0 {
			t := strings.TrimSpace(trimmed)
			if len(t) >= fenceLen && strings.Trim(t, string(fenceChar)) == "" {
				fenceLen = 0
			}
			continue
		}
		if !inComment {
			t := strings.TrimLeft(trimmed, " \t")
			if len(t) >= 3 && (t[0] == '`' || t[0] == '~') {
				n := 0
				for n < len(t) && t[n] == t[0] {
					n++
				}
				if n >= 3 && !(t[0] == '`' && strings.ContainsRune(t[n:], '`')) {
					fenceChar, fenceLen = t[0], n
					continue
				}
			}
			if heading == "" && strings.HasPrefix(trimmed, "# ") {
				heading = strings.TrimSpace(trimmed[2:])
			}
			if firstLine == "" {
				firstLine = strings.TrimSpace(trimmed)
			}
		}

		i := 0
		if inComment {
			j := strings.Index(line, "-->")
			if j < 0 {
				continue
			}
			i, inComment = j+3, false
		}
		for i < len(line) {
			c := line[i]
			switch {
			case c == '\\':
				i += 2
			case c == '`':
				n := 0
				for i+n < len(line) && line[i+n] == '`' {
					n++
				}
				closeAt := -1
				for k := i + n; k < len(line); {
					if line[k] != '`' {
						k++
						continue
					}
					m := 0
					for k+m < len(line) && line[k+m] == '`' {
						m++
					}
					if m == n {
						closeAt = k
						break
					}
					k += m
				}
				if closeAt >= 0 {
					i = closeAt + n
				} else {
					i += n
				}
			case c == '<' && strings.HasPrefix(line[i:], "<!--"):
				if j := strings.Index(line[i+4:], "-->"); j >= 0 {
					i += 4 + j + 3
				} else {
					inComment = true
					i = len(line)
				}
			case c == '[' && strings.HasPrefix(line[i:], "[["), c == '!' && strings.HasPrefix(line[i:], "![["):
				embed := c == '!'
				open := i
				if embed {
					open = i + 1
				}
				if vs, ve, ok := wikiLinkSpan(line, open); ok {
					refs.Links = append(refs.Links, NoteRefLink{Kind: "wiki", Value: line[vs:ve], Embed: embed, Start: base + vs, End: base + ve})
					i = ve + 2
				} else {
					i = open + 1
				}
			case c == ']' && i+1 < len(line) && line[i+1] == '(':
				vs, ve, after, ok := markdownLinkURL(line, i+2)
				if !ok {
					i += 2
					break
				}
				if url := line[vs:ve]; noteLinkURLOK(url) {
					refs.Links = append(refs.Links, NoteRefLink{Kind: "markdown", Value: url, Embed: markdownLinkIsImage(line, i), Start: base + vs, End: base + ve})
				}
				i = after
			case c == '#':
				prevOK := i == 0
				if !prevOK {
					r, _ := utf8.DecodeLastRuneInString(line[:i])
					prevOK = unicode.IsSpace(r) || r == '('
				}
				j := i + 1
				if prevOK {
					for j < len(line) {
						r, size := utf8.DecodeRuneInString(line[j:])
						if !isNoteTagRune(r) {
							break
						}
						j += size
					}
					if value := line[i+1 : j]; value != "" && strings.IndexFunc(value, func(r rune) bool { return !unicode.IsDigit(r) }) >= 0 {
						addTag(value)
					}
				}
				i = j
			default:
				i++
			}
		}
	}

	switch {
	case fmTitle != "":
		refs.Title, refs.TitleSource = fmTitle, "frontmatter"
	case heading != "":
		refs.Title, refs.TitleSource = heading, "heading"
	case firstLine != "":
		refs.Title, refs.TitleSource = firstLine, "body"
	}
	refs.Title = stringLimit(refs.Title, 160)
	return refs
}

// markdownLinkURL parses the destination after "](" and returns the URL span and the index after ")".
func markdownLinkURL(line string, start int) (vs, ve, after int, ok bool) {
	if start < len(line) && line[start] == '<' {
		k := strings.IndexByte(line[start+1:], '>')
		if k < 0 {
			return 0, 0, 0, false
		}
		vs, ve = start+1, start+1+k
		start = ve + 1
	} else {
		ve = start
		for ve < len(line) && line[ve] != ' ' && line[ve] != '\t' && line[ve] != ')' {
			ve++
		}
		vs, start = start, ve
	}
	closeAt := strings.IndexByte(line[start:], ')')
	if closeAt < 0 {
		return 0, 0, 0, false
	}
	return vs, ve, start + closeAt + 1, true
}

// markdownLinkIsImage reports whether the "]" at index close ends an image alt text ("![alt]").
func markdownLinkIsImage(line string, close int) bool {
	depth := 0
	for b := close - 1; b >= 0; b-- {
		switch line[b] {
		case ']':
			depth++
		case '[':
			if depth == 0 {
				return b > 0 && line[b-1] == '!'
			}
			depth--
		}
	}
	return false
}

// wikiAliasSep finds the alias separator: an unescaped "|" or an escaped "\|" (table cells).
func wikiAliasSep(value string) (idx, width int) {
	for i := 0; i < len(value); i++ {
		switch {
		case value[i] == '\\' && i+1 < len(value) && value[i+1] == '|':
			return i, 2
		case value[i] == '\\':
			i++
		case value[i] == '|':
			return i, 1
		}
	}
	return -1, 0
}

// ParseWikiValue splits the inner text of a wikilink into target, subpath and alias.
func ParseWikiValue(value string) WikiValue {
	head, alias := value, ""
	if idx, width := wikiAliasSep(value); idx >= 0 {
		head, alias = value[:idx], value[idx+width:]
	}
	if hash := strings.IndexByte(head, '#'); hash >= 0 {
		return WikiValue{Target: head[:hash], Subpath: head[hash+1:], Alias: alias}
	}
	return WikiValue{Target: head, Alias: alias}
}

// ReplaceWikiTarget swaps the target of a wikilink value and keeps "#sub" and "|alias" exactly.
func ReplaceWikiTarget(value, target string) string {
	head := value
	if idx, _ := wikiAliasSep(value); idx >= 0 {
		head = value[:idx]
	}
	if hash := strings.IndexByte(head, '#'); hash >= 0 {
		head = head[:hash]
	}
	return target + value[len(head):]
}

// RewriteNoteLinks replaces link values in place. Code, comments and frontmatter stay untouched.
func RewriteNoteLinks(content string, rewrite func(NoteRefLink) (string, bool)) string {
	var out strings.Builder
	last := 0
	for _, link := range ExtractNoteRefs(content).Links {
		if replacement, ok := rewrite(link); ok {
			out.WriteString(content[last:link.Start])
			out.WriteString(replacement)
			last = link.End
		}
	}
	if last == 0 {
		return content
	}
	out.WriteString(content[last:])
	return out.String()
}
