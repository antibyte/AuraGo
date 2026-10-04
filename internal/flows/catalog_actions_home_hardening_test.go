package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Characters the tests need. They are named constants so a mangled escape sequence in
// an edit shows up in TestHomeTestConstants instead of silently turning a test into one
// that proves nothing.
const (
	homeLF  = "\n"
	homeCR  = "\r"
	homeNUL = "\x00"
	homeBad = "\xff"
)

func TestHomeTestConstants(t *testing.T) {
	if !reflect.DeepEqual([]byte(homeLF+homeCR+homeNUL+homeBad), []byte{10, 13, 0, 255}) {
		t.Fatal("an escape sequence in a test constant was mangled")
	}
}

// homeAnswers answer each tool of the smart home and planner nodes with a success of
// its own shape.
func homeAnswers(req ToolRequest) (ToolResponse, error) {
	reply := ""
	switch req.Tool {
	case "home_assistant":
		reply = `{"status":"success","service":"light.turn_on","affected_entities":["light.a"],"count":1}`
		if req.Args["operation"] == "get_state" {
			reply = `{"status":"success","entity":{"entity_id":"sensor.t","state":"1","attributes":{"unit":"C"},"last_changed":"2026-10-03T06:00:00Z"}}`
		}
	case "mqtt_publish":
		reply = `{"status":"success","message":"Published to topic 'a'","topic":"a","qos":0,"retained":false}`
	case "manage_appointments", "manage_todos":
		reply = `{"status":"success","message":"created","id":"id_1"}`
	}
	return ToolResponse{Output: "Tool Output: " + reply, Status: "success"}, nil
}

// homeBases are parameters that make each node succeed.
var homeBases = map[string]map[string]any{
	TypeHomeAssistant:  {"entity": "light.a", "service": "turn_on"},
	TypeMQTTPublish:    {"topic": "home/a", "payload": "x"},
	TypeAppointmentAdd: {"title": "t", "date_time": "2026-10-05 09:00"},
	TypeTodoAdd:        {"title": "t"},
}

// homeParams returns the base parameters of typ with over applied. A nil value in over
// sets the parameter to nil.
func homeParams(typ string, over map[string]any) map[string]any {
	params := map[string]any{}
	for k, v := range homeBases[typ] {
		params[k] = v
	}
	for k, v := range over {
		params[k] = v
	}
	return params
}

// homeRun executes a node against tools that always succeed. Times are read in UTC.
func homeRun(t *testing.T, typ string, over map[string]any) (*fakeTools, ExecResult, error) {
	t.Helper()
	return homeRunWith(t, typ, over, homeAnswers)
}

func homeRunWith(t *testing.T, typ string, over map[string]any, respond func(ToolRequest) (ToolResponse, error)) (*fakeTools, ExecResult, error) {
	t.Helper()
	tools := &fakeTools{respond: respond}
	res, err := execDef(lookupDef(t, homeRegistry(t), typ), homeParams(typ, over), &Services{Tools: tools, Location: time.UTC})
	return tools, res, err
}

func homeReply(output string) func(ToolRequest) (ToolResponse, error) {
	return toolReply("Tool Output: " + output)
}

// homeWantInvalid fails unless err is a FLOW_PARAM_INVALID raised before any tool call,
// with a short, clean message that does not echo the value (when the value is long
// enough to be telling).
func homeWantInvalid(t *testing.T, label string, tools *fakeTools, err error, echo string) {
	t.Helper()
	ne := asNodeError(err)
	switch {
	case ne == nil:
		t.Errorf("%s: accepted", label)
	case ne.Code != "FLOW_PARAM_INVALID":
		t.Errorf("%s: code %s: %.100s", label, ne.Code, ne.Message)
	case tools.count() != 0:
		t.Errorf("%s: the tool was called", label)
	case len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) || strings.ContainsAny(ne.Message, homeLF+homeCR+homeNUL):
		t.Errorf("%s: unclean or long message %.100q", label, ne.Message)
	case len(echo) >= 12 && strings.Contains(ne.Message, echo):
		t.Errorf("%s: the message echoes the value: %q", label, ne.Message)
	}
}

func homeEchoOf(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// homeExecRaw runs a node the way the engine does: the node holds the raw parameters,
// Execute gets the resolved ones.
func homeExecRaw(t *testing.T, typ string, raw, resolved map[string]any, tools *fakeTools) (ExecResult, error) {
	t.Helper()
	def := lookupDef(t, homeRegistry(t), typ)
	node := &Node{ID: testNodeID(1), Key: "node", Type: typ, Params: raw}
	return def.Execute(context.Background(), ExecInput{
		Node: node, Params: withDefaults(def, resolved), Services: &Services{Tools: tools},
		Run: RunInfo{ID: "run_test", FlowID: "flow_test", Mode: ModeTest},
	})
}

func homeDeep(levels int) any {
	var v any = "leaf"
	for i := 0; i < levels; i++ {
		v = map[string]any{"k": v}
	}
	return v
}

func TestHomeAssistantEntityIsValidated(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{"nil", nil}, {"empty", ""}, {"blank", "  "}, {"all", "all"}, {"list as text", "light.a,light.b"},
		{"upper case", "Light.A"}, {"no dot", "light"}, {"no object", "light."}, {"no domain", ".a"}, {"two dots", "light.a.b"},
		{"space", "light.a b"}, {"inner line break", "light." + homeLF + "a"}, {"nul", "light.a" + homeNUL}, {"invalid utf-8", "light.a" + homeBad},
		{"path", "light.a/../x"}, {"encoded", "light.a%2f"}, {"too long", "light." + strings.Repeat("a", maxHAEntityBytes-5)},
		{"number", 5.0}, {"bool", true}, {"list", []any{"light.a"}}, {"object", map[string]any{"entity_id": "light.a"}},
	}
	for _, op := range []string{"call_service", "get_state"} {
		for _, c := range cases {
			tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"operation": op, "entity": c.v})
			homeWantInvalid(t, op+" "+c.name, tools, err, homeEchoOf(c.v))
		}
	}
	// What the tool is given is exactly the id, trimmed.
	atLimit := "light." + strings.Repeat("a", maxHAEntityBytes-6)
	for _, entity := range []string{"light.a", "  light.living_room_2 ", "sensor.temp_3", "a.b", atLimit} {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"entity": entity})
		if err != nil || tools.last(t).Args["entity_id"] != strings.TrimSpace(entity) {
			t.Errorf("entity %.30q: %v", entity, err)
		}
	}
}

func TestHomeAssistantServiceIsValidated(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{"nil", nil}, {"empty", ""}, {"blank", " "}, {"upper case", "Turn_On"}, {"domain upper case", "Light.turn_on"}, {"space", "turn on"},
		{"hyphen", "turn-on"}, {"two dots", "light.turn_on.x"}, {"leading dot", ".turn_on"}, {"trailing dot", "light."}, {"slash", "light/turn_on"},
		{"semicolon", "turn_on;x"}, {"template", "{{trigger.data.service}}"}, {"template part", "light.{{x.y}}"}, {"nul", "turn_on" + homeNUL},
		{"inner line break", "turn" + homeLF + "on"}, {"invalid utf-8", "turn_on" + homeBad}, {"too long", strings.Repeat("a", maxHAServiceBytes+1)},
		{"list", []any{"turn_on"}}, {"object", map[string]any{"a": "b"}},
	}
	for _, c := range cases {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"service": c.v})
		homeWantInvalid(t, c.name, tools, err, homeEchoOf(c.v))
	}
	for service, want := range map[string]string{
		"turn_on": "light.turn_on", " turn_off ": "light.turn_off", "switch.toggle": "switch.toggle", "homeassistant.toggle": "homeassistant.toggle",
		"scene.turn_on": "scene.turn_on", "a1_b2.c3_d4": "a1_b2.c3_d4",
	} {
		tools, res, err := homeRun(t, TypeHomeAssistant, map[string]any{"service": service})
		if err != nil || res.Output["service"] != want {
			t.Errorf("service %q: %v %v", service, err, res.Output)
			continue
		}
		if args := tools.last(t).Args; fmt.Sprint(args["domain"])+"."+fmt.Sprint(args["service"]) != want {
			t.Errorf("service %q reached the tool as %v", service, args)
		}
	}
}

// What Home Assistant runs is chosen by the author of the document: a service that
// came out of a template is refused even though the run resolved it to a valid name,
// and Validate says so while the flow is edited.
func TestHomeAssistantServiceMustBeWrittenOut(t *testing.T) {
	tools := &fakeTools{respond: homeAnswers}
	raw := map[string]any{"entity": "light.a", "service": "{{trigger.data.service}}"}
	_, err := homeExecRaw(t, TypeHomeAssistant, raw, map[string]any{"entity": "light.a", "service": "turn_off"}, tools)
	if ne := asNodeError(err); ne == nil || ne.Code != "FLOW_PARAM_INVALID" || tools.count() != 0 {
		t.Fatalf("a templated service ran: %v after %d calls", err, tools.count())
	}
	// A literal service with a templated entity is fine, and runs.
	raw = map[string]any{"entity": "{{trigger.data.entity}}", "service": "light.turn_off"}
	if _, err := homeExecRaw(t, TypeHomeAssistant, raw, map[string]any{"entity": "light.b", "service": "light.turn_off"}, tools); err != nil || tools.count() != 1 {
		t.Fatalf("a templated entity: %v", err)
	}

	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	check := func(params map[string]any) []string {
		var got []string
		for _, is := range def.Validate(&Node{ID: testNodeID(1), Key: "n", Type: TypeHomeAssistant, Params: params}, ValidateContext{Mode: ModePublish}) {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || len(is.Message) > maxEchoMessageBytes {
				t.Errorf("bad issue %+v", is)
			}
			got = append(got, is.Param)
		}
		return got
	}
	for _, c := range []struct {
		name   string
		params map[string]any
		want   []string
	}{
		{"fine", map[string]any{"entity": "light.a", "service": "turn_on", "service_data": map[string]any{"a": 1.0}}, nil},
		{"templates where allowed", map[string]any{"entity": "{{x.e}}", "service": "turn_on", "service_data": "{{x.d}}"}, nil},
		{"templated service", map[string]any{"entity": "light.a", "service": "{{x.s}}"}, []string{"service"}},
		{"bad service", map[string]any{"entity": "light.a", "service": "Turn On"}, []string{"service"}},
		{"bad entity", map[string]any{"entity": "all", "service": "turn_on"}, []string{"entity"}},
		{"bad data", map[string]any{"entity": "light.a", "service": "turn_on", "service_data": "text"}, []string{"service_data"}},
		{"unknown operation", map[string]any{"operation": "toggle", "entity": "light.a", "service": "x"}, []string{"operation"}},
		{"templated operation", map[string]any{"operation": "{{x.o}}", "entity": "light.a", "service": "Bad"}, []string{"service"}},
		{"get_state ignores the service", map[string]any{"operation": "get_state", "entity": "light.a", "service": "Bad Service", "service_data": "x"}, nil},
		{"get_state still checks the entity", map[string]any{"operation": "get_state", "entity": "all"}, []string{"entity"}},
		{"empty values are the required check's", map[string]any{"entity": "", "service": "", "service_data": ""}, nil},
	} {
		if got := check(c.params); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: issues on %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHomeAssistantServiceData(t *testing.T) {
	big := map[string]any{"text": strings.Repeat("x", maxHAServiceDataBytes)}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	many := make([]any, maxHAServiceDataValues)
	for i := range many {
		many[i] = 1.0
	}
	bad := []struct {
		name string
		v    any
	}{
		{"text", "x"}, {"json text", `{"a":1}`}, {"number", 1.0}, {"bool", true}, {"list", []any{1.0}}, {"empty list is no object", []any{}},
		{"too large", big}, {"too deep", homeDeep(maxHAServiceDataDepth + 1)}, {"cyclic", cyclic}, {"too many values", map[string]any{"l": many}},
		{"NaN", map[string]any{"a": math.NaN()}}, {"func", map[string]any{"a": func() {}}},
	}
	for _, c := range bad {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"service_data": c.v})
		homeWantInvalid(t, c.name, tools, err, homeEchoOf(c.v))
	}
	for _, c := range []struct {
		name string
		v    any
	}{
		{"object", map[string]any{"brightness": 128.0, "rgb_color": []any{255.0, 0.0, 0.0}}},
		{"at the depth limit", homeDeep(maxHAServiceDataDepth)},
		{"nested", map[string]any{"data": map[string]any{"actions": []any{map[string]any{"action": "a", "title": "t"}}}}},
	} {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"service_data": c.v})
		if err != nil || !reflect.DeepEqual(tools.last(t).Args["service_data"], c.v) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// No data: the argument is left out, not sent empty.
	for _, v := range []any{nil, "", " ", map[string]any{}, map[string]any(nil)} {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"service_data": v})
		if _, has := tools.last(t).Args["service_data"]; err != nil || has {
			t.Errorf("service_data %#v: %v, sent %v", v, err, tools.last(t).Args)
		}
	}
	// A get_state that is written out ignores the service and its data.
	tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"operation": "get_state", "service": "Bad Service", "service_data": "x"})
	if err != nil || !reflect.DeepEqual(tools.last(t).Args, map[string]any{"operation": "get_state", "entity_id": "light.a"}) {
		t.Errorf("get_state with garbage in the hidden fields: %v %v", err, tools.last(t).Args)
	}
}

func TestHomeAssistantOperationIsStrict(t *testing.T) {
	for _, v := range []any{"get_states", "toggle", "service", "state", "call-service", "{{x.y}}", 5.0, true, []any{"get_state"}, map[string]any{}} {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"operation": v})
		homeWantInvalid(t, fmt.Sprintf("operation %v", v), tools, err, homeEchoOf(v))
	}
	for v, want := range map[string]string{"": "call_service", " ": "call_service", "GET_STATE": "get_state", " get_state ": "get_state", "Call_Service": "call_service"} {
		tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"operation": v})
		if err != nil || tools.last(t).Args["operation"] != want {
			t.Errorf("operation %q: %v %v", v, err, tools.last(t).Args)
		}
	}
	if tools, _, err := homeRun(t, TypeHomeAssistant, map[string]any{"operation": nil}); err != nil || tools.last(t).Args["operation"] != "call_service" {
		t.Errorf("a null operation is the default: %v", err)
	}
}

func TestHomeAssistantEffects(t *testing.T) {
	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	controls := []Effect{EffectControlsDevices}
	for _, c := range []struct {
		name string
		n    *Node
		want []Effect
	}{
		{"get_state", &Node{Params: map[string]any{"operation": "get_state"}}, nil},
		{"upper case with blanks", &Node{Params: map[string]any{"operation": " GET_STATE "}}, nil},
		{"default", &Node{Params: map[string]any{}}, controls},
		{"call_service", &Node{Params: map[string]any{"operation": "call_service"}}, controls},
		{"template", &Node{Params: map[string]any{"operation": "{{x.op}}"}}, controls},
		{"unknown word", &Node{Params: map[string]any{"operation": "get_states"}}, controls},
		{"wrong type", &Node{Params: map[string]any{"operation": []any{"get_state"}}}, controls},
		{"null", &Node{Params: map[string]any{"operation": nil}}, controls},
		{"no params", &Node{}, controls},
		{"no node", nil, controls},
	} {
		if got := def.EffectsOf(c.n); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: effects %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHomeAssistantGetState(t *testing.T) {
	run := func(reply string, entity string) (ExecResult, error) {
		_, res, err := homeRunWith(t, TypeHomeAssistant, map[string]any{"operation": "get_state", "entity": entity}, homeReply(reply))
		return res, err
	}
	res, err := run(`{"status":"success","entity":{"entity_id":"sensor.t","state":"unavailable","attributes":{"a":{"b":[1,2]}}}}`, "sensor.t")
	if err != nil || res.Output["state"] != "unavailable" || res.Output["last_changed"] != "" ||
		!reflect.DeepEqual(res.Output["attributes"], map[string]any{"a": map[string]any{"b": []any{1.0, 2.0}}}) {
		t.Errorf("a state without last_changed: %v %v", res.Output, err)
	}
	// Missing or malformed attributes become an empty object, a missing id the requested one.
	for _, attrs := range []string{``, `,"attributes":null`, `,"attributes":"x"`, `,"attributes":[1]`} {
		res, err = run(`{"status":"success","entity":{"state":"on"`+attrs+`}}`, "light.a")
		if err != nil || res.Output["entity_id"] != "light.a" || !reflect.DeepEqual(res.Output["attributes"], map[string]any{}) {
			t.Errorf("attributes %q: %v %v", attrs, res.Output, err)
		}
	}

	// An answer without a state is no state; the entity in the message is cut.
	long := "light." + strings.Repeat("a", maxHAEntityBytes-6)
	for _, reply := range []string{
		`{"status":"success"}`, `{"status":"success","entity":null}`, `{"status":"success","entity":{}}`, `{"status":"success","entity":"on"}`,
		`{"status":"success","entity":{"state":5}}`, `{"status":"success","entity":{"state":null}}`, `{"status":"success","entity":{"entity_id":"light.a"}}`,
		`{"status":"success","entity":[{"state":"on"}]}`,
	} {
		_, err := run(reply, long)
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || !strings.Contains(ne.Message, "returned no state for ") ||
			utf8.RuneCountInString(ne.Message) > 100 || !strings.Contains(ne.Message, "…") || strings.Contains(ne.Message, long) {
			t.Errorf("%s: %v", reply, err)
		}
	}

	// Attributes nested deeper than the engine takes would fail every flow that reads them.
	deep, err := json.Marshal(map[string]any{"status": "success", "entity": map[string]any{"state": "on", "attributes": homeDeep(maxJSONDepth + 50)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run(string(deep), "light.a"); asNodeError(err) == nil || asNodeError(err).Code != "FLOW_TOOL_ERROR" || !strings.Contains(asNodeError(err).Message, "nested") {
		t.Errorf("deep attributes: %v", err)
	}
	ok, err := json.Marshal(map[string]any{"status": "success", "entity": map[string]any{"state": "on", "attributes": homeDeep(10)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run(string(ok), "light.a"); err != nil {
		t.Errorf("attributes 10 levels deep: %v", err)
	}
}

// The service answer: the changed entities are listed as text only, the service name
// comes from the node's own parameters.
func TestHomeAssistantServiceAnswer(t *testing.T) {
	for name, c := range map[string]struct {
		reply string
		want  []any
	}{
		"listed":     {`{"status":"success","affected_entities":["light.a","light.b"]}`, []any{"light.a", "light.b"}},
		"null":       {`{"status":"success","affected_entities":null}`, []any{}},
		"absent":     {`{"status":"success","message":"Service light.turn_on called successfully","raw_response":"{}"}`, []any{}},
		"not a list": {`{"status":"success","affected_entities":"light.a"}`, []any{}},
		"mixed":      {`{"status":"success","affected_entities":["light.a",5,null,{"x":1},["y"],"light.b"]}`, []any{"light.a", "light.b"}},
	} {
		_, res, err := homeRunWith(t, TypeHomeAssistant, map[string]any{"service": "light.turn_off"}, homeReply(c.reply))
		if err != nil || res.Output["ok"] != true || res.Output["service"] != "light.turn_off" || !reflect.DeepEqual(res.Output["affected_entities"], c.want) {
			t.Errorf("%s: %v %v", name, res.Output, err)
		}
		if raw, _ := json.Marshal(res.Output); strings.Contains(string(raw), "raw_response") {
			t.Errorf("%s: the raw answer is in the output", name)
		}
	}
}

// What the flags say about the data: the entity and the service data decide what is
// switched, the topic and the payload what a device is told. The service is a literal.
func TestHomeSinkAndTaintFlags(t *testing.T) {
	reg := homeRegistry(t)
	sinks := map[string][]string{
		TypeHomeAssistant: {"entity", "service_data"}, TypeMQTTPublish: {"payload", "topic"},
		TypeAppointmentAdd: nil, TypeTodoAdd: nil,
	}
	untrusted := map[string]bool{TypeHomeAssistant: true}
	for typ, want := range sinks {
		def := lookupDef(t, reg, typ)
		var got []string
		for _, p := range def.Params {
			if p.SensitiveSink {
				got = append(got, p.Name)
			}
		}
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: sink parameters %v, want %v", typ, got, want)
		}
		if def.UntrustedOutput != untrusted[typ] {
			t.Errorf("%s: UntrustedOutput %v", typ, def.UntrustedOutput)
		}
	}
	for _, p := range lookupDef(t, reg, TypeHomeAssistant).Params {
		if p.Name == "service" && p.Templatable {
			t.Error("the service must not be templatable")
		}
	}
}

// The lint follows the flags: webhook data in a sink warns, in content does not; what
// Home Assistant reports is untrusted by itself; a payload or description that goes only
// to the effect does not make what the node returns untrusted, a title or topic that is
// echoed does.
func TestHomeNodesInTheLint(t *testing.T) {
	full := triggerRegistry(t)
	for _, err := range []error{registerHomeNodes(full, nil), registerWebNodes(full, nil)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	ref := func(name string) string { return "{{trigger.data.payload." + name + "}}" }
	b := newFlow("Home lint")
	tr := b.node("start", TypeTriggerWebhook, nil)
	nodes := map[string]string{
		"ha": b.node("ha", TypeHomeAssistant, map[string]any{"entity": ref("e"), "service": "light.turn_on", "service_data": ref("d")}),
		"mq": b.node("mq", TypeMQTTPublish, map[string]any{"topic": ref("t"), "payload": ref("p"), "qos": "0"}),
		"ap": b.node("ap", TypeAppointmentAdd, map[string]any{"title": ref("t"), "date_time": ref("d"), "description": ref("x")}),
		"td": b.node("td", TypeTodoAdd, map[string]any{"title": ref("t"), "description": ref("x"), "priority": ref("p"), "due_date": ref("d")}),
	}
	for _, id := range nodes {
		b.edge(tr, PortOut, id)
	}
	byNode := map[string][]string{}
	for _, is := range LintUntrustedData(b.build(), full) {
		if is.Code != IssueUntrustedData || is.Severity != SeverityWarning {
			t.Errorf("unexpected issue %+v", is)
		}
		byNode[is.NodeID] = append(byNode[is.NodeID], is.Param)
	}
	for id, want := range map[string][]string{nodes["ha"]: {"entity", "service_data"}, nodes["mq"]: {"payload", "topic"}, nodes["ap"]: nil, nodes["td"]: nil} {
		got := byNode[id]
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("node %s: warned on %v, want %v", id, got, want)
		}
	}

	// Taint through the outputs, without an untrusted trigger: only Home Assistant's own
	// answer is untrusted, and only what the node echoes carries a tainted parameter on.
	follow := func(typ string, params map[string]any, field string) []Issue {
		b := newFlow("Home follow")
		start := b.node("start", TypeTriggerWebhook, nil)
		src := b.node("src", typ, params)
		get := b.node("get", TypeWebRead, map[string]any{"url": "{{src." + field + "}}"})
		b.edge(start, PortOut, src)
		b.edge(src, PortOut, get)
		var found []Issue
		for _, is := range LintUntrustedData(b.build(), full) {
			if is.NodeID == get {
				found = append(found, is)
			}
		}
		return found
	}
	for _, c := range []struct {
		name   string
		typ    string
		params map[string]any
		field  string
		taints bool
	}{
		{"Home Assistant answer", TypeHomeAssistant, map[string]any{"entity": "light.a", "service": "turn_on"}, "service", true},
		{"mqtt payload only reaches the broker", TypeMQTTPublish, map[string]any{"topic": "a", "payload": ref("p")}, "topic", false},
		{"mqtt topic is echoed", TypeMQTTPublish, map[string]any{"topic": ref("t"), "payload": "x"}, "topic", true},
		{"appointment description", TypeAppointmentAdd, map[string]any{"title": "t", "date_time": "2026-10-05", "description": ref("x")}, "id", false},
		{"appointment title is echoed", TypeAppointmentAdd, map[string]any{"title": ref("t"), "date_time": "2026-10-05"}, "title", true},
		{"todo description", TypeTodoAdd, map[string]any{"title": "t", "description": ref("x")}, "id", false},
		{"todo title is echoed", TypeTodoAdd, map[string]any{"title": ref("t")}, "title", true},
	} {
		got := follow(c.typ, c.params, c.field)
		if (len(got) > 0) != c.taints {
			t.Errorf("%s: %d warnings on the next node, taints=%v: %+v", c.name, len(got), c.taints, got)
		}
	}
}

// The flag says the parameter never reaches the output: run each flagged parameter with
// a marker and look for the marker in what the node returns. The echoed ones, the topic
// and the titles, are not flagged and are in the output.
func TestHomeOutputIndependentFlags(t *testing.T) {
	const marker = "MARKER-5c2e81"
	want := []string{"mqtt.publish.payload", "planner.appointment_add.description", "planner.todo_add.description"}
	var flagged []string
	for _, def := range homeRegistry(t).All() {
		for _, p := range def.Params {
			if p.OutputIndependent {
				flagged = append(flagged, def.Type+"."+p.Name)
			}
		}
	}
	slices.Sort(flagged)
	if !reflect.DeepEqual(flagged, want) {
		t.Fatalf("flagged %v, want %v", flagged, want)
	}
	for _, name := range want {
		i := strings.LastIndex(name, ".")
		typ, param := name[:i], name[i+1:]
		_, res, err := homeRun(t, typ, map[string]any{param: marker})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if raw, _ := json.Marshal(res.Output); strings.Contains(string(raw), marker) {
			t.Errorf("%s is copied into the output %s", name, raw)
		}
	}
	for _, name := range []string{"mqtt.publish.topic", "planner.appointment_add.title", "planner.todo_add.title"} {
		i := strings.LastIndex(name, ".")
		typ, param := name[:i], name[i+1:]
		if slices.Contains(flagged, name) {
			t.Errorf("%s is flagged but echoed", name)
		}
		over := map[string]any{param: "echoed-text"}
		if param == "topic" {
			over[param] = "echoed/text"
		}
		_, res, err := homeRun(t, typ, over)
		if raw, _ := json.Marshal(res.Output); err != nil || !strings.Contains(string(raw), fmt.Sprint(over[param])) {
			t.Errorf("%s: not in the output %s (%v)", name, raw, err)
		}
	}
}
