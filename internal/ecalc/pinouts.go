package ecalc

import (
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Pinout items: a catalog item of kind "pinout" (pinouts/*.yaml) describes a connector, a board header or an IC package:
// the pins with number, name, function, type and optionally the wire colour, and the shape that the page draws. The
// loader checks every rule; the drawing itself is in the page (ecalc-pinout-draw.js).

const pinoutsCategory = "pinouts"

// PinoutGroups are the list sections of the pinout rail, in display order.
var PinoutGroups = []string{"data", "audio", "pc_power", "boards", "debug", "ics"}

// PinoutOutlines are the outlines of a "rows" shape (sorted); the page draws every one of them.
var PinoutOutlines = []string{
	"displayport", "dsub", "hdmi_a", "jst", "minifit", "molex", "obd2", "rect", "rj45", "sata_data", "sata_power",
	"usb_a", "usb_b", "usb_c", "usb_micro_b", "usb_mini_b",
}

var (
	pinoutContacts   = []string{"round", "square", "blade"}
	pinoutDrawn      = []string{"plug", "socket"}
	pinoutRowKeys    = []string{"none", "top", "bottom", "left", "right"}
	pinoutConnectors = []string{"top", "bottom", "left", "right", "none"}
	pinoutPackages   = []string{"to92", "to220"}
	pinoutShapeTypes = []string{"rows", "dip", "to", "circular", "jack", "board"}
)

const (
	maxPinNumberLen = 8
	maxPinNameLen   = 24
	maxRowOffset    = 2.0
)

// Pin is one pin of a connector. Function and Notes are localized; Wire is an id of pinstyles.yaml or "".
type Pin struct {
	Number, Name, Type, Wire string
	Function, Notes          Text
}

// PinoutVariant is one variant of a pinout (T568A and T568B, CTIA and OMTP): the same pin numbers, other pins.
type PinoutVariant struct {
	ID   string
	Name Text
	Pins []Pin
}

// PinoutRow is one row of a "rows" shape or of an inner header, as seen looking at the mating face: Pins are the pin
// numbers left to right, "" is an empty position, Offset is the shift to the right in pitches.
type PinoutRow struct {
	Pins   []string
	Offset float64
}

// PinoutPosition places a pin of a "circular" shape: Angle in degrees clockwise from 12 o'clock, Radius from 0 (centre)
// to 1 (the shell).
type PinoutPosition struct {
	Pin           string
	Angle, Radius float64
}

// PinoutInner is an extra header on a "board" shape with a neutral label (ICSP).
type PinoutInner struct {
	Label string
	Rows  []PinoutRow
}

// PinoutShape is the typed shape. Type is rows, dip, to, circular, jack or board; the fields of the other types are zero.
//
//	rows:     Outline, Contact, Drawn (plug|socket), Rows, Key (none|top|bottom|left|right, none unless the outline is rect)
//	dip:      DipPins (even, 4 to 64; the pins are "1" ... "N")
//	to:       Package (to92|to220), ToPins (left to right)
//	circular: Contact, Drawn (plug|socket), KeyAngle (nil = no notch), Positions
//	jack:     Segments (tip to sleeve, 2 to 5)
//	board:    Left, Right (top to bottom), Top, Bottom (left to right), "" = a gap of one pitch; Connector, Inner
type PinoutShape struct {
	Type, Outline, Contact, Drawn, Key string
	KeyAngle                           *float64
	Rows                               []PinoutRow
	DipPins                            int
	Package                            string
	ToPins                             []string
	Positions                          []PinoutPosition
	Segments                           []string
	Left, Right, Top, Bottom           []string
	Connector                          string
	Inner                              []PinoutInner
}

// Numbers lists every pin number that the shape names, in drawing order (a dip as "1" ... "N"); gaps are left out.
func (s PinoutShape) Numbers() []string {
	var out []string
	add := func(list []string) {
		for _, n := range list {
			if n != "" {
				out = append(out, n)
			}
		}
	}
	rows := func(rows []PinoutRow) {
		for _, r := range rows {
			add(r.Pins)
		}
	}
	switch s.Type {
	case "rows":
		rows(s.Rows)
	case "dip":
		for i := 1; i <= s.DipPins; i++ {
			out = append(out, strconv.Itoa(i))
		}
	case "to":
		add(s.ToPins)
	case "circular":
		for _, p := range s.Positions {
			out = append(out, p.Pin)
		}
	case "jack":
		add(s.Segments)
	case "board":
		add(s.Left)
		add(s.Right)
		add(s.Top)
		add(s.Bottom)
		for _, in := range s.Inner {
			rows(in.Rows)
		}
	}
	return out
}

// Pinout is a catalog item of kind "pinout". Views is nil when the shape has one fixed view; Pins is empty when the
// item has Variants.
type Pinout struct {
	ID, Category, Group string
	Name, Doc, Notes    Text
	Tags                []string
	Source              string
	Views               map[string]Text // "plug" and "socket" captions, nil when the shape has one view
	Shape               PinoutShape
	Pins                []Pin
	Variants            []PinoutVariant
}

// PinsOf returns the pins of a variant, "" meaning the pins of the item or the first variant, together with the id of the
// variant used. ok is false for a variant the item does not have.
func (p *Pinout) PinsOf(variant string) (pins []Pin, id string, ok bool) {
	if len(p.Variants) == 0 {
		return p.Pins, "", variant == ""
	}
	if variant == "" {
		return p.Variants[0].Pins, p.Variants[0].ID, true
	}
	for _, v := range p.Variants {
		if v.ID == variant {
			return v.Pins, v.ID, true
		}
	}
	return nil, "", false
}

// Pinout returns the pinout item with the given id, or nil.
func (c *Catalog) Pinout(id string) *Pinout { return c.pinoutByID[id] }

type rawPinRow struct {
	Pins   []*string `yaml:"pins"`
	Offset float64   `yaml:"offset"`
}

type rawPosition struct {
	Pin    string   `yaml:"pin"`
	Angle  *float64 `yaml:"angle"`
	Radius *float64 `yaml:"radius"`
}

type rawInner struct {
	Label string      `yaml:"label"`
	Rows  []rawPinRow `yaml:"rows"`
}

type rawPinoutShape struct {
	Type      string        `yaml:"type"`
	Outline   string        `yaml:"outline"`
	Contact   string        `yaml:"contact"`
	Drawn     string        `yaml:"drawn"`
	Key       yaml.Node     `yaml:"key"`
	Rows      []rawPinRow   `yaml:"rows"`
	Pins      yaml.Node     `yaml:"pins"`
	Package   string        `yaml:"package"`
	Positions []rawPosition `yaml:"positions"`
	Segments  []string      `yaml:"segments"`
	Left      []*string     `yaml:"left"`
	Right     []*string     `yaml:"right"`
	Top       []*string     `yaml:"top"`
	Bottom    []*string     `yaml:"bottom"`
	Connector string        `yaml:"connector"`
	Inner     []rawInner    `yaml:"inner"`
}

type rawPin struct {
	Number   string `yaml:"number"`
	Name     string `yaml:"name"`
	Function Text   `yaml:"function"`
	Type     string `yaml:"type"`
	Wire     string `yaml:"wire"`
	Notes    Text   `yaml:"notes"`
}

type rawVariant struct {
	ID   string   `yaml:"id"`
	Name Text     `yaml:"name"`
	Pins []rawPin `yaml:"pins"`
}

type rawPinout struct {
	ID       string          `yaml:"id"`
	Kind     string          `yaml:"kind"`
	Category string          `yaml:"category"`
	Group    string          `yaml:"group"`
	Name     Text            `yaml:"name"`
	Doc      Text            `yaml:"doc"`
	Notes    Text            `yaml:"notes"`
	Tags     []string        `yaml:"tags"`
	Source   string          `yaml:"source"`
	Views    map[string]Text `yaml:"views"`
	Shape    rawPinoutShape  `yaml:"shape"`
	Pins     []rawPin        `yaml:"pins"`
	Variants []rawVariant    `yaml:"variants"`
}

// shapeFields are the fields of a shape that each type uses.
var shapeFields = map[string][]string{
	"rows":     {"outline", "contact", "drawn", "key", "rows"},
	"dip":      {"pins"},
	"to":       {"package", "pins"},
	"circular": {"contact", "drawn", "key", "positions"},
	"jack":     {"segments"},
	"board":    {"left", "right", "top", "bottom", "connector", "inner"},
}

// fieldsSet lists the fields that a shape has written down.
func (s *rawPinoutShape) fieldsSet() map[string]bool {
	return map[string]bool{
		"outline": s.Outline != "", "contact": s.Contact != "", "drawn": s.Drawn != "", "key": s.Key.Kind != 0,
		"rows": s.Rows != nil, "pins": s.Pins.Kind != 0, "package": s.Package != "", "positions": s.Positions != nil,
		"segments": s.Segments != nil, "left": s.Left != nil, "right": s.Right != nil, "top": s.Top != nil, "bottom": s.Bottom != nil,
		"connector": s.Connector != "", "inner": s.Inner != nil,
	}
}

// pinouts reads pinouts/*.yaml (sorted) into the catalog.
func (l *loader) pinouts(fsys fs.FS, cat *Catalog) error {
	files, err := fs.Glob(fsys, "pinouts/*.yaml")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		raws, ok := readItems[rawPinout](l, fsys, file)
		if !ok {
			continue
		}
		for i := range raws {
			if p := l.pinout(file, &raws[i], cat); p != nil {
				cat.Pinouts = append(cat.Pinouts, p)
				cat.pinoutByID[p.ID] = p
				cat.kinds[p.ID] = "pinout"
			}
		}
	}
	return nil
}

// pinout builds and validates one pinout item; it returns nil when something is wrong.
func (l *loader) pinout(file string, r *rawPinout, cat *Catalog) *Pinout {
	before := len(l.errs)
	label := r.ID
	if label == "" {
		label = "(item without id)"
	}
	fail := func(format string, args ...any) {
		l.errs = append(l.errs, fmt.Errorf("%s: %s: %s", file, label, fmt.Sprintf(format, args...)))
	}
	text := func(what string, t Text, required []string) {
		l.checkText(fmt.Sprintf("%s: %s: %s", file, label, what), t, required)
	}
	p := &Pinout{ID: r.ID, Category: r.Category, Group: r.Group, Name: r.Name, Doc: r.Doc, Notes: r.Notes, Tags: r.Tags, Source: r.Source}
	l.claimID(r.ID, file, fail)
	if r.Kind != "pinout" {
		fail("kind must be pinout in pinouts/, got %q", r.Kind)
	}
	if r.Category != pinoutsCategory {
		fail("category must be %s, got %q", pinoutsCategory, r.Category)
	}
	if !contains(PinoutGroups, r.Group) {
		fail("group must be one of %s, got %q", strings.Join(PinoutGroups, ", "), r.Group)
	}
	l.commonFields(fail, text, r.Name, r.Doc, r.Tags, r.Source)
	if len(r.Notes) > 0 {
		text("notes", r.Notes, docLocales)
	}

	shapeProblems := 0
	shape, shapeNumbers := buildShape(&r.Shape, func(format string, args ...any) {
		shapeProblems++
		fail(format, args...)
	})
	p.Shape = shape

	// The captions of the two views belong to the shapes that can be drawn as plug and as socket.
	twoViews := shape.Drawn == "plug" || shape.Drawn == "socket"
	switch {
	case twoViews && len(r.Views) == 0:
		fail("views are required when the shape is drawn as plug or socket")
	case !twoViews && len(r.Views) > 0:
		fail("views are only for shapes that are drawn as plug or socket")
	case twoViews:
		for _, v := range pinoutDrawn {
			if len(r.Views[v]) == 0 {
				fail("views: missing %q caption", v)
			} else {
				text("views."+v, r.Views[v], docLocales)
			}
		}
		for v := range r.Views {
			if !contains(pinoutDrawn, v) {
				fail("views: unknown view %q (plug, socket)", v)
			}
		}
		p.Views = r.Views
	}

	// Pins or variants.
	var pinLists [][]Pin
	var listNames []string
	switch {
	case len(r.Pins) > 0 && len(r.Variants) > 0:
		fail("use either pins or variants, not both")
	case len(r.Pins) == 0 && len(r.Variants) == 0:
		fail("pins or variants are required")
	case len(r.Pins) > 0:
		p.Pins = l.pins(fail, text, "pins", r.Pins, cat)
		pinLists, listNames = [][]Pin{p.Pins}, []string{"pins"}
	default:
		if len(r.Variants) < 2 {
			fail("variants needs at least 2 variants (use pins for a single one)")
		}
		seen := map[string]bool{}
		for i := range r.Variants {
			rv := &r.Variants[i]
			what := "variant " + rv.ID
			switch {
			case !idRe.MatchString(rv.ID):
				fail("variant id %q must match %s", rv.ID, idRe)
				what = fmt.Sprintf("variant %d", i+1)
			case seen[rv.ID]:
				fail("variant id %q is used twice", rv.ID)
			}
			seen[rv.ID] = true
			text(what+": name", rv.Name, docLocales)
			v := PinoutVariant{ID: rv.ID, Name: rv.Name, Pins: l.pins(fail, text, what+": pins", rv.Pins, cat)}
			p.Variants = append(p.Variants, v)
			pinLists, listNames = append(pinLists, v.Pins), append(listNames, what)
		}
	}

	// Every variant has the same pin numbers, and the shape draws exactly those.
	if len(pinLists) > 0 {
		base := numbersOf(pinLists[0])
		for i := 1; i < len(pinLists); i++ {
			if missing, extra := diffNumbers(base, numbersOf(pinLists[i])); len(missing)+len(extra) > 0 {
				fail("%s has other pin numbers than %s (missing %s; extra %s)", listNames[i], listNames[0], listOrNone(missing), listOrNone(extra))
			}
		}
		if shapeProblems == 0 { // a broken shape would only add noise
			coverage(fail, shapeNumbers, base)
		}
	}
	if len(l.errs) > before {
		return nil
	}
	return p
}

func listOrNone(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ", ")
}

func numbersOf(pins []Pin) []string {
	out := make([]string, len(pins))
	for i, p := range pins {
		out[i] = p.Number
	}
	return out
}

// diffNumbers returns the numbers of want that are not in got, and those of got that are not in want.
func diffNumbers(want, got []string) (missing, extra []string) {
	in := func(list []string, s string) bool { return contains(list, s) }
	for _, n := range want {
		if !in(got, n) {
			missing = append(missing, n)
		}
	}
	for _, n := range got {
		if !in(want, n) {
			extra = append(extra, n)
		}
	}
	return missing, extra
}

// coverage checks that every pin is drawn exactly once and that every position of the shape names a pin.
func coverage(fail func(string, ...any), shape, pins []string) {
	seen := map[string]bool{}
	for _, n := range shape {
		if seen[n] {
			fail("shape: pin %q is drawn more than once", n)
		}
		seen[n] = true
	}
	for _, n := range shape {
		if !contains(pins, n) {
			fail("shape: position %q names no pin", n)
			pins = append(pins[:len(pins):len(pins)], n) // report it once
		}
	}
	for _, n := range pins {
		if !seen[n] {
			fail("shape: pin %q is not drawn", n)
		}
	}
}

// pins validates and converts one list of pins; what names the list in messages.
func (l *loader) pins(fail func(string, ...any), text func(string, Text, []string), what string, raws []rawPin, cat *Catalog) []Pin {
	out := make([]Pin, 0, len(raws))
	seen := map[string]bool{}
	for i, rp := range raws {
		label := fmt.Sprintf("%s: pin %q", what, rp.Number)
		if rp.Number == "" {
			label = fmt.Sprintf("%s: pin %d", what, i+1)
		}
		switch n := utf8.RuneCountInString(rp.Number); {
		case strings.TrimSpace(rp.Number) != rp.Number || n == 0 || n > maxPinNumberLen:
			fail("%s: number must be 1 to %d characters without surrounding spaces", label, maxPinNumberLen)
		case seen[rp.Number]:
			fail("%s: number is used twice", label)
		}
		seen[rp.Number] = true
		if n := utf8.RuneCountInString(rp.Name); strings.TrimSpace(rp.Name) != rp.Name || n == 0 || n > maxPinNameLen {
			fail("%s: name must be 1 to %d characters without surrounding spaces", label, maxPinNameLen)
		}
		text(label+": function", rp.Function, docLocales)
		if len(rp.Notes) > 0 {
			text(label+": notes", rp.Notes, docLocales)
		}
		if !contains(PinTypes, rp.Type) {
			fail("%s: type must be one of %s, got %q", label, strings.Join(PinTypes, ", "), rp.Type)
		}
		if rp.Wire != "" && cat.wireByID[rp.Wire] == nil {
			fail("%s: wire %q is not in pinstyles.yaml (%s)", label, rp.Wire, strings.Join(wireIDs, ", "))
		}
		out = append(out, Pin{Number: rp.Number, Name: rp.Name, Type: rp.Type, Wire: rp.Wire, Function: rp.Function, Notes: rp.Notes})
	}
	return out
}

// buildShape validates a raw shape and returns the typed one together with every pin number that it names.
func buildShape(raw *rawPinoutShape, fail func(string, ...any)) (PinoutShape, []string) {
	s := PinoutShape{Type: raw.Type}
	if !contains(pinoutShapeTypes, raw.Type) {
		fail("shape: type must be one of %s, got %q", strings.Join(pinoutShapeTypes, ", "), raw.Type)
		return s, nil
	}
	set := raw.fieldsSet()
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if set[name] && !contains(shapeFields[raw.Type], name) {
			fail("shape: field %s is not used by type %s", name, raw.Type)
		}
	}
	need := func(name string) bool {
		if !set[name] {
			fail("shape: type %s needs %s", raw.Type, name)
		}
		return set[name]
	}
	oneOf := func(what, v string, list []string) string {
		if !contains(list, v) {
			fail("shape: %s must be one of %s, got %q", what, strings.Join(list, ", "), v)
		}
		return v
	}

	switch raw.Type {
	case "rows":
		s.Outline, s.Contact, s.Drawn, s.Key = raw.Outline, raw.Contact, raw.Drawn, "none"
		if need("outline") {
			oneOf("outline", raw.Outline, PinoutOutlines)
		}
		if need("contact") {
			oneOf("contact", raw.Contact, pinoutContacts)
		}
		if need("drawn") {
			oneOf("drawn", raw.Drawn, pinoutDrawn)
		}
		if set["key"] {
			var key string
			if raw.Key.Kind != yaml.ScalarNode || raw.Key.Decode(&key) != nil {
				fail("shape: key must be one of %s", strings.Join(pinoutRowKeys, ", "))
			} else {
				s.Key = oneOf("key", key, pinoutRowKeys)
				if key != "none" && raw.Outline != "rect" {
					fail("shape: key %s needs the outline rect", key)
				}
			}
		}
		if need("rows") {
			s.Rows = shapeRows(fail, "rows", raw.Rows)
		}
	case "dip":
		if need("pins") {
			var n int
			if raw.Pins.Kind != yaml.ScalarNode || raw.Pins.Decode(&n) != nil {
				fail("shape: pins of a dip must be the number of pins")
			} else if n < 4 || n > 64 || n%2 != 0 {
				fail("shape: a dip has an even number of pins from 4 to 64, got %d", n)
			} else {
				s.DipPins = n
			}
		}
		s.Drawn = "top"
	case "to":
		s.Drawn = "front"
		if need("package") {
			s.Package = oneOf("package", raw.Package, pinoutPackages)
		}
		if need("pins") {
			var list []string
			switch {
			case raw.Pins.Kind != yaml.SequenceNode || raw.Pins.Decode(&list) != nil:
				fail("shape: pins of a to shape must be the list of pin numbers, left to right")
			case len(list) < 2 || len(list) > 5:
				fail("shape: a to shape has 2 to 5 pins, got %d", len(list))
			default:
				for _, n := range list {
					if n == "" {
						fail("shape: pins must not contain an empty number")
					}
				}
				s.ToPins = list
			}
		}
	case "circular":
		s.Contact, s.Drawn = raw.Contact, raw.Drawn
		if need("contact") {
			oneOf("contact", raw.Contact, pinoutContacts)
		}
		if need("drawn") {
			oneOf("drawn", raw.Drawn, pinoutDrawn)
		}
		if set["key"] {
			var k *float64
			if raw.Key.Kind != yaml.ScalarNode || raw.Key.Decode(&k) != nil {
				fail("shape: key must be an angle in degrees or null")
			} else if k != nil && (*k < 0 || *k >= 360 || math.IsNaN(*k)) {
				fail("shape: key angle must be 0 or more and less than 360, got %v", *k)
			} else {
				s.KeyAngle = k
			}
		}
		if need("positions") {
			if len(raw.Positions) == 0 {
				fail("shape: positions must not be empty")
			}
			for i, rp := range raw.Positions {
				where := fmt.Sprintf("shape: position %d", i+1)
				switch {
				case rp.Pin == "":
					fail("%s: pin is required", where)
				case rp.Angle == nil || *rp.Angle < 0 || *rp.Angle >= 360 || math.IsNaN(*rp.Angle):
					fail("%s (pin %s): angle is required, 0 or more and less than 360", where, rp.Pin)
				case rp.Radius == nil || *rp.Radius < 0 || *rp.Radius > 1 || math.IsNaN(*rp.Radius):
					fail("%s (pin %s): radius is required, from 0 to 1", where, rp.Pin)
				default:
					s.Positions = append(s.Positions, PinoutPosition{Pin: rp.Pin, Angle: *rp.Angle, Radius: *rp.Radius})
				}
			}
		}
	case "jack":
		s.Drawn = "side"
		if need("segments") {
			if len(raw.Segments) < 2 || len(raw.Segments) > 5 {
				fail("shape: a jack has 2 to 5 segments, got %d", len(raw.Segments))
			}
			for _, seg := range raw.Segments {
				if seg == "" {
					fail("shape: segments must not contain an empty number")
				}
			}
			s.Segments = raw.Segments
		}
	case "board":
		s.Drawn = "top"
		s.Connector = raw.Connector
		if need("connector") {
			oneOf("connector", raw.Connector, pinoutConnectors)
		}
		if raw.Left == nil && raw.Right == nil && raw.Top == nil && raw.Bottom == nil {
			fail("shape: a board needs at least one of left, right, top and bottom")
		}
		s.Left = shapeList(fail, "left", raw.Left)
		s.Right = shapeList(fail, "right", raw.Right)
		s.Top = shapeList(fail, "top", raw.Top)
		s.Bottom = shapeList(fail, "bottom", raw.Bottom)
		for i, in := range raw.Inner {
			where := fmt.Sprintf("inner %d", i+1)
			if strings.TrimSpace(in.Label) == "" {
				fail("shape: %s needs a label", where)
			}
			s.Inner = append(s.Inner, PinoutInner{Label: in.Label, Rows: shapeRows(fail, where+" rows", in.Rows)})
		}
	}
	return s, s.Numbers()
}

// shapeList converts a list of pin numbers where null is a gap.
func shapeList(fail func(string, ...any), what string, in []*string) []string {
	out := make([]string, len(in))
	hasPin := false
	for i, n := range in {
		if n == nil {
			continue
		}
		if strings.TrimSpace(*n) == "" {
			fail("shape: %s has an empty pin number (write null for a gap)", what)
		}
		out[i] = *n
		hasPin = true
	}
	if in != nil && !hasPin {
		fail("shape: %s has no pins", what)
	}
	return out
}

// shapeRows converts the rows of a rows shape or of an inner header.
func shapeRows(fail func(string, ...any), what string, in []rawPinRow) []PinoutRow {
	if len(in) == 0 {
		fail("shape: %s must not be empty", what)
	}
	out := make([]PinoutRow, 0, len(in))
	for i, r := range in {
		if r.Offset < 0 || r.Offset > maxRowOffset || math.IsNaN(r.Offset) {
			fail("shape: %s %d: offset must be from 0 to %v pitches, got %v", what, i+1, maxRowOffset, r.Offset)
		}
		out = append(out, PinoutRow{Pins: shapeList(fail, fmt.Sprintf("%s %d", what, i+1), r.Pins), Offset: r.Offset})
	}
	return out
}

// The catalog JSON of a pinout; every field of a shape is always there.
type pinoutViewsJSON struct {
	Plug   string `json:"plug"`
	Socket string `json:"socket"`
}

type pinoutPinJSON struct {
	Number   string `json:"number"`
	Name     string `json:"name"`
	Function string `json:"function"`
	Type     string `json:"type"`
	Notes    string `json:"notes"`
	Wire     string `json:"wire"`
}

type pinoutVariantJSON struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Pins []pinoutPinJSON `json:"pins"`
}

type pinoutRowJSON struct {
	Pins   []*string `json:"pins"`
	Offset float64   `json:"offset"`
}

type pinoutPositionJSON struct {
	Pin    string  `json:"pin"`
	Angle  float64 `json:"angle"`
	Radius float64 `json:"radius"`
}

type pinoutInnerJSON struct {
	Label string          `json:"label"`
	Rows  []pinoutRowJSON `json:"rows"`
}

type pinoutShapeJSON struct {
	Type      string               `json:"type"`
	Outline   string               `json:"outline"`
	Contact   string               `json:"contact"`
	Drawn     string               `json:"drawn"`
	Key       any                  `json:"key"`
	Rows      []pinoutRowJSON      `json:"rows"`
	Pins      any                  `json:"pins"`
	Package   string               `json:"package"`
	Positions []pinoutPositionJSON `json:"positions"`
	Segments  []string             `json:"segments"`
	Left      []*string            `json:"left"`
	Right     []*string            `json:"right"`
	Top       []*string            `json:"top"`
	Bottom    []*string            `json:"bottom"`
	Connector string               `json:"connector"`
	Inner     []pinoutInnerJSON    `json:"inner"`
}

type pinoutJSON struct {
	Kind     string              `json:"kind"`
	ID       string              `json:"id"`
	Category string              `json:"category"`
	Group    string              `json:"group"`
	Name     string              `json:"name"`
	NameEN   string              `json:"name_en"`
	Doc      string              `json:"doc"`
	Notes    string              `json:"notes"`
	Tags     []string            `json:"tags"`
	Source   string              `json:"source"`
	Views    *pinoutViewsJSON    `json:"views"`
	Shape    pinoutShapeJSON     `json:"shape"`
	Pins     []pinoutPinJSON     `json:"pins"`
	Variants []pinoutVariantJSON `json:"variants"`
}

// gapList writes a list of pin numbers with null for a gap; the list is never null.
func gapList(in []string) []*string {
	out := make([]*string, len(in))
	for i := range in {
		if in[i] != "" {
			out[i] = &in[i]
		}
	}
	return out
}

func rowsJSON(in []PinoutRow) []pinoutRowJSON {
	out := make([]pinoutRowJSON, len(in))
	for i, r := range in {
		out[i] = pinoutRowJSON{Pins: gapList(r.Pins), Offset: r.Offset}
	}
	return out
}

func pinsJSON(in []Pin, lang string) []pinoutPinJSON {
	out := make([]pinoutPinJSON, len(in))
	for i, p := range in {
		out[i] = pinoutPinJSON{Number: p.Number, Name: p.Name, Function: p.Function.in(lang), Type: p.Type, Notes: p.Notes.in(lang), Wire: p.Wire}
	}
	return out
}

func (p *Pinout) json(lang string) pinoutJSON {
	out := pinoutJSON{
		Kind: "pinout", ID: p.ID, Category: p.Category, Group: p.Group, Name: p.Name.in(lang), NameEN: p.Name.in("en"),
		Doc: p.Doc.in(lang), Notes: p.Notes.in(lang), Tags: orEmpty(p.Tags), Source: p.Source,
		Pins: pinsJSON(p.Pins, lang), Variants: make([]pinoutVariantJSON, 0, len(p.Variants)),
	}
	if p.Views != nil {
		out.Views = &pinoutViewsJSON{Plug: p.Views["plug"].in(lang), Socket: p.Views["socket"].in(lang)}
	}
	for _, v := range p.Variants {
		out.Variants = append(out.Variants, pinoutVariantJSON{ID: v.ID, Name: v.Name.in(lang), Pins: pinsJSON(v.Pins, lang)})
	}
	s := p.Shape
	sj := pinoutShapeJSON{
		Type: s.Type, Outline: s.Outline, Contact: s.Contact, Drawn: s.Drawn, Rows: rowsJSON(s.Rows), Package: s.Package,
		Positions: make([]pinoutPositionJSON, len(s.Positions)), Segments: orEmpty(s.Segments),
		Left: gapList(s.Left), Right: gapList(s.Right), Top: gapList(s.Top), Bottom: gapList(s.Bottom),
		Connector: s.Connector, Inner: make([]pinoutInnerJSON, len(s.Inner)),
	}
	switch s.Type {
	case "rows":
		sj.Key = s.Key
	case "circular":
		if s.KeyAngle != nil {
			sj.Key = *s.KeyAngle
		}
	case "dip":
		sj.Pins = s.DipPins
	case "to":
		sj.Pins = s.ToPins
	}
	for i, pos := range s.Positions {
		sj.Positions[i] = pinoutPositionJSON{Pin: pos.Pin, Angle: pos.Angle, Radius: pos.Radius}
	}
	for i, in := range s.Inner {
		sj.Inner[i] = pinoutInnerJSON{Label: in.Label, Rows: rowsJSON(in.Rows)}
	}
	out.Shape = sj
	return out
}
