package atlaspdf

import (
	"strings"

	"aurago/internal/infra"
)

type statTile struct {
	label string
	value int
	col   rgb
}

func (d *doc) stats() []statTile {
	counts := map[infra.NodeKind]int{}
	running := 0
	for _, n := range d.m.nodes {
		counts[n.Kind]++
		if n.Power == infra.PowerRunning && !isPlace(n.Kind) {
			running++
		}
	}
	return []statTile{
		{d.lb.get("desktop.infra_pdf_stat_devices"), counts[infra.NodeDevice], kindColor(infra.NodeDevice)},
		{d.lb.get("desktop.infra_pdf_stat_vms"), counts[infra.NodeVM], kindColor(infra.NodeVM)},
		{d.lb.get("desktop.infra_pdf_stat_containers"), counts[infra.NodeContainer], kindColor(infra.NodeContainer)},
		{d.lb.get("desktop.infra_pdf_stat_services"), counts[infra.NodeService], kindColor(infra.NodeService)},
		{d.lb.get("desktop.infra_pdf_stat_networks"), counts[infra.NodeNetwork], kindColor(infra.NodeNetwork)},
		{d.lb.get("desktop.infra_pdf_stat_connections"), len(d.m.connections()), edgeStyleOf(infra.EdgeEthernet).col},
		{d.lb.get("desktop.infra_pdf_stat_running"), running, powerColor(infra.PowerRunning)},
	}
}

// drawOverviewTop paints the title block and the stat tiles of the first map page.
func (d *doc) drawOverviewTop() {
	_, _, right := d.overviewLayout()
	c := d.content
	tilesW := statTiles*statTileW + (statTiles-1)*statTileGap
	titleW := c.W
	if right {
		titleW = c.W - tilesW - 8
	}
	d.fitted(fontBold, 22, colInk, c.X, c.Y+6.5, titleW, d.title, 'L')
	sub := d.opts.Now.Format("2006-01-02") + " · " + d.lb.assets(d.m.assets())
	if !d.textCov.has('·') {
		sub = strings.ReplaceAll(sub, "·", "-")
	}
	d.fitted(fontRegular, 9, colText2, c.X, c.Y+14, titleW, sub, 'L')
	d.fill(colAccent)
	d.pdf.Rect(c.X, c.Y+18.2, 18, 0.8, "F")

	x, y, gap := c.right()-tilesW, c.Y+2, statTileGap
	if !right {
		y = c.Y + 21
		x = c.X
		gap = (c.W - statTiles*statTileW) / float64(statTiles-1)
	}
	for _, t := range d.stats() {
		d.statTile(t, x, y)
		x += statTileW + gap
	}
}

func (d *doc) statTile(t statTile, x, y float64) {
	d.dash(nil)
	d.fill(colWhite)
	d.stroke(colHair, 0.35)
	d.rrect(x, y, statTileW, statTileH, 1.6, "FD")
	d.clipRound(x, y, statTileW, statTileH, 1.6)
	d.fill(t.col)
	d.pdf.Rect(x, y, statTileW, 1, "F")
	d.unclip()
	d.text(fontBold, 14, colInk, x+3, y+7.6, itoa(t.value), 'L')
	d.fitted(fontRegular, 6, colText2, x+3, y+12.6, statTileW-5, t.label, 'L')
}

// drawLegend paints kind swatches, line samples and power dots in up to two rows.
func (d *doc) drawLegend(a rect) {
	const fs = 6.0
	d.dash(nil)
	label := d.lb.get("desktop.infra_pdf_legend")
	d.text(fontBold, fs+0.5, colText3, a.X+1, a.Y+2.3, strings.ToUpper(label), 'L')
	x0 := a.X + 3 + d.width(fontBold, fs+0.5, strings.ToUpper(label))
	x, row := x0, 0
	rowY := func() float64 { return a.Y + 2.3 + float64(row)*4.4 }
	place := func(w float64) (float64, bool) {
		if x+w > a.right() && x > x0 {
			x, row = x0, row+1
		}
		if row > 1 {
			return 0, false
		}
		at := x
		x += w + 4
		return at, true
	}
	kinds := map[infra.NodeKind]bool{}
	hasPower := false
	for _, n := range d.m.nodes {
		if !isPlace(n.Kind) {
			kinds[n.Kind] = true
			hasPower = hasPower || n.Power != ""
		}
	}
	for _, k := range []infra.NodeKind{infra.NodeDevice, infra.NodeVM, infra.NodeContainer, infra.NodeService, infra.NodeNetwork} {
		if !kinds[k] {
			continue
		}
		text := d.lb.kind(k)
		if at, ok := place(3.4 + d.width(fontRegular, fs, text)); ok {
			d.fill(kindColor(k))
			d.rrect(at, rowY()-1.3, 2.6, 2.6, 0.7, "F")
			d.text(fontRegular, fs, colText2, at+3.6, rowY(), text, 'L')
		}
	}
	used := map[infra.EdgeKind]bool{}
	for _, l := range d.sc.lines {
		used[l.Kind] = true
	}
	for _, k := range []infra.EdgeKind{infra.EdgeEthernet, infra.EdgeFiber, infra.EdgeWiFi, infra.EdgeDependsOn, infra.EdgePoweredBy, infra.EdgeMemberOf, infra.EdgeRunsOn} {
		if !used[k] {
			continue
		}
		text := d.lb.edge(k)
		if at, ok := place(8.4 + d.width(fontRegular, fs, text)); ok {
			st := edgeStyleOf(k)
			d.stroke(st.col, max(st.wPt, 0.5))
			d.dash(st.dash)
			d.pdf.Line(at, rowY(), at+6, rowY())
			d.dash(nil)
			d.text(fontRegular, fs, colText2, at+7.4, rowY(), text, 'L')
		}
	}
	if hasPower {
		for _, p := range []string{infra.PowerRunning, infra.PowerPaused, infra.PowerStopped, infra.PowerUnknown} {
			text := d.lb.power(p)
			if at, ok := place(3.8 + d.width(fontRegular, fs, text)); ok {
				d.powerDot(at+1, rowY(), 1, p)
				d.text(fontRegular, fs, colText2, at+3.4, rowY(), text, 'L')
			}
		}
	}
}

// drawChrome adds the running header and footer once the page count is known.
func (d *doc) drawChrome() {
	total := len(d.pages)
	date := d.opts.Now.Format("2006-01-02")
	stamp := d.lb.generated(d.opts.Now.Format("2006-01-02 15:04"))
	sep := " · "
	if !d.textCov.has('·') {
		sep = " - "
	}
	for i, p := range d.pages {
		d.pdf.SetPage(i + 1)
		d.dirty = true
		d.dash(nil)
		d.fill(colAccent)
		d.pdf.Rect(marginX, marginX, 7, 0.6, "F")
		d.fitted(fontMedium, 8, colInk, marginX, marginX+4.4, d.W*0.7, d.title+sep+p.title, 'L')
		d.text(fontRegular, 7.5, colText3, d.W-marginX, marginX+4.4, date, 'R')
		d.stroke(colHair, 0.3)
		d.pdf.Line(marginX, marginX+8, d.W-marginX, marginX+8)
		fy := d.H - marginX - footerBand
		d.pdf.Line(marginX, fy+1.2, d.W-marginX, fy+1.2)
		mid := fy + 4.6
		d.text(fontRegular, 6.5, colText3, marginX, mid, "AuraGo Infra", 'L')
		d.text(fontMedium, 7, colText2, d.W/2, mid, d.lb.pageOf(i+1, total), 'C')
		d.text(fontRegular, 6.5, colText3, d.W-marginX, mid, stamp, 'R')
	}
}
