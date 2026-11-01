package whatsapp

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxChunkRunes is the longest text sent as one WhatsApp message. The sender
	// shortens it by the self-chat prefix.
	maxChunkRunes = 4000

	fenceMarker = "```"
	// fenceCloseRunes is the length of the "\n```" that closes a fence cut in two.
	fenceCloseRunes = len("\n" + fenceMarker)
	// minFenceChunkRunes is the smallest limit at which fences are tracked. A
	// smaller one cannot hold a reopened fence line, its close and a line of code.
	minFenceChunkRunes = 32

	// utf8Replacement is U+FFFD, written as bytes so the source needs no escape.
	utf8Replacement = "\xef\xbf\xbd"
)

// chunkText splits text into parts of at most limit runes. Leading blank lines and
// trailing whitespace are dropped; empty text gives no parts.
//
// A part is cut after a blank line outside a code fence when it has one, else at a
// line end. A single line longer than the room left is cut at the last space inside
// the window, else at a rune boundary. When a part ends inside a fence, it is closed
// with a fence line and the next part starts with the fence's opening line again,
// info string included.
func chunkText(text string, limit int) []string {
	// Invalid UTF-8 would count as one rune per byte and could reach WhatsApp as is.
	text = trimBlankEdges(strings.ToValidUTF8(text, utf8Replacement))
	if text == "" {
		return nil
	}
	if limit < 1 {
		limit = 1
	}
	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}
	c := &chunker{limit: limit, fences: limit >= minFenceChunkRunes}
	for _, line := range strings.Split(text, "\n") {
		c.addLine(line)
	}
	c.finish()
	return c.parts
}

// trimBlankEdges drops leading whitespace-only lines and all trailing whitespace.
func trimBlankEdges(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	for {
		nl := strings.IndexByte(s, '\n')
		if nl < 0 || strings.TrimSpace(s[:nl]) != "" {
			return s
		}
		s = s[nl+1:]
	}
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

func isFenceLine(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), fenceMarker) }

// chunkLine is one line of the part under construction.
type chunkLine struct {
	text    string
	runes   int
	open    string // opening line of the fence in effect after this line, "" outside a fence
	opener  bool   // this line opened that fence
	carried bool   // this line is a reopened fence line, not part of the text
}

// chunker builds the parts line by line. lines is the current part, runes its length
// when joined with newlines.
type chunker struct {
	limit  int
	fences bool
	lines  []chunkLine
	runes  int
	parts  []string
}

// openFence is the fence in effect at the end of the current part.
func (c *chunker) openFence() string {
	if len(c.lines) == 0 {
		return ""
	}
	return c.lines[len(c.lines)-1].open
}

// stateAfter returns the fence state once line is appended. A plain line leaves it
// unchanged; a fence line opens a fence or closes the open one.
func (c *chunker) stateAfter(line string, plain bool) (open string, opener bool) {
	cur := c.openFence()
	if plain {
		return cur, false
	}
	if cur == "" {
		return line, true
	}
	return "", false
}

func (c *chunker) push(l chunkLine) {
	if len(c.lines) > 0 {
		c.runes++
	}
	c.runes += l.runes
	c.lines = append(c.lines, l)
}

// addLine appends line to the current part, cutting the part first when the line
// does not fit.
func (c *chunker) addLine(line string) {
	n := utf8.RuneCountInString(line)
	// A fence line longer than half the limit is plain text: it could not be
	// reopened.
	plain := !c.fences || n > c.limit/2 || !isFenceLine(line)
	for {
		if len(c.lines) == 0 && isBlank(line) {
			return // a part never starts with a blank line
		}
		open, opener := c.stateAfter(line, plain)
		reserve, sep := 0, 0
		if open != "" {
			reserve = fenceCloseRunes
		}
		if len(c.lines) > 0 {
			sep = 1
		}
		if c.runes+sep+n+reserve <= c.limit {
			c.push(chunkLine{text: line, runes: n, open: open, opener: opener})
			return
		}
		if k, ok := c.cutPoint(); ok {
			c.emit(k)
			continue
		}
		if c.trimTrailingBlanks() {
			continue
		}
		c.splitLong([]rune(line))
		return
	}
}

// cutPoint returns how many lines of the current part to emit. It reports false
// when the part holds nothing worth emitting: it is empty, or it holds only the
// opening line of a fence.
func (c *chunker) cutPoint() (int, bool) {
	m := len(c.lines)
	last := -1
	for i := m - 1; i >= 0; i-- {
		if !isBlank(c.lines[i].text) {
			last = i
			break
		}
	}
	if last < 0 {
		return 0, false
	}
	// Paragraph: the latest blank line outside a fence that follows content.
	best, seen := 0, false
	for i, l := range c.lines {
		switch {
		case !isBlank(l.text):
			seen = true
		case seen && l.open == "":
			best = i + 1
		}
	}
	if best > 0 {
		return best, true
	}
	// A fence that holds no code yet is not cut in two: the cut goes before its
	// opening line, or nowhere when that line starts the part.
	if c.lines[m-1].open != "" && c.lines[last].opener {
		for j := 0; j < last; j++ {
			if !isBlank(c.lines[j].text) {
				return last, true
			}
		}
		return 0, false
	}
	return m, true
}

// emit ends the part after its first k lines. The remaining lines start the next
// part, behind the reopened fence line when the cut falls inside a fence.
func (c *chunker) emit(k int) {
	state := c.lines[k-1].open
	head := c.lines[:k]
	rest := append([]chunkLine(nil), c.lines[k:]...)
	for len(head) > 0 && isBlank(head[len(head)-1].text) {
		head = head[:len(head)-1]
	}
	texts := make([]string, len(head))
	for i, l := range head {
		texts[i] = l.text
	}
	body := strings.Join(texts, "\n")
	if state != "" {
		body += "\n" + fenceMarker
	}
	if !isBlank(body) {
		c.parts = append(c.parts, body)
	}
	c.lines, c.runes = nil, 0
	if state != "" {
		reopen := state
		if utf8.RuneCountInString(reopen) > c.limit/4 {
			reopen = fenceMarker
		}
		c.push(chunkLine{text: reopen, runes: utf8.RuneCountInString(reopen), open: reopen, opener: true, carried: true})
	}
	for _, l := range rest {
		if len(c.lines) == 0 && isBlank(l.text) {
			continue
		}
		c.push(l)
	}
}

// trimTrailingBlanks drops blank lines from the end of the current part and
// reports whether it dropped any. A blank line inside a fence is code and stays.
func (c *chunker) trimTrailingBlanks() bool {
	trimmed := false
	for len(c.lines) > 0 && isBlank(c.lines[len(c.lines)-1].text) && c.lines[len(c.lines)-1].open == "" {
		c.lines = c.lines[:len(c.lines)-1]
		trimmed = true
	}
	if trimmed {
		c.runes = 0
		for i, l := range c.lines {
			if i > 0 {
				c.runes++
			}
			c.runes += l.runes
		}
	}
	return trimmed
}

// splitLong places a line that does not fit even in a part without content. Each
// piece fills the room left in its part: it ends at the last space inside the
// window, else at a rune boundary. The line is plain text from here on.
func (c *chunker) splitLong(rs []rune) {
	for len(rs) > 0 {
		open := c.openFence()
		reserve, sep := 0, 0
		if open != "" {
			reserve = fenceCloseRunes
		}
		if len(c.lines) > 0 {
			sep = 1
		}
		avail := c.limit - c.runes - sep - reserve
		if avail < 1 {
			c.emit(len(c.lines)) // the part is one oversized fence line
			continue
		}
		first := 0
		for first < len(rs) && unicode.IsSpace(rs[first]) {
			first++
		}
		if first >= avail {
			rs = rs[first:] // an indentation that fills the window
			continue
		}
		if len(rs) <= avail {
			c.push(chunkLine{text: string(rs), runes: len(rs), open: open})
			return
		}
		end, next := avail, avail
		for p := avail; p > first; p-- {
			if unicode.IsSpace(rs[p]) {
				end, next = p, p
				break
			}
		}
		piece := strings.TrimRightFunc(string(rs[:end]), unicode.IsSpace)
		c.push(chunkLine{text: piece, runes: utf8.RuneCountInString(piece), open: open})
		c.emit(len(c.lines))
		for next < len(rs) && unicode.IsSpace(rs[next]) {
			next++
		}
		rs = rs[next:]
	}
}

// finish emits the last part. It adds no closing fence: when the text ends inside
// a fence, that was the author's. A last part that holds nothing but a reopened
// fence line and fence lines is dropped: the previous part already closed the block.
func (c *chunker) finish() {
	head := c.lines
	for len(head) > 0 && isBlank(head[len(head)-1].text) {
		head = head[:len(head)-1]
	}
	if len(head) == 0 {
		return
	}
	onlyFences := head[0].carried
	texts := make([]string, len(head))
	for i, l := range head {
		texts[i] = l.text
		if !isBlank(l.text) && !isFenceLine(l.text) {
			onlyFences = false
		}
	}
	if onlyFences {
		return
	}
	c.parts = append(c.parts, strings.Join(texts, "\n"))
}
