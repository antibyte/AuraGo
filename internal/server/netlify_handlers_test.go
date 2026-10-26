package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func netlifyTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	return &Server{Cfg: cfg}
}

func TestNetlifyStatusReportsDisabledIntegration(t *testing.T) {
	s := netlifyTestServer(t)
	recorder := httptest.NewRecorder()
	handleNetlifyStatus(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/netlify/status", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, recorder.Body.String())
	}
	if parsed["status"] != "disabled" {
		t.Fatalf("status = %v, want disabled", parsed["status"])
	}
}

func TestNetlifyHandlersRejectWrongMethods(t *testing.T) {
	s := netlifyTestServer(t)
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{"status", handleNetlifyStatus(s), http.MethodPost, "/api/netlify/status"},
		{"test connection", handleNetlifyTestConnection(s), http.MethodGet, "/api/netlify/test-connection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.handler.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", recorder.Code)
			}
		})
	}
}
