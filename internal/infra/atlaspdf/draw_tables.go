package atlaspdf

import (
	"sort"
	"strings"

	"aurago/internal/infra"
)

type column struct {
	title string
	x, w  float64
}

// layoutColumns splits the content width in proportion to weights.
func (d *doc) layoutColumns(titles []string, weights []float64) []column {
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	cols := make([]column, len(titles))
	x := d.content.X
	for i := range titles {
		w := d.content.W * weights[i] / sum
		cols[i] = column{titles[i], x, w}
		x += w
	}
	return cols
}

// table flows rows over pages with a repeated header row.
type table struct {
	d      *doc
	cols   []column
	title  string
	y      float64
	rowH   float64
	headH  float64
	fs     float64
	zebra  int
	cont   func(y float64) float64 // redraws the open group heading, returns its height
	opened bool
}

func (d *doc) newTable(title string, cols []column) *table {
	tk := d.k
	return &table{d: d, cols: cols, title: title, rowH: 5.4 * tk, headH: 6.2 * tk, fs: 6.3 * tk}
}

// ensure makes room for h, starting a new page (with the header row) when needed.
func (t *table) ensure(h float64) {
	d := t.d
	if t.opened && t.y+h <= d.content.bottom() {
		return
	}
	d.newPage(t.title)
	t.opened = true
	t.y = d.content.Y
	d.dash(nil)
	d.fill(colHead)
	d.rrect(d.content.X, t.y, d.content.W, t.headH, 1.2, "F")
	for _, c := range t.cols {
		d.fitted(fontBold, t.fs, colInk, c.x+1.5, t.y+t.headH/2, c.w-2.5, c.title, 'L')
	}
	t.y += t.headH
	t.zebra = 0
	if t.cont != nil && h > 0 {
		t.y += t.cont(t.y)
	}
}

// heading draws a group heading row; it stays with the first row after it.
func (t *table) heading(kind infra.NodeKind, text string) {
	d := t.d
	hh := 6.4 * d.k
	t.cont = nil
	t.ensure(hh + t.rowH)
	draw := func(y float64, suffix string) float64 {
		d.fill(colGroup)
		d.pdf.Rect(d.content.X, y, d.content.W, hh, "F")
		x := d.content.X + 1.5
		if kind != "" {
			d.drawPlaceIcon(kind, x, y+(hh-4.2*d.k)/2, 4.2*d.k)
			x += 5.6 * d.k
		}
		d.fitted(fontBold, 7.2*d.k, colInk, x, y+hh/2, d.content.W-(x-d.content.X)-3, text+suffix, 'L')
		return hh
	}
	t.y += draw(t.y, "")
	cont := " (" + d.lb.get("desktop.infra_pdf_continued") + ")"
	t.cont = func(y float64) float64 { return draw(y, cont) }
	t.zebra = 0
}

// row reserves a row and returns its vertical middle; odd rows get a zebra band.
func (t *table) row() float64 {
	t.ensure(t.rowH)
	d := t.d
	if t.zebra%2 == 1 {
		d.fill(colZebra)
		d.pdf.Rect(d.content.X, t.y, d.content.W, t.rowH, "F")
	}
	t.zebra++
	mid := t.y + t.rowH/2
	t.y += t.rowH
	return mid
}

func (t *table) cell(i int, text string, mono bool, col rgb, yMid float64) {
	c := t.cols[i]
	fam := fontRegular
	if mono {
		fam = fontMono
	}
	t.d.fitted(fam, t.fs, col, c.x+1.5, yMid, c.w-2.5, text, 'L')
}

// nameCell prints an icon and a name in column i.
func (t *table) nameCell(i int, n *infra.Node, yMid float64) {
	c, d := t.cols[i], t.d
	isz := 3.8 * d.k
	d.drawIcon(n, c.x+1.4, yMid-isz/2, isz)
	d.fitted(fontMedium, t.fs, colInk, c.x+2.4+isz, yMid, c.w-3.9-isz, n.Name, 'L')
}

func sourcesText(n *infra.Node) string {
	seen := map[string]bool{}
	var kinds []string
	for _, s := range n.Sources {
		if s.Ignored || s.State == infra.SourceRemoved || seen[s.Kind] {
			continue
		}
		seen[s.Kind] = true
		kinds = append(kinds, s.Kind)
	}
	if len(kinds) == 0 {
		return "–"
	}
	sort.Strings(kinds)
	return strings.Join(kinds, ", ")
}

func (d *doc) drawInventory() {
	type group struct {
		path  string
		kind  infra.NodeKind
		nodes []*infra.Node
	}
	groups := map[string]*group{}
	sep := d.placeSep()
	for i := range d.m.nodes {
		n := &d.m.nodes[i]
		if isPlace(n.Kind) {
			continue
		}
		key := strings.Join(d.m.placePath(n.ID), sep)
		g := groups[key]
		if g == nil {
			g = &group{path: key}
			if p := d.m.node(d.m.parent[n.ID]); p != nil && key != "" {
				g.kind = p.Kind
			}
			groups[key] = g
		}
		g.nodes = append(g.nodes, n)
	}
	if len(groups) == 0 {
		return
	}
	var order []*group
	for _, g := range groups {
		order = append(order, g)
		sort.Slice(g.nodes, func(i, j int) bool {
			a, b := g.nodes[i], g.nodes[j]
			if kindOrder[a.Kind] != kindOrder[b.Kind] {
				return kindOrder[a.Kind] < kindOrder[b.Kind]
			}
			if a.Name != b.Name {
				return a.Name < b.Name
			}
			return a.ID < b.ID
		})
	}
	sort.Slice(order, func(i, j int) bool {
		if (order[i].path == "") != (order[j].path == "") {
			return order[j].path == ""
		}
		return strings.ToLower(order[i].path) < strings.ToLower(order[j].path)
	})
	get := d.lb.get
	cols := d.layoutColumns([]string{
		get("desktop.infra_pdf_col_name"), get("desktop.infra_pdf_col_kind"), get("desktop.infra_pdf_col_address"),
		get("desktop.infra_pdf_col_mac"), get("desktop.infra_pdf_col_host"), get("desktop.infra_pdf_col_system"),
		get("desktop.infra_pdf_col_sources"), get("desktop.infra_pdf_col_power"),
	}, []float64{25, 10, 12, 14, 13, 16, 9, 8})
	t := d.newTable(get("desktop.infra_pdf_inventory_title"), cols)
	for _, g := range order {
		name := g.path
		if name == "" {
			name = get("desktop.infra_pdf_no_place")
		}
		t.heading(g.kind, name)
		for _, n := range g.nodes {
			y := t.row()
			t.nameCell(0, n, y)
			t.cell(1, d.lb.kind(n.Kind), false, colText2, y)
			addr := n.Profile.IPAddress
			if addr == "" {
				addr = n.Profile.CIDR
			}
			t.cell(2, addr, true, colText2, y)
			t.cell(3, n.Profile.MACAddress, true, colText3, y)
			t.cell(4, d.m.name(d.m.hostOf[n.ID]), false, colText2, y)
			system := n.Profile.OS
			if model := strings.TrimSpace(n.Profile.Vendor + " " + n.Profile.Model); model != "" {
				system = strings.TrimSpace(system + " · " + model)
				if n.Profile.OS == "" {
					system = model
				}
			}
			t.cell(5, system, false, colText2, y)
			t.cell(6, sourcesText(n), false, colText3, y)
			if n.Power != "" {
				c := cols[7]
				d.powerDot(c.x+2.4, y, 0.8*d.k, n.Power)
				d.fitted(fontRegular, t.fs, colText2, c.x+4.6, y, c.w-5.4, d.lb.power(n.Power), 'L')
			}
		}
	}
}

func (d *doc) drawConnections() {
	idx := d.m.connections()
	if len(idx) == 0 {
		return
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := d.m.edges[idx[i]], d.m.edges[idx[j]]
		if fa, fb := d.m.name(a.From), d.m.name(b.From); fa != fb {
			return fa < fb
		}
		return d.m.name(a.To) < d.m.name(b.To)
	})
	get := d.lb.get
	cols := d.layoutColumns([]string{
		get("desktop.infra_pdf_col_from"), get("desktop.infra_pdf_col_port"), get("desktop.infra_pdf_col_to"),
		get("desktop.infra_pdf_col_port"), get("desktop.infra_pdf_col_kind"), get("desktop.infra_pdf_col_speed"),
		get("desktop.infra_pdf_col_label"),
	}, []float64{22, 10, 22, 10, 15, 9, 12})
	t := d.newTable(get("desktop.infra_pdf_connections_title"), cols)
	for _, i := range idx {
		e := d.m.edges[i]
		y := t.row()
		t.nameCell(0, d.m.node(e.From), y)
		t.cell(1, e.FromPort, true, colText2, y)
		t.nameCell(2, d.m.node(e.To), y)
		t.cell(3, e.ToPort, true, colText2, y)
		st := edgeStyleOf(e.Kind)
		kc := cols[4]
		d.stroke(st.col, max(st.wPt*1.4, 0.6))
		d.dash(st.dash)
		d.pdf.Line(kc.x+1.5, y, kc.x+7.5, y)
		d.dash(nil)
		d.fitted(fontRegular, t.fs, colText2, kc.x+9, y, kc.w-9.8, d.lb.edge(e.Kind), 'L')
		t.cell(5, e.Speed, false, colText2, y)
		t.cell(6, e.Label, false, colText3, y)
	}
}
