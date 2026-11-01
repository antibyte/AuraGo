package ecalc

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// The micro sign (U+00B5) is the prefix id for 1e-6; the Greek mu (U+03BC) and the ohm sign (U+2126) are folded
// onto it and onto the Greek capital omega (U+03A9, the resistance symbol) when input is parsed.
var (
	micro     = string(rune(0xB5))
	ohm       = string(rune(0x3A9))
	inputFold = strings.NewReplacer(string(rune(0x3BC)), micro, string(rune(0x2126)), ohm)
)

var (
	plainNumRe    = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	prefixedNumRe = regexp.MustCompile(`^([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)([pnu` + micro + `mkKMG])$`)
	infixNumRe    = regexp.MustCompile(`^(\d+)([pnu` + micro + `mkKMG])(\d+)$`)
	infixRRe      = regexp.MustCompile(`^(\d+)R(\d+)$`)
)

// Prefix is an SI prefix: id (p n µ m k M G) and factor.
type Prefix struct {
	ID     string  `yaml:"id" json:"id"`
	Factor float64 `yaml:"factor" json:"factor"`
}

// Unit is an explicit display unit of a dimension. A linear unit has a Factor (base = x * Factor); a non-linear one
// has To and From expressions in the expression language, where x is the value in this unit (To) or in the base
// unit (From).
type Unit struct {
	ID     string  `json:"id"`
	Symbol string  `json:"symbol"`
	Factor float64 `json:"factor"`
	To     string  `json:"to"`
	From   string  `json:"from"`

	to, from *Node
}

// Dim is a physical dimension (voltage, resistance, ...). Values are stored in its SI base unit.
type Dim struct {
	ID       string   `json:"id"`
	Symbol   string   `json:"symbol"`
	Aliases  []string `json:"aliases"`
	Prefixes []string `json:"prefixes"`
	Positive bool     `json:"positive"`
	Units    []Unit   `json:"units"`

	pfx      []Prefix // Prefixes resolved, ascending by factor
	suffixes []string // base symbol and aliases, longest first
	exUnits  []*Unit  // explicit units, longest symbol first
}

// Units is the unit model loaded from catalog/units.yaml.
type Units struct {
	Prefixes []Prefix `json:"prefixes"`
	Dims     []Dim    `json:"dims"`

	prefix map[string]float64
	byDim  map[string]*Dim
}

type rawUnit struct {
	ID     string  `yaml:"id"`
	Symbol string  `yaml:"symbol"`
	Factor float64 `yaml:"factor"`
	To     string  `yaml:"to"`
	From   string  `yaml:"from"`
}

type rawDim struct {
	Symbol   string    `yaml:"symbol"`
	Aliases  []string  `yaml:"aliases"`
	Prefixes []string  `yaml:"prefixes"`
	Positive bool      `yaml:"positive"`
	Units    []rawUnit `yaml:"units"`
}

func loadUnits(data []byte) (*Units, error) {
	var raw struct {
		Prefixes []Prefix  `yaml:"prefixes"`
		Dims     yaml.Node `yaml:"dims"`
	}
	if err := decodeStrict(data, &raw); err != nil {
		return nil, fmt.Errorf("units: %w", err)
	}
	u := &Units{Prefixes: raw.Prefixes, prefix: map[string]float64{"": 1}, byDim: map[string]*Dim{}}
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf("units: "+format, args...)) }

	last := 0.0
	for _, p := range u.Prefixes {
		if p.ID == "" || p.Factor <= last {
			fail("prefix %q: ids must be set and factors positive and ascending", p.ID)
		}
		if _, dup := u.prefix[p.ID]; dup {
			fail("prefix %q is defined twice", p.ID)
		}
		u.prefix[p.ID] = p.Factor
		last = p.Factor
	}

	entries, err := orderedMap(&raw.Dims, "dims")
	if err != nil {
		return nil, fmt.Errorf("units: %w", err)
	}
	for _, e := range entries {
		var rd rawDim
		if err := decodeNode(e.Val, &rd); err != nil {
			fail("dim %q: %v", e.Key, err)
			continue
		}
		d := Dim{ID: e.Key, Symbol: rd.Symbol, Aliases: rd.Aliases, Prefixes: rd.Prefixes, Positive: rd.Positive}
		if d.Aliases == nil {
			d.Aliases = []string{}
		}
		if d.Prefixes == nil {
			d.Prefixes = []string{}
		}
		for _, id := range d.Prefixes {
			f, ok := u.prefix[id]
			if !ok {
				fail("dim %q: unknown prefix %q", d.ID, id)
				continue
			}
			d.pfx = append(d.pfx, Prefix{ID: id, Factor: f})
		}
		sort.Slice(d.pfx, func(i, j int) bool { return d.pfx[i].Factor < d.pfx[j].Factor })
		d.Units = make([]Unit, 0, len(rd.Units))
		seen := map[string]bool{}
		for _, ru := range rd.Units {
			un := Unit{ID: ru.ID, Symbol: ru.Symbol, Factor: ru.Factor, To: ru.To, From: ru.From}
			switch {
			case un.ID == "" || un.Symbol == "" || seen[un.ID]:
				fail("dim %q: unit %q needs a unique id and a symbol", d.ID, un.ID)
			case un.Factor > 0 && un.To == "" && un.From == "":
			case un.Factor == 0 && un.To != "" && un.From != "":
				un.to, un.from = parseUnitExpr(&errs, d.ID, un.ID, un.To), parseUnitExpr(&errs, d.ID, un.ID, un.From)
			default:
				fail("dim %q: unit %q needs either a positive factor or both to and from", d.ID, un.ID)
			}
			seen[un.ID] = true
			d.Units = append(d.Units, un)
		}
		u.Dims = append(u.Dims, d)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	for i := range u.Dims {
		d := &u.Dims[i]
		u.byDim[d.ID] = d
		for _, s := range append([]string{d.Symbol}, d.Aliases...) {
			if s != "" {
				d.suffixes = append(d.suffixes, s)
			}
		}
		sort.SliceStable(d.suffixes, func(a, b int) bool { return len(d.suffixes[a]) > len(d.suffixes[b]) })
		for j := range d.Units {
			d.exUnits = append(d.exUnits, &d.Units[j])
		}
		sort.SliceStable(d.exUnits, func(a, b int) bool { return len(d.exUnits[a].Symbol) > len(d.exUnits[b].Symbol) })
	}
	return u, nil
}

// parseUnitExpr parses a unit conversion expression, whose only variable is x.
func parseUnitExpr(errs *[]error, dim, unit, src string) *Node {
	n, err := ParseExpr(src)
	if err == nil {
		for _, dep := range Deps(n) {
			if dep != "x" {
				err = fmt.Errorf("unknown variable %q (only x is allowed)", dep)
			}
		}
	}
	if err != nil {
		*errs = append(*errs, fmt.Errorf("units: dim %q unit %q: %q: %v", dim, unit, src, err))
	}
	return n
}

// Dim returns the dimension with the given id, or nil.
func (u *Units) Dim(id string) *Dim { return u.byDim[id] }

// Unit returns the explicit unit with the given id, or nil.
func (d *Dim) Unit(id string) *Unit {
	for i := range d.Units {
		if d.Units[i].ID == id {
			return &d.Units[i]
		}
	}
	return nil
}

func (d *Dim) hasPrefix(id string) bool {
	for _, p := range d.pfx {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (d *Dim) hasAlias(a string) bool {
	for _, x := range d.Aliases {
		if x == a {
			return true
		}
	}
	return false
}

func evalUnitExpr(n *Node, x float64) float64 {
	v, err := Eval(n, Env{Vals: map[string]float64{"x": x}})
	if err != nil {
		return math.NaN()
	}
	return v
}

// ToBase converts a value in this unit to the SI base unit.
func (un *Unit) ToBase(x float64) float64 {
	if un.to != nil {
		return evalUnitExpr(un.to, x)
	}
	return x * un.Factor
}

// FromBase converts a value in the SI base unit to this unit.
func (un *Unit) FromBase(v float64) float64 {
	if un.from != nil {
		return evalUnitExpr(un.from, v)
	}
	return v / un.Factor
}

func canonPrefix(s string) string {
	switch s {
	case "u":
		return micro
	case "K":
		return "k"
	}
	return s
}

// Parse reads a number typed by the user and returns it in the SI base unit of the dimension. unitID selects an
// explicit unit ("" = none): a plain number without any unit suffix is then a value in that unit.
//
// Accepted: 4700, 4k7, 4R7 (resistance), 47R, 2u2, 10mA, "10 mA", 1e-6, 1,5k (comma as decimal point), -10dBm.
func (u *Units) Parse(input, dimID, unitID string) (float64, error) {
	d := u.byDim[dimID]
	if d == nil {
		return 0, fmt.Errorf("unknown dimension %q", dimID)
	}
	s := inputFold.Replace(strings.Join(strings.Fields(input), ""))
	return u.parseNormalized(d, strings.ReplaceAll(s, ",", "."), unitID)
}

// listFields splits a list typed as "4k7, 10k; 2k2 1M": items are separated by semicolons, white space and commas
// that are not followed by a digit, because a comma in front of a digit is a decimal comma ("4,7k, 10k" is two
// items). Empty items are dropped. The ECMAScript twin is the regular expression /[;\s]+|,(?!\d)/.
func listFields(input string) []string {
	rs := []rune(input)
	var fields []string
	var item []rune
	end := func() {
		if len(item) > 0 {
			fields = append(fields, string(item))
			item = item[:0]
		}
	}
	for i, r := range rs {
		decimalComma := r == ',' && i+1 < len(rs) && rs[i+1] >= '0' && rs[i+1] <= '9'
		if r == ';' || unicode.IsSpace(r) || (r == ',' && !decimalComma) {
			end()
			continue
		}
		item = append(item, r)
	}
	end()
	return fields
}

// ParseList reads a list typed as "4k7, 10k, 2k2": the items are split by listFields and every item is parsed like
// Parse (a decimal comma inside an item reads as a decimal point).
func (u *Units) ParseList(input, dimID, unitID string) ([]float64, error) {
	d := u.byDim[dimID]
	if d == nil {
		return nil, fmt.Errorf("unknown dimension %q", dimID)
	}
	fields := listFields(input)
	if len(fields) == 0 {
		return nil, errors.New("empty list")
	}
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := u.parseNormalized(d, strings.ReplaceAll(inputFold.Replace(f), ",", "."), unitID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (u *Units) parseNormalized(d *Dim, s, unitID string) (float64, error) {
	if s == "" {
		return 0, errors.New("empty value")
	}
	var sel *Unit
	if unitID != "" {
		if sel = d.Unit(unitID); sel == nil {
			return 0, fmt.Errorf("unknown unit %q for %s", unitID, d.ID)
		}
	}
	orig := s
	bad := func() (float64, error) { return 0, fmt.Errorf("cannot read %q as a %s", orig, d.ID) }

	// An explicit unit written after the number (10dBm, 2Ah).
	for _, un := range d.exUnits {
		if rest, ok := strings.CutSuffix(s, un.Symbol); ok && plainNumRe.MatchString(rest) {
			x, _ := strconv.ParseFloat(rest, 64)
			return un.ToBase(x), nil
		}
	}
	// The base symbol or an alias (V, Ω, ohm, R), possibly behind a prefix (kΩ, mA).
	hadSuffix := false
	for _, suf := range d.suffixes {
		if rest, ok := strings.CutSuffix(s, suf); ok {
			s, hadSuffix = rest, true
			break
		}
	}
	if s == "" {
		return bad()
	}

	var x float64
	prefix := ""
	if plainNumRe.MatchString(s) {
		x, _ = strconv.ParseFloat(s, 64)
	} else if m := prefixedNumRe.FindStringSubmatch(s); m != nil {
		x, _ = strconv.ParseFloat(m[1], 64)
		prefix = canonPrefix(m[2])
	} else if m := infixNumRe.FindStringSubmatch(s); m != nil {
		x, _ = strconv.ParseFloat(m[1]+"."+m[3], 64)
		prefix = canonPrefix(m[2])
	} else if m := infixRRe.FindStringSubmatch(s); m != nil && d.hasAlias("R") {
		x, _ = strconv.ParseFloat(m[1]+"."+m[2], 64)
	} else {
		return bad()
	}

	if prefix != "" {
		if !d.hasPrefix(prefix) {
			return 0, fmt.Errorf("the prefix %q is not allowed for %s", prefix, d.ID)
		}
		if sel != nil && !hadSuffix {
			return 0, fmt.Errorf("a prefix cannot be combined with the selected unit %s", sel.ID)
		}
		return x * u.prefix[prefix], nil
	}
	if sel != nil && !hadSuffix {
		return sel.ToBase(x), nil
	}
	return x, nil
}

// fmtNum writes x as JavaScript's String(x) does: the shortest decimal that reads back as the same double, without
// an exponent for 1e-6 <= |x| < 1e21, otherwise like 1e-7 or 1.5e+21.
func fmtNum(x float64) string {
	if x == 0 {
		return "0"
	}
	if ax := math.Abs(x); ax >= 1e-6 && ax < 1e21 {
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	s := strconv.FormatFloat(x, 'e', -1, 64)
	i := strings.IndexByte(s, 'e')
	exp := strings.TrimLeft(s[i+2:], "0")
	if exp == "" {
		exp = "0"
	}
	return s[:i+2] + exp
}

func fmtSig(x float64, sig int) string { return fmtNum(roundSig(x, sig)) }

func withUnit(num, unit string) string {
	if unit == "" {
		return num
	}
	return num + " " + unit
}

// Format writes v (SI base unit) with sig significant digits (default 4). With an explicit unit selected the value
// is converted into it; otherwise the SI prefix is chosen automatically (4.7 kΩ, 250 mW). Values that are not
// finite give "".
func (u *Units) Format(v float64, dimID, unitID string, sig int) string {
	d := u.byDim[dimID]
	if d == nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	if sig < 1 {
		sig = 4
	}
	if un := d.Unit(unitID); un != nil {
		x := un.FromBase(v)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ""
		}
		return fmtSig(x, sig) + " " + un.Symbol
	}
	if len(d.pfx) == 0 {
		return withUnit(fmtSig(v, sig), d.Symbol)
	}
	if v == 0 {
		return withUnit("0", d.Symbol)
	}
	av := math.Abs(v)
	idx := 0
	for i, p := range d.pfx {
		if p.Factor <= av {
			idx = i
		}
	}
	m := roundSig(v/d.pfx[idx].Factor, sig)
	if math.Abs(m) >= 1000 && idx+1 < len(d.pfx) {
		idx++
		m = roundSig(v/d.pfx[idx].Factor, sig)
	}
	return withUnit(fmtNum(m), d.pfx[idx].ID+d.Symbol)
}
