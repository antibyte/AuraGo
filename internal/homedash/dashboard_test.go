package homedash

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
)

func validDoc() Dashboard {
	d := Dashboard{SchemaVersion: 1, Slug: "living-room", Name: "Living room", Icon: "sofa",
		Look: Look{Family: "glass", Palette: Palette{Mode: "theme", Snapshot: map[string]string{"surface-solid": "#1a212b", "ink": "#f1f4f9"}},
			Backdrop: Backdrop{Kind: "theme"}, Density: "comfortable", RadiusScale: 1},
		Grid: Grid{RowUnit: RowUnit{Auto: true}, Fit: "scroll"},
		Pages: []Page{{ID: "main", Name: "Main", Icon: "home", Widgets: []Widget{
			{ID: "w1", Type: "switch", Variant: "rocker", Family: "tactile", Title: "Lamp", Bindings: map[string]string{"primary": "ha:switch.lamp"},
				Actions: WidgetActions{Tap: &Action{Do: "default"}, Hold: &Action{Do: "more_info"}}},
			{ID: "w2", Type: "gauge", Variant: "arc", Bindings: map[string]string{"primary": "system:cpu"}, Options: map[string]any{"min": 0.0, "max": 100.0}},
		}, Layouts: Layouts{LG: []LayoutItem{{ID: "w1", X: 0, Y: 0, W: 2, H: 2}, {ID: "w2", X: 2, Y: 0, W: 2, H: 2}}}}}}
	Normalize(&d)
	return d
}

func hasIssue(list []Issue, path, code string) bool {
	return slices.ContainsFunc(list, func(i Issue) bool { return i.Path == path && i.Code == code })
}

func TestValidateDashboard(t *testing.T) {
	w := func(d *Dashboard, i int) *Widget { return &d.Pages[0].Widgets[i] }
	wp := "/pages/0/widgets/"
	for _, tc := range []struct {
		name, path, code string
		mut              func(*Dashboard)
	}{
		{"valid", "", "", func(*Dashboard) {}},
		{"unknown type", wp + "0/type", IssueUnknownType, func(d *Dashboard) { w(d, 0).Type = "toaster" }},
		{"unknown variant", wp + "0/variant", IssueUnknownVariant, func(d *Dashboard) { w(d, 0).Variant = "flip" }},
		{"missing binding", wp + "0/bindings/primary", IssueBindingRequired, func(d *Dashboard) { delete(w(d, 0).Bindings, "primary") }},
		{"bad entity id", wp + "0/bindings/primary", IssueBadEntityID, func(d *Dashboard) { w(d, 0).Bindings["primary"] = "nocolon" }},
		{"bad widget id", wp + "0/id", IssueBadFormat, func(d *Dashboard) { w(d, 0).ID = "W 1" }},
		{"duplicate widget id", wp + "1/id", IssueDuplicateID, func(d *Dashboard) { w(d, 1).ID = "w1" }},
		{"overlap", "/pages/0/layouts/lg/1", IssueLayoutOverlap, func(d *Dashboard) { d.Pages[0].Layouts.LG[1].X = 1 }},
		{"out of bounds", "/pages/0/layouts/lg/1", IssueLayoutBounds, func(d *Dashboard) { d.Pages[0].Layouts.LG[1].X = 11 }},
		{"below min size", "/pages/0/layouts/lg/0", IssueSize, func(d *Dashboard) { d.Pages[0].Layouts.LG[0].H = 1 }},
		{"missing layout", "/pages/0/layouts/lg", IssueLayoutMissing, func(d *Dashboard) { d.Pages[0].Layouts.LG = d.Pages[0].Layouts.LG[:1] }},
		{"control chars", wp + "0/title", IssueControlChars, func(d *Dashboard) { w(d, 0).Title = "Lamp\x07" }},
		{"url", wp + "0/title", IssueURLNotAllowed, func(d *Dashboard) { w(d, 0).Title = "see https://x" }},
		{"colour format", "/look/palette/snapshot/ink", IssueBadFormat, func(d *Dashboard) { d.Look.Palette.Snapshot["ink"] = "#FFF" }},
		{"unknown token", "/look/palette/snapshot/brand", IssueNotAllowed, func(d *Dashboard) { d.Look.Palette.Snapshot["brand"] = "#000000" }},
		{"custom without inputs", "/look/palette/custom", IssueRequired, func(d *Dashboard) { d.Look.Palette = Palette{Mode: "custom"} }},
		{"navigate missing page", wp + "0/actions/tap/page", IssuePageNotFound, func(d *Dashboard) { w(d, 0).Actions.Tap = &Action{Do: "navigate", Page: "nope"} }},
		{"toggle unsupported", wp + "1/actions/tap/do", IssueToggleUnsupported, func(d *Dashboard) { w(d, 1).Actions.Tap = &Action{Do: "toggle"} }},
		{"option range", wp + "1/options/decimals", IssueOutOfRange, func(d *Dashboard) { w(d, 1).Options["decimals"] = 9.0 }},
		{"unknown option", wp + "1/options/glow", IssueUnknownOption, func(d *Dashboard) { w(d, 1).Options["glow"] = true }},
		{"gauge min max", wp + "1/options/max", IssueBadOption, func(d *Dashboard) { w(d, 1).Options["max"] = -1.0 }},
		{"kiosk start page", "/kiosk/start_page", IssuePageNotFound, func(d *Dashboard) { d.Kiosk.StartPage = "nope" }},
		{"radius scale zero", "/look/radius_scale", IssueOutOfRange, func(d *Dashboard) { d.Look.RadiusScale = 0 }},
		{"radius scale too large", "/look/radius_scale", IssueOutOfRange, func(d *Dashboard) { d.Look.RadiusScale = 1.6 }},
		{"row unit zero", "/grid/row_unit", IssueOutOfRange, func(d *Dashboard) { d.Grid.RowUnit = RowUnit{PX: 0} }},
		{"row unit too small", "/grid/row_unit", IssueOutOfRange, func(d *Dashboard) { d.Grid.RowUnit = RowUnit{PX: 47} }},
		{"too many pages", "/pages", IssueTooMany, func(d *Dashboard) {
			for i := 0; i < 12; i++ {
				d.Pages = append(d.Pages, Page{ID: fmt.Sprintf("p%d", i), Name: "P", Widgets: []Widget{}, Layouts: Layouts{LG: []LayoutItem{}}})
			}
		}},
	} {
		d := validDoc()
		tc.mut(&d)
		r := Validate(d, Catalog(), nil)
		if tc.code == "" {
			if !r.OK() {
				t.Errorf("%s: unexpected issues %+v", tc.name, r.Issues)
			}
			continue
		}
		if !hasIssue(r.Issues, tc.path, tc.code) {
			t.Errorf("%s: want %s at %s, got %+v", tc.name, tc.code, tc.path, r.Issues)
		}
	}
	r := Validate(validDoc(), Catalog(), func(id EntityID) (Entity, bool) { return Entity{}, false })
	if !r.OK() || !hasIssue(r.Warnings, wp+"0/bindings/primary", WarnEntityUnknown) {
		t.Fatalf("unknown entity must be a warning only: %+v", r)
	}
}

// TestDecodeKeepsExplicitLookAndGridValues pins amendment A3: absent keys take their default,
// explicit out-of-range values reach Validate unchanged and produce exactly one issue.
func TestDecodeKeepsExplicitLookAndGridValues(t *testing.T) {
	base := func(t *testing.T) map[string]any {
		t.Helper()
		raw, err := json.Marshal(validDoc())
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	decode := func(t *testing.T, m map[string]any) Dashboard {
		t.Helper()
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		d, err := DecodeDashboard(raw)
		if err != nil {
			t.Fatal(err)
		}
		Normalize(&d)
		return d
	}
	// Absent keys, and absent whole objects, default.
	m := base(t)
	delete(m["look"].(map[string]any), "radius_scale")
	delete(m["grid"].(map[string]any), "row_unit")
	d := decode(t, m)
	if d.Look.RadiusScale != 1 || d.Grid.RowUnit != (RowUnit{Auto: true}) {
		t.Fatalf("absent keys: radius %v row_unit %+v", d.Look.RadiusScale, d.Grid.RowUnit)
	}
	if r := Validate(d, Catalog(), nil); !r.OK() {
		t.Fatalf("absent keys must validate: %+v", r.Issues)
	}
	m = base(t)
	delete(m, "grid")
	d = decode(t, m)
	if d.Grid.RowUnit != (RowUnit{Auto: true}) || d.Grid.Fit != "scroll" {
		t.Fatalf("absent grid object: %+v", d.Grid)
	}
	// Explicit values are rejected, never repaired.
	for _, tc := range []struct {
		name, path, group, key string
		val                    any
	}{
		{"radius 0", "/look/radius_scale", "look", "radius_scale", 0},
		{"radius 1.6", "/look/radius_scale", "look", "radius_scale", 1.6},
		{"row unit 0", "/grid/row_unit", "grid", "row_unit", 0},
		{"row unit 47", "/grid/row_unit", "grid", "row_unit", 47},
	} {
		m := base(t)
		m[tc.group].(map[string]any)[tc.key] = tc.val
		r := Validate(decode(t, m), Catalog(), nil)
		if len(r.Issues) != 1 || !hasIssue(r.Issues, tc.path, IssueOutOfRange) {
			t.Errorf("%s: want one %s issue, got %+v", tc.name, tc.path, r.Issues)
		}
	}
	// A numeric row unit survives the round trip.
	m = base(t)
	m["grid"].(map[string]any)["row_unit"] = 96
	if d := decode(t, m); d.Grid.RowUnit != (RowUnit{PX: 96}) {
		t.Fatalf("row_unit 96: %+v", d.Grid.RowUnit)
	}
	// Unknown fields stay rejected inside look and grid.
	m = base(t)
	m["look"].(map[string]any)["glow"] = true
	raw, _ := json.Marshal(m)
	if _, err := DecodeDashboard(raw); err == nil {
		t.Fatal("unknown look field accepted")
	}
}

func TestAutopackFillsMissingLayoutsDeterministically(t *testing.T) {
	d := validDoc()
	p := &d.Pages[0]
	p.Widgets = append(p.Widgets, Widget{ID: "w3", Type: "light", Variant: "slider", Bindings: map[string]string{"primary": "ha:light.a"}},
		Widget{ID: "w4", Type: "clock", Variant: "digital"})
	Autopack(&d, Catalog())
	want := []LayoutItem{{"w1", 0, 0, 2, 2}, {"w2", 2, 0, 2, 2}, {"w3", 4, 0, 4, 1}, {"w4", 8, 0, 2, 1}}
	if !slices.Equal(p.Layouts.LG, want) {
		t.Fatalf("lg = %+v, want %+v", p.Layouts.LG, want)
	}
	again := validDoc()
	again.Pages[0].Widgets = p.Widgets
	Autopack(&again, Catalog())
	if !slices.Equal(again.Pages[0].Layouts.LG, want) {
		t.Fatal("autopack is not deterministic")
	}
	if r := Validate(d, Catalog(), nil); !r.OK() {
		t.Fatalf("packed document invalid: %+v", r.Issues)
	}
}

func TestMigrateRejectsNewerSchema(t *testing.T) {
	d := validDoc()
	d.SchemaVersion = 2
	var he *Error
	if err := Migrate(&d); !errors.As(err, &he) || he.Code != CodeSchemaTooNew || d.SchemaVersion != 2 {
		t.Fatalf("newer schema: %v", err)
	}
	d.SchemaVersion = 0
	if err := Migrate(&d); err != nil || d.SchemaVersion != SchemaVersion {
		t.Fatalf("draft: %v %d", err, d.SchemaVersion)
	}
	raw, _ := json.Marshal(validDoc())
	// Clone first: appending to raw[:len-1] would overwrite the closing brace of raw itself.
	if _, err := DecodeDashboard(append(slices.Clone(raw[:len(raw)-1]), []byte(`,"extra":1}`)...)); err == nil {
		t.Fatal("unknown field accepted")
	}
	back, err := DecodeDashboard(raw)
	if err != nil || back.Grid.RowUnit != (RowUnit{Auto: true}) || back.Name != "Living room" {
		t.Fatalf("round trip: %v %+v", err, back.Grid)
	}
}

func TestApplyPatchOps(t *testing.T) {
	two, zero := 2, 0
	d, err := ApplyPatch(validDoc(), []PatchOp{
		{Op: "add_page", PageID: "kitchen", Name: "Kitchen"},
		{Op: "add_widget", PageID: "kitchen", Widget: &Widget{Type: "sensor", Variant: "numeral", Bindings: map[string]string{"primary": "ha:sensor.t"}}},
		{Op: "update_widget", WidgetID: "w1", Widget: &Widget{Title: "Desk lamp"}},
		{Op: "move_widget", WidgetID: "w2", X: &zero, Y: &two, W: &two, H: &two},
		{Op: "set_name", Name: "Home"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Home" || d.Pages[0].Widgets[0].Title != "Desk lamp" || len(d.Pages) != 2 || d.Pages[1].Widgets[0].ID != "w3" {
		t.Fatalf("patched: %+v", d)
	}
	Normalize(&d)
	Autopack(&d, Catalog())
	if r := Validate(d, Catalog(), nil); !r.OK() {
		t.Fatalf("patched document invalid: %+v", r.Issues)
	}
	for _, op := range []PatchOp{
		{Op: "remove_widget", WidgetID: "nope"},
		{Op: "remove_page", PageID: "main"},
		{Op: "add_widget", PageID: "main", Widget: &Widget{ID: "w1", Type: "clock", Variant: "digital"}},
		{Op: "move_widget", WidgetID: "w1", X: &zero},
		{Op: "explode"},
	} {
		var he *Error
		if _, err := ApplyPatch(validDoc(), []PatchOp{op}); !errors.As(err, &he) || he.Code != CodeInvalidPatch {
			t.Errorf("%s: %v", op.Op, err)
		}
	}
}
