package ecalc

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
)

// The query API of the codes runs against the default catalog (the five code items of part K) and against the fixture
// catalog with the test-only code kind test_words. Expected values come from the IEC 60062 vectors of codes.json.

func TestDescribeCode(t *testing.T) {
	cat := queryCatalog(t)
	d, err := cat.DescribeCode(" Resistor_Color_Code ")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "resistor_color_code" || d.Kind != "code" || d.Category != "codes" || d.CodeKind != "resistor_bands" || d.Name == "" || d.Doc == "" || d.Source == "" {
		t.Fatalf("description = %+v", d)
	}
	if len(d.Decode) != 1 || !strings.Contains(d.Decode[0], "band colours") || len(d.Encode) < 3 || len(d.Fields) != 2 {
		t.Errorf("forms = %+v", d)
	}
	// Every code item of the catalog has forms of its own; a kind without an entry is described generically.
	for _, k := range cat.Codes {
		if _, ok := codeFormsByKind[k.CodeKind]; !ok {
			t.Errorf("code kind %s of %s has no input forms", k.CodeKind, k.ID)
		}
	}
	for _, kind := range CodeKindIDs() {
		if _, ok := codeFormsByKind[kind]; !ok && kind != "test_words" {
			t.Errorf("code kind %s has no input forms", kind)
		}
	}
	g, err := queryFixture(t).DescribeCode("T_Code")
	if err != nil {
		t.Fatal(err)
	}
	if g.CodeKind != "test_words" || g.Doc != "Doc." || len(g.Decode) == 0 || len(g.Encode) == 0 || len(g.Fields) == 0 {
		t.Errorf("generic description = %+v", g)
	}
}

func TestDescribeCodeRefusesOtherKinds(t *testing.T) {
	real := queryCatalog(t)
	_, err := real.DescribeCode("awg")
	wantQueryError(t, err, CodeNotACode, "awg is a table, not a code")
	_, err = real.DescribeCode("voltage_divider")
	wantQueryError(t, err, CodeNotACode, "voltage_divider is a calculator, not a code")
	_, err = real.DescribeCode("")
	wantQueryError(t, err, CodeUnknownID, "id is required", "code id")
	_, err = real.DescribeCode("resistor_code")
	qe := wantQueryError(t, err, CodeUnknownID, `unknown code "resistor_code"`, "did you mean")
	if got := suggestedIDs(qe.Message); !slices.Contains(got, "resistor_color_code") {
		t.Errorf("suggestions %v of %q, want resistor_color_code among them", got, qe.Message)
	}
}

func TestResolveCodeKindTakesAKindOrACodeItem(t *testing.T) {
	cat := queryCatalog(t)
	for in, want := range map[string]string{
		"resistor_bands": "resistor_bands", "  CAPACITOR ": "capacitor", "resistor_color_code": "resistor_bands", "SMD_Resistor_Code": "smd_resistor", "ceramic_temp_code": "ceramic_temp",
	} {
		if got, err := cat.resolveCodeKind(in); err != nil || got != want {
			t.Errorf("resolveCodeKind(%q) = %q, %v, want %s", in, got, err, want)
		}
	}
	for _, in := range []string{"", "nope", "awg", "voltage_divider"} {
		_, err := cat.resolveCodeKind(in)
		wantQueryError(t, err, CodeUnknownCodeKind, "kinds: "+strings.Join(CodeKindIDs(), ", "))
	}
}

func TestCodeQueryErrorKeepsTheStableCode(t *testing.T) {
	for _, c := range []struct {
		err  error
		want QueryError
	}{
		{codeErr(CodeInvalidCode, "unknown colour %q", "pink"), QueryError{Code: "invalid_code", Message: `unknown colour "pink"`}},
		{codeErr(CodeInvalidValue, "value: cannot read it"), QueryError{Code: "invalid_value", Message: "value: cannot read it"}},
		{codeErr(CodeUnsupported, "six bands need a tempco"), QueryError{Code: "unsupported", Message: "six bands need a tempco"}},
		{errors.New("something else went wrong"), QueryError{Code: "invalid_code", Message: "something else went wrong"}},
	} {
		// The code is the code of the error and does not appear in front of the message again.
		if got := codeQueryError(c.err); got.Code != c.want.Code || got.Message != c.want.Message || strings.HasPrefix(got.Message, got.Code+":") {
			t.Errorf("codeQueryError(%v) = %+v, want %+v", c.err, *got, c.want)
		}
	}
}

func TestCodeAnswerLeavesOutWhatHasNoNumber(t *testing.T) {
	cat := queryCatalog(t)
	a := cat.codeAnswer(CodeResult{Kind: "x", Code: "X7R", Value: math.NaN(), Dim: "capacitance",
		Fields:   []CodeField{{ID: "min", Name: "Lowest temperature", Text: "-55 °C", Value: 218.15, Dim: "temperature"}, {ID: "class", Name: "Class", Text: "II", Value: math.NaN(), Dim: "number"}},
		Warnings: []string{"w"}})
	if a.Value != nil || a.Formatted != "" || a.Dim != "" || a.Fields[0].Value == nil || *a.Fields[0].Value != 218.15 || a.Fields[0].Dim != "temperature" || a.Fields[1].Value != nil || a.Fields[1].Dim != "" {
		t.Fatalf("answer = %+v", a)
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("a code without a value must encode: %v", err)
	}
	if want := `{"kind":"x","code":"X7R","fields":[{"id":"min","name":"Lowest temperature","text":"-55 °C","value":218.15,"dim":"temperature"},{"id":"class","name":"Class","text":"II"}],"warnings":["w"]}`; string(data) != want {
		t.Errorf("JSON = %s\nwant   %s", data, want)
	}
	// A value is formatted in its dimension with the usual 4 digits; empty lists stay out, the fields are an array.
	b := cat.codeAnswer(CodeResult{Kind: "r", Code: "4k7", Bands: []string{"yellow", "violet", "red"}, Value: 4700, Dim: "resistance"})
	if b.Formatted != "4.7 kΩ" || b.Value == nil || *b.Value != 4700 || b.Dim != "resistance" || len(b.Bands) != 3 || b.Fields == nil || len(b.Fields) != 0 {
		t.Errorf("answer = %+v", b)
	}
	if data, _ = json.Marshal(cat.codeAnswer(CodeResult{Kind: "k", Code: "c", Value: math.NaN(), Bands: []string{}, Warnings: []string{}})); string(data) != `{"kind":"k","code":"c","fields":[]}` {
		t.Errorf("an empty result = %s", data)
	}
}

func TestQueryDecodeReadsTheCodes(t *testing.T) {
	cat := queryCatalog(t)
	for _, c := range []struct {
		kind, code, canonical, formatted, fields string
		bands                                    int
	}{
		{"resistor_bands", "violet green yellow gold", "violet green yellow gold", "750 kΩ", "tolerance", 4},
		{"resistor_bands", " Blue-Grey, RED ", "blue grey red", "6.8 kΩ", "tolerance", 3},
		{"resistor_color_code", "blue grey red", "blue grey red", "6.8 kΩ", "tolerance", 3},
		{"inductor_bands", "silver blue gold grey silver", "silver blue gold grey silver", "6.8 µH", "layout,tolerance", 5},
		{"smd_resistor", "01C", "01C", "10 kΩ", "family", 0},
		{"smd_resistor", "472", "472", "4.7 kΩ", "family", 0},
		{"capacitor", "104", "104", "100 nF", "family", 0},
		{"ceramic_temp", "U2J", "U2J", "-750 ppm/K", "class,tc,tolerance,industry_code", 0},
	} {
		got, err := cat.QueryDecode(c.kind, c.code)
		if err != nil {
			t.Errorf("%s %q: %v", c.kind, c.code, err)
			continue
		}
		var ids []string
		for _, f := range got.Fields {
			ids = append(ids, f.ID)
		}
		kind, _ := cat.resolveCodeKind(c.kind)
		if got.Code != c.canonical || got.Formatted != c.formatted || strings.Join(ids, ",") != c.fields || len(got.Bands) != c.bands || got.Value == nil || got.Kind != kind {
			t.Errorf("%s %q = %+v", c.kind, c.code, got)
		}
	}
	// The tolerance of a resistor is a field with a number and a text.
	got, _ := cat.QueryDecode("resistor_bands", "violet green yellow gold")
	if f := got.Fields[0]; f.Text != "±5 %" || f.Value == nil || *f.Value != 0.05 || f.Dim != "ratio" || got.Dim != "resistance" || *got.Value != 750000 || len(got.Warnings) != 0 {
		t.Errorf("answer = %+v", got)
	}
	// A capacitor marking with tolerance and voltage, and a class 2 code without a value.
	got, err := cat.QueryDecode("capacitor", "104K 1H")
	if err != nil || got.Formatted != "100 nF" || len(got.Fields) < 3 || len(got.Warnings) == 0 {
		t.Errorf("104K 1H = %+v (%v)", got, err)
	}
	got, err = cat.QueryDecode("ceramic_temp", "X7R")
	if err != nil || got.Value != nil || got.Formatted != "" || got.Dim != "" || len(got.Fields) == 0 {
		t.Errorf("X7R = %+v (%v)", got, err)
	}
	if data, err := json.Marshal(got); err != nil || strings.Contains(string(data), "null") {
		t.Errorf("X7R JSON = %s (%v)", data, err)
	}
}

func TestQueryEncodeWritesTheCodes(t *testing.T) {
	cat := queryCatalog(t)
	for _, c := range []struct {
		kind      string
		req       EncodeRequest
		code      string
		formatted string
	}{
		{"resistor_bands", EncodeRequest{Value: "249k", Tolerance: "1%"}, "red yellow white orange brown", "249 kΩ"},
		{"resistor_color_code", EncodeRequest{Value: " 6.8k ", Tolerance: "20%"}, "blue grey red", "6.8 kΩ"},
		{"inductor_bands", EncodeRequest{Value: "6.8u", Tolerance: "10%", Bands: 5}, "silver blue gold grey silver", "6.8 µH"},
		{"smd_resistor", EncodeRequest{Value: "10k", Tolerance: "1%"}, "1002", "10 kΩ"},
		{"capacitor", EncodeRequest{Value: "100n", Tolerance: "K", Voltage: "50V"}, "104K 1H", "100 nF"},
	} {
		got, err := cat.QueryEncode(c.kind, c.req)
		if err != nil || got.Code != c.code || got.Formatted != c.formatted || got.Value == nil || got.Fields == nil {
			t.Errorf("%s %+v = %+v (%v)", c.kind, c.req, got, err)
		}
	}
	// Class 2 of the ceramic codes has no value; the capacitor warns about the voltage convention.
	got, err := cat.QueryEncode("ceramic_temp", EncodeRequest{Class: "2", Value: "-55..125", Tolerance: "15%"})
	if err != nil || got.Code != "X7R" || got.Value != nil {
		t.Errorf("X7R = %+v (%v)", got, err)
	}
	got, _ = cat.QueryEncode("capacitor", EncodeRequest{Value: "100n", Tolerance: "K", Voltage: "50V"})
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "maker convention") {
		t.Errorf("warnings = %v", got.Warnings)
	}
	// A value that is not a standard value of the series is replaced and the warning says so.
	got, err = cat.QueryEncode("resistor_bands", EncodeRequest{Value: "4.8k", Tolerance: "5%"})
	if err != nil || got.Code != "yellow violet red gold" || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "not a standard value") {
		t.Errorf("4.8k = %+v (%v)", got, err)
	}
}

func TestQueryDecodeAndEncodeAnswerWithTheCodeOfTheError(t *testing.T) {
	cat := queryCatalog(t)
	// The wrapper adds the kind check; everything else is the error of the code kind.
	_, err := cat.QueryDecode("nope", "103")
	wantQueryError(t, err, CodeUnknownCodeKind, `unknown code kind "nope"`, "resistor_bands")
	_, err = cat.QueryDecode("capacitor", "   ")
	wantQueryError(t, err, CodeInvalidCode, "the code is empty")
	_, err = cat.QueryDecode("resistor_bands", "gold red yellow")
	qe := wantQueryError(t, err, CodeInvalidCode, "the first band cannot be gold")
	if strings.HasPrefix(qe.Message, "invalid_code") {
		t.Errorf("the code is not repeated in the message: %q", qe.Message)
	}
	_, err = cat.QueryDecode("smd_resistor", "01S")
	wantQueryError(t, err, CodeUnsupported, "ambiguous between makers")
	_, err = cat.QueryDecode("capacitor", strings.Repeat("1", 65))
	wantQueryError(t, err, CodeInvalidCode, "64 characters")
	_, err = cat.QueryEncode("nope", EncodeRequest{Value: "4k7"})
	wantQueryError(t, err, CodeUnknownCodeKind)
	_, err = cat.QueryEncode("resistor_bands", EncodeRequest{Value: "abc"})
	wantQueryError(t, err, CodeInvalidValue, "value:")
	_, err = cat.QueryEncode("capacitor", EncodeRequest{})
	wantQueryError(t, err, CodeInvalidValue, "give a value, a tolerance or a voltage")
	_, err = cat.QueryEncode("resistor_bands", EncodeRequest{Value: "4k7", Tolerance: strings.Repeat("9", 65)})
	wantQueryError(t, err, CodeInvalidValue, "64 characters")
	_, err = cat.QueryEncode("inductor_bands", EncodeRequest{Value: "6.8u", Tolerance: "2%"})
	wantQueryError(t, err, CodeUnsupported, "no band colour for a tolerance")
}

// Every error vector of the code kinds comes back as a QueryError with the same code.
func TestQueryCodesKeepTheErrorsOfTheVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/vectors/codes.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Errors []struct {
			Op       string        `json:"op"`
			Kind     string        `json:"kind"`
			Code     string        `json:"code"`
			Req      EncodeRequest `json:"req"`
			Error    string        `json:"error"`
			Contains string        `json:"contains"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &file); err != nil || len(file.Errors) < 20 {
		t.Fatalf("codes.json: %v (%d error vectors)", err, len(file.Errors))
	}
	cat := queryCatalog(t)
	checked := 0
	for _, v := range file.Errors {
		if _, ok := codeKindOf(v.Kind); !ok {
			continue // an unknown kind is refused before the code kind sees it
		}
		var err error
		if v.Op == "decode" {
			_, err = cat.QueryDecode(v.Kind, v.Code)
		} else {
			_, err = cat.QueryEncode(v.Kind, v.Req)
		}
		qe := AsQueryError(err)
		if qe == nil || qe.Code != v.Error || !strings.Contains(qe.Message, v.Contains) {
			t.Errorf("%s %s %q %+v: %v, want %s containing %q", v.Op, v.Kind, v.Code, v.Req, err, v.Error, v.Contains)
		}
		checked++
	}
	if checked < 20 {
		t.Errorf("only %d error vectors were checked", checked)
	}
}
