package desktop

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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

func isNoteTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-' || r == '/'
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
