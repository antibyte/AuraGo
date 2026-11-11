package flows

import (
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
)

func sentHeaders(t *testing.T, tools *fakeTools) map[string]any {
	t.Helper()
	if tools.count() != 1 {
		t.Fatalf("calls = %d", tools.count())
	}
	h, _ := tools.calls[0].Args["headers"].(map[string]any)
	return h
}

// A moved header, query value and user info still reach the request: B.2's auth_in carries them.
func TestMovedCredentialsStillReachTheRequest(t *testing.T) {
	doc := &Flow{Nodes: []Node{
		{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v1?api_key=qry-token-1234&q=1", "headers": map[string]any{"X-Api-Key": "hdr-token-1234", "Accept": "text/plain"}}},
		{ID: "n2", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v2", "headers": map[string]any{"X-Api-Key": "hdr-token-1234"}}},
	}}
	moved, notMovable, value, err := MoveCredential(doc, "n1", "header", "X-Api-Key", "svc_key")
	if err != nil || value != "hdr-token-1234" || !reflect.DeepEqual(moved, []string{"n1", "n2"}) || len(notMovable) != 0 {
		t.Fatalf("MoveCredential = %v, %v, %q, %v", moved, notMovable, value, err)
	}
	// n1 has its secret slot now, so its query token is no longer movable.
	if _, _, _, err := MoveCredential(doc, "n1", "url_query", "api_key", "svc_q"); !errors.Is(err, ErrCredentialNotMovable) {
		t.Fatalf("second move = %v, want ErrCredentialNotMovable", err)
	}
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), doc.NodeByID("n1").Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_key": "hdr-token-1234"}}); err != nil {
		t.Fatal(err)
	}
	if h := sentHeaders(t, tools); h["X-Api-Key"] != "hdr-token-1234" || h["Accept"] != "text/plain" {
		t.Fatalf("sent headers = %v: no scheme prefix may be added", h)
	}
}

func TestMovedQueryAndUserinfoStillReachTheRequest(t *testing.T) {
	q := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://api.example.com/v1?api_key=qry-token-1234&q=1"}}}}
	if _, _, value, err := MoveCredential(q, "n1", "url_query", "api_key", "svc_q"); err != nil || value != "qry-token-1234" {
		t.Fatalf("query move = %q, %v", value, err)
	}
	if p := q.Nodes[0].Params; p["url"] != "https://api.example.com/v1?q=1" || p["auth_in"] != "query" || p["auth_query"] != "api_key" || p["auth_secret"] != "svc_q" {
		t.Fatalf("query params = %v", p)
	}
	tools := &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), q.Nodes[0].Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_q": "qry-token-1234"}}); err != nil {
		t.Fatal(err)
	}
	if got := tools.calls[0].Args["url"]; got != "https://api.example.com/v1?q=1&api_key=qry-token-1234" {
		t.Fatalf("url = %v", got)
	}

	u := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://user:pw1234@files.example.com/list"}}}}
	want := base64.StdEncoding.EncodeToString([]byte("user:pw1234"))
	if _, _, value, err := MoveCredential(u, "n1", "url_userinfo", "userinfo", "svc_basic"); err != nil || value != want {
		t.Fatalf("userinfo move = %q, %v", value, err)
	}
	tools = &fakeTools{respond: answerWith(apiAnswer(200, "ok"))}
	if _, err := execDef(httpNodeDef(t), u.Nodes[0].Params, &Services{Tools: tools, Secrets: fakeSecrets{"svc_basic": want}}); err != nil {
		t.Fatal(err)
	}
	if h := sentHeaders(t, tools); h["Authorization"] != "Basic "+want || tools.calls[0].Args["url"] != "https://files.example.com/list" {
		t.Fatalf("sent = %v %v", h, tools.calls[0].Args["url"])
	}
}

func TestMoveCredentialRefusesWhatCannotMove(t *testing.T) {
	doc := &Flow{Nodes: []Node{{ID: "n1", Type: TypeHTTPRequest, Params: map[string]any{"url": "https://a.example/x", "method": "POST", "body": map[string]any{"password": "hunter2hunter2"}}}}}
	if _, _, _, err := MoveCredential(doc, "n1", "body_field", "/password", "x"); !errors.Is(err, ErrCredentialNotMovable) {
		t.Fatalf("body field = %v, want ErrCredentialNotMovable", err)
	}
	if _, _, _, err := MoveCredential(doc, "n1", "header", "Authorization", "x"); !errors.Is(err, ErrCredentialGone) {
		t.Fatalf("missing header = %v, want ErrCredentialGone", err)
	}
}
