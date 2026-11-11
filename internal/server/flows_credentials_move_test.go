package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	credHeaderToken = "hdr-token-1234567890"
	credQueryToken  = "qry-token-1234567890"
	credBodyToken   = "body-pass-1234567890"
)

// call, other, files and post carry a typed credential each; call and other share the header value.
var credentialMoveFlowJSON = `{"schema":1,"name":"Wetter","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"call","type":"http.request","type_version":1,"label":"Call","position":{"x":300,"y":0},
  "params":{"url":"https://api.example.com/v1/weather?api_key=` + credQueryToken + `&q=berlin","headers":{"Authorization":"Bearer ` + credHeaderToken + `"}}},
 {"id":"n_cccccccc","key":"other","type":"http.request","type_version":1,"label":"Other","position":{"x":600,"y":0},
  "params":{"url":"https://api.example.com/v1/other","headers":{"Authorization":"Bearer ` + credHeaderToken + `"}}},
 {"id":"n_dddddddd","key":"files","type":"http.request","type_version":1,"label":"Files","position":{"x":900,"y":0},
  "params":{"url":"https://user:pw123456@files.example.com/list"}},
 {"id":"n_eeeeeeee","key":"post","type":"http.request","type_version":1,"label":"Post","position":{"x":1200,"y":0},
  "params":{"url":"https://api.example.com/v1/post","method":"POST","body":{"password":"` + credBodyToken + `"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}},
  {"id":"e_bbbbbbbb","source":{"node":"n_bbbbbbbb","port":"out"},"target":{"node":"n_cccccccc","port":"in"}},
  {"id":"e_cccccccc","source":{"node":"n_cccccccc","port":"out"},"target":{"node":"n_dddddddd","port":"in"}},
  {"id":"e_dddddddd","source":{"node":"n_dddddddd","port":"out"},"target":{"node":"n_eeeeeeee","port":"in"}}]}`

func moveBody(rev int, node, location, path, name string, replace bool) string {
	return fmt.Sprintf(`{"revision":%d,"node_id":%q,"location":%q,"path":%q,"secret_name":%q,"replace":%t}`, rev, node, location, path, name, replace)
}

func moveCall(t *testing.T, s *Server, token, flowID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+flowID+"/credentials/move", token, body)
}

func draftParams(t *testing.T, s *Server, flowID, node string) map[string]any {
	t.Helper()
	rec, err := s.Flows.GetFlow(t.Context(), flowID)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Draft.NodeByID(node).Params
}

func TestFlowCredentialMoveHeaderQueryAndUserinfo(t *testing.T) {
	s, token := newFlowsTestServer(t)
	secret := func(name string) string {
		t.Helper()
		v, err := (flowSecrets{s: s}).ReadSecret(name)
		if err != nil {
			t.Fatalf("ReadSecret(%s): %v", name, err)
		}
		return v
	}
	t.Run("header, and the identical value in another node", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), credHeaderToken) {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		moved, _ := body["moved"].([]any)
		if body["draft_revision"] != float64(rec.DraftRevision+1) || len(moved) != 2 || moved[0] != "n_bbbbbbbb" || moved[1] != "n_cccccccc" {
			t.Fatalf("answer = %v", body)
		}
		if secret("weather_key") != credHeaderToken {
			t.Fatal("the vault entry does not hold the token")
		}
		for _, id := range []string{"n_bbbbbbbb", "n_cccccccc"} {
			p := draftParams(t, s, rec.ID, id)
			if _, has := p["headers"].(map[string]any)["Authorization"]; has || p["auth_secret"] != "weather_key" || p["auth_in"] != "header" || p["auth_header"] != "Authorization" || p["auth_prefix"] != "Bearer " {
				t.Errorf("%s params = %v", id, p)
			}
		}
	})
	t.Run("url query", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "url_query", "api_key", "weather_q", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), credQueryToken) {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		p := draftParams(t, s, rec.ID, "n_bbbbbbbb")
		if secret("weather_q") != credQueryToken || p["url"] != "https://api.example.com/v1/weather?q=berlin" || p["auth_in"] != "query" || p["auth_query"] != "api_key" || p["auth_secret"] != "weather_q" {
			t.Fatalf("params = %v", p)
		}
	})
	t.Run("url user info", func(t *testing.T) {
		rec := createTestFlow(t, s, credentialMoveFlowJSON)
		w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_dddddddd", "url_userinfo", "userinfo", "files_login", false))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "pw123456") {
			t.Fatalf("move = %d, the answer must not carry the value: %s", w.Code, w.Body.String())
		}
		p := draftParams(t, s, rec.ID, "n_dddddddd")
		if secret("files_login") != base64.StdEncoding.EncodeToString([]byte("user:pw123456")) || p["url"] != "https://files.example.com/list" || p["auth_header"] != "Authorization" || p["auth_prefix"] != "Basic " {
			t.Fatalf("params = %v", p)
		}
	})
}

func TestFlowCredentialMoveRefusals(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, credentialMoveFlowJSON)
	good := moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", false)
	code := func(w *httptest.ResponseRecorder) string { return fmt.Sprint(flowsBody(t, w)["code"]) }
	revision := func() int {
		r, err := s.Flows.GetFlow(t.Context(), rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		return r.DraftRevision
	}

	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision+5, "n_bbbbbbbb", "header", "Authorization", "weather_key", false)); w.Code != http.StatusConflict || code(w) != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("stale revision = %d %s", w.Code, w.Body.String())
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "X-Missing", "weather_key", false)); w.Code != http.StatusConflict || code(w) != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("value no longer there = %d %s", w.Code, w.Body.String())
	}
	w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_eeeeeeee", "body_field", "/password", "post_pass", false))
	if w.Code != http.StatusUnprocessableEntity || code(w) != "FLOW_CREDENTIAL_NOT_MOVABLE" || strings.Contains(w.Body.String(), credBodyToken) {
		t.Fatalf("body field = %d %s", w.Code, w.Body.String())
	}
	if _, err := (flowSecrets{s: s}).ReadSecret("post_pass"); err == nil {
		t.Fatal("a refused move wrote a vault entry")
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "Bad-Name", false)); w.Code != http.StatusBadRequest {
		t.Fatalf("bad name = %d %s", w.Code, w.Body.String())
	}

	readOnly, _, err := s.TokenManager.Create("read only", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := moveCall(t, s, readOnly, rec.ID, good); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
		t.Fatalf("non-admin = %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/credentials/move", strings.NewReader(good))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	r.Header.Set("Origin", "https://evil.example")
	cross := httptest.NewRecorder()
	s.handleFlows(cross, r)
	if cross.Code != http.StatusForbidden {
		t.Fatalf("cross origin = %d %s", cross.Code, cross.Body.String())
	}

	// An existing secret is only replaced when asked; a refusal changes nothing.
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/weather_key", token, `{"value":"old-value-123456"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d", w.Code)
	}
	before := revision()
	if w := moveCall(t, s, token, rec.ID, good); w.Code != http.StatusConflict || code(w) != "FLOW_SECRET_EXISTS" {
		t.Fatalf("existing secret = %d %s", w.Code, w.Body.String())
	}
	if v, _ := (flowSecrets{s: s}).ReadSecret("weather_key"); v != "old-value-123456" || revision() != before {
		t.Fatalf("a refused move changed the vault (%q) or the draft (%d)", v, revision())
	}
	if w := moveCall(t, s, token, rec.ID, moveBody(rec.DraftRevision, "n_bbbbbbbb", "header", "Authorization", "weather_key", true)); w.Code != http.StatusOK {
		t.Fatalf("replace = %d %s", w.Code, w.Body.String())
	}
	if v, _ := (flowSecrets{s: s}).ReadSecret("weather_key"); v != credHeaderToken {
		t.Fatalf("replaced value = %q", v)
	}
}
