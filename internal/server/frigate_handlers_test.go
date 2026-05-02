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

func TestHandleFrigateTestReturnsStats(t *testing.T) {
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer frigate-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if r.URL.Path != "/api/stats" {
			t.Fatalf("path = %q, want /api/stats", r.URL.Path)
		}
		fmt.Fprint(w, `{"service":{"uptime":90}}`)
	}))
	defer srv.Close()
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Frigate.Enabled = true
	s.Cfg.Frigate.URL = srv.URL
	s.Cfg.Frigate.APIToken = "frigate-token"

	req := httptest.NewRequest(http.MethodPost, "/api/frigate/test", nil)
	rec := httptest.NewRecorder()
	handleFrigateTest(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %#v, want ok; body=%s", body["status"], rec.Body.String())
	}
}

func TestHandleFrigateTestRequiresURL(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Frigate.Enabled = true
	req := httptest.NewRequest(http.MethodPost, "/api/frigate/test", nil)
	rec := httptest.NewRecorder()
	handleFrigateTest(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if body["status"] != "error" {
		t.Fatalf("status = %#v, want error", body["status"])
	}
}
