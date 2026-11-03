package ecalc

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func queryCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Default()
	if err != nil {
		t.Fatalf("default catalog: %v", err)
	}
	return cat
}

func hitIDs(hits []SearchHit) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

func nv(pairs ...string) []NamedValue {
	var out []NamedValue
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, NamedValue{Name: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func quantity(t *testing.T, res CalcResult, id string) Quantity {
	t.Helper()
	for _, q := range res.Quantities {
		if q.ID == id {
			return q
		}
	}
	t.Fatalf("result has no quantity %q", id)
	return Quantity{}
}

func wantQueryError(t *testing.T, err error, code string, contains ...string) *QueryError {
	t.Helper()
	qe := AsQueryError(err)
	if qe == nil {
		t.Fatalf("error = %v, want a QueryError with code %s", err, code)
	}
	if qe.Code != code {
		t.Fatalf("code = %s (%s), want %s", qe.Code, qe.Message, code)
	}
	for _, want := range contains {
		if !strings.Contains(qe.Message, want) {
			t.Fatalf("message %q lacks %q", qe.Message, want)
		}
	}
	return qe
}

// ---- search ---------------------------------------------------------------------------------------------------

// The search tests pin the ranking rules, not the size of the catalog: calculators come and go, so a query is only
// expected to find "its" calculator first when the id matches exactly and among the first few otherwise.
func TestSearchRanksExactIDBeforeNamesTagsAndSubstrings(t *testing.T) {
	cat := queryCatalog(t)
	// An exact id (also written with spaces) beats every other kind of match.
	for query, want := range map[string]string{
		"voltage_divider": "voltage_divider", // exact id
		"voltage divider": "voltage_divider", // the id with spaces
		"wheatstone":      "wheatstone",      // exact one-word id
		"star delta":      "star_delta",      // the id of two words
	} {
		hits := cat.Search(query, 5)
		if len(hits) == 0 || hits[0].ID != want {
			t.Errorf("Search(%q) = %v, want %s first", query, hitIDs(hits), want)
		}
	}
	// Names (prefix, word), tags and the German names are matched too: the calculator is among the first hits.
	for query, want := range map[string]string{
		"ohm":              "ohms_law",        // name prefix
		"spannungsteiler":  "voltage_divider", // German name
		"messbrücke":       "wheatstone",      // German tag
		"ersatzwiderstand": "series_parallel", // German tag
	} {
		hits := cat.Search(query, 5)
		found := false
		for _, h := range hits[:min(3, len(hits))] {
			found = found || h.ID == want
		}
		if !found {
			t.Errorf("Search(%q) = %v, want %s among the first three", query, hitIDs(hits), want)
		}
	}
	// The case of the query does not matter.
	if got, want := hitIDs(cat.SearchAll("OHM")), hitIDs(cat.SearchAll("ohm")); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SearchAll(OHM) = %v, SearchAll(ohm) = %v", got, want)
	}
}

func TestSearchOrdersEqualRanksByCatalogPosition(t *testing.T) {
	cat := queryCatalog(t)
	// All three have "divider" as a word of their name; equal ranks keep the order of the catalog.
	position := map[string]int{}
	for i, h := range cat.SearchAll("divider") {
		position[h.ID] = i
	}
	order := []string{"voltage_divider", "loaded_divider", "current_divider"}
	for i, id := range order {
		if _, ok := position[id]; !ok {
			t.Fatalf("SearchAll(divider) does not list %s", id)
		}
		if i > 0 && position[order[i-1]] > position[id] {
			t.Errorf("%s ranks after %s although it comes first in the catalog (positions %v)", order[i-1], id, position)
		}
	}
	// An empty query lists every item in catalog order: the calculators, then the tables, then the codes.
	all := cat.SearchAll("")
	if len(all) != cat.ItemCount() || all[0].ID != cat.Calculators[0].ID || all[1].ID != cat.Calculators[1].ID {
		t.Fatalf("an empty query must list the catalog in order, got %v", hitIDs(all))
	}
	var want []string
	for _, calc := range cat.Calculators {
		want = append(want, calc.ID)
	}
	for _, tbl := range cat.Tables {
		want = append(want, tbl.ID)
	}
	for _, code := range cat.Codes {
		want = append(want, code.ID)
	}
	if got := hitIDs(all); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("an empty query lists %v, want the calculators, the tables and the codes in order: %v", got, want)
	}
}

func TestSearchMatchesNamesOfEveryLocale(t *testing.T) {
	cat := queryCatalog(t)
	vd := cat.Calculator("voltage_divider")
	for _, lang := range Locales {
		name := vd.Name[lang]
		hits := cat.Search(name, 5)
		found := false
		for _, h := range hits {
			found = found || h.ID == "voltage_divider"
		}
		if !found {
			t.Errorf("Search(%q) in %s = %v, want voltage_divider among the first five", name, lang, hitIDs(hits))
		}
	}
}

func TestSearchLimitsAndEmptyResults(t *testing.T) {
	cat := queryCatalog(t)
	if hits := cat.Search("", 3); len(hits) != 3 {
		t.Fatalf("limit 3 gave %d hits", len(hits))
	}
	if hits := cat.Search("", 0); len(hits) != min(DefaultSearchLimit, cat.ItemCount()) {
		t.Fatalf("limit 0 gave %d hits, want the default %d", len(hits), DefaultSearchLimit)
	}
	if hits := cat.Search("", 500); len(hits) != min(MaxSearchLimit, cat.ItemCount()) {
		t.Fatalf("limit 500 gave %d hits, want at most %d", len(hits), MaxSearchLimit)
	}
	none := cat.Search("zzzz no such thing", 5)
	if none == nil || len(none) != 0 {
		t.Fatalf("no match must be an empty, non-nil list, got %#v", none)
	}
	data, _ := json.Marshal(none)
	if string(data) != "[]" {
		t.Fatalf("no match encodes as %s", data)
	}
}

func TestSearchAnswersWithEnglishNameAndFirstSentence(t *testing.T) {
	cat := queryCatalog(t)
	hit := cat.Search("ohms_law", 1)[0]
	if hit.Kind != "calculator" || hit.Category != "basics" || hit.Name != "Ohm's law and power" {
		t.Fatalf("hit = %+v", hit)
	}
	want := "Ohm's law relates voltage, current and resistance (U = R × I); the electrical power is P = U × I."
	if hit.Summary != want {
		t.Fatalf("summary = %q, want %q", hit.Summary, want)
	}
	for _, calc := range cat.Calculators {
		for _, h := range cat.SearchAll(calc.ID)[:1] {
			if h.ID != calc.ID {
				t.Errorf("searching the id %q finds %s first", calc.ID, h.ID)
			}
			if strings.TrimSpace(h.Summary) == "" || strings.Contains(h.Summary, "\n") {
				t.Errorf("%s has summary %q", calc.ID, h.Summary)
			}
		}
	}
}

func TestSearchMatchesTheCategory(t *testing.T) {
	cat := queryCatalog(t)
	// The category id is matched like a tag: every calculator of the category is a hit (another calculator that merely
	// mentions the word may be one too, so the count is not pinned).
	listed := map[string]bool{}
	for _, h := range cat.SearchAll("basics") {
		listed[h.ID] = true
	}
	n := 0
	for _, calc := range cat.Calculators {
		if calc.Category == "basics" {
			n++
			if !listed[calc.ID] {
				t.Errorf("Search(basics) does not list the basics calculator %s", calc.ID)
			}
		}
	}
	overview := cat.Overview()
	if overview[0].ID != "basics" || overview[0].Name != "Basics" || overview[0].Count != n {
		t.Fatalf("overview = %+v", overview)
	}
	// Every category that holds an item is listed once with the number of its items of all kinds; a category without
	// items (the pinouts) is not.
	total := 0
	for _, c := range overview {
		total += c.Count
		if c.Count < 1 {
			t.Errorf("the overview lists the empty category %s", c.ID)
		}
	}
	if total != cat.ItemCount() {
		t.Errorf("the overview counts %d items, the catalog has %d", total, cat.ItemCount())
	}
	for _, c := range overview {
		if c.ID == "pinouts" {
			t.Error("the pinouts have no items yet and must not be listed")
		}
	}
}

func TestFirstSentenceSkipsAbbreviationsAndCuts(t *testing.T) {
	cases := map[string]string{
		"One. Two.":                        "One.",
		"Use e.g. 4k7 here. More.":         "Use e.g. 4k7 here.",
		"No period at all":                 "No period at all",
		"  Spaces\n\tand lines  collapse.": "Spaces and lines collapse.",
		"Value is 1.5 V. Next":             "Value is 1.5 V.",
	}
	for in, want := range cases {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
	long := firstSentence(strings.Repeat("word ", 100))
	if n := len([]rune(long)); n != summaryRunes || !strings.HasSuffix(long, "…") {
		t.Fatalf("a long sentence is cut to %d runes with an ellipsis, got %d: %q", summaryRunes, n, long)
	}
}

// ---- describe -------------------------------------------------------------------------------------------------

func TestDescribeVoltageDivider(t *testing.T) {
	cat := queryCatalog(t)
	d, err := cat.Describe("voltage_divider")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "voltage_divider" || d.Kind != "calculator" || d.Category != "basics" || d.Name != "Voltage divider" || d.DOF != 3 {
		t.Fatalf("description = %+v", d)
	}
	byID := map[string]VarInfo{}
	for _, v := range d.Vars {
		byID[v.ID] = v
	}
	if len(d.Vars) != 4 || d.Vars[0].ID != "vin" || d.Vars[3].ID != "vout" {
		t.Fatalf("vars = %+v", d.Vars)
	}
	if v := byID["vin"]; v.Default != "12 V" || v.Unit != "V" || !v.Input || v.Dim != "voltage" || v.Label != "V_in" || v.Name != "Input voltage" {
		t.Fatalf("vin = %+v", v)
	}
	if v := byID["r1"]; v.Default != "4.7 kΩ" || !v.ESeries || v.Unit != "Ω" {
		t.Fatalf("r1 = %+v", v)
	}
	if v := byID["vout"]; v.Default != "" {
		t.Fatalf("vout has default %q", v.Default)
	}
	if len(d.Sets) != 4 || strings.Join(d.Sets[0], ",") != "vin,r1,r2" {
		t.Fatalf("sets = %v", d.Sets)
	}
	if len(d.Derived) != 2 || d.Derived[0].ID != "i" || d.Derived[0].Dim != "current" || d.Derived[1].ID != "p" {
		t.Fatalf("derived = %+v", d.Derived)
	}
	if len(d.Checks) != 1 || !strings.HasPrefix(d.Checks[0], "V_out is not below V_in") {
		t.Fatalf("checks = %v", d.Checks)
	}
	if len(d.Formula) != 1 || d.Formula[0] != "vout = vin * r2 / (r1 + r2)" || !strings.HasPrefix(d.Doc, "Two resistors in series") || d.Source == "" {
		t.Fatalf("formula/doc/source = %v / %q / %q", d.Formula, d.Doc, d.Source)
	}
}

func TestDescribeChoicesListsAndUnits(t *testing.T) {
	cat := queryCatalog(t)
	rms, err := cat.Describe("rms_waveform")
	if err != nil {
		t.Fatal(err)
	}
	if len(rms.Choices) != 1 || rms.Choices[0].ID != "wave" || rms.Choices[0].Default != "sine" || len(rms.Choices[0].Options) != 3 || rms.Choices[0].Options[2].ID != "triangle" || rms.Choices[0].Options[2].Name != "Triangle" {
		t.Fatalf("choices = %+v", rms.Choices)
	}
	sp, _ := cat.Describe("series_parallel")
	if !sp.Vars[0].List || sp.Vars[0].Default != "4.7kΩ, 10kΩ, 2.2kΩ" {
		t.Fatalf("list variable = %+v", sp.Vars[0])
	}
	ce, _ := cat.Describe("charge_energy")
	q := ce.Vars[0]
	if q.Unit != "Ah" || q.Default != "2 Ah" || strings.Join(q.Units, ",") != "C,mAh,Ah" {
		t.Fatalf("charge variable = %+v", q)
	}
	if ce.Derived != nil {
		t.Fatalf("charge_energy has no derived quantities, got %+v", ce.Derived)
	}
}

func TestDescribeUnknownIDSuggestsAlternatives(t *testing.T) {
	cat := queryCatalog(t)
	_, err := cat.Describe("voltage_divide")
	wantQueryError(t, err, CodeUnknownID, "voltage_divide", "did you mean voltage_divider")
	_, err = cat.Describe("")
	wantQueryError(t, err, CodeUnknownID, "id is required")
	if d, err := cat.Describe("  Voltage_Divider "); err != nil || d.ID != "voltage_divider" {
		t.Fatalf("ids are matched case-insensitively: %v %v", d.ID, err)
	}
}

// ---- calculate ------------------------------------------------------------------------------------------------

func TestCalculateVoltageDividerForward(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "r1", "4k7", "r2", "2k2")})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "voltage_divider" || res.Name != "Voltage divider" || strings.Join(res.Basis, ",") != "vin,r1,r2" || res.ESeries != "E24" {
		t.Fatalf("result header = %+v", res)
	}
	if len(res.Contradictions) != 0 || len(res.Warnings) != 0 || res.HasErrors() {
		t.Fatalf("unexpected contradictions/warnings/errors: %+v", res)
	}
	vout := quantity(t, res, "vout")
	// 12 V * 2200 / (4700 + 2200) = 3.8260869...
	if !vout.Computed || vout.Formatted != "3.826 V" || vout.SI == nil || !near(*vout.SI, 3.826086956521739, 1e-12) || vout.Kind != "var" {
		t.Fatalf("vout = %+v", vout)
	}
	if q := quantity(t, res, "r1"); q.Computed || q.Given != "4k7" || q.Formatted != "4.7 kΩ" || q.ESeries != nil {
		t.Fatalf("r1 = %+v", q)
	}
	// i = 12 / 6900 = 1.7391 mA and p = 12 * i = 20.87 mW are derived.
	if i := quantity(t, res, "i"); i.Kind != "derived" || i.Formatted != "1.739 mA" {
		t.Fatalf("i = %+v", i)
	}
	if p := quantity(t, res, "p"); p.Formatted != "20.87 mW" {
		t.Fatalf("p = %+v", p)
	}
}

func TestCalculateSolvesBackwardsWithESeriesSuggestion(t *testing.T) {
	cat := queryCatalog(t)
	req := CalcRequest{Values: nv("vout", "3.3 V", "vin", "12 V", "r1", "10k")}
	res, err := cat.Calculate("voltage_divider", req)
	if err != nil {
		t.Fatal(err)
	}
	// r2 = vout * r1 / (vin - vout) = 3.3 * 10000 / 8.7 = 3793.1 ohm; E24 has 3.6k (-5.1 %) and 3.9k (+2.8 %).
	r2 := quantity(t, res, "r2")
	if !r2.Computed || r2.Formatted != "3.793 kΩ" {
		t.Fatalf("r2 = %+v", r2)
	}
	if h := r2.ESeries; h == nil || h.Series != "E24" || h.Value != 3900 || h.Formatted != "3.9 kΩ" || h.DevPct != 2.82 {
		t.Fatalf("E24 suggestion = %+v", h)
	}
	if strings.Join(res.Basis, ",") != "vin,r1,vout" {
		t.Fatalf("basis = %v", res.Basis)
	}
	// E96 has 3.74k (-1.40 %) and 3.83k (+0.97 %).
	req.ESeries = "e96"
	res, err = cat.Calculate("voltage_divider", req)
	if err != nil {
		t.Fatal(err)
	}
	if h := quantity(t, res, "r2").ESeries; h == nil || h.Series != "E96" || h.Value != 3830 || h.DevPct != 0.973 || res.ESeries != "E96" {
		t.Fatalf("E96 suggestion = %+v", h)
	}
	if res.Warnings != nil {
		t.Fatalf("warnings = %v", res.Warnings)
	}
}

func TestCalculateFiresWarnings(t *testing.T) {
	cat := queryCatalog(t)
	// A passive divider cannot raise the voltage: vout 20 V against vin 12 V fires the check of the calculator.
	res, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "vout", "20 V", "r1", "1k")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || !strings.HasPrefix(res.Warnings[0], "V_out is not below V_in") {
		t.Fatalf("warnings = %v, quantities = %+v", res.Warnings, res.Quantities)
	}
}

func TestCalculateReportsContradictionsAboveOnePercent(t *testing.T) {
	cat := queryCatalog(t)
	// The first solvable set inside {u, i, r} is {u, i}: r = 10 V / 1 A = 10 ohm.
	res, err := cat.Calculate("ohms_law", CalcRequest{Values: nv("u", "10 V", "i", "1 A", "r", "5")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Basis, ",") != "u,i" || len(res.Contradictions) != 1 {
		t.Fatalf("basis %v, contradictions %+v", res.Basis, res.Contradictions)
	}
	c := res.Contradictions[0]
	if c.Name != "r" || c.Given != "5 Ω" || c.Computed != "10 Ω" || c.GivenSI != 5 || c.ComputedSI != 10 || c.DiffPct != 50 {
		t.Fatalf("contradiction = %+v", c)
	}
	if r := quantity(t, res, "r"); !r.Computed || r.Given != "5" || r.Formatted != "10 Ω" {
		t.Fatalf("the quantity shows the computed value and remembers the given one: %+v", r)
	}
	// 10.1 differs by 0.99 % (below the tolerance), 10.2 by 1.96 %.
	res, _ = cat.Calculate("ohms_law", CalcRequest{Values: nv("u", "10 V", "i", "1 A", "r", "10.1")})
	if len(res.Contradictions) != 0 {
		t.Fatalf("0.99 %% is within the tolerance: %+v", res.Contradictions)
	}
	res, _ = cat.Calculate("ohms_law", CalcRequest{Values: nv("u", "10 V", "i", "1 A", "r", "10.2")})
	if len(res.Contradictions) != 1 || res.Contradictions[0].DiffPct != 1.96 {
		t.Fatalf("1.96 %% must be reported: %+v", res.Contradictions)
	}
}

func TestCalculateUnderDeterminedListsTheSolvableSets(t *testing.T) {
	cat := queryCatalog(t)
	_, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V")})
	qe := wantQueryError(t, err, CodeUnderdetermined, "needs 3 values, got 1", "given: vin", "vin + r1 + r2 (add r1, r2)")
	if len(qe.Sets) != 4 || strings.Join(qe.Sets[0], ",") != "vin,r1,r2" {
		t.Fatalf("sets = %v", qe.Sets)
	}
	_, err = cat.Calculate("voltage_divider", CalcRequest{})
	wantQueryError(t, err, CodeUnderdetermined, "needs 3 values, got 0")

	// Enough values, but not a solvable combination: the star needs ra, rb and rc, the delta rab, rbc and rca.
	_, err = cat.Calculate("star_delta", CalcRequest{Values: nv("rab", "10", "rbc", "20", "ra", "5")})
	qe = wantQueryError(t, err, CodeUnderdetermined, "no combination", "rab + rbc + rca (add rca)", "ra + rb + rc (add rb, rc)")
	if strings.Join(qe.Sets[0], ",") != "rab,rbc,rca" {
		t.Fatalf("the set that misses the fewest values comes first: %v", qe.Sets)
	}
}

func TestCalculateRejectsBadNames(t *testing.T) {
	cat := queryCatalog(t)
	_, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("foo", "1")})
	wantQueryError(t, err, CodeUnknownVariable, `"foo"`, "vin (Input voltage)", "r2 (Lower resistor)")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("i", "1 mA")})
	wantQueryError(t, err, CodeNotAnInput, "computed output")
	_, err = cat.Calculate("nope", CalcRequest{})
	wantQueryError(t, err, CodeUnknownID, `"nope"`)
	_, err = cat.Calculate("voltage_divider", CalcRequest{Choices: nv("wave", "sine")})
	wantQueryError(t, err, CodeUnknownChoice, "has no choices")
	_, err = cat.Calculate("rms_waveform", CalcRequest{Values: nv("vrms", "1"), Choices: nv("shape", "sine")})
	wantQueryError(t, err, CodeUnknownChoice, `"shape"`, "wave")
	_, err = cat.Calculate("rms_waveform", CalcRequest{Values: nv("vrms", "1"), Choices: nv("wave", "sawtooth")})
	wantQueryError(t, err, CodeUnknownOption, `"sawtooth"`, "sine, square, triangle")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "1", "VIN", "2")})
	wantQueryError(t, err, CodeDuplicate, "vin")
	_, err = cat.Calculate("rms_waveform", CalcRequest{Values: nv("vrms", "1"), Choices: nv("wave", "sine", "Waveform", "square")})
	wantQueryError(t, err, CodeDuplicate, "wave")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "1"), ESeries: "E7"})
	wantQueryError(t, err, CodeUnknownESeries, `"E7"`, "E3, E6, E12, E24, E48, E96, E192")
}

func TestCalculateAcceptsLabelsCaseAndOptionNames(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("V_in", "12 V", " R1 ", "4k7", "r_2", "2k2")})
	if err != nil || quantity(t, res, "vout").Formatted != "3.826 V" {
		t.Fatalf("labels and case: %v %+v", err, res)
	}
	res, err = cat.Calculate("rms_waveform", CalcRequest{Values: nv("vrms", "230 V"), Choices: nv("Waveform", "Triangle")})
	if err != nil {
		t.Fatal(err)
	}
	// triangle: V_pk = V_rms * sqrt(3) = 398.37 V.
	if q := quantity(t, res, "vpk"); q.Formatted != "398.4 V" {
		t.Fatalf("vpk = %+v", q)
	}
	if len(res.Choices) != 1 || res.Choices[0] != (ChoiceState{ID: "wave", Option: "triangle", OptionName: "Triangle"}) {
		t.Fatalf("choices = %+v", res.Choices)
	}
}

func TestCalculateUsesChoiceDefaultsWhenAbsent(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("rms_waveform", CalcRequest{Values: nv("vrms", "230 V")})
	if err != nil {
		t.Fatal(err)
	}
	// sine: V_pk = 230 * sqrt(2) = 325.27 V, V_pp = 650.54 V.
	if res.Choices[0].Option != "sine" || quantity(t, res, "vpk").Formatted != "325.3 V" || quantity(t, res, "vpp").Formatted != "650.5 V" {
		t.Fatalf("result = %+v", res)
	}
}

func TestCalculateReadsBareNumbersInTheDefaultUnit(t *testing.T) {
	cat := queryCatalog(t)
	// q is shown in Ah, so a bare 2 is 2 Ah = 7200 C; 7200 C * 3.7 V = 26640 J = 7.4 Wh.
	for _, q := range []string{"2", "2 Ah", "2000 mAh", "7200 C"} {
		res, err := cat.Calculate("charge_energy", CalcRequest{Values: nv("q", q, "v", "3.7")})
		if err != nil {
			t.Fatalf("q=%q: %v", q, err)
		}
		if e := quantity(t, res, "e"); e.Formatted != "7.4 Wh" {
			t.Errorf("q=%q: e = %+v", q, e)
		}
	}
}

func TestCalculateListVariable(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("series_parallel", CalcRequest{Values: nv("rs", "4k7, 10k, 2k2")})
	if err != nil {
		t.Fatal(err)
	}
	rs := quantity(t, res, "rs")
	if len(rs.SIList) != 3 || rs.SIList[1] != 10000 || rs.Formatted != "4.7kΩ, 10kΩ, 2.2kΩ" || rs.SI != nil {
		t.Fatalf("rs = %+v", rs)
	}
	// 1/(1/4700 + 1/10000 + 1/2200) = 1303.3 ohm.
	if quantity(t, res, "rser").Formatted != "16.9 kΩ" || quantity(t, res, "rpar").Formatted != "1.303 kΩ" {
		t.Fatalf("totals = %+v", res.Quantities)
	}
	_, err = cat.Calculate("series_parallel", CalcRequest{Values: nv("rs", "4k7 x 10k")})
	wantQueryError(t, err, CodeParse, "rs", "list of resistance values")
}

func TestCalculateParseErrorsNameTheVariable(t *testing.T) {
	cat := queryCatalog(t)
	_, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "r1", "4x7")})
	wantQueryError(t, err, CodeParse, `"4x7"`, "r1", "resistance")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "r1", "5 V")})
	wantQueryError(t, err, CodeParse, "r1")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "")})
	wantQueryError(t, err, CodeParse, "vin")
	_, err = cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", strings.Repeat("1", 65))})
	wantQueryError(t, err, CodeParse, "vin", "longer than")
}

func TestCalculateKeepsPerVariableErrorsInTheResult(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "r1", "-5", "r2", "2k2")})
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() {
		t.Fatalf("a negative resistance is a range error: %+v", res.Quantities)
	}
	if q := quantity(t, res, "r1"); !strings.HasPrefix(q.Error, "range:") || q.Given != "-5" {
		t.Fatalf("r1 = %+v", q)
	}
}

func TestCalculateLimitsTheNumberOfValues(t *testing.T) {
	cat := queryCatalog(t)
	many := make([]NamedValue, MaxQueryValues+1)
	for i := range many {
		many[i] = NamedValue{Name: "vin", Value: "1"}
	}
	_, err := cat.Calculate("voltage_divider", CalcRequest{Values: many})
	wantQueryError(t, err, CodeTooMany, "32")
}

func TestResultsEncodeAsJSONWithoutNaN(t *testing.T) {
	cat := queryCatalog(t)
	// vin == vout divides by zero in the rule of r2 (infinite, not finite); the result must still encode.
	res, err := cat.Calculate("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "vout", "12 V", "r1", "1k")})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("non-finite value in %s", data)
	}
	for _, calc := range cat.Calculators {
		d, err := cat.Describe(calc.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := json.Marshal(d); err != nil {
			t.Fatalf("%s: %v", calc.ID, err)
		}
	}
}

// Every calculator solves from the (twelve-digit) text of its own defaults, which also proves that the default
// formatting, the unit selection and the parser agree for every unit the catalog uses.
func TestCalculateEveryCalculatorFromItsDefaults(t *testing.T) {
	cat := queryCatalog(t)
	for _, calc := range cat.Calculators {
		var values []NamedValue
		for i := range calc.Vars {
			v := &calc.Vars[i]
			switch {
			case v.DefaultList != nil:
				parts := make([]string, len(v.DefaultList))
				for j, x := range v.DefaultList {
					parts[j] = strings.ReplaceAll(cat.Units.Format(x, v.Dim, v.Unit, 12), " ", "")
				}
				values = append(values, NamedValue{Name: v.ID, Value: strings.Join(parts, ", ")})
			case v.Default != nil:
				values = append(values, NamedValue{Name: v.ID, Value: cat.Units.Format(*v.Default, v.Dim, v.Unit, 12)})
			}
		}
		res, err := cat.Calculate(calc.ID, CalcRequest{Values: values})
		if err != nil {
			t.Errorf("%s: %v (values %v)", calc.ID, err, values)
			continue
		}
		if res.HasErrors() || len(res.Contradictions) != 0 || len(res.Basis) != calc.DOF {
			t.Errorf("%s: result from the defaults has errors or contradictions: %+v", calc.ID, res)
		}
		if _, err := json.Marshal(res); err != nil {
			t.Errorf("%s: %v", calc.ID, err)
		}
	}
}

// ---- check ----------------------------------------------------------------------------------------------------

func TestCheckResolvesNamesWithoutSolving(t *testing.T) {
	cat := queryCatalog(t)
	got, err := cat.Check("Voltage_Divider", CalcRequest{Values: nv("V_in", " 12 V ", "r1", "4k7")})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "voltage_divider" || len(got.Values) != 2 || got.Values[0] != (NamedValue{Name: "vin", Value: "12 V"}) || got.Values[1] != (NamedValue{Name: "r1", Value: "4k7"}) || got.Choices != nil {
		t.Fatalf("checked = %+v", got)
	}
	got, err = cat.Check("rms_waveform", CalcRequest{Choices: nv("Waveform", "Triangle")})
	if err != nil || len(got.Choices) != 1 || got.Choices[0] != (NamedValue{Name: "wave", Value: "triangle"}) || got.Values != nil {
		t.Fatalf("checked = %+v, %v", got, err)
	}
	if got, err := cat.Check("voltage_divider", CalcRequest{}); err != nil || got.ID != "voltage_divider" {
		t.Fatalf("a bare page id is fine: %+v %v", got, err)
	}
	// more values than the calculator needs, and fewer, are fine: ECalc takes them as typed.
	if _, err := cat.Check("ohms_law", CalcRequest{Values: nv("u", "1", "i", "2", "r", "3", "p", "4")}); err != nil {
		t.Fatal(err)
	}
	_, err = cat.Check("voltage_divider", CalcRequest{Values: nv("vin", "12 V", "r1", "4x7")})
	wantQueryError(t, err, CodeParse, "r1")
	_, err = cat.Check("voltage_divider", CalcRequest{Values: nv("nope", "1")})
	wantQueryError(t, err, CodeUnknownVariable)
	_, err = cat.Check("nope", CalcRequest{})
	wantQueryError(t, err, CodeUnknownID)
}

// The desktop bridge sanitizes the launch payload with these patterns (sdk-events-bootstrap.js): a longer id would
// be dropped silently, so the catalog must stay inside them.
func TestCatalogIDsFitTheDesktopBridge(t *testing.T) {
	cat := queryCatalog(t)
	page := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	name := regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	// The tables and codes are opened by their id too (open with an id of any kind).
	for _, it := range cat.searchItems() {
		if !page.MatchString(it.id) {
			t.Errorf("%s id %q does not fit the bridge", it.kind, it.id)
		}
	}
	for _, calc := range cat.Calculators {
		for _, v := range calc.Vars {
			if !name.MatchString(v.ID) {
				t.Errorf("%s: variable id %q does not fit the bridge", calc.ID, v.ID)
			}
		}
		for _, ch := range calc.Choices {
			if !name.MatchString(ch.ID) {
				t.Errorf("%s: choice id %q does not fit the bridge", calc.ID, ch.ID)
			}
			for _, o := range ch.Options {
				if !name.MatchString(o.ID) {
					t.Errorf("%s: option id %q does not fit the bridge", calc.ID, o.ID)
				}
			}
		}
	}
}

// ---- phase 2 extensions: combiner, display units, ignored variables, chart, number systems ---------------------

// ignoreCalc exercises the schema extensions of phase 2: a display-unit whitelist (a is shown in dBV), a choice option
// that ignores b (which has a default), a derived power in dBm and a chart.
var ignoreCalc = strings.ReplaceAll(`- id: t_ign
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    a: {dim: voltage, label: A, name: {en: "A", de: "A"}, default: "10 V", unit: dBV, units: ["", dBV, dBu]}
    b: {dim: resistance, label: B, name: {en: "B", de: "B"}, default: "100"}
    c: {dim: current, label: C, name: {en: "C", de: "C"}}
  choices:
    q:
      name: NAME16
      default: o
      options:
        o: {name: NAME16, consts: {k: 1}, ignore: [b]}
        p: {name: NAME16, consts: {k: 2}}
  dof: 2
  rules:
    c: ["a/b*k"]
    a: ["c*b/k"]
    b: ["a*k/c"]
  derived:
    p: {dim: power, label: P, name: {en: "P", de: "P"}, expr: "a*c", unit: dBm, units: [dBm, ""]}
  chart: {x: a, y: c, to: "10", samples: 21}
  formula: ["c = a / b * k"]

- id: t_num
  kind: calculator
  category: basics
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  vars:
    n: {dim: integer, label: N, name: {en: "N", de: "N"}, default: "0xFF", unit: hex}
    m: {dim: integer, label: M, name: {en: "M", de: "M"}, unit: dec}
  dof: 1
  rules:
    m: ["n*2"]
    n: ["m/2"]
  formula: ["m = n * 2"]
`, "NAME16", flow16("T"))

func extensionCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(fixtureFS(t, map[string]string{"t.yaml": ignoreCalc}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func TestCalculateCombinerListsTheBestCombinations(t *testing.T) {
	cat := queryCatalog(t)
	res, err := cat.Calculate("eseries_combiner", CalcRequest{Values: nv("target", "5k"), ESeries: "E24"})
	if err != nil {
		t.Fatal(err)
	}
	// Hand-checked like the vectors of the combiner: 4.7k is 6 % below 5k and 5.1k is 2 % above; 3k + 2k, 3.9k + 1.1k,
	// 7.5k || 15k and 10k || 10k are exactly 5k.
	if len(res.Combos) != 1+2*comboLimit {
		t.Fatalf("%d combos, want the single and %d of each group: %+v", len(res.Combos), comboLimit, res.Combos)
	}
	want := []ComboInfo{
		{Kind: "single", R1: "5.1 kΩ", Value: "5.1 kΩ", DevPct: 2},
		{Kind: "series", R1: "3 kΩ", R2: "2 kΩ", Value: "5 kΩ", DevPct: 0},
		{Kind: "series", R1: "3.9 kΩ", R2: "1.1 kΩ", Value: "5 kΩ", DevPct: 0},
	}
	for i, w := range want {
		if res.Combos[i] != w {
			t.Errorf("combos[%d] = %+v, want %+v", i, res.Combos[i], w)
		}
	}
	parallel := res.Combos[1+comboLimit]
	if parallel != (ComboInfo{Kind: "parallel", R1: "7.5 kΩ", R2: "15 kΩ", Value: "5 kΩ", DevPct: 0}) {
		t.Fatalf("first parallel combination = %+v", parallel)
	}
	if q := quantity(t, res, "target"); q.Computed || q.Given != "5k" || q.Formatted != "5 kΩ" {
		t.Fatalf("target = %+v", q)
	}
	// E96 has 4.99k, 0.2 % below.
	res, err = cat.Calculate("eseries_combiner", CalcRequest{Values: nv("target", "5k"), ESeries: "E96"})
	if err != nil || res.Combos[0] != (ComboInfo{Kind: "single", R1: "4.99 kΩ", Value: "4.99 kΩ", DevPct: -0.2}) {
		t.Fatalf("E96: %v %+v", err, res.Combos)
	}
	// An invalid target has no combinations, only its error.
	res, err = cat.Calculate("eseries_combiner", CalcRequest{Values: nv("target", "-5")})
	if err != nil || res.Combos != nil || !res.HasErrors() {
		t.Fatalf("a negative target: %v %+v", err, res)
	}
	// Other calculators have none.
	if res, _ := cat.Calculate("ohms_law", CalcRequest{Values: nv("u", "1", "i", "1")}); res.Combos != nil {
		t.Fatalf("ohms_law combos = %+v", res.Combos)
	}
	d, err := cat.Describe("eseries_combiner")
	if err != nil || d.Finder != "combine" || len(d.Formula) != 0 || d.Vars[0].Default != "5 kΩ" || d.Chart != nil {
		t.Fatalf("description = %+v, %v", d, err)
	}
	if encoded, _ := json.Marshal(d); strings.Contains(string(encoded), `"formula"`) || strings.Contains(string(encoded), `"chart"`) {
		t.Fatalf("empty formula and no chart are left out: %s", encoded)
	}
}

func TestDescribeExposesUnitsIgnoreAndChart(t *testing.T) {
	cat := extensionCatalog(t)
	d, err := cat.Describe("t_ign")
	if err != nil {
		t.Fatal(err)
	}
	a := d.Vars[0]
	// The whitelist "", dBV, dBu offers the base symbol and the two explicit units; a bare number is read in dBV, the
	// default 10 V = 20 dBV is shown in it.
	if a.Unit != "dBV" || strings.Join(a.Units, ",") != "V,dBV,dBu" || a.Default != "20 dBV" {
		t.Fatalf("a = %+v", a)
	}
	if b := d.Vars[1]; strings.Join(b.Units, ",") != "Ω" || b.Unit != "Ω" {
		t.Fatalf("b = %+v", b)
	}
	opts := d.Choices[0].Options
	if len(opts) != 2 || strings.Join(opts[0].Ignore, ",") != "b" || opts[1].Ignore != nil {
		t.Fatalf("options = %+v", opts)
	}
	if d.Chart == nil || *d.Chart != (ChartInfo{X: "a", Y: "c", To: "10", Samples: 21}) || d.Finder != "" {
		t.Fatalf("chart = %+v finder %q", d.Chart, d.Finder)
	}
	if d.Derived[0].Unit != "dBm" {
		t.Fatalf("derived = %+v", d.Derived)
	}
}

func TestCalculateTakesTheDefaultsOfIgnoredVariables(t *testing.T) {
	cat := extensionCatalog(t)
	// Option o ignores b, which has a default of 100 ohm: a alone is enough. c = a / b * k = 10 V / 100 ohm = 0.1 A.
	res, err := cat.Calculate("t_ign", CalcRequest{Values: nv("a", "10 V")})
	if err != nil {
		t.Fatal(err)
	}
	b := quantity(t, res, "b")
	if strings.Join(res.Basis, ",") != "a,b" || !b.Ignored || !b.Defaulted || b.Computed || b.Given != "" || b.Formatted != "100 Ω" {
		t.Fatalf("basis %v, b = %+v", res.Basis, b)
	}
	if c := quantity(t, res, "c"); !c.Computed || c.Formatted != "100 mA" || c.Ignored {
		t.Fatalf("c = %+v", c)
	}
	// The bare number reads in dBV: 20 dBV is 10 V, and p = a * c = 1 W = 30 dBm.
	res, err = cat.Calculate("t_ign", CalcRequest{Values: nv("a", "20")})
	if err != nil || quantity(t, res, "c").Formatted != "100 mA" || quantity(t, res, "p").Formatted != "30 dBm" || quantity(t, res, "a").Formatted != "20 dBV" {
		t.Fatalf("dBV: %v %+v", err, res.Quantities)
	}
	// Two values of the request win over a default: {a, c} is a set, so b is computed (10 V / 0.2 A = 50 ohm) and
	// stays marked as ignored but not as a default.
	res, err = cat.Calculate("t_ign", CalcRequest{Values: nv("a", "10 V", "c", "0.2 A")})
	if err != nil {
		t.Fatal(err)
	}
	if b := quantity(t, res, "b"); !b.Ignored || b.Defaulted || !b.Computed || b.Formatted != "50 Ω" || strings.Join(res.Basis, ",") != "a,c" || len(res.Contradictions) != 0 {
		t.Fatalf("basis %v, b = %+v", res.Basis, b)
	}
	// An ignored variable that is given is used like any other and keeps its mark: c = 10 V / 50 ohm = 0.2 A.
	res, _ = cat.Calculate("t_ign", CalcRequest{Values: nv("a", "10 V", "b", "50")})
	if b := quantity(t, res, "b"); !b.Ignored || b.Defaulted || b.Given != "50" || quantity(t, res, "c").Formatted != "200 mA" {
		t.Fatalf("b = %+v", b)
	}
	// Option p ignores nothing: one value is not enough, as without the extension.
	_, err = cat.Calculate("t_ign", CalcRequest{Values: nv("a", "10 V"), Choices: nv("q", "p")})
	wantQueryError(t, err, CodeUnderdetermined, "needs 2 values, got 1")
	// With option p, k = 2: c = a / b * 2 = 10 V / 100 ohm * 2 = 0.2 A.
	res, err = cat.Calculate("t_ign", CalcRequest{Values: nv("a", "10 V", "b", "100"), Choices: nv("q", "p")})
	if err != nil || quantity(t, res, "c").Formatted != "200 mA" || quantity(t, res, "b").Ignored {
		t.Fatalf("option p: %v %+v", err, res.Quantities)
	}
}

func TestCalculateFormatsNumberSystemsWithoutAUnit(t *testing.T) {
	cat := extensionCatalog(t)
	d, _ := cat.Describe("t_num")
	if n := d.Vars[0]; n.Unit != "hex" || n.Default != "0xFF" || strings.Join(n.Units, ",") != "dec,hex,bin,oct" {
		t.Fatalf("n = %+v", n)
	}
	// 0xFF = 255; m = 2 * 255 = 510 is shown in decimal, n in hex; a bare "ff" reads in the unit of the variable.
	for _, text := range []string{"0xFF", "ff", "0xff", "FF"} {
		res, err := cat.Calculate("t_num", CalcRequest{Values: nv("n", text)})
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if n, m := quantity(t, res, "n"), quantity(t, res, "m"); n.Formatted != "0xFF" || m.Formatted != "510" || n.SI == nil || *n.SI != 255 {
			t.Errorf("%q: n = %+v, m = %+v", text, n, m)
		}
	}
	// A value above 2^53 - 1 does not parse; half of an odd number is no whole number, which is a range error.
	_, err := cat.Calculate("t_num", CalcRequest{Values: nv("n", "0x20000000000000")})
	wantQueryError(t, err, CodeParse, "n")
	res, err := cat.Calculate("t_num", CalcRequest{Values: nv("m", "255")})
	if err != nil || !res.HasErrors() || !strings.Contains(quantity(t, res, "n").Error, "range") {
		t.Fatalf("255 / 2 is no whole number: %v %+v", err, res.Quantities)
	}
}
