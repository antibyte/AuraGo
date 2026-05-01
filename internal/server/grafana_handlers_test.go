package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func newGrafanaHandlerTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer gf_server_test" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		switch r.URL.Path {
		case "/api/health":
			fmt.Fprint(w, `{"database":"ok","version":"10.4.0"}`)
		case "/api/search":
			fmt.Fprint(w, `[{"uid":"sys","title":"System"}]`)
		case "/api/datasources":
			fmt.Fprint(w, `[{"id":1,"name":"Prometheus"}]`)
		case "/api/alerts":
			fmt.Fprint(w, `[{"id":1,"name":"CPU"}]`)
		case "/api/org":
			fmt.Fprint(w, `{"id":1,"name":"Main Org."}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestHandleGrafanaStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/grafana/status", nil)
	rec := httptest.NewRecorder()

	handleGrafanaStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestHandleGrafanaStatusReturnsSummary(t *testing.T) {
	srv := newGrafanaHandlerTestServer(t)
	defer srv.Close()
	s := &Server{Cfg: &config.Config{Grafana: config.GrafanaConfig{Enabled: true, BaseURL: srv.URL, APIKey: "gf_server_test", RequestTimeout: 5}}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodGet, "/api/grafana/status", nil)
	rec := httptest.NewRecorder()

	handleGrafanaStatus(s).ServeHTTP(rec, req)

	var body struct {
		Status string `json:"status"`
		Data   struct {
			Summary struct {
				Dashboards  int    `json:"dashboards"`
				Datasources int    `json:"datasources"`
				Alerts      int    `json:"alerts"`
				Org         string `json:"org"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Status != "ok" || body.Data.Summary.Dashboards != 1 || body.Data.Summary.Datasources != 1 || body.Data.Summary.Alerts != 1 || body.Data.Summary.Org != "Main Org." {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestHandleGrafanaTestRequiresAPIKey(t *testing.T) {
	s := &Server{Cfg: &config.Config{Grafana: config.GrafanaConfig{BaseURL: "https://grafana.local"}}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/grafana/test", nil)
	rec := httptest.NewRecorder()

	handleGrafanaTest(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "error" {
		t.Fatalf("status = %#v, want error", body["status"])
	}
}
