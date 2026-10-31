package atlaspdf

import "math"

const (
	readableScale    = 34.0 / cardW // mm per world unit at which a card is 34 mm wide
	compactWidth     = 16.0         // card width in mm below which cards shrink to dot pills
	overviewMaxScale = 0.30
	overviewPad      = 16.0
	mergeGap         = 600.0
	tileOverlap      = 0.04
	legendHeight     = 9.0
	statTileW        = 26.0
	statTileH        = 16.0
	statTileGap      = 2.0
	statTiles        = 7
)

// mapPage is one page of the map section. The overview shows the world
// rectangle World in Area at Scale; a detail page packs islands (see islands.go)
// at one shared Scale.
type mapPage struct {
	Title    string
	Overview bool
	Area     rect // mm
	World    rect // overview only: visible world units, World.W*Scale == Area.W
	Scale    float64
	Places   []placement
	Locator  rect // mm, detail pages only
}

// view maps world units to page millimetres.
type view struct {
	area, world rect
	s           float64
}

func (p mapPage) view() view { return view{p.Area, p.World, p.Scale} }

func (v view) pt(p pt) pt { return pt{v.area.X + (p.X-v.world.X)*v.s, v.area.Y + (p.Y-v.world.Y)*v.s} }
func (v view) rect(r rect) rect {
	return rect{v.area.X + (r.X-v.world.X)*v.s, v.area.Y + (r.Y-v.world.Y)*v.s, r.W * v.s, r.H * v.s}
}

// visible reports whether a millimetre rectangle touches the view's area.
func (v view) visible(r rect) bool { return r.intersects(v.area.pad(6)) }

// fitPage centres focus in area at the largest scale up to maxScale.
func fitPage(area, focus rect, maxScale float64) mapPage {
	focus.W, focus.H = math.Max(focus.W, 1), math.Max(focus.H, 1)
	s := math.Min(math.Min(area.W/focus.W, area.H/focus.H), maxScale)
	w, h := area.W/s, area.H/s
	c := focus.center()
	return mapPage{Area: area, World: rect{c.X - w/2, c.Y - h/2, w, h}, Scale: s}
}

func rawFit(area, focus rect) float64 {
	return math.Min(area.W/math.Max(focus.W, 1), area.H/math.Max(focus.H, 1))
}

// overviewLayout splits the first map page into title block, map area and legend.
func (d *doc) overviewLayout() (area, legend rect, tilesRight bool) {
	tilesWidth := statTiles*statTileW + (statTiles-1)*statTileGap
	tilesRight = d.content.W-tilesWidth-6 >= 62
	top := 36.0
	if tilesRight {
		top = 22
	}
	area = rect{d.content.X, d.content.Y + top, d.content.W, d.content.H - top - legendHeight - 3}
	legend = rect{d.content.X, area.bottom() + 3, d.content.W, legendHeight}
	return area, legend, tilesRight
}

// planMap lays out the overview and, when the whole map is too small to read on
// one page, the detail pages.
func (d *doc) planMap() []mapPage {
	area, _, _ := d.overviewLayout()
	sc := d.sc
	focus := rect{0, 0, cardW * 4, cardH * 4}
	if sc.hasBounds {
		focus = sc.bounds.pad(overviewPad)
	}
	ov := fitPage(area, focus, overviewMaxScale)
	ov.Overview = true
	ov.Title = d.lb.get("desktop.infra_pdf_overview")
	pages := []mapPage{ov}
	if !sc.hasBounds || rawFit(area, focus) >= readableScale {
		return pages
	}
	return append(pages, d.planDetail()...)
}

// computeFirst records, for every drawn shape, the first detail page it appears on.
func (d *doc) computeFirst() {
	d.first = map[string]int{}
	for i := 1; i < len(d.mapPages); i++ {
		for _, pl := range d.mapPages[i].Places {
			is := &d.islands[pl.Island]
			for id := range is.cards {
				if _, ok := d.first[id]; !ok {
					d.first[id] = i
				}
			}
			for id := range is.regions {
				if _, ok := d.first[id]; !ok {
					d.first[id] = i
				}
			}
		}
	}
}

// boxGap is the distance between two rectangles, zero when they touch.
func boxGap(a, b rect) float64 {
	dx := math.Max(0, math.Max(a.X-b.right(), b.X-a.right()))
	dy := math.Max(0, math.Max(a.Y-b.bottom(), b.Y-a.bottom()))
	return math.Hypot(dx, dy)
}
