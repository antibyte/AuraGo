package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ff1Token mints a desktop token with the given scopes on the server's token manager.
func ff1Token(t *testing.T, s *Server, name string, scopes ...string) string {
	t.Helper()
	token, _, err := s.TokenManager.Create(name, scopes, nil)
	if err != nil {
		t.Fatalf("create token %s: %v", name, err)
	}
	return token
}

// FF1: a bearer token needs desktop:admin for every flows request that changes something,
// except POST validate (desktop:write). A test run executes the flow's tools on the host
// (shell, sudo, docker), publishing and enabling create missions, cron jobs and webhooks,
// and secrets go into the vault. Reads keep desktop:read; session users are unaffected.
func TestFF1FlowsWritesNeedTheAdminScope(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	writeToken := ff1Token(t, s, "ff1 write", desktopScopeRead, desktopScopeWrite)
	readToken := ff1Token(t, s, "ff1 read", desktopScopeRead)
	adminToken := ff1Token(t, s, "ff1 admin", desktopScopeAdmin)
	rec := createTestFlow(t, s, greetFlowJSON)
	id := rec.ID

	type call struct{ method, path, body string }
	writes := []call{
		{http.MethodPost, "/api/desktop/flows", `{"name":"FF1"}`},
		{http.MethodPut, "/api/desktop/flows/" + id, `{"doc":` + greetFlowJSON + `,"base_revision":1}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/test", `{}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/publish", `{"base_revision":2}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/enabled", `{"enabled":true}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/run", ``},
		{http.MethodPut, "/api/desktop/flows/" + id + "/test-data/n_aaaaaaaa", `{"data":{"name":"Welt"}}`},
		{http.MethodPut, "/api/desktop/flows/secrets/ff1_key", `{"value":"ff1-secret-value"}`},
		{http.MethodDelete, "/api/desktop/flows/secrets/ff1_key", ``},
		{http.MethodPost, "/api/desktop/flows/runs/run_ff1/cancel", ``},
		{http.MethodDelete, "/api/desktop/flows/" + id, ``},
	}
	for _, c := range writes {
		if validRouteBearer(s, writeToken, c.path, c.method) {
			t.Errorf("auth middleware: desktop:write passes %s %s", c.method, c.path)
		}
		if !validRouteBearer(s, adminToken, c.path, c.method) {
			t.Errorf("auth middleware: desktop:admin refused for %s %s", c.method, c.path)
		}
		w := flowsCall(t, s, c.method, c.path, writeToken, c.body)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
			t.Errorf("%s %s with desktop:write = %d %.200s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	if t.Failed() {
		return
	}

	// The admin token passes every gate; the requests run in an order that succeeds.
	want := map[string]int{
		"/api/desktop/flows":                     http.StatusCreated,
		"/api/desktop/flows/runs/run_ff1/cancel": http.StatusNotFound,
		"/api/desktop/flows/" + id + "/test":     http.StatusAccepted,
		"/api/desktop/flows/" + id + "/run":      http.StatusAccepted,
	}
	for _, c := range writes {
		w := flowsCall(t, s, c.method, c.path, adminToken, c.body)
		code, ok := want[c.path]
		if !ok {
			code = http.StatusOK
		}
		if w.Code != code {
			t.Errorf("%s %s with desktop:admin = %d %.300s", c.method, c.path, w.Code, w.Body.String())
		}
	}

	// Reads keep desktop:read and validate keeps desktop:write, in the middleware and the
	// handler.
	other := createTestFlow(t, s, greetFlowJSON)
	reads := []call{
		{http.MethodGet, "/api/desktop/flows", ``},
		{http.MethodGet, "/api/desktop/flows/" + other.ID, ``},
		{http.MethodGet, "/api/desktop/flows/" + other.ID + "/runs", ``},
		{http.MethodGet, "/api/desktop/flows/" + other.ID + "/export", ``},
		{http.MethodGet, "/api/desktop/flows/" + other.ID + "/publish-preview", ``},
		{http.MethodGet, "/api/desktop/flows/" + other.ID + "/test-data/n_aaaaaaaa", ``},
		{http.MethodGet, "/api/desktop/flows/secrets", ``},
		{http.MethodGet, "/api/desktop/flows/templates", ``},
		{http.MethodGet, "/api/desktop/flows/node-types", ``},
	}
	for _, c := range reads {
		if !validRouteBearer(s, readToken, c.path, c.method) {
			t.Errorf("auth middleware: desktop:read refused for %s %s", c.method, c.path)
		}
		if w := flowsCall(t, s, c.method, c.path, readToken, c.body); w.Code != http.StatusOK {
			t.Errorf("%s %s with desktop:read = %d %.200s", c.method, c.path, w.Code, w.Body.String())
		}
	}
	const validate = "/api/desktop/flows/validate"
	if !validRouteBearer(s, writeToken, validate, http.MethodPost) {
		t.Error("auth middleware: desktop:write refused for POST validate")
	}
	if validRouteBearer(s, readToken, validate, http.MethodPost) {
		t.Error("auth middleware: desktop:read passes POST validate")
	}
	if w := flowsCall(t, s, http.MethodPost, validate, writeToken, `{"doc":`+greetFlowJSON+`}`); w.Code != http.StatusOK {
		t.Errorf("POST validate with desktop:write = %d %.200s", w.Code, w.Body.String())
	}
	// Only the exact collection path and its sub-paths are flows routes.
	if !validRouteBearer(s, writeToken, "/api/desktop/flowsx", http.MethodPost) {
		t.Error("a path that only starts with /api/desktop/flows is not a flows route")
	}
}

// FF1: session (cookie) users keep their access to the flows writes; the admin scope is a
// bearer rule only.
func TestFF1FlowsSessionWritesAreUnaffected(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	session := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", "http://example.com")
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		return w
	}
	w := session(http.MethodPost, "/api/desktop/flows", `{"name":"FF1 session"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("session create = %d %s", w.Code, w.Body.String())
	}
	rec := createTestFlow(t, s, greetFlowJSON)
	if w := session(http.MethodPost, "/api/desktop/flows/"+rec.ID+"/test", `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("session test run = %d %s", w.Code, w.Body.String())
	}
}
