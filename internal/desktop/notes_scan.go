package desktop

import (
	"context"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// The scanner mirrors what the Notes editor (micromark: scripts/notes-obsidian-markdown.js) treats as a link:
// fenced and indented code, HTML blocks, comments and code spans (also across the lines of one paragraph)
// are never scanned, including inside block quotes and list items. ui/js/desktop/apps/notes-index.js
// mirrors this file step by step; ui/testdata/notes-links-fixture.json keeps both honest.

const (
	// maxNoteLinkURL bounds a Markdown link destination; longer ones are not links.
	maxNoteLinkURL = 2048
	// noteScanTick is how many scanner steps pass between context checks.
	noteScanTick = 1 << 14

	leafNone  = 0
	leafFence = 1
	leafHTML  = 2

	htmlUntilBlank   = 1
	htmlUntilComment = 2
	htmlUntilRaw     = 3
	htmlUntilPI      = 4
)

var (
	noteHTMLBlockTags = map[string]bool{}
	noteHTMLRawTags   = map[string]bool{"script": true, "pre": true, "style": true, "textarea": true}
)

func init() {
	for _, name := range strings.Fields("address article aside base basefont blockquote body caption center col colgroup dd details dialog dir div dl dt fieldset figcaption figure footer form frame frameset h1 h2 h3 h4 h5 h6 head header hr html iframe legend li link main menu menuitem nav noframes ol optgroup option p param search section summary table tbody td tfoot th thead title tr track ul") {
		noteHTMLBlockTags[name] = true
	}
}

type noteContainerOp struct {
	quote  bool
	indent int // list item content columns
}

type noteLeaf struct {
	kind   int
	char   byte
	length int
	end    int
	rawTag string
	ops    []noteContainerOp
}

type noteParagraph struct {
	text   strings.Builder
	starts []int
	origs  []int
	quote  int
}

func (p *noteParagraph) open() bool { return len(p.starts) > 0 }

func (p *noteParagraph) add(piece string, orig int) {
	if len(p.starts) > 0 {
		p.text.WriteByte('\n')
	}
	p.starts = append(p.starts, p.text.Len())
	p.origs = append(p.origs, orig)
	p.text.WriteString(piece)
}

func (p *noteParagraph) reset() {
	p.text.Reset()
	p.starts, p.origs = p.starts[:0], p.origs[:0]
}

// spanFinder finds the first byte of a set at or after a start, remembering the last answer so that
// repeated searches over one line stay linear.
type spanFinder struct {
	set            string
	valid          bool
	end, from, pos int
}

func (f *spanFinder) find(s string, start, end int) int {
	if start >= end {
		return -1
	}
	if f.valid && f.end == end && start >= f.from {
		if f.pos < 0 {
			return -1
		}
		if start <= f.pos {
			return f.pos
		}
	}
	idx := -1
	if k := strings.IndexAny(s[start:end], f.set); k >= 0 {
		idx = start + k
	}
	f.valid, f.end, f.from, f.pos = true, end, start, idx
	return idx
}

type noteScanner struct {
	ctx    context.Context
	refs   *NoteRefs
	addTag func(string)
	steps  int
	err    error
}

func (sc *noteScanner) tick() bool {
	sc.steps++
	if sc.steps%noteScanTick == 0 && sc.err == nil {
		sc.err = sc.ctx.Err()
	}
	return sc.err != nil
}

func noteBacktickRuns(s string) map[int][]int {
	runs := map[int][]int{}
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		runs[j-i] = append(runs[j-i], i)
		i = j
	}
	return runs
}

// scanParagraph finds links and tags in one paragraph (pieces joined by "\n"; starts/origs map them back).
func (sc *noteScanner) scanParagraph(para string, starts, origs []int) {
	n := len(para)
	orig := func(idx int) int {
		k := sort.Search(len(starts), func(j int) bool { return starts[j] > idx }) - 1
		return origs[k] + idx - starts[k]
	}
	var brackets []int
	var runs map[int][]int
	term := spanFinder{set: " \t)"}
	paren := spanFinder{set: ")"}
	gt := spanFinder{set: ">"}
	noCommentClose := false
	lineEnd, lineFrom := -1, 0
	eol := func(from int) int {
		if from >= lineFrom && from <= lineEnd {
			return lineEnd
		}
		if k := strings.IndexByte(para[from:], '\n'); k >= 0 {
			lineEnd = from + k
		} else {
			lineEnd = n
		}
		lineFrom = from
		return lineEnd
	}
	wikiSpan := func(open int) (vs, ve int, ok bool) {
		vs = open + 2
		for j := vs; j < n; {
			if j-vs > maxWikiValueRunes*4 {
				return 0, 0, false
			}
			switch c := para[j]; {
			case c == '\n' || c == '[':
				return 0, 0, false
			case c == '\\':
				if j+1 >= n || para[j+1] == '\n' {
					return 0, 0, false
				}
				j += 2
				continue
			case c == ']':
				if j+1 >= n || para[j+1] != ']' {
					return 0, 0, false
				}
				value := para[vs:j]
				cnt := utf8.RuneCountInString(value)
				return vs, j, cnt > 0 && cnt <= maxWikiValueRunes && strings.Trim(value, " \t") != ""
			}
			j++
		}
		return 0, 0, false
	}
	linkURL := func(start int) (vs, ve, after int, ok bool) {
		end := eol(start)
		var closeFrom int
		if start < end && para[start] == '<' {
			k := gt.find(para, start+1, end)
			if k < 0 {
				return 0, 0, 0, false
			}
			vs, ve, closeFrom = start+1, k, k+1
		} else {
			ve = term.find(para, start, end)
			if ve < 0 {
				ve = end
			}
			vs, closeFrom = start, ve
		}
		if ve-vs > maxNoteLinkURL {
			return 0, 0, 0, false
		}
		closeAt := paren.find(para, closeFrom, end)
		if closeAt < 0 {
			return 0, 0, 0, false
		}
		return vs, ve, closeAt + 1, true
	}

	for i := 0; i < n; {
		if sc.tick() {
			return
		}
		c := para[i]
		switch {
		case c == '\\':
			i += 2
		case c == '`':
			cnt := 0
			for i+cnt < n && para[i+cnt] == '`' {
				cnt++
			}
			if runs == nil {
				runs = noteBacktickRuns(para)
			}
			list := runs[cnt]
			if k := sort.SearchInts(list, i+cnt); k < len(list) {
				i = list[k] + cnt
			} else {
				i += cnt
			}
		case c == '<' && strings.HasPrefix(para[i:], "<!--"):
			if !noCommentClose {
				if j := strings.Index(para[i+4:], "-->"); j >= 0 {
					i += 4 + j + 3
					break
				}
				noCommentClose = true
			}
			i++
		case c == '[' && strings.HasPrefix(para[i:], "[["), c == '!' && strings.HasPrefix(para[i:], "![["):
			embed := c == '!'
			open := i
			if embed {
				open = i + 1
			}
			if vs, ve, ok := wikiSpan(open); ok {
				start := orig(vs)
				sc.refs.Links = append(sc.refs.Links, NoteRefLink{Kind: "wiki", Value: para[vs:ve], Embed: embed, Start: start, End: start + ve - vs})
				i = ve + 2
			} else {
				brackets = append(brackets, open)
				i = open + 1
			}
		case c == '[':
			brackets = append(brackets, i)
			i++
		case c == ']':
			open := -1
			if len(brackets) > 0 {
				open = brackets[len(brackets)-1]
				brackets = brackets[:len(brackets)-1]
			}
			if i+1 >= n || para[i+1] != '(' {
				i++
				break
			}
			vs, ve, after, ok := linkURL(i + 2)
			if !ok {
				i += 2
				break
			}
			if url := para[vs:ve]; noteLinkURLOK(url) {
				start := orig(vs)
				sc.refs.Links = append(sc.refs.Links, NoteRefLink{Kind: "markdown", Value: url, Embed: open > 0 && para[open-1] == '!', Start: start, End: start + ve - vs})
			}
			i = after
		case c == '#':
			prevOK := i == 0
			if !prevOK {
				r, _ := utf8.DecodeLastRuneInString(para[:i])
				prevOK = unicode.IsSpace(r) || r == '('
			}
			j := i + 1
			if prevOK {
				for j < n {
					r, size := utf8.DecodeRuneInString(para[j:])
					if !isNoteTagRune(r) {
						break
					}
					j += size
				}
				if value := para[i+1 : j]; value != "" && strings.IndexFunc(value, func(r rune) bool { return !unicode.IsDigit(r) }) >= 0 {
					sc.addTag(value)
				}
			}
			i = j
		default:
			i++
		}
	}
}

// ---- block structure ----

// noteIndent returns the columns and bytes of the leading spaces and tabs.
func noteIndent(s string) (cols, bytes int) {
	for bytes < len(s) {
		switch s[bytes] {
		case ' ':
			cols++
		case '\t':
			cols += 4 - cols%4
		default:
			return
		}
		bytes++
	}
	return
}

// noteStripCols returns the bytes of s that make up the first n columns of indentation.
func noteStripCols(s string, n int) int {
	cols, bytes := 0, 0
	for bytes < len(s) && cols < n {
		switch s[bytes] {
		case ' ':
			cols++
		case '\t':
			cols += 4 - cols%4
		default:
			return bytes
		}
		bytes++
	}
	return bytes
}

// stripNoteQuote skips one block quote marker (up to 3 spaces, ">", one optional space) at off.
func stripNoteQuote(s string, off int) (int, bool) {
	cols, nb := noteIndent(s[off:])
	if cols >= 4 || off+nb >= len(s) || s[off+nb] != '>' {
		return off, false
	}
	off += nb + 1
	if off < len(s) && (s[off] == ' ' || s[off] == '\t') {
		off++
	}
	return off, true
}

// noteListMarker detects a list item marker at the start of s (after at most 3 columns of indentation).
// It returns the columns the item content is indented by and the bytes up to the content.
func noteListMarker(s string) (width, bytes int, ok bool) {
	cols, nb := noteIndent(s)
	if cols >= 4 || nb >= len(s) {
		return 0, 0, false
	}
	p := nb
	switch c := s[p]; {
	case c == '-' || c == '*' || c == '+':
		p++
	case c >= '0' && c <= '9':
		for p < len(s) && p-nb < 9 && s[p] >= '0' && s[p] <= '9' {
			p++
		}
		if p >= len(s) || (s[p] != '.' && s[p] != ')') {
			return 0, 0, false
		}
		p++
	default:
		return 0, 0, false
	}
	if p < len(s) && s[p] != ' ' && s[p] != '\t' {
		return 0, 0, false
	}
	markerCols := cols + (p - nb)
	spaceCols, spaceBytes := noteIndent(s[p:])
	if p+spaceBytes >= len(s) {
		return markerCols + 1, p + spaceBytes, true
	}
	if spaceCols >= 5 {
		spaceCols, spaceBytes = 1, 1
	}
	return markerCols + spaceCols, p + spaceBytes, true
}

// continueNoteContainers strips the container prefixes a continuing fence or HTML block must repeat.
func continueNoteContainers(line string, ops []noteContainerOp) (int, bool) {
	off := 0
	for idx, op := range ops {
		if op.quote {
			next, ok := stripNoteQuote(line, off)
			if !ok {
				return 0, false
			}
			off = next
			continue
		}
		rest := line[off:]
		if strings.TrimSpace(rest) == "" {
			for _, later := range ops[idx+1:] {
				if later.quote {
					return 0, false
				}
			}
			return len(line), true
		}
		if cols, _ := noteIndent(rest); cols < op.indent {
			return 0, false
		}
		off += noteStripCols(rest, op.indent)
	}
	return off, true
}

func noteFenceOpener(t string) (char byte, count int, ok bool) {
	if len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return 0, 0, false
	}
	for count < len(t) && t[count] == t[0] {
		count++
	}
	if count < 3 || (t[0] == '`' && strings.ContainsRune(t[count:], '`')) {
		return 0, 0, false
	}
	return t[0], count, true
}

// noteHTMLStart recognises the start of an HTML block (CommonMark kinds 1, 2, 3 and 6).
func noteHTMLStart(t string) (end int, raw string, ok bool) {
	if len(t) < 2 || t[0] != '<' {
		return 0, "", false
	}
	if strings.HasPrefix(t, "<!--") {
		return htmlUntilComment, "", true
	}
	if t[1] == '?' {
		return htmlUntilPI, "", true
	}
	i := 1
	if t[i] == '/' {
		i++
	}
	j := i
	for j < len(t) && (t[j] >= 'a' && t[j] <= 'z' || t[j] >= 'A' && t[j] <= 'Z' || t[j] >= '0' && t[j] <= '9') {
		j++
	}
	if j == i {
		return 0, "", false
	}
	name := strings.ToLower(t[i:j])
	endsName := j >= len(t) || t[j] == ' ' || t[j] == '\t' || t[j] == '>'
	if noteHTMLRawTags[name] && i == 1 && endsName {
		return htmlUntilRaw, name, true
	}
	if noteHTMLBlockTags[name] && (endsName || strings.HasPrefix(t[j:], "/>")) {
		return htmlUntilBlank, "", true
	}
	return 0, "", false
}

// noteHTMLCloses reports whether s holds the end condition of an HTML block (not for blank-terminated ones).
func noteHTMLCloses(end int, raw, s string) bool {
	switch end {
	case htmlUntilComment:
		return strings.Contains(s, "-->")
	case htmlUntilPI:
		return strings.Contains(s, "?>")
	case htmlUntilRaw:
		return strings.Contains(strings.ToLower(s), "</"+raw+">")
	}
	return false
}

func noteATXHeading(t string) bool {
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	return n >= 1 && n <= 6 && (n == len(t) || t[n] == ' ' || t[n] == '\t')
}

// ExtractNoteRefs returns title, aliases, tags and links of a Markdown note.
// Frontmatter, code (fenced, indented, spans), HTML blocks and comments are not scanned for inline syntax.
func ExtractNoteRefs(content string) NoteRefs {
	refs, _ := ExtractNoteRefsContext(context.Background(), content)
	return refs
}

// ExtractNoteRefsContext is ExtractNoteRefs with cancellation; on cancel it returns what was found so far and the error.
func ExtractNoteRefsContext(ctx context.Context, content string) (NoteRefs, error) {
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

	sc := &noteScanner{ctx: ctx, refs: &refs, addTag: addTag}
	heading, firstLine := "", ""
	var leaf noteLeaf
	var para noteParagraph
	listCtx := 0
	flush := func() {
		if para.open() {
			sc.scanParagraph(para.text.String(), para.starts, para.origs)
			para.reset()
		}
	}
	lines := 0
	for pos := bodyStart; pos < len(content) && sc.err == nil; {
		if lines++; lines%256 == 0 {
			sc.err = ctx.Err()
		}
		lineEnd, next := len(content), len(content)
		if end := strings.IndexByte(content[pos:], '\n'); end >= 0 {
			lineEnd, next = pos+end, pos+end+1
		}
		base := pos
		pos = next
		trimmed := strings.TrimRight(content[base:lineEnd], "\r")

		if leaf.kind != leafNone {
			if off, ok := continueNoteContainers(trimmed, leaf.ops); ok {
				rest := trimmed[off:]
				switch {
				case leaf.kind == leafFence:
					if t := strings.TrimSpace(rest); len(t) >= leaf.length && strings.Trim(t, string(leaf.char)) == "" {
						leaf = noteLeaf{}
					}
				case leaf.end == htmlUntilBlank:
					if strings.TrimSpace(rest) == "" {
						leaf = noteLeaf{}
					}
				case noteHTMLCloses(leaf.end, leaf.rawTag, rest):
					leaf = noteLeaf{}
				}
				continue
			}
			leaf = noteLeaf{}
		}

		off := 0
		var ops []noteContainerOp
		quoteDepth := 0
		for {
			next, ok := stripNoteQuote(trimmed, off)
			if !ok {
				break
			}
			off = next
			ops = append(ops, noteContainerOp{quote: true})
			quoteDepth++
		}
		consumed := 0
		if rest := trimmed[off:]; listCtx > 0 && strings.TrimSpace(rest) != "" {
			if cols, _ := noteIndent(rest); cols >= listCtx {
				off += noteStripCols(rest, listCtx)
				ops = append(ops, noteContainerOp{indent: listCtx})
				consumed = listCtx
			} else if _, _, isItem := noteListMarker(rest); !isItem && !para.open() {
				listCtx = 0
			}
		}
		newItem := false
		for guard := 0; guard < 8; guard++ {
			rest := trimmed[off:]
			cols, nb := noteIndent(rest)
			if cols >= 4 {
				break
			}
			if nb < len(rest) && rest[nb] == '>' {
				next, _ := stripNoteQuote(trimmed, off)
				off = next
				ops = append(ops, noteContainerOp{quote: true})
				quoteDepth++
				continue
			}
			if width, bytes, ok := noteListMarker(rest); ok {
				off += bytes
				consumed += width
				ops = append(ops, noteContainerOp{indent: width})
				listCtx = consumed
				newItem = true
				continue
			}
			break
		}
		if newItem || (quoteDepth > 0 && quoteDepth != para.quote) {
			flush()
		}
		rest := trimmed[off:]
		cols, nb := noteIndent(rest)
		if nb >= len(rest) {
			flush()
			continue
		}
		t := rest[nb:]
		if cols < 4 {
			if char, count, ok := noteFenceOpener(t); ok {
				flush()
				leaf = noteLeaf{kind: leafFence, char: char, length: count, ops: ops}
				continue
			}
		}
		if heading == "" && strings.HasPrefix(trimmed, "# ") {
			heading = strings.TrimSpace(trimmed[2:])
		}
		if firstLine == "" {
			firstLine = strings.TrimSpace(trimmed)
		}
		if cols >= 4 && !para.open() {
			continue
		}
		if cols < 4 {
			if end, raw, ok := noteHTMLStart(t); ok {
				flush()
				closed := false
				switch end {
				case htmlUntilComment:
					closed = noteHTMLCloses(end, raw, t[4:])
				case htmlUntilPI:
					closed = noteHTMLCloses(end, raw, t[2:])
				case htmlUntilRaw:
					closed = noteHTMLCloses(end, raw, t)
				}
				if !closed {
					leaf = noteLeaf{kind: leafHTML, end: end, rawTag: raw, ops: ops}
				}
				continue
			}
			if noteATXHeading(t) || t[0] == '|' {
				flush()
				para.quote = quoteDepth
				para.add(rest, base+off)
				flush()
				continue
			}
		}
		if !para.open() {
			para.quote = quoteDepth
		}
		para.add(rest, base+off)
	}
	flush()

	switch {
	case fmTitle != "":
		refs.Title, refs.TitleSource = fmTitle, "frontmatter"
	case heading != "":
		refs.Title, refs.TitleSource = heading, "heading"
	case firstLine != "":
		refs.Title, refs.TitleSource = firstLine, "body"
	}
	refs.Title = stringLimit(refs.Title, 160)
	return refs, sc.err
}
