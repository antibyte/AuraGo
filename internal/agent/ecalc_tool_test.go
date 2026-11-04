package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ecalc"

	openai "github.com/sashabaranov/go-openai"
)

type ecalcJSONBroker struct {
	captureBroker
	mu   sync.Mutex
	json []string
}

func (b *ecalcJSONBroker) SendJSON(s string) {
	b.mu.Lock()
	b.json = append(b.json, s)
	b.mu.Unlock()
}

func ecalcAgentConfig() *config.Config {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

func ecalcNativeCall(t *testing.T, arguments string) ToolCall {
	t.Helper()
	tc := NativeToolCallToToolCall(openai.ToolCall{ID: "call-1", Type: "function", Function: openai.FunctionCall{Name: "ecalc", Arguments: arguments}}, nil)
	if tc.NativeArgsMalformed {
		t.Fatalf("native arguments flagged malformed: %s (%s)", tc.NativeArgsError, arguments)
	}
	return tc
}

func TestECalcSchemaIsStrictCompactAndNamedSafely(t *testing.T) {
	s := ecalcSchema()
	if s.Function.Name != "ecalc" {
		t.Fatalf("name = %q", s.Function.Name)
	}
	params := s.Function.Parameters.(map[string]interface{})
	props := params["properties"].(map[string]interface{})
	if got := props["operation"].(map[string]interface{})["enum"]; !slices.Equal(got.([]string), []string{"search", "describe", "calculate", "open", "table", "pinout", "decode", "encode"}) {
		t.Fatalf("operations = %v", got)
	}
	// The code kinds of the engine are the enum of kind, so the schema cannot drift from them.
	if got := props["kind"].(map[string]interface{})["enum"]; len(ecalc.CodeKindIDs()) < 5 || !slices.Equal(got.([]string), ecalc.CodeKindIDs()) {
		t.Fatalf("kinds = %v, want %v", got, ecalc.CodeKindIDs())
	}
	// The lookup of a table call is a strict object: concrete properties, nothing else allowed.
	lookup := props["lookup"].(map[string]interface{})
	lookupProps := lookup["properties"].(map[string]interface{})
	if lookup["type"] != "object" || lookup["additionalProperties"] != false || !slices.Equal(lookup["required"].([]string), []string{"value"}) || len(lookupProps) != 4 ||
		!slices.Equal(lookupProps["mode"].(map[string]interface{})["enum"].([]string), []string{"exact", "ge", "interp"}) {
		t.Fatalf("lookup = %v", lookup)
	}
	// The parameters that share a name with a field of ToolCall have its type (a string or an integer), so a call with
	// them is never flagged malformed.
	for name, typ := range map[string]string{"code": "string", "value": "string", "offset": "integer", "limit": "integer", "filter": "string", "bands": "integer", "pin": "string", "variant": "string"} {
		if got := props[name].(map[string]interface{})["type"]; got != typ {
			t.Errorf("%s has type %v, want %v", name, got, typ)
		}
	}
	if got := props["eseries"].(map[string]interface{})["enum"]; !slices.Equal(got.([]string), []string{"E3", "E6", "E12", "E24", "E48", "E96", "E192"}) {
		t.Fatalf("series = %v", got)
	}
	if required := params["required"].([]string); !slices.Equal(required, []string{"operation"}) {
		t.Fatalf("required = %v", required)
	}
	for _, key := range []string{"inputs", "choices"} {
		items := props[key].(map[string]interface{})["items"].(map[string]interface{})
		if items["additionalProperties"] != false || len(items["properties"].(map[string]interface{})) != 2 {
			t.Fatalf("%s items = %v", key, items)
		}
	}
	// ToolCall owns "values" for spreadsheets (an array of arrays), so an array of objects under that name would
	// mark every call malformed. The variable values travel as inputs.
	if _, clash := props["values"]; clash {
		t.Fatal("the schema must not use the parameter name values")
	}
	// A name with one of these words is read as a network destination by the action policy.
	for name := range props {
		for _, word := range []string{"url", "uri", "link", "host", "domain", "endpoint"} {
			if strings.Contains(strings.ToLower(name), word) {
				t.Errorf("parameter %q contains %q", name, word)
			}
		}
	}
	// 450 tokens held the four calculator operations; table, decode and encode add three operations and eleven
	// parameters, pinout one operation and two parameters (755 tokens before it, 799 with it). The manual carries the
	// details, so the schema keeps to names, types and one short hint each.
	if got := estimateSingleToolSchemaTokens(s); got > 800 {
		t.Fatalf("the ecalc schema is %d tokens, keep it at most 800", got)
	}
}

func TestECalcIsAlwaysOnAndFoundThroughDiscovery(t *testing.T) {
	if !slices.Contains(builtinToolNames(ToolFeatureFlags{}), "ecalc") {
		t.Fatal("ecalc needs no switch: it must be in the schemas of the zero feature flags")
	}
	if got := ToolCategoryForName("ecalc"); got != "infrastructure" {
		t.Fatalf("category = %q", got)
	}
	if terms := toolSearchTermsFor("ecalc"); len(terms) < 6 || !slices.Contains(terms, "electronics calculator") || !slices.Contains(terms, "Elektronik Rechner") {
		t.Fatalf("search terms = %q", terms)
	}
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, ToolFeatureFlags{}, nil), nil, "")
	for _, query := range []string{"electronics calculator", "Spannungsteiler berechnen", "LED Vorwiderstand", "ohms law"} {
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

func TestECalcNativeCallDecodesInputsAndChoices(t *testing.T) {
	tc := ecalcNativeCall(t, `{"operation":"calculate","id":"rms_waveform","inputs":[{"name":"vrms","value":"230 V"},{"name":"vpk","value":325}],"choices":[{"name":"wave","option":"triangle"}],"eseries":"E96","limit":5,"query":"x"}`)
	req := decodeECalcArgs(tc)
	if req.Operation != "calculate" || req.ID != "rms_waveform" || req.Query != "x" || req.ESeries != "E96" || req.Limit != 5 {
		t.Fatalf("request = %+v", req)
	}
	if len(req.Inputs) != 2 || req.Inputs[0].Name != "vrms" || req.Inputs[0].Value != "230 V" || req.Inputs[1].Value != "325" {
		t.Fatalf("inputs = %+v", req.Inputs)
	}
	if len(req.Choices) != 1 || req.Choices[0].Name != "wave" || req.Choices[0].Value != "triangle" {
		t.Fatalf("choices = %+v", req.Choices)
	}
	// The shapes models send instead of the array of pairs.
	for name, arguments := range map[string]string{
		"object":      `{"operation":"calculate","id":"x","inputs":{"vrms":"230 V"},"choices":{"wave":"sine"}}`,
		"JSON text":   `{"operation":"calculate","id":"x","inputs":"[{\"name\":\"vrms\",\"value\":\"230 V\"}]","choices":"{\"wave\":\"sine\"}"}`,
		"value alias": `{"operation":"calculate","id":"x","inputs":[{"name":"vrms","value":"230 V"}],"choices":[{"name":"wave","value":"sine"}]}`,
	} {
		got := decodeECalcArgs(ecalcNativeCall(t, arguments))
		if len(got.Inputs) != 1 || got.Inputs[0].Name != "vrms" || got.Inputs[0].Value != "230 V" || len(got.Choices) != 1 || got.Choices[0].Value != "sine" {
			t.Errorf("%s: %+v", name, got)
		}
	}
	// Text-mode calls carry the generic fields.
	req = decodeECalcArgs(ToolCall{Action: "ecalc", Operation: " Search ", Query: "ohm", Limit: 3, ID: "ohms_law"})
	if req.Operation != "search" || req.Query != "ohm" || req.Limit != 3 || req.ID != "ohms_law" {
		t.Fatalf("text-mode request = %+v", req)
	}
}

func TestECalcDispatchReadsNeedNoDesktop(t *testing.T) {
	dc := &DispatchContext{Cfg: &config.Config{}, Logger: testLogger, Broker: &ecalcJSONBroker{}}
	call := ecalcNativeCall(t, `{"operation":"calculate","id":"voltage_divider","inputs":[{"name":"vin","value":"12 V"},{"name":"r1","value":"4k7"},{"name":"r2","value":"2k2"}]}`)
	out, ok := dispatchPlatform(context.Background(), call, dc)
	if !ok || !strings.HasPrefix(out, "Tool Output: ") || !strings.Contains(out, `"status":"ok"`) || !strings.Contains(out, `"formatted":"3.826 V"`) {
		t.Fatalf("dispatch = %v %s", ok, out)
	}
	broker := dc.Broker.(*ecalcJSONBroker)
	if len(broker.json) != 0 {
		t.Fatalf("calculate must not broadcast: %v", broker.json)
	}
}

func TestECalcDispatchOpenBroadcastsOpenApp(t *testing.T) {
	broker := &ecalcJSONBroker{}
	dc := &DispatchContext{Cfg: ecalcAgentConfig(), Logger: testLogger, Broker: broker}
	call := ecalcNativeCall(t, `{"operation":"open","id":"voltage_divider","inputs":[{"name":"vin","value":"12 V"},{"name":"r1","value":"4k7"}]}`)
	out, ok := dispatchPlatform(context.Background(), call, dc)
	if !ok || !strings.HasPrefix(out, "Tool Output: ") || !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("dispatch = %v %s", ok, out)
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if len(broker.json) != 1 {
		t.Fatalf("broadcasts = %d", len(broker.json))
	}
	var msg struct {
		Type    string `json:"type"`
		Payload struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Payload struct {
				AppID string `json:"app_id"`
				ECalc struct {
					Page    string            `json:"page"`
					Values  map[string]string `json:"values"`
					Choices map[string]string `json:"choices"`
				} `json:"ecalc"`
			} `json:"payload"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(broker.json[0]), &msg); err != nil {
		t.Fatal(err)
	}
	p := msg.Payload
	if msg.Type != "virtual_desktop_event" || p.Type != "open_app" || p.ID == "" || p.Payload.AppID != "ecalc" ||
		p.Payload.ECalc.Page != "voltage_divider" || p.Payload.ECalc.Values["vin"] != "12 V" || p.Payload.ECalc.Values["r1"] != "4k7" || len(p.Payload.ECalc.Choices) != 0 {
		t.Fatalf("broadcast = %s", broker.json[0])
	}
	// The order of the values is the order in the call.
	if !strings.Contains(broker.json[0], `"values":{"vin":"12 V","r1":"4k7"}`) {
		t.Fatalf("the values lost their order: %s", broker.json[0])
	}
}

func TestECalcDispatchOpenIsDeniedWhileTheDesktopGatesAreClosed(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"desktop off":            func(c *config.Config) { c.VirtualDesktop.Enabled = false },
		"agent control off":      func(c *config.Config) { c.VirtualDesktop.AllowAgentControl = false },
		"desktop tools disabled": func(c *config.Config) { c.Tools.VirtualDesktop.Enabled = false },
	} {
		cfg := ecalcAgentConfig()
		mutate(cfg)
		broker := &ecalcJSONBroker{}
		dc := &DispatchContext{Cfg: cfg, Logger: testLogger, Broker: broker}
		out, ok := dispatchPlatform(context.Background(), ecalcNativeCall(t, `{"operation":"open","id":"ohms_law"}`), dc)
		if !ok || !strings.Contains(out, `"status":"policy_denied"`) || len(broker.json) != 0 {
			t.Errorf("%s: %v %s broadcasts=%d", name, ok, out, len(broker.json))
		}
	}
}

// The runnable examples of the manual must be valid calls: known parameters, schema enums, and they run.
func TestECalcManualExamplesAreValidCalls(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "prompts", "tools_manuals", "ecalc.md"))
	if err != nil {
		t.Fatal(err)
	}
	examples := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindAllSubmatch(data, -1)
	if len(examples) < 4 {
		t.Fatalf("the manual has %d JSON examples, want at least 4", len(examples))
	}
	props := ecalcSchema().Function.Parameters.(map[string]interface{})["properties"].(map[string]interface{})
	seen := map[string]bool{}
	for _, example := range examples {
		var call map[string]any
		if err := json.Unmarshal(example[1], &call); err != nil {
			t.Fatalf("%v\n%s", err, example[1])
		}
		if call["action"] != "ecalc" {
			t.Fatalf("example for %v", call["action"])
		}
		for key, value := range call {
			if key == "action" {
				continue
			}
			property, ok := props[key].(map[string]interface{})
			if !ok {
				t.Fatalf("unknown parameter %q in %s", key, example[1])
			}
			if enum, ok := property["enum"].([]string); ok && !slices.Contains(enum, value.(string)) {
				t.Fatalf("%s=%v is not in %v", key, value, enum)
			}
		}
		seen[call["operation"].(string)] = true
		raw, _ := json.Marshal(call)
		tc := ecalcNativeCall(t, string(raw))
		// The pinout examples name a pinout of the shipped catalog; they run as soon as it has the item (the content comes
		// after the tool), so a typo in the id is caught then.
		if id, _ := call["id"].(string); id != "" && (call["operation"] == "pinout" || call["operation"] == "describe") {
			if cat, err := ecalc.Default(); err == nil && len(cat.Pinouts) == 0 {
				if _, known := cat.Item(id); !known {
					t.Logf("the catalog has no pinouts yet: the example %s is only checked against the schema", example[1])
					continue
				}
			}
		}
		out, ok := dispatchPlatform(context.Background(), tc, &DispatchContext{Cfg: ecalcAgentConfig(), Logger: testLogger, Broker: &ecalcJSONBroker{}})
		if !ok || !strings.Contains(out, `"status":"ok"`) {
			t.Errorf("the example does not run: %s\n%s", example[1], out)
		}
	}
	for _, op := range []string{"search", "describe", "calculate", "open", "table", "pinout", "decode", "encode"} {
		if !seen[op] {
			t.Errorf("the manual has no example for %s", op)
		}
	}
}
