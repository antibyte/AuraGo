package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContainerToolErrorsUseBadGatewayAndKeepTheBody(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	defer replaceContainerProtection(containerProtection{})()

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		req     *http.Request
	}{
		{"list", handleContainersList(s), httptest.NewRequest(http.MethodGet, "/api/containers", nil)},
		{"start", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/start", nil)},
		{"stop", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/stop", nil)},
		{"restart", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil)},
		{"update", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/update", nil)},
		{"logs", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/logs", nil)},
		{"inspect", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/inspect", nil)},
		{"stats", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/stats", nil)},
		{"remove", handleContainerAction(s), httptest.NewRequest(http.MethodDelete, "/api/containers/demo", nil)},
	} {
		rec := httptest.NewRecorder()
		tc.handler(rec, tc.req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("%s status = %d, want 502; body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("%s Content-Type = %q", tc.name, got)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "error" || !strings.Contains(rec.Body.String(), "engine refused") {
			t.Fatalf("%s body = %s (%v); want the tool error JSON", tc.name, rec.Body.String(), err)
		}
	}
}

func TestContainerToolSuccessAndGuardsKeepTheirStatus(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/demo/restart"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	})

	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("restart = %d %s, want 200 ok", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("list = %d %s, want 200 ok", rec.Code, rec.Body.String())
	}

	s.Cfg.Docker.ReadOnly = true
	rec = httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only restart = %d, want 403", rec.Code)
	}
	s.Cfg.Docker.Enabled = false
	rec = httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled list = %d, want 503", rec.Code)
	}
}
