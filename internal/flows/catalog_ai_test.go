package flows

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func aiDef(t *testing.T) *NodeDef {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	return lookupDef(t, reg, TypeAIStep)
}

func TestAIStepText(t *testing.T) {
	llm := &fakeLLM{responses: []LLMResponse{{Text: " Hallo Welt ", Model: "m1", InputTokens: 10, OutputTokens: 3}}}
	res, err := execDef(aiDef(t), map[string]any{"prompt": "Sag hallo", "instructions": "kurz", "model": "p1"}, &Services{LLM: llm})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := map[string]any{"text": "Hallo Welt", "model": "m1", "tokens": map[string]any{"input": 10.0, "output": 3.0}}
	if !reflect.DeepEqual(res.Output, want) {
		t.Fatalf("output = %#v", res.Output)
	}
	req := llm.calls[0]
	if req.Prompt != "Sag hallo" || req.System != "kurz" || req.Model != "p1" || req.JSONSchema != nil || req.RunID != "run_test" {
		t.Fatalf("request = %+v", req)
	}
}

func aiFieldsParams() map[string]any {
	return map[string]any{
		"prompt":      "Analysiere",
		"output_mode": "fields",
		"fields": []any{
			map[string]any{"name": "zusammenfassung", "type": "text", "description": "Kurzfassung"},
			map[string]any{"name": "anzahl", "type": "number"},
			map[string]any{"name": "wichtig", "type": "bool"},
			map[string]any{"name": "themen", "type": "list"},
		},
	}
}

func TestAIStepFields(t *testing.T) {
	llm := &fakeLLM{responses: []LLMResponse{{JSON: map[string]any{"zusammenfassung": "x", "anzahl": "3", "wichtig": true, "themen": []any{"a"}}}}}
	res, err := execDef(aiDef(t), aiFieldsParams(), &Services{LLM: llm})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.Output["zusammenfassung"] != "x" || res.Output["anzahl"] != 3.0 || res.Output["wichtig"] != true ||
		!reflect.DeepEqual(res.Output["themen"], []any{"a"}) {
		t.Fatalf("output = %#v", res.Output)
	}
	schema := llm.calls[0].JSONSchema
	props, _ := schema["properties"].(map[string]any)
	if schema["type"] != "object" || len(props) != 4 || !reflect.DeepEqual(schema["required"], []any{"zusammenfassung", "anzahl", "wichtig", "themen"}) {
		t.Fatalf("schema = %#v", schema)
	}
	if p, _ := props["anzahl"].(map[string]any); p["type"] != "number" {
		t.Fatalf("anzahl schema = %#v", props["anzahl"])
	}
}

func TestAIStepRepairsOnceThenFails(t *testing.T) {
	llm := &fakeLLM{responses: []LLMResponse{
		{JSON: map[string]any{"zusammenfassung": "x"}},
		{Text: "```json\n{\"zusammenfassung\":\"y\",\"anzahl\":1,\"wichtig\":false,\"themen\":[]}\n```"},
	}}
	res, err := execDef(aiDef(t), aiFieldsParams(), &Services{LLM: llm})
	if err != nil || res.Output["zusammenfassung"] != "y" || len(llm.calls) != 2 {
		t.Fatalf("repair = %#v, %v, %d calls", res.Output, err, len(llm.calls))
	}
	if !strings.Contains(llm.calls[1].Prompt, "previous answer was invalid") {
		t.Fatalf("repair prompt = %q", llm.calls[1].Prompt)
	}

	bad := &fakeLLM{responses: []LLMResponse{{Text: "kein JSON"}}}
	if _, err := execDef(aiDef(t), aiFieldsParams(), &Services{LLM: bad}); asNodeError(err).Code != "FLOW_AI_OUTPUT_INVALID" || len(bad.calls) != 2 {
		t.Fatalf("invalid twice = %v after %d calls", err, len(bad.calls))
	}
}

func TestAIStepErrors(t *testing.T) {
	if _, err := execDef(aiDef(t), map[string]any{"prompt": "x"}, &Services{}); asNodeError(err).Code != "FLOW_AI_UNAVAILABLE" {
		t.Fatalf("no LLM = %v", err)
	}
	if _, err := execDef(aiDef(t), map[string]any{"prompt": "  "}, &Services{LLM: &fakeLLM{}}); asNodeError(err).Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("empty prompt = %v", err)
	}
	budget := &fakeLLM{err: NewNodeError("FLOW_BUDGET_EXCEEDED", "budget used up")}
	if _, err := execDef(aiDef(t), map[string]any{"prompt": "x"}, &Services{LLM: budget}); asNodeError(err).Code != "FLOW_BUDGET_EXCEEDED" {
		t.Fatalf("budget = %v", err)
	}
	plain := &fakeLLM{err: errors.New("provider down")}
	if _, err := execDef(aiDef(t), map[string]any{"prompt": "x"}, &Services{LLM: plain}); asNodeError(err).Code != "FLOW_NODE_FAILED" {
		t.Fatalf("provider error = %v", err)
	}
}

func TestAIStepValidationAndFields(t *testing.T) {
	def := aiDef(t)
	params := aiFieldsParams()
	params["fields"] = []any{
		map[string]any{"name": "Zusammen fassung"},
		map[string]any{"name": "a"},
		map[string]any{"name": "a"},
		map[string]any{"name": "tokens"},
		map[string]any{"name": "b", "type": "date"},
	}
	issues := def.Validate(&Node{ID: testNodeID(1), Params: params}, ValidateContext{Mode: ModePublish})
	if len(issues) != 4 {
		t.Fatalf("issues = %+v", issues)
	}
	text := def.FieldsOf(&Node{Params: map[string]any{}})
	if text[0].Name != "text" || !text[0].Primary {
		t.Fatalf("text-mode fields = %+v", text)
	}
	fields := def.FieldsOf(&Node{Params: aiFieldsParams()})
	if fields[0].Name != "zusammenfassung" || !fields[0].Primary || fields[1].Name != "anzahl" || fields[len(fields)-1].Name != "model" {
		t.Fatalf("fields-mode fields = %+v", fields)
	}
}
