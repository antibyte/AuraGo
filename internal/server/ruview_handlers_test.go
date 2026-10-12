package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewRoutesRequireAdminAndWriteOrigin(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.Cfg.RuView.Enabled = true
	s.Cfg.RuView.Mode = "external"
	s.Cfg.RuView.URL = "https://example.invalid"
	s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer s.RuView.Close()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	handler := desktopIntegrationHandler(s, mux)
	admin, _, err := s.TokenManager.Create("ruview fixture", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, route, token, origin string
		want                         int
	}{
		{"GET", "status", readToken, "", 403},
		{"GET", "status", admin, "", 200},
		{"GET", "ui/observatory.html", admin, "", 200},
		{"GET", "ui/pose-fusion/pkg/ruvector-attention/ruvector_attention_wasm_bg.wasm", admin, "", 200},
		{"GET", "test", admin, "", 405},
		{"POST", "test", admin, "https://evil.invalid", 403},
		{"POST", "runtime/start", admin, "http://aurago.test", 403},
		{"POST", "proxy/api/v1/recording/start", admin, "http://aurago.test", 403},
		{"GET", "proxy/ws/sensing", admin, "http://evil.invalid", 403},
		{"GET", "proxy/api/v1/pose/stats", admin, "", 404},
	} {
		r := httptest.NewRequest(tc.method, "http://aurago.test/api/desktop/integrations/ruview/"+tc.route, nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s: %d, want %d: %s", tc.method, tc.route, w.Code, tc.want, w.Body.String())
		}
		if strings.HasSuffix(tc.route, ".wasm") {
			if w.Header().Get("Content-Type") != "application/wasm" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "'wasm-unsafe-eval'") || !strings.HasPrefix(w.Body.String(), "\x00asm") {
				t.Fatal("pinned WebAssembly is missing or blocked by its response policy")
			}
		}
		if strings.HasSuffix(tc.route, ".html") {
			policy := w.Header().Get("Content-Security-Policy")
			_, after, ok := strings.Cut(policy, "'nonce-")
			nonce, _, found := strings.Cut(after, "'")
			if !ok || !found || nonce == "" || strings.Contains(strings.Split(policy, ";")[1], "'unsafe-inline'") {
				t.Fatal("RuView scripts lack a restricted nonce policy")
			}
			if strings.Count(w.Body.String(), "<script") != strings.Count(w.Body.String(), "<script nonce=\""+nonce+"\"") {
				t.Fatal("trusted inline scripts/import maps lack their nonce")
			}
		}
	}
}
