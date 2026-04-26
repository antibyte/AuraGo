package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowYouTubeEmbeds(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	for _, marker := range []string{
		"frame-src 'self' https://www.youtube-nocookie.com https://www.youtube.com",
		"frame-ancestors 'none'",
	} {
		if !strings.Contains(csp, marker) {
			t.Fatalf("Content-Security-Policy missing %q: %s", marker, csp)
		}
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
}
