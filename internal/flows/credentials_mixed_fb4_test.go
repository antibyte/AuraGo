package flows

import (
	"context"
	"testing"
)

func TestFB4MixedCredentialValuesWarnAndAreRedacted(t *testing.T) {
	mixed := map[string]any{
		"Cookie":        "session=abc123; lang={{trigger.data.lang}}",
		"Authorization": "Token sk-live-123 {{trigger.data.suffix}}",
	}
	node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://api.example/x", "headers": mixed}}
	issues := httpNodeDef(t).Validate(node, ValidateContext{Mode: ModePublish})
	if len(issues) != 2 {
		t.Fatalf("issues = %+v, want a warning each for Authorization and Cookie", issues)
	}
	h := redactCredentialHeaders(node.Params, true)["headers"].(map[string]any)
	if h["Cookie"] != redactedText || h["Authorization"] != redactedText {
		t.Fatalf("export keeps a mixed credential: %v", h)
	}
	if mixed["Cookie"] == redactedText {
		t.Fatal("the input map must not be modified")
	}
}

func TestFB4PureTemplateCredentialValuesStay(t *testing.T) {
	pure := map[string]any{
		"Authorization": "Bearer {{secret_ref.api}}",
		"X-Auth-Token":  "{{trigger.data.token}}",
		"Cookie":        "  {{ trigger.data.cookie }}  ",
		"X-Api-Key":     "basic {{trigger.data.k}}",
	}
	node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://api.example/x", "headers": pure}}
	if issues := httpNodeDef(t).Validate(node, ValidateContext{Mode: ModePublish}); len(issues) != 0 {
		t.Fatalf("pure templates warned: %+v", issues)
	}
	h := redactCredentialHeaders(node.Params, true)["headers"].(map[string]any)
	for k, v := range pure {
		if h[k] != v {
			t.Errorf("export changed %s: %v", k, h[k])
		}
	}
	// An escaped brace is typed-in text, not a template.
	if got := literalCredentialHeaders(map[string]any{"Authorization": `Bearer \{{abc`}); len(got) != 1 {
		t.Fatalf("escaped braces = %v, want a warning", got)
	}
}

func TestFB4RunReturnsRedactedDocument(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	f := createStoredFlow(t, s.store, "flow_aaaaaaaaca")
	f.Nodes[0].Params = map[string]any{"headers": map[string]any{"Authorization": "Bearer abc123", "X-Auth-Token": "{{trigger.data.token}}"}}
	rec := RunRecord{ID: "run_aaaaaaaaaaca", FlowID: f.ID, Revision: 1, Mode: ModeTest, Status: RunSuccess, TriggerNode: f.Nodes[0].ID, StartedAt: storeNow}
	if err := s.store.CreateRun(ctx, rec, f); err != nil {
		t.Fatal(err)
	}
	detail, err := s.Run(ctx, rec.ID, true)
	if err != nil || detail.Doc == nil {
		t.Fatalf("Run = %+v, %v", detail, err)
	}
	h := detail.Doc.Nodes[0].Params["headers"].(map[string]any)
	if h["Authorization"] != redactedText || h["X-Auth-Token"] != "{{trigger.data.token}}" {
		t.Fatalf("run document headers = %v", h)
	}
	stored, err := s.store.GetRunDoc(ctx, rec.ID)
	if err != nil || stored.Nodes[0].Params["headers"].(map[string]any)["Authorization"] != "Bearer abc123" {
		t.Fatalf("the stored document must stay as it is: %v, %v", stored, err)
	}
}
