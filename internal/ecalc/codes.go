package ecalc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Component codes: reading and writing the markings of resistors, inductors and capacitors. The five code kinds
// (resistor_bands, inductor_bands, smd_resistor, capacitor, ceramic_temp) are registered here and in codes_*.go; their
// tables (colours, letters, voltage codes ...) are the data of the code items in catalog/codes/*.yaml. This file holds what
// the kinds share: the result types, the errors, Decode and Encode, and the helpers for digits, tolerances and the
// nearest standard value. The JavaScript twin is ui/js/desktop/apps/ecalc/ecalc-codes.js; both run the vectors of
// testdata/vectors/codes.json.

// Error codes of CodeError (the prefix of its message).
const (
	CodeInvalidCode  = "invalid_code"  // the code to decode is malformed
	CodeInvalidValue = "invalid_value" // a field of an encode request cannot be read, or the fields contradict each other
	CodeUnsupported  = "unsupported"   // well formed, but not decodable (not cross-checked, ambiguous) or not codable (outside the scheme)
)

// CodeError is a problem with a decode or encode request; its message starts with the code ("invalid_code: ...").
type CodeError struct {
	Code    string
	Message string
}

func (e *CodeError) Error() string { return e.Code + ": " + e.Message }

func codeErr(code, format string, args ...any) error {
	return &CodeError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// AsCodeError returns err as a *CodeError, or nil when it is none.
func AsCodeError(err error) *CodeError {
	var ce *CodeError
	if errors.As(err, &ce) {
		return ce
	}
	return nil
}

// CodeResult is what Decode and Encode return. Bands holds colour ids for the band kinds, Value is in SI units (Ω, H, F, V,
// 1/K) or NaN when the kind has none, and Fields carry the further decoded parts. Warnings are English.
type CodeResult struct {
	Kind     string      `json:"kind"`
	Code     string      `json:"code"`
	Bands    []string    `json:"bands"`
	Value    float64     `json:"value"`
	Dim      string      `json:"dim"`
	Fields   []CodeField `json:"fields"`
	Warnings []string    `json:"warnings"`
}

// CodeField is one decoded part of a code (tolerance, temperature coefficient, voltage ...). Name is English; the ids are
// stable and are what the pages translate. Text is language neutral ("±5 %", "-55 °C"). Value is in SI units or NaN.
type CodeField struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	Value float64 `json:"value"`
	Dim   string  `json:"dim"`
}

// MarshalJSON writes a NaN value as null.
func (f CodeField) MarshalJSON() ([]byte, error) {
	type plain struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Text  string   `json:"text"`
		Value *float64 `json:"value"`
		Dim   string   `json:"dim"`
	}
	p := plain{ID: f.ID, Name: f.Name, Text: f.Text, Dim: f.Dim}
	if !math.IsNaN(f.Value) {
		v := f.Value
		p.Value = &v
	}
	return json.Marshal(p)
}

// MarshalJSON writes a NaN value as null and the lists as arrays, never null.
func (r CodeResult) MarshalJSON() ([]byte, error) {
	type plain struct {
		Kind     string      `json:"kind"`
		Code     string      `json:"code"`
		Bands    []string    `json:"bands"`
		Value    *float64    `json:"value"`
		Dim      string      `json:"dim"`
		Fields   []CodeField `json:"fields"`
		Warnings []string    `json:"warnings"`
	}
	p := plain{Kind: r.Kind, Code: r.Code, Bands: orEmpty(r.Bands), Dim: r.Dim, Fields: r.Fields, Warnings: orEmpty(r.Warnings)}
	if p.Fields == nil {
		p.Fields = []CodeField{}
	}
	if !math.IsNaN(r.Value) {
		v := r.Value
		p.Value = &v
	}
	return json.Marshal(p)
}

// EncodeRequest asks for the code of a value. Value is a quantity as typed in a field ("4.7k", "100n"); Bands is 0 (choose),
// 3, 4, 5 or 6 for the resistor and 4 (EIA) or 5 (MIL) for the inductor; Tolerance is a percentage ("5%"), for some kinds a
// letter or a quantity; TempCo a temperature coefficient ("50ppm"); Voltage a rated voltage ("50V"); Class the ceramic
// class ("1" or "2"). Fields a kind does not use are ignored.
type EncodeRequest struct {
	Value     string `json:"value"`
	Bands     int    `json:"bands"`
	Tolerance string `json:"tolerance"`
	TempCo    string `json:"tempco"`
	Voltage   string `json:"voltage"`
	Class     string `json:"class"`
}

// codeCodec is what the typed data of a code kind provides.
type codeCodec interface {
	decode(u *Units, code string) (CodeResult, error)
	encode(u *Units, req EncodeRequest) (CodeResult, error)
}

// codeMaxText is the longest code or request field Decode and Encode accept (UTF-16 code units, like Parse).
const codeMaxText = 64

func init() {
	RegisterCodeKind("resistor_bands", CodeKind{Load: codeLoadResistorBands, JSON: codeResistorBandsJSON})
	RegisterCodeKind("inductor_bands", CodeKind{Load: codeLoadInductorBands, JSON: codeInductorBandsJSON})
	RegisterCodeKind("smd_resistor", CodeKind{Load: codeLoadSMDResistor, JSON: codeSMDResistorJSON})
	RegisterCodeKind("capacitor", CodeKind{Load: codeLoadCapacitor, JSON: codeCapacitorJSON})
	RegisterCodeKind("ceramic_temp", CodeKind{Load: codeLoadCeramicTemp, JSON: codeCeramicTempJSON})
}

// codec finds the first code item of a kind and its codec.
func (c *Catalog) codec(kind string) (codeCodec, error) {
	for _, it := range c.Codes {
		if it.CodeKind != kind {
			continue
		}
		if cc, ok := it.Data.(codeCodec); ok {
			return cc, nil
		}
		break
	}
	return nil, codeErr(CodeUnsupported, "unknown code kind %q (kinds: %s)", kind, strings.Join(CodeKindIDs(), ", "))
}

// Decode reads a code of one of the registered kinds: band colours as English ids ("brown black red gold"), SMD and
// capacitor markings as printed ("472", "4R7", "01C", "104K 1H", "4p7"), ceramic codes ("X7R", "C0G"). Errors are
// *CodeError with the codes invalid_code and unsupported.
func (c *Catalog) Decode(kind, code string) (CodeResult, error) {
	cc, err := c.codec(kind)
	if err != nil {
		return CodeResult{}, err
	}
	if textLength(code) > codeMaxText {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is longer than %d characters", codeMaxText)
	}
	return cc.decode(c.Units, code)
}

// Encode writes the code of a value. The value is replaced by the nearest standard value of the series that the tolerance
// implies (20 % E6, 10 % E12, 5 % E24, 2 % E48, 1 % E96, 0.5 % and finer E192; E24 values also count as standard from
// 5 % down) and a warning says so; without a tolerance one is assumed from the value. Errors are *CodeError with the
// codes invalid_value and unsupported.
func (c *Catalog) Encode(kind string, req EncodeRequest) (CodeResult, error) {
	cc, err := c.codec(kind)
	if err != nil {
		return CodeResult{}, err
	}
	for _, s := range []string{req.Value, req.Tolerance, req.TempCo, req.Voltage, req.Class} {
		if textLength(s) > codeMaxText {
			return CodeResult{}, codeErr(CodeInvalidValue, "a request field is longer than %d characters", codeMaxText)
		}
	}
	if req.Bands < 0 || req.Bands > 6 {
		return CodeResult{}, codeErr(CodeInvalidValue, "bands must be 0 (choose), 3, 4, 5 or 6")
	}
	return cc.encode(c.Units, req)
}

// ---- result building --------------------------------------------------------------------------------------------------

func codeNewResult(kind string) CodeResult {
	return CodeResult{Kind: kind, Bands: []string{}, Value: math.NaN(), Fields: []CodeField{}, Warnings: []string{}}
}

func (r *CodeResult) field(id, name, text string, value float64, dim string) {
	r.Fields = append(r.Fields, CodeField{ID: id, Name: name, Text: text, Value: value, Dim: dim})
}

func (r *CodeResult) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// ---- numbers -----------------------------------------------------------------------------------------------------------

// codeDecimal returns n*10^exp as the double nearest to that decimal (the same in JavaScript: Number(n + "e" + exp)).
func codeDecimal(n, exp int) float64 {
	v, _ := strconv.ParseFloat(strconv.Itoa(n)+"e"+strconv.Itoa(exp), 64)
	return v
}

// codeDecimalText returns the number "whole.frac" times 10^exp for digit strings (whole may be empty).
func codeDecimalText(whole, frac string, exp int) float64 {
	if whole == "" {
		whole = "0"
	}
	if frac != "" {
		whole += "." + frac
	}
	v, _ := strconv.ParseFloat(whole+"e"+strconv.Itoa(exp), 64)
	return v
}

// codeScaleDec returns x*10^exp as the double nearest to that decimal (x is a short decimal such as 0.25 or 50; the same in
// JavaScript: Number(String(x) + "e" + exp)). A magnitude outside 1e-6 .. 1e21, where String(x) uses an exponent, is
// multiplied instead.
func codeScaleDec(x float64, exp int) float64 {
	if a := math.Abs(x); a == 0 || a < 1e-6 || a >= 1e21 {
		return x * math.Pow10(exp)
	}
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', -1, 64)+"e"+strconv.Itoa(exp), 64)
	return v
}

// codeSigDigits writes v (> 0) with n significant digits: the integer N of exactly n digits and the exponent e with v = N*10^e,
// and whether v equals that number to within 1e-9 relative (so that it is not a rounded value).
func codeSigDigits(v float64, n int) (N, e int, exact bool) {
	s := strconv.FormatFloat(v, 'e', n-1, 64)
	i := strings.IndexByte(s, 'e')
	N, _ = strconv.Atoi(strings.Replace(s[:i], ".", "", 1))
	e, _ = strconv.Atoi(s[i+1:])
	e -= n - 1
	return N, e, math.Abs(codeDecimal(N, e)-v) <= 1e-9*v
}

// codeSeriesHas reports whether v, written with 3 significant digits, is a value of the E-series (any decade).
func codeSeriesHas(series string, v float64) bool {
	n, _, exact := codeSigDigits(v, 3)
	if !exact {
		return false
	}
	for _, m := range eseriesTables[series] {
		if int(math.Round(m*100)) == n {
			return true
		}
	}
	return false
}

// codeSeriesForTolerance is the E-series that a tolerance (percent) implies.
func codeSeriesForTolerance(tol float64) string {
	switch {
	case tol >= 20:
		return "E6"
	case tol >= 10:
		return "E12"
	case tol >= 5:
		return "E24"
	case tol >= 2:
		return "E48"
	case tol >= 1:
		return "E96"
	}
	return "E192"
}

// codeAssumeTolerance picks a tolerance for a value without one: 5 % for an E24 value, 1 % for an E96 value, 0.5 % for an
// E192 value, otherwise 5 % (the value is then replaced by an E24 value).
func codeAssumeTolerance(v float64) float64 {
	switch {
	case codeSeriesHas("E24", v):
		return 5
	case codeSeriesHas("E96", v):
		return 1
	case codeSeriesHas("E192", v):
		return 0.5
	}
	return 5
}

// codeSnapToSeries returns the standard value for tolerance tol (percent) that is nearest to v: a value of the series that tol
// implies or, for tolerances below 5 %, of E24 (47 kΩ is a normal 1 % value). A tie goes to the lower value. changed says
// that the value is not v (within 1e-9 relative).
func codeSnapToSeries(v, tol float64) (value float64, changed bool, err error) {
	best, _, err := Nearest(codeSeriesForTolerance(tol), v)
	if err != nil {
		return 0, false, codeErr(CodeUnsupported, "the value is outside the range of the code")
	}
	if tol < 5 {
		if alt, _, err := Nearest("E24", v); err == nil {
			da, db := math.Abs(alt-v), math.Abs(best-v)
			if da < db || (da == db && alt < best) {
				best = alt
			}
		}
	}
	return best, math.Abs(best-v) > 1e-9*v, nil
}

// codeSnapWarning is the warning that a value was replaced by the nearest standard value.
func codeSnapWarning(u *Units, dim string, in, out, tol float64) string {
	label := codeSeriesForTolerance(tol)
	if tol < 5 {
		label += " or E24"
	}
	return fmt.Sprintf("%s is not a standard value for ±%s %% (%s); encoded the nearest, %s (%s %%)",
		u.Format(in, dim, "", 4), fmtSig(tol, 6), label, u.Format(out, dim, "", 4), codeSignedText((out-in)/in*100, 2))
}

// codeSignedText writes x with sig significant digits and an explicit sign: "+5", "−0.5" (U+2212), "0".
func codeSignedText(x float64, sig int) string {
	switch {
	case x == 0:
		return "0"
	case x < 0:
		return "−" + fmtSig(-x, sig)
	}
	return "+" + fmtSig(x, sig)
}

// ---- reading request fields --------------------------------------------------------------------------------------------

var (
	codePercentRe = regexp.MustCompile(`^\+?(\d+(?:\.\d+)?|\.\d+)%?$`)
	codeAsymRe    = regexp.MustCompile(`^([+\-−])(\d+(?:\.\d+)?)/([+\-−])(\d+(?:\.\d+)?)%?$`)
)

// codeCompact removes all white space and turns a decimal comma into a point.
func codeCompact(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), ""), ",", ".")
}

// codeParsePercent reads "5", "5%", "±5 %", "0,25%" as a percentage.
func codeParsePercent(s string) (float64, bool) {
	m := codePercentRe.FindStringSubmatch(strings.TrimPrefix(codeCompact(s), "±"))
	if m == nil {
		return 0, false
	}
	p, err := strconv.ParseFloat(m[1], 64)
	return p, err == nil
}

// codeParseAsym reads an asymmetric tolerance "-20/+80", "+22/-56 %": the sizes of the upward and the downward deviation.
func codeParseAsym(s string) (plus, minus float64, ok bool) {
	m := codeAsymRe.FindStringSubmatch(codeCompact(s))
	if m == nil || (m[1] == "+") == (m[3] == "+") {
		return 0, 0, false
	}
	a, _ := strconv.ParseFloat(m[2], 64)
	b, _ := strconv.ParseFloat(m[4], 64)
	if m[1] == "+" {
		return a, b, true
	}
	return b, a, true
}

// codeRequestedTolerance is the tolerance (percent) of an encode request: the percentage given, or one assumed from the value
// (assumed is then true).
func codeRequestedTolerance(req EncodeRequest, v float64) (tol float64, assumed bool, err error) {
	if strings.TrimSpace(req.Tolerance) == "" {
		return codeAssumeTolerance(v), true, nil
	}
	p, ok := codeParsePercent(req.Tolerance)
	if !ok || p <= 0 {
		return 0, false, codeErr(CodeInvalidValue, "tolerance: cannot read %q as a percentage", req.Tolerance)
	}
	return p, false, nil
}

// codeCloseTo reports a == b within 1e-9 relative (exactly equal values included).
func codeCloseTo(a, b float64) bool {
	return a == b || math.Abs(a-b) <= 1e-9*math.Max(math.Abs(a), math.Abs(b))
}

// codeBandSeparators split a list of band colours: white space, comma, semicolon, underscore, hyphen, en dash, em dash.
var codeBandSeparators = regexp.MustCompile(`[\s\p{Z},;_\-–—]+`)

// codeSplitWords splits text at the band separators, lower-cased, without empty parts.
func codeSplitWords(text string) []string {
	var out []string
	for _, w := range codeBandSeparators.Split(strings.ToLower(text), -1) {
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

// codeLocalizedNotes renders the notes of a code kind for a language.
func codeLocalizedNotes(notes []Text, lang string) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.in(lang))
	}
	return out
}

// codeNeedLocales reports a name that lacks one of the 16 locales.
func codeNeedLocales(fail func(string, ...any), what string, t Text) {
	for _, l := range Locales {
		if strings.TrimSpace(t[l]) == "" {
			fail("data: %s: missing %q text", what, l)
		}
	}
	for l := range t {
		if !isLocale(l) {
			fail("data: %s: unknown language %q", what, l)
		}
	}
}

// codeNeedEnDe reports a note that lacks English or German text.
func codeNeedEnDe(fail func(string, ...any), what string, t Text) {
	for _, l := range docLocales {
		if strings.TrimSpace(t[l]) == "" {
			fail("data: %s: missing %q text", what, l)
		}
	}
}

var codeHexRe = regexp.MustCompile(`^(#[0-9A-Fa-f]{6}|transparent)$`)

// ---- shared pieces of the data ----------------------------------------------------------------------------------------

// codeCheckNotes validates the notes of a code kind (English and German each).
func codeCheckNotes(fail func(string, ...any), notes []Text) {
	for i, n := range notes {
		codeNeedEnDe(fail, fmt.Sprintf("notes[%d]", i), n)
	}
}
