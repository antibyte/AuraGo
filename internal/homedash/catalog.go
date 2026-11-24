package homedash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

// CatalogVersion and SchemaVersion are the widget catalog and dashboard document versions.
const (
	CatalogVersion = 1
	SchemaVersion  = 1
)

// WidgetCatalog is the Go source of truth for widget types, variants, bindings and options. It is
// served as JSON and pinned by testdata/widget_catalog.json.
type WidgetCatalog struct {
	Version       int              `json:"version"`
	Hash          string           `json:"hash"` // sha256 hex of the JSON with Hash ""
	SchemaVersion int              `json:"schema_version"`
	Grid          GridSpec         `json:"grid"`
	Limits        DocumentLimits   `json:"limits"`
	Families      []string         `json:"families"`
	Kinds         []Kind           `json:"kinds"`
	Capabilities  []CapabilityInfo `json:"capabilities"`
	PaletteTokens []string         `json:"palette_tokens"`
	CommonOptions []OptionSpec     `json:"common_options"`
	Widgets       []WidgetType     `json:"widgets"`
}

// GridColumns are the column counts per breakpoint.
type GridColumns struct {
	LG int `json:"lg"`
	MD int `json:"md"`
	SM int `json:"sm"`
}

// GridSpec holds the grid constants of schema version 1.
type GridSpec struct {
	Columns     GridColumns `json:"columns"`
	RowUnitMin  int         `json:"row_unit_min"`
	RowUnitMax  int         `json:"row_unit_max"`
	AutoCellMin int         `json:"auto_cell_min"`
	AutoCellMax int         `json:"auto_cell_max"`
	GapMax      int         `json:"gap_max"`
	MaxY        int         `json:"max_y"`
}

// DocumentLimits are the size limits of one document.
type DocumentLimits struct {
	Pages          int `json:"pages"`
	WidgetsPerPage int `json:"widgets_per_page"`
	Widgets        int `json:"widgets"`
	DocumentBytes  int `json:"document_bytes"`
	Bindings       int `json:"bindings"`
	Options        int `json:"options"`
}

// WidgetType is one widget type of the catalog.
type WidgetType struct {
	Type     string                  `json:"type"`
	Category string                  `json:"category"` // control|sensor|climate|media|camera|layout|status
	Variants []Variant               `json:"variants"`
	Bindings []BindingSpec           `json:"bindings"`
	Options  []OptionSpec            `json:"options"`
	MaxSize  Size                    `json:"max_size"`
	Commands map[string][]Capability `json:"commands"` // binding -> commands the widget may issue (kiosk AllowedSet)
}

// Variant is one look of a widget type. FamilyAlias: family -> what that family draws instead. A
// value naming another variant of the same type means the gallery hides this tile in that family;
// "scale" means the hairline scale primitive.
type Variant struct {
	ID          string            `json:"id"`
	MinSize     Size              `json:"min_size"`
	DefaultSize Size              `json:"default_size"`
	FamilyAlias map[string]string `json:"family_alias,omitempty"`
}

// BindingSpec declares one entity binding of a widget type.
type BindingSpec struct {
	Name          string       `json:"name"`
	Required      bool         `json:"required"`
	Kinds         []Kind       `json:"kinds"` // empty = any kind
	AnyCapability []Capability `json:"any_capability,omitempty"`
}

// OptionSpec declares one widget option. A nil Default means "unset".
type OptionSpec struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"` // bool|int|number|string|enum|color|page
	Default  any      `json:"default"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Step     *float64 `json:"step,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	MaxLen   int      `json:"max_len,omitempty"`
	Required bool     `json:"required,omitempty"`
}

// Size is a size in grid cells.
type Size struct {
	W int `json:"w"`
	H int `json:"h"`
}

var (
	catalogOnce  sync.Once
	catalogValue WidgetCatalog
)

// paletteTokens are the palette-owned hex colours of the visual system (minus the family-painted
// surface and the derived accent-soft).
var paletteTokens = []string{
	"canvas", "canvas-2", "scrim", "surface-solid", "surface-raised", "sunken", "field",
	"ink", "ink-muted", "ink-faint", "ink-on-accent", "on-ink", "selected-ink", "btn-ink",
	"accent", "accent-2", "on", "selected", "btn",
	"ok", "warn", "alert", "info", "pending", "unavailable",
	"ok-ink", "warn-ink", "alert-ink", "info-ink",
	"heat", "cool", "ct-2200", "ct-4000", "ct-6500", "light-on",
	"edge-tint", "shadow-ink", "hairline", "focus",
}

// Catalog returns the widget catalog, built once. Callers treat it as read-only.
func Catalog() WidgetCatalog {
	catalogOnce.Do(func() {
		c := WidgetCatalog{
			Version:       CatalogVersion,
			SchemaVersion: SchemaVersion,
			Grid:          GridSpec{Columns: GridColumns{LG: 12, MD: 8, SM: 4}, RowUnitMin: 48, RowUnitMax: 200, AutoCellMin: 56, AutoCellMax: 180, GapMax: 32, MaxY: 500},
			Limits:        DocumentLimits{Pages: 12, WidgetsPerPage: 120, Widgets: 400, DocumentBytes: 262144, Bindings: 8, Options: 32},
			Families:      []string{"glass", "tactile", "minimal", "instrument"},
			Kinds:         AllKinds(),
			Capabilities:  CapabilityRegistry(),
			PaletteTokens: append([]string(nil), paletteTokens...),
			CommonOptions: []OptionSpec{optColor("accent", "")},
			Widgets:       widgetTypes(),
		}
		c.Hash = catalogHash(c)
		catalogValue = c
	})
	return catalogValue
}

// Type finds a widget type.
func (c WidgetCatalog) Type(name string) (WidgetType, bool) {
	for _, t := range c.Widgets {
		if t.Type == name {
			return t, true
		}
	}
	return WidgetType{}, false
}

// Variant finds a variant of the type.
func (t WidgetType) Variant(id string) (Variant, bool) {
	for _, v := range t.Variants {
		if v.ID == id {
			return v, true
		}
	}
	return Variant{}, false
}

// Binding finds a binding declaration of the type.
func (t WidgetType) Binding(name string) (BindingSpec, bool) {
	for _, b := range t.Bindings {
		if b.Name == name {
			return b, true
		}
	}
	return BindingSpec{}, false
}

// Option finds an option of the type, then of the common options.
func (c WidgetCatalog) Option(t WidgetType, key string) (OptionSpec, bool) {
	for _, o := range t.Options {
		if o.Key == key {
			return o, true
		}
	}
	for _, o := range c.CommonOptions {
		if o.Key == key {
			return o, true
		}
	}
	return OptionSpec{}, false
}

// MarshalGolden renders the catalog as the golden file content.
func (c WidgetCatalog) MarshalGolden() ([]byte, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// catalogHash is the sha256 of the compact JSON with Hash empty.
func catalogHash(c WidgetCatalog) string {
	c.Hash = ""
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
