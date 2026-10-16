package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/decisions"
)

func TestDecisionsInstallationCheckAccessAndResult(t *testing.T) {
	s, _ := decisionsServerFixture(t)
	s.Cfg.Decisions.Enabled = false
	manager := decisions.NewManager(s.Cfg.Decisions, t.TempDir(), decisions.WithRuntimeDirectory(t.TempDir()))
	s.Decisions = &routedDecisions{local: manager}
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	for _, tc := range []struct {
		method, origin string
		want           int
	}{
		{"GET", "http://localhost", 405},
		{"POST", "", 403},
		{"POST", "http://attacker.invalid", 403},
		{"POST", "http://localhost", 200},
	} {
		r := httptest.NewRequest(tc.method, "http://localhost/api/decisions/check", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.origin, w.Code, w.Body.String())
		}
		if w.Code == 200 {
			var result decisions.InstallationCheck
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code != decisions.CodeRuntimeArtifactMissing || result.RuntimeAvailable || result.Target == "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unsafe or incomplete result: %+v", result)
			}
		}
	}
	if manager.Status().Enabled {
		t.Fatal("check enabled the service")
	}
	s.Cfg.Decisions.Backend = "provider"
	r := httptest.NewRequest("POST", "http://localhost/api/decisions/check", nil)
	r.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("provider check = %d", w.Code)
	}
	s.Cfg.Auth.Enabled = true
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous check = %d", w.Code)
	}
}
