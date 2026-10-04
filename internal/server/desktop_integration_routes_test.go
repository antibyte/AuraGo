package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesktopIntegrationRoutesEnforcePolicyBeforeSharedHandlers(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("fixture", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	for root := range desktopIntegrationRoots {
		t.Run(root, func(t *testing.T) {
			called := false
			h := desktopIntegrationHandler(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.URL.Path != "/api/"+root+"/fixture" {
					t.Fatal(r.URL.Path)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodPost, "/api/desktop/integrations/"+root+"/fixture", nil)
			r.Header.Set("Authorization", "Bearer "+adminToken)
			w := httptest.NewRecorder()
			h(w, r)
			if called || w.Code != 403 || !strings.Contains(w.Body.String(), "desktop_readonly") {
				t.Fatalf("mutation admitted: %d %s", w.Code, w.Body.String())
			}
			r.Method = http.MethodGet
			w = httptest.NewRecorder()
			h(w, r)
			if !called || w.Code != http.StatusNoContent {
				t.Fatalf("read blocked: %d %s", w.Code, w.Body.String())
			}
			called = false
			r.Header.Set("Authorization", "Bearer "+readToken)
			w = httptest.NewRecorder()
			h(w, r)
			if called || w.Code != 403 {
				t.Fatal("desktop read token widened integration permissions")
			}
		})
	}
}
