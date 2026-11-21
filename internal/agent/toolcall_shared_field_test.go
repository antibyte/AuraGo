package agent

import (
	"encoding/json"
	"reflect"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// Some JSON keys are typed ToolCall fields that belong to one tool (Priority int, PowerAction int, Channel string,
// Quality string, AllowedDomains string, Command string, Env []string) while another tool declares the same property with
// a different JSON type (a priority label, a channel slot number, a screenshot quality, a list of domains, ...). One value
// that does not fit the typed field used to fail the WHOLE call (invalid_arguments). The mismatching value is now withheld
// from the typed decode, the typed field stays at its zero value and Params keeps what the model sent; every other mismatch
// still fails exactly as before.

type sharedFieldCase struct {
	name   string
	json   string
	key    string
	params interface{} // Params[key]: the value exactly as the model sent it
	typed  func(ToolCall) interface{}
	want   interface{} // the typed ToolCall field after decoding
}

func typedPriority(tc ToolCall) interface{}       { return tc.Priority }
func typedPowerAction(tc ToolCall) interface{}    { return tc.PowerAction }
func typedChannel(tc ToolCall) interface{}        { return tc.Channel }
func typedQuality(tc ToolCall) interface{}        { return tc.Quality }
func typedAllowedDomains(tc ToolCall) interface{} { return tc.AllowedDomains }
func typedCommand(tc ToolCall) interface{}        { return tc.Command }
func typedEnv(tc ToolCall) interface{}            { return tc.Env }

func sharedFieldCases() []sharedFieldCase {
	return []sharedFieldCase{
		// priority: Priority int (co-agents, memory notes) against a label (notifications, todos, cyd_display, telegram)
		{"priority low", `{"priority":"low"}`, "priority", "low", typedPriority, 0},
		{"priority normal", `{"priority":"normal"}`, "priority", "normal", typedPriority, 0},
		{"priority medium", `{"priority":"medium"}`, "priority", "medium", typedPriority, 0},
		{"priority high", `{"priority":"high"}`, "priority", "high", typedPriority, 0},
		{"priority critical", `{"priority":"critical"}`, "priority", "critical", typedPriority, 0},
		{"priority number stays typed", `{"priority":3}`, "priority", float64(3), typedPriority, 3},
		{"priority null", `{"priority":null}`, "priority", nil, typedPriority, 0},

		// power_action: PowerAction int against the meshcentral text (off, reset, sleep, amt_on, ...)
		{"power_action reset", `{"power_action":"reset"}`, "power_action", "reset", typedPowerAction, 0},
		{"power_action amt_on", `{"power_action":"amt_on"}`, "power_action", "amt_on", typedPowerAction, 0},
		{"power_action number stays typed", `{"power_action":4}`, "power_action", float64(4), typedPowerAction, 4},
		{"power_action null", `{"power_action":null}`, "power_action", nil, typedPowerAction, 0},

		// channel: Channel string (notifications) against the meshcore slot index
		{"channel slot number", `{"channel":2}`, "channel", float64(2), typedChannel, ""},
		{"channel slot zero", `{"channel":0}`, "channel", float64(0), typedChannel, ""},
		{"channel list", `{"channel":["a","b"]}`, "channel", []interface{}{"a", "b"}, typedChannel, ""},
		{"channel string stays typed", `{"channel":"push"}`, "channel", "push", typedChannel, "push"},
		{"channel null", `{"channel":null}`, "channel", nil, typedChannel, ""},

		// quality: Quality string (image generation: standard/hd) against a screenshot quality 1-100
		{"quality number", `{"quality":80}`, "quality", float64(80), typedQuality, ""},
		{"quality fraction", `{"quality":0.5}`, "quality", 0.5, typedQuality, ""},
		{"quality string stays typed", `{"quality":"hd"}`, "quality", "hd", typedQuality, "hd"},
		{"quality null", `{"quality":null}`, "quality", nil, typedQuality, ""},

		// allowed_domains: AllowedDomains string (site_crawler, comma separated) against a list of domains (here_now_site)
		{"allowed_domains list", `{"allowed_domains":["example.org","b.com"]}`, "allowed_domains", []interface{}{"example.org", "b.com"}, typedAllowedDomains, ""},
		{"allowed_domains empty list", `{"allowed_domains":[]}`, "allowed_domains", []interface{}{}, typedAllowedDomains, ""},
		{"allowed_domains string stays typed", `{"allowed_domains":"a.com,b.com"}`, "allowed_domains", "a.com,b.com", typedAllowedDomains, "a.com,b.com"},
		{"allowed_domains null", `{"allowed_domains":null}`, "allowed_domains", nil, typedAllowedDomains, ""},

		// command: Command string (execute_shell and friends) against the huggingface container command list
		{"command list", `{"command":["python","a.py"]}`, "command", []interface{}{"python", "a.py"}, typedCommand, ""},
		{"command empty list", `{"command":[]}`, "command", []interface{}{}, typedCommand, ""},
		{"command number", `{"command":5}`, "command", float64(5), typedCommand, ""},
		{"command string stays typed", `{"command":"ls -la"}`, "command", "ls -la", typedCommand, "ls -la"},
		{"command null", `{"command":null}`, "command", nil, typedCommand, ""},

		// env: Env []string (docker) against an object or an object text (huggingface; providers turn objects into strings)
		{"env object", `{"env":{"A":"b"}}`, "env", map[string]interface{}{"A": "b"}, typedEnv, []string(nil)},
		{"env object text", `{"env":"{\"A\":\"b\"}"}`, "env", `{"A":"b"}`, typedEnv, []string(nil)},
		{"env plain text", `{"env":"A=b"}`, "env", "A=b", typedEnv, []string(nil)},
		{"env list stays typed", `{"env":["A=b","C=d"]}`, "env", []interface{}{"A=b", "C=d"}, typedEnv, []string{"A=b", "C=d"}},
		{"env null", `{"env":null}`, "env", nil, typedEnv, []string(nil)},
	}
}

func nativeProbeCall(name, arguments string) ToolCall {
	return NativeToolCallToToolCall(openai.ToolCall{ID: "call-1", Type: "function", Function: openai.FunctionCall{Name: name, Arguments: arguments}}, nil)
}

func expectSharedField(t *testing.T, label string, tc ToolCall, key string, params interface{}, typed func(ToolCall) interface{}, want interface{}) {
	t.Helper()
	got, present := tc.Params[key]
	if !present {
		t.Errorf("%s: Params has no %q: %#v", label, key, tc.Params)
	} else if !reflect.DeepEqual(got, params) {
		t.Errorf("%s: Params[%s] = %#v, want %#v", label, key, got, params)
	}
	if got := typed(tc); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: typed field of %s = %#v, want %#v", label, key, got, want)
	}
}

func TestToolCallSharedFieldDecodesWithoutErrorAndKeepsParams(t *testing.T) {
	for _, c := range sharedFieldCases() {
		t.Run(c.name+"/plain", func(t *testing.T) {
			var tc ToolCall
			if err := json.Unmarshal([]byte(c.json), &tc); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			expectSharedField(t, "plain", tc, c.key, c.params, c.typed, c.want)
		})
		t.Run(c.name+"/native", func(t *testing.T) {
			tc := nativeProbeCall("shared_field_probe", c.json)
			if tc.NativeArgsMalformed {
				t.Fatalf("flagged malformed: %s", tc.NativeArgsError)
			}
			expectSharedField(t, "native", tc, c.key, c.params, c.typed, c.want)
		})
	}
}

// A value of another type than the ones above is still a malformed call: the guard is narrow on purpose.
func TestToolCallSharedFieldOtherMismatchesStayRejected(t *testing.T) {
	for _, c := range []struct{ name, json string }{
		{"depth text", `{"depth":"deep"}`},
		{"priority bool", `{"priority":true}`},
		{"priority object", `{"priority":{"a":1}}`},
		{"priority list", `{"priority":["x"]}`},
		{"priority fraction", `{"priority":1.5}`},
		{"power_action bool", `{"power_action":true}`},
		{"power_action list", `{"power_action":["x"]}`},
		{"power_action object", `{"power_action":{"a":1}}`},
		{"channel bool", `{"channel":true}`},
		{"channel object", `{"channel":{"a":1}}`},
		{"quality bool", `{"quality":true}`},
		{"quality object", `{"quality":{"a":1}}`},
		{"allowed_domains bool", `{"allowed_domains":true}`},
		{"allowed_domains object", `{"allowed_domains":{"a":1}}`},
		{"command bool", `{"command":true}`},
		{"command object", `{"command":{"a":1}}`},
		{"env number", `{"env":5}`},
		{"env bool", `{"env":true}`},
		{"env list of objects", `{"env":[{"a":1}]}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			var tc ToolCall
			if err := json.Unmarshal([]byte(c.json), &tc); err == nil {
				t.Errorf("plain decode accepted %s", c.json)
			}
			if native := nativeProbeCall("shared_field_probe", c.json); !native.NativeArgsMalformed {
				t.Errorf("native decode accepted %s", c.json)
			}
		})
	}
}

// Whatever else is in the call keeps decoding into its typed fields next to a withheld key.
func TestToolCallSharedFieldLeavesTheRestOfTheCallTyped(t *testing.T) {
	arguments := `{"operation":"job_run_container","image":"python:3","command":["python","a.py"],"env":{"A":"b"},"limit":5}`
	for name, tc := range map[string]ToolCall{
		"native": nativeProbeCall("huggingface", arguments),
		"plain": func() ToolCall {
			var tc ToolCall
			if err := json.Unmarshal([]byte(arguments), &tc); err != nil {
				t.Fatalf("plain decode: %v", err)
			}
			return tc
		}(),
	} {
		if tc.NativeArgsMalformed {
			t.Fatalf("%s: flagged malformed: %s", name, tc.NativeArgsError)
		}
		if tc.Operation != "job_run_container" || tc.Image != "python:3" || tc.Limit != 5 {
			t.Errorf("%s: typed fields lost: operation %q, image %q, limit %d", name, tc.Operation, tc.Image, tc.Limit)
		}
		if !reflect.DeepEqual(tc.Params["command"], []interface{}{"python", "a.py"}) || !reflect.DeepEqual(tc.Params["env"], map[string]interface{}{"A": "b"}) {
			t.Errorf("%s: Params lost the withheld values: %#v", name, tc.Params)
		}
	}
}

func TestWithholdMismatchedSharedFieldsLeavesDataAloneWhenNothingToWithhold(t *testing.T) {
	for _, input := range []string{
		``, `   `, `null`, `[1,2]`, `"text"`, `{bad`, `{}`,
		`{"priority":3}`, `{"priority":null}`, `{"priority":true}`, `{"priority":{"a":1}}`,
		`{"channel":"push"}`, `{"channel":true}`, `{"quality":"hd"}`, `{"command":"ls"}`, `{"command":true}`,
		`{"allowed_domains":"a.com"}`, `{"env":["A=b"]}`, `{"env":5}`, `{"depth":"deep"}`, `{"other":["x"],"id":4}`,
	} {
		data := []byte(input)
		out, withheld := withholdMismatchedSharedFields(data)
		if len(withheld) != 0 {
			t.Errorf("%q: withheld %v", input, withheld)
		}
		if len(out) != len(data) || (len(data) > 0 && &out[0] != &data[0]) {
			t.Errorf("%q: the original slice was not returned untouched: %q", input, out)
		}
	}
}

func TestWithholdMismatchedSharedFieldsRemovesOnlyTheMismatches(t *testing.T) {
	data := []byte(`{"action":"x","priority":"high","power_action":"reset","channel":2,"quality":80,"allowed_domains":["a"],"command":["p","q"],"env":{"A":"b"},"keep":1,"depth":2}`)
	out, withheld := withholdMismatchedSharedFields(data)
	wantWithheld := map[string]string{
		"priority": `"high"`, "power_action": `"reset"`, "channel": `2`, "quality": `80`,
		"allowed_domains": `["a"]`, "command": `["p","q"]`, "env": `{"A":"b"}`,
	}
	if len(withheld) != len(wantWithheld) {
		t.Errorf("withheld %d keys, want %d: %v", len(withheld), len(wantWithheld), withheld)
	}
	for key, want := range wantWithheld {
		if got := string(withheld[key]); got != want {
			t.Errorf("withheld[%s] = %s, want %s", key, got, want)
		}
	}
	var rest map[string]json.RawMessage
	if err := json.Unmarshal(out, &rest); err != nil {
		t.Fatalf("remaining data is not a JSON object: %v: %s", err, out)
	}
	if len(rest) != 3 || string(rest["action"]) != `"x"` || string(rest["keep"]) != `1` || string(rest["depth"]) != `2` {
		t.Errorf("remaining data = %s, want action, keep and depth only", out)
	}
}

// The tool shapes seen in the schemas: the value the model sends is what the handler reads from Params.
func TestToolCallSharedFieldRealToolShapes(t *testing.T) {
	for _, c := range []struct {
		tool, arguments string
		check           func(t *testing.T, tc ToolCall)
	}{
		{"send_notification", `{"channel":"push","title":"t","message":"m","priority":"high"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "send_notification", tc, "priority", "high", typedPriority, 0)
			if tc.Channel != "push" || tc.Message != "m" || tc.Title != "t" {
				t.Errorf("channel %q, message %q, title %q", tc.Channel, tc.Message, tc.Title)
			}
		}},
		{"manage_todos", `{"operation":"add","title":"t","priority":"high"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "manage_todos", tc, "priority", "high", typedPriority, 0)
			if tc.Operation != "add" || tc.Title != "t" {
				t.Errorf("operation %q, title %q", tc.Operation, tc.Title)
			}
		}},
		{"cyd_display", `{"operation":"notify","title":"t","message":"m","priority":"critical"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "cyd_display", tc, "priority", "critical", typedPriority, 0)
		}},
		{"send_telegram", `{"message":"m","priority":"normal"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "send_telegram", tc, "priority", "normal", typedPriority, 0)
		}},
		{"spawn_coagent", `{"operation":"spawn","task":"t","priority":3}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "spawn_coagent", tc, "priority", float64(3), typedPriority, 3)
		}},
		{"meshcentral", `{"operation":"power_action","node_id":"n","power_action":"reset"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "meshcentral", tc, "power_action", "reset", typedPowerAction, 0)
			if tc.Operation != "power_action" || tc.NodeID != "n" {
				t.Errorf("operation %q, node %q", tc.Operation, tc.NodeID)
			}
		}},
		{"meshcore", `{"operation":"send_channel","channel":2,"text":"hi"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "meshcore", tc, "channel", float64(2), typedChannel, "")
			if tc.Operation != "send_channel" || tc.Text != "hi" || tc.Params["text"] != "hi" {
				t.Errorf("operation %q, text %q, Params[text] %v", tc.Operation, tc.Text, tc.Params["text"])
			}
		}},
		{"remote_control_desktop", `{"operation":"desktop_screenshot","quality":80}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "remote_control_desktop", tc, "quality", float64(80), typedQuality, "")
		}},
		{"remote_control", `{"operation":"desktop_screenshot","format":"jpeg","quality":80}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "remote_control", tc, "quality", float64(80), typedQuality, "")
		}},
		{"here_now_site", `{"operation":"update_access","allowed_domains":["example.org"]}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "here_now_site", tc, "allowed_domains", []interface{}{"example.org"}, typedAllowedDomains, "")
			if tc.Operation != "update_access" {
				t.Errorf("operation %q", tc.Operation)
			}
		}},
		{"huggingface", `{"operation":"job_run_container","command":["python","a.py"],"env":{"A":"b"}}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "huggingface command", tc, "command", []interface{}{"python", "a.py"}, typedCommand, "")
			expectSharedField(t, "huggingface env", tc, "env", map[string]interface{}{"A": "b"}, typedEnv, []string(nil))
		}},
		{"huggingface", `{"operation":"job_run_container","command":["python","a.py"],"env":"{\"A\":\"b\"}"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "huggingface command", tc, "command", []interface{}{"python", "a.py"}, typedCommand, "")
			expectSharedField(t, "huggingface env text", tc, "env", `{"A":"b"}`, typedEnv, []string(nil))
		}},
		{"docker", `{"operation":"run","image":"nginx","env":["A=b"]}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "docker", tc, "env", []interface{}{"A=b"}, typedEnv, []string{"A=b"})
		}},
		{"execute_shell", `{"command":"ls -la"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "execute_shell", tc, "command", "ls -la", typedCommand, "ls -la")
		}},
		{"site_crawler", `{"operation":"crawl","url":"https://example.org","allowed_domains":"a.com,b.com"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "site_crawler", tc, "allowed_domains", "a.com,b.com", typedAllowedDomains, "a.com,b.com")
		}},
		{"generate_image", `{"prompt":"a cat","quality":"hd"}`, func(t *testing.T, tc ToolCall) {
			expectSharedField(t, "generate_image", tc, "quality", "hd", typedQuality, "hd")
		}},
	} {
		tc := nativeProbeCall(c.tool, c.arguments)
		if tc.NativeArgsMalformed {
			t.Errorf("%s %s: flagged malformed: %s", c.tool, c.arguments, tc.NativeArgsError)
			continue
		}
		if tc.Action != c.tool {
			t.Errorf("%s: action %q", c.tool, tc.Action)
		}
		c.check(t, tc)
	}
}
