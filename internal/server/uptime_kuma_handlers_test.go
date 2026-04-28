package server

import (
	"aurago/internal/testutil"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

const testUptimeKumaMetrics = `monitor_status{monitor_name="Main Website",monitor_type="http",monitor_url="https://example.com"} 0
monitor_response_time{monitor_name="Main Website",monitor_type="http",monitor_url="https://example.com"} 321
`

func TestHandleUptimeKumaStatusDisabled(t *testing.T) {
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/uptime-kuma/status", nil)
	rec := httptest.NewRecorder()

	handleUptimeKumaStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestHandleUptimeKumaStatusReturnsSummary(t *testing.T) {
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || pass != "uk2_server_test" {
			t.Fatalf("unexpected basic auth: ok=%v pass=%q", ok, pass)
		}
		fmt.Fprint(w, testUptimeKumaMetrics)
	}))
	defer srv.Close()

	s := &Server{
		Cfg: &config.Config{
			UptimeKuma: config.UptimeKumaConfig{
				Enabled:        true,
				BaseURL:        srv.URL,
				APIKey:         "uk2_server_test",
				RequestTimeout: 5,
			},
		},
		Logger: slog.Default(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/uptime-kuma/status", nil)
	rec := httptest.NewRecorder()

	handleUptimeKumaStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Summary struct {
				Down float64 `json:"down"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Data.Summary.Down != 1 {
		t.Fatalf("down = %v, want 1", body.Data.Summary.Down)
	}
}

func TestHandleUptimeKumaTestRequiresAPIKey(t *testing.T) {
	s := &Server{
		Cfg: &config.Config{
			UptimeKuma: config.UptimeKumaConfig{
				BaseURL: "https://uptime.local",
			},
		},
		Logger: slog.Default(),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/uptime-kuma/test", nil)
	rec := httptest.NewRecorder()

	handleUptimeKumaTest(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "error" {
		t.Fatalf("status = %#v, want error", body["status"])
	}
}
