package ecalc

import (
	"errors"
	"io"
	"io/fs"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// pinstyles.yaml holds the colours of the pinout drawings: one fill and ink per pin type and the standard wire colours.
// They are data and not theme tokens (a pin type must read the same on every skin).

// PinTypes are the six pin types of a pin, in the order of pinstyles.yaml.
var PinTypes = []string{"power", "ground", "signal", "data", "io", "nc"}

// wireIDs are the wire colours of pinstyles.yaml, in file order.
var wireIDs = []string{
	"black", "brown", "red", "orange", "yellow", "green", "blue", "purple", "grey", "white", "pink",
	"white_orange", "white_green", "white_blue", "white_brown",
}

var hexRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// minInkContrast is the WCAG contrast that the ink of a pin type needs on its fill.
const minInkContrast = 4.5

// PinType is the colour of one pin type: the fill of its contacts and the ink of the number on it.
type PinType struct {
	ID   string
	Hex  string
	Ink  string
	Name Text
}

// Wire is a standard wire colour; Stripe is the second colour of a striped wire, "" for a plain one.
type Wire struct {
	ID     string
	Hex    string
	Stripe string
	Name   Text
}

// PinType returns the style of a pin type, or nil.
func (c *Catalog) PinType(id string) *PinType { return c.pinTypeByID[id] }

// Wire returns the wire colour with the given id, or nil.
func (c *Catalog) Wire(id string) *Wire { return c.wireByID[id] }

type rawStyle struct {
	Hex    string `yaml:"hex"`
	Ink    string `yaml:"ink"`
	Stripe string `yaml:"stripe"`
	Name   Text   `yaml:"name"`
}

// pinstyles reads pinstyles.yaml. The file may be missing from a catalog without pinouts; with pinouts it is required.
func (l *loader) pinstyles(fsys fs.FS, cat *Catalog, needed bool) {
	data, err := fs.ReadFile(fsys, "pinstyles.yaml")
	if err != nil {
		if needed || !errors.Is(err, fs.ErrNotExist) {
			l.fail("pinstyles.yaml: %v", err)
		}
		return
	}
	var doc struct {
		Types yaml.Node `yaml:"types"`
		Wires yaml.Node `yaml:"wires"`
	}
	if err := decodeStrict(data, &doc); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("the file is empty")
		}
		l.fail("pinstyles.yaml: %v", err)
		return
	}
	types, err := orderedMap(&doc.Types, "pinstyles.yaml: types")
	if err != nil {
		l.fail("%v", err)
	}
	var typeIDs []string
	for _, e := range types {
		typeIDs = append(typeIDs, e.Key)
		what := "pinstyles.yaml: type " + e.Key
		var rs rawStyle
		if err := decodeNode(e.Val, &rs); err != nil {
			l.fail("%s: %v", what, err)
			continue
		}
		l.styleColour(what, "hex", rs.Hex, false)
		l.styleColour(what, "ink", rs.Ink, false)
		if rs.Stripe != "" {
			l.fail("%s: stripe is only for wires", what)
		}
		if hexRe.MatchString(rs.Hex) && hexRe.MatchString(rs.Ink) {
			if c := contrastRatio(rs.Ink, rs.Hex); c < minInkContrast {
				l.fail("%s: ink %s on %s has a contrast of %.2f, need at least %.1f", what, rs.Ink, rs.Hex, c, minInkContrast)
			}
		}
		l.checkText(what+": name", rs.Name, Locales)
		pt := &PinType{ID: e.Key, Hex: rs.Hex, Ink: rs.Ink, Name: rs.Name}
		cat.PinTypeStyles = append(cat.PinTypeStyles, pt)
		cat.pinTypeByID[e.Key] = pt
	}
	if !equalLists(typeIDs, PinTypes) {
		l.fail("pinstyles.yaml: types must be %s in this order, got %s", strings.Join(PinTypes, ", "), strings.Join(typeIDs, ", "))
	}

	wires, err := orderedMap(&doc.Wires, "pinstyles.yaml: wires")
	if err != nil {
		l.fail("%v", err)
	}
	var ids []string
	for _, e := range wires {
		ids = append(ids, e.Key)
		what := "pinstyles.yaml: wire " + e.Key
		var rs rawStyle
		if err := decodeNode(e.Val, &rs); err != nil {
			l.fail("%s: %v", what, err)
			continue
		}
		l.styleColour(what, "hex", rs.Hex, false)
		l.styleColour(what, "stripe", rs.Stripe, true)
		if rs.Ink != "" {
			l.fail("%s: ink is only for pin types", what)
		}
		if strings.HasPrefix(e.Key, "white_") && rs.Stripe == "" {
			l.fail("%s: a striped wire needs a stripe colour", what)
		}
		l.checkText(what+": name", rs.Name, Locales)
		w := &Wire{ID: e.Key, Hex: rs.Hex, Stripe: rs.Stripe, Name: rs.Name}
		cat.Wires = append(cat.Wires, w)
		cat.wireByID[e.Key] = w
	}
	if !equalLists(ids, wireIDs) {
		l.fail("pinstyles.yaml: wires must be %s in this order, got %s", strings.Join(wireIDs, ", "), strings.Join(ids, ", "))
	}
}

// styleColour checks a #rrggbb colour (lower case); an optional one may be empty.
func (l *loader) styleColour(what, field, value string, optional bool) {
	if value == "" && optional {
		return
	}
	if !hexRe.MatchString(value) {
		l.fail("%s: %s must be a colour like #1565c0 (lower case), got %q", what, field, value)
	}
}

func equalLists(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// luminance is the WCAG relative luminance of a #rrggbb colour.
func luminance(hex string) float64 {
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		s := float64(v) / 255
		if s <= 0.03928 {
			c[i] = s / 12.92
		} else {
			c[i] = math.Pow((s+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}

// contrastRatio is the WCAG contrast ratio of two #rrggbb colours (1 to 21).
func contrastRatio(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

type pinTypeJSON struct {
	ID   string `json:"id"`
	Hex  string `json:"hex"`
	Ink  string `json:"ink"`
	Name string `json:"name"`
}

type wireJSON struct {
	ID     string `json:"id"`
	Hex    string `json:"hex"`
	Stripe string `json:"stripe"`
	Name   string `json:"name"`
}

func (c *Catalog) pinStylesJSON(lang string) ([]pinTypeJSON, []wireJSON) {
	types := make([]pinTypeJSON, 0, len(c.PinTypeStyles))
	for _, t := range c.PinTypeStyles {
		types = append(types, pinTypeJSON{ID: t.ID, Hex: t.Hex, Ink: t.Ink, Name: t.Name.in(lang)})
	}
	wires := make([]wireJSON, 0, len(c.Wires))
	for _, w := range c.Wires {
		wires = append(wires, wireJSON{ID: w.ID, Hex: w.Hex, Stripe: w.Stripe, Name: w.Name.in(lang)})
	}
	return types, wires
}
