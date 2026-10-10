package cydcreator

import (
	"slices"
	"sync"
)

// FontFamilies are the LVGL fonts the builder image converts (spec §4).
// Read-only: validation uses these slices; copy before modifying.
var FontFamilies = []string{"montserrat", "inter", "jetbrains_mono"}

// FontSizes are the allowed pixel sizes of every font family.
// Read-only: validation uses these slices; copy before modifying.
var FontSizes = []int{12, 14, 16, 20, 24, 28, 32, 40, 48}

// Symbols are the button symbols; each maps 1:1 to LV_SYMBOL_<UPPER>.
// Read-only: validation uses these slices; copy before modifying.
var Symbols = []string{
	"audio", "video", "list", "ok", "close", "power", "settings", "home", "download",
	"drive", "refresh", "mute", "volume_mid", "volume_max", "image", "tint", "prev", "play", "pause", "stop",
	"next", "eject", "left", "right", "plus", "minus", "eye_open", "eye_close", "warning", "shuffle", "up",
	"down", "loop", "directory", "upload", "call", "cut", "copy", "save", "bars", "envelope", "charge", "paste",
	"bell", "keyboard", "gps", "file", "wifi", "battery_full", "battery_3", "battery_2", "battery_1",
	"battery_empty", "usb", "bluetooth", "trash", "edit", "backspace", "sd_card", "new_line",
}

// Prop kinds.
const (
	PropString   = "string"    // one line, ≤ MaxLen bytes
	PropText     = "text"      // multi-line, ≤ MaxLen bytes
	PropInt      = "int"       // integer in [Min, Max]; ±1,000,000 when unbounded
	PropBool     = "bool"      //
	PropEnum     = "enum"      // one of Options
	PropColor    = "color"     // "#RRGGBB" or "" (theme)
	PropFont     = "font"      // one of FontFamilies or "" (theme)
	PropFontSize = "font_size" // one of FontSizes or 0 (theme)
	PropAsset    = "asset"     // asset id or ""
	PropList     = "list"      // list of strings, ≤ MaxLen items, each ≤ MaxTextBytes bytes
	PropSymbol   = "symbol"    // one of Symbols or ""
)

// PropSpec describes one widget prop. For string/text kinds MaxLen is the byte
// limit; for list kinds it is the maximum number of items.
type PropSpec struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Default any      `json:"default"`
	Options []string `json:"options,omitempty"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	MaxLen  int      `json:"max_len,omitempty"`
}

// WidgetSpec is one catalog entry. DefaultW 0 means "full parent width".
type WidgetSpec struct {
	Type      string     `json:"type"`
	Group     string     `json:"group"` // basic input display structure composite
	DefaultW  int        `json:"default_w"`
	DefaultH  int        `json:"default_h"`
	Container bool       `json:"container"`
	Events    []string   `json:"events"`
	Props     []PropSpec `json:"props"`
}

var (
	catalogOnce sync.Once
	catalogList []WidgetSpec
	catalogMap  map[string]*WidgetSpec
)

func loadCatalog() {
	catalogOnce.Do(func() {
		catalogList = buildWidgetCatalog()
		catalogMap = make(map[string]*WidgetSpec, len(catalogList))
		for i := range catalogList {
			catalogMap[catalogList[i].Type] = &catalogList[i]
		}
	})
}

// WidgetCatalog returns a deep copy of the catalog in palette order.
func WidgetCatalog() []WidgetSpec {
	loadCatalog()
	out := make([]WidgetSpec, len(catalogList))
	for i, s := range catalogList {
		out[i] = cloneWidgetSpec(s)
	}
	return out
}

// WidgetSpecFor returns a deep copy of the spec of one widget type.
func WidgetSpecFor(typ string) (WidgetSpec, bool) {
	s, ok := widgetSpecRef(typ)
	if !ok {
		return WidgetSpec{}, false
	}
	return cloneWidgetSpec(*s), true
}

// widgetSpecRef returns the shared cached spec; callers must not modify it.
// Events, Options and []string defaults are shared read-only data too — clone
// them before modifying.
func widgetSpecRef(typ string) (*WidgetSpec, bool) {
	loadCatalog()
	s, ok := catalogMap[typ]
	return s, ok
}

func (s *WidgetSpec) prop(name string) (*PropSpec, bool) {
	for i := range s.Props {
		if s.Props[i].Name == name {
			return &s.Props[i], true
		}
	}
	return nil, false
}

func cloneWidgetSpec(s WidgetSpec) WidgetSpec {
	out := s
	out.Events = slices.Clone(s.Events)
	out.Props = make([]PropSpec, len(s.Props))
	for i, p := range s.Props {
		p.Options = slices.Clone(p.Options)
		p.Min = dupIntPtr(p.Min)
		p.Max = dupIntPtr(p.Max)
		if list, ok := p.Default.([]string); ok {
			p.Default = slices.Clone(list)
		}
		out.Props[i] = p
	}
	return out
}

func propBound(v int) *int { return &v }

func pString(name, def string) PropSpec {
	return PropSpec{Name: name, Kind: PropString, Default: def, MaxLen: MaxTextBytes}
}
func pText(name, def string) PropSpec {
	return PropSpec{Name: name, Kind: PropText, Default: def, MaxLen: MaxTextBytes}
}
func pInt(name string, def int) PropSpec { return PropSpec{Name: name, Kind: PropInt, Default: def} }
func pIntRange(name string, def, lo, hi int) PropSpec {
	return PropSpec{Name: name, Kind: PropInt, Default: def, Min: propBound(lo), Max: propBound(hi)}
}
func pBool(name string, def bool) PropSpec { return PropSpec{Name: name, Kind: PropBool, Default: def} }
func pEnum(name, def string, options ...string) PropSpec {
	return PropSpec{Name: name, Kind: PropEnum, Default: def, Options: options}
}
func pList(name string, maxItems int, def ...string) PropSpec {
	if def == nil {
		def = []string{}
	}
	def = slices.Clone(def)
	return PropSpec{Name: name, Kind: PropList, Default: def, MaxLen: maxItems}
}
func pColor(name string) PropSpec    { return PropSpec{Name: name, Kind: PropColor, Default: ""} }
func pFont(name string) PropSpec     { return PropSpec{Name: name, Kind: PropFont, Default: ""} }
func pFontSize(name string) PropSpec { return PropSpec{Name: name, Kind: PropFontSize, Default: 0} }
func pAsset(name string) PropSpec    { return PropSpec{Name: name, Kind: PropAsset, Default: ""} }
func pSymbol(name string) PropSpec   { return PropSpec{Name: name, Kind: PropSymbol, Default: ""} }

func catalogEntry(typ, group string, w, h int, container bool, events []string, props ...PropSpec) WidgetSpec {
	if events == nil {
		events = []string{}
	}
	events = slices.Clone(events)
	if props == nil {
		props = []PropSpec{}
	}
	return WidgetSpec{Type: typ, Group: group, DefaultW: w, DefaultH: h, Container: container, Events: events, Props: props}
}

// buildWidgetCatalog is the fixed catalog from the overview "Widget catalog" table.
func buildWidgetCatalog() []WidgetSpec {
	press := []string{"clicked", "long_pressed"}
	changed := []string{"value_changed"}
	threeOptions := []string{"Eins", "Zwei", "Drei"}
	return []WidgetSpec{
		catalogEntry("label", "basic", 120, 24, false, press,
			pText("text", "Label"), pFont("font"), pFontSize("size"),
			pEnum("align", "left", "left", "center", "right"),
			pEnum("long_mode", "wrap", "wrap", "dots", "scroll", "clip")),
		catalogEntry("button", "basic", 120, 40, false, press,
			pString("text", "Button"), pSymbol("symbol"), pFont("font"), pFontSize("size")),
		catalogEntry("image", "basic", 64, 64, false, press, pAsset("asset")),
		catalogEntry("panel", "basic", 160, 100, true, press, pBool("scrollable", false)),
		catalogEntry("line", "basic", 120, 8, false, nil,
			pEnum("orientation", "horizontal", "horizontal", "vertical"), pIntRange("thickness", 2, 1, 16)),
		catalogEntry("switch", "input", 56, 28, false, changed, pBool("checked", false)),
		catalogEntry("checkbox", "input", 140, 28, false, changed, pString("text", "Option"), pBool("checked", false)),
		catalogEntry("slider", "input", 160, 24, false, changed, pInt("min", 0), pInt("max", 100), pInt("value", 50)),
		catalogEntry("arc", "input", 120, 120, false, changed, pInt("min", 0), pInt("max", 100), pInt("value", 50)),
		catalogEntry("dropdown", "input", 140, 40, false, changed, pList("options", 64, threeOptions...), pInt("selected", 0)),
		catalogEntry("roller", "input", 120, 100, false, changed,
			pList("options", 64, threeOptions...), pInt("selected", 0), pIntRange("visible_rows", 3, 1, 7)),
		catalogEntry("textarea", "input", 200, 40, false, []string{"value_changed", "ready"},
			pString("text", ""), pString("placeholder", ""), pBool("one_line", true), pBool("password", false),
			pIntRange("max_length", 64, 1, 256), pEnum("keyboard", "text", "text", "number")),
		catalogEntry("spinbox", "input", 140, 40, false, changed,
			pInt("min", 0), pInt("max", 100), pInt("value", 0), pIntRange("step", 1, 1, 1000),
			pIntRange("digits", 3, 1, 8), pIntRange("decimals", 0, 0, 3)),
		catalogEntry("keypad", "input", 200, 160, false, changed,
			pEnum("layout", "numeric", "numeric", "numeric_ok", "custom"), pList("keys", 64)),
		catalogEntry("bar", "display", 160, 20, false, nil, pInt("min", 0), pInt("max", 100), pInt("value", 30)),
		catalogEntry("spinner", "display", 48, 48, false, nil),
		catalogEntry("led", "display", 24, 24, false, nil,
			pColor("color"), pIntRange("brightness", 255, 0, 255), pBool("on", true)),
		catalogEntry("chart", "display", 200, 120, false, nil,
			pEnum("kind", "line", "line", "bar"), pIntRange("points", 10, 2, 100),
			pInt("min", 0), pInt("max", 100), pList("series", 8, "Wert")),
		catalogEntry("gauge", "display", 140, 140, false, nil,
			pInt("min", 0), pInt("max", 100), pInt("value", 40),
			pIntRange("ticks", 11, 2, 41), pIntRange("label_every", 2, 1, 10)),
		catalogEntry("table", "display", 200, 120, false, nil,
			pIntRange("columns", 2, 1, 8), pIntRange("rows", 3, 1, 32), pList("cells", 256)),
		catalogEntry("qrcode", "display", 100, 100, false, nil, pString("text", "https://example.com")),
		catalogEntry("tabview", "structure", 300, 200, true, changed,
			pList("tabs", 8, "Tab 1", "Tab 2"), pEnum("bar_position", "top", "top", "bottom", "left", "right"),
			pIntRange("bar_size", 40, 20, 80)),
		catalogEntry("list", "structure", 200, 160, false, []string{"clicked"}, pList("items", 64, "Eintrag 1", "Eintrag 2")),
		catalogEntry("msgbox", "structure", 240, 140, false, []string{"clicked"},
			pString("title", "Hinweis"), pText("text", ""), pList("buttons", 4, "OK")),
		catalogEntry("canvas", "structure", 100, 100, false, nil),
		catalogEntry("statusbar", "composite", 0, 24, false, []string{"long_pressed"},
			pString("title", ""), pBool("show_clock", true), pBool("show_wifi", true)),
	}
}

// Library is one curated PlatformIO library the builder image pre-installs.
type Library struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Version string   `json:"version"`
	PIOSpec string   `json:"pio_spec"` // exact lib_deps entry, identical to the builder image
	Depends []string `json:"depends,omitempty"`
}

// Libraries returns the curated libraries. Versions were checked against the
// PlatformIO registry on 2026-10-10; plan 1b pins the identical specs (PIOSpec
// and every Depends entry) in the builder image, and exports pin them in
// platformio.ini. Depends lists exact lib_deps specs of required dependencies
// so offline builds resolve them from the image.
func Libraries() []Library {
	const unifiedSensor = "adafruit/Adafruit Unified Sensor@1.1.15"
	const busIO = "adafruit/Adafruit BusIO@1.17.4"
	return []Library{
		{ID: "arduinojson", Name: "ArduinoJson", Version: "7.4.3", PIOSpec: "bblanchon/ArduinoJson@7.4.3"},
		{ID: "pubsubclient", Name: "PubSubClient", Version: "2.8", PIOSpec: "knolleary/PubSubClient@2.8"},
		{ID: "onewire", Name: "OneWire", Version: "2.3.8", PIOSpec: "paulstoffregen/OneWire@2.3.8"},
		{ID: "dallastemperature", Name: "DallasTemperature", Version: "4.0.6",
			PIOSpec: "milesburton/DallasTemperature@4.0.6", Depends: []string{"paulstoffregen/OneWire@2.3.8"}},
		{ID: "dht", Name: "DHT sensor library", Version: "1.4.7",
			PIOSpec: "adafruit/DHT sensor library@1.4.7", Depends: []string{unifiedSensor}},
		{ID: "bme280", Name: "Adafruit BME280 Library", Version: "2.3.0",
			PIOSpec: "adafruit/Adafruit BME280 Library@2.3.0", Depends: []string{busIO, unifiedSensor}},
		{ID: "bmp280", Name: "Adafruit BMP280 Library", Version: "3.0.0",
			PIOSpec: "adafruit/Adafruit BMP280 Library@3.0.0", Depends: []string{busIO, unifiedSensor}},
		{ID: "neopixel", Name: "Adafruit NeoPixel", Version: "1.15.5", PIOSpec: "adafruit/Adafruit NeoPixel@1.15.5"},
	}
}

func libraryByID(id string) (Library, bool) {
	for _, l := range Libraries() {
		if l.ID == id {
			return l, true
		}
	}
	return Library{}, false
}
