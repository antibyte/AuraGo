package desktop

import (
	"net/url"
	"path"
	"sort"
	"strings"
)

const notesRootLower = "documents/notes/"

type noteLookupEntry struct {
	path  string
	lower string
	dir   string // lowercase folder
	base  string // lowercase name: notes without ".md", attachments with extension
}

// noteLookupSet indexes a list of paths for the resolution steps shared by notes and attachments.
type noteLookupSet struct {
	entries []noteLookupEntry
	byLower map[string]int
	byBase  map[string][]int
	suffix  string // ".md" for notes, "" for attachments
}

func newNoteLookupSet(paths []string, suffix string) *noteLookupSet {
	set := &noteLookupSet{byLower: map[string]int{}, byBase: map[string][]int{}, suffix: suffix}
	for _, p := range paths {
		p = strings.ReplaceAll(p, "\\", "/")
		lower := strings.ToLower(p)
		if _, dup := set.byLower[lower]; dup {
			continue
		}
		base := path.Base(lower)
		if suffix != "" {
			base = strings.TrimSuffix(base, suffix)
		}
		set.entries = append(set.entries, noteLookupEntry{path: p, lower: lower, dir: path.Dir(lower), base: base})
		idx := len(set.entries) - 1
		set.byLower[lower] = idx
		set.byBase[base] = append(set.byBase[base], idx)
	}
	return set
}

// pick returns the shortest path (then alphabetical) of the candidates.
func (set *noteLookupSet) pick(cands []int) string {
	best := -1
	for _, c := range cands {
		if best < 0 {
			best = c
			continue
		}
		a, b := set.entries[c], set.entries[best]
		if len(a.lower) < len(b.lower) || (len(a.lower) == len(b.lower) && a.lower < b.lower) {
			best = c
		}
	}
	if best < 0 {
		return ""
	}
	return set.entries[best].path
}

// find resolves a name (notes: without .md; attachments: with extension) as seen from the note "from".
func (set *noteLookupSet) find(from, name string) string {
	n := strings.ToLower(name)
	if strings.Contains(n, "/") {
		if i, ok := set.byLower[notesRootLower+n+set.suffix]; ok {
			return set.entries[i].path
		}
		rel := strings.ToLower(path.Join(path.Dir(from), n)) + set.suffix
		if i, ok := set.byLower[rel]; ok {
			return set.entries[i].path
		}
		var cands []int
		for i, e := range set.entries {
			if strings.HasSuffix(e.lower, "/"+n+set.suffix) {
				cands = append(cands, i)
			}
		}
		return set.pick(cands)
	}
	cands := set.byBase[n]
	if len(cands) > 1 {
		fromDir := strings.ToLower(path.Dir(from))
		var same []int
		for _, c := range cands {
			if set.entries[c].dir == fromDir {
				same = append(same, c)
			}
		}
		if len(same) > 0 {
			cands = same
		}
	}
	return set.pick(cands)
}

type noteLookup struct {
	notes   *noteLookupSet
	atts    *noteLookupSet
	byAlias map[string][]int
}

func newNoteLookup(notes []string, aliases map[string][]string, attachments []string) *noteLookup {
	l := &noteLookup{notes: newNoteLookupSet(notes, ".md"), atts: newNoteLookupSet(attachments, ""), byAlias: map[string][]int{}}
	paths := make([]string, 0, len(aliases))
	for p := range aliases {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		idx, ok := l.notes.byLower[strings.ToLower(strings.ReplaceAll(p, "\\", "/"))]
		if !ok {
			continue
		}
		for _, alias := range aliases[p] {
			if key := strings.ToLower(strings.TrimSpace(alias)); key != "" {
				l.byAlias[key] = append(l.byAlias[key], idx)
			}
		}
	}
	return l
}

func normalizeWikiTarget(target string) string {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	target = strings.TrimPrefix(target, "./")
	return strings.TrimLeft(target, "/")
}

// Resolve returns the workspace path a link points to, or "" when it does not resolve.
// kind is "wiki" (value = inner text) or "markdown" (value = relative URL).
func (l *noteLookup) Resolve(from, kind, value string, embed bool) string {
	from = strings.ReplaceAll(from, "\\", "/")
	if kind == "markdown" {
		return l.resolveMarkdown(from, value)
	}
	target := normalizeWikiTarget(ParseWikiValue(value).Target)
	if target == "" {
		return from
	}
	ext := strings.ToLower(path.Ext(target))
	if ext == ".md" {
		return l.findNote(from, target[:len(target)-3])
	}
	if ext != "" {
		if p := l.atts.find(from, target); p != "" {
			return p
		}
	}
	return l.findNote(from, target)
}

func (l *noteLookup) findNote(from, name string) string {
	if p := l.notes.find(from, name); p != "" {
		return p
	}
	return l.notes.pick(l.byAlias[strings.ToLower(name)])
}

func (l *noteLookup) resolveMarkdown(from, value string) string {
	if noteURLScheme.MatchString(value) {
		return ""
	}
	if i := strings.IndexAny(value, "#?"); i >= 0 {
		value = value[:i]
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || decoded == "" {
		return ""
	}
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if strings.HasPrefix(decoded, "/") {
		return ""
	}
	lower := strings.ToLower(path.Join(path.Dir(from), decoded))
	if !strings.HasPrefix(lower, notesRootLower) {
		return ""
	}
	if i, ok := l.notes.byLower[lower]; ok {
		return l.notes.entries[i].path
	}
	if i, ok := l.atts.byLower[lower]; ok {
		return l.atts.entries[i].path
	}
	return ""
}

// lexicalNoteTarget is the workspace path a relative Markdown URL names as seen from note "from", whether or not the
// file exists ("" for URLs that leave Documents/Notes or cannot be decoded).
func lexicalNoteTarget(from, value string) string {
	if i := strings.IndexAny(value, "#?"); i >= 0 {
		value = value[:i]
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || decoded == "" {
		return ""
	}
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if strings.HasPrefix(decoded, "/") {
		return ""
	}
	joined := path.Join(path.Dir(strings.ReplaceAll(from, "\\", "/")), decoded)
	if !strings.HasPrefix(strings.ToLower(joined), notesRootLower) {
		return ""
	}
	return joined
}

// LinkText is the shortest unambiguous wikilink text for a note or attachment path:
// the basename (without .md for notes) if unique, else the Notes-relative path.
func (l *noteLookup) LinkText(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	set, suffix := l.notes, ".md"
	if !strings.EqualFold(path.Ext(p), ".md") {
		set, suffix = l.atts, ""
	}
	name := path.Base(p)
	rel := p
	if len(rel) >= len(notesRootLower) && strings.EqualFold(rel[:len(notesRootLower)], notesRootLower) {
		rel = rel[len(notesRootLower):]
	}
	if suffix != "" {
		name = name[:len(name)-len(suffix)]
		rel = rel[:len(rel)-len(suffix)]
	}
	if len(set.byBase[strings.ToLower(name)]) <= 1 {
		return name
	}
	return rel
}
