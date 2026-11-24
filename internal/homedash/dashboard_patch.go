package homedash

import (
	"errors"
	"fmt"
	"slices"
)

// MaxPatchOps bounds one patch.
const MaxPatchOps = 50

// PatchOp is one operation of ApplyPatch.
type PatchOp struct {
	Op       string  `json:"op"` // add_widget|update_widget|move_widget|remove_widget|add_page|rename_page|remove_page|set_name|set_look
	PageID   string  `json:"page_id,omitempty"`
	WidgetID string  `json:"widget_id,omitempty"`
	Widget   *Widget `json:"widget,omitempty"`
	X        *int    `json:"x,omitempty"`
	Y        *int    `json:"y,omitempty"`
	W        *int    `json:"w,omitempty"`
	H        *int    `json:"h,omitempty"`
	Name     string  `json:"name,omitempty"`
	Look     *Look   `json:"look,omitempty"`
	Confirm  *bool   `json:"confirm,omitempty"`
}

// ApplyPatch applies ops to a copy of doc. The caller runs Normalize, Autopack and Validate next.
func ApplyPatch(doc Dashboard, ops []PatchOp) (Dashboard, error) {
	if len(ops) == 0 || len(ops) > MaxPatchOps {
		return doc, newError(CodeInvalidPatch, fmt.Sprintf("1 to %d operations", MaxPatchOps))
	}
	out, err := cloneDashboard(doc)
	if err != nil {
		return doc, err
	}
	for i, op := range ops {
		if err := applyOp(&out, op); err != nil {
			return doc, newError(CodeInvalidPatch, fmt.Sprintf("op %d (%s): %s", i, op.Op, err))
		}
	}
	return out, nil
}

func findPage(d *Dashboard, id string) *Page {
	for i := range d.Pages {
		if d.Pages[i].ID == id {
			return &d.Pages[i]
		}
	}
	return nil
}

func findWidget(d *Dashboard, id string) (*Page, int) {
	for i := range d.Pages {
		for j := range d.Pages[i].Widgets {
			if d.Pages[i].Widgets[j].ID == id {
				return &d.Pages[i], j
			}
		}
	}
	return nil, -1
}

func nextWidgetID(d *Dashboard) string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("w%d", n)
		if p, _ := findWidget(d, id); p == nil {
			return id
		}
	}
}

func dropItem(items []LayoutItem, id string) []LayoutItem {
	return slices.DeleteFunc(items, func(it LayoutItem) bool { return it.ID == id })
}

func removeWidget(p *Page, idx int) Widget {
	w := p.Widgets[idx]
	p.Widgets = slices.Delete(p.Widgets, idx, idx+1)
	p.Layouts.LG, p.Layouts.MD, p.Layouts.SM = dropItem(p.Layouts.LG, w.ID), dropItem(p.Layouts.MD, w.ID), dropItem(p.Layouts.SM, w.ID)
	return w
}

func placement(op PatchOp) (*LayoutItem, error) {
	n := 0
	for _, p := range []*int{op.X, op.Y, op.W, op.H} {
		if p != nil {
			n++
		}
	}
	switch n {
	case 0:
		return nil, nil
	case 4:
		return &LayoutItem{X: *op.X, Y: *op.Y, W: *op.W, H: *op.H}, nil
	}
	return nil, errors.New("x, y, w and h go together")
}

func applyOp(d *Dashboard, op PatchOp) error {
	switch op.Op {
	case "add_widget":
		p := findPage(d, op.PageID)
		if p == nil {
			return errors.New("page not found")
		}
		if op.Widget == nil {
			return errors.New("widget is required")
		}
		w := *op.Widget
		if w.ID == "" {
			w.ID = nextWidgetID(d)
		} else if q, _ := findWidget(d, w.ID); q != nil {
			return errors.New("widget id already used")
		}
		pl, err := placement(op)
		if err != nil {
			return err
		}
		p.Widgets = append(p.Widgets, w)
		if pl != nil {
			pl.ID = w.ID
			p.Layouts.LG = append(p.Layouts.LG, *pl)
		}
	case "update_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		if op.Widget == nil && op.Confirm == nil {
			return errors.New("widget or confirm is required")
		}
		dst := &p.Widgets[idx]
		if src := op.Widget; src != nil {
			if src.Type != "" {
				dst.Type = src.Type
			}
			if src.Variant != "" {
				dst.Variant = src.Variant
			}
			if src.Family != "" {
				dst.Family = src.Family
			}
			if src.Title != "" {
				dst.Title = src.Title
			}
			if src.Bindings != nil {
				dst.Bindings = src.Bindings
			}
			if src.Options != nil {
				dst.Options = src.Options
			}
			if src.Actions.Tap != nil {
				dst.Actions.Tap = src.Actions.Tap
			}
			if src.Actions.Hold != nil {
				dst.Actions.Hold = src.Actions.Hold
			}
		}
		if op.Confirm != nil {
			dst.Confirm = *op.Confirm
		}
	case "move_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		pl, err := placement(op)
		if err != nil {
			return err
		}
		if op.PageID != "" && op.PageID != p.ID {
			dst := findPage(d, op.PageID)
			if dst == nil {
				return errors.New("page not found")
			}
			dst.Widgets = append(dst.Widgets, removeWidget(p, idx))
			p = dst
		} else if pl == nil {
			return errors.New("give a page or a position")
		}
		if pl != nil {
			pl.ID = op.WidgetID
			p.Layouts.LG = append(dropItem(p.Layouts.LG, op.WidgetID), *pl)
		}
	case "remove_widget":
		p, idx := findWidget(d, op.WidgetID)
		if p == nil {
			return errors.New("widget not found")
		}
		removeWidget(p, idx)
	case "add_page":
		if !ValidPageID(op.PageID) {
			return errors.New("invalid page id")
		}
		if findPage(d, op.PageID) != nil {
			return errors.New("page id already used")
		}
		d.Pages = append(d.Pages, Page{ID: op.PageID, Name: op.Name, Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}})
	case "rename_page":
		p := findPage(d, op.PageID)
		if p == nil {
			return errors.New("page not found")
		}
		p.Name = op.Name
	case "remove_page":
		if findPage(d, op.PageID) == nil {
			return errors.New("page not found")
		}
		if len(d.Pages) == 1 {
			return errors.New("the last page cannot be removed")
		}
		d.Pages = slices.DeleteFunc(d.Pages, func(p Page) bool { return p.ID == op.PageID })
		if d.Kiosk != nil && d.Kiosk.StartPage == op.PageID {
			d.Kiosk.StartPage = d.Pages[0].ID
		}
	case "set_name":
		d.Name = op.Name
	case "set_look":
		if op.Look == nil {
			return errors.New("look is required")
		}
		d.Look = *op.Look
	default:
		return errors.New("unknown operation")
	}
	return nil
}
