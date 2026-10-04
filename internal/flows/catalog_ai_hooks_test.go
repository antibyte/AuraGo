package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// The editor's field list shows what the node can produce: no blank, malformed,
// reserved or repeated names, and an unknown type counts as text.
func TestAIFieldsOfListsUsableFieldsOnly(t *testing.T) {
	def := aiDef(t)
	params := map[string]any{"output_mode": "fields", "fields": []any{
		map[string]any{"name": "Bad Name"},
		map[string]any{"name": " a ", "type": "number"},
		map[string]any{"name": "a", "type": "bool"},
		map[string]any{"name": "tokens"},
		map[string]any{"name": "b", "type": "date"},
		map[string]any{"name": ""},
		map[string]any{"name": 5.0},
		map[string]any{"name": strings.Repeat("c", 41)},
		"not an object",
		map[string]any{"name": "d", "type": []any{"list"}},
	}}
	got := def.FieldsOf(&Node{Params: params})
	want := []FieldSpec{
		{Name: "a", Type: "number", Primary: true}, {Name: "b", Type: "text"}, {Name: "d", Type: "text"},
		{Name: "tokens", Type: "object"}, {Name: "model", Type: "text"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("fields = %+v, want %+v", got, want)
	}
}

// OutputFieldsFunc, Validate and aiFields see raw parameters of any type. None may
// panic, none may produce unbounded text or lists, and Execute must follow the same
// field rules as Validate.
func TestAIHooksSurviveOddParams(t *testing.T) {
	def := aiDef(t)
	vc := ValidateContext{Mode: ModePublish}
	values := oddParamValues()
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	runs := 0

	// check runs every hook and Execute on params. clean says that prompt, instructions
	// and model are plain, so only the fields can make Execute fail.
	check := func(params map[string]any, label string, clean bool) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "ai", Type: TypeAIStep, Params: params}
		var (
			issues  []Issue
			listed  []FieldSpec
			usable  []aiField
			res     ExecResult
			execErr error
			answer  map[string]any
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() { issues = def.Validate(node, vc) }},
			{"FieldsOf", func() { listed = def.FieldsOf(node) }},
			{"aiFields", func() { usable = aiFields(params["fields"]); answer = aiAnswer(usable) }},
			{"Execute", func() {
				llm := &fakeLLM{responses: []LLMResponse{{JSON: answer, Text: "t", Model: "m"}}}
				res, execErr = execDef(def, params, &Services{LLM: llm})
			}},
		} {
			if p := catchPanic(step.fn); p != nil {
				fail("%s: %s panicked: %v", label, step.name, p)
				return
			}
		}
		if len(issues) > 3*maxAIFields+1 {
			fail("%s: %d issues", label, len(issues))
		}
		for _, is := range issues {
			if (is.Code != IssueParamInvalid && is.Code != IssueParamRequired) || is.Severity != SeverityError || is.NodeID != node.ID ||
				len(is.Message) > maxEchoMessageBytes || len(is.Param) > 60 || !utf8.ValidString(is.Message) || strings.ContainsAny(is.Message, "\n\r\x00") {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
		}
		if len(listed) > maxAIFields+2 || len(listed) < 2 {
			fail("%s: FieldsOf returned %d entries", label, len(listed))
		} else if listed[len(listed)-2].Name != "tokens" || listed[len(listed)-1].Name != "model" {
			fail("%s: FieldsOf does not end with tokens and model: %+v", label, listed)
		}
		for _, f := range listed {
			if len(f.Name) > maxAIFieldNameLen || !utf8.ValidString(f.Name) || f.Type == "" {
				fail("%s: bad field %+v", label, f)
			}
		}
		if _, err := json.Marshal(listed); err != nil {
			fail("%s: fields are not JSON encodable: %v", label, err)
		}
		if execErr != nil {
			var ne *NodeError
			if !errors.As(execErr, &ne) || ne == nil || ne.Code == "" || len(ne.Message) > maxEchoMessageBytes || !utf8.ValidString(ne.Message) {
				fail("%s: bad Execute error %T %.200q", label, execErr, execErr.Error())
				return
			}
			if ne.Code == "FLOW_PARAM_INVALID" && strings.HasPrefix(ne.Message, "fields") && aiFieldsMode(params) && len(issues) == 0 {
				fail("%s: Execute rejected the fields (%s) but Validate reported nothing", label, ne.Message)
			}
			if ne.Code == "FLOW_PARAM_INVALID" && strings.HasPrefix(ne.Message, "output_mode") && len(issues) == 0 {
				fail("%s: Execute rejected the output mode (%s) but Validate reported nothing", label, ne.Message)
			}
		} else if raw, err := json.Marshal(res.Output); err != nil || !utf8.Valid(raw) {
			fail("%s: output does not encode: %v", label, err)
		}
		if clean && aiFieldsMode(params) && len(issues) == 0 && len(usable) > 0 && execErr != nil {
			fail("%s: Validate is clean and the answer fits, yet Execute failed: %v", label, execErr)
		}
	}

	plain := func(extra map[string]any) map[string]any {
		p := map[string]any{"prompt": "x", "output_mode": "fields", "fields": []any{map[string]any{"name": "a"}}}
		for k, v := range extra {
			p[k] = v
		}
		return p
	}
	for _, ov := range values {
		v := ov.v
		item := func(key string) map[string]any {
			m := map[string]any{"name": "a"}
			m[key] = v
			return plain(map[string]any{"fields": []any{m}})
		}
		for _, mode := range []string{"text", "fields"} {
			for _, name := range []string{"prompt", "instructions", "model", "unknown_param"} {
				check(plain(map[string]any{"output_mode": mode, name: v}), fmt.Sprintf("%s=%s mode %s", name, ov.name, mode), name == "unknown_param")
			}
		}
		check(plain(map[string]any{"output_mode": v}), "output_mode="+ov.name, true)
		check(plain(map[string]any{"fields": v}), "fields="+ov.name, true)
		check(plain(map[string]any{"fields": []any{v}}), "fields=[]{"+ov.name+"}", true)
		check(plain(map[string]any{"fields": []any{v, map[string]any{"name": "b"}}}), "fields=[]{"+ov.name+", b}", true)
		check(item("name"), "name="+ov.name, true)
		check(item("type"), "type="+ov.name, true)
		check(item("description"), "description="+ov.name, true)
		all := map[string]any{}
		for _, name := range []string{"prompt", "instructions", "output_mode", "fields", "model", "unknown_param"} {
			all[name] = v
		}
		check(all, "all="+ov.name, false)
		all["output_mode"] = "fields"
		check(all, "all, fields mode="+ov.name, false)
	}
	check(nil, "nil params", false)
	check(map[string]any{}, "empty params", false)
	check(map[string]any{"output_mode": "fields"}, "fields mode without fields", false)
	if runs < 700 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}

func TestAIHooksHandleNilNodes(t *testing.T) {
	def := aiDef(t)
	if issues := def.Validate(nil, ValidateContext{}); len(issues) != 0 {
		t.Errorf("Validate(nil) = %+v", issues)
	}
	if got := def.OutputFieldsFunc(nil); len(got) != 3 || got[0].Name != "text" {
		t.Errorf("OutputFieldsFunc(nil) = %+v", got)
	}
	if got := aiFields(nil); got == nil || len(got) != 0 {
		t.Errorf("aiFields(nil) = %#v", got)
	}
	if got := def.FieldsOf(&Node{}); len(got) != 3 || got[0].Name != "text" {
		t.Errorf("FieldsOf with nil params = %+v", got)
	}
}
