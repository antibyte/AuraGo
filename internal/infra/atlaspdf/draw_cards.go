package atlaspdf

import (
	"math"
	"strings"

	"aurago/internal/infra"
)

func tint(c rgb, t float64) rgb { return mix(c, colWhite, t) }

// powerDot paints a status dot with a white ring.
func (d *doc) powerDot(x, y, r float64, power string) {
	if power == "" {
		return
	}
	d.fill(colWhite)
	d.circle(x, y, r+0.4, "F")
	d.fill(powerColor(power))
	d.circle(x, y, r, "F")
}

// subline is the second text line of a card: address, else role or model.
func (d *doc) subline(n *infra.Node) (text string, mono bool) {
	p := n.Profile
	switch {
	case p.IPAddress != "":
		return p.IPAddress, true
	case n.Kind == infra.NodeNetwork && p.CIDR != "":
		return p.CIDR, true
	case p.Role != "":
		return p.Role, false
	case p.Model != "":
		return strings.TrimSpace(p.Vendor + " " + p.Model), false
	}
	return d.lb.kind(n.Kind), false
}

type guestCount struct {
	kind infra.NodeKind
	n    int
}

// guestCounts tallies a card's guests per kind in vm, container, service order.
func (d *doc) guestCounts(c card) []guestCount {
	counts := map[infra.NodeKind]int{}
	for _, id := range c.Guests {
		counts[d.m.node(id).Kind]++
	}
	var out []guestCount
	for _, k := range []infra.NodeKind{infra.NodeVM, infra.NodeContainer, infra.NodeService} {
		if counts[k] > 0 {
			out = append(out, guestCount{k, counts[k]})
		}
	}
	return out
}

func (d *doc) guestLabel(k infra.NodeKind) string {
	switch k {
	case infra.NodeVM:
		return d.lb.get("desktop.infra_pdf_stat_vms")
	case infra.NodeContainer:
		return d.lb.get("desktop.infra_pdf_stat_containers")
	}
	return d.lb.get("desktop.infra_pdf_stat_services")
}

// cardFull draws the readable card into the millimetre rectangle r.
func (d *doc) cardFull(c card, r rect) {
	n := d.m.node(c.ID)
	kc := kindColor(n.Kind)
	rad := math.Min(2, r.H/3)
	d.dash(nil)
	d.fill(colShadow)
	d.rrect(r.X+0.5, r.Y+0.5, r.W, r.H, rad, "F")
	d.fill(colWhite)
	d.stroke(colHair, 0.35)
	d.rrect(r.X, r.Y, r.W, r.H, rad, "FD")
	bar := math.Min(1.4, r.W*0.03)
	d.clipRound(r.X, r.Y, r.W, r.H, rad)
	d.fill(kc)
	d.pdf.Rect(r.X, r.Y, bar, r.H, "F")
	d.unclip()

	isz := 0.7 * r.H
	ix, iy := r.X+bar+(r.H-isz)/2, r.Y+(r.H-isz)/2
	d.drawIcon(n, ix, iy, isz)
	x0 := ix + isz + 0.12*r.H
	xr := r.right() - math.Max(1.2, 0.14*r.H)
	nameRight := xr
	if n.Power != "" {
		d.powerDot(r.right()-1.9, r.Y+1.9, 0.8, n.Power)
		nameRight -= 2.6
	}

	counts := d.guestCounts(c)
	lineH := r.H / 3.6
	fsName := clamp(lineH/ptMM*0.85, 5, 13)
	fsSub := clamp(lineH/ptMM*0.72, 4.2, 10.5)
	// a long name first shrinks to 85 %, then wraps onto a second line when no guest pill needs the room
	nameLines := []string{n.Name}
	if avail := nameRight - x0; d.width(fontBold, fsName, n.Name) > avail {
		fsName *= 0.85
		if d.width(fontBold, fsName, n.Name) > avail && len(counts) == 0 {
			nameLines = d.wrapName(n.Name, fsName, avail)
		}
	}
	slots := 2.0
	if len(counts) > 0 || len(nameLines) > 1 {
		slots = 3
	}
	top := r.Y + (r.H-slots*lineH)/2
	for i, line := range nameLines {
		d.fitted(fontBold, fsName, colInk, x0, top+lineH*(0.5+float64(i)), nameRight-x0, line, 'L')
	}
	subSlot := float64(len(nameLines)) + 0.5
	sub, mono := d.subline(n)
	if mono {
		d.fitted(fontMono, fsSub*0.95, colText2, x0, top+lineH*subSlot, xr-x0, sub, 'L')
	} else {
		d.fitted(fontRegular, fsSub, colText3, x0, top+lineH*subSlot, xr-x0, sub, 'L')
	}
	if len(counts) > 0 {
		d.guestPills(counts, x0, top+lineH*2.5, xr-x0, lineH*0.92, fsSub*0.9)
	}
}

// wrapName splits name at the last space that keeps the first line within maxW;
// a name without a usable space stays on one line.
func (d *doc) wrapName(name string, fs, maxW float64) []string {
	cut := -1
	for i, r := range name {
		if r == ' ' && d.width(fontBold, fs, name[:i]) <= maxW {
			cut = i
		}
	}
	if cut <= 0 {
		return []string{name}
	}
	return []string{name[:cut], name[cut+1:]}
}

// guestPills draws "12 Containers" style pills, or one combined pill if they do not fit.
func (d *doc) guestPills(counts []guestCount, x, yMid, maxW, h, fs float64) {
	texts := make([]string, len(counts))
	total := 0.0
	total2 := 0
	for i, g := range counts {
		label := d.guestLabel(g.kind)
		if g.n == 1 {
			label = d.lb.kind(g.kind)
		}
		texts[i] = itoa(g.n) + " " + label
		total += d.width(fontMedium, fs, texts[i]) + h*0.9 + 0.8
		total2 += g.n
	}
	limit := x + maxW
	if total-0.8 > maxW {
		texts = []string{d.lb.guests(total2)}
		counts = []guestCount{{infra.NodeDevice, total2}}
	}
	for i, g := range counts {
		kc := kindColor(g.kind)
		w := math.Min(d.width(fontMedium, fs, texts[i])+h*0.9, maxW)
		d.fill(tint(kc, 0.86))
		d.rrect(x, yMid-h/2, w, h, h/2, "F")
		d.fitted(fontMedium, fs, mix(kc, colInk, 0.25), x+w/2, yMid, w-h*0.7, texts[i], 'C')
		x += w + 0.8
		if x >= limit {
			break
		}
	}
}

// cardCompact is the overview form: icon disc and a one-line name.
func (d *doc) cardCompact(c card, r rect) {
	n := d.m.node(c.ID)
	kc := kindColor(n.Kind)
	h := r.H
	d.dash(nil)
	d.fill(colWhite)
	d.stroke(colHair, 0.3)
	d.rrect(r.X, r.Y, r.W, h, math.Min(h/2, 2.4), "FD")
	d.fill(tint(kc, 0.82))
	d.circle(r.X+h/2, r.Y+h/2, h*0.4, "F")
	isz := h * 0.62
	d.drawIcon(n, r.X+h/2-isz/2, r.Y+h/2-isz/2, isz)
	pt := clamp(h*0.38/ptMM, 5, 9)
	right := r.right() - 1
	if n.Power != "" {
		right -= 1.2
	}
	d.fitted(fontBold, pt, colInk, r.X+h+0.3, r.Y+h/2, right-(r.X+h+0.3), n.Name, 'L')
	d.powerDot(r.right()-1.1, r.Y+1.1, clamp(h*0.08, 0.45, 0.8), n.Power)
}

// cardDot is the form below the compact size: a coloured dot with the name when it fits;
// it reports whether only the dot was drawn.
func (d *doc) cardDot(c card, r rect) (dotOnly bool) {
	n := d.m.node(c.ID)
	kc := kindColor(n.Kind)
	ctr := r.center()
	const fs = 5.0
	h := clamp(r.H, 3, 4.4)
	tw := d.width(fontMedium, fs, n.Name)
	d.dash(nil)
	if h+tw+1.4 <= r.W+2 {
		w := h + tw + 1.4
		d.fill(colWhite)
		d.stroke(tint(kc, 0.4), 0.3)
		d.rrect(ctr.X-w/2, ctr.Y-h/2, w, h, h/2, "FD")
		d.fill(kc)
		d.circle(ctr.X-w/2+h/2, ctr.Y, h*0.27, "F")
		d.text(fontMedium, fs, colInk, ctr.X-w/2+h+0.2, ctr.Y, n.Name, 'L')
		d.powerDot(ctr.X+w/2-0.6, ctr.Y-h/2+0.3, 0.45, n.Power)
		return false
	}
	rad := clamp(r.H*0.35, 0.8, 1.6)
	d.fill(colWhite)
	d.circle(ctr.X, ctr.Y, rad+0.3, "F")
	d.fill(kc)
	d.circle(ctr.X, ctr.Y, rad, "F")
	return true
}
