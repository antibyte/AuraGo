package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"aurago/internal/config"
)

func TestExternalAssetsRecoveryAuthAndCSRF(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.UILanguage = "de"
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "asset-test-session"
	cfg.Auth.PasswordHash = "configured"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	s.registerAssetRecoveryRoutes(mux)
	s.registerRecoveryUI(mux)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/auth/login", 200}, {"GET", "/api/assets/status", 401}, {"POST", "/api/assets/install", 401},
	} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
		if r.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, r.Code, r.Body.String())
		}
		if tc.path == "/auth/login" && (!strings.Contains(r.Body.String(), `autocomplete="one-time-code"`) || strings.Contains(r.Body.String(), `<script src=`)) {
			t.Fatal("recovery login is not self-contained/TOTP-capable")
		}
	}
	cfg.Auth.Enabled = false
	for _, origin := range []string{"", "https://evil.example"} {
		req := httptest.NewRequest("POST", "http://localhost/api/assets/install", nil)
		req.Header.Set("Origin", origin)
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, req)
		if r.Code != 403 {
			t.Fatalf("CSRF accepted: %d", r.Code)
		}
	}
}

func TestExternalAssetsVersionAndRange(t *testing.T) {
	f := fstest.MapFS{"js/app.js": {Data: []byte("console.log('local')")}, "model.wasm": {Data: []byte("1234567890")}}
	h := versionedUIHandler(f, "digest")
	for _, tc := range []struct {
		url, rangeHeader string
		status           int
		body             string
	}{
		{"/js/app.js?v=digest", "", 200, "console.log('local')"},
		{"/js/app.js?v=older", "", 409, "Reload"},
		{"/model.wasm?v=digest", "bytes=2-4", 206, "345"},
		{"/js/", "", 404, ""},
	} {
		req := httptest.NewRequest("GET", tc.url, nil)
		req.Header.Set("Range", tc.rangeHeader)
		r := httptest.NewRecorder()
		h.ServeHTTP(r, req)
		if r.Code != tc.status || !strings.Contains(r.Body.String(), tc.body) {
			t.Fatalf("%s: %d %s", tc.url, r.Code, r.Body.String())
		}
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/js/app.js", nil))
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unversioned resource can become stale")
	}
}
