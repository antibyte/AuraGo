package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestLogoutRequiresPostAndSameOrigin(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	for _, handler := range []http.HandlerFunc{handleAuthLogout(s), handleAuthLogoutAPI(s)} {
		for _, test := range []struct {
			method, origin string
			want           int
		}{{"GET", "http://example.com", 405}, {"POST", "https://foreign.example", 403}, {"POST", "", 403}, {"POST", "http://example.com", 200}} {
			req := httptest.NewRequest(test.method, "http://example.com/auth/logout", nil)
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Accept", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != test.want {
				t.Errorf("%s origin %s: status %d, want %d", test.method, test.origin, rec.Code, test.want)
			}
			if test.want != 200 && rec.Header().Get("Set-Cookie") != "" {
				t.Error("rejected request mutated the session")
			}
		}
	}
}

func TestMediaProxyRejectsActiveTypes(t *testing.T) {
	for _, kind := range []string{"text/html", "image/svg+xml", "application/xhtml+xml", "text/javascript"} {
		if passiveProxyContentType(kind) {
			t.Errorf("active type accepted: %s", kind)
		}
		if teeveeProxyContentType(kind, "https://example.com/media") == kind {
			t.Errorf("active type forwarded: %s", kind)
		}
	}
	for _, kind := range []string{"video/mp4", "audio/mpeg", "image/jpeg", "multipart/x-mixed-replace; boundary=frame", "application/vnd.apple.mpegurl"} {
		if !passiveProxyContentType(kind) {
			t.Errorf("media type rejected: %s", kind)
		}
	}
}
