package atlaspdf

import (
	"math"
	"sort"

	"aurago/internal/infra"
)

// World units are the Infra map's canvas pixels (port of infra-model.js buildScene).
const (
	cardW, cardH       = 216.0, 64.0
	regionDefW         = 360.0
	regionDefH         = 240.0
	regionPad          = 48.0
	coordLimit         = 1e6
	unplacedCellW      = 240.0
	unplacedCellH      = 96.0
	unplacedGap        = 160.0
	unplacedID         = "\x00unplaced"
	unplacedPageAspect = 1.45
)

type pt struct{ X, Y float64 }

type rect struct{ X, Y, W, H float64 }

func (r rect) right() float64  { return r.X + r.W }
func (r rect) bottom() float64 { return r.Y + r.H }
func (r rect) center() pt      { return pt{r.X + r.W/2, r.Y + r.H/2} }
func (r rect) pad(p float64) rect {
	return rect{r.X - p, r.Y - p, r.W + 2*p, r.H + 2*p}
}
func (r rect) intersects(o rect) bool {
	return r.X < o.right() && r.right() > o.X && r.Y < o.bottom() && r.bottom() > o.Y
}
func (r rect) contains(p pt) bool {
	return p.X >= r.X && p.X <= r.right() && p.Y >= r.Y && p.Y <= r.bottom()
}

func unionRects(rs ...rect) (rect, bool) {
	if len(rs) == 0 {
		return rect{}, false
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, r := range rs {
		minX, minY = math.Min(minX, r.X), math.Min(minY, r.Y)
		maxX, maxY = math.Max(maxX, r.right()), math.Max(maxY, r.bottom())
	}
	return rect{minX, minY, maxX - minX, maxY - minY}, true
}

func cardRectAt(x, y float64) rect { return rect{x - cardW/2, y - cardH/2, cardW, cardH} }

func finite(v float64) bool { return !math.IsNaN(v) && math.Abs(v) <= coordLimit }

type card struct {
	ID       string
	X, Y     float64 // centre
	Guests   []string
	Unplaced bool
}

func (c card) rect() rect { return cardRectAt(c.X, c.Y) }

type region struct {
	ID     string
	Rect   rect
	Depth  int
	Count  int
	Pseudo bool
}

// line is one drawn connection between two shapes (cards or regions).
type line struct {
	Edge           int
	Kind           infra.EdgeKind
	From, To       string
	P0, C0, C1, P1 pt
	Curve          bool
	Mid            pt
}

type scene struct {
	m         *model
	cards     []card
	cardIdx   map[string]int
	regions   []region
	regionIdx map[string]int
	lines     []line
	bounds    rect
	hasBounds bool
}

type shape struct{ c, half pt }

func (sc *scene) shape(id string) (shape, bool) {
	if i, ok := sc.cardIdx[id]; ok {
		c := sc.cards[i]
		return shape{pt{c.X, c.Y}, pt{cardW / 2, cardH / 2}}, true
	}
	if i, ok := sc.regionIdx[id]; ok {
		r := sc.regions[i].Rect
		return shape{r.center(), pt{r.W / 2, r.H / 2}}, true
	}
	return shape{}, false
}

// shapeName is the display name of a card or region.
func (sc *scene) shapeName(id string) string {
	if id == unplacedID {
		return ""
	}
	return sc.m.name(id)
}

func (sc *scene) shapeRect(id string) (rect, bool) {
	if i, ok := sc.cardIdx[id]; ok {
		return sc.cards[i].rect(), true
	}
	if i, ok := sc.regionIdx[id]; ok {
		return sc.regions[i].Rect, true
	}
	return rect{}, false
}

// atlasPlacement returns the usable atlas placement of n.
func atlasPlacement(n *infra.Node) (infra.Placement, bool) {
	p, ok := n.Placements[infra.ViewAtlas]
	if !ok || !finite(p.X) || !finite(p.Y) {
		return p, false
	}
	return p, true
}

func buildScene(m *model, unplacedTitle string) *scene {
	sc := &scene{m: m, cardIdx: map[string]int{}, regionIdx: map[string]int{}}
	var placeIDs []string
	for i := range m.nodes {
		n := &m.nodes[i]
		if _, guest := m.hostOf[n.ID]; guest {
			continue
		}
		p, ok := atlasPlacement(n)
		if !ok {
			continue
		}
		if isPlace(n.Kind) {
			placeIDs = append(placeIDs, n.ID)
			continue
		}
		sc.addCard(card{ID: n.ID, X: p.X, Y: p.Y, Guests: m.allGuests(n.ID)})
	}
	cardRects := map[string][]rect{}
	for _, c := range sc.cards {
		if parent := m.parent[c.ID]; parent != "" {
			cardRects[parent] = append(cardRects[parent], c.rect())
		}
	}
	sc.layoutRegions(placeIDs, cardRects)
	sc.countRegions()
	sc.addUnplaced()
	sc.buildLines()
	return sc
}

func (sc *scene) addCard(c card) {
	sc.cardIdx[c.ID] = len(sc.cards)
	sc.cards = append(sc.cards, c)
}

// layoutRegions ports regionLayout: places are drawn bottom-up so nested places
// grow their parents. Parent cycles are cut where they close.
func (sc *scene) layoutRegions(placeIDs []string, cardRects map[string][]rect) {
	ids := map[string]bool{}
	for _, id := range placeIDs {
		ids[id] = true
	}
	nested := map[string][]string{}
	for _, id := range placeIDs {
		if parent := sc.m.parent[id]; parent != id && ids[parent] {
			nested[parent] = append(nested[parent], id)
		}
	}
	rects := map[string]rect{}
	visiting := map[string]bool{}
	var visit func(id string) (rect, bool)
	visit = func(id string) (rect, bool) {
		if r, ok := rects[id]; ok {
			return r, true
		}
		if visiting[id] {
			return rect{}, false
		}
		visiting[id] = true
		children := append([]rect(nil), cardRects[id]...)
		for _, inner := range nested[id] {
			if r, ok := visit(inner); ok {
				children = append(children, r)
			}
		}
		visiting[id] = false
		p, _ := atlasPlacement(sc.m.node(id))
		r := regionRect(p, children)
		rects[id] = r
		return r, true
	}
	for _, id := range placeIDs {
		visit(id)
	}
	for _, id := range placeIDs {
		sc.regions = append(sc.regions, region{ID: id, Rect: rects[id], Depth: sc.regionDepth(id, ids)})
	}
	sort.SliceStable(sc.regions, func(i, j int) bool {
		if sc.regions[i].Depth != sc.regions[j].Depth {
			return sc.regions[i].Depth < sc.regions[j].Depth
		}
		return sc.regions[i].ID < sc.regions[j].ID
	})
	for i, r := range sc.regions {
		sc.regionIdx[r.ID] = i
	}
}

func (sc *scene) regionDepth(id string, ids map[string]bool) int {
	level := 0
	seen := map[string]bool{id: true}
	for p := sc.m.parent[id]; ids[p] && !seen[p]; p = sc.m.parent[p] {
		seen[p] = true
		level++
	}
	return level
}

// regionRect grows the stored rectangle to hold every child plus the frame padding.
func regionRect(p infra.Placement, children []rect) rect {
	w, h := p.W, p.H
	if !finite(w) || w <= 0 {
		w = regionDefW
	}
	if !finite(h) || h <= 0 {
		h = regionDefH
	}
	all := []rect{{p.X, p.Y, w, h}}
	for _, c := range children {
		all = append(all, c.pad(regionPad))
	}
	r, _ := unionRects(all...)
	return r
}

// countRegions counts the assets (cards plus their guests) inside each region.
func (sc *scene) countRegions() {
	for _, c := range sc.cards {
		seen := map[string]bool{}
		for p := sc.m.parent[c.ID]; p != "" && !seen[p]; p = sc.m.parent[p] {
			seen[p] = true
			if i, ok := sc.regionIdx[p]; ok {
				sc.regions[i].Count += 1 + len(c.Guests)
			}
		}
	}
}

// addUnplaced appends the "Not on the map" block: top-level assets without an
// atlas placement, below everything else (or as the whole scene when nothing is placed).
func (sc *scene) addUnplaced() {
	var ids []string
	for i := range sc.m.nodes {
		n := &sc.m.nodes[i]
		if _, guest := sc.m.hostOf[n.ID]; guest || isPlace(n.Kind) {
			continue
		}
		if _, ok := atlasPlacement(n); ok {
			continue
		}
		if n.Kind == infra.NodeVM || n.Kind == infra.NodeContainer {
			continue
		}
		ids = append(ids, n.ID)
	}
	sc.recomputeBounds()
	if len(ids) == 0 {
		return
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := sc.m.node(ids[i]), sc.m.node(ids[j])
		if kindOrder[a.Kind] != kindOrder[b.Kind] {
			return kindOrder[a.Kind] < kindOrder[b.Kind]
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	framed := sc.hasBounds
	cols := int(math.Ceil(math.Sqrt(float64(len(ids)) * unplacedPageAspect * unplacedCellH / unplacedCellW)))
	cols = max(1, min(cols, len(ids)))
	if len(ids) >= 3 {
		cols = max(cols, 3)
	}
	var ox, oy float64
	if framed {
		ox, oy = sc.bounds.X+regionPad, sc.bounds.bottom()+unplacedGap+regionPad
	}
	col, row := 0, 0
	var prev infra.NodeKind
	for i, id := range ids {
		n := sc.m.node(id)
		if !framed && i > 0 && n.Kind != prev {
			// every kind group starts on a fresh row
			if col != 0 {
				col, row = 0, row+1
			}
		}
		prev = n.Kind
		sc.addCard(card{
			ID: id, Unplaced: true, Guests: sc.m.allGuests(id),
			X: ox + float64(col)*unplacedCellW + unplacedCellW/2,
			Y: oy + float64(row)*unplacedCellH + unplacedCellH/2,
		})
		if col++; col == cols {
			col, row = 0, row+1
		}
	}
	if !framed {
		sc.recomputeBounds()
		return
	}
	rows := row
	if col != 0 {
		rows++
	}
	frame := rect{ox - regionPad, oy - regionPad, float64(cols)*unplacedCellW + 2*regionPad, float64(rows)*unplacedCellH + 2*regionPad}
	sc.regionIdx[unplacedID] = len(sc.regions)
	sc.regions = append(sc.regions, region{ID: unplacedID, Rect: frame, Pseudo: true, Count: len(ids)})
	sc.recomputeBounds()
}

func (sc *scene) recomputeBounds() {
	var all []rect
	for _, c := range sc.cards {
		all = append(all, c.rect())
	}
	for _, r := range sc.regions {
		all = append(all, r.Rect)
	}
	sc.bounds, sc.hasBounds = unionRects(all...)
}

// endpoint maps an edge end to the shape that stands for it: itself when drawn,
// else its (nested) host's card.
func (sc *scene) endpoint(id string) string {
	if _, ok := sc.cardIdx[id]; ok {
		return id
	}
	if _, ok := sc.regionIdx[id]; ok {
		return id
	}
	if root := sc.m.rootHost(id); root != id {
		if _, ok := sc.cardIdx[root]; ok {
			return root
		}
	}
	return ""
}

func (sc *scene) buildLines() {
	for i, e := range sc.m.edges {
		if e.Kind == infra.EdgeRunsOn && sc.m.hostOf[e.From] == e.To {
			continue
		}
		from, to := sc.endpoint(e.From), sc.endpoint(e.To)
		if from == "" || to == "" || from == to {
			continue
		}
		a, _ := sc.shape(from)
		b, _ := sc.shape(to)
		l := edgeCurve(a, b)
		l.Edge, l.Kind, l.From, l.To = i, e.Kind, from, to
		sc.lines = append(sc.lines, l)
	}
}

// borderPoint is where the ray from from towards to leaves a half-size box around from.
func borderPoint(from, to, half pt) pt {
	dx, dy := to.X-from.X, to.Y-from.Y
	if dx == 0 && dy == 0 {
		return from
	}
	k := math.Inf(1)
	if dx != 0 {
		k = half.X / math.Abs(dx)
	}
	if dy != 0 {
		k = math.Min(k, half.Y/math.Abs(dy))
	}
	return pt{from.X + dx*k, from.Y + dy*k}
}

// edgeCurve ports the app's edgeCurve: a cubic that leaves the facing sides of
// both shapes along the wider gap, or a straight border-to-border segment when
// the shapes overlap on both axes.
func edgeCurve(a, b shape) line {
	gapX := math.Abs(b.c.X-a.c.X) - a.half.X - b.half.X
	gapY := math.Abs(b.c.Y-a.c.Y) - a.half.Y - b.half.Y
	if gapX < 0 && gapY < 0 {
		start, end := borderPoint(a.c, b.c, a.half), borderPoint(b.c, a.c, b.half)
		if (end.X-start.X)*(b.c.X-a.c.X)+(end.Y-start.Y)*(b.c.Y-a.c.Y) <= 0 {
			start, end = a.c, b.c
		}
		return line{P0: start, P1: end, Mid: pt{(start.X + end.X) / 2, (start.Y + end.Y) / 2}}
	}
	across := gapX >= gapY
	sign := func(v float64) float64 {
		if v < 0 {
			return -1
		}
		return 1
	}
	var s float64
	var start, end pt
	if across {
		s = sign(b.c.X - a.c.X)
		start, end = pt{a.c.X + s*a.half.X, a.c.Y}, pt{b.c.X - s*b.half.X, b.c.Y}
	} else {
		s = sign(b.c.Y - a.c.Y)
		start, end = pt{a.c.X, a.c.Y + s*a.half.Y}, pt{b.c.X, b.c.Y - s*b.half.Y}
	}
	gap := math.Max(gapX, gapY)
	c := s * math.Min(240, math.Max(gap/2, math.Min(48, gap)))
	c0, c1 := start, end
	if across {
		c0.X, c1.X = start.X+c, end.X-c
	} else {
		c0.Y, c1.Y = start.Y+c, end.Y-c
	}
	return line{P0: start, C0: c0, C1: c1, P1: end, Curve: true, Mid: pt{(start.X + end.X) / 2, (start.Y + end.Y) / 2}}
}

// bbox is the control-polygon bounding box of the line.
func (l line) bbox() rect {
	r, _ := unionRects(rect{l.P0.X, l.P0.Y, 0, 0}, rect{l.P1.X, l.P1.Y, 0, 0})
	if l.Curve {
		r, _ = unionRects(r, rect{l.C0.X, l.C0.Y, 0, 0}, rect{l.C1.X, l.C1.Y, 0, 0})
	}
	return r
}
