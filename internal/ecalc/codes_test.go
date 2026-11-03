package ecalc

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

// The vectors of testdata/vectors/codes.json are shared with scripts/test-ecalc-codes.mjs. A decode vector gives the code
// and the whole result that is wanted, an encode vector the request and the result, an error vector the operation, the
// input and the error code with a fragment of its message. Every vector names its source in ref. The expected values were
// written by hand from the standards and data sheets, never taken from the engine.
type codeWant struct {
	Code     string   `json:"code"`
	Bands    []string `json:"bands"`
	Value    *float64 `json:"value"`
	Dim      string   `json:"dim"`
	Fields   [][]any  `json:"fields"` // [id, name, text, value or null, dim]
	Warnings []string `json:"warnings"`
}

type codeVectors struct {
	Decode []struct {
		Kind string   `json:"kind"`
		Code string   `json:"code"`
		Want codeWant `json:"want"`
		Ref  string   `json:"ref"`
	} `json:"decode"`
	Encode []struct {
		Kind string        `json:"kind"`
		Req  EncodeRequest `json:"req"`
		Want codeWant      `json:"want"`
		Ref  string        `json:"ref"`
	} `json:"encode"`
	Errors []struct {
		Op       string        `json:"op"`
		Kind     string        `json:"kind"`
		Code     string        `json:"code"`
		Req      EncodeRequest `json:"req"`
		Error    string        `json:"error"`
		Contains string        `json:"contains"`
		Ref      string        `json:"ref"`
	} `json:"errors"`
}

const codeVectorFile = "testdata/vectors/codes.json"

func codeTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	return cat
}

// codeCheckResult compares a result with a vector; it returns a description of the first difference or "".
func codeCheckResult(got CodeResult, want codeWant, kind string) string {
	switch {
	case got.Kind != kind:
		return fmt.Sprintf("kind = %q, want %q", got.Kind, kind)
	case got.Code != want.Code:
		return fmt.Sprintf("code = %q, want %q", got.Code, want.Code)
	case !reflect.DeepEqual(orEmpty(got.Bands), orEmpty(want.Bands)):
		return fmt.Sprintf("bands = %v, want %v", got.Bands, want.Bands)
	case got.Dim != want.Dim:
		return fmt.Sprintf("dim = %q, want %q", got.Dim, want.Dim)
	case !reflect.DeepEqual(orEmpty(got.Warnings), orEmpty(want.Warnings)):
		return fmt.Sprintf("warnings = %q, want %q", got.Warnings, want.Warnings)
	case len(got.Fields) != len(want.Fields):
		return fmt.Sprintf("%d fields %v, want %d", len(got.Fields), got.Fields, len(want.Fields))
	}
	wantValue := math.NaN()
	if want.Value != nil {
		wantValue = *want.Value
	}
	if !near(got.Value, wantValue, 1e-12) {
		return fmt.Sprintf("value = %v, want %v", got.Value, wantValue)
	}
	for i, f := range got.Fields {
		w := want.Fields[i]
		if len(w) != 5 {
			return fmt.Sprintf("field %d of the vector has %d entries, want 5", i, len(w))
		}
		fv := math.NaN()
		if w[3] != nil {
			fv = w[3].(float64)
		}
		if f.ID != w[0] || f.Name != w[1] || f.Text != w[2] || f.Dim != w[4] || !near(f.Value, fv, 1e-12) {
			return fmt.Sprintf("field %d = %+v, want %v", i, f, w)
		}
	}
	return ""
}

func TestCodesVectors(t *testing.T) {
	cat := codeTestCatalog(t)
	var v codeVectors
	readJSON(t, codeVectorFile, &v)
	perKind := map[string]int{}
	for _, c := range v.Decode {
		if c.Ref == "" {
			t.Errorf("decode %s %q has no ref", c.Kind, c.Code)
		}
		perKind["decode "+c.Kind]++
		got, err := cat.Decode(c.Kind, c.Code)
		if err != nil {
			t.Errorf("decode %s %q: %v", c.Kind, c.Code, err)
			continue
		}
		if diff := codeCheckResult(got, c.Want, c.Kind); diff != "" {
			t.Errorf("decode %s %q: %s", c.Kind, c.Code, diff)
		}
	}
	for _, c := range v.Encode {
		if c.Ref == "" {
			t.Errorf("encode %s %+v has no ref", c.Kind, c.Req)
		}
		perKind["encode "+c.Kind]++
		got, err := cat.Encode(c.Kind, c.Req)
		if err != nil {
			t.Errorf("encode %s %+v: %v", c.Kind, c.Req, err)
			continue
		}
		if diff := codeCheckResult(got, c.Want, c.Kind); diff != "" {
			t.Errorf("encode %s %+v: %s", c.Kind, c.Req, diff)
		}
	}
	for _, c := range v.Errors {
		if c.Ref == "" {
			t.Errorf("error vector %s %s has no ref", c.Op, c.Kind)
		}
		perKind["error "+c.Kind]++
		var err error
		if c.Op == "decode" {
			_, err = cat.Decode(c.Kind, c.Code)
		} else {
			_, err = cat.Encode(c.Kind, c.Req)
		}
		ce := AsCodeError(err)
		switch {
		case err == nil:
			t.Errorf("%s %s %q %+v: no error, want %s", c.Op, c.Kind, c.Code, c.Req, c.Error)
		case ce == nil:
			t.Errorf("%s %s %q %+v: error %v is no CodeError", c.Op, c.Kind, c.Code, c.Req, err)
		case ce.Code != c.Error || !strings.Contains(ce.Message, c.Contains) || !strings.HasPrefix(err.Error(), c.Error+": "):
			t.Errorf("%s %s %q %+v: error %q, want %s containing %q", c.Op, c.Kind, c.Code, c.Req, err, c.Error, c.Contains)
		}
	}
	for _, kind := range []string{"resistor_bands", "inductor_bands", "smd_resistor", "capacitor", "ceramic_temp"} {
		for _, what := range []string{"decode", "encode", "error"} {
			if n := perKind[what+" "+kind]; n < 8 {
				t.Errorf("%s %s has %d vectors, want at least 8", what, kind, n)
			}
		}
	}
}

func TestCodesKindsAndItemsInTheDefaultCatalog(t *testing.T) {
	cat := codeTestCatalog(t)
	want := map[string]string{
		"resistor_color_code": "resistor_bands", "inductor_color_code": "inductor_bands", "smd_resistor_code": "smd_resistor",
		"capacitor_code": "capacitor", "ceramic_temp_code": "ceramic_temp",
	}
	for id, kind := range want {
		c := cat.Code(id)
		if c == nil || c.CodeKind != kind || c.Category != "codes" {
			t.Errorf("code item %s = %+v, want kind %s in the category codes", id, c, kind)
			continue
		}
		if k, ok := cat.Item(id); !ok || k != "code" {
			t.Errorf("Item(%s) = %q, %v", id, k, ok)
		}
		if !contains(CodeKindIDs(), kind) {
			t.Errorf("CodeKindIDs() lacks %s", kind)
		}
		for _, l := range Locales {
			if strings.TrimSpace(c.Name[l]) == "" {
				t.Errorf("%s: no name in %s", id, l)
			}
		}
		if c.Doc["en"] == "" || c.Doc["de"] == "" || c.Source == "" || len(c.Tags) == 0 {
			t.Errorf("%s: doc, source and tags are required", id)
		}
	}
	if len(cat.Codes) != len(want) {
		t.Errorf("the catalog has %d code items, want %d", len(cat.Codes), len(want))
	}
	// A kind that has no code item in the catalog (a registered kind of a test, or a typo) is unsupported and names the real ones.
	for _, kind := range []string{"nope", "test_words", ""} {
		_, err := cat.Decode(kind, "x")
		if ce := AsCodeError(err); ce == nil || ce.Code != CodeUnsupported || !strings.Contains(ce.Message, "kinds: ") || !strings.Contains(ce.Message, "resistor_bands") {
			t.Errorf("Decode of the kind %q: %v", kind, err)
		}
	}
}

// The JSON of a code item is what the JavaScript twin reads: the tables of the kind, with the names and notes in the
// language of the request.
func TestCodesItemJSONCarriesTheTables(t *testing.T) {
	cat := codeTestCatalog(t)
	body, _, err := cat.LocalizedJSON("de")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	data := map[string]map[string]any{}
	for _, it := range doc.Items {
		if it["kind"] == "code" {
			data[it["code"].(string)] = it["data"].(map[string]any)
		}
	}
	keys := map[string][]string{
		"resistor_bands": {"colors", "layouts", "notes"},
		"inductor_bands": {"colors", "notes"},
		"smd_resistor":   {"eia96", "letters", "rkm", "ambiguous", "notes"},
		"capacitor":      {"multipliers", "rkm", "tolerances", "voltages", "not_cross_checked", "notes"},
		"ceramic_temp":   {"class1", "class2", "aliases", "industry", "not_cross_checked", "common", "notes"},
	}
	for kind, want := range keys {
		d := data[kind]
		if len(d) != len(want) {
			t.Errorf("%s: data has %d keys, want %d: %v", kind, len(d), len(want), d)
		}
		for _, k := range want {
			if _, ok := d[k]; !ok {
				t.Errorf("%s: data lacks %s", kind, k)
			}
		}
	}
	colors := data["resistor_bands"]["colors"].([]any)
	if len(colors) != 14 {
		t.Fatalf("resistor colours = %d, want 14", len(colors))
	}
	first := colors[0].(map[string]any)
	if first["id"] != "black" || first["name"] != "schwarz" || first["hex"] != "#000000" || first["digit"] != 0.0 || first["exp"] != 0.0 || first["tcr"] != 250.0 || first["tol"] != nil {
		t.Errorf("first resistor colour = %v", first)
	}
	if notes := data["resistor_bands"]["notes"].([]any); len(notes) < 5 || !strings.Contains(notes[0].(string), "Lesen") {
		t.Errorf("resistor notes are not German: %v", notes)
	}
	voltages := data["capacitor"]["voltages"].([]any)
	if len(voltages) != 22 {
		t.Errorf("voltage codes = %d, want 22", len(voltages))
	}
	last := voltages[len(voltages)-1].(map[string]any)
	if last["code"] != "3F" || !strings.Contains(last["note"].(string), "Murata führt") || !strings.Contains(last["note_en"].(string), "Murata lists") {
		t.Errorf("3F = %v", last)
	}
	if d := data["smd_resistor"]["eia96"].([]any); len(d) != 96 || d[0] != 100.0 || d[95] != 976.0 {
		t.Errorf("eia96 = %v", d)
	}
}

func TestCodesResultJSON(t *testing.T) {
	res := codeNewResult("capacitor")
	res.field("tolerance", "Tolerance", "±20 % / +80 %", math.NaN(), "ratio")
	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"kind":"capacitor","code":"","bands":[],"value":null,"dim":"","fields":[{"id":"tolerance","name":"Tolerance","text":"±20 % / +80 %","value":null,"dim":"ratio"}],"warnings":[]}`
	if string(body) != want {
		t.Errorf("JSON = %s\nwant   %s", body, want)
	}
	var zero CodeResult
	if body, _ := json.Marshal(zero); !strings.Contains(string(body), `"bands":[]`) || !strings.Contains(string(body), `"fields":[]`) || !strings.Contains(string(body), `"warnings":[]`) {
		t.Errorf("the zero result writes null lists: %s", body)
	}
}

func TestCodesErrorsCarryTheirCode(t *testing.T) {
	cat := codeTestCatalog(t)
	_, err := cat.Decode("resistor_bands", "gold")
	ce := AsCodeError(err)
	if ce == nil || ce.Code != CodeInvalidCode || err.Error() != CodeInvalidCode+": "+ce.Message {
		t.Errorf("error = %#v", err)
	}
	if AsCodeError(nil) != nil || AsCodeError(fmt.Errorf("x")) != nil {
		t.Error("AsCodeError must return nil for other errors")
	}
	if _, err := cat.Decode("resistor_bands", strings.Repeat("x", 65)); AsCodeError(err) == nil || !strings.Contains(err.Error(), "longer than 64") {
		t.Errorf("a long code: %v", err)
	}
	if _, err := cat.Encode("capacitor", EncodeRequest{Value: strings.Repeat("1", 65)}); AsCodeError(err) == nil || AsCodeError(err).Code != CodeInvalidValue {
		t.Errorf("a long request field: %v", err)
	}
}

// The IEC 60062 colour table, pinned independently of the vectors: tolerance and temperature coefficient per colour.
func TestCodesResistorColourTableIsIEC60062(t *testing.T) {
	cat := codeTestCatalog(t)
	d := cat.Code("resistor_color_code").Data.(*codeResistorData)
	type row struct {
		digit, exp int
		tol, tcr   float64
	}
	// colour: digit, exponent of the multiplier (-1 = no digit/multiplier), tolerance %, TCR ppm/K (0 = none)
	want := map[string]row{
		"black": {0, 0, 0, 250}, "brown": {1, 1, 1, 100}, "red": {2, 2, 2, 50}, "orange": {3, 3, 0.05, 15}, "yellow": {4, 4, 0.02, 25},
		"green": {5, 5, 0.5, 20}, "blue": {6, 6, 0.25, 10}, "violet": {7, 7, 0.1, 5}, "grey": {8, 8, 0.01, 1}, "white": {9, 9, 0, 0},
	}
	for id, w := range want {
		c := d.byID[id]
		if c == nil || c.Digit == nil || *c.Digit != w.digit || c.Exp == nil || *c.Exp != w.exp {
			t.Errorf("%s: digit/exp = %+v", id, c)
			continue
		}
		if (w.tol == 0) != (c.Tol == nil) || (c.Tol != nil && *c.Tol != w.tol) || (w.tcr == 0) != (c.TCR == nil) || (c.TCR != nil && *c.TCR != w.tcr) {
			t.Errorf("%s: tol/tcr = %v/%v, want %v/%v", id, c.Tol, c.TCR, w.tol, w.tcr)
		}
	}
	for id, w := range map[string]struct {
		exp int
		tol float64
	}{"gold": {-1, 5}, "silver": {-2, 10}, "pink": {-3, 0}, "none": {0, 20}} {
		c := d.byID[id]
		if c == nil || c.Digit != nil || c.TCR != nil || (w.tol == 0) != (c.Tol == nil) || (c.Tol != nil && *c.Tol != w.tol) || (id != "none" && (c.Exp == nil || *c.Exp != w.exp)) {
			t.Errorf("%s: %+v", id, c)
		}
	}
	// the older chart readings of the three colours IEC 60062:2016 assigned later
	if g := d.byID["grey"]; g.LegacyTol == nil || *g.LegacyTol != 0.05 || !d.byID["orange"].LegacyNone || !d.byID["yellow"].LegacyNone || d.byID["red"].LegacyNone || d.byID["red"].LegacyTol != nil {
		t.Error("the older tolerance readings of grey, orange and yellow are wrong")
	}
	for _, id := range []string{"black", "brown", "red", "orange", "yellow", "green", "blue", "violet", "grey", "white", "gold", "silver", "pink", "none"} {
		if d.byID[id] == nil {
			t.Errorf("colour %s is missing", id)
		}
	}
}

// Every EIA-96 code decodes to the E96 value of IEC 60063 times the multiplier of its letter.
func TestCodesSMDEveryEIA96Code(t *testing.T) {
	cat := codeTestCatalog(t)
	letters := map[string]int{"Y": -2, "X": -1, "A": 0, "B": 1, "C": 2, "D": 3, "E": 4, "F": 5}
	for i := 0; i < 96; i++ {
		for letter, exp := range letters {
			code := fmt.Sprintf("%02d%s", i+1, letter)
			res, err := cat.Decode("smd_resistor", code)
			if err != nil {
				t.Errorf("%s: %v", code, err)
				continue
			}
			want := codeDecimal(int(math.Round(e96[i]*100)), exp)
			if !near(res.Value, want, 1e-12) {
				t.Errorf("%s = %v, want %v", code, res.Value, want)
			}
		}
	}
}

// What Encode writes, Decode reads back to the same value (and the same bands).
func TestCodesEncodeThenDecodeAgrees(t *testing.T) {
	cat := codeTestCatalog(t)
	tolerances := []struct {
		text   string
		series string
	}{{"20%", "E6"}, {"10%", "E12"}, {"5%", "E24"}, {"2%", "E48"}, {"1%", "E96"}, {"0.5%", "E192"}, {"0.25%", "E192"}, {"0.1%", "E192"}, {"0.05%", "E192"}, {"0.02%", "E192"}, {"0.01%", "E192"}}
	checked := 0
	for _, tol := range tolerances {
		mant, _ := ESeriesValues(tol.series)
		for _, decade := range []float64{1, 1e3, 1e5, 1e-1} {
			for i := 0; i < len(mant); i += 7 {
				v := roundSig(mant[i]*decade, 6)
				req := EncodeRequest{Value: fmt.Sprintf("%g", v), Tolerance: tol.text}
				enc, err := cat.Encode("resistor_bands", req)
				if err != nil {
					t.Errorf("encode %+v: %v", req, err)
					continue
				}
				dec, err := cat.Decode("resistor_bands", enc.Code)
				if err != nil || !near(dec.Value, v, 1e-12) || !reflect.DeepEqual(dec.Bands, enc.Bands) || !near(dec.Value, enc.Value, 1e-12) {
					t.Errorf("%+v -> %q -> %+v, %v", req, enc.Code, dec, err)
				}
				checked++
			}
		}
	}
	for _, kind := range []string{"smd_resistor", "capacitor", "inductor_bands"} {
		dim := map[string]string{"smd_resistor": "", "capacitor": "p", "inductor_bands": "u"}[kind]
		mant, _ := ESeriesValues("E12")
		for _, decade := range []float64{1, 10, 100, 1000, 1e4, 1e5} {
			for _, m := range mant {
				req := EncodeRequest{Value: fmt.Sprintf("%g%s", roundSig(m*decade, 6), dim), Tolerance: "10%"}
				enc, err := cat.Encode(kind, req)
				if err != nil {
					continue // out of the range of the kind
				}
				if kind == "capacitor" {
					enc.Code = strings.Fields(enc.Code)[0]
					enc.Code = strings.TrimRight(enc.Code, "K")
				}
				dec, err := cat.Decode(kind, enc.Code)
				if err != nil || !near(dec.Value, enc.Value, 1e-12) {
					t.Errorf("%s %+v -> %q -> %+v, %v", kind, req, enc.Code, dec, err)
				}
				if kind == "smd_resistor" {
					for _, f := range enc.Fields {
						if strings.HasPrefix(f.ID, "code_") {
							other, err := cat.Decode(kind, f.Text)
							if err != nil || !near(other.Value, enc.Value, 1e-12) {
								t.Errorf("%s %s %q decodes to %+v, %v, want %v", kind, f.ID, f.Text, other, err, enc.Value)
							}
						}
					}
				}
				checked++
			}
		}
	}
	// the military inductor style
	for _, v := range []string{"0.1u", "0.47u", "1u", "6.8u", "10u", "82u", "100u", "1000u", "10m", "82m"} {
		enc, err := cat.Encode("inductor_bands", EncodeRequest{Value: v, Tolerance: "10%", Bands: 5})
		if err != nil {
			t.Errorf("MIL %s: %v", v, err)
			continue
		}
		dec, err := cat.Decode("inductor_bands", enc.Code)
		if err != nil || !near(dec.Value, enc.Value, 1e-12) || len(dec.Bands) != 5 {
			t.Errorf("MIL %s -> %q -> %+v, %v", v, enc.Code, dec, err)
		}
		checked++
	}
	// every ceramic code of the tables
	d := cat.Code("ceramic_temp_code").Data.(*codeCeramicData)
	for _, lo := range d.Class2.Low {
		for _, hi := range d.Class2.High {
			for _, ch := range d.Class2.Change {
				code := lo.Letter + hi.Digit + ch.Letter
				tolText := fmt.Sprintf("%g%%", ch.Plus)
				if ch.Plus != ch.Minus {
					tolText = fmt.Sprintf("+%g/-%g", ch.Plus, ch.Minus)
				}
				enc, err := cat.Encode("ceramic_temp", EncodeRequest{Class: "2", Value: fmt.Sprintf("%g..%g", lo.C, hi.C), Tolerance: tolText})
				if err != nil || enc.Code != code {
					t.Errorf("class 2 %s: %q, %v", code, enc.Code, err)
				}
				checked++
			}
		}
	}
	for _, s := range d.Class1.Sig {
		for _, m := range d.Class1.Mult {
			for _, tl := range d.Class1.Tol {
				code := s.Letter + m.Digit + tl.Letter
				dec, err := cat.Decode("ceramic_temp", code)
				if err != nil {
					t.Errorf("class 1 %s: %v", code, err)
					continue
				}
				enc, err := cat.Encode("ceramic_temp", EncodeRequest{Class: "1", TempCo: fmt.Sprintf("%gppm", dec.Value/1e-6), Tolerance: fmt.Sprintf("%gppm", tl.PPM)})
				if err != nil || enc.Code[2:] != code[2:] || !near(enc.Value, dec.Value, 1e-9) {
					t.Errorf("class 1 %s: %q, %v", code, enc.Code, err)
				}
				checked++
			}
		}
	}
	if checked < 700 {
		t.Errorf("only %d round trips ran", checked)
	}
}

// Decode never panics and rejects what is no code: single characters, odd separators and long runs of every kind.
func TestCodesDecodeSurvivesGarbage(t *testing.T) {
	cat := codeTestCatalog(t)
	inputs := []string{"", " ", "-", ",", "µ", "0", "00", "R", "L", "p", "100000", "ÄÖÜ", "☃", "1 2 3 4", "red,", ",red", "--red", "X7R7", "\t\n", "1H1H", "٣٣٣"}
	for _, ch := range "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz.|_" {
		inputs = append(inputs, string(ch), string(ch)+string(ch), "1"+string(ch), string(ch)+"1", "10"+string(ch), "1"+string(ch)+"1")
	}
	for _, kind := range []string{"resistor_bands", "inductor_bands", "smd_resistor", "capacitor", "ceramic_temp"} {
		for _, in := range inputs {
			res, err := cat.Decode(kind, in)
			if err != nil && AsCodeError(err) == nil {
				t.Errorf("%s %q: error %v is no CodeError", kind, in, err)
			}
			if err == nil && res.Code == "" {
				t.Errorf("%s %q: a result without a code", kind, in)
			}
		}
	}
}

// Load checks the data of a code item.
func TestCodesLoadRejectsBrokenData(t *testing.T) {
	read := func(name string) string {
		data, err := catalogFS.ReadFile("catalog/codes/" + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	cases := []struct{ file, old, new, want string }{
		{"resistor_bands", `zh: "黑色"`, `zh: ""`, `colour black name: missing "zh" text`},
		{"resistor_bands", `digit: 1, exp: 1`, `digit: 0, exp: 1`, "digit must be 0 to 9 and used once"},
		{"resistor_bands", `"#8B4513"`, `"brown"`, "hex must be #rrggbb or transparent"},
		{"resistor_bands", `roles: ["digit", "digit", "multiplier"]`, `roles: ["digit", "nope", "multiplier"]`, `unknown role "nope"`},
		{"resistor_bands", `roles: ["digit", "digit", "multiplier"]`, `roles: ["digit", "digit", "digit"]`, "exactly one multiplier band"},
		{"resistor_bands", "    layouts:\n", "    extra: 1\n    layouts:\n", "extra"},
		{"resistor_bands", `tol: 20`, `tol: 21`, "colour none with the tolerance 20 is required"},
		{"resistor_bands", `de: "Lesen ab dem Ende`, `dd: "Lesen ab dem Ende`, `notes[0]: missing "de" text`},
		{"inductor_bands", `digit: 9`, `digit: 8`, "digit must be 0 to 9 and used once"},
		{"inductor_bands", `mil_role: "point"`, `mil_role: "dot"`, "mil_role must be point or identifier"},
		{"smd_resistor", `eia96: [100, 102,`, `eia96: [100, 103,`, "eia96[1] = 103, the E96 value is 102"},
		{"smd_resistor", `{letter: "S"}`, `{letter: "Y"}`, "ambiguous[0]"},
		{"smd_resistor", `{letter: "K", exp: 3}`, `{letter: "k", exp: 3}`, "the letters are L, R, K, M and G"},
		{"capacitor", `makers: ["tdk", "nichicon", "panasonic"]}`, `makers: ["tdk"]}`, "at least two makers are required"},
		{"capacitor", `unit: "pf", when: "lt10pf"`, `unit: "pf", when: "any"`, "a pF tolerance is symmetric and applies below 10 pF"},
		{"capacitor", `{code: "0E",`, `{code: "0e",`, "a unique code of a digit and a capital letter"},
		{"capacitor", `{letters: ["m"], exp: -3}`, `{letters: ["m"], exp: 0}`, "with an exponent other than 0"},
		{"ceramic_temp", `{letter: "M", value: 1}`, `{letter: "C", value: 1}`, "class1.sig[1]: a unique letter"},
		{"ceramic_temp", `{digit: "5", value: 1}`, `{digit: "5", value: 0}`, "a unique digit and a multiplier other than 0"},
		{"ceramic_temp", `{letter: "G", ppm: 30}`, `{letter: "G", ppm: -30}`, "a unique letter and a positive tolerance"},
		{"ceramic_temp", `{name: "NP0", code: "C0G"}`, `{name: "", code: "C0G"}`, "aliases: a name and a 3-character code"},
	}
	for _, c := range cases {
		src := read(c.file)
		if !strings.Contains(src, c.old) {
			t.Errorf("%s: the shipped file does not contain %q", c.file, c.old)
			continue
		}
		_, err := Load(treeFS(t, map[string]string{"codes/k.yaml": strings.Replace(src, c.old, c.new, 1)}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %q -> %q: error = %v, want it to contain %q", c.file, c.old, c.new, err, c.want)
		}
	}
}
