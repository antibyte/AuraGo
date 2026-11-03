package ecalc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// The capacitor kind: the picofarad code of ceramic and film capacitors (104 = 100 nF, 4R7 = 4.7 pF, 4p7, 2n2), the
// tolerance letters and the two-character rated-voltage codes (1H = 50 V). One marking may carry several parts ("104K 1H");
// every part is recognised by its shape. The multipliers, the RKM prefixes, the tolerance letters and the voltage codes are
// the data of the code item.

type codeCapMultiplier struct {
	Digit string `yaml:"digit" json:"digit"`
	Exp   int    `yaml:"exp" json:"exp"`
	Maker string `yaml:"maker" json:"maker"`
}

type codeCapRKM struct {
	Letters []string `yaml:"letters" json:"letters"`
	Exp     int      `yaml:"exp" json:"exp"`
}

type codeCapTolerance struct {
	Letter string  `yaml:"letter" json:"letter"`
	Minus  float64 `yaml:"minus" json:"minus"`
	Plus   float64 `yaml:"plus" json:"plus"`
	Unit   string  `yaml:"unit" json:"unit"`
	When   string  `yaml:"when" json:"when"`
}

type codeCapVoltage struct {
	Code   string   `yaml:"code"`
	Volts  float64  `yaml:"volts"`
	Makers []string `yaml:"makers"`
	Note   Text     `yaml:"note"`
}

type codeCapNotChecked struct {
	Tolerance  []string `yaml:"tolerance" json:"tolerance"`
	Multiplier []string `yaml:"multiplier" json:"multiplier"`
}

type codeCapacitorData struct {
	Multipliers []codeCapMultiplier `yaml:"multipliers"`
	RKM         []codeCapRKM        `yaml:"rkm"`
	Tolerances  []codeCapTolerance  `yaml:"tolerances"`
	Voltages    []codeCapVoltage    `yaml:"voltages"`
	NotChecked  codeCapNotChecked   `yaml:"not_cross_checked"`
	Notes       []Text              `yaml:"notes"`

	mult    map[string]*codeCapMultiplier
	rkm     map[string]int // prefix letter -> exponent
	voltage map[string]*codeCapVoltage
}

const (
	codeWhenLess    = "lt10pf"
	codeWhenAtLeast = "ge10pf"
	codeWhenAny     = "any"
)

func codeLoadCapacitor(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeCapacitorData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.mult, d.rkm, d.voltage = map[string]*codeCapMultiplier{}, map[string]int{}, map[string]*codeCapVoltage{}
	for i := range d.Multipliers {
		m := &d.Multipliers[i]
		if len(m.Digit) != 1 || m.Digit < "0" || m.Digit > "9" || d.mult[m.Digit] != nil || m.Exp < -3 || m.Exp > 9 {
			fail("data: multipliers[%d]: a unique digit and an exponent from -3 to 9 are required", i)
			continue
		}
		d.mult[m.Digit] = m
	}
	for i, r := range d.RKM {
		for _, l := range r.Letters {
			if codeUtf8Len(l) != 1 || d.rkm[l] != 0 || r.Exp == 0 {
				fail("data: rkm[%d]: letter %q must be one character, used once, with an exponent other than 0", i, l)
				continue
			}
			d.rkm[l] = r.Exp
		}
	}
	for i := range d.Tolerances {
		t := &d.Tolerances[i]
		if len(t.Letter) != 1 || t.Letter < "A" || t.Letter > "Z" || !(t.Minus > 0) || !(t.Plus > 0) {
			fail("data: tolerances[%d]: a capital letter and positive deviations are required", i)
		}
		if t.Unit != "pf" && t.Unit != "pct" {
			fail("data: tolerances[%d]: unit must be pf or pct", i)
		}
		if t.When != codeWhenLess && t.When != codeWhenAtLeast && t.When != codeWhenAny {
			fail("data: tolerances[%d]: when must be %s, %s or %s", i, codeWhenLess, codeWhenAtLeast, codeWhenAny)
		}
		if t.Unit == "pf" && (t.Minus != t.Plus || t.When != codeWhenLess) {
			fail("data: tolerances[%d]: a pF tolerance is symmetric and applies below 10 pF", i)
		}
	}
	for i := range d.Voltages {
		v := &d.Voltages[i]
		if len(v.Code) != 2 || v.Code[0] < '0' || v.Code[0] > '9' || v.Code[1] < 'A' || v.Code[1] > 'Z' || d.voltage[v.Code] != nil || !(v.Volts > 0) {
			fail("data: voltages[%d]: a unique code of a digit and a capital letter and positive volts are required", i)
			continue
		}
		if len(v.Makers) < 2 {
			fail("data: voltage code %s: at least two makers are required (a single-source code stays out)", v.Code)
		}
		if len(v.Note) > 0 {
			codeNeedEnDe(fail, "voltage code "+v.Code+" note", v.Note)
		}
		d.voltage[v.Code] = v
	}
	if len(d.mult) == 0 || len(d.rkm) == 0 || len(d.Tolerances) == 0 || len(d.Voltages) == 0 {
		fail("data: multipliers, rkm, tolerances and voltages must not be empty")
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

func codeUtf8Len(s string) int { return utf8.RuneCountInString(s) }

type codeCapVoltageJSON struct {
	Code   string   `json:"code"`
	Volts  float64  `json:"volts"`
	Makers []string `json:"makers"`
	Note   string   `json:"note"`
	NoteEN string   `json:"note_en"` // the English note, which the warnings of a decode use in every language
}

type codeCapacitorDoc struct {
	Multipliers []codeCapMultiplier  `json:"multipliers"`
	RKM         []codeCapRKM         `json:"rkm"`
	Tolerances  []codeCapTolerance   `json:"tolerances"`
	Voltages    []codeCapVoltageJSON `json:"voltages"`
	NotChecked  codeCapNotChecked    `json:"not_cross_checked"`
	Notes       []string             `json:"notes"`
}

func codeCapacitorJSON(data any, lang string) any {
	d := data.(*codeCapacitorData)
	out := codeCapacitorDoc{Multipliers: orEmpty(d.Multipliers), RKM: orEmpty(d.RKM), Tolerances: orEmpty(d.Tolerances), Voltages: make([]codeCapVoltageJSON, 0, len(d.Voltages)),
		NotChecked: codeCapNotChecked{Tolerance: orEmpty(d.NotChecked.Tolerance), Multiplier: orEmpty(d.NotChecked.Multiplier)}, Notes: codeLocalizedNotes(d.Notes, lang)}
	for _, v := range d.Voltages {
		note, noteEN := "", ""
		if len(v.Note) > 0 {
			note, noteEN = v.Note.in(lang), v.Note.in("en")
		}
		out.Voltages = append(out.Voltages, codeCapVoltageJSON{Code: v.Code, Volts: v.Volts, Makers: orEmpty(v.Makers), Note: note, NoteEN: noteEN})
	}
	return out
}

var (
	codeCapRKMRe     = regexp.MustCompile(`^(\d+)([pnuµm])(\d*)([A-Za-z]?)$`)
	codeCapRKMUpper  = regexp.MustCompile(`^(\d+)([PNU])(\d+)([A-Za-z]?)$`)
	codeCapRRe       = regexp.MustCompile(`^(\d?)R(\d{1,2})([A-Z]?)$`)
	codeCapDigitsRe  = regexp.MustCompile(`^(\d{3})([A-Z]?)$`)
	codeCapVoltageRe = regexp.MustCompile(`^\d[A-Z]$`)
	codeCapLetterRe  = regexp.MustCompile(`^[A-Z]$`)
	codeCapPairRe    = regexp.MustCompile(`^[A-Z]{2}$`)
)

// codeTenPicofarad is the boundary of the tolerance letters (below it they mean pF, from it percent).
var codeTenPicofarad = codeDecimal(10, -12)

// toleranceRows selects the tolerance rows of a letter that apply to the capacitance (when known).
func (d *codeCapacitorData) toleranceRows(letter string, capF float64, hasCap bool) ([]*codeCapTolerance, error) {
	var all, rows []*codeCapTolerance
	for i := range d.Tolerances {
		if t := &d.Tolerances[i]; t.Letter == letter {
			all = append(all, t)
		}
	}
	if len(all) == 0 {
		for _, l := range d.NotChecked.Tolerance {
			if l == letter {
				return nil, codeErr(CodeUnsupported, "the tolerance letter %s is not cross-checked (found in one source only)", letter)
			}
		}
		return nil, codeErr(CodeInvalidCode, "unknown tolerance letter %s", letter)
	}
	for _, t := range all {
		switch {
		case !hasCap, t.When == codeWhenAny:
			rows = append(rows, t)
		case capF < codeTenPicofarad && t.When == codeWhenLess, capF >= codeTenPicofarad && t.When == codeWhenAtLeast:
			rows = append(rows, t)
		}
	}
	if len(rows) == 0 {
		if capF < codeTenPicofarad {
			return nil, codeErr(CodeUnsupported, "the tolerance letter %s is not cross-checked for a capacitance below 10 pF", letter)
		}
		return nil, codeErr(CodeUnsupported, "the tolerance letter %s is not cross-checked for a capacitance of 10 pF or more", letter)
	}
	return rows, nil
}

func codeToleranceRowText(t *codeCapTolerance) string {
	switch {
	case t.Unit == "pf":
		return "±" + fmtSig(t.Minus, 6) + " pF"
	case t.Minus == t.Plus:
		return "±" + fmtSig(t.Minus, 6) + " %"
	}
	return "−" + fmtSig(t.Minus, 6) + " % / +" + fmtSig(t.Plus, 6) + " %"
}

// codeAddTolerance adds the tolerance field(s) for the rows of a letter: one row gives a value, several (a letter that means
// pF below 10 pF and percent above) give the readings in the text; an asymmetric tolerance also gives the two deviations.
func codeAddTolerance(res *CodeResult, rows []*codeCapTolerance, whenKnown bool) {
	if len(rows) > 1 {
		parts := make([]string, len(rows))
		for i, t := range rows {
			parts[i] = codeToleranceRowText(t) + codeWhenSuffix(t.When)
		}
		res.field("tolerance", "Tolerance", strings.Join(parts, " or "), math.NaN(), "")
		return
	}
	t := rows[0]
	text := codeToleranceRowText(t)
	if !whenKnown {
		text += codeWhenSuffix(t.When)
	}
	switch {
	case t.Unit == "pf":
		res.field("tolerance", "Tolerance", text, codeScaleDec(t.Minus, -12), "capacitance")
	case t.Minus == t.Plus:
		res.field("tolerance", "Tolerance", text, codeScaleDec(t.Minus, -2), "ratio")
	default:
		res.field("tolerance", "Tolerance", text, math.NaN(), "ratio")
		res.field("tolerance_minus", "Lower deviation", "−"+fmtSig(t.Minus, 6)+" %", codeScaleDec(-t.Minus, -2), "ratio")
		res.field("tolerance_plus", "Upper deviation", "+"+fmtSig(t.Plus, 6)+" %", codeScaleDec(t.Plus, -2), "ratio")
	}
}

func codeWhenSuffix(when string) string {
	switch when {
	case codeWhenLess:
		return " (C < 10 pF)"
	case codeWhenAtLeast:
		return " (C ≥ 10 pF)"
	}
	return ""
}

// codeCapToken is one recognised part of a capacitor marking: a value (with an optional trailing tolerance letter), a tolerance
// letter or a voltage code.
type codeCapToken struct {
	hasValue bool
	value    float64 // farad
	family   string
	canon    string // the part as it is written canonically ("104", "4p7", "R50")
	warning  string // a warning the value part brings with it
	letter   string // a tolerance letter (alone or behind the value)
	voltage  string // a voltage code
}

// classify recognises the shape of one part of the marking.
func (d *codeCapacitorData) classify(tok string) (codeCapToken, error) {
	m := codeCapRKMRe.FindStringSubmatch(tok)
	if m == nil {
		if m = codeCapRKMUpper.FindStringSubmatch(tok); m != nil {
			m[2] = strings.ToLower(m[2])
		}
	}
	if m != nil { // RKM: 4p7, 100n, 2µ2 (the letter is the decimal point and the prefix)
		t := codeCapToken{hasValue: true, value: codeDecimalText(m[1], m[3], d.rkm[m[2]]), family: "RKM notation", canon: m[1] + m[2] + m[3]}
		if m[4] != "" && !(m[4] == "F" && m[3] == "") { // 100nF is the unit farad, not the tolerance letter F
			t.letter = strings.ToUpper(m[4])
			t.canon += t.letter
		}
		return t, nil
	}
	up := strings.ToUpper(tok)
	switch {
	case codeCapVoltageRe.MatchString(up):
		if d.voltage[up] == nil {
			return codeCapToken{}, codeErr(CodeUnsupported, "the voltage code %s is not a standard code (codes with a single source are left out; makers differ, for example Murata writes YA for 35 V)", up)
		}
		return codeCapToken{voltage: up, canon: up}, nil
	case codeCapLetterRe.MatchString(up):
		return codeCapToken{letter: up, canon: up}, nil
	case codeCapPairRe.MatchString(up):
		return codeCapToken{}, codeErr(CodeUnsupported, "%s looks like a maker-specific voltage code (for example Murata YA = 35 V); only the common two-character codes are decoded", up)
	}
	if m := codeCapRRe.FindStringSubmatch(up); m != nil && len(m[1])+len(m[2]) > 0 { // R notation in pF: 4R7, R50, 0R5
		return codeCapToken{hasValue: true, value: codeDecimalText(m[1], m[2], -12), family: "R notation (pF)", canon: m[1] + "R" + m[2] + m[3], letter: m[3]}, nil
	}
	if m := codeCapDigitsRe.FindStringSubmatch(up); m != nil { // 3 digits: two significant digits and the multiplier
		n, _ := strconv.Atoi(m[1][:2])
		row := d.mult[m[1][2:]]
		if row == nil {
			for _, x := range d.NotChecked.Multiplier {
				if x == m[1][2:] {
					return codeCapToken{}, codeErr(CodeUnsupported, "the multiplier digit %s is not cross-checked", x)
				}
			}
			return codeCapToken{}, codeErr(CodeInvalidCode, "unknown multiplier digit %s", m[1][2:])
		}
		t := codeCapToken{hasValue: true, value: codeDecimal(n, row.Exp-12), family: "3-digit pF code", canon: m[1] + m[2], letter: m[2]}
		if row.Maker != "" {
			t.warning = fmt.Sprintf("the third digit %s (×%s) follows the %s ordering convention for values below 10 pF", row.Digit, fmtNum(codeDecimal(1, row.Exp)), strings.ToUpper(row.Maker))
		}
		return t, nil
	}
	return codeCapToken{}, codeErr(CodeInvalidCode, "cannot read %q as a capacitor marking (value code, tolerance letter or voltage code)", tok)
}

func (d *codeCapacitorData) decode(u *Units, code string) (CodeResult, error) {
	tokens := strings.FieldsFunc(code, func(r rune) bool { return unicode.IsSpace(r) || r == ',' || r == ';' })
	if len(tokens) == 0 {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is empty")
	}
	if len(tokens) > 3 {
		return CodeResult{}, codeErr(CodeInvalidCode, "a capacitor marking has at most three parts (value code, tolerance letter, voltage code)")
	}
	res := codeNewResult("capacitor")
	var whole codeCapToken
	var canon []string
	for _, tok := range tokens {
		t, err := d.classify(tok)
		if err != nil {
			return CodeResult{}, err
		}
		switch {
		case t.hasValue && whole.hasValue:
			return CodeResult{}, codeErr(CodeInvalidCode, "more than one capacitance value in the code")
		case t.letter != "" && whole.letter != "":
			return CodeResult{}, codeErr(CodeInvalidCode, "more than one tolerance letter in the code")
		case t.voltage != "" && whole.voltage != "":
			return CodeResult{}, codeErr(CodeInvalidCode, "more than one voltage code in the code")
		}
		if t.hasValue {
			if !(t.value > 0) {
				return CodeResult{}, codeErr(CodeInvalidCode, "the code gives a capacitance of zero")
			}
			whole.hasValue, whole.value, whole.family = true, t.value, t.family
			if t.warning != "" {
				res.warn("%s", t.warning)
			}
		}
		if t.letter != "" {
			whole.letter = t.letter
		}
		if t.voltage != "" {
			whole.voltage = t.voltage
		}
		canon = append(canon, t.canon)
	}
	res.Code = strings.Join(canon, " ")
	if whole.hasValue {
		res.Value, res.Dim = whole.value, "capacitance"
		res.field("family", "Marking", whole.family, math.NaN(), "")
	}
	if whole.letter != "" {
		rows, err := d.toleranceRows(whole.letter, whole.value, whole.hasValue)
		if err != nil {
			return CodeResult{}, err
		}
		codeAddTolerance(&res, rows, whole.hasValue)
	}
	if whole.voltage != "" {
		d.addVoltage(&res, u, d.voltage[whole.voltage], !whole.hasValue)
	}
	return res, nil
}

// addVoltage adds the voltage field and the warnings that come with a voltage code; headline makes the voltage the value.
func (d *codeCapacitorData) addVoltage(res *CodeResult, u *Units, v *codeCapVoltage, headline bool) {
	res.field("voltage", "Rated voltage", u.Format(v.Volts, "voltage", "", 4), v.Volts, "voltage")
	if headline {
		res.Value, res.Dim = v.Volts, "voltage"
	}
	res.warn("voltage codes are a maker convention, not part of IEC 60062; check the datasheet (for example Murata writes YA for 35 V)")
	if len(v.Note) > 0 {
		res.warn("%s: %s", v.Code, v.Note["en"])
	}
}

// codeCapValueCode writes C (farad, two significant digits) as the 3-digit code or the R notation in pF; "" when it has none.
func codeCapValueCode(N, e int) string {
	ep := e + 12 // N * 10^ep is the value in pF
	switch {
	case ep >= 0 && ep <= 6:
		return fmt.Sprintf("%d%d", N, ep)
	case ep == -1:
		return fmt.Sprintf("%dR%d", N/10, N%10)
	case ep == -2:
		return fmt.Sprintf("R%d", N)
	}
	return ""
}

// codeCapRKMCode writes the value N*10^e farad in RKM notation (4p7, 100n, 2µ2): the letter is the decimal point.
func codeCapRKMCode(N, e int) string {
	prefixes := []struct {
		letter string
		exp    int
	}{{"m", -3}, {"µ", -6}, {"n", -9}, {"p", -12}}
	digits := strconv.Itoa(N)
	top := e + len(digits) - 1 // exponent of the leading digit
	pick := prefixes[len(prefixes)-1]
	for _, p := range prefixes {
		if top >= p.exp {
			pick = p
			break
		}
	}
	shift := e - pick.exp // the number is digits * 10^shift in the unit of the prefix
	whole, frac := digits, ""
	switch {
	case shift >= 0:
		whole = digits + strings.Repeat("0", shift)
	case -shift >= len(digits):
		whole, frac = "0", strings.Repeat("0", -shift-len(digits))+digits
	default:
		whole, frac = digits[:len(digits)+shift], digits[len(digits)+shift:]
	}
	frac = strings.TrimRight(frac, "0")
	return whole + pick.letter + frac
}

// tolerancePick finds the tolerance letter for an encode request given as a letter, "±0.25 pF", "10 %" or "-20/+80", the
// rows of that letter and the tolerance (percent) that selects the E-series (0 = do not snap).
func (d *codeCapacitorData) tolerancePick(s string, capF float64, hasCap bool) (letter string, rows []*codeCapTolerance, pct float64, err error) {
	t := strings.TrimSpace(s)
	match := func(ok func(*codeCapTolerance) bool) (string, []*codeCapTolerance) {
		for i := range d.Tolerances {
			x := &d.Tolerances[i]
			if !ok(x) {
				continue
			}
			if r, err := d.toleranceRows(x.Letter, capF, hasCap); err == nil {
				for _, y := range r {
					if y == x {
						return x.Letter, []*codeCapTolerance{x}
					}
				}
			}
		}
		return "", nil
	}
	fail := func() (string, []*codeCapTolerance, float64, error) {
		return "", nil, 0, codeErr(CodeUnsupported, "no tolerance letter for %q (letters: %s)", s, d.letterList())
	}
	switch {
	case codeCapLetterRe.MatchString(strings.ToUpper(t)):
		l := strings.ToUpper(t)
		rows, err := d.toleranceRows(l, capF, hasCap)
		if err != nil {
			return "", nil, 0, err
		}
		if r := rows[0]; len(rows) == 1 && r.Unit == "pct" {
			pct = r.Minus
			if r.Minus != r.Plus {
				pct = 20 // an asymmetric tolerance belongs to the coarse E6 values
			}
		}
		return l, rows, pct, nil
	case strings.HasSuffix(strings.ToLower(codeCompact(t)), "pf"):
		c := codeCompact(t)
		x, ok := codeParsePercent(c[:len(c)-2])
		if !ok {
			return "", nil, 0, codeErr(CodeInvalidValue, "tolerance: cannot read %q", s)
		}
		l, r := match(func(y *codeCapTolerance) bool { return y.Unit == "pf" && codeCloseTo(y.Minus, x) })
		if l == "" {
			return fail()
		}
		return l, r, 0, nil
	}
	if plus, minus, ok := codeParseAsym(t); ok {
		l, r := match(func(y *codeCapTolerance) bool {
			return y.Unit == "pct" && codeCloseTo(y.Plus, plus) && codeCloseTo(y.Minus, minus)
		})
		if l == "" {
			return fail()
		}
		return l, r, 20, nil
	}
	p, ok := codeParsePercent(t)
	if !ok || p <= 0 {
		return "", nil, 0, codeErr(CodeInvalidValue, "tolerance: cannot read %q as a letter, a percentage, a value in pF or an asymmetric tolerance such as -20/+80", s)
	}
	l, r := match(func(y *codeCapTolerance) bool { return y.Unit == "pct" && y.Minus == y.Plus && codeCloseTo(y.Minus, p) })
	if l == "" {
		return fail()
	}
	return l, r, p, nil
}

func (d *codeCapacitorData) letterList() string {
	var out []string
	seen := map[string]bool{}
	for _, t := range d.Tolerances {
		if !seen[t.Letter] {
			seen[t.Letter] = true
			out = append(out, t.Letter)
		}
	}
	return strings.Join(out, " ")
}

func (d *codeCapacitorData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	hasValue := strings.TrimSpace(req.Value) != ""
	hasTol := strings.TrimSpace(req.Tolerance) != ""
	hasVolt := strings.TrimSpace(req.Voltage) != ""
	if !hasValue && !hasTol && !hasVolt {
		return CodeResult{}, codeErr(CodeInvalidValue, "give a value, a tolerance or a voltage")
	}
	res := codeNewResult("capacitor")
	var parts []string
	var capV float64
	if hasValue {
		v, err := u.Parse(req.Value, "capacitance", "")
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
		}
		if !(v > 0) {
			return CodeResult{}, codeErr(CodeInvalidValue, "value: the capacitance must be positive")
		}
		capV = v
	}
	var letter string
	var rows []*codeCapTolerance
	pct := 0.0
	if hasTol {
		var err error
		if letter, rows, pct, err = d.tolerancePick(req.Tolerance, capV, hasValue); err != nil {
			return CodeResult{}, err
		}
	}
	if hasValue {
		vs := capV
		changed := false
		if pct > 0 {
			var err error
			if vs, changed, err = codeSnapToSeries(capV, pct); err != nil {
				return CodeResult{}, err
			}
		}
		N, e, exact := codeSigDigits(vs, 2)
		if !exact {
			return CodeResult{}, codeErr(CodeUnsupported, "%s needs more than 2 significant digits; the 3-digit capacitor code holds 2", u.Format(vs, "capacitance", "", 4))
		}
		vc := codeCapValueCode(N, e)
		if vc == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "%s is outside the range of the capacitor code (%s to %s)", u.Format(vs, "capacitance", "", 4), u.Format(codeDecimal(10, -13), "capacitance", "", 4), u.Format(codeDecimal(99, -6), "capacitance", "", 4))
		}
		res.Value, res.Dim = codeDecimal(N, e), "capacitance"
		res.field("value_code", "Value code", vc, math.NaN(), "")
		res.field("rkm", "RKM notation", codeCapRKMCode(N, e), math.NaN(), "")
		parts = append(parts, vc+letter)
		if changed {
			res.warn("%s", codeSnapWarning(u, "capacitance", capV, vs, pct))
		}
	} else if letter != "" {
		parts = append(parts, letter)
	}
	if letter != "" {
		codeAddTolerance(&res, rows, hasValue)
	}
	if hasVolt {
		v, err := u.Parse(req.Voltage, "voltage", "")
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "voltage: %v", err)
		}
		var row *codeCapVoltage
		for i := range d.Voltages {
			if codeCloseTo(d.Voltages[i].Volts, v) {
				row = &d.Voltages[i]
			}
		}
		if row == nil {
			return CodeResult{}, codeErr(CodeUnsupported, "no standard voltage code for %s", u.Format(v, "voltage", "", 4))
		}
		d.addVoltage(&res, u, row, !hasValue)
		parts = append(parts, row.Code)
	}
	res.Code = strings.Join(parts, " ")
	return res, nil
}
