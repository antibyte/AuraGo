package homedash

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Dashboard is the persisted dashboard document (main design section 4.2).
type Dashboard struct {
	SchemaVersion    int            `json:"schema_version"`
	ID               string         `json:"id"`
	Slug             string         `json:"slug"`
	Name             string         `json:"name"`
	Icon             string         `json:"icon,omitempty"`
	Revision         int64          `json:"revision"`
	ApprovedRevision int64          `json:"approved_revision"`
	UpdatedAt        time.Time      `json:"updated_at"`
	SavedBy          string         `json:"saved_by,omitempty"`
	Look             Look           `json:"look"`
	Grid             Grid           `json:"grid"`
	Pages            []Page         `json:"pages"`
	Kiosk            *KioskSettings `json:"kiosk,omitempty"`
}

// Look is the dashboard-wide visual setup. Absent radius_scale means 1; an explicit value is kept
// as written so that Validate can reject it.
type Look struct {
	Family      string   `json:"family"`
	Palette     Palette  `json:"palette"`
	Backdrop    Backdrop `json:"backdrop"`
	Density     string   `json:"density"`
	RadiusScale float64  `json:"radius_scale"`
}

// Palette selects the colour source.
type Palette struct {
	Mode     string            `json:"mode"`
	Custom   *CustomPalette    `json:"custom,omitempty"`   // only with mode custom
	Snapshot map[string]string `json:"snapshot,omitempty"` // only with mode theme
}

// CustomPalette holds the inputs of the client's palette solver.
type CustomPalette struct {
	Accent     string `json:"accent"`
	SurfaceHue int    `json:"surface_hue"`
	Scheme     string `json:"scheme"`
}

// Backdrop selects the page backdrop; colours come from the palette.
type Backdrop struct {
	Kind string `json:"kind"`
}

// Grid holds the row unit, gap and fit mode. Absent row_unit means "auto".
type Grid struct {
	RowUnit RowUnit `json:"row_unit"`
	Gap     *int    `json:"gap"`
	Fit     string  `json:"fit"`
}

// RowUnit is JSON "auto" or a whole number of pixels. An explicit number is kept as written
// (including 0), so Validate can reject it.
type RowUnit struct {
	Auto bool
	PX   int
}

// Page is one page of a dashboard.
type Page struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Icon    string   `json:"icon,omitempty"`
	Compact *bool    `json:"compact,omitempty"` // nil = true
	Widgets []Widget `json:"widgets"`
	Layouts Layouts  `json:"layouts"`
}

// Widget is one placed widget.
type Widget struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Variant  string            `json:"variant"`
	Family   string            `json:"family,omitempty"`
	Title    string            `json:"title,omitempty"`
	Icon     string            `json:"icon,omitempty"` // optional glyph name (IconNames); absent = the bound entity's icon_hint
	Bindings map[string]string `json:"bindings"`
	Options  map[string]any    `json:"options"`
	Actions  WidgetActions     `json:"actions"`
	Confirm  bool              `json:"confirm"`
}

// WidgetActions are the tap and hold behaviours.
type WidgetActions struct {
	Tap  *Action `json:"tap,omitempty"`
	Hold *Action `json:"hold,omitempty"`
}

// Action is {do} plus a page for navigate.
type Action struct {
	Do   string `json:"do"`
	Page string `json:"page,omitempty"`
}

// Layouts holds the per-breakpoint placements. md and sm are nil (automatic) or a manual override.
type Layouts struct {
	LG []LayoutItem `json:"lg"`
	MD []LayoutItem `json:"md,omitempty"`
	SM []LayoutItem `json:"sm,omitempty"`
}

// LayoutItem places one widget in cells.
type LayoutItem struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
	W  int    `json:"w"`
	H  int    `json:"h"`
}

// KioskSettings are the wall-tablet settings.
type KioskSettings struct {
	StartPage           string  `json:"start_page"`
	IdleDimSeconds      int     `json:"idle_dim_seconds"`
	IdleDimLevel        float64 `json:"idle_dim_level"`
	PageRotationSeconds int     `json:"page_rotation_seconds"`
	ScreenSaver         string  `json:"screen_saver"`
	ShowClock           *bool   `json:"show_clock,omitempty"`
}

func (r RowUnit) MarshalJSON() ([]byte, error) {
	if r.Auto {
		return []byte(`"auto"`), nil
	}
	return json.Marshal(r.PX)
}

func (r *RowUnit) UnmarshalJSON(b []byte) error {
	if string(b) == `"auto"` {
		*r = RowUnit{Auto: true}
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return errors.New(`row_unit must be "auto" or a whole number of pixels`)
	}
	*r = RowUnit{PX: n}
	return nil
}

// strictUnmarshal decodes like the top-level decoder: unknown fields are errors. Custom
// unmarshalers need it because a nested json.Unmarshal does not inherit DisallowUnknownFields.
func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// UnmarshalJSON starts from the defaults of absent keys (radius_scale 1) and keeps explicit values.
func (l *Look) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	type plain Look
	a := plain{RadiusScale: 1}
	if err := strictUnmarshal(b, &a); err != nil {
		return err
	}
	*l = Look(a)
	return nil
}

// UnmarshalJSON starts from the defaults of absent keys (row_unit "auto") and keeps explicit values.
func (g *Grid) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	type plain Grid
	a := plain{RowUnit: RowUnit{Auto: true}}
	if err := strictUnmarshal(b, &a); err != nil {
		return err
	}
	*g = Grid(a)
	return nil
}

func badJSON(msg string) *Error {
	return &Error{Code: CodeInvalidDocument, Message: "the document is not valid JSON for a dashboard",
		Issues: []Issue{{Path: "", Code: "bad_json", Message: msg}}}
}

// DecodeDashboard decodes a document strictly: unknown fields and trailing data are errors. It
// does not normalize; absent look.radius_scale and grid.row_unit decode to their defaults while
// explicit values are kept for Validate.
func DecodeDashboard(data []byte) (Dashboard, error) {
	if len(data) > Catalog().Limits.DocumentBytes {
		return Dashboard{}, &Error{Code: CodeLimitExceeded, Message: "the document is larger than 256 KiB"}
	}
	d := Dashboard{Look: Look{RadiusScale: 1}, Grid: Grid{RowUnit: RowUnit{Auto: true}}}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Dashboard{}, badJSON(err.Error())
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Dashboard{}, badJSON("unexpected data after the document")
	}
	return d, nil
}

// Normalize fills values that are absent. Option defaults are never written into documents, and
// explicit look.radius_scale and grid.row_unit values are left for Validate (the decoder already
// defaulted the absent ones).
func Normalize(d *Dashboard) {
	if d.SchemaVersion == 0 {
		d.SchemaVersion = SchemaVersion
	}
	if d.Look.Family == "" {
		d.Look.Family = "glass"
	}
	if d.Look.Palette.Mode == "" {
		d.Look.Palette.Mode = "theme"
	}
	if d.Look.Backdrop.Kind == "" {
		d.Look.Backdrop.Kind = "theme"
	}
	if d.Look.Density == "" {
		d.Look.Density = "comfortable"
	}
	if d.Grid.Fit == "" {
		d.Grid.Fit = "scroll"
	}
	if d.Pages == nil {
		d.Pages = []Page{}
	}
	for i := range d.Pages {
		p := &d.Pages[i]
		if p.Compact == nil {
			t := true
			p.Compact = &t
		}
		if p.Widgets == nil {
			p.Widgets = []Widget{}
		}
		if p.Layouts.LG == nil {
			p.Layouts.LG = []LayoutItem{}
		}
		for j := range p.Widgets {
			w := &p.Widgets[j]
			if w.Bindings == nil {
				w.Bindings = map[string]string{}
			}
			if w.Options == nil {
				w.Options = map[string]any{}
			}
			if w.Actions.Tap == nil {
				w.Actions.Tap = &Action{Do: "default"}
			}
			if w.Actions.Hold == nil {
				w.Actions.Hold = &Action{Do: "more_info"}
			}
		}
	}
	first := ""
	if len(d.Pages) > 0 {
		first = d.Pages[0].ID
	}
	if d.Kiosk == nil {
		t := true
		d.Kiosk = &KioskSettings{StartPage: first, IdleDimSeconds: 120, IdleDimLevel: 0.3, PageRotationSeconds: 0, ScreenSaver: "clock", ShowClock: &t}
		return
	}
	if d.Kiosk.StartPage == "" {
		d.Kiosk.StartPage = first
	}
	if d.Kiosk.ShowClock == nil {
		t := true
		d.Kiosk.ShowClock = &t
	}
}

// cloneDashboard deep-copies a document through JSON.
func cloneDashboard(d Dashboard) (Dashboard, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return Dashboard{}, fmt.Errorf("clone dashboard: %w", err)
	}
	var out Dashboard
	if err := json.Unmarshal(raw, &out); err != nil {
		return Dashboard{}, fmt.Errorf("clone dashboard: %w", err)
	}
	return out, nil
}
