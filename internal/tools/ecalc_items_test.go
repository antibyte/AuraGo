package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/ecalc"
)

// The tables and codes of the ecalc tool: search and describe cover them, table reads rows and lookups, decode and
// encode work on component codes, open takes any id. The tests use the generated tables (awg, ipc2221, eseries_e24,
// eseries_e192) and never pin the ids of the content tables.

func TestECalcSearchAndDescribeCoverTablesAndCodes(t *testing.T) {
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	// An empty query counts every item and lists the categories that hold items, the tables among them.
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "search"}))
	cats, _ := out["categories"].([]any)
	found := false
	for _, c := range cats {
		found = found || c.(map[string]any)["id"] == "tables"
		if c.(map[string]any)["id"] == "pinouts" {
			t.Errorf("the empty pinouts must not be listed: %v", cats)
		}
	}
	if out["total"] != float64(cat.ItemCount()) || !found || !strings.Contains(out["note"].(string), "calculators, tables and codes") {
		t.Fatalf("output = %v", out)
	}
	// A table is found by its id and says what it is.
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "awg"}))
	results, _ := out["results"].([]any)
	if out["status"] != "ok" || len(results) < 2 || results[0].(map[string]any)["id"] != "awg" || results[0].(map[string]any)["kind"] != "table" || results[0].(map[string]any)["category"] != "tables" {
		t.Fatalf("output = %v", out)
	}
	// A code is found by its name and says what it is; describe names the kind that decode and encode take.
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "search", Query: "resistor color code"}))
	results, _ = out["results"].([]any)
	if len(results) == 0 || results[0].(map[string]any)["id"] != "resistor_color_code" || results[0].(map[string]any)["kind"] != "code" || results[0].(map[string]any)["category"] != "codes" {
		t.Fatalf("code search = %v", out)
	}
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "Resistor_Color_Code"})
	out = ecalcDecode(t, exec)
	if out["status"] != "ok" || out["kind"] != "code" || out["code_kind"] != "resistor_bands" || len(out["decode"].([]any)) == 0 || len(out["encode"].([]any)) < 3 || len(out["fields"].([]any)) == 0 || out["source"] == nil || exec.Event != nil {
		t.Fatalf("describe code = %s", exec.Output)
	}
	// describe answers for a table: columns with units, the key, the row count, the notes.
	exec = ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "AWG"})
	out = ecalcDecode(t, exec)
	columns, _ := out["columns"].([]any)
	if out["status"] != "ok" || out["operation"] != "describe" || out["id"] != "awg" || out["kind"] != "table" || out["group"] != "conductors" || out["key"] != "size" ||
		out["row_count"] != float64(44) || len(columns) != 5 || out["notes"] == nil || out["source"] == nil || out["vars"] != nil || exec.Event != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	size, area := columns[0].(map[string]any), columns[3].(map[string]any)
	if size["id"] != "size" || size["dim"] != "text" || size["key"] != true || area["id"] != "area" || area["unit"] != "mm²" || len(area["units"].([]any)) != 2 {
		t.Errorf("columns = %v", columns)
	}
	if out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "ipc2221"})); out["guide"] != true || !strings.Contains(out["caution"].(string), "authoritative") {
		t.Errorf("ipc2221 is a guide table: %v", out)
	}
	// An unknown id suggests items of any kind; calculate on a table says what it is.
	if out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "awg_wire"})); out["code"] != "unknown_id" || !strings.Contains(out["message"].(string), "did you mean") {
		t.Errorf("unknown = %v", out)
	}
	if out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "calculate", ID: "awg", Inputs: ecalcPairs("x", "1")})); out["code"] != "not_a_calculator" || !strings.Contains(out["message"].(string), "awg is a table") {
		t.Errorf("calculate on a table = %v", out)
	}
}

func TestECalcDescribeShortensATableToTheBudget(t *testing.T) {
	full := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "ipc2221"})
	small := ExecuteECalc(nil, ECalcRequest{Operation: "describe", ID: "ipc2221", OutputBudget: 2100})
	out := ecalcDecode(t, small)
	if len(small.Output) >= len(full.Output) || out["status"] != "ok" || out["truncated"] != true || out["columns"] == nil {
		t.Fatalf("full %d bytes, small %d bytes: %s", len(full.Output), len(small.Output), small.Output)
	}
}

func TestECalcTableListsAPageOfRows(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg"})
	if exec.Event != nil {
		t.Fatal("table must not emit a desktop event")
	}
	out := ecalcDecode(t, exec)
	rows, _ := out["rows"].([]any)
	si, _ := out["si"].([]any)
	columns, _ := out["columns"].([]any)
	if out["status"] != "ok" || out["operation"] != "table" || out["id"] != "awg" || out["total"] != float64(44) || out["offset"] != float64(0) || len(columns) != 5 ||
		out["returned"] != float64(len(rows)) || len(si) != len(rows) || out["source"] == nil || out["notes"] == nil || out["guide"] != nil || out["lookup"] != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	// The default page is 20 rows (the 6,000 bytes of the default budget may allow fewer), and the answer says where
	// to continue.
	if n := len(rows); n < 5 || n > 20 || out["truncated"] != true || out["next_offset"] != float64(len(rows)) {
		t.Errorf("%d rows, truncated %v, next_offset %v", len(rows), out["truncated"], out["next_offset"])
	}
	first := rows[0].([]any)
	if first[0] != "4/0" || first[1] != "-3" || first[2] != "11.68 mm" || first[3] != "107.2 mm²" {
		t.Errorf("first row = %v", first)
	}
	// The next page starts where the first stopped.
	next := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg", Offset: len(rows), Limit: 3}))
	nrows, _ := next["rows"].([]any)
	if next["offset"] != float64(len(rows)) || len(nrows) != 3 || nrows[0].([]any)[0] == "4/0" {
		t.Errorf("second page = %v", next)
	}
}

func TestECalcTableFiltersAndLooksUp(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg", Filter: "/0"}))
	rows, _ := out["rows"].([]any)
	if out["total"] != float64(4) || out["returned"] != float64(4) || len(rows) != 4 || out["truncated"] != nil || rows[3].([]any)[0] != "1/0" {
		t.Fatalf("filter /0 = %v", out)
	}
	// A lookup alone answers the lookup and lists no rows.
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg", Lookup: &ecalc.TableLookup{Column: "awg", Value: "10", Output: "area"}})
	out = ecalcDecode(t, exec)
	lookup, _ := out["lookup"].(map[string]any)
	if out["status"] != "ok" || out["rows"] != nil || out["returned"] != float64(0) || lookup["found"] != true || lookup["mode"] != "exact" || lookup["result"] != "5.261 mm²" || lookup["value"] != "10" ||
		lookup["si"] == nil || len(lookup["row"].([]any)) != 5 {
		t.Fatalf("lookup = %s", exec.Output)
	}
	// With a filter, rows come too.
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg", Filter: "4/0", Lookup: &ecalc.TableLookup{Mode: "ge", Column: "area", Value: "3", Output: "size"}}))
	if rows, _ = out["rows"].([]any); len(rows) != 1 || out["lookup"].(map[string]any)["result"] != "12" {
		t.Errorf("filter and lookup = %v", out)
	}
	// The mistakes of a call say what to change.
	for _, c := range []struct {
		req  ECalcRequest
		code string
	}{
		{ECalcRequest{Operation: "table", ID: "nope"}, "unknown_id"},
		{ECalcRequest{Operation: "table"}, "unknown_id"},
		{ECalcRequest{Operation: "table", ID: "voltage_divider"}, "not_a_table"},
		{ECalcRequest{Operation: "table", ID: "awg", Filter: strings.Repeat("x", 129)}, "invalid_filter"},
		{ECalcRequest{Operation: "table", ID: "awg", Lookup: &ecalc.TableLookup{Column: "nope", Value: "1"}}, "unknown_column"},
		{ECalcRequest{Operation: "table", ID: "awg", Lookup: &ecalc.TableLookup{Column: "awg", Value: "1", Mode: "near"}}, "invalid_lookup"},
		{ECalcRequest{Operation: "table", ID: "awg", Lookup: &ecalc.TableLookup{Column: "awg", Value: "abc"}}, "parse_error"},
	} {
		if got := ecalcDecode(t, ExecuteECalc(nil, c.req)); got["status"] != "error" || got["code"] != c.code {
			t.Errorf("%+v: output = %v, want code %s", c.req, got, c.code)
		}
	}
}

func TestECalcTableShortensToTheBudget(t *testing.T) {
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	req := ECalcRequest{Operation: "table", ID: "awg", Limit: 50, Filter: " ", Lookup: &ecalc.TableLookup{Column: "awg", Value: "10", Output: "area"}}
	req.Limit = 50
	full := ecalcTable(cat, req, 100000)
	fullOut := ecalcDecode(t, full)
	if fullOut["returned"] != float64(44) || fullOut["si"] == nil || fullOut["note"] != nil {
		t.Fatalf("a full answer lists every row with its si values: %s", full.Output)
	}
	// The si matrix goes first, the rows stay.
	noSI := ecalcTable(cat, req, len(full.Output)-1)
	out := ecalcDecode(t, noSI)
	if out["si"] != nil || out["returned"] != float64(44) || !strings.Contains(out["note"].(string), "si values left out") || len(noSI.Output) >= len(full.Output) {
		t.Fatalf("si must go first: %s", noSI.Output)
	}
	// Then fewer rows, and the answer says where to continue; the lookup stays.
	short := ecalcTable(cat, req, 2400)
	out = ecalcDecode(t, short)
	rows, _ := out["rows"].([]any)
	if len(short.Output) > 2400 || len(rows) < 1 || len(rows) >= 44 || out["returned"] != float64(len(rows)) || out["truncated"] != true || out["next_offset"] != float64(len(rows)) ||
		out["lookup"].(map[string]any)["result"] != "5.261 mm²" || out["total"] != float64(44) {
		t.Fatalf("%d bytes: %s", len(short.Output), short.Output)
	}
	// Whatever the budget, one row is kept.
	tiny := ecalcDecode(t, ecalcTable(cat, req, 100))
	if rows, _ = tiny["rows"].([]any); len(rows) != 1 || tiny["returned"] != float64(1) {
		t.Errorf("tiny = %v", tiny)
	}
}

func TestECalcOpenWorksForTablesAndCodes(t *testing.T) {
	code := ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open", ID: "resistor_color_code"})
	if payload, _ := json.Marshal(code.Event.Payload); string(payload) != `{"app_id":"ecalc","ecalc":{"choices":{},"page":"resistor_color_code","values":{}}}` || !strings.Contains(code.Output, "Use decode and encode") {
		t.Fatalf("code page = %s", code.Output)
	}
	exec := ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open", ID: "AWG"})
	if exec.Event == nil || exec.Event.Type != "open_app" || exec.Event.ID == "" {
		t.Fatalf("event = %+v, output = %s", exec.Event, exec.Output)
	}
	payload, _ := json.Marshal(exec.Event.Payload)
	if string(payload) != `{"app_id":"ecalc","ecalc":{"choices":{},"page":"awg","values":{}}}` {
		t.Fatalf("payload = %s", payload)
	}
	out := ecalcDecode(t, exec)
	if out["status"] != "ok" || out["page"] != "awg" || !strings.Contains(out["message"].(string), "ECalc at the AWG wire gauges table. Use table to read its rows yourself") {
		t.Fatalf("output = %s", exec.Output)
	}
	// Values and choices belong to calculators.
	exec = ExecuteECalc(ecalcDesktopConfig(), ECalcRequest{Operation: "open", ID: "awg", Inputs: ecalcPairs("awg", "10")})
	if got := ecalcDecode(t, exec); exec.Event != nil || got["status"] != "error" || got["code"] != "not_a_calculator" || !strings.Contains(got["message"].(string), "awg is a table") {
		t.Fatalf("output = %s", exec.Output)
	}
	// The gates are the same as for a calculator.
	denied := ExecuteECalc(nil, ECalcRequest{Operation: "open", ID: "awg"})
	if denied.Event != nil || ecalcDecode(t, denied)["status"] != "policy_denied" {
		t.Fatalf("no config must be denied: %s", denied.Output)
	}
}

func TestECalcDecodeReadsAComponentCode(t *testing.T) {
	exec := ExecuteECalc(nil, ECalcRequest{Operation: "decode", Kind: "resistor_bands", Code: "violet green yellow gold"})
	if exec.Event != nil {
		t.Fatal("decode must not emit a desktop event")
	}
	out := ecalcDecode(t, exec)
	fields, _ := out["fields"].([]any)
	if out["status"] != "ok" || out["operation"] != "decode" || out["kind"] != "resistor_bands" || out["code"] != "violet green yellow gold" || out["value"] != float64(750000) || out["dim"] != "resistance" ||
		out["formatted"] != "750 kΩ" || len(out["bands"].([]any)) != 4 || len(fields) != 1 || out["warnings"] != nil {
		t.Fatalf("output = %s", exec.Output)
	}
	if f := fields[0].(map[string]any); f["id"] != "tolerance" || f["text"] != "±5 %" || f["value"] != 0.05 || f["dim"] != "ratio" {
		t.Errorf("tolerance field = %v", f)
	}
	// The id of a code item stands for its kind, and a marking is read as printed.
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "decode", Kind: "capacitor_code", Code: "104K 1H"}))
	if out["status"] != "ok" || out["kind"] != "capacitor" || out["formatted"] != "100 nF" || out["bands"] != nil || out["warnings"] == nil {
		t.Errorf("104K 1H = %v", out)
	}
	// A class 2 ceramic code has no value: the answer leaves it out instead of a null.
	exec = ExecuteECalc(nil, ECalcRequest{Operation: "decode", Kind: "ceramic_temp", Code: "X7R"})
	if out = ecalcDecode(t, exec); out["status"] != "ok" || out["value"] != nil || out["formatted"] != nil || out["dim"] != nil || strings.Contains(exec.Output, "null") {
		t.Errorf("X7R = %s", exec.Output)
	}
	if out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "decode", Kind: "ceramic_temp", Code: "U2J"})); out["formatted"] != "-750 ppm/K" {
		t.Errorf("U2J = %v", out)
	}
}

func TestECalcEncodeWritesAComponentCode(t *testing.T) {
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "encode", Kind: "resistor_bands", Value: "249k", Tolerance: "1%"}))
	if out["status"] != "ok" || out["operation"] != "encode" || out["code"] != "red yellow white orange brown" || out["formatted"] != "249 kΩ" || len(out["bands"].([]any)) != 5 {
		t.Fatalf("output = %v", out)
	}
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "encode", Kind: "capacitor", Value: "100n", Tolerance: "K", Voltage: "50V"}))
	if out["code"] != "104K 1H" || out["formatted"] != "100 nF" || len(out["warnings"].([]any)) != 1 {
		t.Errorf("capacitor = %v", out)
	}
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "encode", Kind: "inductor_bands", Value: "6.8u", Tolerance: "10%", Bands: 5}))
	if out["code"] != "silver blue gold grey silver" || out["formatted"] != "6.8 µH" {
		t.Errorf("inductor = %v", out)
	}
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "encode", Kind: "ceramic_temp", Class: "2", Value: "-55..125", Tolerance: "15%"}))
	if out["code"] != "X7R" || out["value"] != nil {
		t.Errorf("ceramic = %v", out)
	}
}

func TestECalcDecodeAndEncodeRefuseWhatTheyCannotRead(t *testing.T) {
	for _, c := range []struct {
		req  ECalcRequest
		code string
	}{
		{ECalcRequest{Operation: "decode", Kind: "nope", Code: "103"}, "unknown_code_kind"},
		{ECalcRequest{Operation: "decode", Kind: "capacitor"}, "invalid_code"},
		{ECalcRequest{Operation: "decode", Kind: "capacitor", Code: strings.Repeat("1", 65)}, "invalid_code"},
		{ECalcRequest{Operation: "decode", Kind: "resistor_bands", Code: "gold red yellow"}, "invalid_code"},
		{ECalcRequest{Operation: "decode", Kind: "smd_resistor", Code: "01S"}, "unsupported"},
		{ECalcRequest{Operation: "encode", Kind: "nope", Value: "4k7"}, "unknown_code_kind"},
		{ECalcRequest{Operation: "encode", Kind: "resistor_bands", Value: "abc"}, "invalid_value"},
		{ECalcRequest{Operation: "encode", Kind: "resistor_bands", Value: "4k7", Bands: 7}, "invalid_value"},
		{ECalcRequest{Operation: "encode", Kind: "capacitor"}, "invalid_value"},
		{ECalcRequest{Operation: "encode", Kind: "inductor_bands", Value: "6.8u", Tolerance: "2%"}, "unsupported"},
	} {
		exec := ExecuteECalc(nil, c.req)
		if got := ecalcDecode(t, exec); exec.Event != nil || got["status"] != "error" || got["code"] != c.code || got["message"] == "" {
			t.Errorf("%+v: output = %v, want code %s", c.req, got, c.code)
		}
	}
}

// The last row of the IPC-2221 spacing tables holds millimetres per volt in the same length columns as the spacings.
// It has no voltage band, so no lookup on a voltage may reach it: a lookup that reached it answered 0.005 mm for 500 V
// (the right spacing is 2.5 mm). This test names the two content tables because the defect lived in their data.
func TestECalcSpacingTablesNeverAnswerWithThePerVoltRow(t *testing.T) {
	lookup := func(id string, lk ecalc.TableLookup) map[string]any {
		t.Helper()
		out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: id, Lookup: &lk}))
		if out["status"] != "ok" {
			t.Fatalf("%s %+v: %v", id, lk, out)
		}
		l, _ := out["lookup"].(map[string]any)
		if l == nil {
			t.Fatalf("%s %+v: no lookup in %v", id, lk, out)
		}
		return l
	}
	for _, id := range []string{"ipc2221b_conductor_spacing", "ipc2221c_conductor_spacing"} {
		// 500 on the key column, ge and interp over the start of the band: nothing, never the per-volt value.
		for name, lk := range map[string]ecalc.TableLookup{
			"exact 500":        {Value: "500", Output: "b2"},
			"exact 500 V":      {Column: "v_min", Value: "500 V", Output: "b2"},
			"ge v_min 400":     {Mode: "ge", Column: "v_min", Value: "400", Output: "b2"},
			"interp v_min":     {Mode: "interp", Column: "v_min", Value: "400", Output: "b2"},
			"interp v_min 600": {Mode: "interp", Column: "v_min", Value: "600", Output: "b1"},
		} {
			if l := lookup(id, lk); l["found"] != false || l["result"] != nil || l["row"] != nil || l["message"] == nil {
				t.Errorf("%s %s = %v, want found:false", id, name, l)
			}
		}
		// 400 V belongs to the 301 to 500 V band: B2 = 2.5 mm.
		if l := lookup(id, ecalc.TableLookup{Mode: "ge", Column: "v_max", Value: "400", Output: "b2"}); l["found"] != true || l["result"] != "2.5 mm" {
			t.Errorf("%s ge v_max 400 = %v, want 2.5 mm", id, l)
		}
		// The band is the key and reads as text.
		if l := lookup(id, ecalc.TableLookup{Value: "301–500 V", Output: "b2"}); l["found"] != true || l["result"] != "2.5 mm" {
			t.Errorf("%s exact band = %v, want 2.5 mm", id, l)
		}
		// The per-volt row is reached by its marker and by its text key, and it says that it is a per-volt row.
		l := lookup(id, ecalc.TableLookup{Mode: "ge", Column: "per_volt", Value: "1", Output: "b1"})
		row, _ := l["row"].([]any)
		if l["found"] != true || len(row) == 0 || !strings.Contains(row[0].(string), "per volt") {
			t.Errorf("%s per_volt marker = %v", id, l)
		}
	}
}

// A transcribed cell is a published number: the answer keeps its digits (434.79 MHz, not 434.8 MHz, which lies outside
// the band), also when the answer is shortened and the si numbers are the first thing to go. The filter finds the number
// as it is written. A generated table keeps its four significant digits.
func TestECalcTableKeepsTheDigitsOfTranscribedCells(t *testing.T) {
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	req := ECalcRequest{Operation: "table", ID: "ism_bands", Filter: "434.79"}
	full := ecalcTable(cat, req, 100000)
	for name, exec := range map[string]VirtualDesktopExecution{
		"default":  ExecuteECalc(nil, req),
		"full":     full,
		"no si":    ecalcTable(cat, req, len(full.Output)-1),
		"lookup":   ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "ism_bands", Lookup: &ecalc.TableLookup{Column: "band", Value: "ISM 433.92 MHz", Output: "f_high"}}),
		"lookupGE": ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "ism_bands", Lookup: &ecalc.TableLookup{Mode: "ge", Column: "f_high", Value: "434.79 MHz", Output: "band"}}),
	} {
		out := ecalcDecode(t, exec)
		if out["status"] != "ok" || !strings.Contains(exec.Output, `"434.79 MHz"`) || strings.Contains(exec.Output, `"434.8 MHz"`) {
			t.Errorf("%s: the band edge must read 434.79 MHz: %s", name, exec.Output)
		}
	}
	if out := ecalcDecode(t, ecalcTable(cat, req, len(full.Output)-1)); out["si"] != nil || out["total"] == float64(0) {
		t.Errorf("the shortened answer must have dropped si and still found the rows: %v", out)
	}
	// The other edges of the issue: 2.4835 GHz, 868.7 to 869.2 MHz, 18.168 MHz.
	out := ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "ism_bands", Filter: "2400", Limit: 50}))
	if !strings.Contains(fmt.Sprint(out["rows"]), "2.4835 GHz") {
		t.Errorf("SRD 2400-2483.5 MHz: %v", out["rows"])
	}
	out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "ism_bands", Filter: "868.7"}))
	if rows := fmt.Sprint(out["rows"]); !strings.Contains(rows, "868.7 MHz") || !strings.Contains(rows, "869.2 MHz") || strings.Contains(rows, "869 MHz") {
		t.Errorf("SRD 868.7-869.2 MHz: %v", out["rows"])
	}
	// The filter finds the full text, not the rounded one.
	if out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "amateur_bands_de", Filter: "18.168"})); out["total"] != float64(1) || !strings.Contains(fmt.Sprint(out["rows"]), "18.168 MHz") {
		t.Errorf("amateur band 17 m: %v", out)
	}
	// A generated table is computed, not published: four significant digits.
	if out = ecalcDecode(t, ExecuteECalc(nil, ECalcRequest{Operation: "table", ID: "awg", Filter: "4/0", Limit: 1})); !strings.Contains(fmt.Sprint(out["rows"]), "11.68 mm") {
		t.Errorf("awg 4/0: %v", out["rows"])
	}
}
