package atlaspdf

import (
	"fmt"
	"io/fs"
	"math"
	"unicode/utf8"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

type fontSet struct{ regular, medium, bold, mono []byte }

// chooseFonts picks Go fonts, or NotoSansSC when some printed text needs it, and
// rewrites the model's strings so they only contain printable runes. Labels the
// available fonts cannot print fall back to English.
func (d *doc) chooseFonts() error {
	goCov, err := newCoverage(goregular.TTF, gobold.TTF, gomedium.TTF)
	if err != nil {
		return fmt.Errorf("atlaspdf: parse fonts: %w", err)
	}
	monoCov, err := newCoverage(gomono.TTF)
	if err != nil {
		return fmt.Errorf("atlaspdf: parse fonts: %w", err)
	}
	d.fonts = fontSet{goregular.TTF, gomedium.TTF, gobold.TTF, gomono.TTF}
	var cjk struct {
		cov           *coverage
		regular, bold []byte
		tried         bool
	}
	loadCJK := func() bool {
		if cjk.tried {
			return cjk.cov != nil
		}
		cjk.tried = true
		if d.opts.CJKFonts == nil {
			return false
		}
		regular, err1 := fs.ReadFile(d.opts.CJKFonts, "NotoSansSC-Regular.ttf")
		bold, err2 := fs.ReadFile(d.opts.CJKFonts, "NotoSansSC-Bold.ttf")
		if err1 != nil || err2 != nil {
			return false
		}
		cov, err := newCoverage(regular, bold)
		if err != nil {
			return false
		}
		cjk.cov, cjk.regular, cjk.bold = cov, regular, bold
		return true
	}
	useCJK := false
	if labelText := d.lb.corpus(); !goCov.all(labelText) {
		if loadCJK() && cjk.cov.all(labelText) {
			useCJK = true
		} else {
			d.lb.useEnglish()
		}
	}
	if !useCJK {
		for _, r := range d.m.corpus() {
			if !goCov.has(r) && loadCJK() && cjk.cov.has(r) {
				useCJK = true
				break
			}
		}
	}
	d.textCov, d.monoCov = goCov, monoCov
	if useCJK {
		d.textCov = cjk.cov
		d.fonts.regular, d.fonts.medium, d.fonts.bold = cjk.regular, cjk.regular, cjk.bold
	}
	d.ellipsis = "..."
	if d.textCov.has('…') {
		d.ellipsis = "…"
	}
	d.m.sanitise(d.textCov.clean, d.monoCov.clean)
	return nil
}

func (d *doc) registerFonts() {
	d.pdf.AddUTF8FontFromBytes(fontRegular, "", d.fonts.regular)
	d.pdf.AddUTF8FontFromBytes(fontMedium, "", d.fonts.medium)
	d.pdf.AddUTF8FontFromBytes(fontBold, "", d.fonts.bold)
	d.pdf.AddUTF8FontFromBytes(fontMono, "", d.fonts.mono)
}

// font selects a face. After a clip or a page switch the PDF graphics state no
// longer matches gofpdf's cache, so the face is forced out once.
func (d *doc) font(family string, pt float64) {
	if d.dirty {
		d.pdf.SetFont(family, "", pt+1)
		d.dirty = false
	}
	d.pdf.SetFont(family, "", pt)
}

func (d *doc) width(family string, pt float64, s string) float64 {
	d.font(family, pt)
	return d.pdf.GetStringWidth(s)
}

// fit shortens s with an ellipsis until it is at most maxW wide.
func (d *doc) fit(family string, pt float64, s string, maxW float64) string {
	if maxW <= 0 || s == "" {
		return ""
	}
	if d.width(family, pt, s) <= maxW {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if d.pdf.GetStringWidth(string(runes[:mid])+d.ellipsis) <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ""
	}
	return string(runes[:lo]) + d.ellipsis
}

// text prints s with its vertical middle at yMid. align is 'L', 'C' or 'R' about x.
func (d *doc) text(family string, pt float64, col rgb, x, yMid float64, s string, align byte) {
	if s == "" || utf8.RuneCountInString(s) == 0 {
		return
	}
	d.font(family, pt)
	w := d.pdf.GetStringWidth(s)
	switch align {
	case 'C':
		x -= w / 2
	case 'R':
		x -= w
	}
	d.pdf.SetTextColor(col.R, col.G, col.B)
	d.pdf.Text(x, yMid+pt*ptMM*0.35, s)
}

// fitted prints s, shortened to maxW.
func (d *doc) fitted(family string, pt float64, col rgb, x, yMid, maxW float64, s string, align byte) {
	d.font(family, pt)
	d.text(family, pt, col, x, yMid, d.fit(family, pt, s, maxW), align)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func (d *doc) fill(c rgb) { d.pdf.SetFillColor(c.R, c.G, c.B) }

func (d *doc) stroke(c rgb, wPt float64) {
	d.pdf.SetDrawColor(c.R, c.G, c.B)
	d.pdf.SetLineWidth(wPt * ptMM)
}

func (d *doc) dash(p []float64) {
	if len(p) == 0 {
		p = []float64{}
	}
	d.pdf.SetDashPattern(p, 0)
}

// rrect draws a rounded rectangle; style is "F", "D" or "FD".
func (d *doc) rrect(x, y, w, h, r float64, style string) {
	if w <= 0 || h <= 0 {
		return
	}
	r = math.Min(r, math.Min(w, h)/2)
	if r <= 0.05 {
		d.pdf.Rect(x, y, w, h, style)
		return
	}
	// gofpdf.RoundedRect leaves a q unclosed, so the path is built here.
	const k = 0.5523
	p := d.pdf
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.CurveBezierCubicTo(x+w-r+k*r, y, x+w, y+r-k*r, x+w, y+r)
	p.LineTo(x+w, y+h-r)
	p.CurveBezierCubicTo(x+w, y+h-r+k*r, x+w-r+k*r, y+h, x+w-r, y+h)
	p.LineTo(x+r, y+h)
	p.CurveBezierCubicTo(x+r-k*r, y+h, x, y+h-r+k*r, x, y+h-r)
	p.LineTo(x, y+r)
	p.CurveBezierCubicTo(x, y+r-k*r, x+r-k*r, y, x+r, y)
	p.ClosePath()
	p.DrawPath(style)
}

func (d *doc) clipRound(x, y, w, h, r float64) {
	d.pdf.ClipRoundedRect(x, y, w, h, math.Min(r, math.Min(w, h)/2), false)
}

func (d *doc) unclip() {
	d.pdf.ClipEnd()
	d.dirty = true
}

func (d *doc) circle(x, y, r float64, style string) { d.pdf.Circle(x, y, r, style) }
