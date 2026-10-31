package atlaspdf

import (
	"math"
	"sort"
	"strings"
)

// Detail pages show "islands": parts of the map cut at place boundaries, drawn at
// one shared scale and packed several to a page.
const (
	islandCaptionH = 5.0 // mm above each island
	packGutter     = 8.0 // mm between islands
	loosePad       = 12.0
	maxDetailScale = 1.3 * readableScale
)

// island is a rectangle of the world with the cards and place frames drawn in it.
type island struct {
	Caption     string
	Top         int
	TopName     string
	Box         rect
	Cards       []int
	Regions     []string
	Part, Parts int
	Assets      int
	cards       map[string]bool
	regions     map[string]bool
}

func (is *island) has(id string) bool { return is.cards[id] || is.regions[id] }

// placement puts an island on a page; X, Y are the top-left of its caption row (mm).
type placement struct {
	Island int
	X, Y   float64
}

// boxRect is the millimetre rectangle of the island's world box.
func (pl placement) boxRect(is *island, s float64) rect {
	return rect{pl.X, pl.Y + islandCaptionH, is.Box.W * s, is.Box.H * s}
}

func (d *doc) locatorSize() (float64, float64) { return 52 * d.k, 34 * d.k }

// boxLimits are the largest island boxes (mm, without caption) a page takes with
// the locator inset reserved: wide and short (a) or narrow and tall (b).
func (d *doc) boxLimits() (a, b [2]float64) {
	lw, lh := d.locatorSize()
	c := d.content
	return [2]float64{c.W, c.H - islandCaptionH - lh - packGutter}, [2]float64{c.W - lw - packGutter, c.H - islandCaptionH}
}

// fitScale is the largest scale at which box fits on a page.
func (d *doc) fitScale(box rect) float64 {
	a, b := d.boxLimits()
	w, h := math.Max(box.W, 1), math.Max(box.H, 1)
	return math.Max(math.Min(a[0]/w, a[1]/h), math.Min(b[0]/w, b[1]/h))
}

func (d *doc) fitsAt(box rect, s float64) bool { return d.fitScale(box) >= s*(1-1e-9) }

func (d *doc) newIsland(caption string, top int, topName string, box rect, cards []int, regs []string) island {
	is := island{Caption: caption, Top: top, TopName: topName, Box: box, Cards: cards, Regions: regs, cards: map[string]bool{}, regions: map[string]bool{}}
	for _, ci := range cards {
		c := d.sc.cards[ci]
		is.cards[c.ID] = true
		is.Assets += 1 + len(c.Guests)
	}
	for _, r := range regs {
		is.regions[r] = true
	}
	return is
}

type topCluster struct {
	name   string
	region string // "" for loose cards
	cards  []int
	box    rect
}

type islandPlanner struct {
	d      *doc
	kids   map[string][]string
	direct map[string][]int
}

func (d *doc) newIslandPlanner() *islandPlanner {
	pl := &islandPlanner{d: d, kids: map[string][]string{}, direct: map[string][]int{}}
	sc := d.sc
	placed := func(id string) bool {
		i, ok := sc.regionIdx[id]
		return ok && !sc.regions[i].Pseudo
	}
	for _, rg := range sc.regions {
		if rg.Pseudo {
			continue
		}
		if p := d.m.parent[rg.ID]; p != rg.ID && placed(p) {
			pl.kids[p] = append(pl.kids[p], rg.ID)
		}
	}
	for _, kids := range pl.kids {
		sort.Slice(kids, func(i, j int) bool {
			a, b := sc.regions[sc.regionIdx[kids[i]]].Rect, sc.regions[sc.regionIdx[kids[j]]].Rect
			if a.Y != b.Y {
				return a.Y < b.Y
			}
			return a.X < b.X
		})
	}
	for ci, c := range sc.cards {
		if p := d.m.parent[c.ID]; !c.Unplaced && placed(p) {
			pl.direct[p] = append(pl.direct[p], ci)
		}
	}
	return pl
}

// subtree lists the cards and place frames below (and including) region rid.
func (pl *islandPlanner) subtree(rid string) (cards []int, regs []string) {
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		regs = append(regs, id)
		cards = append(cards, pl.direct[id]...)
		for _, k := range pl.kids[id] {
			walk(k)
		}
	}
	walk(rid)
	return cards, regs
}

func (pl *islandPlanner) subtreeBox(rid string, cards []int) rect {
	rs := []rect{pl.d.sc.regions[pl.d.sc.regionIdx[rid]].Rect}
	for _, ci := range cards {
		rs = append(rs, pl.d.sc.cards[ci].rect())
	}
	box, _ := unionRects(rs...)
	return box
}

// tops lists the top-level clusters in reading order: depth-0 places, the
// unplaced block, and proximity groups of the remaining loose cards.
func (pl *islandPlanner) tops() []topCluster {
	d, sc := pl.d, pl.d.sc
	var out []topCluster
	assigned := map[int]bool{}
	for _, rg := range sc.regions {
		if rg.Depth != 0 {
			continue
		}
		var cards []int
		if rg.Pseudo {
			for ci, c := range sc.cards {
				if c.Unplaced {
					cards = append(cards, ci)
				}
			}
			out = append(out, topCluster{name: d.lb.get("desktop.infra_pdf_unplaced"), region: rg.ID, cards: cards, box: rg.Rect})
		} else {
			cards, _ = pl.subtree(rg.ID)
			out = append(out, topCluster{name: sc.shapeName(rg.ID), region: rg.ID, cards: cards, box: pl.subtreeBox(rg.ID, cards)})
		}
		for _, ci := range cards {
			assigned[ci] = true
		}
	}
	var loose []int
	for ci := range sc.cards {
		if !assigned[ci] {
			loose = append(loose, ci)
		}
	}
	for _, g := range d.proximityGroups(loose) {
		box := d.cardsBox(g).pad(loosePad)
		out = append(out, topCluster{name: d.looseCaption(g), cards: g, box: box})
	}
	sort.SliceStable(out, func(i, j int) bool {
		bi, bj := math.Floor(out[i].box.Y/300), math.Floor(out[j].box.Y/300)
		if bi != bj {
			return bi < bj
		}
		return out[i].box.X < out[j].box.X
	})
	return out
}

func (d *doc) cardsBox(cards []int) rect {
	rs := make([]rect, 0, len(cards))
	for _, ci := range cards {
		rs = append(rs, d.sc.cards[ci].rect())
	}
	box, _ := unionRects(rs...)
	return box
}

// proximityGroups single-link merges cards whose rectangles are at most mergeGap apart.
func (d *doc) proximityGroups(cards []int) [][]int {
	parent := make([]int, len(cards))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for a := range cards {
		ca := d.sc.cards[cards[a]]
		for b := a + 1; b < len(cards); b++ {
			cb := d.sc.cards[cards[b]]
			dx := math.Max(0, math.Abs(ca.X-cb.X)-cardW)
			dy := math.Max(0, math.Abs(ca.Y-cb.Y)-cardH)
			if math.Hypot(dx, dy) <= mergeGap {
				parent[find(a)] = find(b)
			}
		}
	}
	index := map[int]int{}
	var groups [][]int
	for a, ci := range cards {
		root := find(a)
		g, ok := index[root]
		if !ok {
			g = len(groups)
			index[root] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], ci)
	}
	return groups
}

func (d *doc) crumb(parts []string) string { return strings.Join(parts, d.placeSep()) }

// decompose turns region rid into islands that fit at scale s: the region itself
// when it fits (or has no child places), else its child places plus groups of its own cards.
func (pl *islandPlanner) decompose(rid string, crumb []string, top int, topName string, s float64, seen map[string]bool) []island {
	d := pl.d
	cards, regs := pl.subtree(rid)
	whole := d.newIsland(d.crumb(crumb), top, topName, pl.subtreeBox(rid, cards), cards, regs)
	if d.fitsAt(whole.Box, s) || len(pl.kids[rid]) == 0 || seen[rid] {
		return []island{whole}
	}
	seen[rid] = true
	var out []island
	for _, kid := range pl.kids[rid] {
		out = append(out, pl.decompose(kid, append(append([]string(nil), crumb...), d.m.name(kid)), top, topName, s, seen)...)
	}
	for _, g := range d.proximityGroups(pl.direct[rid]) {
		out = append(out, d.newIsland(d.crumb(crumb), top, topName, d.cardsBox(g).pad(loosePad), g, nil))
	}
	return out
}

// islandsAt decomposes every top-level cluster at scale s, tiling islands that
// cannot be split further and still do not fit when tile is set.
func (pl *islandPlanner) islandsAt(tops []topCluster, s float64, tile bool) []island {
	d := pl.d
	var out []island
	for i, t := range tops {
		var part []island
		switch {
		case t.region == "":
			part = []island{d.newIsland(t.name, i, t.name, t.box, t.cards, nil)}
		case t.region == unplacedID:
			part = []island{d.newIsland(t.name, i, t.name, t.box, t.cards, []string{unplacedID})}
		default:
			part = pl.decompose(t.region, []string{t.name}, i, t.name, s, map[string]bool{})
		}
		for _, is := range part {
			if tile {
				out = append(out, d.tileIsland(is, s)...)
			} else {
				out = append(out, is)
			}
		}
	}
	return out
}

// tileIsland cuts an island that does not fit at scale s into overlapping tiles,
// with cuts in the gaps between cards where possible.
func (d *doc) tileIsland(is island, s float64) []island {
	if d.fitsAt(is.Box, s) {
		return []island{is}
	}
	a, _ := d.boxLimits()
	maxW, maxH := a[0]/s*0.995, a[1]/s*0.995
	cols := min(12, max(1, int(math.Ceil(is.Box.W*(1+tileOverlap)/maxW))))
	rows := min(12, max(1, int(math.Ceil(is.Box.H*(1+tileOverlap)/maxH))))
	xs := d.tileSpans(is.Box.X, is.Box.right(), cols, maxW, func(x float64) int { return d.crossing(is.Cards, x, true) })
	ys := d.tileSpans(is.Box.Y, is.Box.bottom(), rows, maxH, func(y float64) int { return d.crossing(is.Cards, y, false) })
	var tiles []island
	for _, y := range ys {
		for _, x := range xs {
			box := rect{x[0], y[0], x[1] - x[0], y[1] - y[0]}
			var inside []int
			for _, ci := range is.Cards {
				if box.intersects(d.sc.cards[ci].rect()) {
					inside = append(inside, ci)
				}
			}
			if len(is.Cards) > 0 && len(inside) == 0 {
				continue
			}
			t := d.newIsland(is.Caption, is.Top, is.TopName, box, inside, nil)
			t.regions = is.regions
			t.Regions = is.Regions
			tiles = append(tiles, t)
		}
	}
	if len(tiles) == 0 {
		return []island{is}
	}
	for i := range tiles {
		tiles[i].Part, tiles[i].Parts = i+1, len(tiles)
	}
	return tiles
}

// tileSpans cuts [lo, hi] into n overlapping spans no longer than maxLen. Each
// interior cut moves within the slack the length limit leaves to the position
// that crosses the fewest cards.
func (d *doc) tileSpans(lo, hi float64, n int, maxLen float64, crosses func(float64) int) [][2]float64 {
	step := (hi - lo) / float64(n)
	ov := tileOverlap / 2 * step
	slack := math.Min(math.Max(0, (maxLen-2*ov-step)/2)*0.9, step*0.4)
	cuts := make([]float64, 0, n)
	for i := 1; i < n; i++ {
		ideal := lo + float64(i)*step
		best, bestScore := ideal, math.Inf(1)
		inc := math.Max(4, slack/24)
		for x := ideal - slack; x <= ideal+slack+1e-9; x += inc {
			if score := float64(crosses(x))*1e6 + math.Abs(x-ideal); score < bestScore {
				best, bestScore = x, score
			}
		}
		cuts = append(cuts, best)
	}
	spans := make([][2]float64, n)
	for k := range spans {
		spans[k] = [2]float64{lo, hi}
		if k > 0 {
			spans[k][0] = cuts[k-1] - ov
		}
		if k < n-1 {
			spans[k][1] = cuts[k] + ov
		}
	}
	return spans
}

// crossing counts the cards a cut at pos would slice through.
func (d *doc) crossing(cards []int, pos float64, vertical bool) int {
	n := 0
	for _, ci := range cards {
		r := d.sc.cards[ci].rect()
		if vertical && r.X < pos && r.right() > pos || !vertical && r.Y < pos && r.bottom() > pos {
			n++
		}
	}
	return n
}

// planDetail decomposes the map into islands at one shared scale and packs them onto pages.
func (d *doc) planDetail() []mapPage {
	pl := d.newIslandPlanner()
	tops := pl.tops()
	if len(tops) == 0 {
		return nil
	}
	// the largest island that cannot be split further decides the shared scale
	s := maxDetailScale
	for _, is := range pl.islandsAt(tops, maxDetailScale, false) {
		s = math.Min(s, d.fitScale(is.Box))
	}
	s = math.Max(s, readableScale)
	d.islands = pl.islandsAt(tops, s, true)
	return d.packIslands(s)
}

// packPage is a page being filled by the maxrects packer.
type packPage struct {
	free   []rect
	places []placement
	tops   map[int]bool
}

func (d *doc) newPackPage() *packPage {
	c, g := d.content, packGutter
	lw, lh := d.locatorSize()
	loc := rect{c.right() - lw, c.bottom() - lh, lw, lh}
	return &packPage{
		// free space reaches a gutter beyond the page so items carry their own gutter
		free: []rect{{c.X, c.Y, c.W + g, loc.Y - c.Y}, {c.X, c.Y, loc.X - c.X, c.H + g}},
		tops: map[int]bool{},
	}
}

// fit finds the best free rectangle for a w x h item (short-side fit); score is the leftover.
func (pg *packPage) fit(w, h float64) (rect, float64, bool) {
	best, bestScore, ok := rect{}, math.Inf(1), false
	for _, f := range pg.free {
		if w > f.W+1e-6 || h > f.H+1e-6 {
			continue
		}
		if score := math.Min(f.W-w, f.H-h); score < bestScore {
			best, bestScore, ok = rect{f.X, f.Y, w, h}, score, true
		}
	}
	return best, bestScore, ok
}

// put reserves r and splits the free rectangles around it.
func (pg *packPage) put(r rect) {
	var next []rect
	for _, f := range pg.free {
		if !f.intersects(r) {
			next = append(next, f)
			continue
		}
		if r.X > f.X {
			next = append(next, rect{f.X, f.Y, r.X - f.X, f.H})
		}
		if r.right() < f.right() {
			next = append(next, rect{r.right(), f.Y, f.right() - r.right(), f.H})
		}
		if r.Y > f.Y {
			next = append(next, rect{f.X, f.Y, f.W, r.Y - f.Y})
		}
		if r.bottom() < f.bottom() {
			next = append(next, rect{f.X, r.bottom(), f.W, f.bottom() - r.bottom()})
		}
	}
	pg.free = pg.free[:0]
	for i, a := range next {
		contained := false
		for j, b := range next {
			if i != j && a.X >= b.X-1e-9 && a.Y >= b.Y-1e-9 && a.right() <= b.right()+1e-9 && a.bottom() <= b.bottom()+1e-9 && (i > j || a != b) {
				contained = true
				break
			}
		}
		if !contained && a.W > 1e-6 && a.H > 1e-6 {
			pg.free = append(pg.free, a)
		}
	}
}

// packIslands places the islands, related ones together, on as few pages as it can.
func (d *doc) packIslands(s float64) []mapPage {
	order := make([]int, len(d.islands))
	for i := range order {
		order[i] = i
	}
	height := func(i int) float64 { return d.islands[i].Box.H * s }
	sort.SliceStable(order, func(a, b int) bool {
		ia, ib := d.islands[order[a]], d.islands[order[b]]
		if ia.Top != ib.Top {
			return ia.Top < ib.Top
		}
		if ia.Part != ib.Part && ia.Caption == ib.Caption {
			return ia.Part < ib.Part
		}
		return height(order[a]) > height(order[b])
	})
	var pages []*packPage
	for _, i := range order {
		is := d.islands[i]
		w, h := is.Box.W*s+packGutter, is.Box.H*s+islandCaptionH+packGutter
		var pick *packPage
		var pickRect rect
		pickScore := math.Inf(1)
		for _, pg := range pages {
			r, score, ok := pg.fit(w, h)
			if !ok {
				continue
			}
			if !pg.tops[is.Top] {
				score += 1e6 // prefer pages that already hold this cluster
			}
			if score < pickScore {
				pick, pickRect, pickScore = pg, r, score
			}
		}
		if pick == nil {
			pick = d.newPackPage()
			pages = append(pages, pick)
			r, _, ok := pick.fit(w, h)
			if !ok {
				r = rect{d.content.X, d.content.Y, w, h}
			}
			pickRect = r
		}
		pick.put(pickRect)
		pick.tops[is.Top] = true
		pick.places = append(pick.places, placement{Island: i, X: pickRect.X, Y: pickRect.Y})
	}
	lw, lh := d.locatorSize()
	out := make([]mapPage, 0, len(pages))
	for _, pg := range pages {
		var names []string
		seen := map[string]bool{}
		for _, pl := range pg.places {
			if n := d.islands[pl.Island].TopName; !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
		sort.Slice(pg.places, func(a, b int) bool {
			if pg.places[a].Y != pg.places[b].Y {
				return pg.places[a].Y < pg.places[b].Y
			}
			return pg.places[a].X < pg.places[b].X
		})
		out = append(out, mapPage{
			Title: strings.Join(names, ", "), Area: d.content, Scale: s, Places: pg.places,
			Locator: rect{d.content.right() - lw, d.content.bottom() - lh, lw, lh},
		})
	}
	return out
}

// looseCaption names a loose island: the place path its cards declare, else "No place".
func (d *doc) looseCaption(cards []int) string {
	if path := d.m.placePath(d.sc.cards[cards[0]].ID); len(path) > 0 {
		return d.crumb(path)
	}
	return d.lb.get("desktop.infra_pdf_no_place")
}
