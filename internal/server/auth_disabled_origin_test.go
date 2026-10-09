package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestAuthDisabledBrowserWriteOrigins(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, tc := range []struct {
			name, origin, referer string
			status                int
		}{
			{"opaque", "null", "https://aurago.test/preview", http.StatusForbidden},
			{"foreign", "https://foreign.test", "", http.StatusForbidden},
			{"foreign_referer", "", "https://foreign.test/form", http.StatusForbidden},
			{"same_origin", "https://aurago.test", "", http.StatusNoContent},
			{"same_referer", "", "https://aurago.test/desktop", http.StatusNoContent},
			{"native_client", "", "", http.StatusNoContent},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				s := &Server{Cfg: &config.Config{}}
				called := false
				handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				}))
				req := httptest.NewRequest(method, "https://aurago.test/api/game-maker/projects", nil)
				req.Header.Set("Origin", tc.origin)
				req.Header.Set("Referer", tc.referer)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != tc.status || called != (tc.status == http.StatusNoContent) {
					t.Fatalf("status=%d handler_called=%v, want status=%d", rec.Code, called, tc.status)
				}
			})
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method+"/preview_asset", func(t *testing.T) {
			handler := authMiddleware(&Server{Cfg: &config.Config{}}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(method, "https://aurago.test/api/game-maker/preview/token/assets/sheet.png", nil)
			req.Header.Set("Origin", "null")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("preview asset request rejected: %d", rec.Code)
			}
		})
	}
}
