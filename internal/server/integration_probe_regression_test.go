package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

func TestDograhProbeNeverRebindsSavedCredentials(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(`[]`)) }))
	defer target.Close()
	cfg := &config.Config{}
	cfg.Dograh.Enabled, cfg.Dograh.APIURL, cfg.Dograh.APIKey = true, "https://saved.invalid", "saved-fixture"
	s := &Server{Cfg: cfg}
	for _, explicit := range []bool{false, true} {
		patch := map[string]any{"api_url": target.URL}
		if explicit {
			patch["api_key"] = "explicit-fixture"
		}
		body, _ := json.Marshal(map[string]any{"dograh": patch})
		rec := httptest.NewRecorder()
		handleDograhTest(s)(rec, httptest.NewRequest(http.MethodPost, "/api/dograh/test", strings.NewReader(string(body))))
		if !explicit && (rec.Code != 400 || requests.Load() != 0) {
			t.Fatalf("saved key sent to changed target: %d", rec.Code)
		}
		if explicit && (rec.Code != 200 || requests.Load() != 1) {
			t.Fatalf("explicit target credential rejected: %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestYepAPIProbeUsesSavedTargetAndOnlyReads(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.URL.Path != "/custom/v1/ai/models" || r.Header.Get("x-api-key") != "fixture-key" {
			t.Errorf("unexpected probe: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer target.Close()
	cfg := &config.Config{}
	cfg.YepAPI.Enabled, cfg.YepAPI.BaseURL = true, target.URL+"/custom"
	cfg.Providers = []config.ProviderEntry{{ID: "yep", Type: "yepapi", APIKey: "fixture-key"}}
	s := &Server{Cfg: cfg}
	rec := httptest.NewRecorder()
	handleYepAPITest(s)(rec, httptest.NewRequest(http.MethodGet, "/api/yepapi/test", nil))
	if rec.Code != 405 || requests.Load() != 0 {
		t.Fatal("GET triggered provider probe")
	}
	rec = httptest.NewRecorder()
	handleYepAPITest(s)(rec, httptest.NewRequest(http.MethodPost, "/api/yepapi/test", strings.NewReader(`{"base_url":"https://untrusted.invalid"}`)))
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || result["status"] != "ok" || result["billable"] != false || result["authentication_verified"] != false {
		t.Fatalf("probe result = %v", result)
	}
}
