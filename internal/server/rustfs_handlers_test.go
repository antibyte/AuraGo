package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/rustfs"
)

func TestRustFSRoutesRequireAdminAndValidateRequests(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	s.RustFS = rustfs.NewManager(context.Background(), s.Cfg, s.Vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = s.RustFS.Shutdown(context.Background()) })
	admin, _, err := s.TokenManager.Create("RustFS test admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerRustFSRoutes(mux, s)
	for _, tc := range []struct {
		name, method, route, token, body string
		want                             int
	}{
		{"anonymous", "GET", "status", "", "", 401},
		{"read token", "GET", "status", readToken, "", 403},
		{"passive status", "GET", "status", admin, "", 200},
		{"wrong read method", "POST", "status", admin, "{}", 405},
		{"wrong mutation method", "GET", "install", admin, "", 405},
		{"Docker disabled install", "POST", "install", admin, "{}", 403},
		{"unknown endpoint", "POST", "install", admin, `{"endpoint":"http://untrusted.invalid"}`, 400},
		{"unknown credential", "POST", "preflight", admin, `{"secret":"fixture-private-value"}`, 400},
		{"trailing JSON", "POST", "start", admin, "{} {}", 400},
		{"null body", "POST", "stop", admin, "null", 400},
		{"array body", "POST", "remove", admin, "[]", 400},
		{"empty body", "POST", "recreate", admin, "", 400},
		{"oversize", "POST", "preflight", admin, strings.Repeat(" ", 1100) + "{}", 413},
		{"missing receipt", "GET", "operations/missing", admin, "", 404},
		{"nested receipt", "GET", "operations/missing/other", admin, "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://aurago.test/api/rustfs/"+tc.route, strings.NewReader(tc.body))
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "fixture-private-value") || strings.Contains(w.Body.String(), "untrusted.invalid") {
				t.Fatal("request values leaked into the API error")
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("RustFS response can be cached")
			}
			if tc.want == 405 && w.Header().Get("Allow") == "" {
				t.Fatal("wrong-method response is missing Allow")
			}
			if tc.name == "passive status" {
				var status rustfs.Status
				if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
					t.Fatal(err)
				}
				if status.Enabled || status.Installed || status.Running || status.Ready {
					t.Fatal("reading status activated disabled RustFS")
				}
			}
		})
	}
}

func TestRustFSBrowserMutationRequiresSameOrigin(t *testing.T) {
	for _, origin := range []string{"", "https://foreign.invalid"} {
		r := httptest.NewRequest(http.MethodPost, "http://aurago.test/api/rustfs/install", strings.NewReader("{}"))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handleRustFS(&Server{}, "install")(w, r)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "origin_denied") {
			t.Fatalf("origin %q status = %d: %s", origin, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodPost, "http://aurago.test/api/rustfs/install", strings.NewReader("{}"))
	r.Header.Set("Origin", "http://aurago.test")
	w := httptest.NewRecorder()
	handleRustFS(&Server{}, "install")(w, r)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "service_not_ready") {
		t.Fatalf("same-origin request did not reach the manager boundary: %d %s", w.Code, w.Body.String())
	}
}
