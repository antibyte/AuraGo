package whatsapp

import (
	"regexp"
	"strings"
	"unicode"
)

// WhatsApp formatting: *bold*, _italic_, ~strike~, ``` fences and "- " bullets. It
// has no headings, tables or links, so the agent's Markdown is converted line by
// line. The scanner is hand-written: Go regexp has no lookbehind, and a consumed
// boundary rune would break adjacent matches.

const (
	// maxLinkTextCells and maxLinkURLCells bound one link match, so a line full of
	// unmatched brackets costs a bounded scan per bracket.
	maxLinkTextCells = 1000
	maxLinkURLCells  = 2048
	// tableBullet and tableJoiner shape one table row: "• a — b".
	tableBullet = "• "
	tableJoiner = " — "
)

var (
	// headingRe: 1-6 hashes, a space, the text, and an optional closing run of
	// hashes that is separated from the text by a space ("# C#" keeps its "#").
	headingRe = regexp.MustCompile(`^#{1,6}\s+(.*?)(?:\s+#+)?\s*$`)
	// tableSeparatorCellRe is one cell of the "|---|:--:|" row.
	tableSeparatorCellRe = regexp.MustCompile(`^:?-+:?$`)
)

// toWhatsApp converts the agent's Markdown to WhatsApp formatting. Code spans and
// fenced blocks pass through byte for byte.
func toWhatsApp(markdown string) string {
	if markdown == "" {
		return ""
	}
	lines := strings.Split(markdown, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trimmed, "|") {
			end := i
			for end < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[end]), "|") {
				end++
			}
			if end-i >= 2 && isTableSeparator(lines[i+1]) {
				for r := i; r < end; r++ {
					if r == i+1 {
						continue
					}
					if row := tableRow(lines[r]); row != "" {
						out = append(out, row)
					}
				}
			} else {
				for r := i; r < end; r++ {
					out = append(out, convertLine(lines[r]))
				}
			}
			i = end - 1
			continue
		}
		out = append(out, convertLine(line))
	}
	return strings.Join(out, "\n")
}

// convertLine converts one line outside fences and tables. A trailing carriage
// return stays where it is.
func convertLine(line string) string {
	cr := ""
	if strings.HasSuffix(line, "\r") {
		line, cr = line[:len(line)-1], "\r"
	}
	return convertLineBody(line) + cr
}

func convertLineBody(line string) string {
	if m := headingRe.FindStringSubmatch(line); m != nil {
		if text := trimWrapping(strings.TrimSpace(convertInline(m[1]))); text != "" {
			return "*" + text + "*"
		}
		return line
	}
	rest := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(rest)]
	if strings.HasPrefix(rest, "* ") || strings.HasPrefix(rest, "+ ") {
		return indent + "- " + convertInline(rest[2:])
	}
	return convertInline(line)
}

// trimWrapping strips emphasis markers that wrap the whole heading text, so a
// converted heading carries one pair of asterisks. Markers on one side only stay,
// for example the closing underscore of "Use _init_".
func trimWrapping(s string) string {
	for len(s) >= 2 && (s[0] == '*' || s[0] == '_') && s[len(s)-1] == s[0] {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// splitTableCells splits one table row on unescaped pipes. The text before the
// first pipe and after the last one is not a cell; empty cells are returned empty.
func splitTableCells(line string) []string {
	rs := []rune(strings.TrimSpace(line))
	if len(rs) > 0 && rs[0] == '|' {
		rs = rs[1:]
	}
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs) && rs[i+1] == '|':
			cur.WriteRune('|')
			i++
		case rs[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(rs[i])
		}
	}
	return append(cells, cur.String())
}

// isTableSeparator reports whether line is a "|---|:--:|" row.
func isTableSeparator(line string) bool {
	if !strings.HasPrefix(strings.TrimSpace(line), "|") {
		return false
	}
	found := false
	for _, cell := range splitTableCells(line) {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		if !tableSeparatorCellRe.MatchString(cell) {
			return false
		}
		found = true
	}
	return found
}

// tableRow renders one table row as "• a — b". A row without a non-empty cell
// renders as nothing.
func tableRow(line string) string {
	var parts []string
	for _, cell := range splitTableCells(line) {
		if cell = strings.TrimSpace(cell); cell != "" {
			parts = append(parts, convertInline(cell))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return tableBullet + strings.Join(parts, tableJoiner)
}

// inlineCell is one rune of a line under conversion.
type inlineCell struct {
	r    rune
	lock bool // inside a code span, URL or escape: never a delimiter
	kind byte // cellPlain, cellMark (final output rune) or cellDrop (omitted)
}

const (
	cellPlain byte = iota
	cellMark
	cellDrop
)

// convertInline converts bold, italic, strikethrough and links in one line. Code
// spans, bare URLs and backslash escapes pass through unchanged.
func convertInline(s string) string {
	if !strings.ContainsAny(s, "*_~[`") {
		return s
	}
	cells := lockCodeAndEscapes([]rune(s))
	cells = rewriteLinks(cells)
	lockBareURLs(cells)
	convertDoubleDelimiters(cells)
	cells = compactCells(cells)
	convertSingleStars(cells)
	out := make([]rune, len(cells))
	for i, c := range cells {
		out[i] = c.r
	}
	return string(out)
}

// escapable lists the runes a backslash protects.
const escapable = "\\`*_~[]()!#|"

// lockCodeAndEscapes turns runes into cells and locks backtick code spans (a run
// of N backticks up to the next run of exactly N, all of it) and backslash escapes.
// Backticks without a closing run stay plain text.
func lockCodeAndEscapes(rs []rune) []inlineCell {
	cells := make([]inlineCell, 0, len(rs))
	for i := 0; i < len(rs); {
		switch {
		case rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune(escapable, rs[i+1]):
			cells = append(cells, inlineCell{r: rs[i], lock: true}, inlineCell{r: rs[i+1], lock: true})
			i += 2
		case rs[i] == '`':
			n := backtickRun(rs, i)
			if end := closingBackticks(rs, i+n, n); end >= 0 {
				for ; i < end; i++ {
					cells = append(cells, inlineCell{r: rs[i], lock: true})
				}
				continue
			}
			for end := i + n; i < end; i++ {
				cells = append(cells, inlineCell{r: rs[i]})
			}
		default:
			cells = append(cells, inlineCell{r: rs[i]})
			i++
		}
	}
	return cells
}

func backtickRun(rs []rune, i int) int {
	n := 0
	for i+n < len(rs) && rs[i+n] == '`' {
		n++
	}
	return n
}

// closingBackticks returns the index after the first run of exactly n backticks
// at or after from, or -1.
func closingBackticks(rs []rune, from, n int) int {
	for j := from; j < len(rs); {
		if rs[j] != '`' {
			j++
			continue
		}
		run := backtickRun(rs, j)
		if run == n {
			return j + run
		}
		j += run
	}
	return -1
}

// rewriteLinks turns [text](url) into "text (url)", or just the url when text
// equals it. A leftover ![alt](url) is treated the same way.
func rewriteLinks(cells []inlineCell) []inlineCell {
	if !hasPlainRune(cells, '[') {
		return cells
	}
	out := make([]inlineCell, 0, len(cells))
	for i := 0; i < len(cells); {
		open := i
		if isPlain(cells[i], '!') && i+1 < len(cells) && isPlain(cells[i+1], '[') {
			open = i + 1
		}
		if isPlain(cells[open], '[') {
			if textEnd, urlEnd, ok := matchLink(cells, open); ok {
				text := cells[open+1 : textEnd]
				url := cells[textEnd+2 : urlEnd]
				showText := len(text) > 0 && !sameRunes(text, url)
				if showText {
					out = append(out, text...)
					out = append(out, inlineCell{r: ' '}, inlineCell{r: '(', lock: true})
				}
				for _, c := range url {
					out = append(out, inlineCell{r: c.r, lock: true})
				}
				if showText {
					out = append(out, inlineCell{r: ')', lock: true})
				}
				i = urlEnd + 1
				continue
			}
		}
		out = append(out, cells[i])
		i++
	}
	return out
}

// matchLink matches "[text](url)" with the bracket at cells[open]. It returns the
// index of the closing bracket and of the closing parenthesis. The URL must be
// non-empty and free of whitespace.
func matchLink(cells []inlineCell, open int) (textEnd, urlEnd int, ok bool) {
	depth := 0
	textEnd = -1
	for j := open + 1; j < len(cells) && j-open <= maxLinkTextCells; j++ {
		switch {
		case isPlain(cells[j], '['):
			depth++
		case isPlain(cells[j], ']'):
			if depth == 0 {
				textEnd = j
			} else {
				depth--
			}
		}
		if textEnd >= 0 {
			break
		}
	}
	if textEnd < 0 || textEnd+1 >= len(cells) || !isPlain(cells[textEnd+1], '(') {
		return 0, 0, false
	}
	depth = 0
	for j := textEnd + 2; j < len(cells) && j-textEnd <= maxLinkURLCells; j++ {
		if unicode.IsSpace(cells[j].r) {
			return 0, 0, false
		}
		switch {
		case isPlain(cells[j], '('):
			depth++
		case isPlain(cells[j], ')'):
			if depth == 0 {
				return textEnd, j, j > textEnd+2
			}
			depth--
		}
	}
	return 0, 0, false
}

func isPlain(c inlineCell, r rune) bool { return !c.lock && c.kind == cellPlain && c.r == r }

func hasPlainRune(cells []inlineCell, r rune) bool {
	for _, c := range cells {
		if isPlain(c, r) {
			return true
		}
	}
	return false
}

func sameRunes(a, b []inlineCell) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].r != b[i].r {
			return false
		}
	}
	return true
}

// lockBareURLs locks http(s) URLs so an underscore or asterisk inside a path is
// never read as emphasis. The URL ends at whitespace, an emphasis character, a
// quote, an angle bracket or a backtick; trailing punctuation and underscores stay
// outside.
func lockBareURLs(cells []inlineCell) {
	for i := 0; i < len(cells); i++ {
		if cells[i].lock || !startsURL(cells, i) {
			continue
		}
		end := i
		for end < len(cells) && !cells[end].lock && !unicode.IsSpace(cells[end].r) && !strings.ContainsRune("*~<>\"'`", cells[end].r) {
			end++
		}
		for end > i && strings.ContainsRune(".,;:!?)_", cells[end-1].r) {
			end--
		}
		for ; i < end; i++ {
			cells[i].lock = true
		}
		i--
	}
}

func startsURL(cells []inlineCell, i int) bool {
	if i > 0 && isWordCell(cells[i-1]) {
		return false
	}
	for _, scheme := range []string{"https://", "http://"} {
		if i+len(scheme) > len(cells) {
			continue
		}
		match := true
		for k, r := range scheme {
			if cells[i+k].r != r {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isSpaceCell(c inlineCell) bool { return unicode.IsSpace(c.r) }

func isWordCell(c inlineCell) bool {
	return unicode.IsLetter(c.r) || unicode.IsDigit(c.r) || c.r == '_'
}

// opener is a delimiter pair that may still find its closer.
type opener struct {
	idx  int
	kind int
}

// convertDoubleDelimiters matches "**x**", "__x__" and "~~x~~" with a stack and
// marks each matched pair as one output rune. "**" and "__" become "*", "~~"
// becomes "~". Unmatched delimiters stay as they are.
//
// A run of the same rune is judged as a whole, by the runes around it. An opening
// run gives its first two runes to the pair and a closing run its last two, so that
// the leftover rune of "***x***" sits inside the bold pair and becomes italic.
func convertDoubleDelimiters(cells []inlineCell) {
	const pairRunes = "*_~"
	var stack []opener
	var counts [3]int
	for i := 0; i < len(cells); {
		a := cells[i]
		kind := strings.IndexRune(pairRunes, a.r)
		if kind < 0 || a.lock || a.kind != cellPlain {
			i++
			continue
		}
		end := i + 1
		for end < len(cells) && cells[end].r == a.r && !cells[end].lock && cells[end].kind == cellPlain {
			end++
		}
		if end-i < 2 {
			i = end
			continue
		}
		canOpen := end < len(cells) && !isSpaceCell(cells[end])
		canClose := i > 0 && !isSpaceCell(cells[i-1])
		if a.r == '_' {
			canOpen = canOpen && (i == 0 || !isWordCell(cells[i-1]))
			canClose = canClose && (end >= len(cells) || !isWordCell(cells[end]))
		}
		switch {
		case canClose && counts[kind] > 0:
			var o opener
			for {
				o = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				counts[o.kind]--
				if o.kind == kind {
					break
				}
			}
			out := '*'
			if a.r == '~' {
				out = '~'
			}
			closer := end - 2
			cells[o.idx] = inlineCell{r: out, kind: cellMark}
			cells[o.idx+1].kind = cellDrop
			cells[closer] = inlineCell{r: out, kind: cellMark}
			cells[closer+1].kind = cellDrop
		case canOpen:
			stack = append(stack, opener{idx: i, kind: kind})
			counts[kind]++
		}
		i = end
	}
}

// compactCells drops the omitted halves of converted delimiter pairs.
func compactCells(cells []inlineCell) []inlineCell {
	out := cells[:0]
	for _, c := range cells {
		if c.kind != cellDrop {
			out = append(out, c)
		}
	}
	return out
}

// convertSingleStars turns a matched "*x*" into "_x_". The opener needs a
// non-word rune or the line start before it and a non-space after it; the closer
// needs a non-space before it and a non-word rune or the line end after it.
// "2*3*4" therefore stays as it is.
func convertSingleStars(cells []inlineCell) {
	var stack []int
	for i, c := range cells {
		if !isPlain(c, '*') {
			continue
		}
		canOpen := i+1 < len(cells) && !isSpaceCell(cells[i+1]) && (i == 0 || !isWordCell(cells[i-1]))
		canClose := i > 0 && !isSpaceCell(cells[i-1]) && (i+1 == len(cells) || !isWordCell(cells[i+1]))
		if canClose && len(stack) > 0 && stack[len(stack)-1]+1 != i {
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			cells[o] = inlineCell{r: '_', kind: cellMark}
			cells[i] = inlineCell{r: '_', kind: cellMark}
			continue
		}
		if canOpen {
			stack = append(stack, i)
		}
	}
}
