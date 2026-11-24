package homedash

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Issue codes of document validation.
const (
	IssueRequired          = "required"
	IssueTooLong           = "too_long"
	IssueBadFormat         = "bad_format"
	IssueOutOfRange        = "out_of_range"
	IssueNotAllowed        = "not_allowed"
	IssueControlChars      = "control_chars"
	IssueURLNotAllowed     = "url_not_allowed"
	IssueDuplicateID       = "duplicate_id"
	IssueUnknownType       = "unknown_type"
	IssueUnknownVariant    = "unknown_variant"
	IssueUnknownBinding    = "unknown_binding"
	IssueBindingRequired   = "binding_required"
	IssueBadEntityID       = "bad_entity_id"
	IssueTooMany           = "too_many"
	IssueUnknownOption     = "unknown_option"
	IssueBadOption         = "bad_option"
	IssuePageNotFound      = "page_not_found"
	IssueToggleUnsupported = "toggle_unsupported"
	IssueLayoutMissing     = "layout_missing"
	IssueLayoutDuplicate   = "layout_duplicate"
	IssueLayoutUnknown     = "layout_unknown_widget"
	IssueLayoutOverlap     = "layout_overlap"
	IssueLayoutBounds      = "layout_out_of_bounds"
	IssueSize              = "size_out_of_range"
	IssueDocumentTooLarge  = "document_too_large"

	WarnEntityUnknown     = "entity_unknown"
	WarnKindMismatch      = "kind_mismatch"
	WarnCapabilityMissing = "capability_missing"
)

// Report is the result of Validate: issues block a save, warnings do not.
type Report struct {
	Issues   []Issue `json:"issues"`
	Warnings []Issue `json:"warnings"`
}

// OK reports a document without issues.
func (r Report) OK() bool { return len(r.Issues) == 0 }

var hexColourRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

type validator struct {
	cat   WidgetCatalog
	known func(EntityID) (Entity, bool)
	r     Report
}

func (v *validator) err(path, code, msg string) {
	v.r.Issues = append(v.r.Issues, Issue{path, code, msg})
}
func (v *validator) warn(path, code, msg string) {
	v.r.Warnings = append(v.r.Warnings, Issue{path, code, msg})
}

func (v *validator) text(path, s string, min, max int) {
	switch n := utf8.RuneCountInString(s); {
	case n < min:
		v.err(path, IssueRequired, "required")
	case n > max:
		v.err(path, IssueTooLong, fmt.Sprintf("at most %d characters", max))
	case ContainsControl(s):
		v.err(path, IssueControlChars, "control characters are not allowed")
	case ContainsURL(s):
		v.err(path, IssueURLNotAllowed, "URLs are not allowed")
	}
}

func (v *validator) enum(path, s string, allowed ...string) {
	if !slices.Contains(allowed, s) {
		v.err(path, IssueNotAllowed, "one of "+strings.Join(allowed, ", "))
	}
}

func (v *validator) rangeF(path string, f, lo, hi float64) {
	if f < lo || f > hi {
		v.err(path, IssueOutOfRange, fmt.Sprintf("must be %g–%g", lo, hi))
	}
}

// Validate checks a document against the catalog. known (may be nil) reports entities the hub
// knows; unknown entities and kind/capability mismatches are warnings, never errors.
func Validate(doc Dashboard, cat WidgetCatalog, known func(EntityID) (Entity, bool)) Report {
	v := &validator{cat: cat, known: known, r: Report{Issues: []Issue{}, Warnings: []Issue{}}}
	if raw, err := json.Marshal(doc); err != nil || len(raw) > cat.Limits.DocumentBytes {
		v.err("", IssueDocumentTooLarge, "the document is larger than 256 KiB")
	}
	if doc.SchemaVersion != SchemaVersion {
		v.err("/schema_version", IssueOutOfRange, "unsupported schema version")
	}
	if doc.ID != "" && !ValidDashboardID(doc.ID) {
		v.err("/id", IssueBadFormat, "invalid dashboard id")
	}
	if !ValidSlug(doc.Slug) {
		v.err("/slug", IssueBadFormat, "lower-case letters, digits and dashes, at most 48")
	}
	v.text("/name", doc.Name, 1, 80)
	if doc.Icon != "" && !ValidIcon(doc.Icon) {
		v.err("/icon", IssueBadFormat, "invalid icon id")
	}
	v.look(doc.Look)
	v.grid(doc.Grid)
	if len(doc.Pages) == 0 {
		v.err("/pages", IssueRequired, "at least one page")
	} else if len(doc.Pages) > cat.Limits.Pages {
		v.err("/pages", IssueTooMany, fmt.Sprintf("at most %d pages", cat.Limits.Pages))
	}
	pages, total := map[string]bool{}, 0
	for i, p := range doc.Pages {
		path := fmt.Sprintf("/pages/%d", i)
		if !ValidPageID(p.ID) {
			v.err(path+"/id", IssueBadFormat, "invalid page id")
		} else if pages[p.ID] {
			v.err(path+"/id", IssueDuplicateID, "page id used twice")
		}
		pages[p.ID] = true
		v.text(path+"/name", p.Name, 1, 40)
		if p.Icon != "" && !ValidIcon(p.Icon) {
			v.err(path+"/icon", IssueBadFormat, "invalid icon id")
		}
		if len(p.Widgets) > cat.Limits.WidgetsPerPage {
			v.err(path+"/widgets", IssueTooMany, fmt.Sprintf("at most %d widgets per page", cat.Limits.WidgetsPerPage))
		}
		total += len(p.Widgets)
	}
	if total > cat.Limits.Widgets {
		v.err("/pages", IssueTooMany, fmt.Sprintf("at most %d widgets", cat.Limits.Widgets))
	}
	seen := map[string]bool{}
	for i, p := range doc.Pages {
		path := fmt.Sprintf("/pages/%d", i)
		for j, w := range p.Widgets {
			v.widget(fmt.Sprintf("%s/widgets/%d", path, j), w, pages, seen)
		}
		v.layout(path+"/layouts/lg", p, p.Layouts.LG, cat.Grid.Columns.LG)
		if p.Layouts.MD != nil {
			v.layout(path+"/layouts/md", p, p.Layouts.MD, cat.Grid.Columns.MD)
		}
		if p.Layouts.SM != nil {
			v.layout(path+"/layouts/sm", p, p.Layouts.SM, cat.Grid.Columns.SM)
		}
	}
	v.kiosk(doc.Kiosk, pages)
	return v.r
}

func (v *validator) look(l Look) {
	v.enum("/look/family", l.Family, v.cat.Families...)
	v.enum("/look/palette/mode", l.Palette.Mode, "theme", "dark", "light", "custom")
	switch c := l.Palette.Custom; {
	case l.Palette.Mode == "custom" && c == nil:
		v.err("/look/palette/custom", IssueRequired, "a custom palette needs accent, surface_hue and scheme")
	case l.Palette.Mode == "custom":
		if !hexColourRe.MatchString(c.Accent) {
			v.err("/look/palette/custom/accent", IssueBadFormat, "#rrggbb in lower case")
		}
		v.rangeF("/look/palette/custom/surface_hue", float64(c.SurfaceHue), 0, 359)
		v.enum("/look/palette/custom/scheme", c.Scheme, "dark", "light")
	case c != nil:
		v.err("/look/palette/custom", IssueNotAllowed, "only with mode custom")
	}
	if len(l.Palette.Snapshot) > 0 {
		if l.Palette.Mode != "theme" {
			v.err("/look/palette/snapshot", IssueNotAllowed, "only with mode theme")
		}
		if len(l.Palette.Snapshot) > 40 {
			v.err("/look/palette/snapshot", IssueTooMany, "at most 40 tokens")
		}
		for _, k := range sortedKeys(l.Palette.Snapshot) {
			p := "/look/palette/snapshot/" + k
			if !slices.Contains(v.cat.PaletteTokens, k) {
				v.err(p, IssueNotAllowed, "unknown colour token")
			} else if !hexColourRe.MatchString(l.Palette.Snapshot[k]) {
				v.err(p, IssueBadFormat, "#rrggbb in lower case")
			}
		}
	}
	v.enum("/look/backdrop/kind", l.Backdrop.Kind, "theme", "solid", "gradient")
	v.enum("/look/density", l.Density, "compact", "comfortable", "spacious")
	v.rangeF("/look/radius_scale", l.RadiusScale, 0.5, 1.5)
}

func (v *validator) grid(g Grid) {
	if !g.RowUnit.Auto {
		v.rangeF("/grid/row_unit", float64(g.RowUnit.PX), float64(v.cat.Grid.RowUnitMin), float64(v.cat.Grid.RowUnitMax))
	}
	if g.Gap != nil {
		v.rangeF("/grid/gap", float64(*g.Gap), 0, float64(v.cat.Grid.GapMax))
	}
	v.enum("/grid/fit", g.Fit, "scroll", "fit_screen")
}

func (v *validator) widget(path string, w Widget, pages, seen map[string]bool) {
	if !ValidWidgetID(w.ID) {
		v.err(path+"/id", IssueBadFormat, "invalid widget id")
	} else if seen[w.ID] {
		v.err(path+"/id", IssueDuplicateID, "widget id used twice")
	}
	seen[w.ID] = true
	wt, ok := v.cat.Type(w.Type)
	if !ok {
		v.err(path+"/type", IssueUnknownType, "unknown widget type")
		return
	}
	if _, ok := wt.Variant(w.Variant); !ok {
		v.err(path+"/variant", IssueUnknownVariant, "unknown variant")
	}
	if w.Family != "" {
		v.enum(path+"/family", w.Family, v.cat.Families...)
	}
	v.text(path+"/title", w.Title, 0, 80)
	if len(w.Bindings) > v.cat.Limits.Bindings {
		v.err(path+"/bindings", IssueTooMany, "too many bindings")
	}
	for _, b := range wt.Bindings {
		if b.Required && w.Bindings[b.Name] == "" {
			v.err(path+"/bindings/"+b.Name, IssueBindingRequired, "this binding is required")
		}
	}
	for _, name := range sortedKeys(w.Bindings) {
		p, raw := path+"/bindings/"+name, w.Bindings[name]
		b, ok := wt.Binding(name)
		switch {
		case !ok:
			v.err(p, IssueUnknownBinding, "the widget has no such binding")
		case raw == "":
		default:
			if _, _, err := ParseEntityID(raw); err != nil {
				v.err(p, IssueBadEntityID, "invalid entity id")
			} else {
				v.entity(p, EntityID(raw), b)
			}
		}
	}
	v.options(path+"/options", wt, w.Options, pages)
	v.action(path+"/actions/tap", w.Actions.Tap, wt, pages)
	v.action(path+"/actions/hold", w.Actions.Hold, wt, pages)
}

func (v *validator) entity(p string, id EntityID, b BindingSpec) {
	if v.known == nil {
		return
	}
	e, ok := v.known(id)
	if !ok {
		v.warn(p, WarnEntityUnknown, "the entity is not known (offline or removed)")
		return
	}
	if len(b.Kinds) > 0 && !slices.Contains(b.Kinds, e.Kind) {
		v.warn(p, WarnKindMismatch, "the entity kind does not fit this binding")
	}
	if len(b.AnyCapability) > 0 && !slices.ContainsFunc(e.Capabilities, func(c CapabilitySpec) bool { return slices.Contains(b.AnyCapability, c.Name) }) {
		v.warn(p, WarnCapabilityMissing, "the entity lacks a capability this binding needs")
	}
}

func (v *validator) options(path string, wt WidgetType, opts map[string]any, pages map[string]bool) {
	if len(opts) > v.cat.Limits.Options {
		v.err(path, IssueTooMany, "too many options")
	}
	for _, s := range append(slices.Clone(wt.Options), v.cat.CommonOptions...) {
		if val, ok := opts[s.Key]; s.Required && (!ok || val == nil || val == "") {
			v.err(path+"/"+s.Key, IssueRequired, "required")
		}
	}
	for _, k := range sortedKeys(opts) {
		s, ok := v.cat.Option(wt, k)
		if !ok {
			v.err(path+"/"+k, IssueUnknownOption, "unknown option")
			continue
		}
		v.option(path+"/"+k, s, opts[k], pages)
	}
	if wt.Type == "gauge" {
		lo, okLo := toFloat(opts["min"])
		hi, okHi := toFloat(opts["max"])
		if okLo && okHi && lo >= hi {
			v.err(path+"/max", IssueBadOption, "max must be above min")
		}
	}
}

func (v *validator) option(p string, s OptionSpec, val any, pages map[string]bool) {
	bad := func() { v.err(p, IssueBadOption, "wrong type for "+s.Type) }
	switch s.Type {
	case "bool":
		if _, ok := val.(bool); !ok {
			bad()
		}
	case "int", "number":
		if val == nil && s.Default == nil {
			return
		}
		f, ok := toFloat(val)
		if !ok || (s.Type == "int" && f != math.Trunc(f)) {
			bad()
			return
		}
		if (s.Min != nil && f < *s.Min) || (s.Max != nil && f > *s.Max) {
			v.err(p, IssueOutOfRange, "out of range")
		}
	case "string":
		str, ok := val.(string)
		if !ok {
			bad()
			return
		}
		max := s.MaxLen
		if max == 0 || max > 200 {
			max = 200
		}
		v.text(p, str, 0, max)
	case "enum":
		if str, ok := val.(string); !ok || !slices.Contains(s.Enum, str) {
			v.err(p, IssueNotAllowed, "one of "+strings.Join(s.Enum, ", "))
		}
	case "color":
		if str, ok := val.(string); !ok || (str != "" && !hexColourRe.MatchString(str)) {
			v.err(p, IssueBadFormat, "#rrggbb in lower case")
		}
	case "page":
		str, ok := val.(string)
		switch {
		case !ok:
			bad()
		case str != "" && !pages[str]:
			v.err(p, IssuePageNotFound, "no such page")
		}
	}
}

func (v *validator) action(p string, a *Action, wt WidgetType, pages map[string]bool) {
	if a == nil {
		return
	}
	v.enum(p+"/do", a.Do, "default", "toggle", "more_info", "navigate", "none")
	switch {
	case a.Do == "navigate" && a.Page == "":
		v.err(p+"/page", IssueRequired, "navigate needs a page")
	case a.Do == "navigate" && !pages[a.Page]:
		v.err(p+"/page", IssuePageNotFound, "no such page")
	case a.Do != "navigate" && a.Page != "":
		v.err(p+"/page", IssueNotAllowed, "only with navigate")
	}
	if a.Do == "toggle" && !slices.Contains(wt.Commands["primary"], CapToggle) {
		v.err(p+"/do", IssueToggleUnsupported, "this widget cannot toggle")
	}
}

func (v *validator) kiosk(k *KioskSettings, pages map[string]bool) {
	if k == nil {
		v.err("/kiosk", IssueRequired, "required")
		return
	}
	if !pages[k.StartPage] {
		v.err("/kiosk/start_page", IssuePageNotFound, "no such page")
	}
	v.rangeF("/kiosk/idle_dim_seconds", float64(k.IdleDimSeconds), 0, 3600)
	v.rangeF("/kiosk/idle_dim_level", k.IdleDimLevel, 0.05, 1)
	if k.PageRotationSeconds != 0 {
		v.rangeF("/kiosk/page_rotation_seconds", float64(k.PageRotationSeconds), 10, 3600)
	}
	v.enum("/kiosk/screen_saver", k.ScreenSaver, "none", "clock", "black")
}
