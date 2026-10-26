package flows

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestFB4LiteralCredentialHeaderIsAPublishWarning(t *testing.T) {
	node := &Node{ID: testNodeID(1), Params: map[string]any{"url": "https://api.example/x", "headers": map[string]any{
		"Authorization": "Bearer abc123", "X-Api-Key": "k-123", "Cookie": "sid=1", "X-Request-Id": "r1",
		"X-Auth-Token": "{{trigger.data.token}}", "Proxy-Authorization": ""}}}
	issues := httpNodeDef(t).Validate(node, ValidateContext{Mode: ModePublish})
	if len(issues) != 3 {
		t.Fatalf("issues = %+v, want one warning each for Authorization, Cookie and X-Api-Key", issues)
	}
	for i, want := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		is := issues[i]
		if is.Code != IssueLiteralCredential || is.Severity != SeverityWarning || is.Param != "headers" || !strings.Contains(is.Message, want) {
			t.Errorf("issue %d = %+v, want a warning about %s", i, is, want)
		}
	}
}

func TestFB4CredentialHeadersAreRedactedInExportsAndRunLogs(t *testing.T) {
	headers := map[string]any{"Authorization": "Bearer abc123", "X-Auth-Token": "{{trigger.data.token}}", "Accept": "text/plain"}
	params := map[string]any{"url": "https://api.example/x", "headers": headers}
	h := redactCredentialHeaders(params, true)["headers"].(map[string]any)
	if h["Authorization"] != redactedText || h["X-Auth-Token"] != "{{trigger.data.token}}" || h["Accept"] != "text/plain" {
		t.Fatalf("export redaction = %v", h)
	}
	if headers["Authorization"] != "Bearer abc123" {
		t.Fatal("the input map must not be modified")
	}
	if h2 := redactCredentialHeaders(params, false)["headers"].(map[string]any); h2["X-Auth-Token"] != redactedText {
		t.Fatalf("resolved params keep a credential: %v", h2)
	}
	plain := map[string]any{"headers": map[string]any{"Accept": "x"}}
	if got := redactCredentialHeaders(plain, false); reflect.ValueOf(got).Pointer() != reflect.ValueOf(plain).Pointer() {
		t.Fatal("params without credentials must be returned as they are")
	}
	b := newFlow("Export")
	tr := b.node("start", "test.trigger", nil)
	call := b.node("call", "test.echo", map[string]any{"value": "x", "headers": map[string]any{"Authorization": "Bearer abc123"}})
	b.edge(tr, PortOut, call)
	f := b.build()
	out, err := RedactCredentials(f)
	if err != nil || out.NodeByID(call).Params["headers"].(map[string]any)["Authorization"] != redactedText {
		t.Fatalf("RedactCredentials = %v, %v", out, err)
	}
	if f.NodeByID(call).Params["headers"].(map[string]any)["Authorization"] != "Bearer abc123" {
		t.Fatal("the export must not change the draft")
	}
	res, _ := runWith(context.Background(), newTestEngine(newTestRegistry(t), nil, 4), f, RunRequest{TriggerNode: tr})
	if got := stepOf(res, call).Params["headers"].(map[string]any)["Authorization"]; got != redactedText {
		t.Fatalf("stored step params keep the credential: %v", got)
	}
}
