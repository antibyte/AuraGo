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
	plainNumRe    = regexp.MustCompile(`^[+-]?(\d+(?:\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)
	prefixedNumRe = regexp.MustCompile(`^([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)([pnu` + micro + `mkKMG])$`)
	infixNumRe    = regexp.MustCompile(`^(\d+)([pnu` + micro + `mkKMG])(\d+)$`)
	infixRRe      = regexp.MustCompile(`^(\d+)R(\d+)$`)
)

// The longest text Parse (one value) and ParseList (a list, each item still at most MaxValueLength) accept; the text
// fields of the app carry the same maxlength. Longer input is a parse error before any pattern runs, which bounds the
// work an arbitrary string can cause. Lengths count UTF-16 code units, like String.prototype.length in the JavaScript
// twin.
const (
	MaxValueLength = 64
	MaxListLength  = 256
)

// maxSafeInt is 2^53 - 1, the largest whole number that a double (and a JavaScript number) holds exactly. The values of
// an integer dimension and the numbers of a radix unit stay within 0 .. maxSafeInt.
const maxSafeInt = 9007199254740991

// textLength is the length of s in UTF-16 code units.
func textLength(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// Prefix is an SI prefix: id (p n µ m k M G) and factor.
type Prefix struct {
	ID     string  `yaml:"id" json:"id"`
	Factor float64 `yaml:"factor" json:"factor"`
}

// Unit is an explicit display unit of a dimension. A linear unit has a Factor (base = x * Factor); a non-linear one
// has To and From expressions in the expression language, where x is the value in this unit (To) or in the base
// unit (From). Aliases are further spellings that input accepts exactly like the Symbol (the longest spelling is
// tried first); they are never shown. A radix unit (Radix 2, 8, 10 or 16, only in an integer dimension) is a number
// system: it has no conversion, Parse reads the digits of that base and Format writes them.
type Unit struct {
	ID      string   `json:"id"`
	Symbol  string   `json:"symbol"`
	Aliases []string `json:"aliases"`
	Factor  float64  `json:"factor"`
	To      string   `json:"to"`
	From    string   `json:"from"`
	Radix   int      `json:"radix"`

	to, from *Node
}

// exName is one spelling of an explicit unit (its symbol or an alias) that input may carry behind the number.
type exName struct {
	text string
	unit *Unit
}

// Dim is a physical dimension (voltage, resistance, ...). Values are stored in its SI base unit. The values of an
// Integer dimension are whole numbers from 0 to maxSafeInt, and its units are number systems.
type Dim struct {
	ID       string   `json:"id"`
	Symbol   string   `json:"symbol"`
	Aliases  []string `json:"aliases"`
	Prefixes []string `json:"prefixes"`
	Positive bool     `json:"positive"`
	Integer  bool     `json:"integer"`
	Units    []Unit   `json:"units"`

	pfx      []Prefix // Prefixes resolved, ascending by factor
	suffixes []string // base symbol and aliases, longest first
	exNames  []exName // spellings of the explicit units, longest first
}

// Units is the unit model loaded from catalog/units.yaml.
type Units struct {
	Prefixes []Prefix `json:"prefixes"`
	Dims     []Dim    `json:"dims"`

	prefix map[string]float64
	byDim  map[string]*Dim
}

type rawUnit struct {
	ID      string   `yaml:"id"`
	Symbol  string   `yaml:"symbol"`
	Aliases []string `yaml:"aliases"`
	Factor  float64  `yaml:"factor"`
	To      string   `yaml:"to"`
	From    string   `yaml:"from"`
	Radix   int      `yaml:"radix"`
}

type rawDim struct {
	Symbol   string    `yaml:"symbol"`
	Aliases  []string  `yaml:"aliases"`
	Prefixes []string  `yaml:"prefixes"`
	Positive bool      `yaml:"positive"`
	Integer  bool      `yaml:"integer"`
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
		d := Dim{ID: e.Key, Symbol: rd.Symbol, Aliases: rd.Aliases, Prefixes: rd.Prefixes, Positive: rd.Positive, Integer: rd.Integer}
		if d.Aliases == nil {
			d.Aliases = []string{}
		}
		if d.Prefixes == nil {
			d.Prefixes = []string{}
		}
		if d.Integer && len(d.Prefixes) > 0 {
			fail("dim %q: an integer dimension has no prefixes", d.ID)
		}
		if d.Integer && d.Positive {
			fail("dim %q: an integer dimension cannot be positive (zero is a whole number)", d.ID)
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
		// Every spelling input may carry (base symbol, aliases, unit symbols, unit aliases) belongs to one thing only.
		spelled := map[string]string{}
		spell := func(text, what string) {
			switch {
			case text == "":
				fail("dim %q: %s must not be empty", d.ID, what)
			case spelled[text] != "":
				fail("dim %q: %q is used by %s and by %s", d.ID, text, spelled[text], what)
			default:
				spelled[text] = what
			}
		}
		if d.Symbol != "" {
			spell(d.Symbol, "the base symbol")
		}
		for _, a := range d.Aliases {
			spell(a, "an alias of the dimension")
		}
		for _, ru := range rd.Units {
			un := Unit{ID: ru.ID, Symbol: ru.Symbol, Aliases: ru.Aliases, Factor: ru.Factor, To: ru.To, From: ru.From, Radix: ru.Radix}
			if un.Aliases == nil {
				un.Aliases = []string{}
			}
			if un.Symbol != "" {
				spell(un.Symbol, "the symbol of unit "+strconv.Quote(un.ID))
			}
			for _, a := range un.Aliases {
				spell(a, "an alias of unit "+strconv.Quote(un.ID))
			}
			switch {
			case un.ID == "" || un.Symbol == "" || seen[un.ID]:
				fail("dim %q: unit %q needs a unique id and a symbol", d.ID, un.ID)
			case un.Radix != 0:
				switch {
				case !d.Integer:
					fail("dim %q: unit %q has a radix, so the dimension must be integer: true", d.ID, un.ID)
				case un.Radix != 2 && un.Radix != 8 && un.Radix != 10 && un.Radix != 16:
					fail("dim %q: unit %q: the radix must be 2, 8, 10 or 16", d.ID, un.ID)
				case un.Factor != 0 || un.To != "" || un.From != "" || len(un.Aliases) > 0:
					fail("dim %q: radix unit %q must not have a factor, to, from or aliases", d.ID, un.ID)
				}
			case d.Integer:
				fail("dim %q: an integer dimension only has radix units, unit %q has none", d.ID, un.ID)
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
			un := &d.Units[j]
			if un.Radix != 0 {
				continue // a number system is selected, never written behind a number
			}
			d.exNames = append(d.exNames, exName{un.Symbol, un})
			for _, a := range un.Aliases {
				d.exNames = append(d.exNames, exName{a, un})
			}
		}
		sort.SliceStable(d.exNames, func(a, b int) bool { return textLength(d.exNames[a].text) > textLength(d.exNames[b].text) })
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
	if textLength(input) > MaxValueLength {
		return 0, fmt.Errorf("the input is longer than %d characters", MaxValueLength)
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
	if textLength(input) > MaxListLength {
		return nil, fmt.Errorf("the list is longer than %d characters", MaxListLength)
	}
	fields := listFields(input)
	if len(fields) == 0 {
		return nil, errors.New("empty list")
	}
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		if textLength(f) > MaxValueLength {
			return nil, fmt.Errorf("a list item is longer than %d characters", MaxValueLength)
		}
		v, err := u.parseNormalized(d, strings.ReplaceAll(inputFold.Replace(f), ",", "."), unitID)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// parseNormalized reads one value from folded text without blanks (a decimal comma already turned into a point). The
// result must be finite, like in the ECMAScript twin: "1e400" or a product that overflows is a parse error.
func (u *Units) parseNormalized(d *Dim, s, unitID string) (float64, error) {
	v, err := u.parseValue(d, s, unitID)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("the value %q is out of range", s)
	}
	return v, nil
}

func (u *Units) parseValue(d *Dim, s, unitID string) (float64, error) {
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

	// A selected number system reads the digits of its base and nothing else.
	if sel != nil && sel.Radix != 0 {
		return parseRadix(sel, s)
	}

	// An explicit unit written after the number (10dBm, 2Ah, 25degC): its symbol or one of its aliases.
	for _, en := range d.exNames {
		if rest, ok := strings.CutSuffix(s, en.text); ok && plainNumRe.MatchString(rest) {
			x, _ := strconv.ParseFloat(rest, 64)
			return en.unit.ToBase(x), nil
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

// radixLetter is the letter of the optional prefix of a radix number: 0x, 0b and 0o (decimal has no prefix).
func radixLetter(radix int) byte {
	switch radix {
	case 16:
		return 'x'
	case 2:
		return 'b'
	case 8:
		return 'o'
	}
	return 0
}

// parseRadix reads a whole number in the base of un from text without blanks: an optional 0x, 0b or 0o prefix of that
// base (either case), then at least one digit of the base (hex digits in either case) with underscores allowed between
// digits. There is no sign and no SI prefix, and the value must not exceed maxSafeInt.
func parseRadix(un *Unit, s string) (float64, error) {
	bad := fmt.Errorf("cannot read %q as a %s number", s, un.ID)
	digits := s
	if l := radixLetter(un.Radix); l != 0 && len(digits) >= 2 && digits[0] == '0' && digits[1]|0x20 == l {
		digits = digits[2:]
	}
	if digits == "" || digits[0] == '_' || digits[len(digits)-1] == '_' {
		return 0, bad
	}
	var v uint64
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		if c == '_' {
			continue
		}
		var digit int
		switch {
		case c >= '0' && c <= '9':
			digit = int(c - '0')
		case c >= 'a' && c <= 'f':
			digit = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			digit = int(c-'A') + 10
		default:
			return 0, bad
		}
		if digit >= un.Radix {
			return 0, bad
		}
		if v = v*uint64(un.Radix) + uint64(digit); v > maxSafeInt {
			return 0, fmt.Errorf("the value %q is out of range (at most %d)", s, uint64(maxSafeInt))
		}
	}
	return float64(v), nil
}

// groupDigits writes the ASCII text s in groups of n characters counted from the right, joined by underscores.
func groupDigits(s string, n int) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%n == 0 {
			b.WriteByte('_')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// formatRadix writes a whole number in 0 .. maxSafeInt in the given base: decimal as plain digits, hex as 0x and
// upper-case digits grouped by four (0xDEAD_BEEF), binary as 0b grouped by eight, that is by byte (0b1_00000000), octal
// as 0o. Anything else gives "". The largest value, 2^53-1, is 61 characters in binary, so every text this function
// writes is within the 64 characters that Parse accepts. The significant-digit setting does not apply.
func formatRadix(radix int, v float64) string {
	if v != math.Trunc(v) || v < 0 || v > maxSafeInt {
		return ""
	}
	n := uint64(v)
	switch radix {
	case 16:
		return "0x" + groupDigits(strings.ToUpper(strconv.FormatUint(n, 16)), 4)
	case 2:
		return "0b" + groupDigits(strconv.FormatUint(n, 2), 8)
	case 8:
		return "0o" + strconv.FormatUint(n, 8)
	}
	return strconv.FormatUint(n, 10)
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
		if un.Radix != 0 {
			return formatRadix(un.Radix, v)
		}
		x := un.FromBase(v)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return ""
		}
		return fmtSig(x, sig) + " " + un.Symbol
	}
	if d.Integer {
		// No number system selected: decimal digits (never rounded to sig digits) when v is a whole number in range.
		if s := formatRadix(10, v); s != "" {
			return s
		}
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
