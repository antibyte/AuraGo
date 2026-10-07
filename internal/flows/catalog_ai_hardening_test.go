package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// aiCode returns the code of err, failing the test for a nil error or a typed nil.
func aiCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("want an error, got nil")
	}
	var ne *NodeError
	if !errors.As(err, &ne) || ne == nil {
		t.Fatalf("error %T %v is not a non-nil *NodeError", err, err)
	}
	return ne.Code
}

func aiMessage(err error) string {
	var ne *NodeError
	if errors.As(err, &ne) && ne != nil {
		return ne.Message
	}
	return ""
}

func aiFieldList(n int) []any {
	list := make([]any, n)
	for i := range list {
		list[i] = map[string]any{"name": fmt.Sprintf("f%d", i)}
	}
	return list
}

// aiAnswer builds a model answer that satisfies the declared fields.
func aiAnswer(fields []aiField) map[string]any {
	obj := map[string]any{}
	for _, f := range fields {
		switch f.Type {
		case "number":
			obj[f.Name] = 1.0
		case "bool":
			obj[f.Name] = true
		case "list":
			obj[f.Name] = []any{}
		default:
			obj[f.Name] = "t"
		}
	}
	return obj
}

// A field name or type comes from the document and can be anything; the issue text
// repeats it only quoted and cut, so a huge or hostile value cannot flood the issue
// list, break its line or its encoding.
func TestAIFieldIssuesEchoBoundedText(t *testing.T) {
	def := aiDef(t)
	huge := strings.Repeat("ä", 1<<18)
	params := map[string]any{"output_mode": "fields", "fields": []any{
		map[string]any{"name": "a"},
		map[string]any{"name": "a"},
		map[string]any{"name": "b", "type": huge},
		map[string]any{"name": "c", "type": "\xff\xfe\n\x00"},
		map[string]any{"name": huge},
		map[string]any{"name": "d", "description": huge},
	}}
	issues := def.Validate(&Node{ID: testNodeID(1), Params: params}, ValidateContext{Mode: ModePublish})
	wantParams := []string{"fields[1].name", "fields[2].type", "fields[3].type", "fields[4].name", "fields[5].description"}
	if len(issues) != len(wantParams) {
		t.Fatalf("issues = %+v", issues)
	}
	for i, is := range issues {
		if is.Param != wantParams[i] || is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != testNodeID(1) {
			t.Errorf("issue %d = %+v, want an error on %s", i, is, wantParams[i])
		}
		if len(is.Message) > maxEchoMessageBytes || !utf8.ValidString(is.Message) || strings.ContainsAny(is.Message, "\n\r\x00") {
			t.Errorf("issue %d message is not bounded text: %.200q", i, is.Message)
		}
	}
	if issues[0].Message != `duplicate field name "a"` {
		t.Errorf("duplicate message = %q", issues[0].Message)
	}
	if !strings.HasPrefix(issues[1].Message, "unknown field type \"ää") || !strings.HasSuffix(issues[1].Message, `…"`) {
		t.Errorf("type message = %.100q", issues[1].Message)
	}
	if issues[2].Message != `unknown field type "\xff\xfe\n\x00"` {
		t.Errorf("garbage type message = %q", issues[2].Message)
	}
}

func TestAIFieldIssueCodes(t *testing.T) {
	def := aiDef(t)
	check := func(fields any, want ...string) {
		t.Helper()
		params := map[string]any{"output_mode": "fields", "fields": fields}
		issues := def.Validate(&Node{ID: testNodeID(1), Params: params}, ValidateContext{Mode: ModeDraft})
		var got []string
		for _, is := range issues {
			if is.Severity != SeverityError {
				t.Errorf("%v: issue %+v is not an error", fields, is)
			}
			got = append(got, is.Param+" "+is.Code)
		}
		if strings.Join(got, ";") != strings.Join(want, ";") {
			t.Errorf("%v: issues = %v, want %v", fields, got, want)
		}
	}
	check([]any{map[string]any{"name": "ok"}})
	check([]any{map[string]any{"name": " "}}, "fields[0].name "+IssueParamRequired)
	check([]any{map[string]any{}}, "fields[0].name "+IssueParamRequired)
	check([]any{"text"}, "fields[0].name "+IssueParamRequired)
	check([]any{map[string]any{"name": []any{"x"}}}, "fields[0].name "+IssueParamInvalid)
	check([]any{map[string]any{"name": strings.Repeat("a", 41)}}, "fields[0].name "+IssueParamInvalid)
	check([]any{map[string]any{"name": strings.Repeat("a", 40)}})
	check([]any{map[string]any{"name": "model"}}, "fields[0].name "+IssueParamInvalid)
	check([]any{map[string]any{"name": "a", "type": map[string]any{}}}, "fields[0].type "+IssueParamInvalid)
	check([]any{map[string]any{"name": "a", "description": []any{}}}, "fields[0].description "+IssueParamInvalid)
	check([]any{map[string]any{"name": "a", "description": strings.Repeat("ä", maxAIFieldDescRunes)}})
	check([]any{map[string]any{"name": "a", "description": strings.Repeat("ä", maxAIFieldDescRunes+1)}}, "fields[0].description "+IssueParamInvalid)
	check("a, b", "fields "+IssueParamInvalid)
	check(map[string]any{"name": "a"}, "fields "+IssueParamInvalid)
	check(nil)
	check([]any{})
	// Text mode never looks at the fields.
	node := &Node{ID: testNodeID(1), Params: map[string]any{"output_mode": "text", "fields": "garbage"}}
	if issues := def.Validate(node, ValidateContext{Mode: ModePublish}); len(issues) != 0 {
		t.Errorf("text mode issues = %+v", issues)
	}
}

func TestAIStepPromptSizeLimit(t *testing.T) {
	half := maxAIPromptBytes / 2
	cases := []struct {
		name         string
		prompt, inst string
		ok           bool
	}{
		{"prompt at the limit", strings.Repeat("p", maxAIPromptBytes), "", true},
		{"prompt over the limit", strings.Repeat("p", maxAIPromptBytes+1), "", false},
		{"instructions push it over", "x", strings.Repeat("i", maxAIPromptBytes), false},
		{"together at the limit", strings.Repeat("p", half), strings.Repeat("i", half), true},
		{"together over the limit", strings.Repeat("p", half), strings.Repeat("i", half+1), false},
		{"limit counts bytes, not runes", strings.Repeat("ä", half+1), "", false},
		{"8 MiB from a template", strings.Repeat("p", 8<<20), "", false},
		{"padding is trimmed first", strings.Repeat(" ", 2*maxAIPromptBytes) + "x", strings.Repeat("\n", 2*maxAIPromptBytes), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
			_, err := execDef(aiDef(t), map[string]any{"prompt": tc.prompt, "instructions": tc.inst}, &Services{LLM: llm})
			if tc.ok {
				if err != nil || llm.callCount() != 1 {
					t.Fatalf("err = %v, %d calls; want one call", err, llm.callCount())
				}
				return
			}
			if aiCode(t, err) != "FLOW_PARAM_INVALID" || llm.callCount() != 0 {
				t.Fatalf("err = %v, %d calls; want FLOW_PARAM_INVALID and no call", err, llm.callCount())
			}
			if msg := aiMessage(err); len(msg) > 200 || !strings.Contains(msg, "prompt") {
				t.Fatalf("message = %q", msg)
			}
		})
	}
}

func TestAIStepModelNameLimit(t *testing.T) {
	llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
	if _, err := execDef(aiDef(t), map[string]any{"prompt": "x", "model": strings.Repeat("m", maxAIModelBytes)}, &Services{LLM: llm}); err != nil {
		t.Fatalf("model at the limit: %v", err)
	}
	_, err := execDef(aiDef(t), map[string]any{"prompt": "x", "model": strings.Repeat("m", maxAIModelBytes+1)}, &Services{LLM: llm})
	if aiCode(t, err) != "FLOW_PARAM_INVALID" || llm.callCount() != 1 || len(aiMessage(err)) > 200 {
		t.Fatalf("long model = %v after %d calls", err, llm.callCount())
	}
}

func TestAIStepFieldCountLimit(t *testing.T) {
	def := aiDef(t)
	vc := ValidateContext{Mode: ModePublish}
	params := func(n int) map[string]any {
		return map[string]any{"prompt": "x", "output_mode": "fields", "fields": aiFieldList(n)}
	}

	// At the limit: valid, runs, and the editor lists every field plus tokens and model.
	at := params(maxAIFields)
	if issues := def.Validate(&Node{ID: testNodeID(1), Params: at}, vc); len(issues) != 0 {
		t.Fatalf("%d fields: issues = %+v", maxAIFields, issues)
	}
	llm := &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(at["fields"]))}}}
	if _, err := execDef(def, at, &Services{LLM: llm}); err != nil {
		t.Fatalf("%d fields: %v", maxAIFields, err)
	}
	if got := len(def.FieldsOf(&Node{Params: at})); got != maxAIFields+2 {
		t.Fatalf("FieldsOf = %d fields", got)
	}
	if props := llm.callAt(0).JSONSchema["properties"].(map[string]any); len(props) != maxAIFields {
		t.Fatalf("schema has %d properties", len(props))
	}

	// One more: an issue, a failed run without a model call, and a bounded field list.
	for _, n := range []int{maxAIFields + 1, 1000, 100000} {
		over := params(n)
		issues := def.Validate(&Node{ID: testNodeID(1), Params: over}, vc)
		if len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].Severity != SeverityError || issues[0].Param != "fields" {
			t.Fatalf("%d fields: issues = %+v", n, issues)
		}
		llm := &fakeLLM{}
		_, err := execDef(def, over, &Services{LLM: llm})
		if aiCode(t, err) != "FLOW_PARAM_INVALID" || llm.callCount() != 0 || !strings.Contains(aiMessage(err), "50") {
			t.Fatalf("%d fields: err = %v after %d calls", n, err, llm.callCount())
		}
		if got := len(def.FieldsOf(&Node{Params: over})); got != maxAIFields+2 {
			t.Fatalf("%d fields: FieldsOf returned %d entries, want %d", n, got, maxAIFields+2)
		}
	}
}

func TestAIStepResponseTextLimit(t *testing.T) {
	def := aiDef(t)
	atLimit := strings.Repeat("a", maxAIResponseBytes)
	over := atLimit + "a"

	llm := &fakeLLM{responses: []LLMResponse{{Text: atLimit}}}
	res, err := execDef(def, map[string]any{"prompt": "x"}, &Services{LLM: llm})
	if err != nil || len(res.Output["text"].(string)) != maxAIResponseBytes {
		t.Fatalf("answer at the limit: %v", err)
	}

	llm = &fakeLLM{responses: []LLMResponse{{Text: over}}}
	_, err = execDef(def, map[string]any{"prompt": "x"}, &Services{LLM: llm})
	if aiCode(t, err) != "FLOW_OUTPUT_TOO_LARGE" || llm.callCount() != 1 || len(aiMessage(err)) > 200 {
		t.Fatalf("text mode over the limit = %v after %d calls", err, llm.callCount())
	}

	// Fields mode: the oversized answer is refused at once, without a repair call
	// (which would cost the same again) and without parsing it.
	llm = &fakeLLM{responses: []LLMResponse{{Text: `{"zusammenfassung":"` + over + `"}`}}}
	_, err = execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if aiCode(t, err) != "FLOW_OUTPUT_TOO_LARGE" || llm.callCount() != 1 {
		t.Fatalf("fields mode over the limit = %v after %d calls", err, llm.callCount())
	}
	llm = &fakeLLM{responses: []LLMResponse{{Text: "kein JSON"}, {Text: over}}}
	_, err = execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if aiCode(t, err) != "FLOW_OUTPUT_TOO_LARGE" || llm.callCount() != 2 {
		t.Fatalf("oversized repair answer = %v after %d calls", err, llm.callCount())
	}

	// The parser guards itself as well, for any other caller.
	pad := strings.Repeat(" ", maxAIResponseBytes)
	if _, ok := extractJSONObject(`{"a":1}` + pad); ok {
		t.Error("extractJSONObject parsed text over the limit")
	}
	exact := `{"a":"` + strings.Repeat("x", maxAIResponseBytes-len(`{"a":""}`)) + `"}`
	if len(exact) != maxAIResponseBytes {
		t.Fatalf("test input has %d bytes", len(exact))
	}
	if m, ok := extractJSONObject(exact); !ok || len(m["a"].(string)) == 0 {
		t.Error("extractJSONObject refused text exactly at the limit")
	}
}

func TestAIStepOutputIsValidUTF8(t *testing.T) {
	def := aiDef(t)
	check := func(label string, output map[string]any) {
		t.Helper()
		raw, err := json.Marshal(output)
		if err != nil || !utf8.Valid(raw) {
			t.Fatalf("%s: output does not encode: %v", label, err)
		}
		var walk func(v any)
		walk = func(v any) {
			switch x := v.(type) {
			case string:
				if !utf8.ValidString(x) {
					t.Errorf("%s: invalid UTF-8 string %q in the output", label, x)
				}
			case map[string]any:
				for _, item := range x {
					walk(item)
				}
			case []any:
				for _, item := range x {
					walk(item)
				}
			}
		}
		walk(output)
	}

	llm := &fakeLLM{responses: []LLMResponse{{Text: " ok\xff\xfe ", Model: "m\xc3"}}}
	res, err := execDef(def, map[string]any{"prompt": "x"}, &Services{LLM: llm})
	if err != nil {
		t.Fatal(err)
	}
	check("text mode", res.Output)
	// A run of invalid bytes becomes one replacement character.
	if res.Output["text"] != "ok�" || res.Output["model"] != "m�" {
		t.Errorf("output = %#v", res.Output)
	}

	llm = &fakeLLM{responses: []LLMResponse{{Text: "{\"zusammenfassung\":\"a\xffb\",\"anzahl\":1,\"wichtig\":true,\"themen\":[]}", Model: "m\xff"}}}
	res, err = execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if err != nil {
		t.Fatal(err)
	}
	check("fields mode, parsed text", res.Output)

	// A structured answer from the provider is not parsed text: its strings are cleaned too.
	llm = &fakeLLM{responses: []LLMResponse{{JSON: map[string]any{"zusammenfassung": "a\xffb", "anzahl": 1.0, "wichtig": true, "themen": []any{}}}}}
	res, err = execDef(def, aiFieldsParams(), &Services{LLM: llm})
	if err != nil {
		t.Fatal(err)
	}
	check("fields mode, structured answer", res.Output)
	if res.Output["zusammenfassung"] != "a�b" {
		t.Errorf("zusammenfassung = %q", res.Output["zusammenfassung"])
	}
}

// What the model wrote never goes back into the repair prompt, and whatever a
// problem text quotes is cut and valid.
func TestAIStepRepairPromptIsBounded(t *testing.T) {
	garbage := "LEAKMARKER" + strings.Repeat("kein JSON \xff ", 40000)
	llm := &fakeLLM{responses: []LLMResponse{{Text: garbage}, {Text: garbage}}}
	_, err := execDef(aiDef(t), aiFieldsParams(), &Services{LLM: llm})
	if aiCode(t, err) != "FLOW_AI_OUTPUT_INVALID" || llm.callCount() != 2 {
		t.Fatalf("err = %v after %d calls", err, llm.callCount())
	}
	repair := llm.callAt(1).Prompt
	if strings.Contains(repair, "LEAKMARKER") || len(repair) > len("Analysiere")+500 || !utf8.ValidString(repair) {
		t.Fatalf("repair prompt (%d bytes) = %.300q", len(repair), repair)
	}
	if msg := aiMessage(err); strings.Contains(msg, "LEAKMARKER") || utf8.RuneCountInString(msg) > maxAIProblemRunes+1 || !utf8.ValidString(msg) {
		t.Fatalf("error message = %.300q", msg)
	}

	long := aiProblemText(errors.New(strings.Repeat("ä", 5000) + "\xff"))
	if n := utf8.RuneCountInString(long); n != maxAIProblemRunes+1 || !utf8.ValidString(long) || !strings.HasSuffix(long, "…") {
		t.Errorf("problem text has %d runes, valid=%v", n, utf8.ValidString(long))
	}
	if got := aiProblemText(errors.New("short \xff")); got != "short �" {
		t.Errorf("short problem text = %q", got)
	}
	if got := aiProblemText(errors.New(strings.Repeat("a", maxAIProblemRunes))); len(got) != maxAIProblemRunes {
		t.Errorf("a problem text of exactly the limit was cut to %d bytes", len(got))
	}
}

func TestAIStepProviderErrors(t *testing.T) {
	def := aiDef(t)
	run := func(err error, params map[string]any) error {
		_, got := execDef(def, params, &Services{LLM: &fakeLLM{err: err}})
		return got
	}
	text := map[string]any{"prompt": "x"}

	// A typed nil error (a stepper returning a nil *NodeError variable) is a failure with a
	// real message, never a nil pointer.
	var typedNil *NodeError
	got := run(typedNil, text)
	if code := aiCode(t, got); code != "FLOW_NODE_FAILED" || aiMessage(got) == "" {
		t.Errorf("typed nil error = %v", got)
	}

	// Provider text can be huge and invalid; the message is cut and valid.
	got = run(errors.New(strings.Repeat("ä", 5000)+"\xff"), text)
	if msg := aiMessage(got); aiCode(t, got) != "FLOW_NODE_FAILED" || utf8.RuneCountInString(msg) > maxToolMessageRunes+1 || !utf8.ValidString(msg) {
		t.Errorf("huge provider error: %d runes, valid=%v", utf8.RuneCountInString(msg), utf8.ValidString(msg))
	}

	// A NodeError keeps its code, gets a bounded message, and is not modified (the stepper may share it).
	hostile := &NodeError{Code: "FLOW_BUDGET_EXCEEDED", Message: strings.Repeat("b", 5000)}
	got = run(hostile, text)
	if aiCode(t, got) != "FLOW_BUDGET_EXCEEDED" || utf8.RuneCountInString(aiMessage(got)) > maxToolMessageRunes+1 || len(hostile.Message) != 5000 {
		t.Errorf("budget error = %v, original message %d bytes", got, len(hostile.Message))
	}
	got = run(&NodeError{Message: "no code"}, text)
	if aiCode(t, got) != "FLOW_NODE_FAILED" {
		t.Errorf("empty code = %v", got)
	}
	got = run(fmt.Errorf("provider: %w", context.DeadlineExceeded), text)
	if aiCode(t, got) != "FLOW_NODE_TIMEOUT" {
		t.Errorf("deadline = %v", got)
	}

	// In fields mode a failing repair call fails the node with the provider's code, after both calls.
	llm := &fakeLLM{responses: []LLMResponse{{Text: "kein JSON"}}}
	failing := &aiFailSecond{inner: llm, err: NewNodeError("FLOW_BUDGET_EXCEEDED", "budget used up")}
	_, err := execDef(def, aiFieldsParams(), &Services{LLM: failing})
	if aiCode(t, err) != "FLOW_BUDGET_EXCEEDED" || failing.calls != 2 {
		t.Errorf("repair call failure = %v after %d calls", err, failing.calls)
	}
}

// aiFailSecond answers the first call from inner and fails the second.
type aiFailSecond struct {
	inner LLMStepper
	err   error
	calls int
}

func (s *aiFailSecond) Step(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	s.calls++
	if s.calls == 2 {
		return LLMResponse{}, s.err
	}
	return s.inner.Step(ctx, req)
}

// aiCancelingLLM cancels the run while its first answer is on the way back.
type aiCancelingLLM struct {
	cancel context.CancelFunc
	calls  int
}

func (l *aiCancelingLLM) Step(context.Context, LLMRequest) (LLMResponse, error) {
	l.calls++
	l.cancel()
	return LLMResponse{Text: "kein JSON"}, nil
}

func TestAIStepHonoursCancellation(t *testing.T) {
	def := aiDef(t)
	exec := func(ctx context.Context, params map[string]any, llm LLMStepper) error {
		_, err := def.Execute(ctx, ExecInput{
			Node: &Node{ID: testNodeID(1), Type: TypeAIStep}, Params: withDefaults(def, params),
			Services: &Services{LLM: llm}, Run: RunInfo{ID: "run_test", FlowID: "flow_test"},
		})
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	llm := &fakeLLM{}
	if err := exec(ctx, map[string]any{"prompt": "x"}, llm); err == nil || llm.callCount() != 0 {
		t.Fatalf("cancelled before the call: %v, %d calls", err, llm.callCount())
	}
	_ = aiCode(t, exec(ctx, aiFieldsParams(), llm))

	// Cancelled during the first call of fields mode: no second (repair) call is paid for.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	canceling := &aiCancelingLLM{cancel: cancel}
	if err := exec(ctx, aiFieldsParams(), canceling); err == nil || canceling.calls != 1 {
		t.Fatalf("cancelled between attempts: %v after %d calls", err, canceling.calls)
	}
}

// Execute applies the validator's field rules, so a test run of a draft cannot send a
// half-finished field list to the model.
func TestAIStepExecuteRejectsInvalidFields(t *testing.T) {
	def := aiDef(t)
	cases := []struct {
		name   string
		fields any
		want   string
	}{
		{"duplicate", []any{map[string]any{"name": "a"}, map[string]any{"name": "a"}}, `fields[1].name: duplicate field name "a"`},
		{"reserved", []any{map[string]any{"name": "model"}}, `fields[0].name: "model" is reserved`},
		{"malformed", []any{map[string]any{"name": "Bad Name"}}, "fields[0].name: use lowercase"},
		{"blank", []any{map[string]any{"name": "a"}, map[string]any{"name": "  "}}, "fields[1].name: field name is required"},
		{"unknown type", []any{map[string]any{"name": "a", "type": "date"}}, `fields[0].type: unknown field type "date"`},
		{"long description", []any{map[string]any{"name": "a", "description": strings.Repeat("d", maxAIFieldDescRunes+1)}}, "fields[0].description: "},
		{"not a list", "a, b", "fields: the output fields must be a list"},
		{"not objects", []any{"a"}, "fields[0].name: field name is required"},
		{"empty list", []any{}, "define at least one output field"},
		{"missing", nil, "define at least one output field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := &fakeLLM{}
			params := map[string]any{"prompt": "x", "output_mode": "fields"}
			if tc.fields != nil {
				params["fields"] = tc.fields
			}
			_, err := execDef(def, params, &Services{LLM: llm})
			if aiCode(t, err) != "FLOW_PARAM_INVALID" || !strings.HasPrefix(aiMessage(err), tc.want) || llm.callCount() != 0 {
				t.Fatalf("err = %v after %d calls, want %q", err, llm.callCount(), tc.want)
			}
		})
	}
}

// ai.step output is generated by a model that reads the flow's data, so it is untrusted;
// the prompt is not a sensitive sink (the instructions are, see
// TestC14AIStepInstructionsAreASink), and the node has no outward effect and copies no inputs.
func TestAIStepTaintAndEffects(t *testing.T) {
	reg := newTestRegistry(t)
	if err := RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	def := lookupDef(t, reg, TypeAIStep)
	if !def.UntrustedOutput || def.Trigger {
		t.Fatalf("UntrustedOutput = %v, Trigger = %v", def.UntrustedOutput, def.Trigger)
	}
	var walk func(specs []ParamSpec)
	walk = func(specs []ParamSpec) {
		for _, p := range specs {
			if p.SensitiveSink != (p.Name == "instructions") {
				t.Errorf("parameter %s: SensitiveSink = %v", p.Name, p.SensitiveSink)
			}
			walk(p.Fields)
		}
	}
	walk(def.Params)
	if got := def.EffectsOf(&Node{Params: aiFieldsParams()}); len(got) != 0 {
		t.Errorf("effects = %v", got)
	}
	if passesInputs(&Node{Type: TypeAIStep, Params: map[string]any{"keep_input": true}}) {
		t.Error("ai.step does not copy its inputs")
	}

	// A trusted trigger -> ai.step -> shell flow warns about the model's output reaching the command.
	b := newFlow("Lint")
	trg := b.node("start", "test.trigger", nil)
	ai := b.node("ai", TypeAIStep, map[string]any{"prompt": "x"})
	sink := b.node("shell", "test.sink", map[string]any{"command": "{{ai.text}}"})
	b.edge(trg, PortOut, ai)
	b.edge(ai, PortOut, sink)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].NodeID != sink {
		t.Fatalf("lint issues = %+v", issues)
	}

	// The output carries only the declared fields, whatever flows in.
	llm := &fakeLLM{responses: []LLMResponse{{Text: "ok", Model: "m"}}}
	in := NodeInput{NodeID: "n_x", Key: "web", Port: PortOut, Output: map[string]any{"secret": "s"}}
	res, err := execDef(def, map[string]any{"prompt": "x"}, &Services{LLM: llm}, in)
	if err != nil || len(res.Output) != 3 || res.Output["secret"] != nil {
		t.Fatalf("output = %#v, %v", res.Output, err)
	}
}

// Resolved parameters can share memory with run data: Execute must leave them alone.
func TestAIStepDoesNotModifyParams(t *testing.T) {
	params := aiFieldsParams()
	params["instructions"] = "  kurz  "
	before, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	llm := &fakeLLM{responses: []LLMResponse{{Text: "kein JSON"}, {JSON: aiAnswer(aiFields(params["fields"]))}}}
	if _, err := execDef(aiDef(t), params, &Services{LLM: llm}); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(params)
	if string(before) != string(after) {
		t.Fatalf("params changed:\nbefore %s\nafter  %s", before, after)
	}
	if got := llm.callAt(0).System; !strings.HasPrefix(got, "kurz\n\nAnswer with exactly one JSON object") {
		t.Fatalf("system prompt = %q", got)
	}
	if llm.callAt(1).System != llm.callAt(0).System {
		t.Error("the repair call changed the system prompt")
	}
}

// The instructions become the model's system message with the author's authority, so
// untrusted data there gets the lint's warning; in the prompt it does not.
func TestC14AIStepInstructionsAreASink(t *testing.T) {
	reg := newTestRegistry(t)
	if err := RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	b := newFlow("Lint")
	trg := b.node("hook", "test.untrusted_trigger", nil)
	ai := b.node("ai", TypeAIStep, map[string]any{"prompt": "Fasse zusammen: {{trigger.body}}", "instructions": "Antworte wie {{trigger.style}}"})
	b.edge(trg, PortOut, ai)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].NodeID != ai || issues[0].Param != "instructions" ||
		issues[0].Severity != SeverityWarning {
		t.Fatalf("lint issues = %+v", issues)
	}
}
