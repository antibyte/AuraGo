package atlaspdf

import (
	"math"
	"sort"
	"strings"

	"aurago/internal/infra"
)

// hostBlock is one card (or a continuation chunk of it) on a hosts page.
type hostBlock struct {
	host     *infra.Node // nil for the "without host" card
	guests   []string
	total    int
	sub      int // guest sub-columns
	first    bool
	h        float64
	hostless bool
}

type hostPlacement struct {
	b    hostBlock
	x, y float64
}

type hostPage struct {
	colY  []float64
	items []hostPlacement
}

const hostGutter = 6.0

// hostLayout holds the sizes of a hosts page, scaled to the column width.
type hostLayout struct {
	cols               int
	colW, hk           float64
	head, cont, hless  float64
	rowH, padTop, padB float64
}

func (d *doc) hostLayout() hostLayout {
	cols := 3
	if d.k > 1.3 {
		cols = 4
	}
	colW := (d.content.W - float64(cols-1)*hostGutter) / float64(cols)
	hk := colW / 87
	return hostLayout{cols, colW, hk, 21 * hk, 8 * hk, 9 * hk, 5 * hk, 1.2 * hk, 2 * hk}
}

func (d *doc) hostBlocks(l hostLayout) []hostBlock {
	var hosts []*infra.Node
	for i := range d.m.nodes {
		n := &d.m.nodes[i]
		if _, guest := d.m.hostOf[n.ID]; !guest && len(d.m.guestsOf[n.ID]) > 0 {
			hosts = append(hosts, n)
		}
	}
	sort.Slice(hosts, func(i, j int) bool {
		gi, gj := len(d.m.allGuests(hosts[i].ID)), len(d.m.allGuests(hosts[j].ID))
		if gi != gj {
			return gi > gj
		}
		if hosts[i].Name != hosts[j].Name {
			return hosts[i].Name < hosts[j].Name
		}
		return hosts[i].ID < hosts[j].ID
	})
	var blocks []hostBlock
	for _, h := range hosts {
		blocks = append(blocks, d.chunk(l, h, d.m.allGuests(h.ID), false)...)
	}
	var orphans []string
	for _, n := range d.m.nodes {
		if _, guest := d.m.hostOf[n.ID]; !guest && (n.Kind == infra.NodeVM || n.Kind == infra.NodeContainer) {
			orphans = append(orphans, n.ID)
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return d.m.lessAsset(orphans[i], orphans[j]) })
	if len(orphans) > 0 {
		blocks = append(blocks, d.chunk(l, nil, orphans, true)...)
	}
	return blocks
}

// chunk splits a host's guests into blocks no taller than a column.
func (d *doc) chunk(l hostLayout, host *infra.Node, guests []string, hostless bool) []hostBlock {
	sub := 1
	if len(guests) > 8 {
		sub = 2
	}
	var out []hostBlock
	for off := 0; off < len(guests) || off == 0; {
		head := l.cont
		if off == 0 {
			head = l.head
			if hostless {
				head = l.hless
			}
		}
		rows := max(1, int((d.content.H-head-l.padTop-l.padB)/l.rowH))
		take := min(rows*sub, len(guests)-off)
		out = append(out, hostBlock{
			host: host, guests: guests[off : off+take], total: len(guests), sub: sub, first: off == 0, hostless: hostless,
			h: head + l.padTop + math.Ceil(float64(take)/float64(sub))*l.rowH + l.padB,
		})
		off += take
		if take == 0 {
			break
		}
	}
	return out
}

func (d *doc) drawHosts() {
	l := d.hostLayout()
	blocks := d.hostBlocks(l)
	if len(blocks) == 0 {
		return
	}
	var pages []*hostPage
	bottom := d.content.bottom()
	for _, b := range blocks {
		placed := false
		for _, pg := range pages {
			best := -1
			for c, y := range pg.colY {
				if y+b.h <= bottom+0.01 && (best < 0 || y < pg.colY[best]) {
					best = c
				}
			}
			if best >= 0 {
				pg.put(b, best, l, d.content)
				placed = true
				break
			}
		}
		if !placed {
			pg := &hostPage{colY: make([]float64, l.cols)}
			for c := range pg.colY {
				pg.colY[c] = d.content.Y
			}
			pages = append(pages, pg)
			pg.put(b, 0, l, d.content)
		}
	}
	title := d.lb.get("desktop.infra_pdf_hosts_title")
	for _, pg := range pages {
		d.newPage(title)
		for _, it := range pg.items {
			d.drawHostBlock(it.b, it.x, it.y, l)
		}
	}
}

func (pg *hostPage) put(b hostBlock, col int, l hostLayout, area rect) {
	x := area.X + float64(col)*(l.colW+hostGutter)
	pg.items = append(pg.items, hostPlacement{b, x, pg.colY[col]})
	pg.colY[col] += b.h + 4
}

func (d *doc) placeSep() string {
	if d.textCov.has('›') {
		return " › "
	}
	return " > "
}

func (d *doc) drawHostBlock(b hostBlock, x, y float64, l hostLayout) {
	hk, w := l.hk, l.colW
	d.dash(nil)
	d.fill(colShadow)
	d.rrect(x+0.5, y+0.5, w, b.h, 2.5, "F")
	d.fill(colWhite)
	d.stroke(colHair, 0.35)
	d.rrect(x, y, w, b.h, 2.5, "FD")
	head := l.cont
	switch {
	case b.hostless && b.first:
		head = l.hless
		d.drawIconKind(d.icons.register("vm", "generic-device"), infra.NodeVM, x+2.2, y+(head-6*hk)/2, 6*hk)
		d.text(fontBold, 9*hk, colInk, x+10*hk, y+head/2, d.lb.get("desktop.infra_pdf_hostless"), 'L')
		d.badge(itoa(b.total), x+w-2.2, y+head/2, hk, kindColor(infra.NodeVM))
	case b.first:
		head = l.head
		d.hostHeader(b, x, y, l)
	default:
		name := ""
		if b.host != nil {
			name = b.host.Name
		} else {
			name = d.lb.get("desktop.infra_pdf_hostless")
		}
		cont := "(" + d.lb.get("desktop.infra_pdf_continued") + ")"
		cw := d.width(fontRegular, 6.5*hk, cont)
		d.text(fontRegular, 6.5*hk, colText3, x+w-2.2, y+head/2, cont, 'R')
		d.fitted(fontBold, 8*hk, colInk, x+2.2, y+head/2, w-6.4-cw, name, 'L')
	}
	d.stroke(colHair, 0.3)
	d.pdf.Line(x+2, y+head-0.4*hk, x+w-2, y+head-0.4*hk)
	rowsN := int(math.Ceil(float64(len(b.guests)) / float64(b.sub)))
	subW := (w - 4.4) / float64(b.sub)
	gap := 0.0
	if b.sub == 2 {
		gap = 3
	}
	for i, id := range b.guests {
		col, row := 0, i
		if b.sub == 2 {
			col, row = i/rowsN, i%rowsN
		}
		d.guestRow(d.m.node(id), x+2.2+float64(col)*subW, y+head+l.padTop+(float64(row)+0.5)*l.rowH, subW-gap, hk)
	}
}

func (d *doc) badge(text string, right, yMid, hk float64, col rgb) {
	const base = 5.8
	fs := base * hk
	h := 3.8 * hk
	w := d.width(fontMedium, fs, text) + 3*hk
	d.fill(tint(col, 0.86))
	d.rrect(right-w, yMid-h/2, w, h, h/2, "F")
	d.text(fontMedium, fs, mix(col, colInk, 0.25), right-w/2, yMid, text, 'C')
}

func (d *doc) hostHeader(b hostBlock, x, y float64, l hostLayout) {
	hk, w := l.hk, l.colW
	n := b.host
	d.drawIcon(n, x+2.2, y+2.5*hk, 9*hk)
	tx := x + 13*hk
	badgeText := itoa(b.total)
	bw := d.width(fontMedium, 5.8*hk, badgeText) + 3*hk
	d.badge(badgeText, x+w-2.2, y+5.2*hk, hk, kindColor(n.Kind))
	d.fitted(fontBold, 10*hk, colInk, tx, y+5.2*hk, w-(tx-x)-bw-4, n.Name, 'L')
	p := n.Profile
	line2 := y + 10.2*hk
	right := x + w - 2.2
	ipW := 0.0
	if p.IPAddress != "" {
		ipW = d.width(fontMono, 6.6*hk, p.IPAddress) + 2.5
		d.fitted(fontMono, 6.6*hk, colText2, tx, line2, right-tx, p.IPAddress, 'L')
	}
	if p.OS != "" {
		d.fitted(fontRegular, 6.4*hk, colText2, tx+ipW, line2, right-tx-ipW, p.OS, 'L')
	}
	var hw []string
	for _, s := range []string{p.CPU, p.RAM, p.Storage} {
		if s != "" {
			hw = append(hw, s)
		}
	}
	if len(hw) == 0 && (p.Vendor != "" || p.Model != "") {
		hw = append(hw, strings.TrimSpace(p.Vendor+" "+p.Model))
	}
	d.fitted(fontRegular, 6.2*hk, colText3, tx, y+14.2*hk, right-tx, strings.Join(hw, " · "), 'L')
	if path := d.m.placePath(n.ID); len(path) > 0 {
		d.fitted(fontRegular, 6*hk, colText3, tx, y+17.9*hk, right-tx, strings.Join(path, d.placeSep()), 'L')
	}
}

// guestDetail is the short right-hand text of a guest row.
func guestDetail(n *infra.Node) string {
	if n.Profile.IPAddress != "" {
		return n.Profile.IPAddress
	}
	if n.Profile.Role != "" {
		return n.Profile.Role
	}
	return ""
}

func (d *doc) guestRow(n *infra.Node, x, yMid, w, hk float64) {
	kc := kindColor(n.Kind)
	d.fill(powerColor(n.Power))
	if n.Power == "" {
		d.fill(colHair)
	}
	d.circle(x+1, yMid, 0.75*hk, "F")
	d.drawIcon(n, x+2.6*hk+0.6, yMid-1.9*hk, 3.8*hk)
	nx := x + 7.8*hk
	detail := guestDetail(n)
	dw := 0.0
	if detail != "" {
		dw = d.width(fontMono, 5.6*hk, detail)
		if w-7.8*hk-dw-2 < 12*hk {
			detail, dw = "", 0
		}
	}
	if detail != "" {
		d.text(fontMono, 5.6*hk, mix(kc, colInk, 0.15), x+w-1, yMid, detail, 'R')
	}
	d.fitted(fontRegular, 6.8*hk, colInk, nx, yMid, w-(nx-x)-dw-3, n.Name, 'L')
}
