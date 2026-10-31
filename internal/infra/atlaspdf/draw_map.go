package atlaspdf

import (
	"math"
	"strings"

	"aurago/internal/infra"
)

// newPage starts a page; the running header and footer are added at the end.
func (d *doc) newPage(title string) {
	d.pdf.AddPage()
	d.dirty = true
	d.pages = append(d.pages, pageInfo{title: title})
}

func (d *doc) emptyPage() {
	d.newPage(d.lb.get("desktop.infra_pdf_overview"))
	d.drawEmptyMessage(d.content)
}

func (d *doc) drawEmptyMessage(area rect) {
	c := area.center()
	d.text(fontMedium, 11, colText2, c.X, c.Y, d.fit(fontMedium, 11, d.lb.get("desktop.infra_pdf_empty"), area.W-10), 'C')
}

func (d *doc) drawMap() {
	d.mapPages = d.planMap()
	d.computeFirst()
	for i, p := range d.mapPages {
		d.newPage(p.Title)
		d.drawMapPage(i, p)
	}
}

func (d *doc) drawMapPage(idx int, p mapPage) {
	if p.Overview {
		d.drawOverviewTop()
		_, legend, _ := d.overviewLayout()
		defer d.drawLegend(legend)
	}
	a := p.Area
	d.fill(colWhite)
	d.stroke(colHair, 0.35)
	d.dash(nil)
	d.rrect(a.X, a.Y, a.W, a.H, 2, "FD")
	d.clipRound(a.X, a.Y, a.W, a.H, 2)
	defer d.unclip()
	d.drawDotGrid(a)
	if len(d.sc.cards) == 0 && len(d.sc.regions) == 0 {
		d.drawEmptyMessage(a)
		return
	}
	if !p.Overview {
		d.drawDetail(idx, p)
		return
	}
	v := p.view()
	full := p.Scale >= readableScale
	d.drawRegions(v, nil)
	d.drawLines(v, full, nil)
	d.drawCards(v, full, nil)
}

func (d *doc) drawDotGrid(a rect) {
	d.fill(colGrid)
	for y := a.Y + 3; y < a.bottom()-1; y += 6 {
		for x := a.X + 3; x < a.right()-1; x += 6 {
			d.pdf.Rect(x-0.09, y-0.09, 0.18, 0.18, "F")
		}
	}
}

func (d *doc) drawRegions(v view, only func(string) bool) {
	for _, rg := range d.sc.regions {
		if only != nil && !only(rg.ID) {
			continue
		}
		r := v.rect(rg.Rect)
		if !v.visible(r) || r.W < 0.4 || r.H < 0.4 {
			continue
		}
		var n *infra.Node
		kind := infra.NodeGroup
		if !rg.Pseudo {
			if n = d.m.node(rg.ID); n != nil {
				kind = n.Kind
			}
		}
		st := regionStyleOf(kind)
		fill := mix(st.fill, hexColor(0xDCE2EC), math.Min(0.6, 0.12*float64(rg.Depth)))
		d.fill(fill)
		d.stroke(st.stroke, 0.4)
		if st.dashed || rg.Pseudo {
			d.dash([]float64{1.2, 0.9})
		} else {
			d.dash(nil)
		}
		d.rrect(r.X, r.Y, r.W, r.H, math.Min(3, math.Min(r.W, r.H)/4), "FD")
		d.dash(nil)
		d.drawRegionHeader(rg, n, kind, r, v.s)
	}
}

func (d *doc) drawRegionHeader(rg region, n *infra.Node, kind infra.NodeKind, r rect, s float64) {
	pt := clamp(8.5*s/0.25, 6, 11)
	padMM := regionPad * s
	inset := clamp(padMM*0.35, 1.2, 2.6)
	yMid := r.Y + clamp(padMM/2, 2.2, 4.6)
	x := r.X + inset
	if isz := clamp(padMM*0.8, 0, 5); isz >= 3 {
		d.drawPlaceIcon(kind, x, yMid-isz/2, isz)
		x += isz + 1.2
	}
	name := d.lb.get("desktop.infra_pdf_unplaced")
	label := itoa(rg.Count)
	if !rg.Pseudo && n != nil {
		name = n.Name
		label = d.lb.kind(kind) + " · " + itoa(rg.Count)
	}
	right := r.right() - inset
	avail := right - x
	nameW := d.width(fontBold, pt, name)
	lw := d.width(fontRegular, pt*0.85, label)
	if nameW+lw+3 <= avail {
		d.text(fontRegular, pt*0.85, colText3, right, yMid, label, 'R')
		d.text(fontBold, pt, colInk, x, yMid, name, 'L')
		return
	}
	d.fitted(fontBold, pt, colInk, x, yMid, avail, name, 'L')
}

func (d *doc) drawLines(v view, full bool, skip func(line) bool) {
	var placed []rect
	if full {
		for _, c := range d.sc.cards {
			if cr := v.rect(c.rect()); v.visible(cr) {
				placed = append(placed, cr)
			}
		}
	}
	for _, l := range d.sc.lines {
		if skip != nil && skip(l) {
			continue
		}
		if !v.visible(v.rect(l.bbox())) {
			continue
		}
		st := edgeStyleOf(l.Kind)
		d.stroke(st.col, st.wPt)
		d.dash(st.dash)
		p0, p1 := v.pt(l.P0), v.pt(l.P1)
		if l.Curve {
			c0, c1 := v.pt(l.C0), v.pt(l.C1)
			d.pdf.CurveBezierCubic(p0.X, p0.Y, c0.X, c0.Y, c1.X, c1.Y, p1.X, p1.Y, "D")
		} else {
			d.pdf.Line(p0.X, p0.Y, p1.X, p1.Y)
		}
		d.dash(nil)
		if full {
			placed = d.drawEdgeLabel(l, v, placed)
		}
	}
}

// edgeCaption is the port/speed/label text of a connection.
func (d *doc) edgeCaption(e infra.Edge) string {
	arrow := "->"
	if d.textCov.has('→') {
		arrow = "→"
	}
	ports := e.FromPort
	if e.FromPort != "" && e.ToPort != "" {
		ports = e.FromPort + " " + arrow + " " + e.ToPort
	} else if e.ToPort != "" {
		ports = e.ToPort
	}
	var parts []string
	for _, s := range []string{ports, e.Speed, e.Label} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// drawEdgeLabel puts the caption in a white pill at the line's midpoint when it
// fits the line and clears the cards and earlier labels.
func (d *doc) drawEdgeLabel(l line, v view, placed []rect) []rect {
	text := d.edgeCaption(d.m.edges[l.Edge])
	if text == "" {
		return placed
	}
	const fs = 5.6
	w, h := d.width(fontRegular, fs, text)+2.4, 3.3
	p0, p1 := v.pt(l.P0), v.pt(l.P1)
	if w > math.Hypot(p1.X-p0.X, p1.Y-p0.Y)*0.8 {
		return placed
	}
	mid := v.pt(l.Mid)
	box := rect{mid.X - w/2, mid.Y - h/2, w, h}
	if !v.visible(box) {
		return placed
	}
	for _, r := range placed {
		if box.pad(0.4).intersects(r) {
			return placed
		}
	}
	d.fill(colWhite)
	d.stroke(colHair, 0.3)
	d.rrect(box.X, box.Y, w, h, h/2, "FD")
	d.text(fontRegular, fs, colText2, mid.X, mid.Y, text, 'C')
	return append(placed, box)
}

func (d *doc) drawCards(v view, full bool, only func(string) bool) {
	var visible []rect
	var labels []rect
	if !full {
		for _, c := range d.sc.cards {
			if r := v.rect(c.rect()); v.visible(r) {
				visible = append(visible, r)
			}
		}
	}
	for _, c := range d.sc.cards {
		if only != nil && !only(c.ID) {
			continue
		}
		r := v.rect(c.rect())
		if !v.visible(r) {
			continue
		}
		switch {
		case full:
			d.cardFull(c, r)
		case r.W >= compactWidth:
			d.cardCompact(c, r)
		default:
			if d.cardDot(c, r) {
				labels = d.dotLabel(c, r, visible, labels)
			}
		}
	}
}

// dotLabel names a bare dot in tiny type under it, unless that would touch another label or card.
func (d *doc) dotLabel(c card, r rect, cards, labels []rect) []rect {
	const fs = 5.0
	name := d.fit(fontRegular, fs, d.m.node(c.ID).Name, r.W)
	if name == "" {
		return labels
	}
	ctr := r.center()
	w := d.width(fontRegular, fs, name)
	box := rect{ctr.X - w/2, ctr.Y + 1.9, w, 2.1}
	for _, o := range labels {
		if box.pad(0.2).intersects(o) {
			return labels
		}
	}
	for _, o := range cards {
		if o != r && box.intersects(o.pad(-0.3)) {
			return labels
		}
	}
	d.text(fontRegular, fs, colText3, ctr.X, box.Y+box.H/2, name, 'C')
	return append(labels, box)
}
