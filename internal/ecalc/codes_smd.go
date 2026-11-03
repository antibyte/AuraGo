package ecalc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The smd_resistor kind: the markings of chip resistors. Four families are read: the 3-digit code (two significant
// digits and an exponent), the 4-digit code (three digits and an exponent), the RKM / R notation (a letter is the decimal
// point and the multiplier: 4R7, R47, 1L5, 4K7) and the EIA-96 code (two digits indexing the E96 series plus a multiplier
// letter). The E96 table, the letters and the RKM letters are the data of the code item.

type codeSMDLetter struct {
	Letter string `yaml:"letter" json:"letter"`
	Exp    int    `yaml:"exp" json:"exp"`
	Rare   bool   `yaml:"rare" json:"rare"`
}

type codeSMDAmbiguous struct {
	Letter string `yaml:"letter" json:"letter"`
}

type codeSMDData struct {
	EIA96     []int              `yaml:"eia96"`
	Letters   []codeSMDLetter    `yaml:"letters"`
	RKM       []codeSMDLetter    `yaml:"rkm"`
	Ambiguous []codeSMDAmbiguous `yaml:"ambiguous"`
	Notes     []Text             `yaml:"notes"`

	letter map[string]*codeSMDLetter
	rkm    map[string]*codeSMDLetter
	amb    map[string]bool
}

func codeLoadSMDResistor(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeSMDData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.letter, d.rkm, d.amb = map[string]*codeSMDLetter{}, map[string]*codeSMDLetter{}, map[string]bool{}
	if len(d.EIA96) != 96 {
		fail("data: eia96 must list the 96 values of the E96 series, got %d", len(d.EIA96))
	} else {
		for i, v := range d.EIA96 {
			if want := int(math.Round(e96[i] * 100)); v != want {
				fail("data: eia96[%d] = %d, the E96 value is %d (IEC 60063)", i, v, want)
			}
		}
	}
	for i := range d.Letters {
		l := &d.Letters[i]
		if len(l.Letter) != 1 || l.Letter < "A" || l.Letter > "Z" || d.letter[l.Letter] != nil || l.Exp < -3 || l.Exp > 6 {
			fail("data: letters[%d]: a unique capital letter and an exponent from -3 to 6 are required", i)
			continue
		}
		d.letter[l.Letter] = l
	}
	for i := range d.RKM {
		l := &d.RKM[i]
		if len(l.Letter) != 1 || !strings.Contains("LRKMG", l.Letter) || d.rkm[l.Letter] != nil {
			fail("data: rkm[%d]: the letters are L, R, K, M and G, each once", i)
			continue
		}
		d.rkm[l.Letter] = l
	}
	if len(d.rkm) != 5 {
		fail("data: rkm must list L, R, K, M and G")
	}
	for i, a := range d.Ambiguous {
		if len(a.Letter) != 1 || d.letter[a.Letter] != nil {
			fail("data: ambiguous[%d]: a letter that is not a decoded letter is required", i)
			continue
		}
		d.amb[a.Letter] = true
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeSMDDoc struct {
	EIA96     []int              `json:"eia96"`
	Letters   []codeSMDLetter    `json:"letters"`
	RKM       []codeSMDLetter    `json:"rkm"`
	Ambiguous []codeSMDAmbiguous `json:"ambiguous"`
	Notes     []string           `json:"notes"`
}

func codeSMDResistorJSON(data any, lang string) any {
	d := data.(*codeSMDData)
	return codeSMDDoc{EIA96: orEmpty(d.EIA96), Letters: orEmpty(d.Letters), RKM: orEmpty(d.RKM), Ambiguous: orEmpty(d.Ambiguous), Notes: codeLocalizedNotes(d.Notes, lang)}
}

var (
	codeSMDEIA96Re = regexp.MustCompile(`^(\d\d)([A-Z])$`)
	codeSMDRKMRe   = regexp.MustCompile(`^(\d*)([LRKMG])(\d*)$`)
	codeSMDDigitRe = regexp.MustCompile(`^\d{3,4}$`)
	codeSMDZeroRe  = regexp.MustCompile(`^(0{1,4}|0R0)$`)
)

// codeNormalizeSMD removes blanks, underscores and bars (some makers underline a 1 % code) and upper-cases the code.
func codeNormalizeSMD(code string) string {
	s := strings.ToUpper(strings.Join(strings.Fields(code), ""))
	return strings.NewReplacer("_", "", "|", "").Replace(s)
}

func (d *codeSMDData) decode(u *Units, code string) (CodeResult, error) {
	s := codeNormalizeSMD(code)
	if s == "" {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is empty")
	}
	res := codeNewResult("smd_resistor")
	res.Dim = "resistance"
	res.Code = s
	family := func(text string) { res.field("family", "Marking", text, math.NaN(), "") }

	if codeSMDZeroRe.MatchString(s) {
		res.Value = 0
		family("zero-ohm jumper")
		return res, nil
	}
	// two digits and a letter: an EIA-96 code, unless the letter is an RKM letter (10R, 47K, 15L read as RKM codes)
	if m := codeSMDEIA96Re.FindStringSubmatch(s); m != nil && d.rkm[m[2]] == nil {
		idx, _ := strconv.Atoi(m[1])
		if d.amb[m[2]] {
			return CodeResult{}, codeErr(CodeUnsupported, "the EIA-96 letter %s is ambiguous between makers (conflicting meanings); it is not decoded", m[2])
		}
		l := d.letter[m[2]]
		if l == nil {
			return CodeResult{}, codeErr(CodeInvalidCode, "%s is not an EIA-96 multiplier letter", m[2])
		}
		if idx < 1 || idx > 96 {
			return CodeResult{}, codeErr(CodeInvalidCode, "the EIA-96 index must be 01 to 96, got %s", m[1])
		}
		res.Value = codeDecimal(d.EIA96[idx-1], l.Exp)
		family("EIA-96")
		if l.Rare {
			res.warn("the letter %s (×%s) is only used by some makers", l.Letter, fmtNum(codeDecimal(1, l.Exp)))
		}
		return res, nil
	}
	if m := codeSMDRKMRe.FindStringSubmatch(s); m != nil && len(m[1])+len(m[3]) > 0 {
		if len(m[1])+len(m[3]) > 4 {
			return CodeResult{}, codeErr(CodeInvalidCode, "an RKM code has at most 4 digits")
		}
		l := d.rkm[m[2]]
		res.Value = codeDecimalText(m[1], m[3], l.Exp)
		if l.Letter == "L" || l.Letter == "R" {
			family("R notation")
		} else {
			family("RKM notation")
		}
		if idx, _ := strconv.Atoi(m[1]); l.Letter == "R" && len(m[1]) == 2 && m[3] == "" && idx >= 1 && idx <= 96 {
			res.warn("R after two digits is read as the RKM decimal point (%s = %s); some makers use R as an EIA-96 multiplier letter, which is not decoded", s, u.Format(res.Value, "resistance", "", 4))
		}
		return res, nil
	}
	if codeSMDDigitRe.MatchString(s) {
		n := 2
		if len(s) == 4 {
			n = 3
		}
		digits, _ := strconv.Atoi(s[:n])
		exp, _ := strconv.Atoi(s[n:])
		res.Value = codeDecimal(digits, exp)
		family(fmt.Sprintf("%d-digit code", len(s)))
		return res, nil
	}
	return CodeResult{}, codeErr(CodeInvalidCode, "cannot read %q as an SMD resistor code (3 or 4 digits, R notation, RKM or EIA-96)", s)
}

// codeSMD3Digit writes v as a 3-character code ("472", "4R7", "R47", "R05", "1L5"); "" when it has no such code.
func codeSMD3Digit(v float64) string {
	N, e, exact := codeSigDigits(v, 2)
	if !exact {
		return ""
	}
	switch {
	case e >= 0 && e <= 9:
		return fmt.Sprintf("%d%d", N, e)
	case e == -1:
		return fmt.Sprintf("%dR%d", N/10, N%10)
	case e == -2:
		return fmt.Sprintf("R%d", N)
	case e == -3 && N%10 == 0:
		return fmt.Sprintf("R0%d", N/10)
	case e == -4:
		return fmt.Sprintf("%dL%d", N/10, N%10)
	}
	return ""
}

// codeSMD4Digit writes v as a 4-character code ("1001", "10R0", "4R99", "R499", "R020"); "" when it has no such code.
func codeSMD4Digit(v float64) string {
	N, e, exact := codeSigDigits(v, 3)
	if !exact {
		return ""
	}
	switch {
	case e >= 0 && e <= 9:
		return fmt.Sprintf("%d%d", N, e)
	case e == -1:
		return fmt.Sprintf("%dR%d", N/10, N%10)
	case e == -2:
		return fmt.Sprintf("%dR%02d", N/100, N%100)
	case e == -3:
		return fmt.Sprintf("R%03d", N)
	case e == -4 && N%10 == 0:
		return fmt.Sprintf("R%03d", N/10)
	}
	return ""
}

// code96 writes v as an EIA-96 code ("01C"); "" when v is not an E96 value from 1.00 Ω to 97.6 MΩ.
func (d *codeSMDData) code96(v float64) string {
	N, e, exact := codeSigDigits(v, 3)
	if !exact {
		return ""
	}
	for i, x := range d.EIA96 {
		if x != N {
			continue
		}
		for _, l := range d.Letters {
			if l.Exp == e && !l.Rare {
				return fmt.Sprintf("%02d%s", i+1, l.Letter)
			}
		}
	}
	return ""
}

func (d *codeSMDData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	v, err := u.Parse(req.Value, "resistance", "")
	if err != nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: %v", err)
	}
	if v < 0 {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: the resistance must not be negative")
	}
	res := codeNewResult("smd_resistor")
	res.Dim = "resistance"
	if v == 0 {
		res.Code, res.Value = "000", 0
		res.field("family", "Marking", "zero-ohm jumper", math.NaN(), "")
		res.field("code_4digit", "4-digit code", "0000", math.NaN(), "")
		return res, nil
	}
	tol, assumed, err := codeRequestedTolerance(req, v)
	if err != nil {
		return CodeResult{}, err
	}
	vs, changed, err := codeSnapToSeries(v, tol)
	if err != nil {
		return CodeResult{}, err
	}
	c3, c4, c96 := codeSMD3Digit(vs), codeSMD4Digit(vs), d.code96(vs)
	primary, family := c3, "3-digit code"
	if (tol <= 1 && c4 != "") || c3 == "" {
		primary, family = c4, "4-digit code"
	}
	if primary == "" {
		return CodeResult{}, codeErr(CodeUnsupported, "%s has no 3-digit or 4-digit marking", u.Format(vs, "resistance", "", 4))
	}
	N, e, _ := codeSigDigits(vs, 3)
	res.Code = primary
	res.Value = codeDecimal(N, e)
	res.field("family", "Marking", family, math.NaN(), "")
	if c3 != "" {
		res.field("code_3digit", "3-digit code", c3, math.NaN(), "")
	}
	if c4 != "" {
		res.field("code_4digit", "4-digit code", c4, math.NaN(), "")
	}
	if c96 != "" {
		res.field("code_eia96", "EIA-96 code", c96, math.NaN(), "")
	}
	codeToleranceField(&res, tol)
	if assumed {
		res.warn("no tolerance given; ±%s %% assumed", fmtSig(tol, 6))
	}
	if changed {
		res.warn("%s", codeSnapWarning(u, "resistance", v, vs, tol))
	}
	return res, nil
}
