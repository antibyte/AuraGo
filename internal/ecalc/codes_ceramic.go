package ecalc

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// The ceramic_temp kind: the EIA-198 temperature characteristics of ceramic capacitors. A class 1 code (C0G, U2J) is a
// letter for the significant figure of the temperature coefficient, a digit for the multiplier and a letter for the
// tolerance, in ppm/K; a class 2 code (X7R, Y5V) is a letter for the lower and a digit for the upper category temperature
// and a letter for the capacitance change over that range. The letters and digits that only one source lists are not in
// the data (not_cross_checked names them so that the decoder can say so).

type codeCeramicSig struct {
	Letter string  `yaml:"letter" json:"letter"`
	Value  float64 `yaml:"value" json:"value"` // the significant figure
}

type codeCeramicMult struct {
	Digit string  `yaml:"digit" json:"digit"`
	Value float64 `yaml:"value" json:"value"` // the multiplier
}

type codeCeramicTol struct {
	Letter string  `yaml:"letter" json:"letter"`
	PPM    float64 `yaml:"ppm" json:"ppm"`
}

type codeCeramicLow struct {
	Letter string  `yaml:"letter" json:"letter"`
	C      float64 `yaml:"c" json:"c"`
}

type codeCeramicHigh struct {
	Digit string  `yaml:"digit" json:"digit"`
	C     float64 `yaml:"c" json:"c"`
}

type codeCeramicChange struct {
	Letter string  `yaml:"letter" json:"letter"`
	Plus   float64 `yaml:"plus" json:"plus"`
	Minus  float64 `yaml:"minus" json:"minus"`
}

type codeCeramicClass1 struct {
	Sig  []codeCeramicSig  `yaml:"sig" json:"sig"`
	Mult []codeCeramicMult `yaml:"mult" json:"mult"`
	Tol  []codeCeramicTol  `yaml:"tol" json:"tol"`
}

type codeCeramicClass2 struct {
	Low    []codeCeramicLow    `yaml:"low" json:"low"`
	High   []codeCeramicHigh   `yaml:"high" json:"high"`
	Change []codeCeramicChange `yaml:"change" json:"change"`
}

type codeCeramicAlias struct {
	Name string `yaml:"name" json:"name"`
	Code string `yaml:"code" json:"code"`
}

type codeCeramicIndustry struct {
	Code string `yaml:"code" json:"code"`
	Name string `yaml:"name" json:"name"`
}

type codeCeramicNotChecked struct {
	C1Sig    []string `yaml:"c1_sig" json:"c1_sig"`
	C1Mult   []string `yaml:"c1_mult" json:"c1_mult"`
	C2High   []string `yaml:"c2_high" json:"c2_high"`
	C2Change []string `yaml:"c2_change" json:"c2_change"`
}

type codeCeramicData struct {
	Class1     codeCeramicClass1     `yaml:"class1"`
	Class2     codeCeramicClass2     `yaml:"class2"`
	Aliases    []codeCeramicAlias    `yaml:"aliases"`
	Industry   []codeCeramicIndustry `yaml:"industry"`
	NotChecked codeCeramicNotChecked `yaml:"not_cross_checked"`
	Common     []string              `yaml:"common"`
	Notes      []Text                `yaml:"notes"`

	sig      map[string]*codeCeramicSig
	mult     map[string]*codeCeramicMult
	tol      map[string]*codeCeramicTol
	low      map[string]*codeCeramicLow
	high     map[string]*codeCeramicHigh
	change   map[string]*codeCeramicChange
	industry map[string]string
}

func codeLoadCeramicTemp(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeCeramicData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.sig, d.mult, d.tol = map[string]*codeCeramicSig{}, map[string]*codeCeramicMult{}, map[string]*codeCeramicTol{}
	d.low, d.high, d.change, d.industry = map[string]*codeCeramicLow{}, map[string]*codeCeramicHigh{}, map[string]*codeCeramicChange{}, map[string]string{}
	oneChar := func(what, s string, isLetter bool) bool {
		ok := len(s) == 1 && ((isLetter && s >= "A" && s <= "Z") || (!isLetter && s >= "0" && s <= "9"))
		if !ok {
			fail("data: %s: %q must be a single %s", what, s, map[bool]string{true: "capital letter", false: "digit"}[isLetter])
		}
		return ok
	}
	for i := range d.Class1.Sig {
		x := &d.Class1.Sig[i]
		if oneChar("class1.sig", x.Letter, true) && d.sig[x.Letter] == nil && x.Value >= 0 {
			d.sig[x.Letter] = x
		} else {
			fail("data: class1.sig[%d]: a unique letter and a figure of 0 or more are required", i)
		}
	}
	for i := range d.Class1.Mult {
		x := &d.Class1.Mult[i]
		if oneChar("class1.mult", x.Digit, false) && d.mult[x.Digit] == nil && x.Value != 0 {
			d.mult[x.Digit] = x
		} else {
			fail("data: class1.mult[%d]: a unique digit and a multiplier other than 0 are required", i)
		}
	}
	for i := range d.Class1.Tol {
		x := &d.Class1.Tol[i]
		if oneChar("class1.tol", x.Letter, true) && d.tol[x.Letter] == nil && x.PPM > 0 {
			d.tol[x.Letter] = x
		} else {
			fail("data: class1.tol[%d]: a unique letter and a positive tolerance are required", i)
		}
	}
	for i := range d.Class2.Low {
		x := &d.Class2.Low[i]
		if oneChar("class2.low", x.Letter, true) && d.low[x.Letter] == nil {
			d.low[x.Letter] = x
		} else {
			fail("data: class2.low[%d]: a unique letter is required", i)
		}
	}
	for i := range d.Class2.High {
		x := &d.Class2.High[i]
		if oneChar("class2.high", x.Digit, false) && d.high[x.Digit] == nil {
			d.high[x.Digit] = x
		} else {
			fail("data: class2.high[%d]: a unique digit is required", i)
		}
	}
	for i := range d.Class2.Change {
		x := &d.Class2.Change[i]
		if oneChar("class2.change", x.Letter, true) && d.change[x.Letter] == nil && x.Plus > 0 && x.Minus > 0 {
			d.change[x.Letter] = x
		} else {
			fail("data: class2.change[%d]: a unique letter and positive deviations are required", i)
		}
	}
	if len(d.sig) == 0 || len(d.mult) == 0 || len(d.tol) == 0 || len(d.low) == 0 || len(d.high) == 0 || len(d.change) == 0 {
		fail("data: every table of class1 and class2 must be filled")
	}
	for _, a := range d.Aliases {
		if a.Name == "" || len(a.Code) != 3 {
			fail("data: aliases: a name and a 3-character code are required")
		}
	}
	for _, x := range d.Industry {
		if len(x.Code) != 3 || x.Name == "" {
			fail("data: industry: a 3-character code and a name are required")
		}
		d.industry[x.Code] = x.Name
	}
	for _, c := range d.Common {
		if len(c) != 3 {
			fail("data: common: %q is not a 3-character code", c)
		}
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeCeramicDoc struct {
	Class1     codeCeramicClass1     `json:"class1"`
	Class2     codeCeramicClass2     `json:"class2"`
	Aliases    []codeCeramicAlias    `json:"aliases"`
	Industry   []codeCeramicIndustry `json:"industry"`
	NotChecked codeCeramicNotChecked `json:"not_cross_checked"`
	Common     []string              `json:"common"`
	Notes      []string              `json:"notes"`
}

func codeCeramicTempJSON(data any, lang string) any {
	d := data.(*codeCeramicData)
	return codeCeramicDoc{
		Class1:  codeCeramicClass1{Sig: orEmpty(d.Class1.Sig), Mult: orEmpty(d.Class1.Mult), Tol: orEmpty(d.Class1.Tol)},
		Class2:  codeCeramicClass2{Low: orEmpty(d.Class2.Low), High: orEmpty(d.Class2.High), Change: orEmpty(d.Class2.Change)},
		Aliases: orEmpty(d.Aliases), Industry: orEmpty(d.Industry),
		NotChecked: codeCeramicNotChecked{C1Sig: orEmpty(d.NotChecked.C1Sig), C1Mult: orEmpty(d.NotChecked.C1Mult), C2High: orEmpty(d.NotChecked.C2High), C2Change: orEmpty(d.NotChecked.C2Change)},
		Common:     orEmpty(d.Common), Notes: codeLocalizedNotes(d.Notes, lang),
	}
}

// codeListed reports whether s is one of the symbols.
func codeListed(symbols []string, s string) bool { return contains(symbols, s) }

// codeNotCheckedErr is the error for a symbol that only one source lists.
func codeNotCheckedErr(what, symbol string) error {
	return codeErr(CodeUnsupported, "%s %s is not cross-checked (found in one source only)", what, symbol)
}

func (d *codeCeramicData) decode(u *Units, code string) (CodeResult, error) {
	s := strings.NewReplacer("-", "", "_", "").Replace(strings.ToUpper(strings.Join(strings.Fields(code), "")))
	for _, a := range d.Aliases {
		if s == a.Name {
			s = a.Code
		}
	}
	r := []rune(s)
	if len(r) == 3 && r[1] == 'O' && !strings.ContainsRune("XYZ", r[0]) {
		r[1] = '0' // COG for C0G
	}
	if len(r) != 3 {
		return CodeResult{}, codeErr(CodeInvalidCode, "a ceramic temperature code has 3 characters (for example X7R or C0G)")
	}
	a, b, c := string(r[0]), string(r[1]), string(r[2])
	canon := a + b + c
	res := codeNewResult("ceramic_temp")
	res.Code = canon
	if strings.Contains("XYZ", a) {
		low, high, change := d.low[a], d.high[b], d.change[c]
		if high == nil {
			if codeListed(d.NotChecked.C2High, b) {
				return CodeResult{}, codeNotCheckedErr("the upper temperature digit", b)
			}
			return CodeResult{}, codeErr(CodeInvalidCode, "unknown upper temperature digit %s", b)
		}
		if change == nil {
			if codeListed(d.NotChecked.C2Change, c) {
				return CodeResult{}, codeNotCheckedErr("the capacitance change letter", c)
			}
			return CodeResult{}, codeErr(CodeInvalidCode, "unknown capacitance change letter %s", c)
		}
		res.field("class", "Class", "2", math.NaN(), "")
		res.field("temp_low", "Lower category temperature", codeSignedText(low.C, 6)+" °C", roundSig(low.C+273.15, 12), "temperature")
		res.field("temp_high", "Upper category temperature", codeSignedText(high.C, 6)+" °C", roundSig(high.C+273.15, 12), "temperature")
		if change.Plus == change.Minus {
			res.field("change", "Capacitance change", "±"+fmtSig(change.Plus, 6)+" %", codeScaleDec(change.Plus, -2), "ratio")
		} else {
			res.field("change", "Capacitance change", "+"+fmtSig(change.Plus, 6)+" % / −"+fmtSig(change.Minus, 6)+" %", math.NaN(), "ratio")
			res.field("change_plus", "Upper capacitance change", "+"+fmtSig(change.Plus, 6)+" %", codeScaleDec(change.Plus, -2), "ratio")
			res.field("change_minus", "Lower capacitance change", "−"+fmtSig(change.Minus, 6)+" %", codeScaleDec(-change.Minus, -2), "ratio")
		}
		return res, nil
	}
	sig, mult, tol := d.sig[a], d.mult[b], d.tol[c]
	if sig == nil {
		if codeListed(d.NotChecked.C1Sig, a) {
			return CodeResult{}, codeNotCheckedErr("the temperature coefficient letter", a)
		}
		return CodeResult{}, codeErr(CodeInvalidCode, "unknown temperature coefficient letter %s", a)
	}
	if mult == nil {
		if codeListed(d.NotChecked.C1Mult, b) {
			return CodeResult{}, codeNotCheckedErr("the multiplier digit", b)
		}
		return CodeResult{}, codeErr(CodeInvalidCode, "unknown multiplier digit %s", b)
	}
	if tol == nil {
		return CodeResult{}, codeErr(CodeInvalidCode, "unknown tolerance letter %s", c)
	}
	tc := sig.Value * mult.Value
	if tc == 0 {
		tc = 0 // no negative zero
	}
	res.Value, res.Dim = codeScaleDec(tc, -6), "tempco"
	res.field("class", "Class", "1", math.NaN(), "")
	res.field("tc", "Temperature coefficient", codeSignedText(tc, 6)+" ppm/K", codeScaleDec(tc, -6), "tempco")
	res.field("tolerance", "Tolerance", "±"+fmtSig(tol.PPM, 6)+" ppm/K", codeScaleDec(tol.PPM, -6), "tempco")
	if name := d.industry[canon]; name != "" {
		res.field("industry_code", "Industry code", name, math.NaN(), "")
	}
	return res, nil
}

var codeRangeRe = regexp.MustCompile(`(?i)^(.+?)\s*(?:\.\.|…|\bto\b)\s*(.+)$`)

// codeCelsius reads a temperature typed in °C (a bare number is in °C) and returns it in °C.
func codeCelsius(u *Units, s string) (float64, error) {
	k, err := u.Parse(strings.ReplaceAll(s, "−", "-"), "temperature", "degC")
	return k - 273.15, err
}

func (d *codeCeramicData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	class := strings.TrimSpace(req.Class)
	if class == "" {
		switch {
		case codeRangeRe.MatchString(req.Value):
			class = "2"
		case strings.TrimSpace(req.TempCo) != "" || strings.TrimSpace(req.Value) != "":
			class = "1"
		default:
			return CodeResult{}, codeErr(CodeInvalidValue, "give the class (1 or 2)")
		}
	}
	var code string
	switch class {
	case "1":
		src := req.TempCo
		if strings.TrimSpace(src) == "" {
			src = req.Value
		}
		if strings.TrimSpace(src) == "" || strings.TrimSpace(req.Tolerance) == "" {
			return CodeResult{}, codeErr(CodeInvalidValue, "class 1 needs the temperature coefficient (tempco, for example -750ppm) and the tolerance (for example 120ppm)")
		}
		tcSI, err := codeParseTempco(u, src)
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "tempco: %v", err)
		}
		tolSI, err := codeParseTempco(u, req.Tolerance)
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "tolerance: %v", err)
		}
		tc, tolPPM := tcSI/1e-6, math.Abs(tolSI)/1e-6
		var sigL, multD, tolL string
		for _, x := range d.Class1.Sig {
			for _, m := range d.Class1.Mult {
				if p := x.Value * m.Value; sigL == "" && ((tc == 0 && p == 0) || (tc != 0 && p != 0 && codeCloseTo(p, tc))) {
					sigL, multD = x.Letter, m.Digit
				}
			}
		}
		if sigL == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "no class 1 code for a temperature coefficient of %s ppm/K", fmtSig(tc, 6))
		}
		for _, x := range d.Class1.Tol {
			if tolL == "" && codeCloseTo(x.PPM, tolPPM) {
				tolL = x.Letter
			}
		}
		if tolL == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "no class 1 tolerance letter for ±%s ppm/K", fmtSig(tolPPM, 6))
		}
		code = sigL + multD + tolL
	case "2":
		m := codeRangeRe.FindStringSubmatch(req.Value)
		if m == nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "class 2 needs the category temperature range in value, for example -55..125")
		}
		lo, err := codeCelsius(u, m[1])
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
		}
		hi, err := codeCelsius(u, m[2])
		if err != nil {
			return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
		}
		var lowL, highD, changeL string
		for _, x := range d.Class2.Low {
			if lowL == "" && math.Abs(x.C-lo) < 1e-6 {
				lowL = x.Letter
			}
		}
		if lowL == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "no class 2 letter for a lower category temperature of %s °C", fmtSig(lo, 6))
		}
		for _, x := range d.Class2.High {
			if highD == "" && math.Abs(x.C-hi) < 1e-6 {
				highD = x.Digit
			}
		}
		if highD == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "no class 2 digit for an upper category temperature of %s °C", fmtSig(hi, 6))
		}
		if strings.TrimSpace(req.Tolerance) == "" {
			return CodeResult{}, codeErr(CodeInvalidValue, "class 2 needs the capacitance change in tolerance, for example 15%% or +22/-82")
		}
		plus, minus := 0.0, 0.0
		if p, mi, ok := codeParseAsym(req.Tolerance); ok {
			plus, minus = p, mi
		} else if p, ok := codeParsePercent(req.Tolerance); ok {
			plus, minus = p, p
		} else {
			return CodeResult{}, codeErr(CodeInvalidValue, "tolerance: cannot read %q as a percentage or as an asymmetric change such as +22/-82", req.Tolerance)
		}
		for _, x := range d.Class2.Change {
			if changeL == "" && codeCloseTo(x.Plus, plus) && codeCloseTo(x.Minus, minus) {
				changeL = x.Letter
			}
		}
		if changeL == "" {
			return CodeResult{}, codeErr(CodeUnsupported, "no class 2 letter for a capacitance change of %s", fmt.Sprintf("+%s %% / −%s %%", fmtSig(plus, 6), fmtSig(minus, 6)))
		}
		code = lowL + highD + changeL
	default:
		return CodeResult{}, codeErr(CodeInvalidValue, "class must be 1 or 2")
	}
	return d.decode(u, code)
}
