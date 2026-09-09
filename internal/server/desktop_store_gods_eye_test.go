package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/desktopstore"
)

func TestGodsEyeConfigRequiresAdminAndCSRF(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	s.Logger = slog.Default()
	handler := authMiddleware(s, handleDesktopStoreAppRoute(s))
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, token := range []string{"", readToken, writeToken} {
			r := httptest.NewRequest(method, "https://aurago.example/api/desktop/store/apps/gods-eye-view/config", bytes.NewBufferString(`{}`))
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
				t.Fatalf("non-admin %s admitted: %d", method, w.Code)
			}
		}
	}
	for _, origin := range []string{"", "https://foreign.example"} {
		r := httptest.NewRequest(http.MethodPut, "https://aurago.example/api/desktop/store/apps/gods-eye-view/config", bytes.NewBufferString(`{}`))
		r.Header.Set("Origin", origin)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-origin mutation admitted: %d", w.Code)
		}
	}
}

func TestGodsEyeConfigRejectsReadOnly(t *testing.T) {
	for _, policy := range [][3]bool{{true, true, false}, {false, true, true}, {false, false, false}} {
		s := testDesktopStorePolicyServer(t, policy[0], policy[1], policy[2])
		r := httptest.NewRequest(http.MethodPut, "/api/desktop/store/apps/gods-eye-view/config", bytes.NewBufferString(`{"keys":{"OPENAI_API_KEY":"fixture"}}`))
		w := httptest.NewRecorder()
		handleDesktopStoreAppRoute(s)(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("write was not denied: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestGodsEyeTailnetKeepsFramePolicy(t *testing.T) {
	specs, _ := desktopStoreTailscaleProxySpecs([]desktopstore.InstalledApp{{AppID: desktopstore.GodsEyeAppID, Status: desktopstore.AppStatusRunning, TailscaleEnabled: true, HostPort: 4173}})
	if len(specs) != 1 || !specs[0].PreserveFramePolicy {
		t.Fatal("GEV frame policy must survive the Tailnet proxy")
	}
}
