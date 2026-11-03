package ecalc

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// The colour band kinds: resistor_bands (IEC 60062, 3 to 6 bands) and inductor_bands (EIA style with 4 bands, military
// style with 5 bands). The colours, their digits, multipliers (as exponents of ten), tolerances and temperature
// coefficients are the data of the code item.

const (
	codeRoleDigit      = "digit"
	codeRoleMultiplier = "multiplier"
	codeRoleTolerance  = "tolerance"
	codeRoleTCR        = "tcr"
)

var codeRoleLabel = map[string]string{codeRoleDigit: "a digit", codeRoleMultiplier: "a multiplier", codeRoleTolerance: "a tolerance", codeRoleTCR: "a temperature coefficient"}

// codeBandColor is one colour of the resistor code. Exp is the exponent of the multiplier, Tol the tolerance in percent, TCR
// the temperature coefficient in ppm/K; LegacyTol / LegacyNone give what older charts print for the same colour.
type codeBandColor struct {
	ID         string   `yaml:"id"`
	IEC        string   `yaml:"iec"`
	Hex        string   `yaml:"hex"`
	Name       Text     `yaml:"name"`
	Digit      *int     `yaml:"digit"`
	Exp        *int     `yaml:"exp"`
	Tol        *float64 `yaml:"tol"`
	TCR        *float64 `yaml:"tcr"`
	LegacyTol  *float64 `yaml:"legacy_tol"`
	LegacyNone bool     `yaml:"legacy_tol_none"`
}

type codeBandLayout struct {
	Bands int      `yaml:"bands" json:"bands"`
	Roles []string `yaml:"roles" json:"roles"`
}

type codeResistorData struct {
	Colors  []codeBandColor  `yaml:"colors"`
	Layouts []codeBandLayout `yaml:"layouts"`
	Notes   []Text           `yaml:"notes"`

	byName  map[string]*codeBandColor // ids, IEC 60757 letters (lower case) and "gray"; "none" is no band
	byID    map[string]*codeBandColor
	byDigit map[int]*codeBandColor
	byExp   map[int]*codeBandColor
	layout  map[int]*codeBandLayout
}

func codeLoadResistorBands(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeResistorData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.byName, d.byID, d.byDigit, d.byExp, d.layout = map[string]*codeBandColor{}, map[string]*codeBandColor{}, map[int]*codeBandColor{}, map[int]*codeBandColor{}, map[int]*codeBandLayout{}
	for i := range d.Colors {
		c := &d.Colors[i]
		if !idRe.MatchString(c.ID) {
			fail("data: colors[%d]: id must match %s", i, idRe)
			continue
		}
		if d.byID[c.ID] != nil {
			fail("data: colour %s appears twice", c.ID)
			continue
		}
		d.byID[c.ID] = c
		codeNeedLocales(fail, "colour "+c.ID+" name", c.Name)
		if !codeHexRe.MatchString(c.Hex) {
			fail("data: colour %s: hex must be #rrggbb or transparent", c.ID)
		}
		if c.Digit != nil {
			if *c.Digit < 0 || *c.Digit > 9 || d.byDigit[*c.Digit] != nil {
				fail("data: colour %s: digit must be 0 to 9 and used once", c.ID)
			} else {
				d.byDigit[*c.Digit] = c
			}
		}
		if c.Exp != nil {
			if *c.Exp < -3 || *c.Exp > 9 || d.byExp[*c.Exp] != nil {
				fail("data: colour %s: exp must be -3 to 9 and used once", c.ID)
			} else {
				d.byExp[*c.Exp] = c
			}
		}
		for what, p := range map[string]*float64{"tol": c.Tol, "tcr": c.TCR, "legacy_tol": c.LegacyTol} {
			if p != nil && !(*p > 0) {
				fail("data: colour %s: %s must be positive", c.ID, what)
			}
		}
		if c.ID == "none" {
			continue
		}
		names := []string{c.ID}
		if c.IEC != "" {
			names = append(names, strings.ToLower(c.IEC))
		}
		if c.ID == "grey" {
			names = append(names, "gray")
		}
		for _, n := range names {
			if d.byName[n] != nil {
				fail("data: colour name %q is used twice", n)
			}
			d.byName[n] = c
		}
	}
	if b := d.byID["black"]; b == nil || b.Digit == nil || *b.Digit != 0 || b.Exp == nil || *b.Exp != 0 {
		fail("data: colour black with digit 0 and exp 0 is required")
	}
	if n := d.byID["none"]; n == nil || n.Tol == nil || *n.Tol != 20 {
		fail("data: colour none with the tolerance 20 is required (a code without a tolerance band)")
	}
	for i, l := range d.Layouts {
		if len(l.Roles) != l.Bands || l.Bands < 3 || l.Bands > 6 || d.layout[l.Bands] != nil {
			fail("data: layouts[%d]: bands must be 3 to 6, used once, with one role per band", i)
			continue
		}
		multipliers := 0
		for j, r := range l.Roles {
			if codeRoleLabel[r] == "" {
				fail("data: layouts[%d]: unknown role %q", i, r)
			}
			if (j == 0) != (r == codeRoleDigit && j == 0) {
				fail("data: layouts[%d]: the first band is a digit", i)
			}
			if r == codeRoleMultiplier {
				multipliers++
			}
		}
		if multipliers != 1 {
			fail("data: layouts[%d]: exactly one multiplier band is required", i)
		}
		d.layout[l.Bands] = &d.Layouts[i]
	}
	if len(d.Layouts) == 0 {
		fail("data: layouts must not be empty")
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeBandColorJSON struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	IEC           string   `json:"iec"`
	Hex           string   `json:"hex"`
	Digit         *int     `json:"digit"`
	Exp           *int     `json:"exp"`
	Tol           *float64 `json:"tol"`
	TCR           *float64 `json:"tcr"`
	LegacyTol     *float64 `json:"legacy_tol"`
	LegacyTolNone bool     `json:"legacy_tol_none"`
}

type codeResistorBandsDoc struct {
	Colors  []codeBandColorJSON `json:"colors"`
	Layouts []codeBandLayout    `json:"layouts"`
	Notes   []string            `json:"notes"`
}

func codeResistorBandsJSON(data any, lang string) any {
	d := data.(*codeResistorData)
	out := codeResistorBandsDoc{Colors: make([]codeBandColorJSON, 0, len(d.Colors)), Layouts: orEmpty(d.Layouts), Notes: codeLocalizedNotes(d.Notes, lang)}
	for _, c := range d.Colors {
		out.Colors = append(out.Colors, codeBandColorJSON{ID: c.ID, Name: c.Name.in(lang), IEC: c.IEC, Hex: c.Hex, Digit: c.Digit, Exp: c.Exp, Tol: c.Tol, TCR: c.TCR, LegacyTol: c.LegacyTol, LegacyTolNone: c.LegacyNone})
	}
	return out
}

// codeBandsOf reads a list of colour names.
func codeBandsOf(byName map[string]*codeBandColor, code string) ([]*codeBandColor, error) {
	words := codeSplitWords(code)
	if len(words) == 0 {
		return nil, codeErr(CodeInvalidCode, "the code is empty")
	}
	out := make([]*codeBandColor, 0, len(words))
	for _, w := range words {
		c := byName[w]
		if c == nil {
			var ids []string
			for n, c := range byName {
				if n == c.ID {
					ids = append(ids, n)
				}
			}
			sort.Strings(ids)
			return nil, codeErr(CodeInvalidCode, "unknown colour %q (colours: %s)", w, strings.Join(ids, ", "))
		}
		out = append(out, c)
	}
	return out, nil
}

func codeBandIDs(cs []*codeBandColor) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

// codeLegacyWarning says how older charts read a tolerance colour that IEC 60062:2016 assigned later ("" when they agree).
func codeLegacyWarning(c *codeBandColor) string {
	switch {
	case c.LegacyTol != nil:
		return fmt.Sprintf("%s tolerance band: ±%s %% in IEC 60062:2016; older charts and some makers read ±%s %%", c.ID, fmtSig(*c.Tol, 6), fmtSig(*c.LegacyTol, 6))
	case c.LegacyNone:
		return fmt.Sprintf("%s tolerance band: ±%s %% in IEC 60062:2016; older charts give %s no tolerance", c.ID, fmtSig(*c.Tol, 6), c.ID)
	}
	return ""
}

func codeToleranceField(res *CodeResult, tol float64) {
	res.field("tolerance", "Tolerance", "±"+fmtSig(tol, 6)+" %", codeScaleDec(tol, -2), "ratio")
}

func (d *codeResistorData) decode(u *Units, code string) (CodeResult, error) {
	cs, err := codeBandsOf(d.byName, code)
	if err != nil {
		return CodeResult{}, err
	}
	res := codeNewResult("resistor_bands")
	res.Dim = "resistance"
	res.Bands = codeBandIDs(cs)
	res.Code = strings.Join(res.Bands, " ")
	if len(cs) == 1 {
		if cs[0].ID != "black" {
			return CodeResult{}, codeErr(CodeInvalidCode, "a single band must be black (a zero-ohm jumper)")
		}
		res.Value = 0
		res.warn("a single black band marks a zero-ohm jumper (no tolerance)")
		return res, nil
	}
	lay := d.layout[len(cs)]
	if lay == nil {
		return CodeResult{}, codeErr(CodeInvalidCode, "a resistor code has 3 to 6 bands (or one black band), got %d", len(cs))
	}
	digits, exp := 0, 0
	var tol, tcr *codeBandColor
	for i, c := range cs {
		role := lay.Roles[i]
		ok := false
		switch role {
		case codeRoleDigit:
			ok = c.Digit != nil
		case codeRoleMultiplier:
			ok = c.Exp != nil
		case codeRoleTolerance:
			ok = c.Tol != nil
		case codeRoleTCR:
			ok = c.TCR != nil
		}
		if !ok {
			if i == 0 {
				return CodeResult{}, codeErr(CodeInvalidCode, "the first band cannot be %s (gold, silver and pink never stand first); the bands may be read from the wrong end", c.ID)
			}
			return CodeResult{}, codeErr(CodeInvalidCode, "band %d (%s) cannot be %s", i+1, c.ID, codeRoleLabel[role])
		}
		switch role {
		case codeRoleDigit:
			digits = digits*10 + *c.Digit
		case codeRoleMultiplier:
			exp = *c.Exp
		case codeRoleTolerance:
			tol = c
		case codeRoleTCR:
			tcr = c
		}
	}
	res.Value = codeDecimal(digits, exp)
	if tol == nil {
		codeToleranceField(&res, *d.byID["none"].Tol)
	} else {
		codeToleranceField(&res, *tol.Tol)
		if w := codeLegacyWarning(tol); w != "" {
			res.warn("%s", w)
		}
	}
	if tcr != nil {
		res.field("tcr", "Temperature coefficient", "±"+fmtSig(*tcr.TCR, 6)+" ppm/K", codeScaleDec(*tcr.TCR, -6), "tempco")
	}
	if res.Value > 0 && !codeSeriesHas("E24", res.Value) && !codeSeriesHas("E192", res.Value) {
		res.warn("the value is not an E24 or E192 value; check the reading direction")
	}
	return res, nil
}

// codeParseTempco reads a temperature coefficient ("50", "50ppm", "±50 ppm/K") in 1/K.
func codeParseTempco(u *Units, s string) (float64, error) {
	t := strings.TrimPrefix(strings.TrimSpace(s), "±")
	if strings.HasSuffix(strings.ToLower(t), "ppm") {
		t = t[:len(t)-3] + "ppm/K"
	}
	return u.Parse(t, "tempco", "ppmK")
}

// tolerances lists the tolerance values (percent) of the coloured bands, ascending, as text.
func (d *codeResistorData) toleranceList() string {
	var vals []float64
	for _, c := range d.Colors {
		if c.Tol != nil {
			vals = append(vals, *c.Tol)
		}
	}
	sort.Float64s(vals)
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmtSig(v, 6)
	}
	return strings.Join(parts, ", ")
}

func (d *codeResistorData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	v, err := u.Parse(req.Value, "resistance", "")
	if err != nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
	}
	if v < 0 {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: the resistance must not be negative")
	}
	res := codeNewResult("resistor_bands")
	res.Dim = "resistance"
	if v == 0 {
		res.Bands, res.Code, res.Value = []string{"black"}, "black", 0
		res.warn("a single black band marks a zero-ohm jumper (no tolerance)")
		return res, nil
	}
	bands := req.Bands
	if bands != 0 && d.layout[bands] == nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "bands must be 0 (choose), 3, 4, 5 or 6")
	}

	// tolerance
	tol, assumed := 20.0, true // the 3-band code without a tolerance band is +-20 %
	if bands != 3 || strings.TrimSpace(req.Tolerance) != "" {
		var err error
		if tol, assumed, err = codeRequestedTolerance(req, v); err != nil {
			return CodeResult{}, err
		}
	}
	var tolC *codeBandColor
	for i := range d.Colors {
		if c := &d.Colors[i]; c.Tol != nil && codeCloseTo(*c.Tol, tol) {
			tolC = c
		}
	}
	if tolC == nil {
		return CodeResult{}, codeErr(CodeUnsupported, "no band colour for a tolerance of ±%s %% (tolerances: %s)", fmtSig(tol, 6), d.toleranceList())
	}

	// temperature coefficient
	var tcrC *codeBandColor
	if strings.TrimSpace(req.TempCo) != "" {
		x, err := codeParseTempco(u, req.TempCo)
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "tempco: %v", err)
		}
		for i := range d.Colors {
			if c := &d.Colors[i]; c.TCR != nil && codeCloseTo(codeScaleDec(*c.TCR, -6), x) {
				tcrC = c
			}
		}
		if tcrC == nil {
			return CodeResult{}, codeErr(CodeUnsupported, "no band colour for a temperature coefficient of %s ppm/K", fmtSig(x/1e-6, 6))
		}
		if bands != 0 && bands != 6 {
			return CodeResult{}, codeErr(CodeInvalidValue, "a temperature coefficient needs the 6-band code")
		}
		bands = 6
	}
	if bands == 6 && tcrC == nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "the 6-band code needs a temperature coefficient (tempco)")
	}
	noTol := tolC.ID == "none"
	if bands == 3 && !noTol {
		return CodeResult{}, codeErr(CodeInvalidValue, "the 3-band code has no tolerance band (±20 %%); give a tolerance of 20%% or use 4 bands")
	}
	if noTol {
		if bands > 3 {
			return CodeResult{}, codeErr(CodeUnsupported, "±20 %% has no tolerance colour; the code without a tolerance band has 3 bands")
		}
		bands = 3
	}

	// the standard value
	vs, changed, err := codeSnapToSeries(v, tol)
	if err != nil {
		return CodeResult{}, err
	}
	_, _, two := codeSigDigits(vs, 2)
	if bands == 0 {
		if two {
			bands = 4
		} else {
			bands = 5
		}
	}
	n := 3
	if bands == 3 || bands == 4 {
		n = 2
		if !two {
			return CodeResult{}, codeErr(CodeUnsupported, "%s needs 3 significant digits; a %d-band code holds 2 (use 5 or 6 bands)", u.Format(vs, "resistance", "", 4), bands)
		}
	}
	N, e, exact := codeSigDigits(vs, n)
	if !exact {
		return CodeResult{}, codeErr(CodeUnsupported, "%s needs more than %d significant digits", u.Format(vs, "resistance", "", 4), n)
	}
	mult := d.byExp[e]
	if mult == nil {
		lo, hi := codeDecimal(int(math.Pow10(n-1)), -3), codeDecimal(int(math.Pow10(n))-1, 9)
		return CodeResult{}, codeErr(CodeUnsupported, "%s is outside the range of a %d-band code (%s to %s)", u.Format(vs, "resistance", "", 4), bands, u.Format(lo, "resistance", "", 4), u.Format(hi, "resistance", "", 4))
	}
	digitsText := fmt.Sprintf("%0*d", n, N)
	for _, ch := range digitsText {
		res.Bands = append(res.Bands, d.byDigit[int(ch-'0')].ID)
	}
	res.Bands = append(res.Bands, mult.ID)
	if !noTol {
		res.Bands = append(res.Bands, tolC.ID)
	}
	if tcrC != nil {
		res.Bands = append(res.Bands, tcrC.ID)
	}
	res.Code = strings.Join(res.Bands, " ")
	res.Value = codeDecimal(N, e)
	codeToleranceField(&res, tol)
	if tcrC != nil {
		res.field("tcr", "Temperature coefficient", "±"+fmtSig(*tcrC.TCR, 6)+" ppm/K", codeScaleDec(*tcrC.TCR, -6), "tempco")
	}
	if assumed {
		res.warn("no tolerance given; ±%s %% assumed", fmtSig(tol, 6))
	}
	if changed {
		res.warn("%s", codeSnapWarning(u, "resistance", v, vs, tol))
	}
	if !noTol {
		if w := codeLegacyWarning(tolC); w != "" {
			res.warn("%s", w)
		}
	}
	return res, nil
}

// ---- inductor_bands ------------------------------------------------------------------------------------------------------

// codeInductorColor is one colour of the inductor code. EIAExp / MILExp are exponents of ten of the multiplier (value in µH);
// MILRole is "point" (gold, the decimal point of the military style) or "identifier" (silver, the wide first band).
type codeInductorColor struct {
	ID     string   `yaml:"id"`
	Hex    string   `yaml:"hex"`
	Name   Text     `yaml:"name"`
	Digit  *int     `yaml:"digit"`
	EIAExp *int     `yaml:"eia_exp"`
	EIATol *float64 `yaml:"eia_tol"`
	MILExp *int     `yaml:"mil_exp"`
	MILTol *float64 `yaml:"mil_tol"`
	Role   string   `yaml:"mil_role"`
}

type codeInductorData struct {
	Colors []codeInductorColor `yaml:"colors"`
	Notes  []Text              `yaml:"notes"`

	byName     map[string]*codeInductorColor
	byDigit    map[int]*codeInductorColor
	byEIAExp   map[int]*codeInductorColor
	byMILExp   map[int]*codeInductorColor
	identifier *codeInductorColor
	point      *codeInductorColor
}

func codeLoadInductorBands(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeInductorData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.byName, d.byDigit, d.byEIAExp, d.byMILExp = map[string]*codeInductorColor{}, map[int]*codeInductorColor{}, map[int]*codeInductorColor{}, map[int]*codeInductorColor{}
	for i := range d.Colors {
		c := &d.Colors[i]
		if !idRe.MatchString(c.ID) || d.byName[c.ID] != nil {
			fail("data: colors[%d]: id %q must match %s and be unique", i, c.ID, idRe)
			continue
		}
		d.byName[c.ID] = c
		if c.ID == "grey" {
			d.byName["gray"] = c
		}
		codeNeedLocales(fail, "colour "+c.ID+" name", c.Name)
		if !codeHexRe.MatchString(c.Hex) {
			fail("data: colour %s: hex must be #rrggbb", c.ID)
		}
		if c.Digit != nil {
			if *c.Digit < 0 || *c.Digit > 9 || d.byDigit[*c.Digit] != nil {
				fail("data: colour %s: digit must be 0 to 9 and used once", c.ID)
			} else {
				d.byDigit[*c.Digit] = c
			}
		}
		if c.EIAExp != nil {
			if *c.EIAExp < -2 || *c.EIAExp > 3 || d.byEIAExp[*c.EIAExp] != nil {
				fail("data: colour %s: eia_exp must be -2 to 3 and used once", c.ID)
			} else {
				d.byEIAExp[*c.EIAExp] = c
			}
		}
		if c.MILExp != nil {
			if *c.MILExp < 0 || *c.MILExp > 3 || d.byMILExp[*c.MILExp] != nil {
				fail("data: colour %s: mil_exp must be 0 to 3 and used once", c.ID)
			} else {
				d.byMILExp[*c.MILExp] = c
			}
		}
		for what, p := range map[string]*float64{"eia_tol": c.EIATol, "mil_tol": c.MILTol} {
			if p != nil && !(*p > 0) {
				fail("data: colour %s: %s must be positive", c.ID, what)
			}
		}
		switch c.Role {
		case "":
		case "point":
			d.point = c
		case "identifier":
			d.identifier = c
		default:
			fail("data: colour %s: mil_role must be point or identifier", c.ID)
		}
	}
	for n := 0; n <= 9; n++ {
		if d.byDigit[n] == nil {
			fail("data: no colour for the digit %d", n)
		}
	}
	if d.point == nil || d.identifier == nil {
		fail("data: the military style needs a colour with mil_role point and one with mil_role identifier")
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeInductorColorJSON struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Hex    string   `json:"hex"`
	Digit  *int     `json:"digit"`
	EIAExp *int     `json:"eia_exp"`
	EIATol *float64 `json:"eia_tol"`
	MILExp *int     `json:"mil_exp"`
	MILTol *float64 `json:"mil_tol"`
	Role   string   `json:"mil_role"`
}

type codeInductorBandsDoc struct {
	Colors []codeInductorColorJSON `json:"colors"`
	Notes  []string                `json:"notes"`
}

func codeInductorBandsJSON(data any, lang string) any {
	d := data.(*codeInductorData)
	out := codeInductorBandsDoc{Colors: make([]codeInductorColorJSON, 0, len(d.Colors)), Notes: codeLocalizedNotes(d.Notes, lang)}
	for _, c := range d.Colors {
		out.Colors = append(out.Colors, codeInductorColorJSON{ID: c.ID, Name: c.Name.in(lang), Hex: c.Hex, Digit: c.Digit, EIAExp: c.EIAExp, EIATol: c.EIATol, MILExp: c.MILExp, MILTol: c.MILTol, Role: c.Role})
	}
	return out
}

const (
	codeLayoutEIA = "EIA (4 bands)"
	codeLayoutMIL = "MIL-PRF-15305 (5 bands, wide silver first)"
)

func codeInductorBandIDs(cs []*codeInductorColor) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func (d *codeInductorData) decode(u *Units, code string) (CodeResult, error) {
	words := codeSplitWords(code)
	if len(words) == 0 {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is empty")
	}
	cs := make([]*codeInductorColor, 0, len(words))
	for _, w := range words {
		c := d.byName[w]
		if c == nil {
			return CodeResult{}, codeErr(CodeInvalidCode, "unknown colour %q", w)
		}
		cs = append(cs, c)
	}
	res := codeNewResult("inductor_bands")
	res.Dim = "inductance"
	res.Bands = codeInductorBandIDs(cs)
	res.Code = strings.Join(res.Bands, " ")
	digit := func(i int) (int, error) {
		if cs[i].Digit == nil {
			return 0, codeErr(CodeInvalidCode, "band %d (%s) cannot be a digit", i+1, cs[i].ID)
		}
		return *cs[i].Digit, nil
	}
	var value float64
	var tol float64
	switch len(cs) {
	case 4: // EIA: digit, digit, multiplier, tolerance
		d1, err := digit(0)
		if err != nil {
			return CodeResult{}, err
		}
		d2, err := digit(1)
		if err != nil {
			return CodeResult{}, err
		}
		if cs[2].EIAExp == nil {
			return CodeResult{}, codeErr(CodeInvalidCode, "band 3 (%s) cannot be a multiplier (black, brown, red, orange, gold or silver)", cs[2].ID)
		}
		if cs[3].EIATol == nil {
			return CodeResult{}, codeErr(CodeUnsupported, "the tolerance colour %s is not cross-checked for inductors (gold ±5 %%, silver ±10 %%)", cs[3].ID)
		}
		value = codeDecimal(d1*10+d2, *cs[2].EIAExp-6)
		tol = *cs[3].EIATol
		res.field("layout", "Layout", codeLayoutEIA, math.NaN(), "")
	case 5: // military: identifier, three value bands, tolerance
		if cs[0] != d.identifier {
			return CodeResult{}, codeErr(CodeInvalidCode, "a 5-band inductor code starts with the wide %s military band", d.identifier.ID)
		}
		if cs[4].MILTol == nil {
			return CodeResult{}, codeErr(CodeUnsupported, "the tolerance colour %s is not cross-checked for inductors (gold ±5 %%, silver ±10 %%)", cs[4].ID)
		}
		tol = *cs[4].MILTol
		switch {
		case cs[1] == d.point && cs[2] == d.point, cs[3] == d.point:
			return CodeResult{}, codeErr(CodeInvalidCode, "%s is the decimal point: only band 2 or 3 can be %s, not both and not band 4", d.point.ID, d.point.ID)
		case cs[1] == d.point: // 0.d d
			a, err := digit(2)
			if err != nil {
				return CodeResult{}, err
			}
			b, err := digit(3)
			if err != nil {
				return CodeResult{}, err
			}
			value = codeDecimal(a*10+b, -2-6)
		case cs[2] == d.point: // d.d
			a, err := digit(1)
			if err != nil {
				return CodeResult{}, err
			}
			b, err := digit(3)
			if err != nil {
				return CodeResult{}, err
			}
			value = codeDecimal(a*10+b, -1-6)
		default: // dd x multiplier
			a, err := digit(1)
			if err != nil {
				return CodeResult{}, err
			}
			b, err := digit(2)
			if err != nil {
				return CodeResult{}, err
			}
			if cs[3].MILExp == nil {
				return CodeResult{}, codeErr(CodeInvalidCode, "band 4 (%s) cannot be a multiplier (black, brown, red or orange)", cs[3].ID)
			}
			value = codeDecimal(a*10+b, *cs[3].MILExp-6)
		}
		res.field("layout", "Layout", codeLayoutMIL, math.NaN(), "")
	default:
		return CodeResult{}, codeErr(CodeInvalidCode, "an inductor code has 4 bands (EIA) or 5 bands (military, wide silver first), got %d", len(cs))
	}
	if value == 0 {
		return CodeResult{}, codeErr(CodeInvalidCode, "the bands give an inductance of zero")
	}
	res.Value = value
	codeToleranceField(&res, tol)
	return res, nil
}

func (d *codeInductorData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	v, err := u.Parse(req.Value, "inductance", "")
	if err != nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
	}
	if !(v > 0) {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: the inductance must be positive")
	}
	mil := false
	switch req.Bands {
	case 0, 4:
	case 5:
		mil = true
	default:
		return CodeResult{}, codeErr(CodeInvalidValue, "bands must be 4 (EIA) or 5 (military)")
	}
	res := codeNewResult("inductor_bands")
	res.Dim = "inductance"

	tol, assumed, err := codeRequestedTolerance(req, v)
	if err != nil {
		return CodeResult{}, err
	}
	var tolC *codeInductorColor
	for i := range d.Colors {
		c := &d.Colors[i]
		if c.EIATol != nil && c.MILTol != nil && codeCloseTo(*c.EIATol, tol) {
			tolC = c
		}
	}
	if tolC == nil {
		return CodeResult{}, codeErr(CodeUnsupported, "no band colour for a tolerance of ±%s %% (gold ±5 %%, silver ±10 %%)", fmtSig(tol, 6))
	}
	vs, changed, err := codeSnapToSeries(v, tol)
	if err != nil {
		return CodeResult{}, err
	}
	N, e, exact := codeSigDigits(vs, 2)
	if !exact {
		return CodeResult{}, codeErr(CodeUnsupported, "%s needs more than 2 significant digits", u.Format(vs, "inductance", "", 4))
	}
	micro := e + 6 // N * 10^micro is the value in µH
	digitColors := func(n int) []string { return []string{d.byDigit[n/10].ID, d.byDigit[n%10].ID} }
	if !mil {
		mult := d.byEIAExp[micro]
		if mult == nil {
			return CodeResult{}, codeErr(CodeUnsupported, "%s is outside the range of the EIA code (%s to %s)", u.Format(vs, "inductance", "", 4), u.Format(codeDecimal(10, -8), "inductance", "", 4), u.Format(codeDecimal(99, -3), "inductance", "", 4))
		}
		res.Bands = append(digitColors(N), mult.ID, tolC.ID)
		res.field("layout", "Layout", codeLayoutEIA, math.NaN(), "")
	} else {
		a, b := digitColors(N)[0], digitColors(N)[1]
		switch {
		case micro >= 0 && micro <= 3:
			res.Bands = []string{d.identifier.ID, a, b, d.byMILExp[micro].ID, tolC.ID}
		case micro == -1: // d.d
			res.Bands = []string{d.identifier.ID, a, d.point.ID, b, tolC.ID}
		case micro == -2: // 0.dd
			res.Bands = []string{d.identifier.ID, d.point.ID, a, b, tolC.ID}
		default:
			return CodeResult{}, codeErr(CodeUnsupported, "%s is outside the range of the military code (%s to %s)", u.Format(vs, "inductance", "", 4), u.Format(codeDecimal(10, -8), "inductance", "", 4), u.Format(codeDecimal(99, -3), "inductance", "", 4))
		}
		res.field("layout", "Layout", codeLayoutMIL, math.NaN(), "")
	}
	res.Code = strings.Join(res.Bands, " ")
	res.Value = codeDecimal(N, e)
	codeToleranceField(&res, tol)
	if assumed {
		res.warn("no tolerance given; ±%s %% assumed", fmtSig(tol, 6))
	}
	if changed {
		res.warn("%s", codeSnapWarning(u, "inductance", v, vs, tol))
	}
	return res, nil
}
