package flows

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestMQTTTopicIsValidated(t *testing.T) {
	cases := []struct {
		name string
		v    any
	}{
		{"nil", nil}, {"empty", ""}, {"blank", " \t "}, {"plus", "home/+/light"}, {"hash", "home/#"}, {"only hash", "#"}, {"hash inside", "home/a#b"},
		{"nul", "home/a" + homeNUL + "b"}, {"line feed", "home/a" + homeLF + "b"}, {"carriage return", "home/a" + homeCR + "b"}, {"tab", "home/a\tb"},
		{"invalid utf-8", "home/" + homeBad}, {"too long", strings.Repeat("a", maxMQTTTopicBytes+1)},
		{"list", []any{"home/a"}}, {"object", map[string]any{"topic": "home/a"}},
	}
	for _, c := range cases {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"topic": c.v})
		homeWantInvalid(t, c.name, tools, err, homeEchoOf(c.v))
	}
	for _, topic := range []string{"home/a", "/", "a//b", "$SYS/x", "Zimmer/Küche/Licht", "  home/a  ", strings.Repeat("a", maxMQTTTopicBytes), "5"} {
		tools, res, err := homeRun(t, TypeMQTTPublish, map[string]any{"topic": topic})
		if err != nil || tools.last(t).Args["topic"] != strings.TrimSpace(topic) || res.Output["topic"] != strings.TrimSpace(topic) {
			t.Errorf("topic %.30q: %v", topic, err)
		}
	}
	// A number is text of its own form.
	if tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"topic": 42.0}); err != nil || tools.last(t).Args["topic"] != "42" {
		t.Errorf("numeric topic: %v", err)
	}
}

func TestMQTTQoS(t *testing.T) {
	for _, c := range []struct {
		v    any
		want int
	}{
		{"0", 0}, {"1", 1}, {"2", 2}, {" 1 ", 1}, {"", 0}, {" ", 0}, {nil, 0}, {0.0, 0}, {1.0, 1}, {2.0, 2}, {2, 2}, {int64(1), 1}, {float32(2), 2},
	} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"qos": c.v})
		if got := tools.last(t).Args["qos"]; err != nil || got != c.want {
			t.Errorf("qos %#v: %v %#v", c.v, err, got)
		}
	}
	// The default (parameter absent) is 0.
	tools := &fakeTools{respond: homeAnswers}
	def := lookupDef(t, homeRegistry(t), TypeMQTTPublish)
	if _, err := execDef(def, map[string]any{"topic": "a"}, &Services{Tools: tools}); err != nil || tools.last(t).Args["qos"] != 0 {
		t.Errorf("default qos: %v %v", err, tools.last(t).Args)
	}
	// The tool would use its configured default for anything outside 0 to 2; the node refuses.
	for _, v := range []any{"3", "-1", "01", "1e0", "0x1", "abc", "1,0", 3.0, -1.0, 0.5, 1.5, 2.000001, math.NaN(), math.Inf(1), true, false, []any{}, []any{1.0}, map[string]any{}, 1e300} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"qos": v})
		homeWantInvalid(t, fmt.Sprintf("qos %v", v), tools, err, homeEchoOf(v))
	}
}

func TestMQTTRetain(t *testing.T) {
	for _, c := range []struct {
		v    any
		want bool
	}{
		{true, true}, {false, false}, {nil, false}, {"", false}, {"  ", false}, {"true", true}, {"YES", true}, {"on", true}, {"1", true}, {"ja", true},
		{"false", false}, {"no", false}, {"off", false}, {"0", false}, {"nein", false}, {1.0, true}, {0.0, false}, {2.0, true},
	} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"retain": c.v})
		if err != nil || tools.last(t).Args["retain"] != c.want {
			t.Errorf("retain %#v: %v %v", c.v, err, tools.last(t).Args["retain"])
		}
	}
	// A flag that cannot be read fails: a retained message stays on the broker.
	for _, v := range []any{"maybe", "tru", "2x", "retain", []any{}, []any{true}, map[string]any{}, map[string]any{"a": true}, math.NaN()} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"retain": v})
		homeWantInvalid(t, fmt.Sprintf("retain %v", v), tools, err, homeEchoOf(v))
	}
	// The default (parameter absent) is off.
	tools := &fakeTools{respond: homeAnswers}
	def := lookupDef(t, homeRegistry(t), TypeMQTTPublish)
	if _, err := execDef(def, map[string]any{"topic": "a"}, &Services{Tools: tools}); err != nil || tools.last(t).Args["retain"] != false {
		t.Errorf("default retain: %v %v", err, tools.last(t).Args)
	}
}

func TestMQTTPayload(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	atLimit := strings.Repeat("p", maxMQTTPayloadBytes)
	for _, c := range []struct {
		name string
		v    any
		want string
	}{
		{"text as it is", "  on \n", "  on \n"}, {"nil", nil, ""}, {"empty", "", ""}, {"number", 21.5, "21.5"}, {"whole number", 3.0, "3"}, {"bool", true, "true"},
		{"object", map[string]any{"on": true, "level": 3.0}, `{"level":3,"on":true}`}, {"list", []any{1.0, "a", nil}, `[1,"a",null]`},
		{"html is not escaped", map[string]any{"a": "<b>&"}, `{"a":"<b>&"}`}, {"empty object", map[string]any{}, "{}"}, {"empty list", []any{}, "[]"},
		{"at the limit", atLimit, atLimit}, {"umlauts", "Küche an", "Küche an"},
	} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"payload": c.v})
		if got := tools.last(t).Args["payload"]; err != nil || got != c.want {
			t.Errorf("%s: %.60q, %v", c.name, got, err)
		}
	}
	for _, c := range []struct {
		name string
		v    any
	}{
		{"over the limit", atLimit + "p"}, {"invalid utf-8", "on" + homeBad}, {"cyclic", cyclic}, {"func", func() {}}, {"chan", make(chan int)},
		{"object with NaN", map[string]any{"a": math.NaN()}}, {"list with Inf", []any{math.Inf(1)}}, {"object too large", map[string]any{"a": atLimit}},
	} {
		tools, _, err := homeRun(t, TypeMQTTPublish, map[string]any{"payload": c.v})
		homeWantInvalid(t, c.name, tools, err, homeEchoOf(c.v))
	}
}

// The tool refuses with an error answer or with plain text (disabled, read-only, a
// broker that is down): none of that is a published message. The output never carries
// anything of the answer or the payload.
func TestMQTTPublishAnswers(t *testing.T) {
	for name, reply := range map[string]ToolResponse{
		"read-only":        {Output: `Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.readonly to allow changes"}`, Status: "success"},
		"not connected":    {Output: `Tool Output: {"status":"error","message":"MQTT publish failed: MQTT client is not connected"}`, Status: "success"},
		"plain text":       {Output: "[PERMISSION DENIED] mqtt is disabled", Status: "success"},
		"empty":            {Output: "", Status: "success"},
		"no status":        {Output: `Tool Output: {"message":"Published to topic 'a'"}`, Status: "success"},
		"other status":     {Output: `Tool Output: {"status":"queued"}`, Status: "success"},
		"flagged as error": {Output: `Tool Output: {"status":"success"}`, Status: "success", IsError: true},
		"huge refusal":     {Output: "[PERMISSION DENIED] " + strings.Repeat("no ", 5000), Status: "success"},
	} {
		tools, res, err := homeRunWith(t, TypeMQTTPublish, nil, func(ToolRequest) (ToolResponse, error) { return reply, nil })
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || res.Output != nil || tools.count() != 1 || len(ne.Message) > maxEchoMessageBytes {
			t.Errorf("%s: %v, output %v", name, err, res.Output)
		}
	}
	for status, code := range map[string]string{"denied": "FLOW_TOOL_DENIED", "policy_denied": "FLOW_TOOL_DENIED", "needs_setup": "FLOW_NODE_UNAVAILABLE"} {
		_, _, err := homeRunWith(t, TypeMQTTPublish, nil, func(ToolRequest) (ToolResponse, error) {
			return ToolResponse{Output: "MQTT is not enabled", Status: status}, nil
		})
		if asNodeError(err) == nil || asNodeError(err).Code != code {
			t.Errorf("%s: %v, want %s", status, err, code)
		}
	}
	// Whatever case or blanks the status has, a success is one; the answer's fields are not copied.
	_, res, err := homeRunWith(t, TypeMQTTPublish, map[string]any{"payload": "secret-payload"},
		homeReply(`{"status":" SUCCESS ","topic":"other","payload":"secret-payload","message":"x"}`))
	if raw, _ := json.Marshal(res.Output); err != nil || !reflect.DeepEqual(res.Output, map[string]any{"published": true, "topic": "home/a"}) || strings.Contains(string(raw), "secret") {
		t.Errorf("output %s, %v", raw, err)
	}
}

func TestMQTTValidate(t *testing.T) {
	def := lookupDef(t, homeRegistry(t), TypeMQTTPublish)
	check := func(params map[string]any) []string {
		var got []string
		for _, is := range def.Validate(&Node{ID: testNodeID(1), Key: "n", Type: TypeMQTTPublish, Params: params}, ValidateContext{Mode: ModePublish}) {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || len(is.Message) > maxEchoMessageBytes || strings.ContainsAny(is.Message, homeLF+homeCR+homeNUL) {
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
		{"fine", map[string]any{"topic": "home/a", "payload": map[string]any{"a": 1.0}, "qos": "1", "retain": true}, nil},
		{"templates", map[string]any{"topic": "{{x.t}}", "payload": "{{x.p}}", "qos": "{{x.q}}", "retain": "{{x.r}}"}, nil},
		{"template inside text", map[string]any{"topic": "home/{{x.room}}/light", "payload": "v={{x.v}}"}, nil},
		{"bare", map[string]any{}, nil},
		{"wildcard", map[string]any{"topic": "home/#"}, []string{"topic"}},
		{"long topic", map[string]any{"topic": strings.Repeat("a", maxMQTTTopicBytes+1)}, []string{"topic"}},
		{"huge payload", map[string]any{"payload": strings.Repeat("p", maxMQTTPayloadBytes+1)}, []string{"payload"}},
		{"bad qos", map[string]any{"qos": "5"}, []string{"qos"}},
		{"bad retain", map[string]any{"retain": "perhaps"}, []string{"retain"}},
		{"everything", map[string]any{"topic": "a/+", "payload": []any{math.NaN()}, "qos": 9.0, "retain": []any{}}, []string{"topic", "payload", "qos", "retain"}},
		{"empty values are the required check's", map[string]any{"topic": "", "payload": "", "qos": "", "retain": ""}, nil},
	} {
		if got := check(c.params); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: issues on %v, want %v", c.name, got, c.want)
		}
	}
}
