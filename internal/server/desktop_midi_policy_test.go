package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSynthStudioMIDIPolicyOnlyAllowsDesktop(t *testing.T) {
	for _, tls := range []bool{false, true} {
		for _, path := range []string{"/desktop", "/desktop/", "/desktop.html", "/", "/login", "/files/desktop/Apps/test/index.html", "/api/game-maker/preview/test/", "/desktop/untrusted", "/desktop.html/"} {
			t.Run(path, func(t *testing.T) {
				w := httptest.NewRecorder()
				securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }), tls, false).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				want := "midi=()"
				if path == "/desktop" || path == "/desktop/" || path == "/desktop.html" {
					want = "midi=(self)"
				}
				fields := strings.Join(w.Header().Values("Permissions-Policy"), ", ")
				if !strings.Contains(fields, want) {
					t.Fatalf("MIDI policy = %q, want %q", fields, want)
				}
				if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
					t.Fatalf("lost existing security header: %q", got)
				}
			})
		}
	}
}
