package newspaper

import (
	"bytes"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/text/encoding/charmap"
)

// Print-style A4 layout: a full-width masthead and lead headline on page 1,
// then a two-column flow for stories, sources and the colophon. All text is
// wrapped and drawn line by line by this file (gofpdf's MultiCell and Write
// drop CJK runes at wrap points).
const (
	pdfMargin   = 16.0
	pdfContentW = 178.0
	pdfGutter   = 7.0
	pdfColW     = 85.5
	pdfPageW    = 210.0
	pdfPageH    = 297.0
	pdfBottom   = 16.0
	pdfTrigger  = pdfPageH - pdfBottom
	pt          = 25.4 / 72
)

type rgb struct{ r, g, b int }

var (
	pdfInk    = rgb{27, 27, 25}
	pdfMuted  = rgb{85, 85, 78}
	pdfRule   = rgb{188, 183, 170}
	pdfAccent = rgb{139, 51, 56}
)

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
		(r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// noBreakBefore lists closing punctuation that must not start a line.
func noBreakBefore(r rune) bool {
	return strings.ContainsRune("。，、）」』】》〉〕］｝！？：；．，.,)!?:;ー〜ぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮヵヶ", r)
}

// wrapText splits text into lines. The first line may have its own width.
// Breaks happen after spaces, around CJK runes (never before closing
// punctuation), at newlines, and inside overlong tokens. Spaces at a break
// are consumed; no other rune is ever dropped.
func wrapText(text string, first, rest float64, measure func(string) float64) []string {
	var lines []string
	limit := func() float64 {
		if len(lines) == 0 {
			return first
		}
		return rest
	}
	for _, seg := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		r := []rune(seg)
		s, brk := 0, -1
		for i := 0; i < len(r); {
			if i > s {
				if r[i-1] == ' ' && r[i] != ' ' {
					brk = i
				} else if r[i] != ' ' && r[i-1] != ' ' && (isCJK(r[i-1]) || isCJK(r[i])) && !noBreakBefore(r[i]) {
					brk = i
				}
			}
			if i > s && r[i] != ' ' && measure(strings.TrimRight(string(r[s:i+1]), " ")) > limit() {
				end := i
				if brk > s {
					end = brk
				}
				lines = append(lines, strings.TrimRight(string(r[s:end]), " "))
				s, brk = end, -1
				i = s
				continue
			}
			i++
		}
		lines = append(lines, strings.TrimRight(string(r[s:]), " "))
	}
	return lines
}

type pdfDoc struct {
	p         *gofpdf.Fpdf
	e         Edition
	l         editionLabels
	flow      bool
	col       int
	colTop    float64
	col0      float64
	col1      bool
	lastStory bool
}

func (d *pdfDoc) text(c rgb)                      { d.p.SetTextColor(c.r, c.g, c.b) }
func (d *pdfDoc) draw(c rgb)                      { d.p.SetDrawColor(c.r, c.g, c.b) }
func (d *pdfDoc) fill(c rgb)                      { d.p.SetFillColor(c.r, c.g, c.b) }
func (d *pdfDoc) font(style string, size float64) { d.p.SetFont("News", style, size) }
func (d *pdfDoc) measure(s string) float64        { return d.p.GetStringWidth(s) }

func (d *pdfDoc) x0() float64 {
	if d.flow {
		return pdfMargin + float64(d.col)*(pdfColW+pdfGutter)
	}
	return pdfMargin
}

func (d *pdfDoc) w() float64 {
	if d.flow {
		return pdfColW
	}
	return pdfContentW
}

// bar draws a filled rule so thick rules stay crisp.
func (d *pdfDoc) bar(x, y, w, h float64, c rgb) {
	d.fill(c)
	d.p.Rect(x, y, w, h, "F")
}

func (d *pdfDoc) setCol(i int) {
	x := pdfMargin + float64(i)*(pdfColW+pdfGutter)
	d.col = i
	d.p.SetLeftMargin(x)
	d.p.SetRightMargin(pdfPageW - x - pdfColW)
	d.p.SetX(x)
}

func (d *pdfDoc) gutterRule(y1, y2 float64) {
	if y2 <= y1 {
		return
	}
	x := pdfMargin + pdfColW + pdfGutter/2
	d.draw(pdfRule)
	d.p.SetLineWidth(.15)
	d.p.Line(x, y1, x, y2)
}

// acceptBreak is only a safety net; ensure() handles every break itself.
func (d *pdfDoc) acceptBreak() bool {
	if !d.flow {
		return true
	}
	if d.col == 0 {
		d.col0 = d.p.GetY()
		d.col1 = true
		d.lastStory = false
		d.setCol(1)
		d.p.SetY(d.colTop)
		return false
	}
	d.gutterRule(d.colTop, pdfTrigger)
	d.col1 = false
	d.lastStory = false
	d.setCol(0)
	return true
}

// breakColumn moves to the next column or page on purpose.
func (d *pdfDoc) breakColumn() {
	d.lastStory = false
	if !d.flow {
		d.p.AddPage()
		return
	}
	if d.col == 0 {
		d.col0 = d.p.GetY()
		d.col1 = true
		d.setCol(1)
		d.p.SetY(d.colTop)
		return
	}
	d.gutterRule(d.colTop, pdfTrigger)
	d.col1 = false
	d.setCol(0)
	d.p.AddPage()
	d.setCol(0)
	d.p.SetY(d.colTop)
}

func (d *pdfDoc) atTop() bool {
	if !d.flow {
		return d.p.GetY() <= 16.01
	}
	return d.p.GetY() <= d.colTop+.01
}

// ensure breaks column or page when a block of height h does not fit.
func (d *pdfDoc) ensure(h float64) {
	if d.p.GetY()+h > pdfTrigger && !d.atTop() {
		d.breakColumn()
	}
}

func (d *pdfDoc) startFlow() {
	d.flow = true
	d.colTop = d.p.GetY() + 2
	d.setCol(0)
	d.p.SetY(d.colTop)
}

func (d *pdfDoc) endFlow() {
	if d.col1 {
		end := d.p.GetY()
		if d.col0 > end {
			end = d.col0
		}
		d.gutterRule(d.colTop, end)
	}
}

// wrap wraps text at the current font within the current column width.
func (d *pdfDoc) wrap(text string, w float64) []string {
	return wrapText(text, w, w, d.measure)
}

// drawLines draws wrapped lines top to bottom, breaking columns as needed.
func (d *pdfDoc) drawLines(lines []string, lh float64, align, link string) {
	p := d.p
	for _, ln := range lines {
		d.ensure(lh)
		w := d.w()
		if link != "" {
			w = d.measure(ln) + .2
		}
		p.SetXY(d.x0(), p.GetY())
		p.CellFormat(w, lh, ln, "", 0, align, false, 0, link)
		p.SetY(p.GetY() + lh)
	}
}

func (d *pdfDoc) para(text string, lh float64, align string) {
	d.drawLines(d.wrap(text, d.w()), lh, align, "")
}

// fit truncates a one-line string so it fits width w in the current font.
func (d *pdfDoc) fit(s string, w float64) string {
	if d.measure(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && d.measure(string(r)+"...") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "..."
}

func (d *pdfDoc) header() {
	p := d.p
	if p.PageNo() < 2 {
		return
	}
	d.font("", 7)
	d.text(pdfMuted)
	up := strings.ToUpper
	p.SetXY(pdfMargin, 8)
	p.CellFormat(62, 4, d.fit(up(d.e.Title), 62), "", 0, "L", false, 0, "")
	p.SetXY(pdfMargin+62, 8)
	p.CellFormat(54, 4, d.fit(up(longDate(d.e)), 54), "", 0, "C", false, 0, "")
	p.SetXY(pdfMargin+116, 8)
	p.CellFormat(62, 4, d.fit(up(editionTag(d.e)), 62), "", 0, "R", false, 0, "")
	d.draw(pdfInk)
	p.SetLineWidth(.2)
	p.Line(pdfMargin, 12.5, pdfMargin+pdfContentW, 12.5)
	d.text(pdfInk)
	p.SetY(16)
	d.colTop = 16
}

func (d *pdfDoc) footer() {
	p := d.p
	d.font("", 7.5)
	d.text(pdfMuted)
	p.SetXY(pdfMargin, pdfPageH-11)
	p.CellFormat(pdfContentW, 4, fmt.Sprintf("%s %d", d.l.Page, p.PageNo()), "", 0, "C", false, 0, "")
	d.text(pdfInk)
}

// cp1252 returns s as Windows-1252 bytes for the core fonts; ok is false when
// any rune cannot be encoded.
func cp1252(s string) (string, bool) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		b, ok := charmap.Windows1252.EncodeRune(r)
		if !ok || r < 0x20 {
			return "", false
		}
		out = append(out, b)
	}
	return string(out), true
}

// pdfExtraText lists every string drawn with the News fonts that Text(e) does
// not already contain, so font coverage is checked up front.
func pdfExtraText(e Edition) string {
	l := labels(e)
	up := strings.ToUpper
	parts := []string{
		l.Motto, l.Checked, articleCount(e, len(e.Stories)), up(sourceCount(e, len(e.Sources))), up(editionTag(e)),
		up(folioPlace(e)), up(longDate(e)), up(e.Title), up(l.Correction), up(l.About), up(l.Page), l.Page,
		l.Sources, colophon(e), "...", "0123456789",
	}
	for _, s := range e.Sources {
		parts = append(parts, sourceDates(l, s))
	}
	return strings.Join(parts, "\n")
}

func renderPDF(e Edition, fonts fs.FS) ([]byte, error) {
	regular, bold, err := pdfFonts(Text(e)+"\n"+pdfExtraText(e), fonts)
	if err != nil {
		return nil, err
	}
	d := &pdfDoc{e: e, l: labels(e)}
	p := gofpdf.New("P", "mm", "A4", "")
	d.p = p
	p.SetMargins(pdfMargin, 14, pdfMargin)
	p.SetCellMargin(0)
	p.SetAutoPageBreak(true, pdfBottom)
	p.AddUTF8FontFromBytes("News", "", regular)
	p.AddUTF8FontFromBytes("News", "B", bold)
	p.SetTitle(e.Title, true)
	p.SetAuthor("AuraGo Newspaper", false)
	p.SetHeaderFunc(d.header)
	p.SetFooterFunc(d.footer)
	p.SetAcceptPageBreakFunc(d.acceptBreak)
	p.AddPage()

	d.masthead()
	d.notices()

	stories := e.Stories
	previous := ""
	if len(stories) > 0 {
		lead := stories[0]
		d.sectionHead(lead.Section)
		previous = lead.Section
		d.font("B", 22)
		d.text(pdfInk)
		d.para(lead.Headline, 9, "L")
		p.SetY(p.GetY() + 1.5)
		if lead.Deck != "" {
			d.font("B", 11)
			d.para(lead.Deck, 5.2, "L")
			p.SetY(p.GetY() + 1.5)
		}
		d.singleSource(lead)
		d.startFlow()
		d.paragraphs(lead)
		d.lastStory = true
	} else {
		d.startFlow()
	}
	for i := 1; i < len(stories); i++ {
		s := stories[i]
		changed := s.Section != previous
		need := d.storyHeight(s)
		if changed {
			need += 6.5 + 4
			if !d.atTop() {
				need += 4
			}
		}
		if d.lastStory && !changed && !d.atTop() {
			need += 5.5
		}
		d.ensure(need)
		if changed {
			d.sectionHead(s.Section)
			previous = s.Section
		} else if d.lastStory && !d.atTop() {
			d.storyRule()
		}
		d.storyHead(s)
		d.paragraphs(s)
		d.lastStory = true
	}
	d.sources()
	d.colophonBlock()
	d.endFlow()

	var out bytes.Buffer
	if err = p.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (d *pdfDoc) masthead() {
	p, e := d.p, d.e
	const top, earW, gap = 14.0, 34.0, 5.0
	ears := true
	avail := pdfContentW - 2*(earW+gap)
	plate, core := cp1252(e.Title)
	setPlate := func(size float64) {
		if core {
			p.SetFont("Times", "B", size)
		} else {
			d.font("B", size)
		}
	}
	// measurePlate measures a title line in whichever font draws it.
	measurePlate := func(s string) float64 {
		if core {
			enc, _ := cp1252(s)
			return p.GetStringWidth(enc)
		}
		return p.GetStringWidth(s)
	}
	_ = plate
	size := 46.0
	fits := func(w float64) bool {
		for size = 46; size >= 24; size-- {
			setPlate(size)
			if measurePlate(e.Title) <= w {
				return true
			}
		}
		return false
	}
	wrap := false
	if !fits(avail) {
		ears = false
		fitted := false
		for size = 46; size >= 20; size-- {
			setPlate(size)
			if measurePlate(e.Title) <= pdfContentW {
				fitted = true
				break
			}
		}
		if !fitted {
			size, wrap = 20, true
		}
	}
	earH := 13.0
	type earBox struct {
		x      float64
		a, b   []string
		bh, sh float64
	}
	var boxes []earBox
	if ears {
		mk := func(x float64, a, b string) earBox {
			d.font("B", 9.5)
			la := wrapText(a, earW-3, earW-3, d.measure)
			d.font("", 6.5)
			lb := wrapText(strings.ToUpper(b), earW-3, earW-3, d.measure)
			return earBox{x: x, a: la, b: lb}
		}
		boxes = []earBox{
			mk(pdfMargin, articleCount(e, len(e.Stories)), sourceCount(e, len(e.Sources))),
			mk(pdfMargin+pdfContentW-earW, d.l.Checked, editionTag(e)),
		}
		for _, b := range boxes {
			if h := float64(len(b.a))*4.2 + float64(len(b.b))*3.2 + 5; h > earH {
				earH = h
			}
		}
		for _, b := range boxes {
			d.draw(pdfInk)
			p.SetLineWidth(.25)
			p.Rect(b.x, top, earW, earH, "D")
			content := float64(len(b.a))*4.2 + float64(len(b.b))*3.2
			y := top + (earH-content)/2
			d.font("B", 9.5)
			d.text(pdfInk)
			for _, ln := range b.a {
				p.SetXY(b.x, y)
				p.CellFormat(earW, 4.2, ln, "", 0, "C", false, 0, "")
				y += 4.2
			}
			d.font("", 6.5)
			d.text(pdfMuted)
			for _, ln := range b.b {
				p.SetXY(b.x, y)
				p.CellFormat(earW, 3.2, ln, "", 0, "C", false, 0, "")
				y += 3.2
			}
		}
	}
	setPlate(size)
	d.text(pdfInk)
	lineH := size * pt * 1.05
	x, w := pdfMargin, pdfContentW
	if ears {
		x, w = pdfMargin+earW+gap, avail
	}
	draw := func(s string) string {
		if core {
			enc, _ := cp1252(s)
			return enc
		}
		return s
	}
	y := top - (lineH-earH)/2
	if y < 11 {
		y = 11
	}
	if wrap {
		for _, ln := range wrapText(e.Title, w, w, measurePlate) {
			p.SetXY(x, y)
			p.CellFormat(w, lineH, draw(ln), "", 0, "C", false, 0, "")
			y += lineH
		}
	} else {
		p.SetXY(x, y)
		p.CellFormat(w, lineH, draw(e.Title), "", 0, "C", false, 0, "")
		y += lineH
	}
	if ears && y < top+earH {
		y = top + earH
	}
	y += 1.5

	// Motto in the core Times Italic font; upright News when not encodable.
	d.text(pdfMuted)
	p.SetXY(pdfMargin, y)
	if enc, ok := cp1252(d.l.Motto); ok {
		p.SetFont("Times", "I", 10.5)
		p.CellFormat(pdfContentW, 5, enc, "", 0, "C", false, 0, "")
	} else {
		d.font("", 10)
		p.CellFormat(pdfContentW, 5, d.l.Motto, "", 0, "C", false, 0, "")
	}
	y += 7.5

	// Folio row between two rules, then the heavy rule.
	d.bar(pdfMargin, y, pdfContentW, .2, pdfInk)
	d.font("B", 7)
	d.text(pdfMuted)
	up := strings.ToUpper
	p.SetXY(pdfMargin, y+.2)
	p.CellFormat(60, 5.6, d.fit(up(folioPlace(e)), 60), "", 0, "L", false, 0, "")
	p.SetXY(pdfMargin+60, y+.2)
	p.CellFormat(58, 5.6, d.fit(up(longDate(e)), 58), "", 0, "C", false, 0, "")
	p.SetXY(pdfMargin+118, y+.2)
	p.CellFormat(60, 5.6, d.fit(up(editionTag(e)), 60), "", 0, "R", false, 0, "")
	d.bar(pdfMargin, y+5.8, pdfContentW, .2, pdfInk)
	d.bar(pdfMargin, y+7, pdfContentW, .9, pdfInk)
	d.text(pdfInk)
	p.SetXY(pdfMargin, y+10)
}

// noticeBox draws a bordered notice with a heavy bar on the left.
func (d *pdfDoc) noticeBox(label, body string) {
	p := d.p
	const barW, pad, lh = 1.2, 3.0, 4.4
	innerX := pdfMargin + barW + pad
	innerW := pdfContentW - barW - 2*pad
	d.font("", 9.5)
	lines := wrapText(body, innerW, innerW, d.measure)
	h := 2*pad + float64(len(lines))*lh
	if label != "" {
		h += 4
	}
	if p.GetY()+h > pdfTrigger {
		p.AddPage()
	}
	y := p.GetY()
	d.draw(pdfInk)
	p.SetLineWidth(.25)
	p.Rect(pdfMargin, y, pdfContentW, h, "D")
	d.bar(pdfMargin, y, barW, h, pdfInk)
	cy := y + pad
	if label != "" {
		d.font("B", 7)
		d.text(pdfAccent)
		p.SetXY(innerX, cy)
		p.CellFormat(innerW, 3.6, strings.ToUpper(label), "", 0, "L", false, 0, "")
		cy += 4
	}
	d.font("", 9.5)
	d.text(pdfInk)
	for _, ln := range lines {
		p.SetXY(innerX, cy)
		p.CellFormat(innerW, lh, ln, "", 0, "L", false, 0, "")
		cy += lh
	}
	p.SetXY(pdfMargin, y+h+3)
}

func (d *pdfDoc) notices() {
	if d.e.Partial {
		d.noticeBox("", d.l.Partial)
	}
	for _, note := range d.e.Corrections {
		d.noticeBox(d.l.Correction, note)
	}
}

func (d *pdfDoc) sectionHead(section string) {
	p := d.p
	d.font("B", 7)
	d.text(pdfAccent)
	d.ensure(6.5)
	if d.flow && !d.atTop() {
		p.SetY(p.GetY() + 4)
	}
	d.drawLines(d.wrap(strings.ToUpper(sectionLabel(d.e.Language, section)), d.w()), 3.6, "L", "")
	d.bar(d.x0(), p.GetY()+.2, d.w(), .5, pdfInk)
	p.SetY(p.GetY() + 2.7)
	d.text(pdfInk)
	d.lastStory = false
}

// singleSourceLines/Height measure the boxed single-source note.
func (d *pdfDoc) singleSourceLines() []string {
	d.font("", 7.5)
	return wrapText(d.l.SingleSource, d.w()-3, d.w()-3, d.measure)
}

func (d *pdfDoc) singleSource(s Story) {
	if !s.SingleSource {
		return
	}
	p := d.p
	lines := d.singleSourceLines()
	h := float64(len(lines))*4.2 + 2
	d.ensure(h + 2)
	y := p.GetY()
	d.draw(pdfAccent)
	p.SetLineWidth(.25)
	p.Rect(d.x0(), y, d.w(), h, "D")
	d.text(pdfInk)
	cy := y + 1
	for _, ln := range lines {
		p.SetXY(d.x0()+1.5, cy)
		p.CellFormat(d.w()-3, 4.2, ln, "", 0, "L", false, 0, "")
		cy += 4.2
	}
	p.SetXY(d.x0(), y+h+1.8)
}

// storyHeight measures headline, deck, single-source box and two body lines.
func (d *pdfDoc) storyHeight(s Story) float64 {
	d.font("B", 13)
	h := float64(len(d.wrap(s.Headline, pdfColW))) * 5.6
	if s.Deck != "" {
		d.font("B", 9.5)
		h += float64(len(d.wrap(s.Deck, pdfColW)))*4.4 + 1.2
	}
	if s.SingleSource {
		h += float64(len(d.singleSourceLines()))*4.2 + 2 + 1.8
	}
	return h + .8 + 2*4.3
}

func (d *pdfDoc) storyHead(s Story) {
	d.font("B", 13)
	d.text(pdfInk)
	d.para(s.Headline, 5.6, "L")
	d.p.SetY(d.p.GetY() + .8)
	if s.Deck != "" {
		d.font("B", 9.5)
		d.para(s.Deck, 4.4, "L")
		d.p.SetY(d.p.GetY() + 1.2)
	}
	d.singleSource(s)
}

func (d *pdfDoc) paragraphs(s Story) {
	for i, para := range s.Paragraphs {
		indent := 0.0
		if i > 0 {
			indent = 3.5
		}
		d.paragraph(para, indent)
	}
}

// paragraph draws one body paragraph with an optional first-line indent and a
// raised citation group that is never split across lines.
func (d *pdfDoc) paragraph(para Paragraph, indent float64) {
	p := d.p
	const lh = 4.3
	w := d.w()
	d.font("", 9.5)
	d.text(pdfInk)
	lines := wrapText(para.Text, w-indent, w, d.measure)
	var lastX, lastW float64
	for i, ln := range lines {
		d.ensure(lh)
		x := d.x0()
		if i == 0 {
			x += indent
		}
		p.SetXY(x, p.GetY())
		p.CellFormat(w-(x-d.x0()), lh, ln, "", 0, "L", false, 0, "")
		lastX, lastW = x, d.measure(ln)
		if i < len(lines)-1 {
			p.SetY(p.GetY() + lh)
		}
	}
	y := p.GetY()
	if len(para.SourceIDs) > 0 {
		cite := refs(para.SourceIDs)
		space := d.measure(" ")
		d.font("", 6)
		cw := d.measure(cite)
		d.text(pdfAccent)
		if lastX+lastW+space+cw <= d.x0()+w {
			p.SetXY(lastX+lastW+space, y-1.2)
			p.CellFormat(cw+.2, lh, cite, "", 0, "L", false, 0, "")
		} else {
			p.SetY(y + lh)
			d.ensure(lh)
			y = p.GetY()
			p.SetXY(d.x0(), y)
			p.CellFormat(cw+.2, lh, cite, "", 0, "L", false, 0, "")
		}
		d.font("", 9.5)
		d.text(pdfInk)
	}
	p.SetY(y + lh + .5)
}

// storyRule draws the thin separator that precedes a story.
func (d *pdfDoc) storyRule() {
	p := d.p
	p.SetY(p.GetY() + 2.5)
	d.draw(pdfRule)
	p.SetLineWidth(.15)
	p.Line(d.x0(), p.GetY(), d.x0()+pdfColW, p.GetY())
	p.SetY(p.GetY() + 3)
}

// naturalLess orders IDs such as src-2 before src-10.
func naturalLess(a, b string) bool {
	split := func(s string) (string, int, bool) {
		i := strings.IndexFunc(s, unicode.IsDigit)
		if i < 0 {
			return s, 0, false
		}
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		n, err := strconv.Atoi(s[i:j])
		return s[:i], n, err == nil && j == len(s)
	}
	pa, na, oka := split(a)
	pb, nb, okb := split(b)
	if pa != pb {
		return pa < pb
	}
	if oka && okb && na != nb {
		return na < nb
	}
	return a < b
}

type sourceBlock struct {
	title, url, dates []string
	h                 float64
}

func (d *pdfDoc) sourceBlock(s Source) sourceBlock {
	var b sourceBlock
	d.font("B", 7.5)
	b.title = d.wrap("["+s.ID+"] "+s.Publisher+" — "+s.Title, pdfColW)
	d.font("", 6.5)
	b.url = d.wrap(s.URL, pdfColW)
	b.dates = d.wrap(sourceDates(d.l, s), pdfColW)
	b.h = float64(len(b.title))*3.4 + float64(len(b.url)+len(b.dates))*3 + 2
	return b
}

func (d *pdfDoc) sources() {
	p := d.p
	d.font("B", 11)
	head := d.wrap(d.l.Sources, pdfColW)
	headH := 0.8 + 1.5 + float64(len(head))*5.2 + 2.8
	sources := append([]Source(nil), d.e.Sources...)
	sort.SliceStable(sources, func(i, j int) bool { return naturalLess(sources[i].ID, sources[j].ID) })
	need := headH
	var first sourceBlock
	if len(sources) > 0 {
		first = d.sourceBlock(sources[0])
		need += first.h
	}
	d.ensure(need)
	d.bar(d.x0(), p.GetY(), pdfColW, .8, pdfInk)
	p.SetY(p.GetY() + 1.5)
	d.font("B", 11)
	d.text(pdfInk)
	d.drawLines(head, 5.2, "L", "")
	d.bar(d.x0(), p.GetY()+.3, pdfColW, .2, pdfInk)
	p.SetY(p.GetY() + 2.8)
	d.lastStory = false
	for _, s := range sources {
		b := d.sourceBlock(s)
		d.ensure(b.h)
		d.font("B", 7.5)
		d.text(pdfInk)
		d.drawLines(b.title, 3.4, "L", "")
		d.font("", 6.5)
		d.text(pdfAccent)
		link := ""
		if safeLinkURL(s.URL) {
			link = s.URL
		}
		d.drawLines(b.url, 3, "L", link)
		d.text(pdfMuted)
		d.drawLines(b.dates, 3, "L", "")
		d.text(pdfInk)
		p.SetY(p.GetY() + 2)
	}
}

func (d *pdfDoc) colophonBlock() {
	p := d.p
	d.font("", 6.5)
	lines := d.wrap(colophon(d.e), pdfColW)
	keep := len(lines)
	if keep > 2 {
		keep = 2
	}
	d.ensure(1 + 2.4 + 3 + float64(keep)*3)
	p.SetY(p.GetY() + 1)
	d.bar(d.x0(), p.GetY(), pdfColW, .2, pdfInk)
	d.bar(d.x0(), p.GetY()+1, pdfColW, .2, pdfInk)
	p.SetY(p.GetY() + 2.4)
	d.font("B", 6.5)
	d.text(pdfInk)
	d.drawLines([]string{strings.ToUpper(d.l.About)}, 3, "L", "")
	d.font("", 6.5)
	d.text(pdfMuted)
	d.drawLines(lines, 3, "L", "")
	d.text(pdfInk)
}
