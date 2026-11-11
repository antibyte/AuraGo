package flows

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type credentialCase struct {
	Name string `json:"name"`
	Node struct {
		ID     string         `json:"id"`
		Type   string         `json:"type"`
		Params map[string]any `json:"params"`
	} `json:"node"`
	Findings []struct {
		Param    string `json:"param"`
		Location string `json:"location"`
		Path     string `json:"path"`
		Movable  bool   `json:"movable"`
	} `json:"findings"`
	MaskedParams map[string]any `json:"masked_params"`
}

func loadCredentialCases(t *testing.T) []credentialCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/credential_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []credentialCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 18 {
		t.Fatalf("only %d shared vectors; the rule needs at least 18", len(cases))
	}
	return cases
}

// The vectors are shared with the client rule (scripts/test-easydrag-credentials.mjs).
func TestCredentialRuleSharedVectors(t *testing.T) {
	for _, c := range loadCredentialCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			n := Node{ID: c.Node.ID, Type: c.Node.Type, Params: c.Node.Params}
			var got, want []string
			for _, f := range FindLiteralCredentials(&Flow{Nodes: []Node{n}}) {
				if f.NodeID != c.Node.ID || f.Value == "" {
					t.Errorf("finding %+v lacks its node or value", f)
				}
				got = append(got, fmt.Sprintf("%s|%s|%s|%t", f.Param, f.Location, f.Path, f.Movable))
			}
			for _, f := range c.Findings {
				want = append(want, fmt.Sprintf("%s|%s|%s|%t", f.Param, f.Location, f.Path, f.Movable))
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("findings = %v, want %v", got, want)
			}
			if masked := redactCredentialParams(n.Type, n.Params, true); !reflect.DeepEqual(masked, c.MaskedParams) {
				t.Errorf("masked params = %v, want %v", masked, c.MaskedParams)
			}
		})
	}
}

func TestCredentialRuleOnAnyJSONValue(t *testing.T) {
	v := map[string]any{"a": []any{map[string]any{"password": "hunter2hunter2", "url": "https://h.example/x?token=abc12345&q=1"}}, "note": "ok"}
	if got, want := literalCredentialsInValue(v), []string{"hunter2hunter2", "abc12345"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("literals = %v, want %v", got, want)
	}
	red := redactCredentialValue(v).(map[string]any)["a"].([]any)[0].(map[string]any)
	if red["password"] != CredentialRedactionMarker || red["url"] != "https://h.example/x?token="+CredentialRedactionMarker+"&q=1" {
		t.Fatalf("redacted = %v", red)
	}
	if v["a"].([]any)[0].(map[string]any)["password"] != "hunter2hunter2" {
		t.Fatal("the input value must not change")
	}
	if raw, _ := json.Marshal(CredentialFinding{NodeID: "n", Value: "hunter2hunter2"}); strings.Contains(string(raw), "hunter2") {
		t.Fatalf("a finding serialises its value: %s", raw)
	}
}

// credentialIssues runs the real validator on a flow with one http.request node.
func credentialIssues(t *testing.T, params map[string]any) []Issue {
	t.Helper()
	b := newFlow("Creds")
	b.node("call", TypeHTTPRequest, params)
	var out []Issue
	for _, is := range Validate(b.build(), webRegistry(t, nil), ValidateContext{Mode: ModePublish}) {
		if is.Code == IssueLiteralCredential {
			out = append(out, is)
		}
	}
	return out
}

func TestCredentialRuleValidationWarnsOncePerFinding(t *testing.T) {
	issues := credentialIssues(t, map[string]any{
		"url":     "https://u:p4ss@api.example.com/x?api_key=abc123&sig=def456",
		"headers": map[string]any{"Authorization": "Bearer tok-1234567890"},
	})
	want := []struct {
		param, location, path string
		movable               bool
	}{{"headers", "header", "Authorization", true}, {"url", "url_userinfo", "userinfo", true}, {"url", "url_query", "api_key", true}, {"url", "url_query", "sig", true}}
	if len(issues) != len(want) {
		t.Fatalf("issues = %+v, want %d warnings", issues, len(want))
	}
	for i, w := range want {
		is := issues[i]
		if is.Severity != SeverityWarning || is.Param != w.param || is.Location != w.location || is.Path != w.path || is.Movable != w.movable || is.Message == "" {
			t.Errorf("issue %d = %+v, want %+v", i, is, w)
		}
	}
}

func TestStoredStepParamsAndExportsRedactTheWholeRule(t *testing.T) {
	b := newFlow("Rule")
	tr := b.node("start", "test.trigger", nil)
	call := b.node("call", "test.echo", map[string]any{"value": "x", "url": "https://h.example/x?api_key=abc123&q=1",
		"payload": map[string]any{"password": "hunter2hunter2", "ok": "yes"}})
	b.edge(tr, PortOut, call)
	f := b.build()
	check := func(what string, p map[string]any) {
		t.Helper()
		payload, _ := p["payload"].(map[string]any)
		if p["url"] != "https://h.example/x?api_key="+CredentialRedactionMarker+"&q=1" || payload["password"] != CredentialRedactionMarker || payload["ok"] != "yes" {
			t.Errorf("%s keeps a credential: %v", what, p)
		}
	}
	out, err := RedactCredentials(f)
	if err != nil {
		t.Fatal(err)
	}
	check("export", out.NodeByID(call).Params)
	if f.NodeByID(call).Params["url"] != "https://h.example/x?api_key=abc123&q=1" {
		t.Fatal("the export must not change the draft")
	}
	res, _ := runWith(t.Context(), newTestEngine(newTestRegistry(t), nil, 4), f, RunRequest{TriggerNode: tr})
	check("stored step params", stepOf(res, call).Params)
}
