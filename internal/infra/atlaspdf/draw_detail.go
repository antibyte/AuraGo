package atlaspdf

import (
	"math"
	"sort"
	"strings"

	"aurago/internal/infra"

	"github.com/phpdave11/gofpdf"
)

const (
	stubLength = 10.0 // mm
	maxPills   = 3
)

// drawDetail paints the islands of a detail page, their off-island stubs and the locator inset.
func (d *doc) drawDetail(idx int, p mapPage) {
	onPage := map[string]bool{}
	var occupied, highlights []rect
	for _, pl := range p.Places {
		is := &d.islands[pl.Island]
		r := pl.boxRect(is, p.Scale)
		v := view{r, is.Box, p.Scale}
		d.drawCaption(is, pl.X, pl.Y, r.W)
		occupied = append(occupied, rect{pl.X, pl.Y, r.W, islandCaptionH})
		for _, rg := range d.sc.regions {
			if is.regions[rg.ID] {
				if rr := v.rect(rg.Rect); rr.intersects(r) {
					occupied = append(occupied, rect{rr.X, rr.Y, rr.W, math.Min(rr.H, 6)})
				}
			}
		}
		d.dash(nil)
		if len(is.Regions) == 0 {
			d.fill(colWhite)
			d.stroke(colHair, 0.35)
			d.dash([]float64{1.2, 0.9})
			d.rrect(r.X, r.Y, r.W, r.H, 2, "FD")
			d.dash(nil)
		}
		tiled := is.Parts > 1
		if tiled {
			d.clipRound(r.X, r.Y, r.W, r.H, 1)
		}
		d.drawRegions(v, func(id string) bool { return is.regions[id] })
		d.drawLines(v, true, func(l line) bool { return !(is.has(l.From) && is.has(l.To)) })
		d.drawCards(v, true, func(id string) bool { return is.cards[id] })
		if tiled {
			d.unclip()
		}
		for id := range is.cards {
			onPage[id] = true
		}
		for id := range is.regions {
			onPage[id] = true
		}
		for _, ci := range is.Cards {
			if cr := v.rect(d.sc.cards[ci].rect()); cr.intersects(r) {
				occupied = append(occupied, cr)
			}
		}
		highlights = append(highlights, is.Box)
	}
	d.drawStubs(idx, p, onPage, append(occupied, p.Locator.pad(0.6)))
	d.drawLocator(p.Locator, highlights)
}

func (d *doc) drawCaption(is *island, x, y, maxW float64) {
	text := is.Caption
	if is.Parts > 1 {
		text += " · " + d.lb.part(is.Part, is.Parts)
	}
	count := itoa(is.Assets)
	cw := d.width(fontRegular, 6.8, count)
	yMid := y + 2.2
	name := d.fit(fontBold, 8, text, maxW-cw-2)
	if name == "" {
		return
	}
	nw := d.width(fontBold, 8, name)
	d.text(fontBold, 8, colInk, x, yMid, name, 'L')
	if nw+cw+2 <= maxW {
		d.text(fontRegular, 6.8, colText3, x+nw+2, yMid, count, 'L')
	}
}

type stubTarget struct {
	id   string
	kind infra.EdgeKind
}

func (d *doc) linesAt() map[string][]int {
	if d.lineIdx == nil {
		d.lineIdx = map[string][]int{}
		for i, l := range d.sc.lines {
			d.lineIdx[l.From] = append(d.lineIdx[l.From], i)
			d.lineIdx[l.To] = append(d.lineIdx[l.To], i)
		}
	}
	return d.lineIdx
}

func (d *doc) drawStubs(idx int, p mapPage, onPage map[string]bool, occupied []rect) {
	occ := occupied
	for _, pl := range p.Places {
		is := &d.islands[pl.Island]
		r := pl.boxRect(is, p.Scale)
		v := view{r, is.Box, p.Scale}
		for _, ci := range is.Cards {
			c := d.sc.cards[ci]
			if cr := v.rect(c.rect()); cr.intersects(r) {
				occ = d.cardStubs(idx, c, cr, is, onPage, occ)
			}
		}
	}
}

// cardStubs draws the short arrows with page pills that stand for a card's
// connections to other islands: at most three pills plus a "+N" pill.
func (d *doc) cardStubs(idx int, c card, cr rect, is *island, onPage map[string]bool, occ []rect) []rect {
	seen := map[string]bool{}
	groups := map[int][]stubTarget{}
	for _, li := range d.linesAt()[c.ID] {
		l := d.sc.lines[li]
		other := l.To
		if l.From != c.ID {
			other = l.From
		}
		if l.Kind == infra.EdgeMemberOf || is.has(other) || seen[other] {
			continue
		}
		seen[other] = true
		key := 0
		if onPage[other] {
			key = -1
		} else if pg, ok := d.first[other]; ok {
			key = pg + 1
		}
		groups[key] = append(groups[key], stubTarget{other, l.Kind})
	}
	if len(groups) == 0 {
		return occ
	}
	keys := make([]int, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	rest := 0
	for _, k := range keys[min(maxPills, len(keys)):] {
		rest += len(groups[k])
	}
	keys = keys[:min(maxPills, len(keys))]
	ctr := cr.center()
	var last rect
	placedAny := false
	for _, k := range keys {
		ts := groups[k]
		o, ok := d.sc.shape(ts[0].id)
		if !ok {
			continue
		}
		dir := pt{o.c.X - c.X, o.c.Y - c.Y}
		n := math.Hypot(dir.X, dir.Y)
		if n == 0 {
			continue
		}
		dir = pt{dir.X / n, dir.Y / n}
		var names []string
		for _, t := range ts {
			names = append(names, d.sc.shapeName(t.id))
		}
		arrow := "->"
		if d.textCov.has('→') {
			arrow = "→"
		}
		suffix := ""
		if k > 0 {
			suffix = " · " + d.lb.pageRef(k)
		}
		const fs = 5.5
		label := arrow + " " + d.fit(fontMedium, fs, strings.Join(names, ", "), 46-d.width(fontMedium, fs, suffix)) + suffix
		w, h := d.width(fontMedium, fs, label)+2.6, 3.4
		half := pt{cr.W / 2, cr.H / 2}
		var start pt
		box, length, found := rect{}, 0.0, false
		want := dir
	search:
		// try the direction of the target first, then turn away from neighbouring cards
		for _, turn := range []float64{0, 0.45, -0.45, 0.9, -0.9, 1.57, -1.57, 3.14} {
			sin, cos := math.Sincos(turn)
			dir = pt{want.X*cos - want.Y*sin, want.X*sin + want.Y*cos}
			start = borderPoint(ctr, pt{ctr.X + dir.X, ctr.Y + dir.Y}, half)
			support := math.Abs(dir.X)*w/2 + math.Abs(dir.Y)*h/2
			for j := 0; j < 3; j++ {
				length = stubLength + float64(j)*3
				if !segmentClear(start, dir, length, occ) {
					continue
				}
				for t := 0; t < 5; t++ {
					off := float64((t+1)/2) * (h + 0.6)
					if t%2 == 1 {
						off = -off
					}
					cx := start.X + dir.X*(length+0.8+support) - dir.Y*off
					cy := start.Y + dir.Y*(length+0.8+support) + dir.X*off
					box = rect{cx - w/2, cy - h/2, w, h}
					tip := pt{start.X + dir.X*length, start.Y + dir.Y*length}
					if boxGap(box, rect{tip.X, tip.Y, 0, 0}) > 3 {
						continue // a pill must stay next to its arrow
					}
					if d.pillFree(box, occ) {
						found = true
						break search
					}
				}
			}
		}
		if !found {
			continue
		}
		st := edgeStyleOf(ts[0].kind)
		end := pt{start.X + dir.X*length, start.Y + dir.Y*length}
		base := pt{end.X - dir.X*1.8, end.Y - dir.Y*1.8}
		d.stroke(st.col, math.Max(st.wPt, 0.5))
		d.dash(st.dash)
		d.pdf.Line(start.X, start.Y, base.X, base.Y)
		d.dash(nil)
		d.fill(st.col)
		perp := pt{-dir.Y * 0.85, dir.X * 0.85}
		d.pdf.Polygon([]gofpdf.PointType{{X: end.X, Y: end.Y}, {X: base.X + perp.X, Y: base.Y + perp.Y}, {X: base.X - perp.X, Y: base.Y - perp.Y}}, "F")
		d.fill(colWhite)
		d.stroke(st.col, 0.3)
		d.rrect(box.X, box.Y, box.W, box.H, h/2, "FD")
		d.text(fontMedium, fs, colText2, box.X+box.W/2, box.Y+box.H/2, label, 'C')
		occ = append(occ, box)
		last, placedAny = box, true
	}
	if rest > 0 && placedAny {
		text := "+" + itoa(rest)
		w, h := d.width(fontMedium, 5.5, text)+2.4, 3.4
		for _, at := range []pt{{last.right() + 0.6, last.Y}, {last.X - w - 0.6, last.Y}, {last.X, last.bottom() + 0.5}, {last.X, last.Y - h - 0.5}} {
			box := rect{at.X, at.Y, w, h}
			if d.pillFree(box, occ) {
				d.fill(colWhite)
				d.stroke(colHair, 0.3)
				d.rrect(box.X, box.Y, w, h, h/2, "FD")
				d.text(fontMedium, 5.5, colText2, box.X+w/2, box.Y+h/2, text, 'C')
				occ = append(occ, box)
				break
			}
		}
	}
	return occ
}

// pillFree is true when box lies on the page and touches no card, pill or the locator.
func (d *doc) pillFree(box rect, occ []rect) bool {
	c := d.content.pad(-0.5)
	if box.X < c.X || box.Y < c.Y || box.right() > c.right() || box.bottom() > c.bottom() {
		return false
	}
	for _, o := range occ {
		if box.pad(0.3).intersects(o) {
			return false
		}
	}
	return true
}

// drawLocator draws the whole scene as a small inset with the island boxes of this page highlighted.
func (d *doc) drawLocator(box rect, highlights []rect) {
	if !d.sc.hasBounds {
		return
	}
	d.fill(colWhite)
	d.stroke(hexColor(0xB8C0CF), 0.3)
	d.dash(nil)
	d.rrect(box.X, box.Y, box.W, box.H, 1.5, "FD")
	inner := box.pad(-2.2)
	b := d.sc.bounds
	s := math.Min(inner.W/b.W, inner.H/b.H)
	ox := inner.X + (inner.W-b.W*s)/2
	oy := inner.Y + (inner.H-b.H*s)/2
	mini := view{area: rect{ox, oy, b.W * s, b.H * s}, world: b, s: s}
	d.clipRound(box.X, box.Y, box.W, box.H, 1.5)
	d.fill(mix(colAccent, colWhite, 0.86))
	d.stroke(colAccent, 0.8)
	for _, h := range highlights {
		r := mini.rect(h)
		d.pdf.Rect(r.X, r.Y, r.W, r.H, "FD")
	}
	d.stroke(hexColor(0xAEB7C6), 0.2)
	for _, rg := range d.sc.regions {
		r := mini.rect(rg.Rect)
		d.pdf.Rect(r.X, r.Y, r.W, r.H, "D")
	}
	d.fill(hexColor(0x8A93A3))
	for _, c := range d.sc.cards {
		p := mini.pt(pt{c.X, c.Y})
		d.circle(p.X, p.Y, 0.3, "F")
	}
	d.unclip()
}

// segmentClear is true when a stub of the given length from start runs through no occupied rectangle.
func segmentClear(start, dir pt, length float64, occ []rect) bool {
	for _, f := range []float64{0.25, 0.5, 0.75, 1} {
		p := pt{start.X + dir.X*length*f, start.Y + dir.Y*length*f}
		for _, o := range occ {
			if o.contains(p) {
				return false
			}
		}
	}
	return true
}
