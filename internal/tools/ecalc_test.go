package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ecalc"
)

func ecalcDesktopConfig() *config.Config {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

func ecalcPairs(pairs ...string) []ecalc.NamedValue {
	var out []ecalc.NamedValue
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, ecalc.NamedValue{Name: pairs[i], Value: pairs[i+1]})
	}
	return out
}

// ecalcDecode decodes the JSON output of the tool into a generic map.
func ecalcDecode(t *testing.T, exec VirtualDesktopExecution) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(exec.Output), &out); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, exec.Output)
	}
	return out
}

func TestECalcSearchListsRankedResults(t *testing.T) {
	// The catalog grows with its content, so the totals are not pinned: what matters is the order of the top hit, that
	// the answer never lists more than the limit, and what truncated means (more matches exist than are listed).
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "voltage divider", Limit: 1})
	if exec.Event != nil {
		t.Fatal("search must not emit a desktop event")
	}
	out := ecalcDecode(t, exec)
	results, _ := out["results"].([]any)
	total, _ := out["total"].(float64)
	// voltage_divider and loaded_divider always match, so a limit of 1 cuts the list: the single hit that fits is
	// listed (the default budget is far from reached) and the answer says it is truncated.
	if out["status"] != "ok" || out["operation"] != "search" || out["query"] != "voltage divider" || len(results) != 1 || out["returned"] != float64(1) || total < 2 || out["truncated"] != true {
		t.Fatalf("output = %s", exec.Output)
	}
	first := results[0].(map[string]any)
	if first["id"] != "voltage_divider" || first["kind"] != "calculator" || first["category"] != "basics" || first["name"] != "Voltage divider" || first["summary"] == "" {
		t.Fatalf("first result = %v", first)
	}
	// A larger limit lists at most that many; truncated is set exactly when more matches exist than are listed.
	for _, limit := range []int{0, 3, 25, 500} {
		out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "voltage divider", Limit: limit}))
		results, _ = out["results"].([]any)
		returned, _ := out["returned"].(float64)
		total, _ = out["total"].(float64)
		most := limit
		if limit < 1 {
			most = 10 // the default
		}
		most = min(most, 25)
		// This query has a handful of matches that are far from the budget, so the limit and the number of matches are
		// the only things that can cut the list (a hit is not dropped for size too early).
		if int(returned) != min(most, int(total)) || int(returned) != len(results) || len(results) < 1 || len(results) > most || total < returned || (out["truncated"] == true) != (total > returned) {
			t.Errorf("limit %d: returned %v of %v with %d results and truncated=%v: %s", limit, returned, total, len(results), out["truncated"], ecalcMarshalForLog(out))
		}
		// The exact id ranks first whatever else matches.
		if ids := resultIDs(results); len(ids) == 0 || ids[0] != "voltage_divider" {
			t.Errorf("limit %d: the ranking starts with %v", limit, ids)
		}
	}
}

func resultIDs(results []any) []string {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i], _ = r.(map[string]any)["id"].(string)
	}
	return ids
}

func ecalcMarshalForLog(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestECalcSearchWithoutQueryListsCategories(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "SEARCH", Query: "   "}))
	cats, _ := out["categories"].([]any)
	if out["status"] != "ok" || len(cats) < 10 || out["results"] != nil || !strings.Contains(out["note"].(string), "Give a query") {
		t.Fatalf("output = %v", out)
	}
	if first := cats[0].(map[string]any); first["id"] != "basics" || first["name"] != "Basics" || first["count"].(float64) < 11 {
		t.Fatalf("first category = %v", first)
	}
}

func TestECalcSearchWithoutMatchSaysSo(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "zzzz no such calculator"}))
	if out["status"] != "ok" || out["total"] != float64(0) || out["returned"] != float64(0) || out["results"] != nil || !strings.Contains(out["note"].(string), "No calculator matches") {
		t.Fatalf("output = %v", out)
	}
}

func TestECalcSearchShortensToTheBudget(t *testing.T) {
	// A budget of 2100 leaves 1500 bytes for the answer, which holds fewer than 25 hits.
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "a", Limit: 25, OutputBudget: 2100})
	out := ecalcDecode(t, exec)
	results, _ := out["results"].([]any)
	if len(exec.Output) > 1500 || len(results) < 1 || out["truncated"] != true || out["returned"] != float64(len(results)) {
		t.Fatalf("%d bytes, %d results: %s", len(exec.Output), len(results), exec.Output)
	}
}

func TestECalcDescribeAnswersTheStructure(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "voltage_divider"})
	out := ecalcDecode(t, exec)
	vars, _ := out["vars"].([]any)
	sets, _ := out["sets"].([]any)
	if out["status"] != "ok" || out["operation"] != "describe" || out["id"] != "voltage_divider" || out["dof"] != float64(3) || len(vars) != 4 || len(sets) != 4 || out["truncated"] != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	vin := vars[0].(map[string]any)
	if vin["id"] != "vin" || vin["default"] != "12 V" || vin["unit"] != "V" || vin["input"] != true {
		t.Fatalf("vin = %v", vin)
	}
	if strings.Contains(exec.Output, `<`) || strings.Contains(exec.Output, `&`) {
		t.Fatalf("HTML escapes in %s", exec.Output)
	}
}

func TestECalcDescribeShortensToTheBudget(t *testing.T) {
	full := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "wheatstone"})
	small := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "wheatstone", OutputBudget: 2100})
	out := ecalcDecode(t, small)
	if len(small.Output) >= len(full.Output) || out["status"] != "ok" || out["truncated"] != true {
		t.Fatalf("full %d bytes, small %d bytes: %s", len(full.Output), len(small.Output), small.Output)
	}
	if doc, _ := out["doc"].(string); len([]rune(doc)) > 240 {
		t.Fatalf("doc has %d runes", len([]rune(doc)))
	}
}

func TestECalcDescribeUnknownIDIsAnError(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "voltage_divide"}))
	if out["status"] != "error" || out["code"] != "unknown_id" || !strings.Contains(out["message"].(string), "did you mean voltage_divider") {
		t.Fatalf("output = %v", out)
	}
}

func TestECalcCalculateAnswersWithSIAndFormattedValues(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vin", "12 V", "r1", "4k7", "r2", "2k2")})
	if exec.Event != nil {
		t.Fatal("calculate must not emit a desktop event")
	}
	out := ecalcDecode(t, exec)
	qs, _ := out["quantities"].([]any)
	if out["status"] != "ok" || out["operation"] != "calculate" || out["eseries"] != "E24" || len(qs) != 6 || out["contradictions"] != nil || out["message"] != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	vout := qs[3].(map[string]any)
	if vout["id"] != "vout" || vout["formatted"] != "3.826 V" || vout["computed"] != true || vout["kind"] != "var" {
		t.Fatalf("vout = %v", vout)
	}
	if si, _ := vout["si"].(float64); si < 3.8260869 || si > 3.8260870 {
		t.Fatalf("vout si = %v", vout["si"])
	}
}

func TestECalcCalculateSuggestsStandardValues(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vout", "3.3 V", "vin", "12 V", "r1", "10k"), ESeries: "E24"})
	if !strings.Contains(exec.Output, `"eseries":{"series":"E24","value":3900,"formatted":"3.9 kΩ","dev_pct":2.82}`) {
		t.Fatalf("output = %s", exec.Output)
	}
}

func TestECalcCalculateErrorsListTheAlternatives(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vin", "12 V")}))
	sets, _ := out["sets"].([]any)
	if out["status"] != "error" || out["code"] != "underdetermined" || len(sets) != 4 || !strings.Contains(out["message"].(string), "needs 3 values, got 1") {
		t.Fatalf("output = %v", out)
	}
	for _, c := range []struct {
		req  ECalcRequest
		code string
	}{
		{ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("foo", "1")}, "unknown_variable"},
		{ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vin", "12 V", "r1", "4x7")}, "parse_error"},
		{ECalcRequest{Operation: "calculate", ID: "rms_waveform", Inputs: ecalcPairs("vrms", "1"), Choices: ecalcPairs("wave", "sawtooth")}, "unknown_option"},
		{ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vin", "1"), ESeries: "E7"}, "unknown_eseries"},
		{ECalcRequest{Operation: "calculate", ID: "", Inputs: ecalcPairs("vin", "1")}, "unknown_id"},
	} {
		if got := ecalcDecode(t, ExecuteECalc(nil, c.req)); got["status"] != "error" || got["code"] != c.code {
			t.Errorf("%+v: output = %v, want code %s", c.req, got, c.code)
		}
	}
}

func TestECalcCalculateReportsContradictionsAndPerQuantityErrors(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "ohms_law", Inputs: ecalcPairs("u", "10 V", "i", "1 A", "r", "5")}))
	contradictions, _ := out["contradictions"].([]any)
	if out["status"] != "ok" || len(contradictions) != 1 || !strings.Contains(out["message"].(string), "contradict the result by more than 1 %: r.") {
		t.Fatalf("output = %v", out)
	}
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "voltage_divider", Inputs: ecalcPairs("vin", "12 V", "r1", "-5", "r2", "2k2")}))
	if out["status"] != "error" || out["code"] != "calculation_failed" || !strings.Contains(out["message"].(string), "r1 (range:") || out["quantities"] == nil {
		t.Fatalf("output = %v", out)
	}
}

func TestECalcCalculateShortensToTheBudget(t *testing.T) {
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	req := ECalcRequest{Operation: "calculate", ID: "wheatstone", Inputs: ecalcPairs("vs", "5 V", "r1", "1k", "r2", "1k", "r3", "1k", "r4", "1k")}
	full := ecalcCalculate(cat, req, 100000)
	if !strings.Contains(full.Output, `"label":"`) || !strings.Contains(full.Output, `"name":"`) || !strings.Contains(full.Output, `"dim":"`) {
		t.Fatalf("a full answer names every quantity: %s", full.Output)
	}
	// The labels go first, then the names, then the dimensions; the values stay.
	noLabels := ecalcCalculate(cat, req, len(full.Output)-1)
	if strings.Contains(noLabels.Output, `"label"`) || !strings.Contains(noLabels.Output, `"name":"`) || len(noLabels.Output) >= len(full.Output) {
		t.Fatalf("labels must go first: %s", noLabels.Output)
	}
	tiny := ecalcCalculate(cat, req, 100)
	out := ecalcDecode(t, tiny)
	qs, _ := out["quantities"].([]any)
	if out["status"] != "ok" || len(qs) != 7 || !strings.Contains(tiny.Output, `"formatted":"0 V"`) {
		t.Fatalf("output = %s", tiny.Output)
	}
	for _, raw := range qs {
		q := raw.(map[string]any)
		if q["label"] != nil || q["name"] != nil || q["dim"] != nil || q["formatted"] == nil || q["id"] == nil {
			t.Fatalf("nothing but the values is left: %v", q)
		}
	}
}

func TestECalcOpenIsGatedLikeTheOtherDesktopTools(t *testing.T) {
	req := ECalcRequest{Operation: "open", ID: "voltage_divider"}
	for name, mutate := range map[string]func(*config.Config){
		"desktop off":            func(c *config.Config) { c.VirtualDesktop.Enabled = false },
		"agent control off":      func(c *config.Config) { c.VirtualDesktop.AllowAgentControl = false },
		"desktop tools disabled": func(c *config.Config) { c.Tools.VirtualDesktop.Enabled = false },
	} {
		cfg := ecalcDesktopConfig()
		mutate(cfg)
		exec := ExecuteECalc(cfg, req)
		out := ecalcDecode(t, exec)
		if exec.Event != nil || out["status"] != "policy_denied" {
			t.Errorf("%s: event=%v output=%s", name, exec.Event, exec.Output)
		}
	}
	if exec := ExecuteECalc(nil, req); exec.Event != nil || ecalcDecode(t, exec)["status"] != "policy_denied" {
		t.Fatalf("no config must be denied: %s", exec.Output)
	}
	denied := ecalcDecode(t, ExecuteECalc(func() *config.Config { c := ecalcDesktopConfig(); c.VirtualDesktop.Enabled = false; return c }(), req))
	if denied["code"] != "desktop_unavailable" {
		t.Fatalf("code = %v", denied["code"])
	}
	denied = ecalcDecode(t, ExecuteECalc(func() *config.Config { c := ecalcDesktopConfig(); c.VirtualDesktop.AllowAgentControl = false; return c }(), req))
	if denied["code"] != "desktop_agent_control_disabled" {
		t.Fatalf("code = %v", denied["code"])
	}
	// The read operations never need the desktop.
	if out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "ohms_law"})); out["status"] != "ok" {
		t.Fatalf("describe without a desktop: %v", out)
	}
}

func TestECalcOpenEmitsTheLaunchPayloadInGivenOrder(t *testing.T) {
	exec := ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{
		Operation: "open", ID: "Voltage_Divider",
		Inputs: ecalcPairs("vin", "12 V", "r1", "4k7"), ESeries: "E96",
	})
	if exec.Event == nil || exec.Event.Type != "open_app" || exec.Event.ID == "" {
		t.Fatalf("event = %+v, output = %s", exec.Event, exec.Output)
	}
	payload, err := json.Marshal(exec.Event.Payload)
	if err != nil {
		t.Fatal(err)
	}
	// "vin" stays in front of "r1": the app reads the order as the order of typing, which a sorted object would lose.
	want := `{"app_id":"ecalc","ecalc":{"choices":{},"page":"voltage_divider","values":{"vin":"12 V","r1":"4k7"}}}`
	if string(payload) != want {
		t.Fatalf("payload = %s\nwant      %s", payload, want)
	}
	out := ecalcDecode(t, exec)
	if out["status"] != "ok" || out["operation"] != "open" || out["page"] != "voltage_divider" || !strings.Contains(out["message"].(string), "Voltage divider calculator, pre-filled with 2 values") {
		t.Fatalf("output = %s", exec.Output)
	}
}

func TestECalcOpenSendsChoicesAndPlainPages(t *testing.T) {
	exec := ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{
		Operation: "open", ID: "rms_waveform", Inputs: ecalcPairs("vrms", "230 V"), Choices: ecalcPairs("Waveform", "Triangle"),
	})
	payload, _ := json.Marshal(exec.Event.Payload)
	if string(payload) != `{"app_id":"ecalc","ecalc":{"choices":{"wave":"triangle"},"page":"rms_waveform","values":{"vrms":"230 V"}}}` {
		t.Fatalf("payload = %s", payload)
	}
	exec = ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open", ID: "ohms_law"})
	payload, _ = json.Marshal(exec.Event.Payload)
	if string(payload) != `{"app_id":"ecalc","ecalc":{"choices":{},"page":"ohms_law","values":{}}}` {
		t.Fatalf("payload = %s", payload)
	}
	if strings.Contains(exec.Output, "pre-filled") {
		t.Fatalf("a bare page is not pre-filled: %s", exec.Output)
	}
	exec = ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open"})
	payload, _ = json.Marshal(exec.Event.Payload)
	if string(payload) != `{"app_id":"ecalc"}` {
		t.Fatalf("payload = %s", payload)
	}
}

func TestECalcOpenValidatesLikeCalculate(t *testing.T) {
	for _, c := range []struct {
		req  ECalcRequest
		code string
	}{
		{ECalcRequest{Operation: "open", ID: "nope"}, "unknown_id"},
		{ECalcRequest{Operation: "open", Inputs: ecalcPairs("vin", "12 V")}, "unknown_id"},
		{ECalcRequest{Operation: "open", ID: "voltage_divider", Inputs: ecalcPairs("zzz", "1")}, "unknown_variable"},
		{ECalcRequest{Operation: "open", ID: "voltage_divider", Inputs: ecalcPairs("vin", "12 apples")}, "parse_error"},
		{ECalcRequest{Operation: "open", ID: "voltage_divider", Choices: ecalcPairs("wave", "sine")}, "unknown_choice"},
	} {
		exec := ExecuteECalc(ecalcDesktopConfig(), c.req)
		if out := ecalcDecode(t, exec); exec.Event != nil || out["status"] != "error" || out["code"] != c.code {
			t.Errorf("%+v: event=%v output=%s, want code %s", c.req, exec.Event, exec.Output, c.code)
		}
	}
	// Fewer or more values than the calculator needs are fine: the app takes them as typed.
	exec := ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open", ID: "ohms_law", Inputs: ecalcPairs("u", "1", "i", "2", "r", "3", "p", "4")})
	if exec.Event == nil {
		t.Fatalf("output = %s", exec.Output)
	}
}

func TestECalcRejectsUnknownOperations(t *testing.T) {
	for _, op := range []string{"", "table", "decode", "run"} {
		out := ecalcDecode(t, ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: op}))
		if out["status"] != "error" || out["code"] != "invalid_operation" {
			t.Errorf("operation %q: output = %v", op, out)
		}
	}
}

func TestECalcCalculateAnswersTheCombinerWithCombinations(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "eseries_combiner", Inputs: ecalcPairs("target", "5k"), ESeries: "E24"})
	out := ecalcDecode(t, exec)
	combos, _ := out["combos"].([]any)
	if out["status"] != "ok" || len(combos) != 11 {
		t.Fatalf("output = %s", exec.Output)
	}
	// 4.7k is 6 % below and 5.1k 2 % above 5k; 3k + 2k and 7.5k || 15k are exactly 5k.
	for _, want := range []string{
		`"combos":[{"kind":"single","r1":"5.1 kΩ","value":"5.1 kΩ","dev_pct":2},{"kind":"series","r1":"3 kΩ","r2":"2 kΩ","value":"5 kΩ","dev_pct":0}`,
		`{"kind":"parallel","r1":"7.5 kΩ","r2":"15 kΩ","value":"5 kΩ","dev_pct":0}`,
	} {
		if !strings.Contains(exec.Output, want) {
			t.Fatalf("output lacks %s: %s", want, exec.Output)
		}
	}
	// A short budget keeps the single value and two entries of each group.
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	short := ecalcCalculate(cat, ECalcRequest{Operation: "calculate", ID: "eseries_combiner", Inputs: ecalcPairs("target", "5k")}, 100)
	if got, _ := ecalcDecode(t, short)["combos"].([]any); len(got) != 5 {
		t.Fatalf("a short answer lists %d combinations, want the single and two of each group: %s", len(got), short.Output)
	}
	desc := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "eseries_combiner"}))
	if desc["finder"] != "combine" || desc["formula"] != nil {
		t.Fatalf("describe = %v", desc)
	}
}
