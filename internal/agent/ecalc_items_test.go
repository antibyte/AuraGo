package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

// The tables and codes of the ecalc tool as the agent sees them: how the arguments of table, decode and encode are read
// and that the dispatcher runs them without the desktop.

func TestECalcNativeCallDecodesTableArguments(t *testing.T) {
	tc := ecalcNativeCall(t, `{"operation":"table","id":"awg","filter":"1/0","offset":4,"limit":7,"lookup":{"column":"awg","value":"10","mode":"GE","output":"area"}}`)
	req := decodeECalcArgs(tc)
	if req.Operation != "table" || req.ID != "awg" || req.Filter != "1/0" || req.Offset != 4 || req.Limit != 7 {
		t.Fatalf("request = %+v", req)
	}
	if l := req.Lookup; l == nil || l.Column != "awg" || l.Value != "10" || l.Mode != "ge" || l.Output != "area" {
		t.Fatalf("lookup = %+v", req.Lookup)
	}
	// The shapes models send for the lookup instead of the object.
	for name, arguments := range map[string]string{
		"number value": `{"operation":"table","id":"awg","lookup":{"column":"awg","value":10,"output":"area"}}`,
		"JSON text":    `{"operation":"table","id":"awg","lookup":"{\"column\":\"awg\",\"value\":\"10\",\"output\":\"area\"}"}`,
	} {
		got := decodeECalcArgs(ecalcNativeCall(t, arguments)).Lookup
		if got == nil || got.Column != "awg" || got.Value != "10" || got.Output != "area" || got.Mode != "" {
			t.Errorf("%s: lookup = %+v", name, got)
		}
	}
	// A lookup without a usable shape is an empty lookup, not no lookup: the tool then asks for the value.
	for name, arguments := range map[string]string{
		"text":  `{"operation":"table","id":"awg","lookup":"ten"}`,
		"array": `{"operation":"table","id":"awg","lookup":[1,2]}`,
		"empty": `{"operation":"table","id":"awg","lookup":{}}`,
	} {
		if got := decodeECalcArgs(ecalcNativeCall(t, arguments)).Lookup; got == nil || got.Value != "" || got.Column != "" {
			t.Errorf("%s: lookup = %+v, want an empty lookup", name, got)
		}
	}
	if got := decodeECalcArgs(ecalcNativeCall(t, `{"operation":"table","id":"awg"}`)); got.Lookup != nil || got.Filter != "" || got.Offset != 0 {
		t.Errorf("a plain table call = %+v", got)
	}
	// Text-mode calls carry the generic fields.
	req = decodeECalcArgs(ToolCall{Action: "ecalc", Operation: "Table", ID: "awg", Offset: 3, Limit: 2})
	if req.Operation != "table" || req.Offset != 3 || req.Limit != 2 {
		t.Fatalf("text-mode request = %+v", req)
	}
}

func TestECalcNativeCallDecodesCodeArguments(t *testing.T) {
	req := decodeECalcArgs(ecalcNativeCall(t, `{"operation":"decode","kind":"Resistor_Bands","code":"brown black red gold"}`))
	if req.Operation != "decode" || req.Kind != "resistor_bands" || req.Code != "brown black red gold" {
		t.Fatalf("decode request = %+v", req)
	}
	req = decodeECalcArgs(ecalcNativeCall(t, `{"operation":"encode","kind":"capacitor","value":"100 nF","bands":5,"tolerance":"1%","tempco":"50 ppm/K","voltage":"63 V","class":"X7R"}`))
	if req.Operation != "encode" || req.Kind != "capacitor" || req.Value != "100 nF" || req.Bands != 5 || req.Tolerance != "1%" || req.TempCo != "50 ppm/K" || req.Voltage != "63 V" || req.Class != "X7R" {
		t.Fatalf("encode request = %+v", req)
	}
	// Numbers and numbers as text are read as typed.
	req = decodeECalcArgs(ecalcNativeCall(t, `{"operation":"encode","kind":"resistor_bands","value":"4k7","bands":"6","tolerance":5,"tempco":50,"voltage":16}`))
	if req.Bands != 6 || req.Tolerance != "5" || req.TempCo != "50" || req.Voltage != "16" {
		t.Fatalf("lenient encode request = %+v", req)
	}
	// Text-mode calls carry code and value in the generic fields.
	req = decodeECalcArgs(ToolCall{Action: "ecalc", Operation: "decode", Code: "103", Value: "x"})
	if req.Code != "103" || req.Value != "x" {
		t.Fatalf("text-mode request = %+v", req)
	}
}

// A text-mode call (a JSON object in the reply instead of a native tool call) reaches the same request.
func TestECalcTextModeCallsDecodeLikeNativeOnes(t *testing.T) {
	tc := ParseToolCall(`{"action":"ecalc","operation":"table","id":"awg","filter":"1/0","offset":2,"limit":5,"lookup":{"column":"awg","value":"10","output":"area"}}`)
	req := decodeECalcArgs(tc)
	if tc.Action != "ecalc" || req.Operation != "table" || req.ID != "awg" || req.Filter != "1/0" || req.Offset != 2 || req.Limit != 5 || req.Lookup == nil || req.Lookup.Column != "awg" || req.Lookup.Output != "area" {
		t.Fatalf("table request = %+v (lookup %+v)", req, req.Lookup)
	}
	tc = ParseToolCall(`{"action":"ecalc","operation":"encode","kind":"resistor_bands","value":"4k7","bands":5,"tolerance":"1%"}`)
	req = decodeECalcArgs(tc)
	if req.Operation != "encode" || req.Kind != "resistor_bands" || req.Value != "4k7" || req.Bands != 5 || req.Tolerance != "1%" {
		t.Fatalf("encode request = %+v", req)
	}
	tc = ParseToolCall(`{"action":"ecalc","operation":"decode","kind":"smd_resistor","code":"103"}`)
	if req = decodeECalcArgs(tc); req.Code != "103" || req.Kind != "smd_resistor" {
		t.Fatalf("decode request = %+v", req)
	}
}

func TestECalcDispatchRunsTableCallsWithoutTheDesktop(t *testing.T) {
	broker := &ecalcJSONBroker{}
	dc := &DispatchContext{Cfg: &config.Config{}, Logger: testLogger, Broker: broker}
	call := ecalcNativeCall(t, `{"operation":"table","id":"awg","lookup":{"column":"awg","value":"10","output":"area"}}`)
	out, ok := dispatchPlatform(context.Background(), call, dc)
	if !ok || !strings.HasPrefix(out, "Tool Output: ") || !strings.Contains(out, `"status":"ok"`) || !strings.Contains(out, `"result":"5.261 mm²"`) {
		t.Fatalf("dispatch = %v %s", ok, out)
	}
	if len(broker.json) != 0 {
		t.Fatalf("table must not broadcast: %v", broker.json)
	}
	// The answer is sized to the inline budget of the caller.
	call = ecalcNativeCall(t, `{"operation":"table","id":"eseries_e192","limit":50}`)
	out, _ = dispatchPlatform(context.Background(), call, dc)
	if !strings.Contains(out, `"status":"ok"`) || !strings.Contains(out, `"truncated":true`) || len(out) > toolResultInlineBudget(dc) {
		t.Fatalf("%d bytes (budget %d): %.200s", len(out), toolResultInlineBudget(dc), out)
	}
}

func TestECalcSearchTermsFindTablesAndCodes(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, ToolFeatureFlags{}, nil), nil, "")
	for _, query := range []string{"AWG wire gauge table", "cable cross-section table", "resistor color code", "Widerstand Farbcode", "SMD resistor marking", "Kabelquerschnitt Tabelle"} {
		matches := catalog.Search(query)
		found := false
		for _, entry := range matches[:min(5, len(matches))] {
			found = found || entry.Name == "ecalc"
		}
		if !found {
			t.Errorf("%q did not find ecalc in the top five", query)
		}
	}
}
