package ecalc

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The query API of the pinouts for the agent tool: DescribePinout describes a pinout item without its pins, QueryPinout
// lists the pins of one variant (all of them, those with a number or name, or those that match a filter), one page at a
// time. Output texts are English; the structs carry JSON tags because the tool sends them as they are.

const (
	// DefaultPinoutLimit and MaxPinoutLimit bound the number of pins of one QueryPinout answer.
	DefaultPinoutLimit = 50
	MaxPinoutLimit     = 50
	// maxPinQueryLength is the longest pin number or name a request can match (the loader allows 24 characters for a
	// name); a longer text finds nothing and is not repeated in the answer.
	maxPinQueryLength = 24
)

// pinoutOutlineNames are the English names of the outlines of a "rows" shape (the keys are PinoutOutlines).
var pinoutOutlineNames = map[string]string{
	"displayport": "DisplayPort", "dsub": "D-sub", "hdmi_a": "HDMI type A", "jst": "JST", "minifit": "Mini-Fit", "molex": "Molex",
	"obd2": "OBD-II", "rect": "Rectangular", "rj45": "RJ45", "sata_data": "SATA data", "sata_power": "SATA power",
	"usb_a": "USB-A", "usb_b": "USB-B", "usb_c": "USB-C", "usb_micro_b": "USB micro-B", "usb_mini_b": "USB mini-B",
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// pinoutConnector is the short English text for the shape of a pinout: what it is and how many pins it has.
func pinoutConnector(p *Pinout, pinCount int) string {
	s := p.Shape
	pins := plural(pinCount, "pin", "pins")
	switch s.Type {
	case "rows":
		name := pinoutOutlineNames[s.Outline]
		if name == "" {
			name = s.Outline
		}
		return fmt.Sprintf("%s connector, %s, %s contacts, %s", name, plural(len(s.Rows), "row", "rows"), s.Contact, pins)
	case "dip":
		return "DIP package, " + pins
	case "to":
		return strings.ToUpper(s.Package[:2]) + "-" + s.Package[2:] + " package, " + pins
	case "circular":
		return fmt.Sprintf("Circular connector, %s contacts, %s", s.Contact, pins)
	case "jack":
		return "Audio jack plug, " + plural(len(s.Segments), "segment", "segments")
	case "board":
		if s.Connector == "none" {
			return "Board without a main connector, " + pins
		}
		return fmt.Sprintf("Board, main connector at the %s, %s", s.Connector, pins)
	}
	return pins
}

// lowerFirst makes the first letter of a caption lower case unless the caption starts with an acronym (USB, HDMI).
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 || !unicode.IsUpper(r[0]) || (len(r) > 1 && unicode.IsUpper(r[1])) {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// pinoutView is one English sentence that says what the drawing of the pinout shows, derived from the shape and the
// English caption of the view that is drawn.
func pinoutView(p *Pinout) string {
	s := p.Shape
	face := func(what string) string {
		caption := strings.TrimRight(strings.TrimSpace(p.Views[s.Drawn].in("en")), ".")
		if caption == "" {
			caption = map[string]string{"plug": "male connector (plug)", "socket": "female connector (socket)"}[s.Drawn]
		}
		return fmt.Sprintf("%s as seen looking at the mating face of the %s.", what, lowerFirst(caption))
	}
	switch s.Type {
	case "rows":
		return face("Rows")
	case "circular":
		return face("Pin positions")
	case "dip":
		return "Package seen from above: notch at the top, pin 1 at the top left, numbers run counter-clockwise."
	case "to":
		side := "flat side"
		if s.Package == "to220" {
			side = "label side"
		}
		return fmt.Sprintf("%s seen from the front (%s), leads pointing down, pins left to right.", strings.ToUpper(s.Package[:2])+"-"+s.Package[2:], side)
	case "jack":
		return "Plug seen from the side, tip at the left, segments from tip to sleeve."
	case "board":
		if s.Connector == "none" {
			return "Board seen from the component side, portrait, without a main connector."
		}
		return fmt.Sprintf("Board seen from the component side, portrait, main connector at the %s.", s.Connector)
	}
	return ""
}

// typeCounts counts the pins of every type (types without a pin are left out).
func typeCounts(pins []Pin) map[string]int {
	counts := map[string]int{}
	for _, p := range pins {
		counts[p.Type]++
	}
	return counts
}

// lookupPinout finds a pinout by id (case-insensitive), like lookupTable.
func (c *Catalog) lookupPinout(id string) (*Pinout, error) {
	if qe := idTooLong(id, "pinout"); qe != nil {
		return nil, qe
	}
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return nil, queryErr(CodeUnknownID, "id is required; use search to find a pinout id")
	}
	if p := c.Pinout(key); p != nil {
		return p, nil
	}
	return nil, c.notFound(id, key, "pinout")
}

// ---- describe -------------------------------------------------------------------------------------------------

// PinoutVariantInfo is one variant in a PinoutDescription: its id, its name and the number of pins of every type.
type PinoutVariantInfo struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Counts map[string]int `json:"counts"`
}

// PinoutDescription is what DescribePinout returns: everything but the pins. Connector says what the shape is, View what
// the drawing shows. PinCount and Counts (pins per type) are those of the first variant for an item with variants.
type PinoutDescription struct {
	ID        string              `json:"id"`
	Kind      string              `json:"kind"`
	Category  string              `json:"category"`
	Group     string              `json:"group"`
	Name      string              `json:"name"`
	Doc       string              `json:"doc,omitempty"`
	Connector string              `json:"connector"`
	View      string              `json:"view"`
	PinCount  int                 `json:"pin_count"`
	Counts    map[string]int      `json:"counts"`
	Variants  []PinoutVariantInfo `json:"variants,omitempty"`
	Notes     string              `json:"notes,omitempty"`
	Source    string              `json:"source"`
}

// DescribePinout returns what a pinout is: its group, connector, view, number of pins, variants, the pin counts per type,
// the notes and the source. The pins themselves are read with QueryPinout.
func (c *Catalog) DescribePinout(id string) (PinoutDescription, error) {
	p, err := c.lookupPinout(id)
	if err != nil {
		return PinoutDescription{}, err
	}
	pins, _, _ := p.PinsOf("")
	d := PinoutDescription{
		ID: p.ID, Kind: "pinout", Category: p.Category, Group: p.Group, Name: p.Name.in("en"), Doc: p.Doc.in("en"),
		Connector: pinoutConnector(p, len(pins)), View: pinoutView(p), PinCount: len(pins), Counts: typeCounts(pins),
		Notes: p.Notes.in("en"), Source: p.Source,
	}
	for _, v := range p.Variants {
		d.Variants = append(d.Variants, PinoutVariantInfo{ID: v.ID, Name: v.Name.in("en"), Counts: typeCounts(v.Pins)})
	}
	return d, nil
}

// ---- pins -----------------------------------------------------------------------------------------------------

// PinoutRequest is the input of QueryPinout. Pin keeps the pins whose number or name equals it (case-insensitive, exact:
// several pins can share a name such as GND). Variant is a variant id ("" = the first). Filter keeps the pins with a
// number, name, function or note that contains the text (English and German, case-insensitive); with Pin both have to
// match. Offset and Limit page through the kept pins (Limit 0 = DefaultPinoutLimit, at most MaxPinoutLimit).
type PinoutRequest struct {
	Pin     string
	Variant string
	Filter  string
	Offset  int
	Limit   int
}

// PinAnswer is one pin of a QueryPinout answer. Wire is the English name of the standard wire colour, empty when the
// pin has none.
type PinAnswer struct {
	Number   string `json:"number"`
	Name     string `json:"name"`
	Function string `json:"function"`
	Type     string `json:"type"`
	Wire     string `json:"wire,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// PinoutResult is what QueryPinout returns. Variant (with VariantName) is the variant that was read and Variants the ids
// of all variants (both absent for an item without variants). Found is set when a Pin was asked for: false means no
// pin of that variant has the number or name, which is an answer and not an error. Total counts the pins that match,
// Truncated means that more of them exist than are listed and NextOffset is where to continue.
type PinoutResult struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Group       string      `json:"group"`
	Connector   string      `json:"connector"`
	View        string      `json:"view"`
	Variant     string      `json:"variant,omitempty"`
	VariantName string      `json:"variant_name,omitempty"`
	Variants    []string    `json:"variants,omitempty"`
	Found       *bool       `json:"found,omitempty"`
	Pins        []PinAnswer `json:"pins"`
	Total       int         `json:"total"`
	Offset      int         `json:"offset"`
	Returned    int         `json:"returned"`
	Truncated   bool        `json:"truncated,omitempty"`
	NextOffset  int         `json:"next_offset,omitempty"`
	Notes       string      `json:"notes,omitempty"`
	Source      string      `json:"source"`
	Note        string      `json:"note,omitempty"`
}

// Keep shortens the listed pins to the first n (the tool does that to fit its output budget) and keeps Returned,
// Truncated and NextOffset consistent.
func (r *PinoutResult) Keep(n int) {
	if n < 0 || n >= len(r.Pins) {
		return
	}
	r.Pins = r.Pins[:n]
	r.Returned = n
	r.Truncated = true
	r.NextOffset = r.Offset + n
}

func pinMatchesFilter(p Pin, needle string) bool {
	for _, s := range []string{p.Number, p.Name, p.Function.in("en"), p.Function.in("de"), p.Notes.in("en"), p.Notes.in("de")} {
		if s != "" && strings.Contains(strings.ToLower(s), needle) {
			return true
		}
	}
	return false
}

// QueryPinout reads the pins of a pinout: those that match Pin and Filter, one page of them.
func (c *Catalog) QueryPinout(id string, req PinoutRequest) (PinoutResult, error) {
	p, err := c.lookupPinout(id)
	if err != nil {
		return PinoutResult{}, err
	}
	filter := strings.TrimSpace(req.Filter)
	if utf8.RuneCountInString(filter) > MaxFilterLength {
		return PinoutResult{}, queryErr(CodeInvalidFilter, "filter must be at most %d characters; give a word or a number that occurs in a pin", MaxFilterLength)
	}
	variant := strings.ToLower(strings.TrimSpace(req.Variant))
	pins, used, ok := p.PinsOf(variant)
	if !ok {
		if len(p.Variants) == 0 {
			return PinoutResult{}, queryErr(CodeUnknownVariant, "pinout %s has no variants; leave variant out", p.ID)
		}
		return PinoutResult{}, queryErr(CodeUnknownVariant, "pinout %s has no variant %q; variants: %s", p.ID, clipText(req.Variant, MaxIDLength), strings.Join(variantIDs(p), ", "))
	}
	res := PinoutResult{
		ID: p.ID, Name: p.Name.in("en"), Group: p.Group, Connector: pinoutConnector(p, len(pins)), View: pinoutView(p),
		Variant: used, Variants: variantIDs(p), Pins: []PinAnswer{}, Offset: max(req.Offset, 0), Notes: p.Notes.in("en"), Source: p.Source,
	}
	for _, v := range p.Variants {
		if v.ID == used {
			res.VariantName = v.Name.in("en")
		}
	}

	pin := strings.TrimSpace(req.Pin)
	needle := strings.ToLower(filter)
	var matches []Pin
	if utf8.RuneCountInString(pin) <= maxPinQueryLength {
		for _, pn := range pins {
			if pin != "" && !strings.EqualFold(pn.Number, pin) && !strings.EqualFold(pn.Name, pin) {
				continue
			}
			if needle != "" && !pinMatchesFilter(pn, needle) {
				continue
			}
			matches = append(matches, pn)
		}
	}
	res.Total = len(matches)
	if pin != "" {
		found := res.Total > 0
		res.Found = &found
		if !found {
			what := "No pin has that number or name"
			if utf8.RuneCountInString(pin) <= maxPinQueryLength {
				what = fmt.Sprintf("No pin has the number or name %q", pin)
			}
			if needle != "" {
				what += " that also matches the filter"
			}
			res.Note = what + fmt.Sprintf(" in %s", pinoutScope(p, used)) + "."
			if len(p.Variants) > 1 {
				res.Note += " Variants: " + strings.Join(variantIDs(p), ", ") + "."
			}
			res.Note += " Leave pin out to list the pins."
		}
	} else if res.Total == 0 {
		res.Note = "No pin matches the filter."
	}

	limit := req.Limit
	if limit < 1 {
		limit = DefaultPinoutLimit
	}
	limit = min(limit, MaxPinoutLimit)
	page := matches[min(res.Offset, len(matches)):]
	if len(page) > limit {
		page = page[:limit]
	}
	for _, pn := range page {
		out := PinAnswer{Number: pn.Number, Name: pn.Name, Function: pn.Function.in("en"), Type: pn.Type, Notes: pn.Notes.in("en")}
		if pn.Wire != "" {
			out.Wire = pn.Wire
			if w := c.Wire(pn.Wire); w != nil {
				out.Wire = w.Name.in("en")
			}
		}
		res.Pins = append(res.Pins, out)
	}
	res.Returned = len(page)
	if end := min(res.Offset, len(matches)) + len(page); end < len(matches) {
		res.Truncated, res.NextOffset = true, end
	}
	return res, nil
}

func variantIDs(p *Pinout) []string {
	var ids []string
	for _, v := range p.Variants {
		ids = append(ids, v.ID)
	}
	return ids
}

// pinoutScope names where a pin was looked for: the pinout, with its variant when it has any.
func pinoutScope(p *Pinout, variant string) string {
	if variant == "" {
		return p.ID
	}
	return p.ID + " variant " + variant
}

// clipText cuts s to at most n characters (a trailing ellipsis counts), for texts of a request that an error repeats.
func clipText(s string, n int) string {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n-1])) + "…"
	}
	return s
}
