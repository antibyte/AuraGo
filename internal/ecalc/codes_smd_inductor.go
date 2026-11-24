package ecalc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The smd_inductor kind: the value marking of SMD inductors, three characters in one of three notations: three digits (two
// significant digits and the number of zeros, in µH: 101 = 100 µH), R as the decimal point in µH (4R7, R47) and N as the
// decimal point in nH (4N7, 47N). A fourth character is a tolerance letter. The value rules agree across makers; the tolerance
// letters do not (D is ±0.5 nH at Murata and ±0.3 nH at Bourns), so only the letters that two makers state identically are
// decoded and every other letter is listed in not_cross_checked.

type codeSMDITolerance struct {
	Letter  string  `yaml:"letter" json:"letter"`
	Percent float64 `yaml:"percent" json:"percent"`
	NH      float64 `yaml:"nh" json:"nh"`
}

type codeSMDINotChecked struct {
	Tolerance []string `yaml:"tolerance" json:"tolerance"`
}

type codeSMDInductorData struct {
	Tolerances []codeSMDITolerance `yaml:"tolerances"`
	NotChecked codeSMDINotChecked  `yaml:"not_cross_checked"`
	Notes      []Text              `yaml:"notes"`

	tol map[string]*codeSMDITolerance
	not map[string]bool
}

func codeLoadSMDInductor(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeSMDInductorData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.tol, d.not = map[string]*codeSMDITolerance{}, map[string]bool{}
	for i := range d.Tolerances {
		t := &d.Tolerances[i]
		if len(t.Letter) != 1 || t.Letter < "A" || t.Letter > "Z" || d.tol[t.Letter] != nil {
			fail("data: tolerances[%d]: a unique capital letter is required", i)
			continue
		}
		if (t.Percent > 0) == (t.NH > 0) || t.Percent < 0 || t.NH < 0 {
			fail("data: tolerances[%d]: exactly one of percent and nh, a positive number, is required", i)
			continue
		}
		d.tol[t.Letter] = t
	}
	if len(d.tol) == 0 {
		fail("data: tolerances: at least one tolerance letter is required")
	}
	for i, l := range d.NotChecked.Tolerance {
		if len(l) != 1 || l < "A" || l > "Z" || d.tol[l] != nil || d.not[l] {
			fail("data: not_cross_checked.tolerance[%d]: a capital letter that is not decoded, once, is required", i)
			continue
		}
		d.not[l] = true
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeSMDInductorDoc struct {
	Tolerances []codeSMDITolerance `json:"tolerances"`
	NotChecked codeSMDINotChecked  `json:"not_cross_checked"`
	Notes      []string            `json:"notes"`
}

func codeSMDInductorJSON(data any, lang string) any {
	d := data.(*codeSMDInductorData)
	return codeSMDInductorDoc{Tolerances: orEmpty(d.Tolerances), NotChecked: codeSMDINotChecked{Tolerance: orEmpty(d.NotChecked.Tolerance)}, Notes: codeLocalizedNotes(d.Notes, lang)}
}

// A value code of three characters (three digits, or two digits and one R or N) and an optional tolerance letter.
var (
	codeSMDIRe       = regexp.MustCompile(`^(\d\d\d|\d\d[RN]|\d[RN]\d|[RN]\d\d)([A-Z]?)$`)
	codeSMDILetterRe = regexp.MustCompile(`^[A-Za-z]$`)
)

func (d *codeSMDInductorData) toleranceText(t *codeSMDITolerance) string {
	if t.NH > 0 {
		return "±" + fmtSig(t.NH, 6) + " nH"
	}
	return "±" + fmtSig(t.Percent, 6) + " %"
}

func (d *codeSMDInductorData) addTolerance(res *CodeResult, t *codeSMDITolerance) {
	if t.NH > 0 {
		res.field("tolerance", "Tolerance", d.toleranceText(t), codeScaleDec(t.NH, -9), "inductance")
		return
	}
	codeToleranceField(res, t.Percent)
}

// letter finds a tolerance letter; a letter that is not cross-checked or not known is an error.
func (d *codeSMDInductorData) letter(l string) (*codeSMDITolerance, error) {
	if t := d.tol[l]; t != nil {
		return t, nil
	}
	if d.not[l] {
		return nil, codeErr(CodeUnsupported, "the tolerance letter %s is not cross-checked (makers disagree, or only one maker states it)", l)
	}
	return nil, codeErr(CodeInvalidCode, "unknown tolerance letter %s", l)
}

func (d *codeSMDInductorData) decode(u *Units, code string) (CodeResult, error) {
	s := codeNormalizeSMD(code)
	if s == "" {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is empty")
	}
	m := codeSMDIRe.FindStringSubmatch(s)
	if m == nil {
		return CodeResult{}, codeErr(CodeInvalidCode, "cannot read %q as an SMD inductor code (3 digits, or two digits with R or N as the decimal point, and an optional tolerance letter)", s)
	}
	v, family := 0.0, ""
	switch c := m[1]; {
	case strings.Contains(c, "R"):
		i := strings.IndexByte(c, 'R')
		v, family = codeDecimalText(c[:i], c[i+1:], -6), "R notation (µH)"
	case strings.Contains(c, "N"):
		i := strings.IndexByte(c, 'N')
		v, family = codeDecimalText(c[:i], c[i+1:], -9), "N notation (nH)"
	default:
		digits, _ := strconv.Atoi(c[:2])
		exp, _ := strconv.Atoi(c[2:])
		v, family = codeDecimal(digits, exp-6), "3-digit code (µH)"
	}
	if v == 0 {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code gives an inductance of zero")
	}
	var tol *codeSMDITolerance
	if m[2] != "" {
		var err error
		if tol, err = d.letter(m[2]); err != nil {
			return CodeResult{}, err
		}
	}
	res := codeNewResult("smd_inductor")
	res.Dim = "inductance"
	res.Code = s
	res.Value = v
	res.field("family", "Marking", family, math.NaN(), "")
	if tol != nil {
		d.addTolerance(&res, tol)
	}
	return res, nil
}

// codeSMDInductorValue writes v (> 0, in H) as a 3-character code: "" and a reason when it has none.
func codeSMDInductorValue(u *Units, v float64) (code, family string, err error) {
	N, e, exact := codeSigDigits(v, 2)
	if !exact {
		return "", "", codeErr(CodeUnsupported, "%s needs more than 2 significant digits", u.Format(v, "inductance", "", 4))
	}
	switch eu := e + 6; { // the exponent in µH
	case eu >= 0 && eu <= 9:
		return fmt.Sprintf("%d%d", N, eu), "3-digit code (µH)", nil
	case eu == -1:
		return fmt.Sprintf("%dR%d", N/10, N%10), "R notation (µH)", nil
	case eu == -2:
		return fmt.Sprintf("R%d", N), "R notation (µH)", nil
	case eu == -3:
		return fmt.Sprintf("%dN", N), "N notation (nH)", nil
	case eu == -4:
		return fmt.Sprintf("%dN%d", N/10, N%10), "N notation (nH)", nil
	case eu == -5 && N%10 == 0:
		return fmt.Sprintf("0N%d", N/10), "N notation (nH)", nil
	case eu > 9:
		return "", "", codeErr(CodeUnsupported, "%s is outside the range of the code (up to 99 × 10⁹ µH)", u.Format(v, "inductance", "", 4))
	}
	return "", "", codeErr(CodeUnsupported, "%s has no 3-character marking (below 0.1 µH the N notation holds 0.3 nH, 1 nH to 9.9 nH in steps of 0.1 and 10 nH to 99 nH)", u.Format(v, "inductance", "", 4))
}

// tolerancePick finds the letter an encode request means: a letter or a percentage.
func (d *codeSMDInductorData) tolerancePick(text string) (*codeSMDITolerance, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if codeSMDILetterRe.MatchString(text) {
		l := strings.ToUpper(text)
		if t := d.tol[l]; t != nil {
			return t, nil
		}
		if d.not[l] {
			return nil, codeErr(CodeUnsupported, "the tolerance letter %s is not cross-checked (makers disagree, or only one maker states it)", l)
		}
		return nil, codeErr(CodeUnsupported, "no tolerance letter %s", l)
	}
	p, ok := codeParsePercent(text)
	if !ok || p <= 0 {
		return nil, codeErr(CodeInvalidValue, "tolerance: cannot read %q as a letter or a percentage", text)
	}
	for i := range d.Tolerances {
		if t := &d.Tolerances[i]; t.Percent > 0 && codeCloseTo(t.Percent, p) {
			return t, nil
		}
	}
	return nil, codeErr(CodeUnsupported, "no tolerance letter for ±%s %% (cross-checked letters only)", fmtSig(p, 6))
}

func (d *codeSMDInductorData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	if strings.TrimSpace(req.Value) == "" {
		return CodeResult{}, codeErr(CodeInvalidValue, "give a value")
	}
	v, err := u.Parse(req.Value, "inductance", "")
	if err != nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
	}
	if v <= 0 {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: the inductance must be positive")
	}
	tol, err := d.tolerancePick(req.Tolerance)
	if err != nil {
		return CodeResult{}, err
	}
	code, family, err := codeSMDInductorValue(u, v)
	if err != nil {
		return CodeResult{}, err
	}
	N, e, _ := codeSigDigits(v, 2)
	res := codeNewResult("smd_inductor")
	res.Dim = "inductance"
	res.Code = code
	res.Value = codeDecimal(N, e)
	res.field("family", "Marking", family, math.NaN(), "")
	if tol != nil {
		res.Code += tol.Letter
		d.addTolerance(&res, tol)
	}
	return res, nil
}
