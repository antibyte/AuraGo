package homedash

import "fmt"

type cell struct{ x, y int }

func (v *validator) layout(path string, p Page, items []LayoutItem, cols int) {
	byID := map[string]Widget{}
	for _, w := range p.Widgets {
		byID[w.ID] = w
	}
	seen, occ := map[string]bool{}, map[cell]string{}
	for i, it := range items {
		ip := fmt.Sprintf("%s/%d", path, i)
		w, ok := byID[it.ID]
		switch {
		case !ok:
			v.err(ip, IssueLayoutUnknown, "no widget with this id on the page")
			continue
		case seen[it.ID]:
			v.err(ip, IssueLayoutDuplicate, "widget placed twice")
			continue
		}
		seen[it.ID] = true
		if it.X < 0 || it.Y < 0 || it.Y > v.cat.Grid.MaxY || it.W < 1 || it.H < 1 || it.X+it.W > cols {
			v.err(ip, IssueLayoutBounds, fmt.Sprintf("outside the %d-column grid", cols))
			continue
		}
		if wt, ok := v.cat.Type(w.Type); ok {
			if vr, ok := wt.Variant(w.Variant); ok && (it.W < vr.MinSize.W || it.H < vr.MinSize.H || it.W > wt.MaxSize.W || it.H > wt.MaxSize.H) {
				v.err(ip, IssueSize, fmt.Sprintf("size must be %dx%d to %dx%d", vr.MinSize.W, vr.MinSize.H, wt.MaxSize.W, wt.MaxSize.H))
			}
		}
		overlap := ""
		for x := it.X; x < it.X+it.W && overlap == ""; x++ {
			for y := it.Y; y < it.Y+it.H; y++ {
				if other := occ[cell{x, y}]; other != "" {
					overlap = other
					break
				}
			}
		}
		if overlap != "" {
			v.err(ip, IssueLayoutOverlap, "overlaps "+overlap)
			continue
		}
		mark(occ, it)
	}
	for _, w := range p.Widgets {
		if !seen[w.ID] {
			v.err(path, IssueLayoutMissing, "widget "+w.ID+" has no place in this layout")
		}
	}
}

func mark(occ map[cell]string, it LayoutItem) {
	for x := it.X; x < it.X+it.W; x++ {
		for y := it.Y; y < it.Y+it.H; y++ {
			occ[cell{x, y}] = it.ID
		}
	}
}

func free(occ map[cell]string, x, y int, s Size) bool {
	for dx := 0; dx < s.W; dx++ {
		for dy := 0; dy < s.H; dy++ {
			if occ[cell{x + dx, y + dy}] != "" {
				return false
			}
		}
	}
	return true
}

// Autopack gives every widget without an lg item a place: widget order, the variant's default size
// (clamped to the lg columns), first free position scanning rows top-down and columns left-right.
// It never moves existing items and never fills md or sm.
func Autopack(doc *Dashboard, cat WidgetCatalog) {
	cols, maxY := cat.Grid.Columns.LG, cat.Grid.MaxY
	for pi := range doc.Pages {
		p := &doc.Pages[pi]
		occ, placed := map[cell]string{}, map[string]bool{}
		for _, it := range p.Layouts.LG {
			placed[it.ID] = true
			if it.W > 0 && it.H > 0 && it.X >= 0 && it.Y >= 0 {
				mark(occ, it)
			}
		}
		for _, w := range p.Widgets {
			if placed[w.ID] || !ValidWidgetID(w.ID) {
				continue
			}
			size := Size{2, 1}
			if wt, ok := cat.Type(w.Type); ok {
				if vr, ok := wt.Variant(w.Variant); ok {
					size = vr.DefaultSize
				}
			}
			size.W = min(size.W, cols)
			it := LayoutItem{ID: w.ID, X: 0, Y: maxY + 1, W: size.W, H: size.H}
		search:
			for y := 0; y <= maxY; y++ {
				for x := 0; x+size.W <= cols; x++ {
					if free(occ, x, y, size) {
						it.X, it.Y = x, y
						break search
					}
				}
			}
			mark(occ, it)
			p.Layouts.LG = append(p.Layouts.LG, it)
			placed[w.ID] = true
		}
	}
}
