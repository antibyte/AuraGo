package flows

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const aiAnswerSentence = "Answer with exactly one JSON object with the fields: "

// The model must learn each field's type and description. The JSON schema is only a
// flag for some providers (an adapter may merely switch on JSON mode), so the system
// prompt carries them as one line per field after the unchanged opening sentence.
func TestAIStepSystemPromptDescribesFields(t *testing.T) {
	def := aiDef(t)
	tail := "\n- zusammenfassung (string): Kurzfassung\n- anzahl (number)\n- wichtig (boolean)\n- themen (array of strings)"
	sentence := aiAnswerSentence + "zusammenfassung, anzahl, wichtig, themen."

	llm := &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(aiFieldsParams()["fields"]))}}}
	params := aiFieldsParams()
	params["instructions"] = "kurz"
	if _, err := execDef(def, params, &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	if got, want := llm.callAt(0).System, "kurz\n\n"+sentence+tail; got != want {
		t.Fatalf("system prompt =\n%q\nwant\n%q", got, want)
	}

	// Without instructions the prompt starts with the sentence.
	llm = &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(aiFieldsParams()["fields"]))}}}
	if _, err := execDef(def, aiFieldsParams(), &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	if got, want := llm.callAt(0).System, sentence+tail; got != want {
		t.Fatalf("system prompt =\n%q\nwant\n%q", got, want)
	}

	// A description stays on its line whatever whitespace it holds; empty ones are left out.
	params = map[string]any{"prompt": "x", "output_mode": "fields", "fields": []any{
		map[string]any{"name": "a", "description": "  line one\n\n- injected\tline two  "},
		map[string]any{"name": "b", "description": "   "},
	}}
	llm = &fakeLLM{responses: []LLMResponse{{JSON: map[string]any{"a": "x", "b": "y"}}}}
	if _, err := execDef(def, params, &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	if got, want := llm.callAt(0).System, aiAnswerSentence+"a, b.\n- a (string): line one - injected line two\n- b (string)"; got != want {
		t.Fatalf("system prompt =\n%q\nwant\n%q", got, want)
	}

	// Text mode has no field lines.
	llm = &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
	if _, err := execDef(def, map[string]any{"prompt": "x", "instructions": "kurz"}, &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	if got := llm.callAt(0).System; got != "kurz" {
		t.Fatalf("text mode system prompt = %q", got)
	}
}

// The caps on field count and description length bound what the hints add.
func TestAIStepFieldHintsAreBounded(t *testing.T) {
	list := make([]any, maxAIFields)
	for i := range list {
		list[i] = map[string]any{"name": aiFieldName(i), "type": "list", "description": strings.Repeat("ä", maxAIFieldDescRunes)}
	}
	params := map[string]any{"prompt": "x", "output_mode": "fields", "fields": list}
	llm := &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(list))}}}
	if _, err := execDef(aiDef(t), params, &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	system := llm.callAt(0).System
	if n := strings.Count(system, "\n- "); n != maxAIFields {
		t.Fatalf("%d field lines, want %d", n, maxAIFields)
	}
	// 50 lines of at most 40 + 20 + 1000 bytes of text, plus the sentence.
	if len(system) > maxAIFields*1100+200 || !utf8.ValidString(system) {
		t.Fatalf("system prompt has %d bytes", len(system))
	}
}

func aiFieldName(i int) string {
	return "f" + strings.Repeat("a", i/26) + string(rune('a'+i%26))
}

// Every attempt is paid for, so the reported tokens are the sums over the attempts.
func TestAIStepSumsTokensAcrossRepair(t *testing.T) {
	def := aiDef(t)
	valid := map[string]any{"zusammenfassung": "y", "anzahl": 1.0, "wichtig": false, "themen": []any{}}
	llm := &fakeLLM{responses: []LLMResponse{
		{Text: "no json", Model: "m1", InputTokens: 100, OutputTokens: 50},
		{JSON: valid, Model: "m2", InputTokens: 120, OutputTokens: 60},
	}}
	res, err := execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if err != nil || llm.callCount() != 2 {
		t.Fatalf("err = %v after %d calls", err, llm.callCount())
	}
	if want := map[string]any{"input": 220.0, "output": 110.0}; !reflect.DeepEqual(res.Output["tokens"], want) {
		t.Fatalf("tokens = %#v, want %#v", res.Output["tokens"], want)
	}
	if res.Output["model"] != "m2" {
		t.Fatalf("model = %v, want the model of the answer that was used", res.Output["model"])
	}

	// A single call reports exactly its own numbers.
	llm = &fakeLLM{responses: []LLMResponse{{JSON: valid, Model: "m1", InputTokens: 7, OutputTokens: 3}}}
	res, err = execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if err != nil || !reflect.DeepEqual(res.Output["tokens"], map[string]any{"input": 7.0, "output": 3.0}) {
		t.Fatalf("single call tokens = %#v, %v", res.Output["tokens"], err)
	}
	llm = &fakeLLM{responses: []LLMResponse{{Text: "ok", Model: "m1", InputTokens: 5, OutputTokens: 2}}}
	res, err = execDef(def, map[string]any{"prompt": "x"}, &Services{LLM: llm})
	if err != nil || !reflect.DeepEqual(res.Output["tokens"], map[string]any{"input": 5.0, "output": 2.0}) {
		t.Fatalf("text mode tokens = %#v, %v", res.Output["tokens"], err)
	}
}

func TestAIStepListSchemaHasItems(t *testing.T) {
	llm := &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(aiFieldsParams()["fields"]))}}}
	if _, err := execDef(aiDef(t), aiFieldsParams(), &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	props := llm.callAt(0).JSONSchema["properties"].(map[string]any)
	if got := props["themen"].(map[string]any); !reflect.DeepEqual(got, map[string]any{"type": "array", "items": map[string]any{"type": "string"}}) {
		t.Fatalf("themen schema = %#v", got)
	}
	for _, name := range []string{"zusammenfassung", "anzahl", "wichtig"} {
		if _, has := props[name].(map[string]any)["items"]; has {
			t.Errorf("%s must not have items", name)
		}
	}
}

func TestExtractJSONObjectMessyInput(t *testing.T) {
	obj := map[string]any{"a": 1.0}
	cases := []struct {
		name string
		in   string
		want map[string]any // nil: no object
	}{
		{"plain", `{"a":1}`, obj},
		{"padded", "  \n{\"a\":1}\n ", obj},
		{"fence", "```json\n{\"a\":1}\n```", obj},
		{"fence without tag", "```\n{\"a\":1}\n```", obj},
		{"fence upper case", "```JSON\n{\"a\":1}\n```", obj},
		{"prose before", `Here is the result: {"a":1}`, obj},
		{"prose before and after", "Result:\n```json\n{\"a\":1}\n```\nHope that helps!", obj},
		{"braces in prose before", `Sure {see below}: {"a":1}`, obj},
		{"braces in prose after", "{\"a\":1}\nNote: {x} is a placeholder", obj},
		{"braces both sides", "Use {name} like this: {\"a\":1} (and {that})", obj},
		{"brace as the last thing", `Result: {"a":1} }`, obj},
		{"two objects take the first", "{\"a\":1}\n{\"b\":2}", obj},
		{"first invalid, second valid", `{oops} {"a":1} {"b":2}`, obj},
		{"array of one", `[{"a":1}]`, obj},
		{"array of two takes the first", `[{"a":1},{"a":2}]`, obj},
		{"braces inside a string", `{"a":"}{"}`, map[string]any{"a": "}{"}},
		{"nested", `{"a":{"b":[1,{"c":null}]}}`, map[string]any{"a": map[string]any{"b": []any{1.0, map[string]any{"c": nil}}}}},
		{"empty object", `{}`, map[string]any{}},
		{"text then fenced nested", "Result:\n```json\n{\"a\": {\"b\": 1}}\n```\nHope that helps {smile}", map[string]any{"a": map[string]any{"b": 1.0}}},
		{"empty", ``, nil},
		{"blank", " \n ", nil},
		{"no braces", `just words`, nil},
		{"null", `null`, nil},
		{"array of numbers", `[1,2]`, nil},
		{"trailing comma", `{"a":1,}`, nil},
		{"single quotes", `{'a':1}`, nil},
		{"unterminated", `{"a":`, nil},
		{"lone braces", `{`, nil},
		{"reversed braces", `}{`, nil},
		{"only junk braces", `{x} {y}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractJSONObject(tc.in)
			if tc.want == nil {
				if ok || got != nil {
					t.Fatalf("got %#v, true; want no object", got)
				}
				return
			}
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, %v; want %#v", got, ok, tc.want)
			}
		})
	}

	// Only the first few opening braces are tried, so prose full of braces stays cheap.
	junk := strings.Repeat("{x}", maxAIJSONCandidates-1)
	if got, ok := extractJSONObject(junk + `{"a":1}`); !ok || !reflect.DeepEqual(got, obj) {
		t.Errorf("object after %d junk braces: %#v, %v", maxAIJSONCandidates-1, got, ok)
	}
	if got, ok := extractJSONObject(junk + `{x}{"a":1}`); ok {
		t.Errorf("object after %d junk braces was found: %#v", maxAIJSONCandidates, got)
	}
}

// The parser cannot be cancelled; hostile input of the largest size must not make it
// run away. The guard has far more headroom than the milliseconds it takes.
func TestExtractJSONObjectStaysCheapOnHostileInput(t *testing.T) {
	inputs := map[string]string{
		"braces":           strings.Repeat("{", maxAIResponseBytes),
		"open nesting":     strings.Repeat(`{"a":`, maxAIResponseBytes/5),
		"open arrays":      "{" + strings.Repeat("[", maxAIResponseBytes-1),
		"long open string": `{"a":"` + strings.Repeat("x", maxAIResponseBytes-10),
		"many candidates":  strings.Repeat(`{"a":1`, maxAIResponseBytes/6),
		"close braces":     strings.Repeat("}", maxAIResponseBytes),
	}
	start := time.Now()
	for name, in := range inputs {
		if len(in) > maxAIResponseBytes {
			t.Fatalf("%s: input has %d bytes", name, len(in))
		}
		if got, ok := extractJSONObject(in); ok {
			t.Errorf("%s: found %d keys", name, len(got))
		}
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("parsing hostile input took %v", elapsed)
	}
}

// An empty answer is no answer: a retry (FLOW_AI_OUTPUT_INVALID is retried) can fix it,
// an empty text field in the output cannot.
func TestAIStepEmptyTextAnswerFails(t *testing.T) {
	for _, text := range []string{"", " ", " \n\t "} {
		llm := &fakeLLM{responses: []LLMResponse{{Text: text, Model: "m", InputTokens: 1}}}
		_, err := execDef(aiDef(t), map[string]any{"prompt": "x"}, &Services{LLM: llm})
		if aiCode(t, err) != "FLOW_AI_OUTPUT_INVALID" || !strings.Contains(aiMessage(err), "empty answer") || llm.callCount() != 1 {
			t.Errorf("answer %q: err = %v after %d calls", text, err, llm.callCount())
		}
	}
	// Text that is only invalid bytes becomes a replacement character, which is text.
	llm := &fakeLLM{responses: []LLMResponse{{Text: "\xff"}}}
	if res, err := execDef(aiDef(t), map[string]any{"prompt": "x"}, &Services{LLM: llm}); err != nil || res.Output["text"] != "�" {
		t.Errorf("invalid bytes: %#v, %v", res.Output, err)
	}
}

// An output_mode that is neither text nor fields would silently run as text, so the
// validator reports it; a template is resolved at run time and cannot be judged yet.
func TestAIOutputModeValidation(t *testing.T) {
	def := aiDef(t)
	check := func(mode any, wantIssue bool) {
		t.Helper()
		params := map[string]any{"prompt": "x", "fields": []any{map[string]any{"name": "a"}}}
		if mode != nil {
			params["output_mode"] = mode
		}
		issues := def.Validate(&Node{ID: testNodeID(1), Params: params}, ValidateContext{Mode: ModePublish})
		if !wantIssue {
			if len(issues) != 0 {
				t.Errorf("output_mode %.30v: issues = %+v", mode, issues)
			}
			return
		}
		if len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].Severity != SeverityError ||
			issues[0].Param != "output_mode" || len(issues[0].Message) > 100 {
			t.Errorf("output_mode %.30v: issues = %+v", mode, issues)
		}
	}
	for _, mode := range []any{nil, "text", "fields", "", "{{trigger.mode}}", "x {{trigger.mode}}"} {
		check(mode, false)
	}
	for _, mode := range []any{"Fields", "TEXT", " fields", "json", "fields ", 5.0, true, []any{}, map[string]any{}, strings.Repeat("x", 1<<20), "{ {x} }"} {
		check(mode, true)
	}

	// A wrong mode is not also checked for fields (it runs as text); a flow-level validation reports it too.
	node := &Node{ID: testNodeID(1), Params: map[string]any{"output_mode": "json", "fields": "garbage"}}
	if issues := def.Validate(node, ValidateContext{Mode: ModePublish}); len(issues) != 1 {
		t.Errorf("issues = %+v", issues)
	}
	reg := newTestRegistry(t)
	if err := RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	b := newFlow("Mode")
	trg := b.node("start", "test.trigger", nil)
	ai := b.node("ai", TypeAIStep, map[string]any{"prompt": "x", "output_mode": "Fields"})
	b.edge(trg, PortOut, ai)
	var found bool
	for _, is := range Validate(b.build(), reg, ValidateContext{Mode: ModePublish}) {
		if is.Code == IssueParamInvalid && is.NodeID == ai && is.Param == "output_mode" {
			found = true
		}
	}
	if !found {
		t.Error("the flow validation does not report the wrong output mode")
	}

	// Execute holds the same line for a value that only became wrong at run time.
	for _, mode := range []string{"Fields", "json"} {
		llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
		_, err := execDef(def, map[string]any{"prompt": "x", "output_mode": mode}, &Services{LLM: llm})
		if aiCode(t, err) != "FLOW_PARAM_INVALID" || !strings.HasPrefix(aiMessage(err), "output_mode") || llm.callCount() != 0 {
			t.Errorf("Execute with output_mode %q: %v after %d calls", mode, err, llm.callCount())
		}
	}
	llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
	if _, err := execDef(def, map[string]any{"prompt": "x", "output_mode": ""}, &Services{LLM: llm}); err != nil {
		t.Errorf("an empty output_mode runs as text: %v", err)
	}
}
